package rpc

import (
	"path/filepath"
	"testing"
)

// TestModelThinkingSetPersistAndReport drives the model/thinking verb:
// setting a level persists it in the settings overlay, an alias clears back
// to auto, and the report always answers with the resolved state.
func TestModelThinkingSetPersistAndReport(t *testing.T) {
	settingsPath := filepath.Join(t.TempDir(), "settings.yaml")
	env := newControlTestEnv(t, func(deps *ControlDeps) {
		deps.SettingsPath = settingsPath
	})

	// Read first: no overlay, the test model declares no thinking policy.
	result, rpcErr := callControl(t, env.handler, "model/thinking", nil)
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	report := result.(map[string]any)
	if report["thinking"] != "auto" || report["read_only"] == true {
		t.Fatalf("initial report = %v", report)
	}

	// An explicit level persists and echoes back.
	result, rpcErr = callControl(t, env.handler, "model/thinking", map[string]any{"level": "high"})
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	if report = result.(map[string]any); report["thinking"] != "high" || report["effective"] != "high" {
		t.Fatalf("set report = %v", report)
	}

	// The persisted preference survives: a bare call reports it.
	result, rpcErr = callControl(t, env.handler, "model/thinking", nil)
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	if report = result.(map[string]any); report["thinking"] != "high" {
		t.Fatalf("persisted report = %v", report)
	}

	// "auto" clears the overlay back to the empty preference.
	if _, rpcErr = callControl(t, env.handler, "model/thinking", map[string]any{"level": "auto"}); rpcErr != nil {
		t.Fatal(rpcErr)
	}
	result, rpcErr = callControl(t, env.handler, "model/thinking", nil)
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	if report = result.(map[string]any); report["thinking"] != "auto" {
		t.Fatalf("cleared report = %v", report)
	}
}

func TestModelThinkingRejectsUnknownLevelAndReadOnly(t *testing.T) {
	env := newControlTestEnv(t, func(deps *ControlDeps) {
		deps.SettingsPath = filepath.Join(t.TempDir(), "settings.yaml")
	})
	if _, rpcErr := callControl(t, env.handler, "model/thinking", map[string]any{"level": "brain"}); rpcErr == nil || rpcErr.Code != InvalidParams {
		t.Fatalf("unknown level: %v", rpcErr)
	}

	ro := newControlTestEnv(t)
	if _, rpcErr := callControl(t, ro.handler, "model/thinking", map[string]any{"level": "high"}); rpcErr == nil || rpcErr.Code != CodeConflict {
		t.Fatalf("read-only set: %v", rpcErr)
	}
}

// TestModelThinkingLevelsAnswersDeclaredSurface: the test model declares no
// thinking support, so the levels verb answers an empty list.
func TestModelThinkingLevelsAnswersDeclaredSurface(t *testing.T) {
	env := newControlTestEnv(t)
	result, rpcErr := callControl(t, env.handler, "model/thinking/levels", nil)
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	view := result.(map[string]any)
	if levels, ok := view["levels"].([]string); ok && len(levels) > 0 {
		t.Fatalf("non-thinking model reported levels: %v", levels)
	}
	if view["supports_thinking"] != false {
		t.Fatalf("supports_thinking = %v", view["supports_thinking"])
	}
}
