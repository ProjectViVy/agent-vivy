package tools

import "agent-vivy/sdk/port/toolworld"

// DynamicToolWorldSource is an internal composition hint. It does not execute
// or register a Tool itself; the application extracts the Provider and gives
// it to the sole ToolHost. Keeping the interface here avoids a dependency from
// the generic ToolHost back into runtime MCP internals.
type DynamicToolWorldSource interface {
	DynamicToolWorld() toolworld.Provider
}

type mcpDynamicToolWorldProvider interface {
	MCPToolWorldProvider() toolworld.Provider
}

func (tool *mcpListToolsTool) DynamicToolWorld() toolworld.Provider {
	if tool == nil || tool.ops == nil {
		return nil
	}
	provider, ok := tool.ops.(mcpDynamicToolWorldProvider)
	if !ok {
		return nil
	}
	return provider.MCPToolWorldProvider()
}

var _ DynamicToolWorldSource = (*mcpListToolsTool)(nil)
