package defaults

import (
	"context"
	"encoding/json"

	"agent-vivy/internal/tools"
	"agent-vivy/sdk/port/tool"
	"agent-vivy/sdk/port/toolworld"
)

func ProtectedToolProviders() []tool.ToolProvider {
	ids := tools.AssemblyControlledToolNames()
	out := make([]tool.ToolProvider, 0, len(ids))
	for _, id := range ids {
		out = append(out, protectedToolProvider{id: id})
	}
	return out
}

type protectedToolProvider struct{ id string }

func (p protectedToolProvider) Definition() tool.Definition { return tool.Definition{ID: p.id} }
func (p protectedToolProvider) Invoke(ctx context.Context, host tool.Host, args json.RawMessage) (tool.Result, error) {
	text, err := host.InvokeTool(ctx, p.id, args)
	return tool.Result{Text: text}, err
}

func NewMCPProvider() toolworld.Provider { return mcpProvider{} }

type mcpProvider struct{}

func (mcpProvider) Definition() toolworld.Definition {
	return toolworld.Definition{ID: "mcp", Description: "Model Context Protocol tools"}
}
func (mcpProvider) Discover(ctx context.Context, host toolworld.Host) ([]toolworld.ToolDefinition, error) {
	return nil, nil
}
func (mcpProvider) Invoke(ctx context.Context, host toolworld.Host, id string, args json.RawMessage) (toolworld.Result, error) {
	return toolworld.Result{}, toolworld.ErrInvalidArgs
}
func (mcpProvider) Close(context.Context) error { return nil }
