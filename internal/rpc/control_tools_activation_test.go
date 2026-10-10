package rpc

import (
	"encoding/json"
	"testing"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/tools"
)

// tools/activate flips a session-scoped deferred activation and
// tools/list projects exposure + activated state back to the caller.
func TestControlToolsActivateListProjection(t *testing.T) {
	env := newControlTestEnv(t, func(deps *ControlDeps) {
		deps.ToolCatalog = []domain.ToolSpec{
			{Name: tools.EchoInfoName, Description: "echo", Readonly: true},
			{Name: tools.ReadFileName, Description: "read", Readonly: true},
			{Name: "secret.tool", Description: "hidden", Exposure: domain.ToolExposureHidden},
		}
	})
	created, rpcErr := callControl(t, env.handler, "session/create", map[string]string{"title": "exposure"})
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	raw, _ := json.Marshal(created)
	var session sessionResult
	if err := json.Unmarshal(raw, &session); err != nil {
		t.Fatal(err)
	}
	list := func() []toolsCatalogEntry {
		result, rpcErr := callControl(t, env.handler, "tools/list", map[string]string{"session_id": string(session.ID)})
		if rpcErr != nil {
			t.Fatal(rpcErr)
		}
		data, _ := json.Marshal(result)
		var view toolsCatalogView
		if err := json.Unmarshal(data, &view); err != nil {
			t.Fatal(err)
		}
		return view.Tools
	}
	entry := func(entries []toolsCatalogEntry, name string) toolsCatalogEntry {
		t.Helper()
		for _, e := range entries {
			if e.Name == name {
				return e
			}
		}
		t.Fatalf("tools/list missing %s: %+v", name, entries)
		return toolsCatalogEntry{}
	}

	entries := list()
	if got := entry(entries, tools.EchoInfoName); got.Exposure != string(domain.ToolExposureDeferred) || got.Activated {
		t.Fatalf("echo_info = %+v, want deferred/unactivated", got)
	}
	if got := entry(entries, tools.ReadFileName); got.Exposure != string(domain.ToolExposureDirect) {
		t.Fatalf("read_file = %+v, want direct", got)
	}
	if got := entry(entries, "secret.tool"); got.Exposure != string(domain.ToolExposureHidden) {
		t.Fatalf("secret.tool = %+v, want hidden", got)
	}

	out, rpcErr := callControl(t, env.handler, "tools/activate", map[string]any{
		"session_id": string(session.ID), "ids": []string{tools.EchoInfoName, "secret.tool", "missing.tool"},
	})
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	outJSON, _ := json.Marshal(out)
	var outcomes struct {
		Tools map[string]string `json:"tools"`
	}
	if err := json.Unmarshal(outJSON, &outcomes); err != nil {
		t.Fatal(err)
	}
	if outcomes.Tools[tools.EchoInfoName] != "activated" || outcomes.Tools["secret.tool"] != "hidden" || outcomes.Tools["missing.tool"] != "unknown_tool" {
		t.Fatalf("activate outcomes = %v", outcomes.Tools)
	}
	if got := entry(list(), tools.EchoInfoName); !got.Activated {
		t.Fatalf("activated echo_info = %+v", got)
	}

	if _, rpcErr := callControl(t, env.handler, "tools/deactivate", map[string]any{
		"session_id": string(session.ID), "ids": []string{tools.EchoInfoName},
	}); rpcErr != nil {
		t.Fatal(rpcErr)
	}
	if got := entry(list(), tools.EchoInfoName); got.Activated {
		t.Fatalf("deactivated echo_info = %+v", got)
	}
}
