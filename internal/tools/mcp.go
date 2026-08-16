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
