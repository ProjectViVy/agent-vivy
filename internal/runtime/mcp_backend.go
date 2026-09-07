package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/tools"

	einomcp "github.com/cloudwego/eino-ext/components/tool/mcp"
	einotool "github.com/cloudwego/eino/components/tool"
	"github.com/mark3labs/mcp-go/client"
	mcptransport "github.com/mark3labs/mcp-go/client/transport"
	"github.com/mark3labs/mcp-go/mcp"
)

const (
	defaultMCPTimeout   = 8 * time.Second
	maxMCPResponseBytes = 512 << 10
	maxMCPContentBytes  = 256 << 10
	maxMCPResourcePages = 32
)

type MCPServerConfig struct {
	Name     string
	Endpoint string
	AuthEnv  string
}

// MCPBackend is the Vivy governance adapter around one official MCP
// client per configured server. Eino's MCP component is intentionally used
// only for tool discovery/schema conversion; the resulting Eino tools are
// projected into Vivy's untrusted catalog and are never mounted into the
// model. Calls continue through mcp_call and PrepareMCPCall.
type MCPBackend struct {
	client           *http.Client
	servers          map[string]*mcpServer
	mu               sync.RWMutex
	retired          sync.WaitGroup
	closeDone        chan struct{}
	closeErr         error
	maxResponseBytes int
	timeout          time.Duration
	closed           bool
}

// mcpStatusRecord belongs to one logical server configuration. A session
// replacement keeps this record, while a settings replacement receives a new
// one, so delayed work from a retired configuration cannot contaminate the
// status of a same-named replacement.
type mcpStatusRecord struct {
	mu        sync.Mutex
	initError string
	toolCount int
}

type mcpServer struct {
	config MCPServerConfig
	cli    *client.Client
	status *mcpStatusRecord

	initMu  sync.Mutex
	mu      sync.Mutex
	initErr error
	caps    mcp.ServerCapabilities
	ready   bool

	refs    int
	closing bool
	closed  bool

	closeOnce sync.Once
	closeDone chan struct{}
	closeErr  error
}

// MCPServerStatus is a secret-free snapshot of one configured server's
// process state. Initialized means the backend completed the MCP handshake;
// configured servers are not reported as connected before that happens.
// Error is the sanitized last handshake failure for this configuration (empty
// after a success); AuthMissing flags a configured auth env var that is
// currently unset; ToolCount is the last successful catalog size, negative
// when the server was never listed.
type MCPServerStatus struct {
	Name        string
	Initialized bool
	Error       string
	AuthMissing bool
	ToolCount   int
}

var _ tools.MCPOperations = (*MCPBackend)(nil)
var _ tools.MCPResourceOperations = (*MCPBackend)(nil)
var _ tools.MCPPromptOperations = (*MCPBackend)(nil)
var _ interface {
	PrepareMCPCall(context.Context, domain.RunID, tools.MCPCallRequest) (domain.ToolProposal, error)
} = (*MCPBackend)(nil)

// NewMCPBackend builds a backend without opening any network connection.
// Each server is started and initialized lazily on its first operation.
func NewMCPBackend(configs []MCPServerConfig, httpClient *http.Client) *MCPBackend {
	backend := &MCPBackend{
		maxResponseBytes: maxMCPResponseBytes,
		timeout:          defaultMCPTimeout,
		closeDone:        make(chan struct{}),
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: defaultMCPTimeout}
	}
	backend.client = boundedMCPHTTPClient(httpClient, func() int { return backend.maxResponseBytes })
	backend.ReplaceServers(configs)
	return backend
}

// ReplaceServers swaps the live catalog. Entries whose endpoint/auth config
// is unchanged are retained; removed or replaced entries are closed after any
// in-flight operation releases its reference.
func (b *MCPBackend) ReplaceServers(configs []MCPServerConfig) {
	normalized := make(map[string]MCPServerConfig, len(configs))
	for _, config := range configs {
		config.Name = strings.TrimSpace(config.Name)
		config.Endpoint = strings.TrimSpace(config.Endpoint)
		config.AuthEnv = strings.TrimSpace(config.AuthEnv)
		if config.Name == "" || config.Endpoint == "" {
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
		if previous := old[name]; previous != nil && previous.config == config {
			next[name] = previous
			continue
		}
		entry, err := b.newServer(config)
		if err != nil {
			// Config validation normally catches malformed endpoints. Keep the
			// runtime fail-closed if a direct caller bypasses that boundary.
			continue
		}
		next[name] = entry
	}
	removed := make([]*mcpServer, 0)
	for name, previous := range old {
		if next[name] != previous {
			removed = append(removed, previous)
		}
	}
	// Add while holding the backend lock and before publishing the replacement.
	// Close takes the same lock before waiting, so it cannot race a late Add.
	b.retired.Add(len(removed))
	b.servers = next
	b.mu.Unlock()

	b.startRetiredCleanup(removed)
}

// ConfiguredServers returns a deterministic snapshot of the live catalog.
func (b *MCPBackend) ConfiguredServers() []MCPServerConfig {
	b.mu.RLock()
	out := make([]MCPServerConfig, 0, len(b.servers))
	for _, entry := range b.servers {
		out = append(out, entry.config)
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
			Name:        name,
			Initialized: entry.isReady(),
			Error:       entry.status.initError,
			ToolCount:   entry.status.toolCount,
		}
		entry.status.mu.Unlock()
		if entry.config.AuthEnv != "" && os.Getenv(entry.config.AuthEnv) == "" {
			status.AuthMissing = true
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
	if b.closed || b.servers[entry.config.Name] != entry {
		return
	}
	entry.status.mu.Lock()
	defer entry.status.mu.Unlock()
	if err == nil {
		entry.status.initError = ""
		return
	}
	entry.status.initError = boundedMCPStatusError(err.Error(), entry.config.Endpoint)
}

// noteToolCount records the projected catalog size after a successful listing.
// It is called with the concrete entry held by runMCP, not only its name.
func (b *MCPBackend) noteToolCount(entry *mcpServer, count int) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if b.closed || b.servers[entry.config.Name] != entry {
		return
	}
	entry.status.mu.Lock()
	entry.status.toolCount = count
	entry.status.mu.Unlock()
}

// boundedMCPStatusError keeps the sidebar secret-free: the server's endpoint
// (which may embed credentials) never appears, control characters are
// stripped, and the message is capped.
func boundedMCPStatusError(message, endpoint string) string {
	message = strings.ReplaceAll(message, endpoint, "")
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

// Close shuts down every live MCP client. It is idempotent and safe to call
// while a request is in flight; the final client close is deferred until its
// operation reference is released.
func (b *MCPBackend) Close() error {
	b.mu.Lock()
	if b.closed {
		done := b.closeDone
		b.mu.Unlock()
		<-done
		b.mu.RLock()
		err := b.closeErr
		b.mu.RUnlock()
		return err
	}
	b.closed = true
	old := b.servers
	b.servers = make(map[string]*mcpServer)
	b.mu.Unlock()

	err := closeMCPServers(mapValues(old))
	// ReplaceServers may already have retired clients whose asynchronous close
	// is still in flight. Wait for those too before declaring shutdown complete.
	b.retired.Wait()
	b.mu.Lock()
	b.closeErr = err
	close(b.closeDone)
	b.mu.Unlock()
	return err
}

func mapValues(values map[string]*mcpServer) []*mcpServer {
	entries := make([]*mcpServer, 0, len(values))
	for _, entry := range values {
		entries = append(entries, entry)
	}
	return entries
}

// closeMCPServers closes transports concurrently. Streamable HTTP's legacy
// Close can wait up to five seconds for its DELETE; serial shutdown would turn
// N configured servers into an N*5s settings/shutdown stall.
func closeMCPServers(entries []*mcpServer) error {
	if len(entries) == 0 {
		return nil
	}
	errs := make(chan error, len(entries))
	var wg sync.WaitGroup
	for _, entry := range entries {
		if entry == nil {
			continue
		}
		wg.Add(1)
		go func(entry *mcpServer) {
			defer wg.Done()
			if err := entry.markClosing(); err != nil {
				errs <- err
				return
			}
			if err := entry.waitClosed(); err != nil {
				errs <- err
			}
		}(entry)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		return err
	}
	return nil
}

func (b *MCPBackend) startRetiredCleanup(entries []*mcpServer) {
	if len(entries) == 0 {
		return
	}
	for _, entry := range entries {
		if entry == nil {
			b.retired.Done()
			continue
		}
		go func(entry *mcpServer) {
			defer b.retired.Done()
			_ = closeMCPServers([]*mcpServer{entry})
		}(entry)
	}
}

func (b *MCPBackend) ListTools(ctx context.Context, _ domain.RunID, server string) (tools.MCPListResponse, error) {
	ctx, cancel := b.operationContext(ctx)
	defer cancel()
	server = strings.TrimSpace(server)
	if server != "" {
		result, err := runMCP(ctx, b, server, true, func(ctx context.Context, entry *mcpServer) ([]tools.MCPTool, error) {
			mcpTools, err := einomcp.GetTools(ctx, &einomcp.Config{Cli: boundedMCPClient{MCPClient: entry.cli}})
			if err != nil {
				return nil, err
			}
			result, err := projectEinoTools(ctx, mcpTools, server)
			if err == nil {
				b.noteToolCount(entry, len(result))
			}
			return result, err
		})
		if err != nil {
			return tools.MCPListResponse{}, err
		}
		return tools.MCPListResponse{Tools: result, Untrusted: true}, nil
	}

	all := make([]tools.MCPTool, 0)
	used := 0
	for _, name := range b.serverNames() {
		result, err := runMCP(ctx, b, name, true, func(ctx context.Context, entry *mcpServer) ([]tools.MCPTool, error) {
			mcpTools, err := einomcp.GetTools(ctx, &einomcp.Config{Cli: boundedMCPClient{MCPClient: entry.cli}})
			if err != nil {
				return nil, err
			}
			result, err := projectEinoTools(ctx, mcpTools, name)
			if err == nil {
				b.noteToolCount(entry, len(result))
			}
			return result, err
		})
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

func projectEinoTools(ctx context.Context, mcpTools []einotool.BaseTool, server string) ([]tools.MCPTool, error) {
	out := make([]tools.MCPTool, 0, len(mcpTools))
	used := 0
	for _, remote := range mcpTools {
		if remote == nil {
			continue
		}
		info, err := remote.Info(ctx)
		if err != nil {
			return nil, fmt.Errorf("mcp %s: tool info: %w", server, err)
		}
		if info == nil || strings.TrimSpace(info.Name) == "" || tools.IsBrowserUseName(info.Name) {
			continue
		}
		var inputSchema json.RawMessage
		if info.ParamsOneOf != nil {
			schema, err := info.ParamsOneOf.ToJSONSchema()
			if err != nil {
				return nil, fmt.Errorf("mcp %s/%s: tool schema: %w", server, info.Name, err)
			}
			inputSchema, err = json.Marshal(schema)
			if err != nil {
				return nil, fmt.Errorf("mcp %s/%s: marshal tool schema: %w", server, info.Name, err)
			}
			inputSchema = boundedRaw(inputSchema, maxMCPContentBytes)
		}
		projected := tools.MCPTool{Server: server, Name: info.Name, Description: info.Desc, InputSchema: inputSchema}
		size := mcpToolProjectedSize(projected)
		if size > maxMCPContentBytes-used {
			break
		}
		out = append(out, projected)
		used += size
	}
	return out, nil
}

func mcpToolProjectedSize(tool tools.MCPTool) int {
	return len(tool.Server) + len(tool.Name) + len(tool.Description) + len(tool.InputSchema)
}

// ListResources performs read-only resources/list. An empty server fans out
// in deterministic catalog order and applies one shared content budget.
func (b *MCPBackend) ListResources(ctx context.Context, _ domain.RunID, server string) (tools.MCPListResourcesResponse, error) {
	ctx, cancel := b.operationContext(ctx)
	defer cancel()
	server = strings.TrimSpace(server)
	if server != "" {
		resources, err := b.listResourcesServer(ctx, server, maxMCPContentBytes)
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
		resources, err := b.listResourcesServer(ctx, name, maxMCPContentBytes-used)
		if err != nil {
			return tools.MCPListResourcesResponse{}, err
		}
		all = append(all, resources...)
		for _, resource := range resources {
			used += mcpResourceProjectedSize(resource)
		}
	}
	return tools.MCPListResourcesResponse{Resources: all, Untrusted: true}, nil
}

// ListPrompts returns bounded, untrusted prompt metadata. Servers without the
// prompts capability fail closed to an empty catalog.
func (b *MCPBackend) ListPrompts(ctx context.Context, _ domain.RunID, server string) (tools.MCPListPromptsResponse, error) {
	ctx, cancel := b.operationContext(ctx)
	defer cancel()
	server = strings.TrimSpace(server)
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
	return runMCP(ctx, b, server, true, func(ctx context.Context, entry *mcpServer) ([]tools.MCPPrompt, error) {
		if !entry.promptCapable() {
			return []tools.MCPPrompt{}, nil
		}
		return listPromptsPages(ctx, entry.cli, server)
	})
}

// GetPrompt fetches one prompt and flattens user text into bounded, untrusted
// model input. Non-text content is rejected rather than guessed.
func (b *MCPBackend) GetPrompt(ctx context.Context, _ domain.RunID, request tools.MCPGetPromptRequest) (tools.MCPGetPromptResponse, error) {
	ctx, cancel := b.operationContext(ctx)
	defer cancel()
	request.Server = strings.TrimSpace(request.Server)
	if strings.TrimSpace(request.Name) == "" {
		return tools.MCPGetPromptResponse{}, errors.New("mcp: prompt name is required")
	}
	result, err := runMCP(ctx, b, request.Server, true, func(ctx context.Context, entry *mcpServer) (tools.MCPGetPromptResponse, error) {
		payload, err := entry.cli.GetPrompt(ctx, mcp.GetPromptRequest{Params: mcp.GetPromptParams{Name: request.Name, Arguments: request.Arguments}})
		if err != nil {
			return tools.MCPGetPromptResponse{}, err
		}
		return projectPrompt(request.Server, request.Name, payload)
	})
	if err != nil {
		return tools.MCPGetPromptResponse{}, err
	}
	return result, nil
}

func (b *MCPBackend) listResourcesServer(ctx context.Context, server string, limit int) ([]tools.MCPResource, error) {
	return runMCP(ctx, b, server, true, func(ctx context.Context, entry *mcpServer) ([]tools.MCPResource, error) {
		return listResourcePages(ctx, entry.cli, server, limit)
	})
}

// ReadResource performs resources/read. The URI is returned verbatim and
// content remains untrusted text/base64; it is never mounted or written.
func (b *MCPBackend) ReadResource(ctx context.Context, _ domain.RunID, request tools.MCPReadResourceRequest) (tools.MCPReadResourceResponse, error) {
	ctx, cancel := b.operationContext(ctx)
	defer cancel()
	request.Server = strings.TrimSpace(request.Server)
	if strings.TrimSpace(request.URI) == "" {
		return tools.MCPReadResourceResponse{}, errors.New("mcp: resource URI is required")
	}
	if len(request.URI) > maxMCPContentBytes {
		return tools.MCPReadResourceResponse{}, fmt.Errorf("mcp: resource URI exceeds size limit (%d bytes)", maxMCPContentBytes)
	}
	result, err := runMCP(ctx, b, request.Server, true, func(ctx context.Context, entry *mcpServer) (tools.MCPReadResourceResponse, error) {
		payload, err := entry.cli.ReadResource(ctx, mcp.ReadResourceRequest{Params: mcp.ReadResourceParams{URI: request.URI}})
		if err != nil {
			return tools.MCPReadResourceResponse{}, err
		}
		return projectReadResource(request.Server, request.URI, payload)
	})
	if err != nil {
		return tools.MCPReadResourceResponse{}, err
	}
	return result, nil
}

// CallTool deliberately has no session-retry. A remote side effect may have
// happened before a transport error was observed. IsError is preserved in
// Vivy's response instead of being converted to a Go error.
func (b *MCPBackend) CallTool(ctx context.Context, _ domain.RunID, request tools.MCPCallRequest) (tools.MCPCallResponse, error) {
	ctx, cancel := b.operationContext(ctx)
	defer cancel()
	request.Server = strings.TrimSpace(request.Server)
	request.Tool = strings.TrimSpace(request.Tool)
	result, err := runMCP(ctx, b, request.Server, false, func(ctx context.Context, entry *mcpServer) (tools.MCPCallResponse, error) {
		payload, err := entry.cli.CallTool(ctx, mcp.CallToolRequest{Params: mcp.CallToolParams{Name: request.Tool, Arguments: request.Arguments}})
		if err != nil {
			return tools.MCPCallResponse{}, err
		}
		return projectCallResult(request.Server, request.Tool, payload)
	})
	if err != nil {
		return tools.MCPCallResponse{}, err
	}
	return result, nil
}

func (b *MCPBackend) PrepareMCPCall(_ context.Context, _ domain.RunID, request tools.MCPCallRequest) (domain.ToolProposal, error) {
	request.Server = strings.TrimSpace(request.Server)
	request.Tool = strings.TrimSpace(request.Tool)
	if _, err := b.lookupServer(request.Server); err != nil {
		return domain.ToolProposal{}, err
	}
	payload, _ := json.Marshal(request)
	preview := string(payload)
	if len(preview) > 4096 {
		preview = preview[:4096] + "..."
	}
	return domain.ToolProposal{Action: tools.MCPCallName, Target: request.Server + "/" + request.Tool, Preview: preview, RiskFindings: []string{"remote MCP side effect is unknown"}, Data: payload}, nil
}

func (b *MCPBackend) newServer(config MCPServerConfig) (*mcpServer, error) {
	cli, err := client.NewStreamableHttpClient(config.Endpoint,
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
	if err != nil {
		return nil, fmt.Errorf("mcp %s: create streamable HTTP client: %w", config.Name, err)
	}
	return &mcpServer{
		config:    config,
		cli:       cli,
		status:    &mcpStatusRecord{toolCount: -1},
		closeDone: make(chan struct{}),
	}, nil
}

func (b *MCPBackend) lookupServer(name string) (*mcpServer, error) {
	name = strings.TrimSpace(name)
	b.mu.RLock()
	entry := b.servers[name]
	closed := b.closed
	b.mu.RUnlock()
	if closed {
		return nil, errors.New("mcp: backend is closed")
	}
	if entry == nil || strings.TrimSpace(entry.config.Endpoint) == "" {
		return nil, fmt.Errorf("mcp: server %q is not configured", name)
	}
	return entry, nil
}

func (b *MCPBackend) acquireServer(name string) (*mcpServer, error) {
	name = strings.TrimSpace(name)
	b.mu.RLock()
	defer b.mu.RUnlock()
	if b.closed {
		return nil, errors.New("mcp: backend is closed")
	}
	entry := b.servers[name]
	if entry == nil || strings.TrimSpace(entry.config.Endpoint) == "" {
		return nil, fmt.Errorf("mcp: server %q is not configured", name)
	}
	if !entry.acquire() {
		return nil, fmt.Errorf("mcp: server %q is being replaced", name)
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

// replaceSession creates a fresh official client after a server reports that
// its stateful HTTP session was terminated. It never re-adds a server removed
// by a concurrent settings update.
func (b *MCPBackend) replaceSession(name string, expected *mcpServer) {
	b.mu.RLock()
	current := b.servers[name]
	closed := b.closed
	b.mu.RUnlock()
	if closed || current != expected {
		return
	}
	entry, err := b.newServer(expected.config)
	if err != nil {
		return
	}
	entry.status = expected.status
	b.mu.Lock()
	if b.closed || b.servers[name] != expected {
		b.mu.Unlock()
		_ = entry.markClosing()
		return
	}
	b.retired.Add(1)
	b.servers[name] = entry
	b.mu.Unlock()
	b.startRetiredCleanup([]*mcpServer{expected})
}

func (s *mcpServer) acquire() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closing || s.closed {
		return false
	}
	s.refs++
	return true
}

func (s *mcpServer) release() {
	s.mu.Lock()
	if s.refs > 0 {
		s.refs--
	}
	closeNow := s.refs == 0 && s.closing && !s.closed
	if closeNow {
		s.closed = true
	}
	s.mu.Unlock()
	if closeNow {
		// The retired/current cleanup worker is already waiting on closeDone;
		// complete the close here so no lifecycle goroutine escapes tracking.
		_ = s.closeClient()
	}
}

func (s *mcpServer) markClosing() error {
	s.mu.Lock()
	s.closing = true
	closeNow := s.refs == 0 && !s.closed
	if closeNow {
		s.closed = true
	}
	s.mu.Unlock()
	if closeNow {
		return s.closeClient()
	}
	return nil
}

func (s *mcpServer) waitClosed() error {
	<-s.closeDone
	s.mu.Lock()
	err := s.closeErr
	s.mu.Unlock()
	return err
}

func (s *mcpServer) closeClient() error {
	s.closeOnce.Do(func() {
		err := s.cli.Close()
		s.mu.Lock()
		s.closeErr = err
		s.mu.Unlock()
		close(s.closeDone)
	})
	return s.waitClosed()
}

func (s *mcpServer) isReady() bool {
	s.mu.Lock()
	ready := s.ready
	s.mu.Unlock()
	return ready
}

func (s *mcpServer) promptCapable() bool {
	s.mu.Lock()
	capable := s.ready && s.caps.Prompts != nil
	s.mu.Unlock()
	return capable
}

func (s *mcpServer) initialize(ctx context.Context) error {
	// initMu gives concurrent first operations one handshake. A failed
	// handshake is cached on this client only; runMCP replaces and closes this
	// client, so a later operation gets a fresh initialization attempt instead
	// of permanently poisoning the configured server.
	s.initMu.Lock()
	defer s.initMu.Unlock()
	s.mu.Lock()
	if s.ready || s.initErr != nil {
		err := s.initErr
		s.mu.Unlock()
		return err
	}
	s.mu.Unlock()
	if err := s.cli.Start(ctx); err != nil {
		err = fmt.Errorf("mcp %s: start client: %w", s.config.Name, err)
		s.mu.Lock()
		s.initErr = err
		s.mu.Unlock()
		return err
	}
	result, err := s.cli.Initialize(ctx, mcp.InitializeRequest{Params: mcp.InitializeParams{
		ProtocolVersion: mcp.LATEST_PROTOCOL_VERSION,
		Capabilities:    mcp.ClientCapabilities{},
		ClientInfo:      mcp.Implementation{Name: "vivy", Version: "0.1"},
	}})
	if err != nil {
		err = fmt.Errorf("mcp %s: initialize: %w", s.config.Name, err)
		s.mu.Lock()
		s.initErr = err
		s.mu.Unlock()
		return err
	}
	s.mu.Lock()
	s.caps = result.Capabilities
	s.ready = true
	s.mu.Unlock()
	return nil
}

func isSessionTerminated(err error) bool {
	return errors.Is(err, mcptransport.ErrSessionTerminated)
}

func runMCP[T any](ctx context.Context, b *MCPBackend, name string, retry bool, op func(context.Context, *mcpServer) (T, error)) (T, error) {
	var zero T
	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		entry, err := b.acquireServer(name)
		if err != nil {
			return zero, err
		}
		initErr := entry.initialize(ctx)
		b.noteInitResult(entry, initErr)
		err = initErr
		var result T
		if err == nil {
			result, err = op(ctx, entry)
		}
		entry.release()
		if err == nil {
			return result, nil
		}
		if initErr != nil || isSessionTerminated(err) {
			b.replaceSession(name, entry)
		}
		lastErr = mapMCPError(err)
		if !retry || !isSessionTerminated(err) || attempt == 1 {
			return zero, lastErr
		}
	}
	return zero, lastErr
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
