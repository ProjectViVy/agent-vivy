package app

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"agent-vivy/internal/config"
	"agent-vivy/internal/domain"
	"agent-vivy/internal/runtime"
	"agent-vivy/internal/toolhost"
	"agent-vivy/internal/tools"
)

func TestResolveToolExposurePrecedence(t *testing.T) {
	resolve := resolveToolExposure(config.Tools{
		Exposure: map[string]string{
			"direct.tool":   "direct",
			"hidden.tool":   "hidden",
			"invalid.level": "not-a-level",
			"glob.wins":     "deferred",
		},
		DeferredTools: []string{"listed.tool"},
	}, []runtime.MCPServerConfig{{
		Name: "docs",
		ToolExposure: map[string]string{
			"mcp.docs.secret_*": "hidden",
			"mcp.docs.*":        "deferred",
		},
	}})

	cases := []struct {
		name, serverID, want string
	}{
		{"direct.tool", "", "direct"},
		{"hidden.tool", "", "hidden"},
		{"invalid.level", "", ""},
		{"listed.tool", "", "deferred"},
		{"mcp.docs.secret_read", "docs", "hidden"},
		{"mcp.docs.search", "docs", "deferred"},
		{"other.tool", "docs", ""},
		{"mcp.docs.search", "other-server", ""},
	}
	for _, tc := range cases {
		if got := resolve(tc.name, tc.serverID); string(got) != tc.want {
			t.Fatalf("resolve(%q,%q) = %q, want %q", tc.name, tc.serverID, got, tc.want)
		}
	}
}

func TestGovernedToolExposureEnforcement(t *testing.T) {
	resolve := func(name, _ string) domain.ToolExposure {
		switch name {
		case "hidden.tool":
			return domain.ToolExposureHidden
		case "lazy.tool":
			return domain.ToolExposureDeferred
		case "open.tool":
			return domain.ToolExposureDirect
		}
		return domain.ToolExposureUnset
	}
	registry, err := bindGeneratedTools(nil, tools.NewRegistry(
		fixtureLegacyTool{id: "hidden.tool", result: "hidden-ok"},
		fixtureLegacyTool{id: "lazy.tool", result: "lazy-ok"},
		fixtureLegacyTool{id: "open.tool", result: "open-ok"},
	), resolve)
	if err != nil {
		t.Fatal(err)
	}
	lookup := func(name string) tools.Tool {
		t.Helper()
		tool, ok := registry.Lookup(name)
		if !ok {
			t.Fatalf("registry missing %s", name)
		}
		return tool
	}
	modelCtx := tools.WithToolCallID(context.Background(), "call-1")

	if _, err := lookup("hidden.tool").InvokableRun(modelCtx, json.RawMessage(`{}`)); !errors.Is(err, toolhost.ErrToolNotActive) {
		t.Fatalf("hidden model-path call error = %v, want tool_not_active", err)
	}
	// Internal callers (no bound tool_call identity) stay unrestricted.
	if result, err := lookup("hidden.tool").InvokableRun(context.Background(), json.RawMessage(`{}`)); err != nil || result != "hidden-ok" {
		t.Fatalf("hidden internal call = %q, %v", result, err)
	}
	// Deferred is disclosure-only: a model-side call still executes —
	// hidden is the only execution wall.
	if result, err := lookup("lazy.tool").InvokableRun(modelCtx, json.RawMessage(`{}`)); err != nil || result != "lazy-ok" {
		t.Fatalf("deferred model-path call = %q, %v", result, err)
	}
	activation := tools.NewToolActivation()
	activation.Activate("lazy.tool")
	activeCtx := tools.WithToolActivation(modelCtx, activation)
	if result, err := lookup("lazy.tool").InvokableRun(activeCtx, json.RawMessage(`{}`)); err != nil || result != "lazy-ok" {
		t.Fatalf("activated deferred call = %q, %v", result, err)
	}
	if result, err := lookup("open.tool").InvokableRun(modelCtx, json.RawMessage(`{}`)); err != nil || result != "open-ok" {
		t.Fatalf("unrestricted call = %q, %v", result, err)
	}
}
