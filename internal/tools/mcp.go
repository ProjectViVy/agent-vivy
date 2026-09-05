package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"agent-vivy/internal/domain"
)

const (
	MCPListToolsName = "mcp_list_tools"
	MCPCallName      = "mcp_call"
)

type MCPTool struct {
	Server      string          `json:"server"`
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	InputSchema json.RawMessage `json:"input_schema,omitempty"`
}

type MCPContent struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
	Data string `json:"data,omitempty"`
	MIME string `json:"mime_type,omitempty"`
}

type MCPListResponse struct {
	Tools     []MCPTool `json:"tools"`
	Untrusted bool      `json:"untrusted"`
}

type MCPCallRequest struct {
	Server    string         `json:"server"`
	Tool      string         `json:"tool"`
	Arguments map[string]any `json:"arguments,omitempty"`
}

type MCPCallResponse struct {
	Server    string       `json:"server"`
	Tool      string       `json:"tool"`
	Content   []MCPContent `json:"content"`
	IsError   bool         `json:"is_error,omitempty"`
	Untrusted bool         `json:"untrusted"`
}

// MCPResource is one entry returned by the remote resources/list method.
// The server name is added by Vivy so a result remains attributable even
// when several configured servers expose the same URI. Remote metadata is
// untrusted and is never interpreted as a local path or mounted state.
type MCPResource struct {
	Server      string          `json:"server"`
	URI         string          `json:"uri"`
	Name        string          `json:"name,omitempty"`
	Title       string          `json:"title,omitempty"`
	Description string          `json:"description,omitempty"`
	MIME        string          `json:"mime_type,omitempty"`
	Size        int64           `json:"size,omitempty"`
	Annotations json.RawMessage `json:"annotations,omitempty"`
	Meta        json.RawMessage `json:"_meta,omitempty"`
}

// MCPResourceContent is one untrusted item returned by resources/read. MCP
// permits either text or base64-encoded blob content; both fields are kept
// distinct so callers do not accidentally decode or execute remote data.
type MCPResourceContent struct {
	URI         string          `json:"uri"`
	MIME        string          `json:"mime_type,omitempty"`
	Text        *string         `json:"text,omitempty"`
	Blob        *string         `json:"blob,omitempty"`
	Annotations json.RawMessage `json:"annotations,omitempty"`
	Meta        json.RawMessage `json:"_meta,omitempty"`
}

// MCPRemoteError preserves a JSON-RPC error code from an MCP server so the
// control plane can map stable protocol failures without parsing text.
type MCPRemoteError struct {
	Code    int
	Message string
}

func (e *MCPRemoteError) Error() string {
	return fmt.Sprintf("remote error %d: %s", e.Code, e.Message)
}

// MCPListResourcesResponse is the bounded, read-only TUI/control-plane view
// of resources/list. It intentionally carries no mounted/connected claim.
type MCPListResourcesResponse struct {
	Server    string        `json:"server,omitempty"`
	Resources []MCPResource `json:"resources"`
	Untrusted bool          `json:"untrusted"`
}

type MCPReadResourceRequest struct {
	Server string `json:"server"`
	URI    string `json:"uri"`
}

// MCPReadResourceResponse is the bounded, read-only TUI/control-plane view
// of resources/read. URI is echoed for the requested resource while Contents
// preserves each remote content item's actual URI and representation.
type MCPReadResourceResponse struct {
	Server    string               `json:"server"`
	URI       string               `json:"uri"`
	Contents  []MCPResourceContent `json:"contents"`
	Untrusted bool                 `json:"untrusted"`
}

// MCPPromptArgument is one server-declared prompt argument. Remote labels and
// descriptions are presentation-only; Name is the protocol identity sent
// back to prompts/get.
type MCPPromptArgument struct {
	Name        string `json:"name"`
	Title       string `json:"title,omitempty"`
	Description string `json:"description,omitempty"`
	Required    bool   `json:"required,omitempty"`
}

type MCPPrompt struct {
	Server      string              `json:"server"`
	Name        string              `json:"name"`
	Title       string              `json:"title,omitempty"`
	Description string              `json:"description,omitempty"`
	Arguments   []MCPPromptArgument `json:"arguments,omitempty"`
}

type MCPListPromptsResponse struct {
	Prompts   []MCPPrompt `json:"prompts"`
	Untrusted bool        `json:"untrusted"`
}

type MCPGetPromptRequest struct {
	Server    string            `json:"server"`
	Name      string            `json:"name"`
	Arguments map[string]string `json:"arguments,omitempty"`
}

type MCPGetPromptResponse struct {
	Server      string `json:"server"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Text        string `json:"text"`
	Untrusted   bool   `json:"untrusted"`
}

// MCPResourceOperations is the read-only resource surface used by the
// control plane. It is deliberately separate from MCPOperations so MCP
// resources are not registered as model-visible tools or given side effects.
type MCPResourceOperations interface {
	ListResources(context.Context, domain.RunID, string) (MCPListResourcesResponse, error)
	ReadResource(context.Context, domain.RunID, MCPReadResourceRequest) (MCPReadResourceResponse, error)
}

// MCPPromptOperations is a control-plane-only prompt surface. Prompt content
// is untrusted model input and is never executed as a local command.
type MCPPromptOperations interface {
	ListPrompts(context.Context, domain.RunID, string) (MCPListPromptsResponse, error)
	GetPrompt(context.Context, domain.RunID, MCPGetPromptRequest) (MCPGetPromptResponse, error)
}

type MCPOperations interface {
	ListTools(context.Context, domain.RunID, string) (MCPListResponse, error)
	CallTool(context.Context, domain.RunID, MCPCallRequest) (MCPCallResponse, error)
}

type mcpProposalOperations interface {
	PrepareMCPCall(context.Context, domain.RunID, MCPCallRequest) (domain.ToolProposal, error)
}

type mcpListToolsTool struct{ ops MCPOperations }
type mcpCallTool struct{ ops MCPOperations }

func NewMCPListTools(ops MCPOperations) Tool { return &mcpListToolsTool{ops: ops} }
func NewMCPCall(ops MCPOperations) Tool      { return &mcpCallTool{ops: ops} }
func (t *mcpListToolsTool) Spec() domain.ToolSpec {
	return domain.ToolSpec{Name: MCPListToolsName, Description: "Lists remote MCP tools with server provenance; remote schemas are untrusted.", Readonly: true, Keywords: []string{"mcp", "remote", "tools", "catalog"}, Params: map[string]domain.ToolParam{"server": {Desc: "Optional configured MCP server name."}}}
}

func (t *mcpListToolsTool) InvokableRun(ctx context.Context, args json.RawMessage) (string, error) {
	var input struct {
		Server string `json:"server"`
	}
	if err := json.Unmarshal(args, &input); err != nil {
		return "", fmt.Errorf("mcp_list_tools: invalid arguments: %w", err)
	}
	if t.ops == nil {
		return "", fmt.Errorf("tools: MCP backend not wired")
	}
	result, err := t.ops.ListTools(ctx, RunIDFromContext(ctx), strings.TrimSpace(input.Server))
	if err != nil {
		return "", err
	}
	return marshalToolResult(result)
}

func (t *mcpCallTool) Spec() domain.ToolSpec {
	return domain.ToolSpec{Name: MCPCallName, Description: "Calls a configured MCP tool with explicit server and tool provenance; always approval-gated.", Readonly: false, Keywords: []string{"mcp", "remote", "call"}, Params: map[string]domain.ToolParam{"server": {Desc: "Configured MCP server name.", Required: true}, "tool": {Desc: "Remote MCP tool name.", Required: true}, "arguments": {Desc: "JSON object passed to the remote tool.", Type: "object"}}}
}

func (t *mcpCallTool) InvokableRun(ctx context.Context, args json.RawMessage) (string, error) {
	request, err := decodeMCPCall(args)
	if err != nil {
		return "", err
	}
	if t.ops == nil {
		return "", fmt.Errorf("tools: MCP backend not wired")
	}
	result, err := t.ops.CallTool(ctx, RunIDFromContext(ctx), request)
	if err != nil {
		return "", err
	}
	return marshalToolResult(result)
}
func (t *mcpCallTool) PrepareProposal(ctx context.Context, args json.RawMessage) (domain.ToolProposal, error) {
	request, err := decodeMCPCall(args)
	if err != nil {
		return domain.ToolProposal{}, err
	}
	if planner, ok := t.ops.(mcpProposalOperations); ok {
		return planner.PrepareMCPCall(ctx, RunIDFromContext(ctx), request)
	}
	payload, _ := json.Marshal(request)
	return domain.ToolProposal{Action: MCPCallName, Target: request.Server + "/" + request.Tool, Preview: string(payload), RiskFindings: []string{"remote MCP side effect is unknown"}, Data: payload}, nil
}

func decodeMCPCall(args json.RawMessage) (MCPCallRequest, error) {
	var request MCPCallRequest
	if err := json.Unmarshal(args, &request); err != nil {
		return MCPCallRequest{}, fmt.Errorf("mcp_call: invalid arguments: %w", err)
	}
	request.Server, request.Tool = strings.TrimSpace(request.Server), strings.TrimSpace(request.Tool)
	if request.Server == "" {
		return MCPCallRequest{}, &ArgError{Field: "server", Reason: "is required"}
	}
	if request.Tool == "" {
		return MCPCallRequest{}, &ArgError{Field: "tool", Reason: "is required"}
	}
	if IsBrowserUseName(request.Tool) {
		return MCPCallRequest{}, fmt.Errorf("mcp_call: browser automation tool %q is excluded from Vivy", request.Tool)
	}
	if request.Arguments == nil {
		request.Arguments = map[string]any{}
	}
	return request, nil
}
