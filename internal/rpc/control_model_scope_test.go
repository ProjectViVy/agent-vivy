package rpc

import (
	"strings"
	"testing"

	"agent-vivy/internal/app/settings"
	"agent-vivy/internal/provider"
	"encoding/json"
)

// newScopeTestEnv mirrors the selectModel fixture: one vendor whose endpoint
// declares three models, plus a project root so pins are exercised.
func newScopeTestEnv(t *testing.T) (*controlTestEnv, string) {
	t.Helper()
	return newSettingsHandlerEnvWith(t, &settingsApplierProbe{}, func(deps *ControlDeps) {
		deps.ProjectRoot = "/work/alpha"
		deps.ProviderVendors = []provider.Vendor{{
			Name: "openai", DisplayName: "OpenAI",
			Endpoints: []provider.Endpoint{{
				Adapter: provider.AdapterOpenAICompletions, BaseURL: "https://api.openai.com/v1",
				DefaultModel: "gpt-4o-mini",
				Models:       []provider.Model{{ID: "gpt-4o-mini"}, {ID: "gpt-5"}, {ID: "gpt-5-mini"}},
			}},
		}}
	})
}

func scopeParam(provider, model string) map[string]any {
	return map[string]any{"provider": provider, "model": model}
}

func scopeResult(t *testing.T, result any) (view providersResult, scoped bool) {
	t.Helper()
	raw, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	var decoded struct {
		providersResult
		Scoped bool `json:"scoped"`
	}
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatal(err)
	}
	return decoded.providersResult, decoded.Scoped
}

func TestModelScopeTogglesLiveSelection(t *testing.T) {
	env, settingsPath := newScopeTestEnv(t)

	// Select a catalog model first so the live selection is the one being
	// scoped (the fixture service boots on a label, not a vendor model).
	if _, rpcErr := callControl(t, env.handler, "settings/model/select", scopeParam("openai", "gpt-4o-mini")); rpcErr != nil {
		t.Fatalf("select live model: %v", rpcErr)
	}
	// No params: the live selection is scoped.
	result, rpcErr := callControl(t, env.handler, "model/scope", nil)
	if rpcErr != nil {
		t.Fatalf("scope live selection: %v", rpcErr)
	}
	view, scoped := scopeResult(t, result)
	if !scoped {
		t.Fatal("first toggle must report scoped=true")
	}
	if len(view.ScopedModels) != 1 || view.ScopedModels[0].Model != "gpt-4o-mini" {
		t.Fatalf("scoped set = %+v, want the live selection", view.ScopedModels)
	}
	loaded, err := settings.Load(settingsPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.ScopedModels) != 1 || !loaded.ScopedModels[0].Matches(settings.ScopedModel{Provider: "openai", Model: "gpt-4o-mini"}) {
		t.Fatalf("scoped_models not persisted: %+v", loaded.ScopedModels)
	}

	// Second toggle removes it.
	result, rpcErr = callControl(t, env.handler, "model/scope", nil)
	if rpcErr != nil {
		t.Fatalf("unscope live selection: %v", rpcErr)
	}
	if _, scoped = scopeResult(t, result); scoped {
		t.Fatal("second toggle must report scoped=false")
	}
	loaded, err = settings.Load(settingsPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.ScopedModels) != 0 {
		t.Fatalf("scoped_models must be empty: %+v", loaded.ScopedModels)
	}
}

func TestModelCycleWalksDeclaredOrderAndWraps(t *testing.T) {
	env, settingsPath := newScopeTestEnv(t)

	for _, model := range []string{"gpt-4o-mini", "gpt-5", "gpt-5-mini"} {
		if _, rpcErr := callControl(t, env.handler, "model/scope", scopeParam("openai", model)); rpcErr != nil {
			t.Fatalf("scope %s: %v", model, rpcErr)
		}
	}
	// The cycle starts after the live selection (deepseek-flash, not in the
	// set), so it begins at the first declared entry.
	want := []string{"gpt-4o-mini", "gpt-5", "gpt-5-mini", "gpt-4o-mini"}
	for i, expected := range want {
		result, rpcErr := callControl(t, env.handler, "model/cycle", nil)
		if rpcErr != nil {
			t.Fatalf("cycle %d: %v", i, rpcErr)
		}
		view := result.(providersResult)
		if view.ActiveModel != expected || view.ActiveProvider != "openai" {
			t.Fatalf("cycle %d = %s/%s, want openai/%s", i, view.ActiveProvider, view.ActiveModel, expected)
		}
	}
	// The pin is written through the same transaction as the selection.
	loaded, err := settings.Load(settingsPath)
	if err != nil {
		t.Fatal(err)
	}
	pin, ok := loaded.ProjectDefault("/work/alpha")
	if !ok || pin.Model != "gpt-4o-mini" {
		t.Fatalf("project pin = %+v %t, want openai/gpt-4o-mini", pin, ok)
	}
}

func TestModelCycleSkipsUnavailableEntries(t *testing.T) {
	env, settingsPath := newScopeTestEnv(t)

	// Seed the set directly: one entry whose model left the catalog followed
	// by a valid one.
	if _, err := settings.Update(settingsPath, func(cur settings.Settings) (settings.Settings, error) {
		cur.ScopedModels = []settings.ScopedModel{
			{Provider: "openai", Model: "invented"},
			{Provider: "openai", Model: "gpt-5"},
		}
		return cur, nil
	}); err != nil {
		t.Fatal(err)
	}
	result, rpcErr := callControl(t, env.handler, "model/cycle", nil)
	if rpcErr != nil {
		t.Fatalf("cycle must skip the stale entry: %v", rpcErr)
	}
	if view := result.(providersResult); view.ActiveModel != "gpt-5" {
		t.Fatalf("cycle = %s, want gpt-5", view.ActiveModel)
	}
}

func TestModelCycleWithoutScopedSetFails(t *testing.T) {
	env, _ := newScopeTestEnv(t)
	if _, rpcErr := callControl(t, env.handler, "model/cycle", nil); rpcErr == nil || rpcErr.Code != InvalidParams {
		t.Fatalf("cycle with no scoped set = %v, want InvalidParams", rpcErr)
	}
	initResult, rpcErr := callControl(t, env.handler, "initialize", nil)
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	raw, err := json.Marshal(initResult)
	if err != nil {
		t.Fatal(err)
	}
	for _, capability := range []string{"model.scope", "model.cycle"} {
		if !strings.Contains(string(raw), capability) {
			t.Fatalf("capabilities missing %s: %s", capability, raw)
		}
	}
}
