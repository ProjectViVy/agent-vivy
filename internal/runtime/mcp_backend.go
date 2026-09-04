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
	"sync/atomic"
	"time"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/tools"
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
	initialized     bool
	sessionID       string
	protocolVersion string
}

// MCPServerStatus is a secret-free snapshot of one configured server's
// process state. Initialized means the backend completed the MCP handshake;
// configured servers are not reported as connected before that happens.
type MCPServerStatus struct {
	Name        string
	Initialized bool
}

var _ tools.MCPOperations = (*EinoMCPBackend)(nil)
var _ tools.MCPResourceOperations = (*EinoMCPBackend)(nil)
var _ interface {
	PrepareMCPCall(context.Context, domain.RunID, tools.MCPCallRequest) (domain.ToolProposal, error)
} = (*EinoMCPBackend)(nil)

func NewEinoMCPBackend(configs []MCPServerConfig, client *http.Client) *EinoMCPBackend {
	if client == nil {
		client = &http.Client{Timeout: defaultMCPTimeout}
	}
	backend := &EinoMCPBackend{client: client, maxResponseBytes: maxMCPResponseBytes, timeout: defaultMCPTimeout}
	backend.ReplaceServers(configs)
	return backend
}

// ReplaceServers swaps the live catalog and drops cached sessions so the
// next list/call re-initializes against the new endpoints.
func (b *EinoMCPBackend) ReplaceServers(configs []MCPServerConfig) {
	servers := make(map[string]MCPServerConfig, len(configs))
	for _, config := range configs {
		name := strings.TrimSpace(config.Name)
		if name == "" || strings.TrimSpace(config.Endpoint) == "" {
			continue
		}
		config.Name = name
		config.Endpoint = strings.TrimSpace(config.Endpoint)
		config.AuthEnv = strings.TrimSpace(config.AuthEnv)
		servers[name] = config
	}
	b.mu.Lock()
	b.servers = servers
	b.sessions = make(map[string]*mcpSession)
	b.mu.Unlock()
}

// ConfiguredServers returns a snapshot of the live catalog (enabled
// servers only; the backend never stores disabled entries).
func (b *EinoMCPBackend) ConfiguredServers() []MCPServerConfig {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make([]MCPServerConfig, 0, len(b.servers))
	for _, config := range b.servers {
		out = append(out, config)
	}
	return out
}

// ServerStatuses reports the backend's current in-process MCP truth without
// probing the network or exposing endpoints and authentication metadata.
func (b *EinoMCPBackend) ServerStatuses() []MCPServerStatus {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make([]MCPServerStatus, 0, len(b.servers))
	for name := range b.servers {
		status := MCPServerStatus{Name: name}
		if session := b.sessions[name]; session != nil {
			status.Initialized = session.initialized
		}
		out = append(out, status)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
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

// ListResources performs the read-only MCP resources/list operation. An
// empty server lists each configured server in catalog order (the TUI uses a
// specific server so a one-shot command cannot accidentally fan out).
func (b *EinoMCPBackend) ListResources(ctx context.Context, _ domain.RunID, server string) (tools.MCPListResourcesResponse, error) {
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
		if used >= maxMCPContentBytes {
			break
		}
	}
	return tools.MCPListResourcesResponse{Resources: all, Untrusted: true}, nil
}

func (b *EinoMCPBackend) listResourcesServer(ctx context.Context, server string, limit int) ([]tools.MCPResource, error) {
	config, err := b.server(server)
	if err != nil {
		return nil, err
	}
	resources := make([]tools.MCPResource, 0)
	used := 0
	cursor := ""
	seenCursors := make(map[string]struct{})
	for page := 0; page < maxMCPResourcePages; page++ {
		params := map[string]any{}
		if cursor != "" {
			params["cursor"] = cursor
		}
		result, err := b.rpc(ctx, config, "resources/list", params, true)
		if err != nil {
			return nil, err
		}
		var payload struct {
			Resources []struct {
				URI         string          `json:"uri"`
				Name        string          `json:"name"`
				Title       string          `json:"title"`
				Description string          `json:"description"`
				MIMEType    string          `json:"mimeType"`
				Size        int64           `json:"size"`
				Annotations json.RawMessage `json:"annotations"`
				Meta        json.RawMessage `json:"_meta"`
			} `json:"resources"`
			NextCursor string `json:"nextCursor"`
		}
		if err := json.Unmarshal(result, &payload); err != nil {
			return nil, fmt.Errorf("mcp %s: resources/list result: %w", server, err)
		}
		for _, item := range payload.Resources {
			if strings.TrimSpace(item.URI) == "" {
				continue
			}
			identityBytes := len(server) + len(item.URI)
			if identityBytes > maxMCPContentBytes {
				return nil, fmt.Errorf("mcp %s: resource URI exceeds size limit", server)
			}
			if identityBytes > limit-used {
				return resources, nil
			}
			resource, consumed := boundedMCPResource(tools.MCPResource{
				Server: server, URI: item.URI, Name: item.Name, Title: item.Title,
				Description: item.Description, MIME: item.MIMEType, Size: item.Size,
				Annotations: item.Annotations, Meta: item.Meta,
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
		next := payload.NextCursor
		if next == "" {
			return resources, nil
		}
		if len(next) > maxMCPContentBytes {
			return nil, fmt.Errorf("mcp %s: resources/list cursor exceeds size limit", server)
		}
		if _, duplicate := seenCursors[next]; duplicate {
			return nil, fmt.Errorf("mcp %s: resources/list repeated cursor", server)
		}
		seenCursors[next] = struct{}{}
		cursor = next
	}
	return nil, fmt.Errorf("mcp %s: resources/list exceeded %d pages", server, maxMCPResourcePages)
}

// ReadResource performs the read-only MCP resources/read operation. The
// remote URI is returned verbatim (subject to the shared size budget) and
// resource content remains untrusted text/base64; it is not mounted, parsed,
// or written into a tenant workspace.
func (b *EinoMCPBackend) ReadResource(ctx context.Context, _ domain.RunID, request tools.MCPReadResourceRequest) (tools.MCPReadResourceResponse, error) {
	request.Server = strings.TrimSpace(request.Server)
	if strings.TrimSpace(request.URI) == "" {
		return tools.MCPReadResourceResponse{}, errors.New("mcp: resource URI is required")
	}
	if len(request.URI) > maxMCPContentBytes {
		return tools.MCPReadResourceResponse{}, fmt.Errorf("mcp: resource URI exceeds size limit (%d bytes)", maxMCPContentBytes)
	}
	config, err := b.server(request.Server)
	if err != nil {
		return tools.MCPReadResourceResponse{}, err
	}
	result, err := b.rpc(ctx, config, "resources/read", map[string]any{"uri": request.URI}, true)
	if err != nil {
		return tools.MCPReadResourceResponse{}, err
	}
	var payload struct {
		Contents []struct {
			URI         string          `json:"uri"`
			MIMEType    string          `json:"mimeType"`
			Text        *string         `json:"text"`
			Blob        *string         `json:"blob"`
			Annotations json.RawMessage `json:"annotations"`
			Meta        json.RawMessage `json:"_meta"`
		} `json:"contents"`
	}
	if err := json.Unmarshal(result, &payload); err != nil {
		return tools.MCPReadResourceResponse{}, fmt.Errorf("mcp %s/%s: resources/read result: %w", request.Server, request.URI, err)
	}
	contents := make([]tools.MCPResourceContent, 0, len(payload.Contents))
	used := 0
	for _, item := range payload.Contents {
		if strings.TrimSpace(item.URI) == "" {
			return tools.MCPReadResourceResponse{}, fmt.Errorf("mcp %s/%s: resource content URI is required", request.Server, request.URI)
		}
		if (item.Text == nil) == (item.Blob == nil) {
			return tools.MCPReadResourceResponse{}, fmt.Errorf("mcp %s/%s: resource content must contain exactly one of text or blob", request.Server, request.URI)
		}
		if len(item.URI)+len(item.MIMEType) > maxMCPContentBytes {
			return tools.MCPReadResourceResponse{}, fmt.Errorf("mcp %s/%s: resource content identity exceeds size limit", request.Server, request.URI)
		}
		if len(item.URI)+len(item.MIMEType) > maxMCPContentBytes-used {
			break
		}
		content, consumed := boundedMCPResourceContent(tools.MCPResourceContent{
			URI:         item.URI,
			MIME:        item.MIMEType,
			Text:        item.Text,
			Blob:        item.Blob,
			Annotations: item.Annotations,
			Meta:        item.Meta,
		}, maxMCPContentBytes-used)
		if consumed == 0 {
			break
		}
		contents = append(contents, content)
		used += consumed
		if used >= maxMCPContentBytes {
			break
		}
	}
	return tools.MCPReadResourceResponse{Server: request.Server, URI: request.URI, Contents: contents, Untrusted: true}, nil
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
	b.mu.Lock()
	config, ok := b.servers[name]
	b.mu.Unlock()
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
	var firstErr error
	for attempt := 0; attempt < 2; attempt++ {
		if method != "initialize" {
			if err := b.ensureSession(ctx, config); err != nil {
				if retryable && attempt == 0 {
					firstErr = err
					b.invalidate(config.Name)
					continue
				}
				if firstErr != nil {
					return nil, fmt.Errorf("%v; retry failed: %w", firstErr, err)
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
			if firstErr != nil {
				return nil, fmt.Errorf("%v; retry failed: %w", firstErr, err)
			}
			return nil, err
		}
		firstErr = err
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
	b.sessions[config.Name] = &mcpSession{initialized: true, sessionID: sessionID, protocolVersion: response.ProtocolVersion}
	b.mu.Unlock()
	_, _, _ = b.send(ctx, config, "notifications/initialized", map[string]any{}, false)
	return nil
}
func (b *EinoMCPBackend) send(ctx context.Context, config MCPServerConfig, method string, params any, withResponse bool) ([]byte, string, error) {
	var requestID uint64
	message := map[string]any{"jsonrpc": "2.0", "method": method, "params": params}
	if withResponse {
		requestID = atomic.AddUint64(&b.nextID, 1)
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
	if session := b.sessions[config.Name]; session != nil && session.protocolVersion != "" {
		req.Header.Set("MCP-Protocol-Version", session.protocolVersion)
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
	payload, err := decodeMCPResponse(raw, resp.Header.Get("Content-Type"), requestID)
	if err != nil {
		return nil, "", fmt.Errorf("mcp %s: %w", config.Name, err)
	}
	return payload, resp.Header.Get("Mcp-Session-Id"), nil
}

func decodeMCPResponse(raw []byte, contentType string, requestID uint64) ([]byte, error) {
	bodies := [][]byte{raw}
	if strings.Contains(strings.ToLower(contentType), "text/event-stream") {
		extracted, err := extractSSEJSON(raw)
		if err != nil {
			return nil, err
		}
		bodies = extracted
	}
	for _, body := range bodies {
		var envelope struct {
			ID     json.RawMessage `json:"id"`
			Result json.RawMessage `json:"result"`
			Error  *struct {
				Code    int    `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
		}
		if err := json.Unmarshal(body, &envelope); err != nil {
			return nil, fmt.Errorf("invalid JSON-RPC response: %w", err)
		}
		if strings.TrimSpace(string(envelope.ID)) != fmt.Sprintf("%d", requestID) {
			continue
		}
		if envelope.Error != nil {
			return nil, &tools.MCPRemoteError{Code: envelope.Error.Code, Message: envelope.Error.Message}
		}
		if len(envelope.Result) == 0 {
			return nil, errors.New("response has no result")
		}
		return envelope.Result, nil
	}
	return nil, fmt.Errorf("response has no matching id %d", requestID)
}

func extractSSEJSON(raw []byte) ([][]byte, error) {
	normalized := strings.ReplaceAll(string(raw), "\r\n", "\n")
	var events [][]byte
	for _, block := range strings.Split(normalized, "\n\n") {
		var data []string
		for _, line := range strings.Split(block, "\n") {
			if strings.HasPrefix(line, "data:") {
				value := strings.TrimPrefix(line, "data:")
				value = strings.TrimPrefix(value, " ")
				data = append(data, value)
			}
		}
		if len(data) == 0 {
			continue
		}
		joined := []byte(strings.Join(data, "\n"))
		if !json.Valid(joined) {
			return nil, errors.New("event-stream data is not JSON")
		}
		events = append(events, joined)
	}
	if len(events) == 0 {
		return nil, errors.New("event-stream response has no data")
	}
	return events, nil
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
	if limit <= 0 {
		return nil
	}
	if len(value) <= limit {
		return value
	}
	if limit < len("null") {
		return nil
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
	// Identity fields come first so a long description cannot crowd out the
	// server and URI needed to attribute the untrusted result.
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
		if limit-used <= 0 {
			return ""
		}
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
