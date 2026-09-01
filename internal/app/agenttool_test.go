package app

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/tools"
)

type fakeAgentSurfaceTool struct {
	name     string
	readonly bool
}

func (f fakeAgentSurfaceTool) Spec() domain.ToolSpec {
	return domain.ToolSpec{Name: f.name, Readonly: f.readonly}
}

func (f fakeAgentSurfaceTool) InvokableRun(context.Context, json.RawMessage) (string, error) {
	return "", nil
}

func TestReadOnlyToolNamesFiltersSurface(t *testing.T) {
	manager := &workerManager{
		tools: map[string]tools.Tool{
			"read_file":     fakeAgentSurfaceTool{name: "read_file", readonly: true},
			"write_file":    fakeAgentSurfaceTool{name: "write_file"},
			"bash":          fakeAgentSurfaceTool{name: "bash"},
			"mcp_call":      fakeAgentSurfaceTool{name: "mcp_call", readonly: true},
			tools.AgentName: fakeAgentSurfaceTool{name: tools.AgentName, readonly: true},
			"grep":          fakeAgentSurfaceTool{name: "grep", readonly: true},
		},
		toolOrder: []string{"read_file", "write_file", "bash", "mcp_call", tools.AgentName, "grep"},
	}
	names := manager.readOnlyToolNames()
	want := []string{"read_file", "grep"}
	if len(names) != len(want) {
		t.Fatalf("names = %v, want %v", names, want)
	}
	for i := range want {
		if names[i] != want[i] {
			t.Fatalf("names = %v, want %v", names, want)
		}
	}
}

func TestAgentSystemPromptIncludesMask(t *testing.T) {
	base := agentSystemPrompt("")
	if strings.Contains(base, "Persona hint") {
		t.Fatalf("base prompt = %q, want no mask section", base)
	}
	masked := agentSystemPrompt("terse reviewer")
	if !strings.Contains(masked, "Persona hint (mask): terse reviewer") {
		t.Fatalf("masked prompt = %q, want the persona hint", masked)
	}
	if !strings.HasPrefix(masked, base) {
		t.Fatal("masked prompt must extend the base prompt")
	}
}

func TestStartAgentTaskRequiresWiredManager(t *testing.T) {
	ref := &agentToolRef{}
	if _, err := ref.StartAgentTask(context.Background(), "task", ""); err == nil || !strings.Contains(err.Error(), "not wired") {
		t.Fatalf("err = %v, want not-wired failure", err)
	}
}

func TestStartAgentTaskRequiresRunScopedParent(t *testing.T) {
	manager, _, _, _ := newChildBrokerTest(t)
	ref := &agentToolRef{}
	ref.arm(manager)
	if _, err := ref.StartAgentTask(context.Background(), "task", ""); err == nil || !strings.Contains(err.Error(), "run-scoped") {
		t.Fatalf("err = %v, want run-scoped parent failure", err)
	}
}

func TestStartAgentTaskRejectsBeforeSpawning(t *testing.T) {
	manager, _, root, _ := newChildBrokerTest(t)
	manager.parentCounts[root.ID] = maxChildrenPerParent
	ref := &agentToolRef{}
	ref.arm(manager)
	ctx := tools.WithRunID(context.Background(), root.ID)
	if _, err := ref.StartAgentTask(ctx, "blocked", ""); err == nil {
		t.Fatal("parent concurrency limit must reject the delegation before spawning")
	}
}
