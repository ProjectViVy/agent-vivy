package runtime

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"agent-vivy/internal/mcphost"
	"agent-vivy/internal/tools"
)

// MCPHostSessionFactory is the quarantined bridge from MCPHost's Vivy-only
// session contract to the existing mcp-go/EinoExt-backed runtime backend.
// Constructing one session performs no network connection; MCPHost decides
// when the first operation activates it.
type MCPHostSessionFactory struct {
	httpClient *http.Client
	options    MCPBackendOptions
}

func NewMCPHostSessionFactory(httpClient *http.Client, options MCPBackendOptions) *MCPHostSessionFactory {
	return &MCPHostSessionFactory{httpClient: httpClient, options: options}
}

func (factory *MCPHostSessionFactory) Open(ctx context.Context, config mcphost.InstanceConfig) (mcphost.Session, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	backend := NewMCPBackendWithOptions([]MCPServerConfig{{
		Name: config.ID, Endpoint: config.Endpoint, Command: config.Command,
		Args: append([]string(nil), config.Args...), EnvFrom: cloneStringMap(config.EnvFrom),
		Cwd: config.Cwd, AuthEnv: config.AuthEnv,
	}}, factory.httpClient, factory.options)
	return &mcpHostSession{backend: backend, server: config.ID}, nil
}

// existingMCPBackendSessionFactory lets MCPHost share the already-composed
// control-plane MCP backend. The Host owns activation/retry/circuit policy,
// while the backend continues to own the pinned mcp-go/EinoExt transport.
type existingMCPBackendSessionFactory struct {
	backend *MCPBackend
}

func (factory existingMCPBackendSessionFactory) Open(ctx context.Context, config mcphost.InstanceConfig) (mcphost.Session, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if factory.backend == nil {
		return nil, mcphost.ErrInstanceUnavailable
	}
	return &mcpHostSession{backend: factory.backend, server: config.ID}, nil
}

type mcpHostSession struct {
	backend *MCPBackend
	server  string
}

func (session *mcpHostSession) DiscoverTools(ctx context.Context) ([]mcphost.RemoteTool, error) {
	response, err := session.backend.ListTools(ctx, "", session.server)
	if err != nil {
		return nil, err
	}
	out := make([]mcphost.RemoteTool, 0, len(response.Tools))
	for _, tool := range response.Tools {
		out = append(out, mcphost.RemoteTool{
			Name: tool.Name, Description: tool.Description,
			Schema: append(json.RawMessage(nil), tool.InputSchema...),
		})
	}
	return out, nil
}

func (session *mcpHostSession) CallTool(ctx context.Context, remoteName string, arguments json.RawMessage) (mcphost.ToolResult, error) {
	values := map[string]any{}
	if len(arguments) > 0 {
		if err := json.Unmarshal(arguments, &values); err != nil {
			return mcphost.ToolResult{}, err
		}
	}
	response, err := session.backend.CallTool(ctx, "", tools.MCPCallRequest{
		Server: session.server, Tool: strings.TrimSpace(remoteName), Arguments: values,
	})
	if err != nil {
		return mcphost.ToolResult{}, err
	}
	raw, err := json.Marshal(response)
	if err != nil {
		return mcphost.ToolResult{}, err
	}
	return mcphost.ToolResult{Text: string(raw), IsError: response.IsError}, nil
}

func (session *mcpHostSession) ListResources(ctx context.Context) ([]mcphost.RemoteResource, error) {
	response, err := session.backend.ListResources(ctx, "", session.server)
	if err != nil {
		return nil, err
	}
	out := make([]mcphost.RemoteResource, 0, len(response.Resources))
	for _, resource := range response.Resources {
		out = append(out, mcphost.RemoteResource{
			URI: resource.URI, Name: resource.Name, Description: resource.Description, MediaType: resource.MIME,
		})
	}
	return out, nil
}

func (session *mcpHostSession) ReadResource(ctx context.Context, uri string) (mcphost.ResourceContent, error) {
	response, err := session.backend.ReadResource(ctx, "", tools.MCPReadResourceRequest{Server: session.server, URI: uri})
	if err != nil {
		return mcphost.ResourceContent{}, err
	}
	result := mcphost.ResourceContent{URI: uri}
	var textParts []string
	var blobParts []string
	for _, content := range response.Contents {
		if result.MediaType == "" {
			result.MediaType = content.MIME
		}
		if content.Text != nil {
			textParts = append(textParts, *content.Text)
		}
		if content.Blob != nil {
			blobParts = append(blobParts, *content.Blob)
		}
	}
	result.Text = strings.Join(textParts, "\n\n")
	if len(blobParts) > 0 {
		result.Blob = []byte(strings.Join(blobParts, ""))
	}
	return result, nil
}

func (session *mcpHostSession) Close() error {
	if session.backend == nil {
		return nil
	}
	return session.backend.closeServer(session.server)
}

func cloneStringMap(values map[string]string) map[string]string {
	if values == nil {
		return nil
	}
	out := make(map[string]string, len(values))
	for key, value := range values {
		out[key] = value
	}
	return out
}

var _ mcphost.SessionFactory = (*MCPHostSessionFactory)(nil)
var _ mcphost.SessionFactory = existingMCPBackendSessionFactory{}
var _ mcphost.Session = (*mcpHostSession)(nil)
