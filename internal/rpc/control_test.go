package rpc

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/eval"
	"agent-vivy/internal/events"
	"agent-vivy/internal/provider"
	"agent-vivy/internal/runtime"
	"agent-vivy/internal/storage/sqlite"
	"agent-vivy/internal/studio"
	"agent-vivy/internal/tools"
)

type controlTestEnv struct {
	backend *sqlite.Backend
	handler Handler
}

type childControllerStub struct{}

func (childControllerStub) StartChild(_ context.Context, request ChildRequest) (ChildResult, error) {
	return ChildResult{ID: "child-stub", ParentRunID: request.ParentRunID, Status: "active", Depth: 1}, nil
}

func (childControllerStub) GetChild(context.Context, string) (ChildResult, error) {
	return ChildResult{ID: "child-stub", Status: "completed", Result: "done"}, nil
}

func (childControllerStub) ListChildren(context.Context, string, bool) ([]ChildResult, error) {
	return []ChildResult{{ID: "child-stub", Status: "completed"}}, nil
}

func (childControllerStub) WaitChild(context.Context, string) (ChildResult, error) {
	return ChildResult{ID: "child-stub", Status: "completed", Result: "done"}, nil
}

func (childControllerStub) CancelChild(context.Context, string) (ChildResult, error) {
	return ChildResult{ID: "child-stub", Status: "cancelled"}, nil
}

func newControlTestEnv(t *testing.T) *controlTestEnv {
	t.Helper()
	ctx := context.Background()
	backend, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "rpc.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = backend.Close() })
	ts, err := tools.Builtin(backend).Resolve([]string{tools.EchoInfoName})
	if err != nil {
		t.Fatal(err)
	}
	engine, err := runtime.NewEngine(ctx, runtime.WrapModel(provider.NewMock()), ts, runtime.EngineConfig{
		StreamBuffer: 8, MaxEventPayloadBytes: 64 << 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	bus := events.NewBus(8)
	service := runtime.NewService(engine, "mock", "mock", runtime.ServiceDeps{
		Journal: backend, Runs: backend, Messages: backend, Approvals: backend, Questions: backend, Sink: bus,
	})
	liveTools := make([]domain.ToolSpec, 0, len(ts))
	for _, tool := range ts {
		liveTools = append(liveTools, tool.Spec())
	}
	handler, err := NewControlHandler(ControlDeps{
		Sessions: backend, Messages: backend, Runs: backend, Journal: backend,
		Approvals: backend, Questions: backend, Bus: bus, Service: service,
		Studio: studio.NewService(backend),
		Live: studio.LiveView{
			Provider:      "mock",
			PolicyProfile: domain.PolicyProfileDefault,
			PolicyHash:    "policy-hash-test",
			Tools:         liveTools,
		},
		Eval: eval.NewRunner(eval.Runner{
			Studio:     studio.NewService(backend),
			Executable: filepath.Join(t.TempDir(), "missing.exe"),
			EvalRoot:   filepath.Join(t.TempDir(), "evals"),
			Isolation: eval.Isolation{
				ProductionSQLite: filepath.Join(t.TempDir(), "prod.db"),
				BundleDir:        filepath.Join("..", "..", "fixtures", "provider"),
			},
		}),
		Children: childControllerStub{},
	})
	if err != nil {
		t.Fatal(err)
	}
	return &controlTestEnv{backend: backend, handler: handler}
}

func callControl(t *testing.T, handler Handler, method string, params any) (any, *Error) {
	t.Helper()
	encoded, err := json.Marshal(params)
	if err != nil {
		t.Fatal(err)
	}
	return handler.Handle(context.Background(), nil, Request{JSONRPC: "2.0", Method: method, Params: encoded})
}

func TestControlHandlerUsesVersionedSnakeCaseContracts(t *testing.T) {
	env := newControlTestEnv(t)
	result, rpcErr := callControl(t, env.handler, "initialize", nil)
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	initJSON, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	var init struct {
		ProtocolVersion string `json:"protocol_version"`
	}
	if err := json.Unmarshal(initJSON, &init); err != nil {
		t.Fatal(err)
	}
	if init.ProtocolVersion != ProtocolVersion {
		t.Fatalf("protocol version = %q", init.ProtocolVersion)
	}

	created, rpcErr := callControl(t, env.handler, "session/create", map[string]string{"title": "RPC"})
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	createdJSON, _ := json.Marshal(created)
	var session sessionResult
	if err := json.Unmarshal(createdJSON, &session); err != nil {
		t.Fatal(err)
	}
	if session.ID == "" || session.Title != "RPC" {
		t.Fatalf("session = %+v", session)
	}

	preflight, rpcErr := callControl(t, env.handler, "preflight/run", map[string]string{
		"session_id": string(session.ID), "text": "hello",
	})
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	preflightJSON, _ := json.Marshal(preflight)
	var preview preflightResult
	if err := json.Unmarshal(preflightJSON, &preview); err != nil {
		t.Fatal(err)
	}
	if preview.PolicyProfile == "" || preview.Status == "" {
		t.Fatalf("preflight = %+v", preview)
	}

	started, rpcErr := callControl(t, env.handler, "turn/start", map[string]string{
		"session_id": string(session.ID), "text": "hello",
	})
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	startedJSON, _ := json.Marshal(started)
	var accepted struct {
		RunID string `json:"run_id"`
	}
	if err := json.Unmarshal(startedJSON, &accepted); err != nil {
		t.Fatal(err)
	}
	if accepted.RunID == "" {
		t.Fatal("turn/start did not return run_id")
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		run, err := env.backend.GetRun(context.Background(), domain.RunID(accepted.RunID))
		if err == nil && run.Status.Terminal() {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	logResult, rpcErr := callControl(t, env.handler, "run/log", map[string]any{"run_id": accepted.RunID, "after_seq": 0})
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	logJSON, _ := json.Marshal(logResult)
	var logEnvelope struct {
		Events []eventResult `json:"events"`
	}
	if err := json.Unmarshal(logJSON, &logEnvelope); err != nil {
		t.Fatal(err)
	}
	if len(logEnvelope.Events) == 0 || logEnvelope.Events[0].RunID != domain.RunID(accepted.RunID) {
		t.Fatalf("run log = %+v", logEnvelope)
	}
}

func TestControlHandlerUnknownMethodAndInvalidParams(t *testing.T) {
	env := newControlTestEnv(t)
	if _, rpcErr := callControl(t, env.handler, "missing", nil); rpcErr == nil || rpcErr.Code != MethodNotFound {
		t.Fatalf("unknown method error = %+v", rpcErr)
	}
	if _, rpcErr := callControl(t, env.handler, "run/get", map[string]string{}); rpcErr == nil || rpcErr.Code != InvalidParams {
		t.Fatalf("invalid params error = %+v", rpcErr)
	}
}

func TestControlHandlerChildLifecycleContract(t *testing.T) {
	env := newControlTestEnv(t)
	started, rpcErr := callControl(t, env.handler, "child/start", ChildRequest{ParentRunID: "run-parent", Text: "delegate"})
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	var child ChildResult
	encoded, _ := json.Marshal(started)
	if err := json.Unmarshal(encoded, &child); err != nil || child.ID != "child-stub" || child.Status != "active" {
		t.Fatalf("child/start result = %+v, err = %v", child, err)
	}
	for _, method := range []string{"child/get", "child/wait", "child/cancel"} {
		result, rpcErr := callControl(t, env.handler, method, map[string]string{"run_id": child.ID})
		if rpcErr != nil {
			t.Fatalf("%s: %v", method, rpcErr)
		}
		if result == nil {
			t.Fatalf("%s returned nil", method)
		}
	}
	result, rpcErr := callControl(t, env.handler, "child/list", map[string]any{"parent_run_id": "run-parent", "tree": true})
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	if result == nil {
		t.Fatal("child/list returned nil")
	}
}

func TestStudioRPCEmptyListAndPromote(t *testing.T) {
	env := newControlTestEnv(t)
	listed, rpcErr := callControl(t, env.handler, "generations/list", map[string]any{})
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	raw, _ := json.Marshal(listed)
	var wrap struct {
		Generations []generationResult `json:"generations"`
	}
	if err := json.Unmarshal(raw, &wrap); err != nil {
		t.Fatal(err)
	}
	if wrap.Generations == nil || len(wrap.Generations) != 0 {
		t.Fatalf("empty generations = %#v", listed)
	}

	from, rpcErr := callControl(t, env.handler, "generations/create", map[string]any{
		"artifact_sha256": "aaa",
		"recipe":          map[string]any{"loop": "eino", "world": "sandbox"},
	})
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	fromJSON, _ := json.Marshal(from)
	var fromGen generationResult
	if err := json.Unmarshal(fromJSON, &fromGen); err != nil {
		t.Fatal(err)
	}
	to, rpcErr := callControl(t, env.handler, "generations/create", map[string]any{
		"parent_id":       fromGen.ID,
		"artifact_sha256": "bbb",
		"recipe":          map[string]any{"loop": "eino", "world": "sandbox", "plugins": []string{"acme"}},
	})
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	toJSON, _ := json.Marshal(to)
	var toGen generationResult
	if err := json.Unmarshal(toJSON, &toGen); err != nil {
		t.Fatal(err)
	}

	_, rpcErr = callControl(t, env.handler, "promotions/promote", map[string]any{
		"from_id": fromGen.ID, "to_id": toGen.ID,
	})
	if rpcErr == nil || rpcErr.Code != CodeConflict {
		t.Fatalf("promote without eval = %+v, want conflict", rpcErr)
	}

	if _, rpcErr = callControl(t, env.handler, "evals/record", map[string]any{
		"candidate_id": toGen.ID, "baseline_id": fromGen.ID, "suite": "s1", "verdict": "better",
	}); rpcErr != nil {
		t.Fatal(rpcErr)
	}
	first, rpcErr := callControl(t, env.handler, "promotions/promote", map[string]any{
		"from_id": fromGen.ID, "to_id": toGen.ID, "actor": "human",
	})
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	if first == nil {
		t.Fatal("promote returned nil")
	}
	_, rpcErr = callControl(t, env.handler, "promotions/promote", map[string]any{
		"from_id": fromGen.ID, "to_id": toGen.ID, "actor": "human",
	})
	if rpcErr == nil || rpcErr.Code != CodeConflict {
		t.Fatalf("second promote = %+v, want conflict", rpcErr)
	}
}

func TestSpeciesInspectBuiltinThenPromoted(t *testing.T) {
	env := newControlTestEnv(t)
	first, rpcErr := callControl(t, env.handler, "species/inspect", map[string]any{})
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	rep := decodeInspect(t, first)
	if rep.ProtocolVersion != ProtocolVersion {
		t.Fatalf("protocol_version = %q", rep.ProtocolVersion)
	}
	if rep.BinaryID == "" || rep.GenerationID != studio.BuiltinGenerationID {
		t.Fatalf("builtin inspect = %+v", rep)
	}
	if len(rep.Tools) == 0 {
		t.Fatal("expected live tools")
	}
	assertInspectSafe(t, first)

	from, rpcErr := callControl(t, env.handler, "generations/create", map[string]any{
		"artifact_sha256": "from-sha",
		"recipe":          map[string]any{"loop": "eino", "world": "sandbox"},
	})
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	fromGen := decodeGeneration(t, from)
	to, rpcErr := callControl(t, env.handler, "generations/create", map[string]any{
		"parent_id":       fromGen.ID,
		"artifact_sha256": "to-sha",
		"recipe":          map[string]any{"loop": "eino", "world": "sandbox", "tools": []string{"echo_info"}},
	})
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	toGen := decodeGeneration(t, to)
	if _, rpcErr = callControl(t, env.handler, "evals/record", map[string]any{
		"candidate_id": toGen.ID, "baseline_id": fromGen.ID, "suite": "s1", "verdict": "better",
	}); rpcErr != nil {
		t.Fatal(rpcErr)
	}
	if _, rpcErr = callControl(t, env.handler, "promotions/promote", map[string]any{
		"from_id": fromGen.ID, "to_id": toGen.ID, "actor": "human",
	}); rpcErr != nil {
		t.Fatal(rpcErr)
	}

	second, rpcErr := callControl(t, env.handler, "species/inspect", map[string]any{})
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	promoted := decodeInspect(t, second)
	if promoted.GenerationID != toGen.ID || promoted.ArtifactSHA256 != "to-sha" {
		t.Fatalf("promoted inspect = %+v, want generation %s", promoted, toGen.ID)
	}
	if promoted.Recipe.Loop != "eino" {
		t.Fatalf("promoted recipe = %+v", promoted.Recipe)
	}
	assertInspectSafe(t, second)
}

func TestGenerationsRejectRPC(t *testing.T) {
	env := newControlTestEnv(t)
	created, rpcErr := callControl(t, env.handler, "generations/create", map[string]any{
		"artifact_sha256": "cand",
		"recipe":          map[string]any{"loop": "eino", "world": "sandbox"},
	})
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	gen := decodeGeneration(t, created)
	rejected, rpcErr := callControl(t, env.handler, "generations/reject", map[string]any{"id": gen.ID})
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	got := decodeGeneration(t, rejected)
	if got.Phase != domain.GenerationRejected {
		t.Fatalf("rejected = %+v", got)
	}
	if _, rpcErr = callControl(t, env.handler, "generations/reject", map[string]any{"id": gen.ID}); rpcErr == nil || rpcErr.Code != CodeConflict {
		t.Fatalf("second reject = %+v", rpcErr)
	}
}

func TestEvalsStartUnknownSuiteAndFailedProbe(t *testing.T) {
	env := newControlTestEnv(t)
	created, rpcErr := callControl(t, env.handler, "generations/create", map[string]any{
		"artifact_sha256": "cand",
		"recipe":          map[string]any{"loop": "eino", "world": "sandbox"},
	})
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	gen := decodeGeneration(t, created)

	_, rpcErr = callControl(t, env.handler, "evals/start", map[string]any{
		"candidate_id": gen.ID, "suite": "not-a-suite",
	})
	if rpcErr == nil || rpcErr.Code != InvalidParams {
		t.Fatalf("unknown suite = %+v, want invalid params", rpcErr)
	}

	result, rpcErr := callControl(t, env.handler, "evals/start", map[string]any{
		"candidate_id": gen.ID, "suite": eval.SuiteAirgapProbe,
	})
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	raw, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	var got evalResult
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	if got.Verdict != domain.EvalFailedToRun || got.CandidateID != gen.ID {
		t.Fatalf("failed probe = %+v", got)
	}
}

func decodeInspect(t *testing.T, result any) studio.Report {
	t.Helper()
	raw, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	var rep studio.Report
	if err := json.Unmarshal(raw, &rep); err != nil {
		t.Fatal(err)
	}
	return rep
}

func decodeGeneration(t *testing.T, result any) generationResult {
	t.Helper()
	raw, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	var gen generationResult
	if err := json.Unmarshal(raw, &gen); err != nil {
		t.Fatal(err)
	}
	return gen
}

func assertInspectSafe(t *testing.T, result any) {
	t.Helper()
	raw, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	body := string(raw)
	if containsFold(body, "api_key") {
		t.Fatalf("inspect leaked api_key: %s", body)
	}
	for i := 0; i+2 < len(body); i++ {
		if ((body[i] >= 'A' && body[i] <= 'Z') || (body[i] >= 'a' && body[i] <= 'z')) && body[i+1] == ':' && (body[i+2] == '\\' || body[i+2] == '/') {
			t.Fatalf("inspect leaked host path: %s", body)
		}
	}
}

func containsFold(s, sub string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(sub))
}

func TestSettingsGetAndUpdate(t *testing.T) {
	ctx := context.Background()
	backend, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "rpc.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = backend.Close() })
	ts, err := tools.Builtin(backend).Resolve([]string{tools.EchoInfoName})
	if err != nil {
		t.Fatal(err)
	}
	engine, err := runtime.NewEngine(ctx, runtime.WrapModel(provider.NewMock()), ts, runtime.EngineConfig{
		StreamBuffer: 8, MaxEventPayloadBytes: 64 << 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	bus := events.NewBus(8)
	service := runtime.NewService(engine, "mock", "mock", runtime.ServiceDeps{
		Journal: backend, Runs: backend, Messages: backend, Approvals: backend, Questions: backend, Sink: bus,
	})
	settingsPath := filepath.Join(t.TempDir(), "agent-home", "settings.yaml")
	handler, err := NewControlHandler(ControlDeps{
		Sessions: backend, Messages: backend, Runs: backend, Journal: backend,
		Approvals: backend, Questions: backend, Bus: bus, Service: service,
		Studio:         studio.NewService(backend),
		SettingsPath:   settingsPath,
		ConfigProvider: "mock",
		ConfigModel:    "mock",
	})
	if err != nil {
		t.Fatal(err)
	}

	// Read-only path is rejected when no settings document is configured.
	roHandler, err := NewControlHandler(ControlDeps{
		Sessions: backend, Messages: backend, Runs: backend, Journal: backend,
		Approvals: backend, Questions: backend, Bus: bus, Service: service,
		Studio: studio.NewService(backend),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, rpcErr := callControl(t, roHandler, "settings/update", map[string]any{
		"provider": "openai",
	}); rpcErr == nil || rpcErr.Code != CodeConflict {
		t.Fatalf("expected conflict when settings path is empty, got %v", rpcErr)
	}

	// Initial get reports config defaults and no overlay.
	result, rpcErr := callControl(t, handler, "settings/get", nil)
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	get := result.(settingsResult)
	if get.ReadOnly {
		t.Fatal("settings should be writable when path is configured")
	}
	if get.ConfigProvider != "mock" {
		t.Fatalf("config_provider = %q, want mock", get.ConfigProvider)
	}

	// Invalid update is rejected (bad provider).
	if _, rpcErr := callControl(t, handler, "settings/update", map[string]any{
		"provider": "banana",
	}); rpcErr == nil {
		t.Fatal("expected invalid provider to be rejected")
	}

	// Valid update persists and is reflected on the next get.
	if _, rpcErr := callControl(t, handler, "settings/update", map[string]any{
		"provider":      "openai",
		"default_model": "gpt-4o",
		"base_url":      "https://gw.example.com/v1",
	}); rpcErr != nil {
		t.Fatal(rpcErr)
	}
	result, rpcErr = callControl(t, handler, "settings/get", nil)
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	get = result.(settingsResult)
	if get.Provider != "openai" || get.DefaultModel != "gpt-4o" || get.BaseURL != "https://gw.example.com/v1" {
		t.Fatalf("settings not persisted: %+v", get)
	}

	// Availability roster is always present and keyless providers are
	// reported configured.
	if len(get.NetworkSearch.Providers) == 0 {
		t.Fatalf("network_search providers roster missing: %+v", get.NetworkSearch)
	}
	keyless := map[string]bool{}
	for _, info := range get.NetworkSearch.Providers {
		if info.Name == "duckduckgo" || info.Name == "wikipedia" {
			keyless[info.Name] = info.Keyless && info.Configured
		}
	}
	if !keyless["duckduckgo"] || !keyless["wikipedia"] {
		t.Fatalf("keyless providers must be configured: %+v", get.NetworkSearch.Providers)
	}

	// Invalid network_search provider is rejected.
	if _, rpcErr := callControl(t, handler, "settings/update", map[string]any{
		"network_search": map[string]any{"provider": "alta vista"},
	}); rpcErr == nil {
		t.Fatal("expected invalid network_search provider to be rejected")
	}

	// network_search preference persists and is echoed back.
	if _, rpcErr := callControl(t, handler, "settings/update", map[string]any{
		"provider":       "openai",
		"network_search": map[string]any{"provider": "searxng"},
	}); rpcErr != nil {
		t.Fatal(rpcErr)
	}
	result, rpcErr = callControl(t, handler, "settings/get", nil)
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	get = result.(settingsResult)
	if get.NetworkSearch.Provider != "searxng" {
		t.Fatalf("network_search preference not persisted: %+v", get.NetworkSearch)
	}
}

func TestSettingsCapabilitiesAdvertised(t *testing.T) {
	env := newControlTestEnv(t)
	result, rpcErr := callControl(t, env.handler, "initialize", nil)
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	raw, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	if !containsFold(string(raw), "settings.get") || !containsFold(string(raw), "settings.update") {
		t.Fatalf("settings capabilities not advertised: %s", raw)
	}
}
