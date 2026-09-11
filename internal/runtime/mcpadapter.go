package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"agent-vivy/internal/mcphost"
	"agent-vivy/internal/tools"

	einomcp "github.com/cloudwego/eino-ext/components/tool/mcp"
	einotool "github.com/cloudwego/eino/components/tool"
	"github.com/mark3labs/mcp-go/mcp"
)

// MCPHostSessionFactory is the quarantined runtime adapter boundary. It
// creates exactly one official mcp-go transport session for each MCPHost
// instance activation. The resulting session is owned and closed by
// MCPHost; this factory never stores or retries it.
type MCPHostSessionFactory struct {
	builder *MCPBackend
	// status binds a Host instance to the exact logical configuration that
	// created its transport. Resolving by name alone is unsafe during a live
	// replacement: ReplaceServers publishes the new status record before the
	// retired Host generation is detached.
	status func(MCPServerConfig) *mcpServer
}

func NewMCPHostSessionFactory(httpClient *http.Client, options MCPBackendOptions) *MCPHostSessionFactory {
	builder := &MCPBackend{processRoot: strings.TrimSpace(options.ProcessRoot), logger: options.Logger}
	if builder.logger == nil {
		builder.logger = slog.Default()
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: defaultMCPTimeout}
	}
	builder.maxResponseBytes = maxMCPResponseBytes
	builder.client = boundedMCPHTTPClient(httpClient, func() int { return builder.maxResponseBytes })
	return &MCPHostSessionFactory{builder: builder}
}

// newMCPHostSessionFactory is used by the production configuration facade so
// Host sessions update the facade's status projection without borrowing any
// transport or lifecycle state from it.
func newMCPHostSessionFactory(backend *MCPBackend) *MCPHostSessionFactory {
	if backend == nil {
		return nil
	}
	return &MCPHostSessionFactory{
		builder: backend,
		status:  backend.statusForConfig,
	}
}

func (factory *MCPHostSessionFactory) Open(ctx context.Context, config mcphost.InstanceConfig) (mcphost.Session, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if factory == nil || factory.builder == nil {
		return nil, mcphost.ErrInstanceUnavailable
	}
	var record *mcpServer
	serverConfig := MCPServerConfig{
		Name: config.ID, Endpoint: config.Endpoint, Command: config.Command,
		Args: append([]string(nil), config.Args...), EnvFrom: cloneStringMap(config.EnvFrom),
		Cwd: config.Cwd, AuthEnv: config.AuthEnv, ResourceBridge: config.ResourceBridge,
		DeferredReason: config.DeferredReason, Enabled: config.Enabled,
	}
	if factory.status != nil {
		record = factory.status(serverConfig)
	}
	transport, err := factory.builder.newServerWithStatus(serverConfig, record)
	if err != nil {
		return nil, err
	}
	return &mcpHostSession{
		backend:      factory.builder,
		server:       config.ID,
		transport:    transport,
		privateOwner: factory.status == nil,
	}, nil
}

// existingMCPBackendSessionFactory remains a source-compatible composition
// seam for older internal callers. It still creates a fresh Host-owned
// transport; it never returns a borrowed or no-op-close session.
type existingMCPBackendSessionFactory struct{ backend *MCPBackend }

func (factory existingMCPBackendSessionFactory) Open(ctx context.Context, config mcphost.InstanceConfig) (mcphost.Session, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if factory.backend == nil {
		return nil, mcphost.ErrInstanceUnavailable
	}
	return newMCPHostSessionFactory(factory.backend).Open(ctx, config)
}

// mcpHostSession owns one transport session. backend is only a status/control
// facade and is never closed as part of a production session close.
type mcpHostSession struct {
	backend      *MCPBackend
	server       string
	transport    *mcpServer
	privateOwner bool
}

func (session *mcpHostSession) entry() (*mcpServer, error) {
	if session == nil || session.transport == nil {
		return nil, mcphost.ErrInstanceUnavailable
	}
	return session.transport, nil
}

func (session *mcpHostSession) DiscoverTools(ctx context.Context) ([]mcphost.RemoteTool, error) {
	entry, err := session.entry()
	if err != nil {
		return nil, err
	}
	if err := session.initialize(ctx, entry); err != nil {
		return nil, err
	}
	captureCtx, capture := withRawMCPToolSchemaCapture(ctx)
	mcpTools, err := einomcp.GetTools(captureCtx, &einomcp.Config{Cli: boundedMCPClient{MCPClient: entry.cli}})
	if err != nil {
		return nil, session.mapOperationError(entry, err)
	}
	projected, err := projectEinoTools(captureCtx, mcpTools, session.server, capture)
	if err != nil {
		return nil, err
	}
	entry.status.mu.Lock()
	entry.status.toolCount = len(projected)
	entry.status.mu.Unlock()
	out := make([]mcphost.RemoteTool, 0, len(projected))
	for _, item := range projected {
		out = append(out, mcphost.RemoteTool{Name: item.Name, Description: item.Description, Schema: append(json.RawMessage(nil), item.InputSchema...)})
	}
	return out, nil
}

// projectEinoTools is the sole EinoExt-to-Vivy tool conversion seam. Eino
// component types do not cross MCPHost or ToolHost boundaries; the adapter
// projects their bounded metadata into the runtime catalog after Host has
// initialized the owned transport.
func projectEinoTools(ctx context.Context, mcpTools []einotool.BaseTool, server string, captures ...*rawMCPToolSchemaCapture) ([]tools.MCPTool, error) {
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
		var rawPresent bool
		if len(captures) > 0 {
			inputSchema, rawPresent = captures[0].get(info.Name)
		}
		if !rawPresent && info.ParamsOneOf != nil {
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
		if len(inputSchema) > 0 {
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

func (session *mcpHostSession) CallTool(ctx context.Context, remoteName string, arguments json.RawMessage) (mcphost.ToolResult, error) {
	entry, err := session.entry()
	if err != nil {
		return mcphost.ToolResult{}, err
	}
	if err := session.initialize(ctx, entry); err != nil {
		return mcphost.ToolResult{}, err
	}
	params := mcp.CallToolParams{Name: strings.TrimSpace(remoteName)}
	if len(strings.TrimSpace(string(arguments))) > 0 {
		var value any
		if err := json.Unmarshal(arguments, &value); err != nil {
			return mcphost.ToolResult{}, fmt.Errorf("mcp %s/%s: invalid arguments: %w", session.server, remoteName, err)
		}
		params.Arguments = value
		params.RawArguments = append(json.RawMessage(nil), arguments...)
	}
	payload, err := entry.cli.CallTool(ctx, mcp.CallToolRequest{Params: params})
	if err != nil {
		return mcphost.ToolResult{}, session.mapOperationError(entry, err)
	}
	projected, err := projectCallResult(session.server, remoteName, payload)
	if err != nil {
		return mcphost.ToolResult{}, err
	}
	raw, err := json.Marshal(projected)
	if err != nil {
		return mcphost.ToolResult{}, err
	}
	return mcphost.ToolResult{Text: string(raw), IsError: projected.IsError}, nil
}

func (session *mcpHostSession) ListResources(ctx context.Context) ([]mcphost.RemoteResource, error) {
	entry, err := session.entry()
	if err != nil {
		return nil, err
	}
	if err := session.initialize(ctx, entry); err != nil {
		return nil, err
	}
	resources, err := listResourcePages(ctx, entry.cli, session.server, maxMCPContentBytes)
	if err != nil {
		return nil, session.mapOperationError(entry, err)
	}
	out := make([]mcphost.RemoteResource, 0, len(resources))
	for _, resource := range resources {
		out = append(out, mcphost.RemoteResource{
			URI: resource.URI, Name: resource.Name, Title: resource.Title,
			Description: resource.Description, MediaType: resource.MIME,
			Size: resource.Size, Annotations: append(json.RawMessage(nil), resource.Annotations...),
			Meta: append(json.RawMessage(nil), resource.Meta...),
		})
	}
	return out, nil
}

func (session *mcpHostSession) ReadResource(ctx context.Context, uri string) (mcphost.ResourceContent, error) {
	entry, err := session.entry()
	if err != nil {
		return mcphost.ResourceContent{}, err
	}
	if err := session.initialize(ctx, entry); err != nil {
		return mcphost.ResourceContent{}, err
	}
	payload, err := entry.cli.ReadResource(ctx, mcp.ReadResourceRequest{Params: mcp.ReadResourceParams{URI: uri}})
	if err != nil {
		return mcphost.ResourceContent{}, session.mapOperationError(entry, err)
	}
	projected, err := projectReadResource(session.server, uri, payload)
	if err != nil {
		return mcphost.ResourceContent{}, err
	}
	result := mcphost.ResourceContent{URI: projected.URI}
	var textParts []string
	var blobParts []string
	for _, content := range projected.Contents {
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

func (session *mcpHostSession) ListPrompts(ctx context.Context) ([]mcphost.RemotePrompt, error) {
	entry, err := session.entry()
	if err != nil {
		return nil, err
	}
	if err := session.initialize(ctx, entry); err != nil {
		return nil, err
	}
	if !entry.promptCapable() {
		return []mcphost.RemotePrompt{}, nil
	}
	prompts, err := listPromptsPages(ctx, entry.cli, session.server)
	if err != nil {
		return nil, session.mapOperationError(entry, err)
	}
	out := make([]mcphost.RemotePrompt, 0, len(prompts))
	for _, prompt := range prompts {
		arguments := make([]mcphost.RemotePromptArgument, 0, len(prompt.Arguments))
		for _, argument := range prompt.Arguments {
			arguments = append(arguments, mcphost.RemotePromptArgument{Name: argument.Name, Title: argument.Title, Description: argument.Description, Required: argument.Required})
		}
		out = append(out, mcphost.RemotePrompt{Name: prompt.Name, Title: prompt.Title, Description: prompt.Description, Arguments: arguments})
	}
	return out, nil
}

func (session *mcpHostSession) GetPrompt(ctx context.Context, name string, arguments map[string]string) (mcphost.PromptResult, error) {
	entry, err := session.entry()
	if err != nil {
		return mcphost.PromptResult{}, err
	}
	if err := session.initialize(ctx, entry); err != nil {
		return mcphost.PromptResult{}, err
	}
	payload, err := entry.cli.GetPrompt(ctx, mcp.GetPromptRequest{Params: mcp.GetPromptParams{Name: name, Arguments: arguments}})
	if err != nil {
		return mcphost.PromptResult{}, session.mapOperationError(entry, err)
	}
	projected, err := projectPrompt(session.server, name, payload)
	if err != nil {
		return mcphost.PromptResult{}, err
	}
	return mcphost.PromptResult{Name: projected.Name, Description: projected.Description, Text: projected.Text}, nil
}

func (session *mcpHostSession) initialize(ctx context.Context, entry *mcpServer) error {
	err := entry.initialize(ctx)
	if session.backend != nil {
		session.backend.noteInitResult(entry, err)
	}
	return err
}

func (session *mcpHostSession) mapOperationError(entry *mcpServer, err error) error {
	if entry.config.isStdio() && isStdioTransportClosed(err) {
		err = entry.markDead(err)
		if session.backend != nil {
			session.backend.noteInitResult(entry, err)
		}
	}
	return mapMCPError(err)
}

func (session *mcpHostSession) Close() error {
	if session == nil || session.transport == nil {
		return nil
	}
	err := session.transport.closeClient()
	if session.privateOwner && session.backend != nil {
		session.backend.mu.Lock()
		session.backend.closed = true
		session.backend.servers = make(map[string]*mcpServer)
		session.backend.mu.Unlock()
	}
	return err
}

func (session *mcpHostSession) TerminalFailure(_ error) bool {
	entry, err := session.entry()
	if err != nil {
		return true
	}
	// Local process initialization/death is terminal for this Host instance;
	// a settings replacement creates the only allowed fresh process. HTTP
	// sessions remain retryable so MCPHost can recover a terminated session.
	return entry.config.isStdio() && (entry.deadError() != nil || entry.initializeError() != nil)
}

func (session *mcpHostSession) RetryFailure(err error) bool {
	if session == nil {
		return false
	}
	entry, entryErr := session.entry()
	if entryErr != nil || entry.config.isStdio() {
		return false
	}
	// MCP's stateful HTTP session can be safely reopened only when mcp-go
	// reports the session as terminated (normally an HTTP 404). Handshake,
	// schema, and other transport failures are returned to the control caller
	// rather than acquiring an unbounded second authority here.
	return isSessionTerminated(err)
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
var _ mcphost.PromptSession = (*mcpHostSession)(nil)
var _ mcphost.RetryFailurePolicy = (*mcpHostSession)(nil)
