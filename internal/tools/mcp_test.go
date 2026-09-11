package tools

import (
	"context"
	"reflect"
	"testing"

	"agent-vivy/internal/domain"
)

type catalogOnlyMCPOps struct{}

func (catalogOnlyMCPOps) ListTools(context.Context, domain.RunID, string) (MCPListResponse, error) {
	return MCPListResponse{}, nil
}

func TestBuiltinWithMCPDoesNotExposeDirectCallBypass(t *testing.T) {
	registry := BuiltinWithMCP(nil, nil, nil, nil, nil, nil, catalogOnlyMCPOps{})
	if _, ok := registry.Lookup(MCPListToolsName); !ok {
		t.Fatal("MCP control-plane listing tool disappeared")
	}
	if _, ok := registry.Lookup("mcp_call"); ok {
		t.Fatal("mcp_call remains model-visible and bypasses the governed ToolHost")
	}
}

func TestMCPOperationsHasNoDirectExecutionMethod(t *testing.T) {
	if _, ok := reflect.TypeOf((*MCPOperations)(nil)).Elem().MethodByName("CallTool"); ok {
		t.Fatal("MCPOperations still authorizes direct remote execution")
	}
}
