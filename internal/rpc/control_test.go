package rpc

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/events"
	"agent-vivy/internal/provider"
	"agent-vivy/internal/runtime"
	"agent-vivy/internal/storage/sqlite"
	"agent-vivy/internal/tools"
)

type controlTestEnv struct {
	backend *sqlite.Backend
	handler Handler
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
	handler, err := NewControlHandler(ControlDeps{
		Sessions: backend, Messages: backend, Runs: backend, Journal: backend,
		Approvals: backend, Questions: backend, Bus: bus, Service: service,
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
