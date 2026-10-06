package app

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"agent-vivy/internal/actionhost"
	"agent-vivy/internal/cognitivecontract"
	"agent-vivy/internal/config"
	"agent-vivy/internal/domain"
	genassembly "agent-vivy/internal/generated/assembly"
	divacognitive "agent-vivy/internal/modules/diva-cognitive"
	"agent-vivy/internal/runtime"
)

// Use the shipped factory, not a directly constructed DIVA test bundle.
func TestDefaultPersonaAuthorityPersistsAndFreezesPerSession(t *testing.T) {
	ctx := context.Background()
	assembly := genassembly.BuildDefault()
	if !assembly.HasCognitiveFactory() {
		t.Fatal("default/headless generation omits the persona authority")
	}
	cfg := config.Config{Storage: config.Storage{DataDir: t.TempDir()}}
	bundle, err := cognitiveBundleForAssembly(ctx, &assembly, cfg, assembly.GenerationID, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = bundle.Close() })
	dispatch := bundle.(cognitivecontract.DispatcherProvider).Dispatcher()
	invoke := func(id, input string) json.RawMessage {
		t.Helper()
		raw, err := dispatch.Invoke(ctx, id, json.RawMessage(input))
		if err != nil {
			t.Fatal(err)
		}
		var out struct {
			Status string          `json:"status"`
			Value  json.RawMessage `json:"value"`
		}
		if err := json.Unmarshal(raw, &out); err != nil || out.Status != "ok" {
			t.Fatalf("%s: %s (%v)", id, raw, err)
		}
		return out.Value
	}
	if _, err := bundle.Prepare(ctx, cognitivecontract.PrimaryContextInput{SessionID: "first"}); err == nil {
		t.Fatal("uninitialized authority silently admitted a conversation")
	}
	invoke(divacognitive.ActionPersonaInitialize, `{"session_id":"first","initialization":{"identity":"PERSONA_BEFORE","relationship":"partner","redline":"boundaries","user":"preferences","world":"WORLD_NOT_IN_PROMPT"}}`)
	first, err := bundle.Prepare(ctx, cognitivecontract.PrimaryContextInput{SessionID: "first"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(first.Text, "PERSONA_BEFORE") || strings.Contains(first.Text, "WORLD_NOT_IN_PROMPT") {
		t.Fatalf("wrong projection: %s", first.Text)
	}
	var doc struct {
		Revision uint64 `json:"revision"`
	}
	if err := json.Unmarshal(invoke(divacognitive.ActionPersonaRead, `{"session_id":"first","kind":"identity"}`), &doc); err != nil {
		t.Fatal(err)
	}
	input, _ := json.Marshal(map[string]any{"session_id": "first", "kind": "identity", "content": "PERSONA_AFTER", "base_revision": doc.Revision, "reason": "owner edit"})
	invoke(divacognitive.ActionPersonaSave, string(input))
	if err := bundle.Close(); err != nil {
		t.Fatal(err)
	}
	bundle, err = cognitiveBundleForAssembly(ctx, &assembly, cfg, assembly.GenerationID, nil)
	if err != nil {
		t.Fatal(err)
	}
	again, err := bundle.Prepare(ctx, cognitivecontract.PrimaryContextInput{SessionID: "first"})
	if err != nil {
		t.Fatal(err)
	}
	if again.Digest != first.Digest || again.Text != first.Text {
		t.Fatal("reopened session authority drifted")
	}
	next, err := bundle.Prepare(ctx, cognitivecontract.PrimaryContextInput{SessionID: "next"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(next.Text, "PERSONA_AFTER") {
		t.Fatal("new session did not capture saved authority")
	}
}

func TestDefaultPolicyAllowsPersonaEditsButNotEvolutionControl(t *testing.T) {
	authorize := actionAuthorize(policyEngine(config.Default()), domain.PolicyProfileDefault)
	for _, provider := range divacognitive.ActionProviders() {
		def := provider.Definition()
		err := authorize(context.Background(), actionhost.Identity{}, def, json.RawMessage(`{}`))
		switch def.ID {
		case divacognitive.ActionPersonaInitialize, divacognitive.ActionPersonaSave, divacognitive.ActionPersonaReviewDecide:
			if err != nil {
				t.Errorf("persona action %s denied: %v", def.ID, err)
			}
		case divacognitive.ActionPolicySet, divacognitive.ActionTrigger, divacognitive.ActionCancel:
			if err == nil {
				t.Errorf("unrequested evolution action %s was allowed", def.ID)
			}
		}
	}
}

func TestDefaultPersonaAppAcceptsRelativeDataPath(t *testing.T) {
	runtime.SetEngineVersionOverride(pinnedEinoVersion)
	t.Cleanup(func() { runtime.SetEngineVersionOverride("") })
	// Match config.example.yaml, while keeping all test data isolated.
	t.Chdir(t.TempDir())
	cfg := uninitializedDeepSeekTestConfig(t)
	cfg.Storage.DataDir = ""
	cfg.Storage.SQLite.Path = "data/vivy.db"
	a, err := New(context.Background(), cfg, WithoutEars())
	if err != nil {
		t.Fatalf("start default app with documented relative path: %v", err)
	}
	t.Cleanup(func() { _ = a.Close() })
}
