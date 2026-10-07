package acp

import (
	"context"
	"encoding/json"
	"path/filepath"
	"sync"

	acp "github.com/eino-contrib/acp"
	acpconn "github.com/eino-contrib/acp/conn"

	faceport "agent-vivy/sdk/port/face"
)

// Spec bounds: one connection owns at most 32 admitted sessions.
const maxSessionsPerConnection = 32

// requiredControlCapabilities is the §5 authority set initialization must
// find on the host; a miss is an initialization failure, not degradation.
var requiredControlCapabilities = []string{
	"session", "turn", "run", "run.subscribe",
	"approval", "question", "review",
}

// agent is the ACP Agent endpoint served on the owned stdio connection. It
// embeds BaseAgent so every unsupported method fails closed with the
// method-not-supported response, and owns the negotiated capabilities plus
// the owned-session map for this connection.
type agent struct {
	acp.BaseAgent

	host faceport.Host

	// initMu serializes the single connection-wide initialization call.
	initMu      sync.Mutex
	initialized bool

	mu           sync.Mutex
	sessions     map[string]*sessionState
	reservations int
	draining     bool
	conn         *acpconn.AgentConnection
}

// sessionState owns one admitted ACP session: the canonical workspace root
// resolved at admission and per-session prompt capacity/generation state.
type sessionState struct {
	root string

	mu sync.Mutex
	// nextGeneration numbers prompts owned by this session.
	nextGeneration uint64
}

func newAgent(host faceport.Host) *agent {
	return &agent{host: host, sessions: make(map[string]*sessionState)}
}

func (a *agent) bindConnection(conn *acpconn.AgentConnection) {
	a.mu.Lock()
	a.conn = conn
	a.mu.Unlock()
}

// drain tears down the private connection after an ambiguous outcome where
// the durable state may exist but cannot be observed (spec §5: ambiguous
// session creation drains the connection instead of retrying).
func (a *agent) drain() {
	a.mu.Lock()
	conn := a.conn
	a.draining = true
	a.mu.Unlock()
	if conn != nil {
		_ = conn.Close()
	}
}

func (a *agent) isInitialized() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.initialized
}

// Initialize performs the serialized connection-wide negotiation: it checks
// the host's Control capabilities, then advertises only the restricted-pilot
// surface (spec §5, §12.4).
func (a *agent) Initialize(ctx context.Context, req acp.InitializeRequest) (acp.InitializeResponse, error) {
	a.initMu.Lock()
	defer a.initMu.Unlock()

	a.mu.Lock()
	if a.initialized {
		a.mu.Unlock()
		return acp.InitializeResponse{},
			rpcError(-32602, "initialize already completed", "INVALID_INPUT")
	}
	a.mu.Unlock()

	raw, err := a.call(ctx, "initialize", nil)
	if err != nil {
		return acp.InitializeResponse{}, safeRPCError(err)
	}
	var caps struct {
		Capabilities      []string `json:"capabilities"`
		CodeModeAvailable bool     `json:"code_mode_available"`
	}
	if err := json.Unmarshal(raw, &caps); err != nil {
		return acp.InitializeResponse{},
			rpcError(-32603, "malformed host capabilities", "REQUIRED_CAPABILITY_UNAVAILABLE")
	}
	have := make(map[string]bool, len(caps.Capabilities))
	for _, c := range caps.Capabilities {
		have[c] = true
	}
	missing := false
	for _, req := range requiredControlCapabilities {
		if !have[req] {
			missing = true
			break
		}
	}
	if missing || !caps.CodeModeAvailable {
		return acp.InitializeResponse{},
			rpcError(-32603, "host lacks required capabilities", "REQUIRED_CAPABILITY_UNAVAILABLE")
	}

	a.mu.Lock()
	a.initialized = true
	a.mu.Unlock()

	return acp.InitializeResponse{
		ProtocolVersion: 1,
		AgentInfo: &acp.Implementation{
			Name:    moduleID,
			Title:   "Vivy ACP face (restricted pilot)",
			Version: moduleVer,
		},
		AgentCapabilities: &acp.AgentCapabilities{
			LoadSession:         false,
			PromptCapabilities:  &acp.PromptCapabilities{Image: false, Audio: false, EmbeddedContext: false},
			MCPCapabilities:     &acp.MCPCapabilities{HTTP: false, SSE: false},
			SessionCapabilities: &acp.SessionCapabilities{},
		},
		AuthMethods: []acp.AuthMethod{},
	}, nil
}

// NewSession validates the whole request before any durable session is
// created, then reserves capacity, calls session/create once, and stores
// only the real session ID plus the canonical root (spec §5, §12.4).
func (a *agent) NewSession(ctx context.Context, req acp.NewSessionRequest) (acp.NewSessionResponse, error) {
	if !a.isInitialized() {
		return acp.NewSessionResponse{},
			rpcError(-32602, "connection is not initialized", "INVALID_INPUT")
	}
	a.mu.Lock()
	if a.draining {
		a.mu.Unlock()
		return acp.NewSessionResponse{},
			rpcError(-32603, "connection is draining", "INTERNAL_FAILURE")
	}
	a.mu.Unlock()

	if err := validateNewSession(req); err != nil {
		return acp.NewSessionResponse{}, err
	}

	a.mu.Lock()
	if len(a.sessions)+a.reservations >= maxSessionsPerConnection {
		a.mu.Unlock()
		return acp.NewSessionResponse{},
			rpcError(-32001, "session capacity reached", "CAPACITY_EXCEEDED")
	}
	a.reservations++
	a.mu.Unlock()
	release := true
	defer func() {
		if release {
			a.mu.Lock()
			a.reservations--
			a.mu.Unlock()
		}
	}()

	raw, err := a.call(ctx, "session/create", map[string]string{"workspace_path": req.Cwd})
	if err != nil {
		if _, coded := rpcCodeOf(err); !coded {
			// Ambiguous: the durable session may exist but is
			// unobservable — drain rather than double-create.
			a.drain()
			return acp.NewSessionResponse{}, safeRPCError(err)
		}
		return acp.NewSessionResponse{}, safeRPCError(err)
	}
	var created struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(raw, &created); err != nil || created.ID == "" {
		a.drain()
		return acp.NewSessionResponse{},
			rpcError(-32603, "malformed session result", "INTERNAL_FAILURE")
	}

	a.mu.Lock()
	a.reservations--
	release = false
	a.sessions[created.ID] = &sessionState{root: filepath.Clean(req.Cwd)}
	a.mu.Unlock()
	return acp.NewSessionResponse{SessionID: acp.SessionID(created.ID)}, nil
}

// validateNewSession applies the §12.4 admission rules before any durable
// effect: one absolute control-character-free root, empty mcpServers, and no
// additional directories.
func validateNewSession(req acp.NewSessionRequest) error {
	if len(req.MCPServers) > 0 {
		return rpcError(-32602, "mcpServers are not supported in this pilot", "CLIENT_MCP_UNSUPPORTED")
	}
	if len(req.AdditionalDirectories) > 0 {
		return rpcError(-32602, "additionalDirectories are not supported", "INVALID_INPUT")
	}
	cwd := req.Cwd
	if cwd == "" || !filepath.IsAbs(cwd) || hasControlChar(cwd) {
		return rpcError(-32602, "cwd must be an absolute path without control characters", "INVALID_INPUT")
	}
	return nil
}

func hasControlChar(s string) bool {
	for _, r := range s {
		if r == 0 || (r < 0x20) || r == 0x7f {
			return true
		}
	}
	return false
}

// lookupSession resolves an owned session ID, returning -32002 for unknown
// or foreign IDs before any Control call (spec §5/§12.7).
func (a *agent) lookupSession(id string) (*sessionState, error) {
	a.mu.Lock()
	s, ok := a.sessions[id]
	a.mu.Unlock()
	if !ok {
		return nil, rpcError(-32002, "unknown session", "RESOURCE_NOT_FOUND")
	}
	return s, nil
}
