package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	goRuntime "runtime"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"

	"agent-vivy/internal/commandpolicy"
	"agent-vivy/internal/contexthost"
	"agent-vivy/internal/domain"
	"agent-vivy/internal/mcphost"
	"agent-vivy/internal/tools"

	"github.com/mark3labs/mcp-go/client"
	mcptransport "github.com/mark3labs/mcp-go/client/transport"
	"github.com/mark3labs/mcp-go/mcp"
)

const (
	defaultMCPTimeout     = 8 * time.Second
	defaultMCPHostRetries = 1
	maxMCPResponseBytes   = 512 << 10
	maxMCPContentBytes    = 256 << 10
	maxMCPResourcePages   = 32
)

type MCPServerConfig struct {
	Name           string
	Endpoint       string
	Command        string
	Args           []string
	EnvFrom        map[string]string
	Cwd            string
	AuthEnv        string
	ResourceBridge bool
	DeferredReason string
	// Enabled is optional at this runtime boundary so existing callers that
	// construct a config literal keep the safe default (enabled). Settings
	// resolves nil to true before writes; the pointer remains useful for
	// disabled saved entries that must stay visible in status projections.
	Enabled *bool
}

// MCPServerState is the snapshot vocabulary shared by MCPHost and the RPC/UI
// projection. The values intentionally use the stable wire spelling rather
// than the uppercase assembly manifest spelling.
type MCPServerState string

const (
	MCPStateNotCompiled  MCPServerState = "not-compiled"
	MCPStateUnconfigured MCPServerState = "unconfigured"
	MCPStateInactive     MCPServerState = "inactive"
	MCPStateReady        MCPServerState = "ready"
	MCPStateUnavailable  MCPServerState = "unavailable"
	MCPStateDeferred     MCPServerState = "deferred"
)

func MCPServerConfigEnabled(config MCPServerConfig) bool {
	return config.Enabled == nil || *config.Enabled
}

// MCPBackendOptions supplies the local-process policy used by stdio MCP
// servers. ProcessRoot is the only root from which configured relative cwd
// values may resolve. Logger is optional and defaults to slog.Default().
type MCPBackendOptions struct {
	ProcessRoot string
	Logger      *slog.Logger
}

// MCPBackend is the configuration/control facade for MCPHost. It retains only
// logical server settings and secret-free status records; every live
// mcp-go client and child process is created, initialized, replaced, and
// closed by MCPHost through the runtime adapter. Eino's MCP component is used
// there only for tool discovery/schema conversion; resulting tools enter the
// sole ToolHost governance path and are never mounted directly by this type.
type MCPBackend struct {
	client      *http.Client
	processRoot string
	logger      *slog.Logger
	// servers is a logical configuration/status catalog only. It never owns a
	// live mcp-go client or child process; MCPHost owns those resources through
	// the session factory below.
	servers          map[string]*mcpServer
	mu               sync.RWMutex
	maxResponseBytes int
	timeout          time.Duration
	closed           bool
	// The facade serializes legacy catalog reads so concurrent control-plane
	// callers do not turn an otherwise identical discovery into a Host
	// generation race. MCPHost still retains its strict stale-discovery guard
	// for direct dynamic-world consumers.
	discoveryMu sync.Mutex

	bridgeMu          sync.Mutex
	bridgeHost        *mcphost.Host
	bridgeWorld       *mcphost.ToolWorld
	bridgeContextHost *contexthost.Host
}

// mcpStatusRecord belongs to one logical server configuration. A session
// replacement keeps this record, while a settings replacement receives a new
// one, so delayed work from a retired configuration cannot contaminate the
// status of a same-named replacement.
type mcpStatusRecord struct {
	mu          sync.Mutex
	initError   string
	initialized bool
	toolCount   int
	closeOnce   sync.Once
	closeDone   chan struct{}
}

type mcpServer struct {
	config     MCPServerConfig
	cli        *client.Client
	rawSchemas *rawMCPToolSchemas
	status     *mcpStatusRecord

	processCtx    context.Context
	processCancel context.CancelFunc

	initMu  sync.Mutex
	mu      sync.Mutex
	initErr error
	caps    mcp.ServerCapabilities
	ready   bool
	dead    bool
	deadErr error

	closeOnce sync.Once
	closeDone chan struct{}
	closeErr  error
}

// rawMCPToolSchemas provides request-local collectors for the inputSchema
// bytes in one tools/list response. A server-scoped name cache is unsafe:
// omitted schemas would inherit an older contract and concurrent list calls
// for the same remote name could cross-contaminate one another.
type rawMCPToolSchemas struct{}

type rawMCPToolSchemaCapture struct {
	mu      sync.RWMutex
	byName  map[string]json.RawMessage
	present map[string]bool
}

type rawMCPToolSchemaCaptureKey struct{}

func newRawMCPToolSchemas() *rawMCPToolSchemas { return &rawMCPToolSchemas{} }

func withRawMCPToolSchemaCapture(ctx context.Context) (context.Context, *rawMCPToolSchemaCapture) {
	capture := &rawMCPToolSchemaCapture{
		byName:  make(map[string]json.RawMessage),
		present: make(map[string]bool),
	}
	return context.WithValue(ctx, rawMCPToolSchemaCaptureKey{}, capture), capture
}

func rawMCPToolSchemaCaptureFromContext(ctx context.Context) *rawMCPToolSchemaCapture {
	if ctx == nil {
		return nil
	}
	capture, _ := ctx.Value(rawMCPToolSchemaCaptureKey{}).(*rawMCPToolSchemaCapture)
	return capture
}

func (schemas *rawMCPToolSchemas) capture(ctx context.Context, response *mcptransport.JSONRPCResponse) {
	if schemas == nil {
		return
	}
	if capture := rawMCPToolSchemaCaptureFromContext(ctx); capture != nil {
		capture.capture(response)
	}
}

func (capture *rawMCPToolSchemaCapture) capture(response *mcptransport.JSONRPCResponse) {
	if capture == nil || response == nil || len(response.Result) == 0 {
		return
	}
	var result struct {
		Tools []struct {
			Name        string          `json:"name"`
			InputSchema json.RawMessage `json:"inputSchema"`
		} `json:"tools"`
	}
	if json.Unmarshal(response.Result, &result) != nil {
		return
	}
	capture.mu.Lock()
	defer capture.mu.Unlock()
	for _, tool := range result.Tools {
		name := strings.TrimSpace(tool.Name)
		if name == "" {
			continue
		}
		capture.present[name] = true
		capture.byName[name] = append(json.RawMessage(nil), tool.InputSchema...)
	}
}

func (capture *rawMCPToolSchemaCapture) get(name string) (json.RawMessage, bool) {
	if capture == nil {
		return nil, false
	}
	capture.mu.RLock()
	defer capture.mu.RUnlock()
	if !capture.present[name] {
		return nil, false
	}
	return append(json.RawMessage(nil), capture.byName[name]...), true
}

// capturingMCPTransport preserves only the untrusted tools/list schema
// document. All lifecycle, retry, notification and execution behavior stays
// in the pinned mcp-go transport.
type capturingMCPTransport struct {
	mcptransport.Interface
	rawSchemas *rawMCPToolSchemas
}

func (transport *capturingMCPTransport) SendRequest(ctx context.Context, request mcptransport.JSONRPCRequest) (*mcptransport.JSONRPCResponse, error) {
	response, err := transport.Interface.SendRequest(ctx, request)
	if err == nil && request.Method == string(mcp.MethodToolsList) {
		transport.rawSchemas.capture(ctx, response)
	}
	return response, err
}

// MCPServerStatus is a secret-free snapshot of one configured server's
// process state. Initialized means the backend completed the MCP handshake;
// configured servers are not reported as connected before that happens.
// Error is the sanitized last handshake or stdio lifecycle failure for this
// configuration (empty after a success); AuthMissing flags a configured auth
// env var that is currently unset; ToolCount is the last successful catalog
// size, negative when the server was never listed. A dead stdio process is
// represented through Error and never restarted by an MCP operation.
type MCPServerStatus struct {
	Name           string
	Transport      string
	State          MCPServerState
	Initialized    bool
	Error          string
	AuthMissing    bool
	EnvMissing     []string
	ToolCount      int
	ResourceBridge bool
	DeferredReason string
}

var _ tools.MCPOperations = (*MCPBackend)(nil)
var _ tools.MCPResourceOperations = (*MCPBackend)(nil)
var _ tools.MCPPromptOperations = (*MCPBackend)(nil)

// NewMCPBackend builds a backend without opening any network connection.
// Each server is started and initialized lazily on its first operation.
func NewMCPBackend(configs []MCPServerConfig, httpClient *http.Client) *MCPBackend {
	return NewMCPBackendWithOptions(configs, httpClient, MCPBackendOptions{})
}

// NewMCPBackendWithOptions is the policy-aware constructor used by the app
// composition root. The compatibility constructor above keeps focused HTTP
// tests and embedders on the original seam.
func NewMCPBackendWithOptions(configs []MCPServerConfig, httpClient *http.Client, options MCPBackendOptions) *MCPBackend {
	backend := &MCPBackend{
		maxResponseBytes: maxMCPResponseBytes,
		timeout:          defaultMCPTimeout,
		processRoot:      strings.TrimSpace(options.ProcessRoot),
		logger:           options.Logger,
		servers:          make(map[string]*mcpServer),
	}
	if backend.logger == nil {
		backend.logger = slog.Default()
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: defaultMCPTimeout}
	}
	backend.client = boundedMCPHTTPClient(httpClient, func() int { return backend.maxResponseBytes })
	backend.ReplaceServers(configs)
	return backend
}

func normalizeMCPServerConfig(config MCPServerConfig) MCPServerConfig {
	if !safeMCPServerName(config.Name) {
		return config
	}
	config.Endpoint = strings.TrimSpace(config.Endpoint)
	config.Command = strings.TrimSpace(config.Command)
	config.Cwd = strings.TrimSpace(config.Cwd)
	config.AuthEnv = strings.TrimSpace(config.AuthEnv)
	config.DeferredReason = strings.TrimSpace(config.DeferredReason)
	config.Args = append([]string(nil), config.Args...)
	if config.Enabled != nil {
		enabled := *config.Enabled
		config.Enabled = &enabled
	}
	if config.EnvFrom != nil {
		envFrom := make(map[string]string, len(config.EnvFrom))
		for child, host := range config.EnvFrom {
			envFrom[strings.TrimSpace(child)] = strings.TrimSpace(host)
		}
		config.EnvFrom = envFrom
	}
	return config
}

// safeMCPServerName mirrors the MCP namespace contract at the runtime
// boundary. Names are identities, so unsafe input must be rejected before any
// trimming or map lookup can alias it to a configured server.
func safeMCPServerName(name string) bool {
	if len(name) == 0 || len(name) > 128 {
		return false
	}
	if !isASCIIMCPNameLetter(name[0]) && !isASCIIMCPNameDigit(name[0]) {
		return false
	}
	for i := 1; i < len(name); i++ {
		value := name[i]
		if !isASCIIMCPNameLetter(value) && !isASCIIMCPNameDigit(value) && value != '_' && value != '-' {
			return false
		}
	}
	return true
}

func isASCIIMCPNameLetter(value byte) bool {
	return value >= 'A' && value <= 'Z' || value >= 'a' && value <= 'z'
}

func isASCIIMCPNameDigit(value byte) bool { return value >= '0' && value <= '9' }

func sameMCPServerConfig(left, right MCPServerConfig) bool {
	return reflect.DeepEqual(left, right)
}

func hasMCPTransport(config MCPServerConfig) bool {
	return strings.TrimSpace(config.Endpoint) != "" || strings.TrimSpace(config.Command) != ""
}

func mcpServerTransport(config MCPServerConfig) string {
	if config.isStdio() {
		return "stdio"
	}
	return "http"
}

func (config MCPServerConfig) isStdio() bool {
	return strings.TrimSpace(config.Command) != ""
}

// ReplaceServers swaps the live configuration catalog. Transport sessions are
// deliberately not stored here: the MCPHost session factory creates and owns
// each live client/process, and Host.ReplaceInstances retires and closes it.
func (b *MCPBackend) ReplaceServers(configs []MCPServerConfig) {
	normalized := make(map[string]MCPServerConfig, len(configs))
	for _, config := range configs {
		config = normalizeMCPServerConfig(config)
		if !safeMCPServerName(config.Name) {
			continue
		}
		normalized[config.Name] = config
	}

	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		return
	}
	old := b.servers
	next := make(map[string]*mcpServer, len(normalized))
	for name, config := range normalized {
		if previous := old[name]; previous != nil && sameMCPServerConfig(previous.config, config) {
			next[name] = previous
			continue
		}
		next[name] = newMCPStatusServer(config)
	}
	b.servers = next
	b.mu.Unlock()

	b.replaceMCPBridge()
}

func newMCPStatusServer(config MCPServerConfig) *mcpServer {
	status := &mcpStatusRecord{toolCount: -1, closeDone: make(chan struct{})}
	return &mcpServer{
		config: config,
		status: status,
		// The logical status entry exposes the same close signal as the
		// host-owned transport that represents it. This preserves the
		// configuration facade's observable cleanup boundary without making
		// the facade a transport owner.
		closeDone: status.closeDone,
	}
}

// statusForConfig returns the status record only when the Host's immutable
// instance configuration still names this exact backend generation. A Host
// operation may be opening a retired configuration while ReplaceServers has
// already published a same-named replacement; binding by ID in that window
// would let the retired transport mark the replacement ready.
func (b *MCPBackend) statusForConfig(config MCPServerConfig) *mcpServer {
	if b == nil {
		return nil
	}
	config = normalizeMCPServerConfig(config)
	b.mu.RLock()
	defer b.mu.RUnlock()
	entry := b.servers[config.Name]
	if entry == nil || !sameMCPServerConfig(entry.config, config) {
		return nil
	}
	return entry
}

// ConfiguredServers returns a deterministic snapshot of the live catalog.
func (b *MCPBackend) ConfiguredServers() []MCPServerConfig {
	b.mu.RLock()
	out := make([]MCPServerConfig, 0, len(b.servers))
	for _, entry := range b.servers {
		out = append(out, normalizeMCPServerConfig(entry.config))
	}
	b.mu.RUnlock()
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// ServerStatuses reports the backend's current in-process MCP truth without
// probing the network or exposing endpoints and authentication metadata.
func (b *MCPBackend) ServerStatuses() []MCPServerStatus {
	b.mu.RLock()
	out := make([]MCPServerStatus, 0, len(b.servers))
	for name, entry := range b.servers {
		entry.status.mu.Lock()
		status := MCPServerStatus{
			Name:           name,
			Transport:      mcpServerTransport(entry.config),
			Initialized:    entry.status.initialized,
			Error:          entry.status.initError,
			ToolCount:      entry.status.toolCount,
			State:          MCPStateInactive,
			ResourceBridge: entry.config.ResourceBridge,
			DeferredReason: entry.config.DeferredReason,
		}
		if !MCPServerConfigEnabled(entry.config) {
			status.State = MCPStateInactive
		} else if entry.config.DeferredReason != "" {
			status.State = MCPStateDeferred
		} else if !hasMCPTransport(entry.config) {
			status.State = MCPStateUnconfigured
		} else if status.Initialized {
			status.State = MCPStateReady
		}
		if entry.config.isStdio() {
			status.EnvMissing = missingMCPEnvChildren(entry.config.EnvFrom)
			if len(status.EnvMissing) > 0 {
				status.Error = "mcp: required environment variables are missing"
			}
		}
		entry.status.mu.Unlock()
		if entry.config.AuthEnv != "" && os.Getenv(entry.config.AuthEnv) == "" {
			status.AuthMissing = true
		}
		if MCPServerConfigEnabled(entry.config) && status.State != MCPStateUnconfigured && status.State != MCPStateDeferred &&
			(status.Error != "" || status.AuthMissing || len(status.EnvMissing) > 0) {
			status.State = MCPStateUnavailable
		}
		out = append(out, status)
	}
	b.mu.RUnlock()
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// noteInitResult records or clears the handshake failure after an
// initialization attempt. The backend read lock makes the current-entry check
// and status write one operation relative to ReplaceServers.
func (b *MCPBackend) noteInitResult(entry *mcpServer, err error) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if entry == nil || b.closed {
		return
	}
	current := b.servers[entry.config.Name]
	if current == nil || current.status != entry.status {
		return
	}
	entry.status.mu.Lock()
	defer entry.status.mu.Unlock()
	entry.status.initialized = err == nil
	if err == nil {
		entry.status.initError = ""
		return
	}
	entry.status.initError = b.sanitizeMCPStatusError(entry, err)
}

// noteToolCount records the projected catalog size after a successful listing.
// Host-owned session adapters pass their concrete transport entry; the
// status-pointer check rejects writes from a retired configuration.
func (b *MCPBackend) noteToolCount(entry *mcpServer, count int) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if entry == nil || b.closed {
		return
	}
	current := b.servers[entry.config.Name]
	if current == nil || current.status != entry.status {
		return
	}
	entry.status.mu.Lock()
	entry.status.toolCount = count
	entry.status.mu.Unlock()
}

// SanitizeMCPError is the control-plane error seam. It keeps endpoints,
// commands, cwd/path material, and resolved environment values out of RPC,
// browser, and TUI projections while preserving a short diagnostic cause.
func (b *MCPBackend) SanitizeMCPError(name string, err error) string {
	if err == nil {
		return ""
	}
	b.mu.RLock()
	entry := b.servers[name]
	b.mu.RUnlock()
	if entry == nil {
		return boundedMCPStatusError(err.Error())
	}
	return b.sanitizeMCPStatusError(entry, err)
}

func (b *MCPBackend) sanitizeMCPStatusError(entry *mcpServer, err error) string {
	if err == nil {
		return ""
	}
	redactions := []string{
		entry.config.Endpoint,
		entry.config.Command,
		entry.config.Cwd,
		b.processRoot,
	}
	if root := strings.TrimSpace(b.processRoot); root != "" {
		if absolute, absoluteErr := filepath.Abs(root); absoluteErr == nil {
			redactions = append(redactions, absolute)
			if entry.config.Cwd != "" {
				redactions = append(redactions, filepath.Join(absolute, filepath.Clean(entry.config.Cwd)))
			}
		}
	}
	if entry.config.AuthEnv != "" {
		if value, ok := os.LookupEnv(entry.config.AuthEnv); ok {
			redactions = append(redactions, value)
		}
	}
	for _, host := range entry.config.EnvFrom {
		if value, ok := os.LookupEnv(strings.TrimSpace(host)); ok {
			redactions = append(redactions, value)
		}
	}
	return boundedMCPStatusError(err.Error(), redactions...)
}

// boundedMCPStatusError keeps the sidebar secret-free: configured endpoint,
// command, cwd/path material, and environment values are redacted by the
// caller; control characters are stripped, and the message is capped at 160
// runes.
func boundedMCPStatusError(message string, redactions ...string) string {
	for _, redaction := range redactions {
		if redaction != "" {
			message = strings.ReplaceAll(message, redaction, "")
		}
	}
	message = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || isMCPStatusBidiControl(r) {
			return -1
		}
		return r
	}, message)
	runes := []rune(strings.TrimSpace(message))
	if len(runes) > 160 {
		runes = runes[:160]
	}
	return string(runes)
}

func isMCPStatusBidiControl(r rune) bool {
	return r == '\u061c' || r == '\u200e' || r == '\u200f' || (r >= '\u202a' && r <= '\u202e') || (r >= '\u2066' && r <= '\u2069')
}

// Close closes the MCPHost authority and marks this configuration/control
// facade closed. Host owns all live transport/session cleanup; the facade
// never performs a second client/process close.
func (b *MCPBackend) Close() error {
	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		return nil
	}
	b.closed = true
	b.servers = make(map[string]*mcpServer)
	b.mu.Unlock()
	return b.closeMCPBridge()
}

func (b *MCPBackend) ensureOpen() error {
	if b == nil {
		return errors.New("mcp: backend is closed")
	}
	b.mu.RLock()
	closed := b.closed
	b.mu.RUnlock()
	if closed {
		return errors.New("mcp: backend is closed")
	}
	return nil
}

func (b *MCPBackend) ListTools(ctx context.Context, _ domain.RunID, server string) (tools.MCPListResponse, error) {
	if err := b.ensureOpen(); err != nil {
		return tools.MCPListResponse{}, err
	}
	ctx, cancel := b.operationContext(ctx)
	defer cancel()
	if server != "" {
		result, err := b.listToolsFromHost(ctx, server)
		if err != nil {
			return tools.MCPListResponse{}, err
		}
		return tools.MCPListResponse{Tools: result, Untrusted: true}, nil
	}

	all := make([]tools.MCPTool, 0)
	used := 0
	for _, name := range b.serverNames() {
		result, err := b.listToolsFromHost(ctx, name)
		if err != nil {
			return tools.MCPListResponse{}, err
		}
		for _, item := range result {
			size := mcpToolProjectedSize(item)
			if size > maxMCPContentBytes-used {
				return tools.MCPListResponse{Tools: all, Untrusted: true}, nil
			}
			all = append(all, item)
			used += size
		}
	}
	return tools.MCPListResponse{Tools: all, Untrusted: true}, nil
}

func (b *MCPBackend) listToolsFromHost(ctx context.Context, server string) ([]tools.MCPTool, error) {
	b.discoveryMu.Lock()
	defer b.discoveryMu.Unlock()
	host, err := b.ensureMCPBridge()
	if err != nil {
		return nil, err
	}
	definitions, err := host.DiscoverTools(ctx, server)
	if err != nil {
		return nil, err
	}
	out := make([]tools.MCPTool, 0, len(definitions))
	for _, definition := range definitions {
		out = append(out, tools.MCPTool{Server: server, Name: definition.RemoteName, Description: definition.Description, InputSchema: append(json.RawMessage(nil), definition.Schema...)})
	}
	return out, nil
}

func mcpToolProjectedSize(tool tools.MCPTool) int {
	return len(tool.Server) + len(tool.Name) + len(tool.Description) + len(tool.InputSchema)
}

// ListResources performs read-only resources/list. An empty server fans out
// in deterministic catalog order and applies one shared content budget.
func (b *MCPBackend) ListResources(ctx context.Context, _ domain.RunID, server string) (tools.MCPListResourcesResponse, error) {
	if err := b.ensureOpen(); err != nil {
		return tools.MCPListResourcesResponse{}, err
	}
	ctx, cancel := b.operationContext(ctx)
	defer cancel()
	if server != "" {
		resources, err := b.hostListResources(ctx, server)
		if err != nil {
			return tools.MCPListResourcesResponse{}, err
		}
		return tools.MCPListResourcesResponse{Server: server, Resources: resources, Untrusted: true}, nil
	}

	all := make([]tools.MCPResource, 0)
	used := 0
	for _, name := range b.serverNames() {
		if used >= maxMCPContentBytes {
			break
		}
		resources, err := b.hostListResources(ctx, name)
		if err != nil {
			return tools.MCPListResourcesResponse{}, err
		}
		for _, resource := range resources {
			if size := mcpResourceProjectedSize(resource); size > maxMCPContentBytes-used {
				break
			}
			all = append(all, resource)
			used += mcpResourceProjectedSize(resource)
		}
	}
	return tools.MCPListResourcesResponse{Resources: all, Untrusted: true}, nil
}

// ListPrompts returns bounded, untrusted prompt metadata. Servers without the
// prompts capability fail closed to an empty catalog.
func (b *MCPBackend) ListPrompts(ctx context.Context, _ domain.RunID, server string) (tools.MCPListPromptsResponse, error) {
	if err := b.ensureOpen(); err != nil {
		return tools.MCPListPromptsResponse{}, err
	}
	ctx, cancel := b.operationContext(ctx)
	defer cancel()
	if server != "" {
		prompts, err := b.listPromptsServer(ctx, server)
		return tools.MCPListPromptsResponse{Prompts: prompts, Untrusted: true}, err
	}

	all := make([]tools.MCPPrompt, 0)
	used := 0
	for _, name := range b.serverNames() {
		prompts, err := b.listPromptsServer(ctx, name)
		if err != nil {
			return tools.MCPListPromptsResponse{}, err
		}
		for _, prompt := range prompts {
			size := mcpPromptProjectedSize(prompt)
			if size > maxMCPContentBytes-used {
				return tools.MCPListPromptsResponse{Prompts: all, Untrusted: true}, nil
			}
			all = append(all, prompt)
			used += size
		}
	}
	return tools.MCPListPromptsResponse{Prompts: all, Untrusted: true}, nil
}

func (b *MCPBackend) listPromptsServer(ctx context.Context, server string) ([]tools.MCPPrompt, error) {
	host, err := b.ensureMCPBridge()
	if err != nil {
		return nil, err
	}
	prompts, err := host.ListPrompts(ctx, server)
	if err != nil {
		return nil, err
	}
	out := make([]tools.MCPPrompt, 0, len(prompts))
	for _, prompt := range prompts {
		item := tools.MCPPrompt{Server: server, Name: prompt.Name, Title: prompt.Title, Description: prompt.Description}
		for _, argument := range prompt.Arguments {
			item.Arguments = append(item.Arguments, tools.MCPPromptArgument{Name: argument.Name, Title: argument.Title, Description: argument.Description, Required: argument.Required})
		}
		out = append(out, item)
	}
	return out, nil
}

// GetPrompt fetches one prompt and flattens user text into bounded, untrusted
// model input. Non-text content is rejected rather than guessed.
func (b *MCPBackend) GetPrompt(ctx context.Context, _ domain.RunID, request tools.MCPGetPromptRequest) (tools.MCPGetPromptResponse, error) {
	if err := b.ensureOpen(); err != nil {
		return tools.MCPGetPromptResponse{}, err
	}
	ctx, cancel := b.operationContext(ctx)
	defer cancel()
	if strings.TrimSpace(request.Name) == "" {
		return tools.MCPGetPromptResponse{}, errors.New("mcp: prompt name is required")
	}
	host, err := b.ensureMCPBridge()
	if err != nil {
		return tools.MCPGetPromptResponse{}, err
	}
	prompt, err := host.GetPrompt(ctx, request.Server, request.Name, request.Arguments)
	if err != nil {
		return tools.MCPGetPromptResponse{}, err
	}
	return tools.MCPGetPromptResponse{Server: request.Server, Name: prompt.Name, Description: prompt.Description, Text: prompt.Text, Untrusted: true}, nil
}

func (b *MCPBackend) listResourcesServer(ctx context.Context, server string, limit int) ([]tools.MCPResource, error) {
	host, err := b.ensureMCPBridge()
	if err != nil {
		return nil, err
	}
	resources, err := host.ListResources(ctx, server)
	if err != nil {
		return nil, err
	}
	return projectHostResources(server, resources, limit), nil
}

// ReadResource performs resources/read. The URI is returned verbatim and
// content remains untrusted text/base64; it is never mounted or written.
func (b *MCPBackend) ReadResource(ctx context.Context, _ domain.RunID, request tools.MCPReadResourceRequest) (tools.MCPReadResourceResponse, error) {
	if err := b.ensureOpen(); err != nil {
		return tools.MCPReadResourceResponse{}, err
	}
	ctx, cancel := b.operationContext(ctx)
	defer cancel()
	if strings.TrimSpace(request.URI) == "" {
		return tools.MCPReadResourceResponse{}, errors.New("mcp: resource URI is required")
	}
	if len(request.URI) > maxMCPContentBytes {
		return tools.MCPReadResourceResponse{}, fmt.Errorf("mcp: resource URI exceeds size limit (%d bytes)", maxMCPContentBytes)
	}
	host, err := b.ensureMCPBridge()
	if err != nil {
		return tools.MCPReadResourceResponse{}, err
	}
	content, err := host.ReadResource(ctx, request.Server, request.URI)
	if err != nil {
		return tools.MCPReadResourceResponse{}, err
	}
	result := tools.MCPReadResourceResponse{Server: request.Server, URI: request.URI, Untrusted: true}
	if content.Text != "" {
		text := content.Text
		result.Contents = append(result.Contents, tools.MCPResourceContent{URI: content.URI, MIME: content.MediaType, Text: &text})
	}
	if len(content.Blob) > 0 {
		blob := string(content.Blob)
		result.Contents = append(result.Contents, tools.MCPResourceContent{URI: content.URI, MIME: content.MediaType, Blob: &blob})
	}
	return result, nil
}

func (b *MCPBackend) newServer(config MCPServerConfig) (*mcpServer, error) {
	return b.newServerWithStatus(config, nil)
}

// newServerWithStatus constructs one transport session for MCPHost. The
// returned value is owned by that Host session; the optional status record is
// only a secret-free projection and never a transport owner.
func (b *MCPBackend) newServerWithStatus(config MCPServerConfig, record *mcpServer) (*mcpServer, error) {
	config = normalizeMCPServerConfig(config)
	status := (*mcpStatusRecord)(nil)
	if record != nil {
		status = record.status
	}
	if status == nil {
		status = &mcpStatusRecord{toolCount: -1, closeDone: make(chan struct{})}
	}
	if !hasMCPTransport(config) {
		return &mcpServer{
			config:    config,
			status:    status,
			closeDone: make(chan struct{}),
		}, nil
	}
	var cli *client.Client
	var processCtx context.Context
	var processCancel context.CancelFunc
	rawSchemas := newRawMCPToolSchemas()
	if config.isStdio() {
		processCtx, processCancel = context.WithCancel(context.Background())
		stdio := mcptransport.NewStdioWithOptions(config.Command, nil, config.Args,
			mcptransport.WithCommandFunc(b.stdioCommandFunc(config)),
		)
		// NewStdioWithOptions only constructs the official transport. Start is
		// deliberately deferred to initialize so the backend remains lazy.
		cli = client.NewClient(&capturingMCPTransport{Interface: stdio, rawSchemas: rawSchemas})
	} else {
		streamable, transportErr := mcptransport.NewStreamableHTTP(config.Endpoint,
			mcptransport.WithHTTPBasicClient(b.client),
			mcptransport.WithHTTPHeaderFunc(func(context.Context) map[string]string {
				if config.AuthEnv == "" {
					return nil
				}
				if token := os.Getenv(config.AuthEnv); token != "" {
					return map[string]string{"Authorization": "Bearer " + token}
				}
				return nil
			}),
		)
		if transportErr != nil {
			return nil, fmt.Errorf("mcp %s: create streamable HTTP client: %w", config.Name, transportErr)
		}
		options := make([]client.ClientOption, 0, 1)
		if streamable.GetSessionId() != "" {
			options = append(options, client.WithSession())
		}
		cli = client.NewClient(&capturingMCPTransport{Interface: streamable, rawSchemas: rawSchemas}, options...)
	}
	return &mcpServer{
		config:        config,
		cli:           cli,
		rawSchemas:    rawSchemas,
		status:        status,
		processCtx:    processCtx,
		processCancel: processCancel,
		closeDone:     make(chan struct{}),
	}, nil
}

func (b *MCPBackend) stdioCommandFunc(config MCPServerConfig) mcptransport.CommandFunc {
	return func(ctx context.Context, command string, _ []string, args []string) (*exec.Cmd, error) {
		if err := validateMCPRuntimeConfig(config); err != nil {
			return nil, err
		}
		env, err := resolveMCPEnv(config.EnvFrom)
		if err != nil {
			return nil, err
		}
		cwd, err := b.resolveMCPWorkingDir(config.Cwd)
		if err != nil {
			return nil, err
		}

		executable := command
		commandArgs := append([]string(nil), args...)
		if goRuntime.GOOS == "windows" {
			ext := strings.ToLower(filepath.Ext(command))
			if ext == ".cmd" || ext == ".bat" {
				comspec := strings.TrimSpace(os.Getenv("ComSpec"))
				if comspec == "" {
					comspec = "cmd.exe"
				}
				line, err := windowsMCPCommandLine(command, args)
				if err != nil {
					return nil, err
				}
				executable = comspec
				commandArgs = []string{"/d", "/s", "/c", line}
			}
		}
		cmd := exec.CommandContext(ctx, executable, commandArgs...)
		configureMCPProcess(cmd)
		cmd.Env = env
		cmd.Dir = cwd
		cmd.WaitDelay = 2 * time.Second
		b.logger.Info("mcp stdio spawn", "server", config.Name, "command", filepath.Base(command))
		return cmd, nil
	}
}

func validateMCPRuntimeConfig(config MCPServerConfig) error {
	if strings.TrimSpace(config.Command) == "" {
		return errors.New("mcp: stdio command is empty")
	}
	command := strings.TrimSpace(config.Command)
	abs := filepath.IsAbs(command)
	if strings.ContainsAny(command, "\t\r\n;&|><$()\"'`") || (!abs && strings.ContainsRune(command, ' ')) {
		return errors.New("mcp: stdio command contains shell syntax")
	}
	if strings.ContainsAny(command, `/\\`) && !abs {
		return errors.New("mcp: stdio command must be a PATH name or absolute path")
	}
	base := strings.ToLower(filepath.Base(command))
	ext := filepath.Ext(base)
	if ext == ".exe" || ext == ".cmd" || ext == ".bat" {
		base = strings.TrimSuffix(base, ext)
	}
	if commandpolicy.IsDeniedExecutable(command) {
		return fmt.Errorf("mcp: executable %q is denied by the MCP command safety policy", base)
	}
	if len(config.Args) > maxMCPArgsRuntime {
		return fmt.Errorf("mcp: too many stdio arguments (maximum %d)", maxMCPArgsRuntime)
	}
	total := 0
	for _, arg := range config.Args {
		if len(arg) > maxMCPArgBytesRuntime {
			return fmt.Errorf("mcp: stdio argument exceeds %d bytes", maxMCPArgBytesRuntime)
		}
		if strings.IndexByte(arg, 0) >= 0 || strings.ContainsAny(arg, "\r\n") {
			return errors.New("mcp: stdio argument contains NUL or newline")
		}
		if (ext == ".cmd" || ext == ".bat") && strings.ContainsAny(arg, "&|<>^%") {
			return errors.New("mcp: batch-file argument contains shell metacharacters")
		}
		total += len(arg)
		if total > maxMCPArgsBytesRuntime {
			return fmt.Errorf("mcp: stdio argument payload exceeds %d bytes", maxMCPArgsBytesRuntime)
		}
	}
	if err := validateMCPWorkingDirRuntime(config.Cwd); err != nil {
		return err
	}
	return nil
}

func resolveMCPEnv(envFrom map[string]string) ([]string, error) {
	env := make([]string, 0, 6+len(envFrom))
	for _, key := range []string{"PATH", "PATHEXT", "SYSTEMROOT", "WINDIR", "TEMP", "TMP", "ComSpec"} {
		if value := os.Getenv(key); value != "" {
			env = append(env, key+"="+value)
		}
	}
	keys := make([]string, 0, len(envFrom))
	for child := range envFrom {
		keys = append(keys, child)
	}
	sort.Strings(keys)
	for _, child := range keys {
		host := strings.TrimSpace(envFrom[child])
		child = strings.TrimSpace(child)
		if !validMCPEnvKey(child) || !validMCPEnvKey(host) {
			return nil, fmt.Errorf("mcp: env_from %q must map environment variable names", child)
		}
		value, ok := os.LookupEnv(host)
		if !ok || value == "" {
			return nil, fmt.Errorf("mcp: required environment variable for child %q is not set", child)
		}
		if len(value) > maxMCPEnvValueBytes || strings.IndexByte(value, 0) >= 0 {
			return nil, fmt.Errorf("mcp: environment value for child %q is invalid or too large", child)
		}
		env = append(env, child+"="+value)
	}
	return env, nil
}

func missingMCPEnvChildren(envFrom map[string]string) []string {
	children := make([]string, 0, len(envFrom))
	for child, host := range envFrom {
		child = strings.TrimSpace(child)
		host = strings.TrimSpace(host)
		if value, ok := os.LookupEnv(host); !ok || value == "" {
			children = append(children, child)
		}
	}
	sort.Strings(children)
	return children
}

func (b *MCPBackend) resolveMCPWorkingDir(cwd string) (string, error) {
	if err := validateMCPWorkingDirRuntime(cwd); err != nil {
		return "", err
	}
	root := strings.TrimSpace(b.processRoot)
	if root == "" {
		return "", errors.New("mcp: stdio process root is not configured")
	}
	root, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("mcp: resolve process root: %w", err)
	}
	realRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", fmt.Errorf("mcp: resolve process root: %w", err)
	}
	target := filepath.Join(realRoot, filepath.Clean(cwd))
	realTarget, err := filepath.EvalSymlinks(target)
	if err != nil {
		return "", fmt.Errorf("mcp: resolve stdio cwd: %w", err)
	}
	relative, err := filepath.Rel(realRoot, realTarget)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
		return "", errors.New("mcp: stdio cwd escapes runtime.workspace_root")
	}
	info, err := os.Stat(realTarget)
	if err != nil || !info.IsDir() {
		return "", errors.New("mcp: stdio cwd is not a directory")
	}
	return realTarget, nil
}

func validateMCPWorkingDirRuntime(cwd string) error {
	if cwd == "" {
		return nil
	}
	if strings.IndexByte(cwd, 0) >= 0 || filepath.IsAbs(cwd) {
		return errors.New("mcp: stdio cwd must be relative to runtime.workspace_root")
	}
	clean := filepath.Clean(cwd)
	if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return errors.New("mcp: stdio cwd escapes runtime.workspace_root")
	}
	return nil
}

func windowsMCPCommandLine(command string, args []string) (string, error) {
	parts := make([]string, 0, 1+len(args))
	for _, value := range append([]string{command}, args...) {
		if strings.ContainsAny(value, "\r\n\x00&|<>^%") {
			return "", errors.New("mcp: Windows batch command contains shell metacharacters")
		}
		parts = append(parts, windowsMCPQuote(value))
	}
	return strings.Join(parts, " "), nil
}

func windowsMCPQuote(value string) string {
	if value == "" {
		return `""`
	}
	if !strings.ContainsAny(value, " \t\"") {
		return value
	}
	return `"` + strings.ReplaceAll(value, `"`, `\"`) + `"`
}

func validMCPEnvKey(value string) bool {
	if value == "" || (value[0] < 'A' || value[0] > 'Z') {
		return false
	}
	for _, ch := range value[1:] {
		if (ch < 'A' || ch > 'Z') && (ch < '0' || ch > '9') && ch != '_' {
			return false
		}
	}
	return true
}

const (
	maxMCPArgsRuntime      = 128
	maxMCPArgBytesRuntime  = 4096
	maxMCPArgsBytesRuntime = 64 << 10
	maxMCPEnvValueBytes    = 4096
)

func (b *MCPBackend) lookupServer(name string) (*mcpServer, error) {
	if !safeMCPServerName(name) {
		return nil, fmt.Errorf("mcp: server %q is not a safe namespace identifier", name)
	}
	b.mu.RLock()
	entry := b.servers[name]
	closed := b.closed
	b.mu.RUnlock()
	if closed {
		return nil, errors.New("mcp: backend is closed")
	}
	if entry == nil {
		return nil, fmt.Errorf("mcp: server %q is not configured", name)
	}
	if !MCPServerConfigEnabled(entry.config) {
		return nil, fmt.Errorf("mcp: server %q is inactive", name)
	}
	if entry.config.DeferredReason != "" {
		return nil, fmt.Errorf("mcp: server %q is deferred", name)
	}
	if !hasMCPTransport(entry.config) {
		return nil, fmt.Errorf("mcp: server %q is not configured", name)
	}
	return entry, nil
}

func (b *MCPBackend) acquireServer(name string) (*mcpServer, error) {
	if !safeMCPServerName(name) {
		return nil, fmt.Errorf("mcp: server %q is not a safe namespace identifier", name)
	}
	b.mu.RLock()
	defer b.mu.RUnlock()
	if b.closed {
		return nil, errors.New("mcp: backend is closed")
	}
	entry := b.servers[name]
	if entry == nil {
		return nil, fmt.Errorf("mcp: server %q is not configured", name)
	}
	if !MCPServerConfigEnabled(entry.config) {
		return nil, fmt.Errorf("mcp: server %q is inactive", name)
	}
	if entry.config.DeferredReason != "" {
		return nil, fmt.Errorf("mcp: server %q is deferred", name)
	}
	if !hasMCPTransport(entry.config) {
		return nil, fmt.Errorf("mcp: server %q is not configured", name)
	}
	return entry, nil
}

func (b *MCPBackend) serverNames() []string {
	b.mu.RLock()
	names := make([]string, 0, len(b.servers))
	for name := range b.servers {
		names = append(names, name)
	}
	b.mu.RUnlock()
	sort.Strings(names)
	return names
}

func (b *MCPBackend) operationContext(ctx context.Context) (context.Context, context.CancelFunc) {
	if b.timeout <= 0 {
		return context.WithCancel(ctx)
	}
	return context.WithTimeout(ctx, b.timeout)
}

// acquire/release remain inert source-compatibility shims for older internal
// tests. They do not guard or close a transport: MCPHost owns that lifecycle.
func (*mcpServer) acquire() bool { return true }
func (*mcpServer) release()      {}

func (s *mcpServer) closeClient() error {
	s.closeOnce.Do(func() {
		if s.processCancel != nil {
			s.processCancel()
		}
		var err error
		if s.cli != nil {
			err = s.cli.Close()
		}
		if s.processCancel != nil && isExpectedMCPStdioCloseError(err) {
			// CommandContext intentionally terminates a local child during
			// shutdown. mcp-go reports that expected termination as an
			// exec.ExitError on Windows and Unix; it is not a backend close
			// failure. Preserve real transport timeout/cleanup errors.
			err = nil
		}
		s.mu.Lock()
		s.closeErr = err
		s.mu.Unlock()
		if s.status != nil {
			s.status.closeOnce.Do(func() { close(s.status.closeDone) })
		}
		if s.status == nil || s.closeDone != s.status.closeDone {
			close(s.closeDone)
		}
	})
	<-s.closeDone
	s.mu.Lock()
	err := s.closeErr
	s.mu.Unlock()
	return err
}

func isExpectedMCPStdioCloseError(err error) bool {
	if err == nil || errors.Is(err, context.Canceled) || errors.Is(err, os.ErrProcessDone) {
		return true
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return true
	}
	// On Windows, CommandContext can race mcp-go's Wait/TerminateProcess
	// cleanup and surface an *exec.Error instead of ExitError. The process is
	// already under intentional shutdown here; this message is not a useful
	// backend failure and must not make Close flaky.
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "canceling cmd") || strings.Contains(message, "terminateprocess")
}

func (s *mcpServer) markDead(err error) error {
	if err == nil {
		err = mcptransport.ErrTransportClosed
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.deadErr != nil {
		return s.deadErr
	}
	s.dead = true
	s.ready = false
	s.deadErr = fmt.Errorf("mcp %s: stdio process exited: %w", s.config.Name, err)
	s.initErr = s.deadErr
	return s.deadErr
}

func (s *mcpServer) deadError() error {
	s.mu.Lock()
	err := s.deadErr
	s.mu.Unlock()
	return err
}

func (s *mcpServer) initializeError() error {
	s.mu.Lock()
	err := s.initErr
	s.mu.Unlock()
	return err
}

func (s *mcpServer) promptCapable() bool {
	s.mu.Lock()
	capable := s.ready && s.caps.Prompts != nil
	s.mu.Unlock()
	return capable
}

func (s *mcpServer) initialize(ctx context.Context) error {
	// initMu gives concurrent first operations one handshake. Stdio failures
	// remain cached until the configuration is replaced: a dead local process
	// is fail-closed and is never silently restarted by an MCP operation.
	s.initMu.Lock()
	defer s.initMu.Unlock()
	if s.cli == nil {
		return s.cacheInitializeError(fmt.Errorf("mcp %s: no transport configured", s.config.Name))
	}
	s.mu.Lock()
	if s.ready || s.initErr != nil || s.dead {
		err := s.initErr
		s.mu.Unlock()
		return err
	}
	s.mu.Unlock()
	startCtx := ctx
	if s.processCtx != nil {
		startCtx = s.processCtx
	}
	if err := s.cli.Start(startCtx); err != nil {
		err = fmt.Errorf("mcp %s: start client: %w", s.config.Name, err)
		return s.cacheInitializeError(err)
	}
	result, err := s.cli.Initialize(ctx, mcp.InitializeRequest{Params: mcp.InitializeParams{
		ProtocolVersion: mcp.LATEST_PROTOCOL_VERSION,
		Capabilities:    mcp.ClientCapabilities{},
		ClientInfo:      mcp.Implementation{Name: "vivy", Version: "0.1"},
	}})
	if err != nil {
		err = fmt.Errorf("mcp %s: initialize: %w", s.config.Name, err)
		return s.cacheInitializeError(err)
	}
	s.mu.Lock()
	s.caps = result.Capabilities
	s.ready = true
	s.mu.Unlock()
	return nil
}

func (s *mcpServer) cacheInitializeError(err error) error {
	s.mu.Lock()
	s.initErr = err
	s.mu.Unlock()
	if s.processCancel != nil {
		// A failed stdio handshake otherwise leaves the child waiting on its
		// pipe forever even though this configuration is now fail-closed.
		// Close is idempotent and a replacement config gets a fresh client.
		_ = s.closeClient()
	}
	return err
}

func isSessionTerminated(err error) bool {
	return errors.Is(err, mcptransport.ErrSessionTerminated)
}

func isStdioTransportClosed(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, mcptransport.ErrTransportClosed) || errors.Is(err, os.ErrProcessDone) {
		return true
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "broken pipe") ||
		strings.Contains(message, "pipe closed") ||
		strings.Contains(message, "pipe is being closed") ||
		strings.Contains(message, "file already closed")
}

type hostMCPTool struct {
	name        string
	description string
	schema      json.RawMessage
}

func (b *MCPBackend) hostListTools(ctx context.Context, server string) ([]hostMCPTool, error) {
	b.discoveryMu.Lock()
	defer b.discoveryMu.Unlock()
	host, err := b.ensureMCPBridge()
	if err != nil {
		return nil, err
	}
	definitions, err := host.DiscoverTools(ctx, server)
	if err != nil {
		return nil, err
	}
	out := make([]hostMCPTool, 0, len(definitions))
	for _, definition := range definitions {
		out = append(out, hostMCPTool{name: definition.RemoteName, description: definition.Description, schema: append(json.RawMessage(nil), definition.Schema...)})
	}
	return out, nil
}

func (b *MCPBackend) hostCallTool(ctx context.Context, server, toolName string, arguments map[string]any) (tools.MCPCallResponse, error) {
	if arguments == nil {
		return b.hostCallToolRaw(ctx, server, toolName, nil)
	}
	raw, err := json.Marshal(arguments)
	if err != nil {
		return tools.MCPCallResponse{}, err
	}
	return b.hostCallToolRaw(ctx, server, toolName, raw)
}

func (b *MCPBackend) hostCallToolRaw(ctx context.Context, server, toolName string, arguments json.RawMessage) (tools.MCPCallResponse, error) {
	host, err := b.ensureMCPBridge()
	if err != nil {
		return tools.MCPCallResponse{}, err
	}
	result, err := host.CallRemoteTool(ctx, server, toolName, arguments)
	if err != nil {
		return tools.MCPCallResponse{}, err
	}
	var response tools.MCPCallResponse
	if err := json.Unmarshal([]byte(result.Text), &response); err != nil {
		return tools.MCPCallResponse{}, fmt.Errorf("mcp %s/%s: invalid projected result: %w", server, toolName, err)
	}
	return response, nil
}

func (b *MCPBackend) hostListResources(ctx context.Context, server string) ([]tools.MCPResource, error) {
	host, err := b.ensureMCPBridge()
	if err != nil {
		return nil, err
	}
	resources, err := host.ListResources(ctx, server)
	if err != nil {
		return nil, err
	}
	return projectHostResources(server, resources, maxMCPContentBytes), nil
}

func (b *MCPBackend) hostReadResource(ctx context.Context, server, uri string) (tools.MCPReadResourceResponse, error) {
	host, err := b.ensureMCPBridge()
	if err != nil {
		return tools.MCPReadResourceResponse{}, err
	}
	content, err := host.ReadResource(ctx, server, uri)
	if err != nil {
		return tools.MCPReadResourceResponse{}, err
	}
	result := tools.MCPReadResourceResponse{Server: server, URI: uri, Untrusted: true}
	if content.Text != "" {
		text := content.Text
		result.Contents = append(result.Contents, tools.MCPResourceContent{URI: content.URI, MIME: content.MediaType, Text: &text})
	}
	if len(content.Blob) > 0 {
		blob := string(content.Blob)
		result.Contents = append(result.Contents, tools.MCPResourceContent{URI: content.URI, MIME: content.MediaType, Blob: &blob})
	}
	return result, nil
}

func mapMCPError(err error) error {
	if err == nil || isSessionTerminated(err) {
		return err
	}
	var existing *tools.MCPRemoteError
	if errors.As(err, &existing) {
		return err
	}
	code := 0
	switch {
	case errors.Is(err, mcp.ErrParseError):
		code = mcp.PARSE_ERROR
	case errors.Is(err, mcp.ErrInvalidRequest):
		code = mcp.INVALID_REQUEST
	case errors.Is(err, mcp.ErrMethodNotFound):
		code = mcp.METHOD_NOT_FOUND
	case errors.Is(err, mcp.ErrInvalidParams):
		code = mcp.INVALID_PARAMS
	case errors.Is(err, mcp.ErrInternalError):
		code = mcp.INTERNAL_ERROR
	case errors.Is(err, mcp.ErrRequestInterrupted):
		code = mcp.REQUEST_INTERRUPTED
	case errors.Is(err, mcp.ErrResourceNotFound):
		code = mcp.RESOURCE_NOT_FOUND
	default:
		return err
	}
	return &tools.MCPRemoteError{Code: code, Message: err.Error()}
}

type boundedMCPClient struct{ client.MCPClient }

func (c boundedMCPClient) ListTools(ctx context.Context, request mcp.ListToolsRequest) (*mcp.ListToolsResult, error) {
	result, err := c.MCPClient.ListToolsByPage(ctx, request)
	if err != nil {
		return nil, err
	}
	if result == nil {
		return nil, errors.New("mcp: tools/list returned no result")
	}
	used := 0
	var truncated bool
	result.Tools, used, truncated, err = boundedMCPTools(result.Tools, used)
	if err != nil {
		return nil, err
	}
	if truncated {
		result.NextCursor = ""
		return result, nil
	}
	seen := map[mcp.Cursor]struct{}{}
	for page := 1; result.NextCursor != ""; page++ {
		if page >= maxMCPResourcePages {
			return nil, fmt.Errorf("mcp: tools/list exceeded %d pages", maxMCPResourcePages)
		}
		if len(result.NextCursor) > maxMCPContentBytes {
			return nil, fmt.Errorf("mcp: tools/list cursor exceeds size limit")
		}
		if _, ok := seen[result.NextCursor]; ok {
			return nil, fmt.Errorf("mcp: tools/list repeated cursor")
		}
		seen[result.NextCursor] = struct{}{}
		request.Params.Cursor = result.NextCursor
		next, err := c.MCPClient.ListToolsByPage(ctx, request)
		if err != nil {
			return nil, err
		}
		if next == nil {
			return nil, errors.New("mcp: tools/list returned no result")
		}
		var pageTools []mcp.Tool
		pageTools, used, truncated, err = boundedMCPTools(next.Tools, used)
		if err != nil {
			return nil, err
		}
		result.Tools = append(result.Tools, pageTools...)
		if truncated {
			result.NextCursor = ""
			break
		}
		result.NextCursor = next.NextCursor
	}
	return result, nil
}

// boundedMCPTools caps the aggregate catalog handed to Eino's GetTools. The
// transport body guard caps each response at 512 KiB, but 32 pages could
// otherwise accumulate 16 MiB before schema conversion.
func boundedMCPTools(items []mcp.Tool, used int) ([]mcp.Tool, int, bool, error) {
	bounded := make([]mcp.Tool, 0, len(items))
	for _, item := range items {
		raw, err := json.Marshal(item)
		if err != nil {
			return nil, used, false, fmt.Errorf("mcp: tools/list item: %w", err)
		}
		if len(raw) > maxMCPContentBytes {
			continue
		}
		if len(raw) > maxMCPContentBytes-used {
			return bounded, used, true, nil
		}
		bounded = append(bounded, item)
		used += len(raw)
	}
	return bounded, used, false, nil
}

func listResourcePages(ctx context.Context, cli client.MCPClient, server string, limit int) ([]tools.MCPResource, error) {
	resources := make([]tools.MCPResource, 0)
	used := 0
	var cursor mcp.Cursor
	seen := make(map[mcp.Cursor]struct{})
	for page := 0; page < maxMCPResourcePages; page++ {
		result, err := cli.ListResourcesByPage(ctx, mcp.ListResourcesRequest{PaginatedRequest: mcp.PaginatedRequest{Params: mcp.PaginatedParams{Cursor: cursor}}})
		if err != nil {
			return nil, err
		}
		if result == nil {
			return nil, fmt.Errorf("mcp %s: resources/list returned no result", server)
		}
		for _, item := range result.Resources {
			if strings.TrimSpace(item.URI) == "" {
				continue
			}
			if len(server)+len(item.URI) > maxMCPContentBytes {
				return nil, fmt.Errorf("mcp %s: resource URI exceeds size limit", server)
			}
			if len(server)+len(item.URI) > limit-used {
				return resources, nil
			}
			annotations := optionalJSON(item.Annotations)
			meta := optionalJSON(item.Meta)
			size := int64(0)
			if item.Size != nil {
				size = *item.Size
			}
			resource, consumed := boundedMCPResource(tools.MCPResource{
				Server: server, URI: item.URI, Name: item.Name, Title: item.Title,
				Description: item.Description, MIME: item.MIMEType, Size: size,
				Annotations: annotations, Meta: meta,
			}, limit-used)
			if consumed == 0 {
				return resources, nil
			}
			resources = append(resources, resource)
			used += consumed
			if used >= limit {
				return resources, nil
			}
		}
		if result.NextCursor == "" {
			return resources, nil
		}
		if len(result.NextCursor) > maxMCPContentBytes {
			return nil, fmt.Errorf("mcp %s: resources/list cursor exceeds size limit", server)
		}
		if _, ok := seen[result.NextCursor]; ok {
			return nil, fmt.Errorf("mcp %s: resources/list repeated cursor", server)
		}
		seen[result.NextCursor] = struct{}{}
		cursor = result.NextCursor
	}
	return nil, fmt.Errorf("mcp %s: resources/list exceeded %d pages", server, maxMCPResourcePages)
}

func listPromptsPages(ctx context.Context, cli client.MCPClient, server string) ([]tools.MCPPrompt, error) {
	out := make([]tools.MCPPrompt, 0)
	used := 0
	var cursor mcp.Cursor
	seen := make(map[mcp.Cursor]struct{})
	for page := 0; page < maxMCPResourcePages; page++ {
		result, err := cli.ListPromptsByPage(ctx, mcp.ListPromptsRequest{PaginatedRequest: mcp.PaginatedRequest{Params: mcp.PaginatedParams{Cursor: cursor}}})
		if err != nil {
			return nil, err
		}
		if result == nil {
			return nil, fmt.Errorf("mcp %s: prompts/list returned no result", server)
		}
		for _, item := range result.Prompts {
			name := strings.TrimSpace(item.Name)
			if name == "" || len(name) > 256 {
				continue
			}
			prompt := tools.MCPPrompt{Server: server, Name: name, Title: boundedString(item.Title, 512), Description: boundedString(item.Description, 4096)}
			valid := true
			seenArguments := make(map[string]struct{})
			for _, argument := range item.Arguments {
				argumentName := strings.TrimSpace(argument.Name)
				if argumentName == "" || len(argumentName) > 128 || len(prompt.Arguments) == 64 {
					valid = false
					break
				}
				if _, duplicate := seenArguments[argumentName]; duplicate {
					valid = false
					break
				}
				seenArguments[argumentName] = struct{}{}
				prompt.Arguments = append(prompt.Arguments, tools.MCPPromptArgument{Name: argumentName, Title: boundedString(argument.Title, 256), Description: boundedString(argument.Description, 1024), Required: argument.Required})
			}
			if !valid {
				continue
			}
			size := mcpPromptProjectedSize(prompt)
			if size > maxMCPContentBytes-used {
				return out, nil
			}
			out = append(out, prompt)
			used += size
		}
		if result.NextCursor == "" {
			return out, nil
		}
		if len(result.NextCursor) > maxMCPContentBytes {
			return nil, fmt.Errorf("mcp %s: prompts/list cursor exceeds size limit", server)
		}
		if _, ok := seen[result.NextCursor]; ok {
			return nil, fmt.Errorf("mcp %s: prompts/list repeated cursor", server)
		}
		seen[result.NextCursor] = struct{}{}
		cursor = result.NextCursor
	}
	return nil, fmt.Errorf("mcp %s: prompts/list exceeded %d pages", server, maxMCPResourcePages)
}

func projectHostResources(server string, resources []mcphost.RemoteResource, limit int) []tools.MCPResource {
	if limit <= 0 {
		return nil
	}
	out := make([]tools.MCPResource, 0, len(resources))
	used := 0
	for _, resource := range resources {
		projected := tools.MCPResource{
			Server: server, URI: resource.URI, Name: resource.Name, Title: resource.Title,
			Description: resource.Description, MIME: resource.MediaType, Size: resource.Size,
			Annotations: append(json.RawMessage(nil), resource.Annotations...), Meta: append(json.RawMessage(nil), resource.Meta...),
		}
		size := mcpResourceProjectedSize(projected)
		if size > limit-used {
			break
		}
		out = append(out, projected)
		used += size
	}
	return out
}

func projectPrompt(server, name string, payload *mcp.GetPromptResult) (tools.MCPGetPromptResponse, error) {
	if payload == nil {
		return tools.MCPGetPromptResponse{}, fmt.Errorf("mcp %s: prompts/get returned no result", server)
	}
	texts := make([]string, 0, len(payload.Messages))
	used := 0
	for _, message := range payload.Messages {
		if message.Role != mcp.RoleUser {
			continue
		}
		content, ok := mcp.AsTextContent(message.Content)
		if !ok || strings.TrimSpace(content.Text) == "" {
			continue
		}
		addition := len(content.Text)
		if len(texts) > 0 {
			addition++
		}
		if addition > maxMCPContentBytes-used {
			return tools.MCPGetPromptResponse{}, fmt.Errorf("mcp %s: prompt content exceeds size limit", server)
		}
		texts = append(texts, content.Text)
		used += addition
	}
	text := strings.Join(texts, " ")
	if strings.TrimSpace(text) == "" {
		return tools.MCPGetPromptResponse{}, fmt.Errorf("mcp %s: prompt returned no user text", server)
	}
	return tools.MCPGetPromptResponse{Server: server, Name: name, Description: boundedString(payload.Description, 4096), Text: text, Untrusted: true}, nil
}

func projectReadResource(server, requestURI string, payload *mcp.ReadResourceResult) (tools.MCPReadResourceResponse, error) {
	if payload == nil {
		return tools.MCPReadResourceResponse{}, fmt.Errorf("mcp %s/%s: resources/read returned no result", server, requestURI)
	}
	contents := make([]tools.MCPResourceContent, 0, len(payload.Contents))
	used := 0
	for _, item := range payload.Contents {
		content := tools.MCPResourceContent{}
		switch value := item.(type) {
		case mcp.TextResourceContents:
			content.URI, content.MIME, content.Text = value.URI, value.MIMEType, &value.Text
			content.Meta = optionalJSON(value.Meta)
		case mcp.BlobResourceContents:
			content.URI, content.MIME, content.Blob = value.URI, value.MIMEType, &value.Blob
			content.Meta = optionalJSON(value.Meta)
		default:
			return tools.MCPReadResourceResponse{}, fmt.Errorf("mcp %s/%s: resource content has unsupported type", server, requestURI)
		}
		if strings.TrimSpace(content.URI) == "" {
			return tools.MCPReadResourceResponse{}, fmt.Errorf("mcp %s/%s: resource content URI is required", server, requestURI)
		}
		if len(content.URI)+len(content.MIME) > maxMCPContentBytes {
			return tools.MCPReadResourceResponse{}, fmt.Errorf("mcp %s/%s: resource content identity exceeds size limit", server, requestURI)
		}
		if len(content.URI)+len(content.MIME) > maxMCPContentBytes-used {
			break
		}
		bounded, consumed := boundedMCPResourceContent(content, maxMCPContentBytes-used)
		if consumed == 0 {
			break
		}
		contents = append(contents, bounded)
		used += consumed
		if used >= maxMCPContentBytes {
			break
		}
	}
	return tools.MCPReadResourceResponse{Server: server, URI: requestURI, Contents: contents, Untrusted: true}, nil
}

func projectCallResult(server, toolName string, payload *mcp.CallToolResult) (tools.MCPCallResponse, error) {
	if payload == nil {
		return tools.MCPCallResponse{}, fmt.Errorf("mcp %s/%s: tools/call returned no result", server, toolName)
	}
	content := make([]tools.MCPContent, 0, len(payload.Content))
	used := 0
	for _, item := range payload.Content {
		projected, ok := projectCallContent(item)
		if !ok {
			continue
		}
		projected.Type = boundedString(projected.Type, maxMCPContentBytes-used)
		projected.MIME = boundedString(projected.MIME, maxMCPContentBytes-used-len(projected.Type))
		remaining := maxMCPContentBytes - used - len(projected.Type) - len(projected.MIME)
		if remaining <= 0 {
			break
		}
		projected.Text = boundedString(projected.Text, remaining)
		remaining -= len(projected.Text)
		projected.Data = boundedString(projected.Data, remaining)
		used += len(projected.Type) + len(projected.MIME) + len(projected.Text) + len(projected.Data)
		if projected.Text == "" && projected.Data == "" && projected.Type != "resource" {
			continue
		}
		content = append(content, projected)
		if used >= maxMCPContentBytes {
			break
		}
	}
	return tools.MCPCallResponse{Server: server, Tool: toolName, Content: content, IsError: payload.IsError, Untrusted: true}, nil
}

func projectCallContent(content mcp.Content) (tools.MCPContent, bool) {
	switch value := content.(type) {
	case mcp.TextContent:
		return tools.MCPContent{Type: value.Type, Text: value.Text}, true
	case mcp.ImageContent:
		return tools.MCPContent{Type: value.Type, Data: value.Data, MIME: value.MIMEType}, true
	case mcp.AudioContent:
		return tools.MCPContent{Type: value.Type, Data: value.Data, MIME: value.MIMEType}, true
	case mcp.ResourceLink:
		return tools.MCPContent{Type: value.Type, Data: value.URI, MIME: value.MIMEType}, true
	case mcp.EmbeddedResource:
		switch nested := value.Resource.(type) {
		case mcp.TextResourceContents:
			return tools.MCPContent{Type: value.Type, Text: nested.Text, MIME: nested.MIMEType}, true
		case mcp.BlobResourceContents:
			return tools.MCPContent{Type: value.Type, Data: nested.Blob, MIME: nested.MIMEType}, true
		}
	}
	return tools.MCPContent{}, false
}

func optionalJSON(value any) json.RawMessage {
	if value == nil {
		return nil
	}
	raw, err := json.Marshal(value)
	if err != nil || string(raw) == "null" {
		return nil
	}
	return raw
}

func boundedMCPHTTPClient(source *http.Client, limit func() int) *http.Client {
	clone := *source
	base := source.Transport
	if base == nil {
		base = http.DefaultTransport
	}
	clone.Transport = boundedMCPTransport{base: base, limit: limit}
	if clone.Timeout == 0 {
		clone.Timeout = defaultMCPTimeout
	}
	return &clone
}

type boundedMCPTransport struct {
	base  http.RoundTripper
	limit func() int
}

func (t boundedMCPTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	response, err := t.base.RoundTrip(request)
	if err != nil || response == nil || response.Body == nil {
		return response, err
	}
	limit := maxMCPResponseBytes
	if t.limit != nil && t.limit() > 0 {
		limit = t.limit()
	}
	if response.ContentLength > int64(limit) {
		_ = response.Body.Close()
		return nil, fmt.Errorf("mcp: response exceeds size limit (%d bytes)", limit)
	}
	response.Body = &boundedMCPBody{ReadCloser: response.Body, limit: int64(limit)}
	return response, nil
}

type boundedMCPBody struct {
	io.ReadCloser
	limit   int64
	read    int64
	checked bool
}

func (b *boundedMCPBody) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	if b.read < b.limit {
		remaining := b.limit - b.read
		if int64(len(p)) > remaining {
			p = p[:remaining]
		}
		n, err := b.ReadCloser.Read(p)
		b.read += int64(n)
		return n, err
	}
	if b.checked {
		return 0, io.EOF
	}
	var one [1]byte
	n, err := b.ReadCloser.Read(one[:])
	if n > 0 {
		b.checked = true
		return 0, fmt.Errorf("mcp: response exceeds size limit")
	}
	if err != nil {
		b.checked = true
	}
	return 0, err
}

func boundedRaw(value json.RawMessage, limit int) json.RawMessage {
	if limit <= 0 || len(value) == 0 {
		return nil
	}
	if len(value) <= limit {
		return append(json.RawMessage(nil), value...)
	}
	return json.RawMessage("null")
}

func boundedString(value string, limit int) string {
	if limit <= 0 {
		return ""
	}
	if len(value) <= limit {
		return value
	}
	return value[:limit]
}

func boundedMCPResource(resource tools.MCPResource, limit int) (tools.MCPResource, int) {
	if limit <= 0 {
		return tools.MCPResource{}, 0
	}
	out := tools.MCPResource{Size: resource.Size}
	used := 0
	takeString := func(value string) string {
		part := boundedString(value, limit-used)
		used += len(part)
		return part
	}
	takeRaw := func(value json.RawMessage) json.RawMessage {
		part := boundedRaw(value, limit-used)
		used += len(part)
		return part
	}
	out.Server = takeString(resource.Server)
	out.URI = takeString(resource.URI)
	out.Name = takeString(resource.Name)
	out.Title = takeString(resource.Title)
	out.MIME = takeString(resource.MIME)
	out.Description = takeString(resource.Description)
	out.Annotations = takeRaw(resource.Annotations)
	out.Meta = takeRaw(resource.Meta)
	return out, used
}

func boundedMCPResourceContent(content tools.MCPResourceContent, limit int) (tools.MCPResourceContent, int) {
	if limit <= 0 {
		return tools.MCPResourceContent{}, 0
	}
	out := tools.MCPResourceContent{}
	used := 0
	takeString := func(value string) string {
		part := boundedString(value, limit-used)
		used += len(part)
		return part
	}
	takeRaw := func(value json.RawMessage) json.RawMessage {
		part := boundedRaw(value, limit-used)
		used += len(part)
		return part
	}
	out.URI = takeString(content.URI)
	out.MIME = takeString(content.MIME)
	if content.Text != nil {
		value := takeString(*content.Text)
		out.Text = &value
	}
	if content.Blob != nil {
		value := takeString(*content.Blob)
		out.Blob = &value
	}
	out.Annotations = takeRaw(content.Annotations)
	out.Meta = takeRaw(content.Meta)
	return out, used
}

func mcpResourceProjectedSize(resource tools.MCPResource) int {
	return len(resource.Server) + len(resource.URI) + len(resource.Name) + len(resource.Title) + len(resource.Description) + len(resource.MIME) + len(resource.Annotations) + len(resource.Meta)
}

func mcpPromptProjectedSize(prompt tools.MCPPrompt) int {
	size := len(prompt.Server) + len(prompt.Name) + len(prompt.Title) + len(prompt.Description)
	for _, argument := range prompt.Arguments {
		size += len(argument.Name) + len(argument.Title) + len(argument.Description) + 1
	}
	return size
}
