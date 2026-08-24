package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/tools"
)

const (
	defaultMCPTimeout   = 8 * time.Second
	maxMCPResponseBytes = 512 << 10
	maxMCPContentBytes  = 256 << 10
)

type MCPServerConfig struct {
	Name     string
	Endpoint string
	AuthEnv  string
}

type EinoMCPBackend struct {
	client           *http.Client
	servers          map[string]MCPServerConfig
	sessions         map[string]*mcpSession
	mu               sync.Mutex
	nextID           uint64
	maxResponseBytes int
	timeout          time.Duration
}

type mcpSession struct {
	initialized bool
	sessionID   string
}

var _ tools.MCPOperations = (*EinoMCPBackend)(nil)
var _ interface {
	PrepareMCPCall(context.Context, domain.RunID, tools.MCPCallRequest) (domain.ToolProposal, error)
} = (*EinoMCPBackend)(nil)

func NewEinoMCPBackend(configs []MCPServerConfig, client *http.Client) *EinoMCPBackend {
	if client == nil {
		client = &http.Client{Timeout: defaultMCPTimeout}
	}
	servers := make(map[string]MCPServerConfig, len(configs))
	for _, config := range configs {
		name := strings.TrimSpace(config.Name)
		if name != "" {
			servers[name] = config
		}
	}
	return &EinoMCPBackend{client: client, servers: servers, sessions: make(map[string]*mcpSession), maxResponseBytes: maxMCPResponseBytes, timeout: defaultMCPTimeout}
}
func (b *EinoMCPBackend) ListTools(ctx context.Context, _ domain.RunID, server string) (tools.MCPListResponse, error) {
	server = strings.TrimSpace(server)
	if server != "" {
		result, err := b.listServer(ctx, server)
		if err != nil {
			return tools.MCPListResponse{}, err
		}
		return tools.MCPListResponse{Tools: result, Untrusted: true}, nil
	}
	all := make([]tools.MCPTool, 0)
	for _, name := range b.serverNames() {
		result, err := b.listServer(ctx, name)
		if err != nil {
			return tools.MCPListResponse{}, err
		}
		all = append(all, result...)
	}
	return tools.MCPListResponse{Tools: all, Untrusted: true}, nil
}

func (b *EinoMCPBackend) listServer(ctx context.Context, server string) ([]tools.MCPTool, error) {
	config, err := b.server(server)
	if err != nil {
		return nil, err
	}
	result, err := b.rpc(ctx, config, "tools/list", map[string]any{}, true)
	if err != nil {
		return nil, err
	}
	var payload struct {
		Tools []struct {
			Name        string          `json:"name"`
			Description string          `json:"description"`
			InputSchema json.RawMessage `json:"inputSchema"`
		} `json:"tools"`
	}
	if err := json.Unmarshal(result, &payload); err != nil {
		return nil, fmt.Errorf("mcp %s: tools/list result: %w", server, err)
	}
	toolsOut := make([]tools.MCPTool, 0, len(payload.Tools))
	for _, item := range payload.Tools {
		if strings.TrimSpace(item.Name) == "" {
			continue
		}
		if tools.IsBrowserUseName(item.Name) {
			continue
		}
		toolsOut = append(toolsOut, tools.MCPTool{Server: server, Name: item.Name, Description: item.Description, InputSchema: boundedRaw(item.InputSchema, maxMCPContentBytes)})
	}
	return toolsOut, nil
}

func (b *EinoMCPBackend) CallTool(ctx context.Context, _ domain.RunID, request tools.MCPCallRequest) (tools.MCPCallResponse, error) {
	config, err := b.server(request.Server)
	if err != nil {
		return tools.MCPCallResponse{}, err
	}
	result, err := b.rpc(ctx, config, "tools/call", map[string]any{"name": request.Tool, "arguments": request.Arguments}, false)
	if err != nil {
		return tools.MCPCallResponse{}, err
	}
	var payload struct {
		Content []struct {
			Type     string `json:"type"`
			Text     string `json:"text"`
			Data     string `json:"data"`
			MIMEType string `json:"mimeType"`
		} `json:"content"`
		IsError bool `json:"isError"`
	}
	if err := json.Unmarshal(result, &payload); err != nil {
		return tools.MCPCallResponse{}, fmt.Errorf("mcp %s/%s result: %w", request.Server, request.Tool, err)
	}
	content := make([]tools.MCPContent, 0, len(payload.Content))
	used := 0
	for _, item := range payload.Content {
		text, data := boundedString(item.Text, maxMCPContentBytes-used), boundedString(item.Data, maxMCPContentBytes-used)
		used += len(text) + len(data)
		if text == "" && data == "" && item.Type != "resource" {
			continue
		}
		content = append(content, tools.MCPContent{Type: item.Type, Text: text, Data: data, MIME: item.MIMEType})
		if used >= maxMCPContentBytes {
			break
		}
	}
	return tools.MCPCallResponse{Server: request.Server, Tool: request.Tool, Content: content, IsError: payload.IsError, Untrusted: true}, nil
}
func (b *EinoMCPBackend) PrepareMCPCall(_ context.Context, _ domain.RunID, request tools.MCPCallRequest) (domain.ToolProposal, error) {
	if _, err := b.server(request.Server); err != nil {
		return domain.ToolProposal{}, err
	}
	payload, _ := json.Marshal(request)
	preview := string(payload)
	if len(preview) > 4096 {
		preview = preview[:4096] + "..."
	}
	return domain.ToolProposal{Action: tools.MCPCallName, Target: request.Server + "/" + request.Tool, Preview: preview, RiskFindings: []string{"remote MCP side effect is unknown"}, Data: payload}, nil
}

func (b *EinoMCPBackend) server(name string) (MCPServerConfig, error) {
	name = strings.TrimSpace(name)
	config, ok := b.servers[name]
	if !ok || strings.TrimSpace(config.Endpoint) == "" {
		return MCPServerConfig{}, fmt.Errorf("mcp: server %q is not configured", name)
	}
	return config, nil
}

func (b *EinoMCPBackend) serverNames() []string {
	b.mu.Lock()
	defer b.mu.Unlock()
	names := make([]string, 0, len(b.servers))
	for name := range b.servers {
		names = append(names, name)
	}
	return names
}

func (b *EinoMCPBackend) rpc(ctx context.Context, config MCPServerConfig, method string, params any, retryable bool) ([]byte, error) {
	for attempt := 0; attempt < 2; attempt++ {
		if method != "initialize" {
			if err := b.ensureSession(ctx, config); err != nil {
				if retryable && attempt == 0 {
					b.invalidate(config.Name)
					continue
				}
				return nil, err
			}
		}
		result, sessionID, err := b.send(ctx, config, method, params, true)
		if err == nil {
			if sessionID != "" {
				b.setSessionID(config.Name, sessionID)
			}
			return result, nil
		}
		b.invalidate(config.Name)
		if !retryable || attempt == 1 {
			return nil, err
		}
	}
	return nil, errors.New("mcp: request retry exhausted")
}

func (b *EinoMCPBackend) ensureSession(ctx context.Context, config MCPServerConfig) error {
	b.mu.Lock()
	session := b.sessions[config.Name]
	if session != nil && session.initialized {
		b.mu.Unlock()
		return nil
	}
	b.mu.Unlock()
	params := map[string]any{"protocolVersion": "2024-11-05", "capabilities": map[string]any{}, "clientInfo": map[string]string{"name": "vivy", "version": "0.1"}}
	result, sessionID, err := b.send(ctx, config, "initialize", params, true)
	if err != nil {
		return err
	}
	var response struct {
		ProtocolVersion string `json:"protocolVersion"`
	}
	if err := json.Unmarshal(result, &response); err != nil || response.ProtocolVersion == "" {
		return fmt.Errorf("mcp %s: invalid initialize result", config.Name)
	}
	b.mu.Lock()
	b.sessions[config.Name] = &mcpSession{initialized: true, sessionID: sessionID}
	b.mu.Unlock()
	_, _, _ = b.send(ctx, config, "notifications/initialized", map[string]any{}, false)
	return nil
}
func (b *EinoMCPBackend) send(ctx context.Context, config MCPServerConfig, method string, params any, withResponse bool) ([]byte, string, error) {
	requestID := atomic.AddUint64(&b.nextID, 1)
	message := map[string]any{"jsonrpc": "2.0", "method": method, "params": params}
	if withResponse {
		message["id"] = requestID
	}
	body, err := json.Marshal(message)
	if err != nil {
		return nil, "", err
	}
	reqCtx, cancel := context.WithTimeout(ctx, b.timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, config.Endpoint, strings.NewReader(string(body)))
	if err != nil {
		return nil, "", fmt.Errorf("mcp %s: build request: %w", config.Name, err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	b.mu.Lock()
	if session := b.sessions[config.Name]; session != nil && session.sessionID != "" {
		req.Header.Set("Mcp-Session-Id", session.sessionID)
	}
	b.mu.Unlock()
	if config.AuthEnv != "" {
		if value := os.Getenv(config.AuthEnv); value != "" {
			req.Header.Set("Authorization", "Bearer "+value)
		}
	}
	resp, err := b.client.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("mcp %s: %w", config.Name, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, "", fmt.Errorf("mcp %s: HTTP %d", config.Name, resp.StatusCode)
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, int64(b.maxResponseBytes)+1))
	if err != nil {
		return nil, "", err
	}
	if len(raw) > b.maxResponseBytes {
		return nil, "", fmt.Errorf("mcp %s: response exceeds size limit", config.Name)
	}
	if !withResponse {
		return nil, resp.Header.Get("Mcp-Session-Id"), nil
	}
	var envelope struct {
		Result json.RawMessage `json:"result"`
		Error  *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, "", fmt.Errorf("mcp %s: invalid JSON-RPC response: %w", config.Name, err)
	}
	if envelope.Error != nil {
		return nil, "", fmt.Errorf("mcp %s: remote error %d: %s", config.Name, envelope.Error.Code, envelope.Error.Message)
	}
	if len(envelope.Result) == 0 {
		return nil, "", fmt.Errorf("mcp %s: response has no result", config.Name)
	}
	return envelope.Result, resp.Header.Get("Mcp-Session-Id"), nil
}
func (b *EinoMCPBackend) invalidate(name string) {
	b.mu.Lock()
	delete(b.sessions, name)
	b.mu.Unlock()
}

func (b *EinoMCPBackend) setSessionID(name, id string) {
	if id == "" {
		return
	}
	b.mu.Lock()
	if session := b.sessions[name]; session != nil {
		session.sessionID = id
	}
	b.mu.Unlock()
}

func boundedRaw(value json.RawMessage, limit int) json.RawMessage {
	if len(value) <= limit {
		return value
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
