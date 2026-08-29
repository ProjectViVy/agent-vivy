package rpc

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"agent-vivy/internal/app/settings"
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
		Approvals: backend, Questions: backend, Todos: backend, Bus: bus, Service: service,
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
	if session.PermissionPreset != domain.PermissionPresetSmart || session.SandboxMode != domain.SandboxModeWorkspaceWrite {
		t.Fatalf("new session sandbox = %+v", session)
	}

	switched, rpcErr := callControl(t, env.handler, "session/set_permission", map[string]string{
		"session_id": string(session.ID), "preset": "cautious",
	})
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	switchedJSON, _ := json.Marshal(switched)
	if err := json.Unmarshal(switchedJSON, &session); err != nil {
		t.Fatal(err)
	}
	if session.PermissionPreset != domain.PermissionPresetCautious || session.SandboxMode != domain.SandboxModeReadOnly {
		t.Fatalf("switched session = %+v", session)
	}
	if _, rpcErr := callControl(t, env.handler, "session/set_permission", map[string]string{
		"session_id": string(session.ID), "preset": "custom",
	}); rpcErr == nil || rpcErr.Code != InvalidParams {
		t.Fatalf("custom preset error = %v", rpcErr)
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
		Studio:                         studio.NewService(backend),
		SettingsPath:                   settingsPath,
		ConfigProvider:                 "openai",
		ConfigModel:                    "gpt-4o-mini",
		ConfigNetworkSearchProvider:    "duckduckgo",
		ConfigExecuteMaxTimeoutSeconds: 30,
		DefaultPermissionPreset:        domain.PermissionPresetSmart,
		ConfigSandboxDenyPrivateIPs:    true,
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
	if get.ConfigProvider != "openai" {
		t.Fatalf("config_provider = %q, want openai", get.ConfigProvider)
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
	if get.APIKeySet {
		t.Fatal("api_key_set should be false before any key overlay")
	}
	// Network search section reflects the config default until overridden.
	if get.NetworkSearch.Provider != "" || get.NetworkSearch.ConfigProvider != "duckduckgo" {
		t.Fatalf("network_search defaults = %+v", get.NetworkSearch)
	}
	if len(get.NetworkSearch.Providers) != 5 {
		t.Fatalf("network_search provider roster = %d, want 5", len(get.NetworkSearch.Providers))
	}
	// Availability roster is always present and keyless providers are
	// reported configured.
	keyless := map[string]bool{}
	for _, info := range get.NetworkSearch.Providers {
		if info.Name == "duckduckgo" || info.Name == "wikipedia" {
			keyless[info.Name] = info.Keyless && info.Configured
		}
	}
	if !keyless["duckduckgo"] || !keyless["wikipedia"] {
		t.Fatalf("keyless providers must be configured: %+v", get.NetworkSearch.Providers)
	}
	// Execute ceiling: config fallback reported.
	if get.ConfigExecuteMaxTimeoutSeconds != 30 {
		t.Fatalf("config_execute_max_timeout_seconds = %d, want 30", get.ConfigExecuteMaxTimeoutSeconds)
	}
	if get.Sandbox.DefaultPreset != domain.PermissionPresetSmart || !get.Sandbox.DenyPrivateIPs {
		t.Fatalf("sandbox defaults = %+v", get.Sandbox)
	}

	// Update with an api_key overlay: the flag is set but the value is
	// never echoed back (settingsResult has no key field; JSON must too).
	if _, rpcErr := callControl(t, handler, "settings/update", map[string]any{
		"provider":      "openai",
		"default_model": "gpt-4o",
		"base_url":      "https://gw.example.com/v1",
		"api_key":       "sk-test-overlay",
	}); rpcErr != nil {
		t.Fatal(rpcErr)
	}
	result, rpcErr = callControl(t, handler, "settings/get", nil)
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	get = result.(settingsResult)
	if !get.APIKeySet {
		t.Fatal("api_key_set should be true after overlay save")
	}
	body, err := json.Marshal(get)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(body), "sk-test-overlay") {
		t.Fatalf("settings result leaked api_key value: %s", body)
	}

	// Update without api_key keeps the overlay; select does not clear keys.
	if _, rpcErr := callControl(t, handler, "settings/update", map[string]any{
		"provider": "openai",
	}); rpcErr != nil {
		t.Fatal(rpcErr)
	}
	result, rpcErr = callControl(t, handler, "settings/get", nil)
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	get = result.(settingsResult)
	if !get.APIKeySet {
		t.Fatal("api_key_set should stay true when select omits api_key")
	}

	// Execute ceiling: override persisted and echoed with the config
	// fallbacks; an out-of-bounds value is rejected without clobbering the
	// saved document.
	if _, rpcErr := callControl(t, handler, "settings/update", map[string]any{
		"provider":                    "openai",
		"default_model":               "gpt-4o",
		"base_url":                    "https://gw.example.com/v1",
		"execute_max_timeout_seconds": 300,
	}); rpcErr != nil {
		t.Fatal(rpcErr)
	}
	result, rpcErr = callControl(t, handler, "settings/get", nil)
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	get = result.(settingsResult)
	if get.ExecuteMaxTimeoutSeconds != 300 {
		t.Fatalf("execute_max_timeout_seconds not persisted: %+v", get)
	}
	if get.ConfigExecuteMaxTimeoutSeconds != 30 || get.ConfigProvider != "openai" || get.ConfigModel != "gpt-4o-mini" {
		t.Fatalf("update echo must include config fallbacks: %+v", get)
	}
	if _, rpcErr := callControl(t, handler, "settings/update", map[string]any{
		"execute_max_timeout_seconds": 601,
	}); rpcErr == nil {
		t.Fatal("expected execute_max_timeout_seconds above hard cap to be rejected")
	}
	result, rpcErr = callControl(t, handler, "settings/get", nil)
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	get = result.(settingsResult)
	if get.ExecuteMaxTimeoutSeconds != 300 {
		t.Fatalf("rejected update must not clobber the document: %+v", get)
	}

	// Network search preference persists: update with a provider, then the
	// next get echoes it back alongside the config default. An unsupported
	// provider is rejected (validation) without overwriting the saved one.
	if _, rpcErr := callControl(t, handler, "settings/update", map[string]any{
		"provider":       "openai",
		"network_search": map[string]any{"provider": "searxng"},
	}); rpcErr != nil {
		t.Fatal(rpcErr)
	}
	if _, rpcErr := callControl(t, handler, "settings/update", map[string]any{
		"provider":       "openai",
		"network_search": map[string]any{"provider": "yandex"},
	}); rpcErr == nil {
		t.Fatal("expected unsupported network_search provider to be rejected")
	}
	result, rpcErr = callControl(t, handler, "settings/get", nil)
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	get = result.(settingsResult)
	if get.NetworkSearch.Provider != "searxng" || get.NetworkSearch.ConfigProvider != "duckduckgo" {
		t.Fatalf("network_search after update = %+v", get.NetworkSearch)
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
	for _, method := range []string{
		"settings.providers", "settings.providers.upsert", "settings.providers.delete",
		"settings.mcp", "settings.mcp.upsert", "settings.mcp.delete", "settings.mcp.probe",
	} {
		if !containsFold(string(raw), method) {
			t.Fatalf("capability %s not advertised: %s", method, raw)
		}
	}
}

type mcpCatalogStub struct {
	listed   tools.MCPListResponse
	listErr  error
	replaced []runtime.MCPServerConfig
}

func (s *mcpCatalogStub) ListTools(context.Context, domain.RunID, string) (tools.MCPListResponse, error) {
	return s.listed, s.listErr
}

func (s *mcpCatalogStub) ReplaceServers(configs []runtime.MCPServerConfig) {
	s.replaced = append([]runtime.MCPServerConfig(nil), configs...)
}

func TestMCPSettingsCRUDAndProbe(t *testing.T) {
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
	catalog := &mcpCatalogStub{listed: tools.MCPListResponse{Tools: []tools.MCPTool{{Name: "echo"}}, Untrusted: true}}
	var changes int
	handler, err := NewControlHandler(ControlDeps{
		Sessions: backend, Messages: backend, Runs: backend, Journal: backend,
		Approvals: backend, Questions: backend, Bus: bus, Service: service,
		Studio:            studio.NewService(backend),
		SettingsPath:      filepath.Join(t.TempDir(), "settings.yaml"),
		MCP:               catalog,
		OnSettingsChanged: func() { changes++ },
	})
	if err != nil {
		t.Fatal(err)
	}

	listed, rpcErr := callControl(t, handler, "settings/mcp", nil)
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	if view := listed.(mcpListResult); len(view.Servers) != 0 || view.ReadOnly {
		t.Fatalf("empty overlay = %+v", view)
	}

	saved, rpcErr := callControl(t, handler, "settings/mcp/upsert", map[string]any{
		"name": "docs", "endpoint": "https://docs.example.com/mcp", "auth_env": "MCP_DOCS_TOKEN",
	})
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	entry := saved.(mcpServerResult)
	if entry.Name != "docs" || entry.Endpoint != "https://docs.example.com/mcp" || !entry.Enabled || entry.AuthEnv != "MCP_DOCS_TOKEN" {
		t.Fatalf("upsert result = %+v", entry)
	}
	if changes != 1 {
		t.Fatalf("OnSettingsChanged calls = %d, want 1", changes)
	}

	if _, rpcErr := callControl(t, handler, "settings/mcp/upsert", map[string]any{
		"name": "bad", "endpoint": "ftp://example.com/mcp",
	}); rpcErr == nil || rpcErr.Code != InvalidParams {
		t.Fatalf("expected invalid endpoint, got %v", rpcErr)
	}

	ro, err := NewControlHandler(ControlDeps{
		Sessions: backend, Messages: backend, Runs: backend, Journal: backend,
		Approvals: backend, Questions: backend, Bus: bus, Service: service,
		Studio: studio.NewService(backend),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, rpcErr := callControl(t, ro, "settings/mcp/upsert", map[string]any{
		"name": "docs", "endpoint": "https://docs.example.com/mcp",
	}); rpcErr == nil || rpcErr.Code != CodeConflict {
		t.Fatalf("expected read-only conflict, got %v", rpcErr)
	}

	probed, rpcErr := callControl(t, handler, "settings/mcp/probe", map[string]any{"name": "docs"})
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	probe := probed.(mcpServerResult)
	if probe.Status != "ok" || probe.ToolCount != 1 {
		t.Fatalf("probe = %+v", probe)
	}

	catalog.listErr = errMCPProbe
	failed, rpcErr := callControl(t, handler, "settings/mcp/probe", map[string]any{"name": "docs"})
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	if view := failed.(mcpServerResult); view.Status != "error" || view.Error == "" {
		t.Fatalf("failed probe = %+v", view)
	}

	if _, rpcErr := callControl(t, handler, "settings/mcp/delete", map[string]any{"name": "docs"}); rpcErr != nil {
		t.Fatal(rpcErr)
	}
	if changes != 2 {
		t.Fatalf("OnSettingsChanged after delete = %d, want 2", changes)
	}
	listed, rpcErr = callControl(t, handler, "settings/mcp", nil)
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	if view := listed.(mcpListResult); len(view.Servers) != 0 {
		t.Fatalf("delete did not empty overlay: %+v", view)
	}
	if _, rpcErr := callControl(t, handler, "settings/mcp/delete", map[string]any{"name": "docs"}); rpcErr == nil || rpcErr.Code != CodeNotFound {
		t.Fatalf("expected not found, got %v", rpcErr)
	}
}

var errMCPProbe = errString("remote down")

type errString string

func (e errString) Error() string { return string(e) }

// newSettingsHandlerEnv builds a control handler with a writable settings
// document under a temp dir and an OnSettingsChanged probe.
func newSettingsHandlerEnv(t *testing.T, probe *settingsApplierProbe) (*controlTestEnv, string) {
	return newSettingsHandlerEnvWith(t, probe, nil)
}

// newSettingsHandlerEnvWith is newSettingsHandlerEnv plus a hook to mutate
// ControlDeps before the handler is constructed (e.g. injecting the upstream
// model-list client for settings/providers/refresh tests).
func newSettingsHandlerEnvWith(t *testing.T, probe *settingsApplierProbe, mutate func(*ControlDeps)) (*controlTestEnv, string) {
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
	settingsPath := filepath.Join(t.TempDir(), "agent-home", "settings.yaml")
	deps := ControlDeps{
		Sessions: backend, Messages: backend, Runs: backend, Journal: backend,
		Approvals: backend, Questions: backend, Bus: bus, Service: service,
		Studio:                         studio.NewService(backend),
		SettingsPath:                   settingsPath,
		ConfigProvider:                 "openai",
		ConfigModel:                    "gpt-4o-mini",
		ConfigNetworkSearchProvider:    "duckduckgo",
		ConfigExecuteMaxTimeoutSeconds: 30,
	}
	if probe != nil {
		deps.OnSettingsChanged = func() { probe.n++ }
	}
	if mutate != nil {
		mutate(&deps)
	}
	handler, err := NewControlHandler(deps)
	if err != nil {
		t.Fatal(err)
	}
	return &controlTestEnv{backend: backend, handler: handler}, settingsPath
}

type settingsApplierProbe struct {
	n int
}

func TestProviderRegistryRPC(t *testing.T) {
	probe := &settingsApplierProbe{}
	env, settingsPath := newSettingsHandlerEnv(t, probe)
	ctx := context.Background()
	_ = ctx

	// Empty registry lists nothing with config defaults.
	result, rpcErr := callControl(t, env.handler, "settings/providers", nil)
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	view := result.(providersResult)
	if len(view.Entries) != 0 || view.ConfigProvider != "openai" || view.ReadOnly {
		t.Fatalf("empty registry view = %+v", view)
	}

	// Upsert an entry with api_key; the response is redacted.
	result, rpcErr = callControl(t, env.handler, "settings/providers/upsert", map[string]any{
		"id": "custom-1", "display_name": "My Gateway", "bundle": "openai",
		"base_url": "https://gateway.example.com/v1", "default_model": "deepseek-chat",
		"models": []string{"deepseek-chat", "deepseek-v4-pro"}, "api_key": "sk-entry",
	})
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	entry := result.(providerEntryResult)
	if entry.ID != "custom-1" || entry.DisplayName != "My Gateway" || !entry.APIKeySet {
		t.Fatalf("upsert echo = %+v", entry)
	}
	body, err := json.Marshal(entry)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(body), "sk-entry") {
		t.Fatalf("provider upsert response leaked api_key: %s", body)
	}

	// The document persisted the entry with the key.
	loaded, err := settings.Load(settingsPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Providers) != 1 || loaded.Providers[0].ApiKey != "sk-entry" {
		t.Fatalf("registry not persisted: %+v", loaded)
	}
	// Write-through applied the document to the env probe (key is inactive
	// so no env change, but the applier still saw the saved doc).
	if probe.n != 1 {
		t.Fatalf("OnSettingsChanged calls = %d, want 1", probe.n)
	}

	// List now reports the redacted entry.
	result, rpcErr = callControl(t, env.handler, "settings/providers", nil)
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	view = result.(providersResult)
	if len(view.Entries) != 1 || !view.Entries[0].APIKeySet || view.Entries[0].BaseURL != "https://gateway.example.com/v1" {
		t.Fatalf("registry list = %+v", view)
	}
	body, _ = json.Marshal(view)
	if strings.Contains(string(body), "sk-entry") {
		t.Fatalf("registry list leaked api_key: %s", body)
	}

	// Upsert an invalid entry (bad bundle) is rejected and does not persist.
	if _, rpcErr := callControl(t, env.handler, "settings/providers/upsert", map[string]any{
		"id": "custom-2", "display_name": "Bad", "bundle": "banana", "base_url": "https://bad.example.com/v1",
	}); rpcErr == nil {
		t.Fatal("expected invalid bundle to be rejected")
	}
	loaded, _ = settings.Load(settingsPath)
	if len(loaded.Providers) != 1 {
		t.Fatalf("rejected upsert must not persist: %+v", loaded)
	}

	// Duplicate (bundle, base_url) is rejected.
	if _, rpcErr := callControl(t, env.handler, "settings/providers/upsert", map[string]any{
		"id": "custom-2", "display_name": "Dup", "bundle": "openai", "base_url": "https://gateway.example.com/v1",
	}); rpcErr == nil {
		t.Fatal("expected duplicate (bundle, base_url) to be rejected")
	}

	// Delete removes the entry and the write-through applies the cleared doc.
	if _, rpcErr := callControl(t, env.handler, "settings/providers/delete", map[string]any{"id": "custom-1"}); rpcErr != nil {
		t.Fatal(rpcErr)
	}
	result, rpcErr = callControl(t, env.handler, "settings/providers", nil)
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	view = result.(providersResult)
	if len(view.Entries) != 0 {
		t.Fatalf("registry after delete = %+v", view)
	}
	if probe.n != 2 {
		t.Fatalf("OnSettingsChanged calls = %d, want 2 (upsert + delete; rejected upserts are no-ops)", probe.n)
	}

	// Deleting a missing entry is a not-found error.
	if _, rpcErr := callControl(t, env.handler, "settings/providers/delete", map[string]any{"id": "nope"}); rpcErr == nil || rpcErr.Code != CodeNotFound {
		t.Fatalf("expected not-found on missing delete, got %v", rpcErr)
	}

	// Read-only deployment rejects provider writes.
	roEnv := newControlTestEnv(t)
	if _, rpcErr := callControl(t, roEnv.handler, "settings/providers/upsert", map[string]any{
		"id": "custom-1", "display_name": "A", "bundle": "openai", "base_url": "https://a.example.com/v1",
	}); rpcErr == nil || rpcErr.Code != CodeConflict {
		t.Fatalf("expected conflict on read-only upsert, got %v", rpcErr)
	}
}

// refreshUpstreamServer serves an OpenAI-compatible /models payload and
// records the Authorization header + paths it received.
func refreshUpstreamServer(t *testing.T, status int, payload any) (*httptest.Server, *string) {
	t.Helper()
	var auth string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		if payload != nil {
			_ = json.NewEncoder(w).Encode(payload)
		}
	}))
	t.Cleanup(ts.Close)
	return ts, &auth
}

func TestProviderRefreshRPC(t *testing.T) {
	upstream, auth := refreshUpstreamServer(t, http.StatusOK, map[string]any{
		"data": []map[string]any{
			{"id": "upstream-a"},
			{"id": "upstream-b"},
		},
	})
	client := &provider.ModelListClient{HTTP: upstream.Client()}
	probe := &settingsApplierProbe{}
	env, settingsPath := newSettingsHandlerEnvWith(t, probe, func(deps *ControlDeps) {
		deps.ModelLists = client
	})

	// Seed a registry entry with a key and a manually added model.
	if _, rpcErr := callControl(t, env.handler, "settings/providers/upsert", map[string]any{
		"id": "custom-1", "display_name": "My Gateway", "bundle": "openai",
		"base_url": upstream.URL, "default_model": "upstream-a",
		"models": []string{"manual-a"}, "api_key": "sk-entry",
	}); rpcErr != nil {
		t.Fatal(rpcErr)
	}

	// Wire contract: an entry with no models yet must serialize models as []
	// — never null — so the UI validator keeps the fresh entry visible.
	emptyResult, rpcErr := callControl(t, env.handler, "settings/providers/upsert", map[string]any{
		"id": "custom-empty", "display_name": "No Models Yet", "bundle": "openai",
		"base_url": "https://empty.example.com/v1", "models": []string{},
	})
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	emptyBody, err := json.Marshal(emptyResult)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(emptyBody), `"models":null`) {
		t.Fatalf("empty models must serialize as [], got null: %s", emptyBody)
	}
	if !strings.Contains(string(emptyBody), `"models":[]`) {
		t.Fatalf("empty models must serialize as []: %s", emptyBody)
	}
	// The empty entry must appear in the list view (models [] on the wire).
	listResult, rpcErr := callControl(t, env.handler, "settings/providers", nil)
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	listView := listResult.(providersResult)
	if len(listView.Entries) != 2 || listView.Entries[1].ID != "custom-empty" || listView.Entries[1].Models == nil {
		t.Fatalf("list view must include the empty-models entry: %+v", listView)
	}
	// Clean up the scratch entry so later assertions count only custom-1.
	if _, rpcErr := callControl(t, env.handler, "settings/providers/delete", map[string]any{"id": "custom-empty"}); rpcErr != nil {
		t.Fatal(rpcErr)
	}

	// Refresh by id: upstream ids first, the manual extra preserved.
	result, rpcErr := callControl(t, env.handler, "settings/providers/refresh", map[string]any{"id": "custom-1"})
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	entry := result.(providerEntryResult)
	wantModels := []string{"upstream-a", "upstream-b", "manual-a"}
	if len(entry.Models) != len(wantModels) {
		t.Fatalf("refreshed models = %v, want %v", entry.Models, wantModels)
	}
	for i, m := range wantModels {
		if entry.Models[i] != m {
			t.Fatalf("refreshed models = %v, want %v", entry.Models, wantModels)
		}
	}
	if !entry.APIKeySet {
		t.Fatalf("refresh must keep the key set flag: %+v", entry)
	}
	body, err := json.Marshal(entry)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(body), "sk-entry") {
		t.Fatalf("refresh response leaked api_key: %s", body)
	}
	if *auth != "Bearer sk-entry" {
		t.Fatalf("upstream authorization = %q, want Bearer sk-entry", *auth)
	}
	loaded, err := settings.Load(settingsPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Providers) != 1 || loaded.Providers[0].ApiKey != "sk-entry" {
		t.Fatalf("refresh must not clear the registry api_key: %+v", loaded)
	}
	if len(loaded.Providers[0].Models) != len(wantModels) {
		t.Fatalf("persisted models = %v, want %v", loaded.Providers[0].Models, wantModels)
	}
	if probe.n != 4 {
		t.Fatalf("OnSettingsChanged calls = %d, want 4 (2 upserts + delete + refresh)", probe.n)
	}

	// Upstream failure: no row is created, the registry stays untouched.
	failing, _ := refreshUpstreamServer(t, http.StatusInternalServerError, nil)
	if _, rpcErr := callControl(t, env.handler, "settings/providers/refresh", map[string]any{
		"bundle": "openai", "base_url": failing.URL,
	}); rpcErr == nil || rpcErr.Code != InternalError {
		t.Fatalf("expected internal error on upstream failure, got %v", rpcErr)
	}
	loaded, _ = settings.Load(settingsPath)
	if len(loaded.Providers) != 1 {
		t.Fatalf("failed refresh must not create a registry row: %+v", loaded)
	}
	if probe.n != 4 {
		t.Fatalf("failed refresh must not notify: %d", probe.n)
	}

	// Catalog vendor with no registry row is cloned into the registry.
	result, rpcErr = callControl(t, env.handler, "settings/providers/refresh", map[string]any{
		"bundle": "openai", "base_url": upstream.URL + "/v1", "display_name": "Catalog Gateway", "default_model": "upstream-a",
	})
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	clone := result.(providerEntryResult)
	if clone.ID == "" || clone.ID == "custom-1" || clone.DisplayName != "Catalog Gateway" {
		t.Fatalf("cloned entry = %+v", clone)
	}
	if len(clone.Models) != 2 || clone.Models[0] != "upstream-a" {
		t.Fatalf("cloned models = %v", clone.Models)
	}
	if *auth != "" {
		t.Fatalf("keyless clone must not send an authorization header, got %q", *auth)
	}
	loaded, _ = settings.Load(settingsPath)
	if len(loaded.Providers) != 2 {
		t.Fatalf("clone must persist a second entry: %+v", loaded)
	}
	if probe.n != 5 {
		t.Fatalf("OnSettingsChanged calls = %d, want 5", probe.n)
	}

	// Unknown id is a not-found error.
	if _, rpcErr := callControl(t, env.handler, "settings/providers/refresh", map[string]any{"id": "nope"}); rpcErr == nil || rpcErr.Code != CodeNotFound {
		t.Fatalf("expected not-found on missing id, got %v", rpcErr)
	}

	// Anthropic-native providers are rejected, by bundle and by entry id.
	if _, rpcErr := callControl(t, env.handler, "settings/providers/refresh", map[string]any{
		"bundle": "anthropic", "base_url": "https://api.anthropic.com",
	}); rpcErr == nil || rpcErr.Code != InvalidParams {
		t.Fatalf("expected anthropic bundle to be rejected, got %v", rpcErr)
	}
	if _, rpcErr := callControl(t, env.handler, "settings/providers/upsert", map[string]any{
		"id": "custom-2", "display_name": "Anthropic", "bundle": "anthropic",
		"base_url": "https://api.anthropic.com", "models": []string{"claude-opus-4"},
	}); rpcErr != nil {
		t.Fatal(rpcErr)
	}
	if _, rpcErr := callControl(t, env.handler, "settings/providers/refresh", map[string]any{"id": "custom-2"}); rpcErr == nil || rpcErr.Code != InvalidParams {
		t.Fatalf("expected anthropic entry to be rejected, got %v", rpcErr)
	}

	// Read-only deployment rejects refresh.
	roEnv := newControlTestEnv(t)
	if _, rpcErr := callControl(t, roEnv.handler, "settings/providers/refresh", map[string]any{"id": "custom-1"}); rpcErr == nil || rpcErr.Code != CodeConflict {
		t.Fatalf("expected conflict on read-only refresh, got %v", rpcErr)
	}

	// Frozen (ENV-locked) session rejects refresh.
	frozenEnv, _ := newSettingsHandlerEnvWith(t, &settingsApplierProbe{}, func(deps *ControlDeps) {
		deps.Frozen = true
	})
	if _, rpcErr := callControl(t, frozenEnv.handler, "settings/providers/refresh", map[string]any{"id": "custom-1"}); rpcErr == nil || rpcErr.Code != CodeConflict {
		t.Fatalf("expected conflict on frozen refresh, got %v", rpcErr)
	}

	// The capabilities contract advertises the refresh method.
	result, rpcErr = callControl(t, env.handler, "initialize", nil)
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	initJSON, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(initJSON), "settings.providers.refresh") {
		t.Fatalf("capabilities missing settings.providers.refresh: %s", initJSON)
	}
}

func TestSettingsUpdatePreservesRegistry(t *testing.T) {
	probe := &settingsApplierProbe{}
	env, settingsPath := newSettingsHandlerEnv(t, probe)

	// Register a provider, then select it plus its model without repeating
	// the key. The registry entry must survive the update.
	if _, rpcErr := callControl(t, env.handler, "settings/providers/upsert", map[string]any{
		"id": "custom-1", "display_name": "My Gateway", "bundle": "openai",
		"base_url": "https://gateway.example.com/v1", "default_model": "deepseek-chat",
		"models": []string{"deepseek-chat"}, "api_key": "sk-entry",
	}); rpcErr != nil {
		t.Fatal(rpcErr)
	}
	if _, rpcErr := callControl(t, env.handler, "settings/update", map[string]any{
		"provider": "openai", "default_model": "deepseek-chat", "base_url": "https://gateway.example.com/v1",
	}); rpcErr != nil {
		t.Fatal(rpcErr)
	}
	loaded, err := settings.Load(settingsPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Providers) != 1 {
		t.Fatalf("update must preserve the registry: %+v", loaded)
	}
	if loaded.Provider != "openai" || loaded.DefaultModel != "deepseek-chat" {
		t.Fatalf("active selection not saved: %+v", loaded)
	}
	// The resolved active key now comes from the registry entry.
	if key := settings.ActiveKey(loaded, "openai", "https://gateway.example.com/v1"); key != "sk-entry" {
		t.Fatalf("active key resolution = %q, want sk-entry", key)
	}
	// The write-through env probe saw a doc whose active key resolves.
	if probe.n != 2 {
		t.Fatalf("OnSettingsChanged calls = %d, want 2", probe.n)
	}
}

func TestControlHandlerListsSessionTodos(t *testing.T) {
	env := newControlTestEnv(t)
	created, rpcErr := callControl(t, env.handler, "session/create", map[string]string{"title": "Todos"})
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	createdJSON, err := json.Marshal(created)
	if err != nil {
		t.Fatal(err)
	}
	var session sessionResult
	if err := json.Unmarshal(createdJSON, &session); err != nil {
		t.Fatal(err)
	}

	empty, rpcErr := callControl(t, env.handler, "session/todos", map[string]string{"session_id": string(session.ID)})
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	emptyJSON, err := json.Marshal(empty)
	if err != nil {
		t.Fatal(err)
	}
	var emptyOut struct {
		Todos []todoResult `json:"todos"`
	}
	if err := json.Unmarshal(emptyJSON, &emptyOut); err != nil {
		t.Fatal(err)
	}
	if emptyOut.Todos == nil || len(emptyOut.Todos) != 0 {
		t.Fatalf("empty todos = %+v, want []", emptyOut.Todos)
	}

	now := time.Now().UnixMilli()
	if err := env.backend.CreateTodo(context.Background(), domain.Todo{
		ID: "1", SessionID: session.ID, Subject: "wire rpc", Description: "list session todos",
		Status: domain.TodoInProgress, Blocks: []string{}, BlockedBy: []string{},
		ActiveForm: "Wiring RPC", Position: 0, CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatal(err)
	}

	listed, rpcErr := callControl(t, env.handler, "session/todos", map[string]string{"session_id": string(session.ID)})
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	listedJSON, err := json.Marshal(listed)
	if err != nil {
		t.Fatal(err)
	}
	var out struct {
		Todos []todoResult `json:"todos"`
	}
	if err := json.Unmarshal(listedJSON, &out); err != nil {
		t.Fatal(err)
	}
	if len(out.Todos) != 1 {
		t.Fatalf("todos = %+v", out.Todos)
	}
	item := out.Todos[0]
	if item.ID != "1" || item.Subject != "wire rpc" || item.Status != domain.TodoInProgress || item.ActiveForm != "Wiring RPC" {
		t.Fatalf("todo = %+v", item)
	}
	if item.SessionID != session.ID {
		t.Fatalf("session_id = %q, want %q", item.SessionID, session.ID)
	}

	if _, rpcErr := callControl(t, env.handler, "session/todos", map[string]string{"session_id": "missing"}); rpcErr == nil || rpcErr.Code != CodeNotFound {
		t.Fatalf("missing session error = %v", rpcErr)
	}
	if _, rpcErr := callControl(t, env.handler, "session/todos", map[string]string{}); rpcErr == nil || rpcErr.Code != InvalidParams {
		t.Fatalf("missing session_id error = %v", rpcErr)
	}

	unwiredBus := events.NewBus(8)
	unwired, err := NewControlHandler(ControlDeps{
		Sessions: env.backend, Messages: env.backend, Runs: env.backend, Journal: env.backend,
		Approvals: env.backend, Questions: env.backend, Bus: unwiredBus,
		Service: runtime.NewService(nil, "mock", "mock", runtime.ServiceDeps{
			Journal: env.backend, Runs: env.backend, Messages: env.backend, Approvals: env.backend, Questions: env.backend, Sink: unwiredBus,
		}),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, rpcErr := callControl(t, unwired, "session/todos", map[string]string{"session_id": string(session.ID)}); rpcErr == nil || rpcErr.Code != MethodNotFound {
		t.Fatalf("unwired todos error = %v", rpcErr)
	}
}

func TestControlHandlerSkillsCatalog(t *testing.T) {
	env := newControlTestEnv(t)
	if _, rpcErr := callControl(t, env.handler, "skills/list", nil); rpcErr == nil || rpcErr.Code != MethodNotFound {
		t.Fatalf("unwired skills list error = %v", rpcErr)
	}

	root := filepath.Join(t.TempDir(), "skills")
	dir := filepath.Join(root, "demo-skill")
	if err := os.MkdirAll(filepath.Join(dir, "references"), 0o700); err != nil {
		t.Fatal(err)
	}
	doc := "---\nname: demo-skill\ndescription: A test skill\n---\n\nUse this carefully.\n"
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(doc), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "references", "guide.md"), []byte("reference content"), 0o600); err != nil {
		t.Fatal(err)
	}
	backend, err := runtime.NewEinoSkillBackend(root, env.backend)
	if err != nil {
		t.Fatal(err)
	}
	wired, err := NewControlHandler(ControlDeps{
		Sessions: env.backend, Messages: env.backend, Runs: env.backend, Journal: env.backend,
		Approvals: env.backend, Questions: env.backend, Todos: env.backend, Skills: backend,
		Bus: events.NewBus(8), Service: runtime.NewService(nil, "mock", "mock", runtime.ServiceDeps{
			Journal: env.backend, Runs: env.backend, Messages: env.backend, Approvals: env.backend, Questions: env.backend, Sink: events.NewBus(8),
		}),
	})
	if err != nil {
		t.Fatal(err)
	}

	listed, rpcErr := callControl(t, wired, "skills/list", nil)
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	listedJSON, _ := json.Marshal(listed)
	var catalog struct {
		Skills []skillSummaryResult `json:"skills"`
	}
	if err := json.Unmarshal(listedJSON, &catalog); err != nil {
		t.Fatal(err)
	}
	if len(catalog.Skills) != 1 || catalog.Skills[0].Name != "demo-skill" || catalog.Skills[0].Hash == "" {
		t.Fatalf("skills = %+v", catalog.Skills)
	}

	viewed, rpcErr := callControl(t, wired, "skills/get", map[string]string{"name": "demo-skill"})
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	viewedJSON, _ := json.Marshal(viewed)
	var view skillViewResult
	if err := json.Unmarshal(viewedJSON, &view); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(view.Content, "Use this carefully") || view.RelativePath != "SKILL.md" {
		t.Fatalf("skill view = %+v", view)
	}

	ref, rpcErr := callControl(t, wired, "skills/get", map[string]string{"name": "demo-skill", "path": "references/guide.md"})
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	refJSON, _ := json.Marshal(ref)
	var refView skillViewResult
	if err := json.Unmarshal(refJSON, &refView); err != nil {
		t.Fatal(err)
	}
	if refView.Content != "reference content" {
		t.Fatalf("supporting view = %+v", refView)
	}

	if _, rpcErr := callControl(t, wired, "skills/get", map[string]string{}); rpcErr == nil || rpcErr.Code != InvalidParams {
		t.Fatalf("missing name error = %v", rpcErr)
	}
	if _, rpcErr := callControl(t, wired, "skills/get", map[string]string{"name": "demo-skill", "path": "../SKILL.md"}); rpcErr == nil {
		t.Fatal("path traversal should fail")
	}
	if _, rpcErr := callControl(t, wired, "skills/get", map[string]string{"name": "missing"}); rpcErr == nil || rpcErr.Code != CodeNotFound {
		t.Fatalf("missing skill error = %v", rpcErr)
	}

	initResult, rpcErr := callControl(t, env.handler, "initialize", nil)
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	raw, _ := json.Marshal(initResult)
	if !containsFold(string(raw), "skills.list") || !containsFold(string(raw), "skills.get") {
		t.Fatalf("skills capabilities not advertised: %s", raw)
	}
}

// TestContextCompactionRPC covers the real surface end to end: compaction
// settings persist via settings/update and round-trip through settings/get
// with config fallbacks; session/context reports real pressure; context/compact
// performs one durable session compaction.
func TestContextCompactionRPC(t *testing.T) {
	ctx := context.Background()
	backend, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "compaction.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = backend.Close() })
	ts, err := tools.Builtin(backend).Resolve([]string{tools.EchoInfoName})
	if err != nil {
		t.Fatal(err)
	}
	engine, err := runtime.NewEngine(ctx, runtime.WrapModel(provider.NewMock()), ts, runtime.EngineConfig{
		StreamBuffer: 8, MaxEventPayloadBytes: 64 << 10, MaxContextBytes: 1 << 20,
		Compaction: &runtime.CompactionPolicy{Enabled: true, MaxTokens: 128000, TriggerPercent: 80, KeepRecent: 12},
	})
	if err != nil {
		t.Fatal(err)
	}
	bus := events.NewBus(8)
	service := runtime.NewService(engine, "mock", "mock", runtime.ServiceDeps{
		Journal: backend, Runs: backend, Messages: backend, Approvals: backend, Questions: backend, Sink: bus, Compactions: backend,
	})
	settingsPath := filepath.Join(t.TempDir(), "agent-home", "settings.yaml")
	handler, err := NewControlHandler(ControlDeps{
		Sessions: backend, Messages: backend, Runs: backend, Journal: backend,
		Approvals: backend, Questions: backend, Bus: bus, Service: service,
		Studio:                         studio.NewService(backend),
		SettingsPath:                   settingsPath,
		ConfigProvider:                 "openai",
		ConfigModel:                    "gpt-4o-mini",
		ConfigNetworkSearchProvider:    "duckduckgo",
		ConfigCompaction:               runtime.CompactionPolicy{Enabled: true, MaxTokens: 0, TriggerPercent: 80, KeepRecent: 12},
		ConfigExecuteMaxTimeoutSeconds: 30,
		DefaultPermissionPreset:        domain.PermissionPresetSmart,
		ConfigSandboxDenyPrivateIPs:    true,
	})
	if err != nil {
		t.Fatal(err)
	}

	// settings/update persists the compaction overlay.
	if _, rpcErr := callControl(t, handler, "settings/update", map[string]any{
		"compaction": map[string]any{"enabled": true, "max_tokens": 65536, "trigger_percent": 60, "keep_recent": 4},
	}); rpcErr != nil {
		t.Fatal(rpcErr)
	}
	get, rpcErr := callControl(t, handler, "settings/get", nil)
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	res := get.(settingsResult)
	if res.Compaction.MaxTokens != 65536 || res.Compaction.TriggerPercent != 60 || res.Compaction.KeepRecent != 4 {
		t.Fatalf("compaction view = %+v, want overlay values", res.Compaction)
	}
	if res.Compaction.ConfigMaxTokens != 0 || res.Compaction.ConfigTriggerPercent != 80 {
		t.Fatalf("compaction config fallbacks = %+v", res.Compaction)
	}
	// The persisted document is merge-shaped: provider selection survives.
	saved, err := settings.Load(settingsPath)
	if err != nil || saved.Compaction == nil || saved.Compaction.MaxTokens != 65536 {
		t.Fatalf("persisted compaction = %+v err=%v", saved.Compaction, err)
	}

	// session/context works for any session id, even one without messages.
	created, rpcErr := callControl(t, handler, "session/create", map[string]any{"title": "ctx"})
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	sessionID := created.(sessionResult).ID
	status, rpcErr := callControl(t, handler, "session/context", map[string]any{"session_id": sessionID})
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	sc := status.(runtime.ContextStatusResult)
	if sc.SessionID != sessionID || sc.ModelLimitTokens <= 0 || sc.TriggerTokens <= 0 {
		t.Fatalf("session context = %+v", sc)
	}

	// context/compact on an empty session reports nothing to do.
	compactResult, rpcErr := callControl(t, handler, "context/compact", map[string]any{"session_id": sessionID})
	if rpcErr != nil {
		t.Fatalf("compact empty session: %v", rpcErr)
	}
	if !compactResult.(runtime.CompactionResult).Skipped {
		t.Fatalf("expected skipped compaction for empty session, got %+v", compactResult)
	}
}
