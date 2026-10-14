package app

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"agent-vivy/internal/config"
	"agent-vivy/internal/domain"
	genassembly "agent-vivy/internal/generated/assembly"
	"agent-vivy/internal/runtime"
	"agent-vivy/internal/storage"
)

// The headless default composition declares its own form identity in the
// generated Assembly (HeadlessGenerationID). A binary built from it — the dev
// split pair included — must therefore arm masks, primary admission, and the
// action host through the same contracts as a packed build, with no injected
// identity and no legacy downgrade.
func TestHeadlessFormArmsMasksAndAdmissionWithoutInjectedIdentity(t *testing.T) {
	var once sync.Once
	entered := make(chan struct{})
	modelInput := make(chan string, 1)
	uninitializedInput := make(chan string, 1)
	model := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		if strings.Contains(string(raw), "Your personal identity has not been initialized.") {
			uninitializedInput <- string(raw)
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = io.WriteString(w, "data: {\"id\":\"first-arrival\",\"object\":\"chat.completion.chunk\",\"model\":\"deepseek-flash\",\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\",\"content\":\"我刚来到世界，我还不太了解自己。\"},\"finish_reason\":null}]}\n\ndata: {\"id\":\"first-arrival\",\"object\":\"chat.completion.chunk\",\"model\":\"deepseek-flash\",\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n")
			return
		}
		once.Do(func() { modelInput <- string(raw); close(entered) })
		<-r.Context().Done()
	}))
	t.Cleanup(model.Close)

	runtime.SetEngineVersionOverride(pinnedEinoVersion)
	t.Cleanup(func() { runtime.SetEngineVersionOverride("") })
	t.Setenv("DEEPSEEK_API_KEY", "headless-mask-arming-key")
	t.Setenv("VIVY_PROVIDER", "deepseek")
	t.Setenv("VIVY_API_BASE", model.URL)

	cfg := uninitializedDeepSeekTestConfig(t)
	cfg.Storage.SQLite.Path = filepath.Join(t.TempDir(), "headless-arming.db")
	// The product default governance profile must allow mask operations on
	// its own: module actions have no approval row, so without the shipped
	// vivy.masks.* allow rule every mask operation would die as
	// "module action is not authorized" in the default posture.
	cfg.Governance = config.Default().Governance

	assembly := genassembly.BuildDefault()
	if assembly.GenerationID != genassembly.HeadlessGenerationID {
		t.Fatalf("BuildDefault GenerationID = %q, want declared %q", assembly.GenerationID, genassembly.HeadlessGenerationID)
	}
	a, err := NewWithAssembly(context.Background(), cfg, assembly, WithoutEars(), WithoutGateway())
	if err != nil {
		t.Fatalf("compose headless app: %v", err)
	}
	t.Cleanup(func() { _ = a.Close() })
	if a.ActionHost() == nil {
		t.Fatal("headless form did not arm the action host")
	}

	client, err := a.DialControl(context.Background(), nil)
	if err != nil {
		t.Fatalf("dial control: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })

	init := callControl(t, client, "initialize", map[string]any{"protocol_version": "vivy.rpc.v1"})
	initJSON, err := json.Marshal(init)
	if err != nil {
		t.Fatalf("encode initialize result: %v", err)
	}
	if !strings.Contains(string(initJSON), "module.action.invoke") {
		t.Fatalf("initialize capabilities missing module.action.invoke: %s", initJSON)
	}

	session := callControl(t, client, "session/create", map[string]any{"title": "headless masks"})
	sessionID, _ := session["id"].(string)
	if sessionID == "" {
		t.Fatalf("session/create result = %v", session)
	}
	uninitializedTurn := callControl(t, client, "turn/start", map[string]any{"session_id": sessionID, "text": "Who are you?"})
	waitForAppRun(t, client, uninitializedTurn)
	select {
	case input := <-uninitializedInput:
		if strings.Contains(input, "You are Vivy") || !strings.Contains(input, "我刚来到世界，我还不太了解自己") {
			t.Fatal("fresh install used the default identity instead of first-arrival guidance")
		}
	default:
		t.Fatal("uninitialized conversation never reached the model")
	}
	initialized := callControl(t, client, "module.action.invoke", map[string]any{
		"module_id": "vivy/diva-cognitive",
		"action_id": "diva.cognitive.persona.initialize",
		"input": map[string]any{"session_id": sessionID, "initialization": map[string]string{
			"identity": "HEADLESS_PERSONA_SENTINEL", "relationship": "partner",
			"redline": "respect boundaries", "user": "preferences", "world": "WORLD_EXCLUDED_SENTINEL",
		}},
	})
	if initialized["status"] != "ok" {
		t.Fatalf("persona initialization failed: %v", initialized)
	}

	catalog := callControl(t, client, "module.action.invoke", map[string]any{
		"module_id": "vivy/masks",
		"action_id": "vivy.masks.catalog.list",
		"input":     map[string]any{},
	})
	items, _ := catalog["items"].([]any)
	if len(items) == 0 {
		t.Fatalf("mask catalog list returned no items: %v", catalog)
	}
	writerFound := false
	for _, item := range items {
		definition, _ := item.(map[string]any)
		if definition["id"] != "builtin/writer" {
			continue
		}
		writerFound = true
		if definition["generation_id"] != genassembly.HeadlessGenerationID || definition["built_in"] != true {
			t.Fatalf("builtin/writer = %v, want headless generation identity and built-in", definition)
		}
	}
	if !writerFound {
		t.Fatalf("mask catalog list is missing builtin/writer: %v", catalog)
	}

	selection := callControl(t, client, "module.action.invoke", map[string]any{
		"module_id": "vivy/masks",
		"action_id": "vivy.masks.selection.set",
		"input":     map[string]any{"session_id": sessionID, "mask_id": "builtin/writer", "expected_revision": 0},
	})
	if selection["mask_id"] != "builtin/writer" || selection["revision"] != float64(1) || selection["available"] != true {
		t.Fatalf("selection.set result = %v", selection)
	}

	// Admission persists the prompt snapshot before any model call, so the
	// hanging model endpoint only proves the snapshot was written first.
	turn := callControl(t, client, "turn/start", map[string]any{"session_id": sessionID, "text": "headless admission"})
	runID, _ := turn["run_id"].(string)
	if runID == "" {
		t.Fatalf("turn/start result = %v", turn)
	}
	select {
	case <-entered:
		input := <-modelInput
		if !strings.Contains(input, "HEADLESS_PERSONA_SENTINEL") || strings.Contains(input, "WORLD_EXCLUDED_SENTINEL") || strings.Contains(input, "You are Vivy") || strings.Contains(input, "Your personal identity has not been initialized.") {
			t.Fatalf("model request did not use the real persona projection: %s", input)
		}
	case <-time.After(30 * time.Second):
		t.Fatal("run never reached the model; admission likely failed")
	}

	store, ok := a.backend.(storage.RunAdmissionStore)
	if !ok {
		t.Fatal("headless backend lacks RunAdmissionStore")
	}
	snapshot, err := store.LoadRunPrompt(context.Background(), domain.RunID(runID))
	if err != nil {
		t.Fatalf("load prompt snapshot: %v", err)
	}
	if snapshot.GenerationID != genassembly.HeadlessGenerationID {
		t.Fatalf("snapshot generation id = %q, want %q", snapshot.GenerationID, genassembly.HeadlessGenerationID)
	}
	if !strings.Contains(string(snapshot.Payload), "builtin/writer") {
		t.Fatalf("snapshot payload does not carry the selected mask: %s", snapshot.Payload)
	}
	if !strings.Contains(string(snapshot.Payload), "HEADLESS_PERSONA_SENTINEL") {
		t.Fatal("persisted prompt omits the real persona")
	}
}
