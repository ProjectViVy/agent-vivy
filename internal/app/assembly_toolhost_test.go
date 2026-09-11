package app

import (
	"context"
	"encoding/json"
	"testing"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/tools"
	toolworldport "agent-vivy/sdk/port/toolworld"
)

type fixtureLegacyTool struct {
	id     string
	result string
}

func (tool fixtureLegacyTool) Spec() domain.ToolSpec {
	return domain.ToolSpec{Name: tool.id, Description: "fixture legacy tool", Readonly: true}
}

func (tool fixtureLegacyTool) InvokableRun(context.Context, json.RawMessage) (string, error) {
	return tool.result, nil
}

type fixtureWorldProvider struct {
	id          string
	discoveries int
	invocations int
}

func (provider *fixtureWorldProvider) Definition() toolworldport.Definition {
	id := provider.id
	if id == "" {
		id = "fixture.world"
	}
	return toolworldport.Definition{ID: id}
}

func (provider *fixtureWorldProvider) Discover(context.Context, toolworldport.Host) ([]toolworldport.ToolDefinition, error) {
	provider.discoveries++
	id := "remote.echo"
	if provider.id == "mcp" {
		id = "mcp.docs.echo"
	}
	return []toolworldport.ToolDefinition{{
		ID:          id,
		Description: "fixture dynamic tool",
		Effect:      toolworldport.EffectRead,
		Schema:      json.RawMessage(`{"type":"object"}`),
	}}, nil
}

func (provider *fixtureWorldProvider) Invoke(context.Context, toolworldport.Host, string, json.RawMessage) (toolworldport.Result, error) {
	provider.invocations++
	return toolworldport.Result{Text: "dynamic-ok"}, nil
}

func (*fixtureWorldProvider) Close(context.Context) error { return nil }

func TestProductionMCPWorldIsComposedThroughToolHost(t *testing.T) {
	world := &fixtureWorldProvider{id: "mcp"}
	staged, err := bindToolWorlds(context.Background(), []toolworldport.Provider{world}, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(staged) != 1 {
		t.Fatalf("MCP world was skipped during production composition: %d staged entries", len(staged))
	}
	registry, err := bindGeneratedTools(nil, tools.NewRegistry(staged...))
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := registry.Lookup("mcp.docs.echo"); !ok {
		t.Fatalf("MCP dynamic tool did not enter the ToolHost catalog: %#v", registry.Specs())
	}
	if _, ok := registry.Lookup("mcp_call"); ok {
		t.Fatal("legacy mcp_call remained model-visible alongside the governed MCP ToolWorld")
	}
}

type governedRuntimeTool interface {
	tools.Tool
	GovernedToolID() string
}

func TestAssemblyRuntimeToolsAreSoleToolHostViews(t *testing.T) {
	world := &fixtureWorldProvider{}
	staged, err := bindToolWorlds(context.Background(), []toolworldport.Provider{world}, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if world.discoveries != 0 {
		t.Fatalf("bindToolWorlds discoveries = %d, want staging only", world.discoveries)
	}

	registry := tools.NewRegistry(fixtureLegacyTool{id: "legacy.echo", result: "legacy-ok"}).WithAdditional(staged...)
	governed, err := bindGeneratedTools(nil, registry)
	if err != nil {
		t.Fatal(err)
	}
	if world.discoveries != 1 {
		t.Fatalf("ToolHost discoveries = %d, want 1", world.discoveries)
	}

	for _, id := range []string{"legacy.echo", "remote.echo"} {
		bound, ok := governed.Lookup(id)
		if !ok {
			t.Fatalf("governed registry missing %s", id)
		}
		view, ok := bound.(governedRuntimeTool)
		if !ok {
			t.Fatalf("%s is %T, want ToolHost-governed runtime view", id, bound)
		}
		if view.GovernedToolID() != id {
			t.Fatalf("GovernedToolID = %q, want %q", view.GovernedToolID(), id)
		}
	}

	legacy, _ := governed.Lookup("legacy.echo")
	if result, err := legacy.InvokableRun(context.Background(), nil); err != nil || result != "legacy-ok" {
		t.Fatalf("legacy result = %q, err=%v", result, err)
	}
	dynamic, _ := governed.Lookup("remote.echo")
	if result, err := dynamic.InvokableRun(context.Background(), nil); err != nil || result != "dynamic-ok" {
		t.Fatalf("dynamic result = %q, err=%v", result, err)
	}
	if world.invocations != 1 {
		t.Fatalf("dynamic provider invocations = %d, want 1", world.invocations)
	}
}
