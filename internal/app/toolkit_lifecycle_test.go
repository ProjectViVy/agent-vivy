package app

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agent-vivy/internal/config"
	genassembly "agent-vivy/internal/generated/assembly"
	controlrpc "agent-vivy/internal/rpc"
	actionport "agent-vivy/sdk/port/controlaction"
	toolworldport "agent-vivy/sdk/port/toolworld"
)

func TestToolkitLifecycle(t *testing.T) {
	cfg := newDeepSeekTestConfig(t)
	cfg.Governance = config.Default().Governance
	assembly := genassembly.BuildDefault()
	a, err := NewWithAssembly(context.Background(), cfg, assembly, WithoutEars(), WithoutGateway())
	if err != nil {
		t.Fatalf("compose app: %v", err)
	}
	client, err := a.DialControl(context.Background(), nil)
	if err != nil {
		_ = a.Close()
		t.Fatalf("dial control: %v", err)
	}
	callControl(t, client, "initialize", map[string]any{"protocol_version": "vivy.rpc.v1"})
	callControl(t, client, "session/create", map[string]any{"title": "toolkit lifecycle"})

	initial := toolkitAction(t, client, "vivy.bundles.list", map[string]any{})
	capabilities := toolkitObject(t, initial, "capabilities")
	owners := toolkitObject(t, capabilities, "owners")
	for _, owner := range []string{"skill", "mask", "rule", "hook", "mcp"} {
		if owners[owner] != true {
			t.Fatalf("default app owner %q capability = %v", owner, owners[owner])
		}
	}
	if items := toolkitArray(t, initial, "items"); len(items) != 0 {
		t.Fatalf("fresh toolkit catalog contains %d installations", len(items))
	}

	source := writeToolkitLifecycleFixture(t)
	preview := toolkitAction(t, client, "vivy.bundles.preview", map[string]any{
		"operation_id": "30000000-0000-4000-8000-000000000001", "expected_catalog": initial["catalog"], "location": source,
	})
	installed := toolkitAction(t, client, "vivy.bundles.install", map[string]any{
		"operation_id": "30000000-0000-4000-8000-000000000002", "expected_catalog": initial["catalog"],
		"preview_operation_id": preview["operation_id"], "expected_release_digest": preview["release_digest"],
	})
	_ = installed

	state := toolkitAction(t, client, "vivy.bundles.list", map[string]any{})
	items := toolkitArray(t, state, "items")
	if len(items) != 1 {
		t.Fatalf("after install catalog contains %d items: %v", len(items), items)
	}
	installation, ok := items[0].(map[string]any)
	if !ok || installation["state"] != "disabled" {
		t.Fatalf("install should be disabled: %v", items[0])
	}
	installID, _ := installation["install_id"].(string)
	if installID == "" {
		t.Fatalf("installation is missing ID: %v", installation)
	}

	enabled := prepareToolkitMutation(t, client, state, installation, "enable", "30000000-0000-4000-8000-000000000003")
	publishToolkitCandidate(t, client, enabled)
	state = toolkitAction(t, client, "vivy.bundles.list", map[string]any{})
	items = toolkitArray(t, state, "items")
	installation = items[0].(map[string]any)
	if installation["state"] != "enabled" {
		t.Fatalf("published enable state = %v", installation["state"])
	}

	detail := toolkitAction(t, client, "vivy.bundles.component.get", map[string]any{
		"install_id": installID, "kind": "skill", "local_id": "plan", "expected_release_digest": installation["release_digest"],
		"offset": 0, "max_bytes": 1024,
	})
	if detail["content"] != "---\ndescription: Plan work\n---\nPlan this release.\n" {
		t.Fatalf("captured Skill body = %q", detail["content"])
	}

	state = toolkitAction(t, client, "vivy.bundles.list", map[string]any{})
	installation = toolkitArray(t, state, "items")[0].(map[string]any)
	disabled := prepareToolkitMutation(t, client, state, installation, "disable", "30000000-0000-4000-8000-000000000004")
	publishToolkitCandidate(t, client, disabled)
	state = toolkitAction(t, client, "vivy.bundles.list", map[string]any{})
	installation = toolkitArray(t, state, "items")[0].(map[string]any)
	uninstall := prepareToolkitMutation(t, client, state, installation, "uninstall", "30000000-0000-4000-8000-000000000005")
	publishToolkitCandidate(t, client, uninstall)
	state = toolkitAction(t, client, "vivy.bundles.list", map[string]any{})
	installation = toolkitArray(t, state, "items")[0].(map[string]any)
	if installation["state"] != "orphaned" || installation["tombstone"] != true {
		t.Fatalf("uninstall did not retain an orphaned installation record: %v", installation)
	}

	if err := client.Close(); err != nil {
		t.Fatalf("close control client: %v", err)
	}
	if err := a.Close(); err != nil {
		t.Fatalf("close app: %v", err)
	}
	// Reopen the same durable database and prove uninstall history survives
	// app recovery without restoring the disabled content to the active view.
	a, err = NewWithAssembly(context.Background(), cfg, assembly, WithoutEars(), WithoutGateway())
	if err != nil {
		t.Fatalf("reopen app: %v", err)
	}
	t.Cleanup(func() { _ = a.Close() })
	client, err = a.DialControl(context.Background(), nil)
	if err != nil {
		t.Fatalf("dial reopened app: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })
	callControl(t, client, "initialize", map[string]any{"protocol_version": "vivy.rpc.v1"})
	callControl(t, client, "session/create", map[string]any{"title": "toolkit lifecycle recovered"})
	recovered := toolkitAction(t, client, "vivy.bundles.list", map[string]any{})
	items = toolkitArray(t, recovered, "items")
	if len(items) != 1 {
		t.Fatalf("recovered catalog contains %d installation records", len(items))
	}
	installation = items[0].(map[string]any)
	if installation["state"] != "orphaned" || installation["tombstone"] != true {
		t.Fatalf("recovered uninstall history = %v", installation)
	}
}

func TestToolkitOwnerOmissionIsReflectedByApp(t *testing.T) {
	cfg := newDeepSeekTestConfig(t)
	cfg.Governance = config.Default().Governance
	assembly := genassembly.BuildDefault()
	assembly.Manifest.Modules = filterToolkitValues(assembly.Manifest.Modules, func(value string) bool {
		return value != "vivy/masks" && value != "vivy/masks-ui" && value != "vivy/mcp-host"
	})
	assembly.MaskFactory = nil
	assembly.Manifest.Actions = filterToolkitValues(assembly.Manifest.Actions, func(value string) bool {
		return !strings.HasPrefix(value, "vivy.masks.")
	})
	assembly.ActionSets = filterToolkitValues(assembly.ActionSets, func(value actionport.ProviderSet) bool {
		return value.ModuleID != "vivy/masks"
	})
	assembly.Worlds = filterToolkitValues(assembly.Worlds, func(value toolworldport.Provider) bool {
		return value == nil || value.Definition().ID != "mcp"
	})
	delete(assembly.ToolWorldGrants, "mcp")
	assembly.Manifest.ToolWorlds = filterToolkitValues(assembly.Manifest.ToolWorlds, func(value string) bool { return value != "mcp" })

	a, err := NewWithAssembly(context.Background(), cfg, assembly, WithoutEars(), WithoutGateway())
	if err != nil {
		t.Fatalf("compose app without optional owners: %v", err)
	}
	t.Cleanup(func() { _ = a.Close() })
	client, err := a.DialControl(context.Background(), nil)
	if err != nil {
		t.Fatalf("dial control: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })
	callControl(t, client, "initialize", map[string]any{"protocol_version": "vivy.rpc.v1"})
	callControl(t, client, "session/create", map[string]any{"title": "toolkit owner omission"})

	state := toolkitAction(t, client, "vivy.bundles.list", map[string]any{})
	owners := toolkitObject(t, toolkitObject(t, state, "capabilities"), "owners")
	if owners["skill"] != true || owners["rule"] != true || owners["hook"] != true {
		t.Fatalf("omitting Mask/MCP changed unrelated owners: %v", owners)
	}
	if owners["mask"] != false || owners["mcp"] != false {
		t.Fatalf("omitted owner capability was advertised: %v", owners)
	}

	for _, fixture := range []struct {
		name     string
		manifest string
		files    map[string]string
	}{
		{
			name:     "mask",
			manifest: `{"apiVersion":"vivy.bundle/v1","id":"example/omitted-mask","version":"1.0.0","name":"Mask","components":{"masks":[{"id":"pm","name":"PM","path":"masks/pm.md"}]}}`,
			files:    map[string]string{"masks/pm.md": "# PM\n"},
		},
		{
			name:     "mcp",
			manifest: `{"apiVersion":"vivy.bundle/v1","id":"example/omitted-mcp","version":"1.0.0","name":"MCP","components":{"mcp":[{"id":"docs","endpoint":"https://example.invalid/mcp","args":[],"resourceBridge":false}]}}`,
		},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			source := t.TempDir()
			fixtureFiles := map[string]string{"vivy-bundle.json": fixture.manifest}
			for path, content := range fixture.files {
				fixtureFiles[path] = content
			}
			for path, content := range fixtureFiles {
				fullPath := filepath.Join(source, filepath.FromSlash(path))
				if err := os.MkdirAll(filepath.Dir(fullPath), 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(fullPath, []byte(content), 0600); err != nil {
					t.Fatal(err)
				}
			}
			_, err := client.Call(context.Background(), "module.action.invoke", map[string]any{
				"module_id": "vivy/toolkits", "action_id": "vivy.bundles.preview",
				"input": map[string]any{
					"operation_id":     "30000000-0000-4000-8000-00000000001" + map[string]string{"mask": "1", "mcp": "2"}[fixture.name],
					"expected_catalog": state["catalog"], "location": source,
				},
			})
			if err == nil {
				t.Fatalf("preview with omitted %s owner succeeded", fixture.name)
			}
		})
	}
}

func filterToolkitValues[T any](values []T, keep func(T) bool) []T {
	filtered := make([]T, 0, len(values))
	for _, value := range values {
		if keep(value) {
			filtered = append(filtered, value)
		}
	}
	return filtered
}

func toolkitAction(t *testing.T, client *controlrpc.Peer, actionID string, input map[string]any) map[string]any {
	t.Helper()
	return callControl(t, client, "module.action.invoke", map[string]any{
		"module_id": "vivy/toolkits", "action_id": actionID, "input": input,
	})
}

func toolkitObject(t *testing.T, source map[string]any, key string) map[string]any {
	t.Helper()
	value, ok := source[key].(map[string]any)
	if !ok {
		t.Fatalf("%s is not an object: %T (%v)", key, source[key], source[key])
	}
	return value
}

func toolkitArray(t *testing.T, source map[string]any, key string) []any {
	t.Helper()
	value, ok := source[key].([]any)
	if !ok {
		t.Fatalf("%s is not an array: %T (%v)", key, source[key], source[key])
	}
	return value
}

func prepareToolkitMutation(t *testing.T, client *controlrpc.Peer, state, installation map[string]any, kind, operationID string) map[string]any {
	t.Helper()
	return toolkitAction(t, client, "vivy.bundles."+kind, map[string]any{
		"operation_id": operationID, "expected_catalog": state["catalog"], "install_id": installation["install_id"],
		"expected_install_revision": installation["revision"],
	})
}

func publishToolkitCandidate(t *testing.T, client *controlrpc.Peer, prepared map[string]any) {
	t.Helper()
	operation := toolkitObject(t, prepared, "operation")
	toolkitAction(t, client, "vivy.bundles.operation.publish", map[string]any{
		"operation_id": operation["id"], "candidate_digest": prepared["candidate_digest"],
		"expected_catalog": prepared["expected_catalog"],
	})
}

func writeToolkitLifecycleFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	files := map[string]string{
		"skills/plan/SKILL.md": "---\ndescription: Plan work\n---\nPlan this release.\n",
		"masks/pm.md":          "# Product Manager\n",
		"vivy-bundle.json":     `{"apiVersion":"vivy.bundle/v1","id":"example/lifecycle","version":"1.0.0","name":"Lifecycle","components":{"skills":[{"id":"plan","path":"skills/plan/SKILL.md"}],"masks":[{"id":"pm","name":"Product Manager","path":"masks/pm.md","recommendedSkills":["plan"]}]}}`,
	}
	for relative, content := range files {
		path := filepath.Join(root, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	// Keep this guard close to the fixture: malformed JSON must fail during
	// test setup rather than obscure the action-level lifecycle assertion.
	var manifest map[string]any
	if err := json.Unmarshal([]byte(files["vivy-bundle.json"]), &manifest); err != nil {
		t.Fatal(err)
	}
	return root
}
