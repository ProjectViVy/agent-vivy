package rpc

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"agent-vivy/internal/app/settings"
	"agent-vivy/internal/channelhost"
	"agent-vivy/internal/channelhost/fake"
	"agent-vivy/internal/config"
	"agent-vivy/internal/domain"
	"agent-vivy/internal/eval"
	"agent-vivy/internal/events"
	"agent-vivy/internal/i18n"
	"agent-vivy/internal/modelhost"
	"agent-vivy/internal/provider"
	"agent-vivy/internal/runtime"
	"agent-vivy/internal/storage"
	"agent-vivy/internal/storage/sqlite"
	"agent-vivy/internal/studio"
	"agent-vivy/internal/testsupport"
	"agent-vivy/internal/tools"
	plugin "agent-vivy/sdk/port/channel"
)

func TestSessionHistoryRepairsDurableAssistantProjection(t *testing.T) {
	env := newControlTestEnv(t)
	ctx := context.Background()
	created, rpcErr := callControl(t, env.handler, "session/create", map[string]string{"title": "repair"})
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	raw, _ := json.Marshal(created)
	var session sessionResult
	if err := json.Unmarshal(raw, &session); err != nil {
		t.Fatal(err)
	}
	runID := domain.RunID("run-rpc-projection-repair")
	if err := env.backend.CreateRun(ctx, domain.Run{ID: runID, SessionID: session.ID, Status: domain.RunCompleted, CreatedAt: 1}); err != nil {
		t.Fatal(err)
	}
	content := "durable rpc answer"
	digest := fmt.Sprintf("%x", sha256.Sum256([]byte(content)))
	if _, err := env.backend.Append(ctx, storage.Commit{RunID: runID, Events: []domain.RunEvent{
		{Type: domain.EventModelRequest, CreatedAt: 10, PayloadVersion: 1, Payload: []byte(`{"messages":0,"preamble_bytes":0}`)},
		{Type: domain.EventModelDelta, CreatedAt: 20, PayloadVersion: 1, Payload: []byte(`{"delta":"durable rpc answer"}`)},
		{Type: domain.EventModelCompleted, CreatedAt: 30, PayloadVersion: 2, Payload: []byte(fmt.Sprintf(`{"content_sha256":%q,"byte_len":%d}`, digest, len([]byte(content))))},
		{Type: domain.EventRunCompleted, CreatedAt: 40, PayloadVersion: 1, Payload: []byte(`{}`)},
	}}); err != nil {
		t.Fatal(err)
	}
	before, err := env.backend.ListMessages(ctx, session.ID)
	if err != nil || len(before) != 0 {
		t.Fatalf("precondition messages = %+v, err=%v", before, err)
	}
	for _, method := range []string{"session/messages", "session/get"} {
		result, rpcErr := callControl(t, env.handler, method, map[string]string{"session_id": string(session.ID)})
		if rpcErr != nil {
			t.Fatalf("%s: %v", method, rpcErr)
		}
		encoded, _ := json.Marshal(result)
		if !strings.Contains(string(encoded), content) {
			t.Fatalf("%s did not repair assistant projection: %s", method, encoded)
		}
	}
	after, err := env.backend.ListMessages(ctx, session.ID)
	if err != nil || len(after) != 1 || after[0].Content != content {
		t.Fatalf("repaired messages = %+v, err=%v", after, err)
	}
}

type controlTestEnv struct {
	backend *sqlite.Backend
	handler Handler
}

type childControllerStub struct{}

func TestRunSubscriptionStopsWhenPeerClosesWhileIdle(t *testing.T) {
	env := newControlTestEnv(t)
	handler := env.handler.(*controlHandler)
	left, right := net.Pipe()
	defer right.Close()
	peer := NewPeer(NewJSONLTransport(left, left, left.Close), nil, Options{OutgoingBuffer: 2})
	params, err := json.Marshal(map[string]any{"run_id": "run_idle", "after_seq": 0})
	if err != nil {
		t.Fatal(err)
	}
	request := Request{JSONRPC: "2.0", ID: json.RawMessage(`"idle"`), Method: "run/subscribe", Params: params}
	if _, rpcErr := handler.subscribe(context.Background(), peer, request); rpcErr != nil {
		t.Fatalf("subscribe: %v", rpcErr)
	}
	peer.runAfterResponse(request.ID)
	waitForSubscriptionCount := func(want int) {
		deadline := time.Now().Add(time.Second)
		for time.Now().Before(deadline) {
			handler.mu.Lock()
			got := len(handler.subscriptions)
			handler.mu.Unlock()
			if got == want {
				return
			}
			time.Sleep(time.Millisecond)
		}
		handler.mu.Lock()
		got := len(handler.subscriptions)
		handler.mu.Unlock()
		t.Fatalf("subscription count = %d, want %d", got, want)
	}
	waitForSubscriptionCount(1)
	if err := peer.Close(); err != nil {
		t.Fatal(err)
	}
	waitForSubscriptionCount(0)
}

func TestRunSubscriptionTerminalReplayCleansUpImmediately(t *testing.T) {
	env := newControlTestEnv(t)
	handler := env.handler.(*controlHandler)
	runID := domain.RunID("run_terminal_replay")
	ctx := context.Background()
	if err := env.backend.CreateRun(ctx, domain.Run{ID: runID, SessionID: "sess_terminal_replay", Status: domain.RunActive, CreatedAt: time.Now().UnixMilli()}); err != nil {
		t.Fatal(err)
	}
	if _, err := env.backend.Append(ctx, storage.Commit{RunID: runID, Events: []domain.RunEvent{{
		Type: domain.EventRunCompleted, CreatedAt: time.Now().UnixMilli(), PayloadVersion: 1, Payload: []byte(`{}`),
	}}}); err != nil {
		t.Fatal(err)
	}
	left, right := net.Pipe()
	defer right.Close()
	peer := NewPeer(NewJSONLTransport(left, left, left.Close), nil, Options{OutgoingBuffer: 2})
	defer peer.Close()
	params, err := json.Marshal(map[string]any{"run_id": string(runID), "after_seq": 0})
	if err != nil {
		t.Fatal(err)
	}
	request := Request{JSONRPC: "2.0", ID: json.RawMessage(`"terminal"`), Method: "run/subscribe", Params: params}
	if _, rpcErr := handler.subscribe(ctx, peer, request); rpcErr != nil {
		t.Fatalf("subscribe: %v", rpcErr)
	}
	peer.runAfterResponse(request.ID)
	waitForControlSubscriptionCount(t, handler, 0)
	if got := handler.deps.Bus.Subscribers(runID); got != 0 {
		t.Fatalf("bus subscribers = %d, want 0", got)
	}
}

// TestRunSubscriptionEmptyReplayAfterTerminalCleansUp pins the N5 contract:
// a resubscribe whose after_seq already covers the terminal record must
// release the subscription instead of pinning the bus until peer close.
func TestRunSubscriptionEmptyReplayAfterTerminalCleansUp(t *testing.T) {
	env := newControlTestEnv(t)
	handler := env.handler.(*controlHandler)
	runID := domain.RunID("run_empty_replay_terminal")
	ctx := context.Background()
	if err := env.backend.CreateRun(ctx, domain.Run{ID: runID, SessionID: "sess_empty_replay", Status: domain.RunCompleted, CreatedAt: time.Now().UnixMilli()}); err != nil {
		t.Fatal(err)
	}
	if _, err := env.backend.Append(ctx, storage.Commit{RunID: runID, Events: []domain.RunEvent{{
		Type: domain.EventRunCompleted, CreatedAt: time.Now().UnixMilli(), PayloadVersion: 1, Payload: []byte(`{}`),
	}}}); err != nil {
		t.Fatal(err)
	}
	left, right := net.Pipe()
	defer right.Close()
	peer := NewPeer(NewJSONLTransport(left, left, left.Close), nil, Options{OutgoingBuffer: 2})
	defer peer.Close()
	params, err := json.Marshal(map[string]any{"run_id": string(runID), "after_seq": 5})
	if err != nil {
		t.Fatal(err)
	}
	request := Request{JSONRPC: "2.0", ID: json.RawMessage(`"empty-replay"`), Method: "run/subscribe", Params: params}
	if _, rpcErr := handler.subscribe(ctx, peer, request); rpcErr != nil {
		t.Fatalf("subscribe: %v", rpcErr)
	}
	peer.runAfterResponse(request.ID)
	waitForControlSubscriptionCount(t, handler, 0)
	if got := handler.deps.Bus.Subscribers(runID); got != 0 {
		t.Fatalf("bus subscribers = %d, want 0", got)
	}
}

// TestRunSubscriptionUnknownRunReleasesSubscription keeps an absent run id
// from pinning a dead subscription; it can never produce events.
func TestRunSubscriptionUnknownRunReleasesSubscription(t *testing.T) {
	env := newControlTestEnv(t)
	handler := env.handler.(*controlHandler)
	ctx := context.Background()
	left, right := net.Pipe()
	defer right.Close()
	peer := NewPeer(NewJSONLTransport(left, left, left.Close), nil, Options{OutgoingBuffer: 2})
	defer peer.Close()
	params, err := json.Marshal(map[string]any{"run_id": "run_never_existed", "after_seq": 0})
	if err != nil {
		t.Fatal(err)
	}
	request := Request{JSONRPC: "2.0", ID: json.RawMessage(`"unknown-run"`), Method: "run/subscribe", Params: params}
	if _, rpcErr := handler.subscribe(ctx, peer, request); rpcErr != nil {
		t.Fatalf("subscribe: %v", rpcErr)
	}
	peer.runAfterResponse(request.ID)
	waitForControlSubscriptionCount(t, handler, 0)
}

func TestRunSubscriptionResponseCloseCleansPendingEntry(t *testing.T) {
	env := newControlTestEnv(t)
	handler := env.handler.(*controlHandler)
	left, right := net.Pipe()
	defer right.Close()
	peer := NewPeer(NewJSONLTransport(left, left, left.Close), handler, Options{OutgoingBuffer: 1})
	if err := peer.Notify("occupy", nil); err != nil {
		t.Fatal(err)
	}
	params, err := json.Marshal(map[string]any{"run_id": "run_pending_response", "after_seq": 0})
	if err != nil {
		t.Fatal(err)
	}
	request := Request{JSONRPC: "2.0", ID: json.RawMessage(`"pending"`), Method: "run/subscribe", Params: params}
	done := make(chan struct{})
	go func() {
		peer.handleRequest(context.Background(), request)
		close(done)
	}()
	waitForControlSubscriptionCount(t, handler, 1)
	if err := peer.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("request did not unblock after peer close")
	}
	waitForControlSubscriptionCount(t, handler, 0)
	peer.mu.Lock()
	after := len(peer.after)
	peer.mu.Unlock()
	if after != 0 {
		t.Fatalf("after-response callbacks = %d, want 0", after)
	}
}

func TestRunSubscriptionContextCancelCleansPendingEntry(t *testing.T) {
	env := newControlTestEnv(t)
	handler := env.handler.(*controlHandler)
	left, right := net.Pipe()
	defer right.Close()
	peer := NewPeer(NewJSONLTransport(left, left, left.Close), handler, Options{OutgoingBuffer: 1})
	defer peer.Close()
	if err := peer.Notify("occupy", nil); err != nil {
		t.Fatal(err)
	}
	params, err := json.Marshal(map[string]any{"run_id": "run_cancelled_response", "after_seq": 0})
	if err != nil {
		t.Fatal(err)
	}
	request := Request{JSONRPC: "2.0", ID: json.RawMessage(`"cancelled"`), Method: "run/subscribe", Params: params}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		peer.handleRequest(ctx, request)
		close(done)
	}()
	waitForControlSubscriptionCount(t, handler, 1)
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("request did not unblock after context cancellation")
	}
	waitForControlSubscriptionCount(t, handler, 0)
	peer.mu.Lock()
	after := len(peer.after)
	peer.mu.Unlock()
	if after != 0 {
		t.Fatalf("after-response callbacks = %d, want 0", after)
	}
}

func waitForControlSubscriptionCount(t *testing.T, handler *controlHandler, want int) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		handler.mu.Lock()
		got := len(handler.subscriptions)
		handler.mu.Unlock()
		if got == want {
			return
		}
		time.Sleep(time.Millisecond)
	}
	handler.mu.Lock()
	got := len(handler.subscriptions)
	handler.mu.Unlock()
	t.Fatalf("subscription count = %d, want %d", got, want)
}

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

func newControlTestEnv(t *testing.T, mutators ...func(*ControlDeps)) *controlTestEnv {
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
	engine, err := runtime.NewEngine(ctx, runtime.WrapModel(testsupport.NewEchoModel()), ts, runtime.EngineConfig{
		StreamBuffer: 8, MaxEventPayloadBytes: 64 << 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	bus := events.NewBus(8)
	service := runtime.NewService(engine, "test", "test-model", runtime.ServiceDeps{
		Journal: backend, Runs: backend, Messages: backend, Approvals: backend, Questions: backend,
		Sessions: backend, Crons: backend, Sink: bus, Truncations: backend,
	})
	liveTools := make([]domain.ToolSpec, 0, len(ts))
	for _, tool := range ts {
		liveTools = append(liveTools, tool.Spec())
	}
	deps := ControlDeps{
		Sessions: backend, Messages: backend, Runs: backend, Journal: backend,
		Approvals: backend, Questions: backend, Todos: backend, Bus: bus, Service: service,
		Crons: backend, CronRunner: service, Truncations: backend,
		Studio: studio.NewService(backend),
		Live: studio.LiveView{
			Provider:      "test",
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
	}
	for _, mutate := range mutators {
		mutate(&deps)
	}
	handler, err := NewControlHandler(deps)
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

func TestControlSessionCreateKeepsEmptyTitleUntitled(t *testing.T) {
	env := newControlTestEnv(t)
	created, rpcErr := callControl(t, env.handler, "session/create", map[string]string{"title": "   "})
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	createdJSON, _ := json.Marshal(created)
	var session sessionResult
	if err := json.Unmarshal(createdJSON, &session); err != nil {
		t.Fatal(err)
	}
	// An empty title marks the session untitled for the auto-titler; the
	// legacy "New session" defaulting is gone.
	if session.Title != "" {
		t.Fatalf("title = %q, want empty", session.Title)
	}
	if _, rpcErr := callControl(t, env.handler, "session/get", map[string]string{"session_id": string(session.ID)}); rpcErr != nil {
		t.Fatal(rpcErr)
	}
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

func TestControlShellStartIsGovernedAndStrict(t *testing.T) {
	var shellService *runtime.Service
	env := newControlTestEnv(t, func(deps *ControlDeps) {
		backend := deps.Sessions.(*sqlite.Backend)
		root := t.TempDir()
		workspace, err := runtime.NewWorkspaceManager(root)
		if err != nil {
			t.Fatal(err)
		}
		sandbox, err := runtime.NewSandboxManager(domain.SandboxModeWorkspaceWrite, root, []string{"go"}, nil)
		if err != nil {
			t.Fatal(err)
		}
		commands := runtime.NewCommandBackend(workspace, sandbox, []string{"go"}, 5*time.Second)
		active, err := tools.NewRegistry(tools.NewBash(commands)).Resolve([]string{tools.BashName})
		if err != nil {
			t.Fatal(err)
		}
		engine, err := runtime.NewEngine(context.Background(), runtime.WrapModel(testsupport.NewEchoModel()), active, runtime.EngineConfig{
			StreamBuffer: 8, MaxEventPayloadBytes: 64 << 10,
		})
		if err != nil {
			t.Fatal(err)
		}
		shellService = runtime.NewService(engine, "test", "test-model", runtime.ServiceDeps{
			Journal: backend, Runs: backend, Messages: backend, Approvals: backend, Questions: backend,
			Sessions: backend, Workspaces: workspace, ShellState: backend.Blobs(), Sink: deps.Bus,
		})
		deps.Service = shellService
		deps.Live.Tools = []domain.ToolSpec{active[0].Spec()}
	})
	t.Cleanup(func() {
		shellService.CancelAll()
		shellService.WaitIdle(context.Background())
	})

	initialized, rpcErr := callControl(t, env.handler, "initialize", nil)
	if rpcErr != nil || !containsCapability(initialized, "shell.start") {
		t.Fatalf("initialize shell capability = %v, err = %v", initialized, rpcErr)
	}
	created, rpcErr := callControl(t, env.handler, "session/create", map[string]string{"title": "shell"})
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	raw, _ := json.Marshal(created)
	var session sessionResult
	if err := json.Unmarshal(raw, &session); err != nil {
		t.Fatal(err)
	}
	if _, rpcErr := callControl(t, env.handler, "session/set_permission", map[string]string{
		"session_id": string(session.ID), "preset": string(domain.PermissionPresetTrusted),
	}); rpcErr != nil {
		t.Fatal(rpcErr)
	}
	for _, params := range []any{
		map[string]string{"session_id": string(session.ID)},
		map[string]string{"session_id": string(session.ID), "script": "echo safe", "cwd": "."},
	} {
		if _, rpcErr := callControl(t, env.handler, "shell/start", params); rpcErr == nil || rpcErr.Code != InvalidParams {
			t.Fatalf("strict shell params error = %v", rpcErr)
		}
	}
	if _, rpcErr := callControl(t, env.handler, "shell/start", map[string]string{
		"session_id": string(session.ID), "script": "curl https://example.com",
	}); rpcErr == nil || rpcErr.Code != InvalidParams {
		t.Fatalf("network shell error = %v, want InvalidParams", rpcErr)
	}
	started, rpcErr := callControl(t, env.handler, "shell/start", map[string]string{
		"session_id": string(session.ID), "script": "printf rpc_shell_ok",
	})
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	acceptedJSON, _ := json.Marshal(started)
	var accepted struct {
		RunID string `json:"run_id"`
	}
	if err := json.Unmarshal(acceptedJSON, &accepted); err != nil || accepted.RunID == "" {
		t.Fatalf("shell/start result = %s, err = %v", acceptedJSON, err)
	}
	waitForControlRunTerminal(t, env.backend, accepted.RunID)
	events, rpcErr := callControl(t, env.handler, "run/log", map[string]any{"run_id": accepted.RunID, "after_seq": 0})
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	eventsJSON, _ := json.Marshal(events)
	if !strings.Contains(string(eventsJSON), "rpc_shell_ok") || strings.Contains(string(eventsJSON), string(domain.EventModelRequest)) {
		t.Fatalf("governed shell log = %s", eventsJSON)
	}
	history, rpcErr := callControl(t, env.handler, "session/messages", map[string]any{
		"session_id": string(session.ID), "include_attachment_data": false,
	})
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	historyJSON, _ := json.Marshal(history)
	if !strings.Contains(string(historyJSON), `"tool_name":"bash"`) ||
		!strings.Contains(string(historyJSON), "bash script [redacted") ||
		strings.Contains(string(historyJSON), "printf rpc_shell_ok") {
		t.Fatalf("sanitized shell history = %s", historyJSON)
	}
}

func TestMessageProjectionNeverExposesOrdinaryBashArguments(t *testing.T) {
	result := toMessageResult(domain.Message{
		ToolName: tools.BashName, ToolCallID: "call_model_bash",
		ToolArgs: json.RawMessage(`{"command":"echo raw-model-command"}`),
	}, false)
	if result.ToolName != tools.BashName || result.ToolPreview != "" {
		t.Fatalf("ordinary bash projection leaked preview: %+v", result)
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
	engine, err := runtime.NewEngine(ctx, runtime.WrapModel(testsupport.NewEchoModel()), ts, runtime.EngineConfig{
		StreamBuffer: 8, MaxEventPayloadBytes: 64 << 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	bus := events.NewBus(8)
	service := runtime.NewService(engine, "test", "test-model", runtime.ServiceDeps{
		Journal: backend, Runs: backend, Messages: backend, Approvals: backend, Questions: backend,
		Sessions: backend, Crons: backend, Sink: bus, Truncations: backend,
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

func TestSettingsGetExposesBackendAuthoritativeLocale(t *testing.T) {
	env, settingsPath := newSettingsHandlerEnvWith(t, nil, func(deps *ControlDeps) {
		deps.GenerationLocale = i18n.English
		deps.DeveloperLocale = i18n.English
		deps.SealedGeneration = false
	})
	if _, err := settings.Save(settingsPath, settings.Settings{Locale: "zh"}); err != nil {
		t.Fatalf("seed locale: %v", err)
	}

	result, rpcErr := callControl(t, env.handler, "settings/get", nil)
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	get := result.(settingsResult)
	if get.Locale != "zh" || get.GenerationLocale != "en" || get.WorkspaceLocale != "zh" || get.localeSettingsResult.ReadOnly {
		t.Fatalf("locale view = %+v; want effective zh, generation en, workspace zh, writable", get.localeSettingsResult)
	}
	raw, err := json.Marshal(get)
	if err != nil {
		t.Fatal(err)
	}
	var wire struct {
		Locale           string `json:"locale"`
		GenerationLocale string `json:"generation_locale"`
		WorkspaceLocale  string `json:"workspace_locale"`
		ReadOnly         bool   `json:"locale_read_only"`
	}
	if err := json.Unmarshal(raw, &wire); err != nil {
		t.Fatal(err)
	}
	if wire.Locale != "zh" || wire.GenerationLocale != "en" || wire.WorkspaceLocale != "zh" || wire.ReadOnly {
		t.Fatalf("settings/get locale wire = %s", raw)
	}
	result, rpcErr = callControl(t, env.handler, "settings/update", map[string]any{"provider": "openai"})
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	updated := result.(settingsResult)
	if updated.Locale != "zh" || updated.GenerationLocale != "en" || updated.WorkspaceLocale != "zh" || updated.localeSettingsResult.ReadOnly {
		t.Fatalf("settings/update locale view = %+v; want persisted effective zh", updated.localeSettingsResult)
	}
}

func TestSettingsGetPropagatesDocumentErrors(t *testing.T) {
	tests := []struct {
		name    string
		prepare func(*testing.T, string)
	}{
		{
			name: "malformed yaml",
			prepare: func(t *testing.T, path string) {
				t.Helper()
				if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte("locale: [\n"), 0o600); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "invalid locale",
			prepare: func(t *testing.T, path string) {
				t.Helper()
				if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte("locale: ja\n"), 0o600); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "read error",
			prepare: func(t *testing.T, path string) {
				t.Helper()
				if err := os.MkdirAll(path, 0o700); err != nil {
					t.Fatal(err)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env, path := newSettingsHandlerEnv(t, nil)
			tt.prepare(t, path)
			result, rpcErr := callControl(t, env.handler, "settings/get", nil)
			if result != nil || rpcErr == nil || rpcErr.Code != InternalError {
				t.Fatalf("settings/get = result %#v, error %v; want nil InternalError", result, rpcErr)
			}
		})
	}
}

func TestSettingsGetMissingDocumentReturnsEmptyOverlay(t *testing.T) {
	env, _ := newSettingsHandlerEnvWith(t, nil, func(deps *ControlDeps) {
		deps.GenerationLocale = i18n.English
	})
	result, rpcErr := callControl(t, env.handler, "settings/get", nil)
	if rpcErr != nil {
		t.Fatalf("missing settings document: %v", rpcErr)
	}
	get := result.(settingsResult)
	if get.Locale != "en" || get.WorkspaceLocale != "" {
		t.Fatalf("missing settings overlay = %+v; want generation locale en and empty workspace override", get.localeSettingsResult)
	}
}

func TestToolsListPropagatesSettingsDocumentError(t *testing.T) {
	env, path := newSettingsHandlerEnv(t, nil)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("locale: [\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	result, rpcErr := callControl(t, env.handler, "tools/list", nil)
	if result != nil || rpcErr == nil || rpcErr.Code != InternalError {
		t.Fatalf("tools/list = result %#v, error %v; want nil InternalError", result, rpcErr)
	}
}

func TestSettingsLocaleUpdatesOnlyLocaleAndAllowsFrozenProvider(t *testing.T) {
	probe := &settingsApplierProbe{}
	env, settingsPath := newSettingsHandlerEnvWith(t, probe, func(deps *ControlDeps) {
		deps.GenerationLocale = i18n.English
		deps.DeveloperLocale = i18n.Chinese
		deps.SealedGeneration = true
		deps.Frozen = true
	})
	toolsEnabled := []string{"echo_info"}
	mcpServers := []settings.MCPServer{{Name: "docs", Endpoint: "https://docs.example.com/mcp"}}
	channelEnabled := true
	allowedSenders := []string{"alice"}
	initial := settings.Settings{
		Provider:     settings.ProviderOpenAI,
		DefaultModel: "gpt-4o",
		Providers: []settings.ProviderEntry{{
			ID: "custom-openai", DisplayName: "Custom OpenAI", Bundle: settings.ProviderOpenAI,
			BaseURL: "https://gateway.example.com/v1", DefaultModel: "gpt-4o", Models: []string{"gpt-4o"},
		}},
		MCPServers:   &mcpServers,
		ToolsEnabled: &toolsEnabled,
		Channels: []settings.ChannelOverlay{{
			Name: "telegram", Enabled: &channelEnabled, AllowFrom: &allowedSenders,
		}},
	}
	if _, err := settings.Save(settingsPath, initial); err != nil {
		t.Fatalf("seed settings: %v", err)
	}

	for _, locale := range []string{"zh", "en"} {
		result, rpcErr := callControl(t, env.handler, "settings/locale", map[string]any{"locale": locale})
		if rpcErr != nil {
			t.Fatalf("update locale %s: %v", locale, rpcErr)
		}
		view := result.(localeSettingsResult)
		if view.Locale != locale || view.WorkspaceLocale != locale || view.GenerationLocale != "en" || view.ReadOnly {
			t.Fatalf("locale update %s view = %+v", locale, view)
		}
		loaded, err := settings.Load(settingsPath)
		if err != nil {
			t.Fatalf("load locale %s: %v", locale, err)
		}
		want := initial
		want.Locale = locale
		if !reflect.DeepEqual(loaded, want) {
			t.Fatalf("locale update %s changed unrelated settings:\n got: %+v\nwant: %+v", locale, loaded, want)
		}
	}
	if probe.n != 2 {
		t.Fatalf("OnSettingsChanged calls = %d, want 2", probe.n)
	}

	if _, rpcErr := callControl(t, env.handler, "settings/locale", map[string]any{"locale": "ja"}); rpcErr == nil || rpcErr.Code != InvalidParams {
		t.Fatalf("invalid locale error = %v, want InvalidParams", rpcErr)
	}
	loaded, err := settings.Load(settingsPath)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Locale != "en" || probe.n != 2 {
		t.Fatalf("rejected locale changed state: locale=%q callbacks=%d", loaded.Locale, probe.n)
	}
}

func TestSettingsLocaleIsReadOnlyWithoutSettingsPath(t *testing.T) {
	env := newControlTestEnv(t, func(deps *ControlDeps) {
		deps.GenerationLocale = i18n.English
		deps.DeveloperLocale = i18n.Chinese
	})
	result, rpcErr := callControl(t, env.handler, "settings/get", nil)
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	get := result.(settingsResult)
	if get.Locale != "zh" || get.WorkspaceLocale != "" || !get.localeSettingsResult.ReadOnly {
		t.Fatalf("read-only locale view = %+v; want developer zh with no workspace override", get.localeSettingsResult)
	}
	if _, rpcErr := callControl(t, env.handler, "settings/locale", map[string]any{"locale": "en"}); rpcErr == nil || rpcErr.Code != CodeConflict {
		t.Fatalf("locale update without settings path = %v, want conflict", rpcErr)
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
	if !containsFold(string(raw), "settings.get") || !containsFold(string(raw), "settings.update") || !containsFold(string(raw), "settings.locale") {
		t.Fatalf("settings capabilities not advertised: %s", raw)
	}
	for _, method := range []string{
		"settings.providers", "settings.providers.upsert", "settings.providers.delete",
		"settings.mcp", "settings.mcp.upsert", "settings.mcp.delete", "settings.mcp.probe",
		"settings.mcp.resources", "settings.mcp.read", "mcp.resources.list", "mcp.resources.read",
		"channel.inspect", "channel.get", "channel.update",
	} {
		if !containsFold(string(raw), method) {
			t.Fatalf("capability %s not advertised: %s", method, raw)
		}
	}
}

type mcpCatalogStub struct {
	listed        tools.MCPListResponse
	listErr       error
	resources     tools.MCPListResourcesResponse
	resourcesErr  error
	read          tools.MCPReadResourceResponse
	readErr       error
	readRequest   tools.MCPReadResourceRequest
	prompts       tools.MCPListPromptsResponse
	promptsErr    error
	prompt        tools.MCPGetPromptResponse
	promptErr     error
	promptRequest tools.MCPGetPromptRequest
	replaced      []runtime.MCPServerConfig
	listCalls     int
}

type mcpStatusCatalogStub struct {
	*mcpCatalogStub
	statuses []runtime.MCPServerStatus
}

func (s *mcpStatusCatalogStub) ServerStatuses() []runtime.MCPServerStatus {
	return append([]runtime.MCPServerStatus(nil), s.statuses...)
}

func (s *mcpCatalogStub) ListPrompts(context.Context, domain.RunID, string) (tools.MCPListPromptsResponse, error) {
	return s.prompts, s.promptsErr
}

func (s *mcpCatalogStub) GetPrompt(_ context.Context, _ domain.RunID, request tools.MCPGetPromptRequest) (tools.MCPGetPromptResponse, error) {
	s.promptRequest = request
	return s.prompt, s.promptErr
}

func (s *mcpCatalogStub) ListTools(context.Context, domain.RunID, string) (tools.MCPListResponse, error) {
	s.listCalls++
	return s.listed, s.listErr
}

func (s *mcpCatalogStub) ListResources(context.Context, domain.RunID, string) (tools.MCPListResourcesResponse, error) {
	return s.resources, s.resourcesErr
}

func (s *mcpCatalogStub) ReadResource(_ context.Context, _ domain.RunID, request tools.MCPReadResourceRequest) (tools.MCPReadResourceResponse, error) {
	s.readRequest = request
	return s.read, s.readErr
}

func (s *mcpCatalogStub) ReplaceServers(configs []runtime.MCPServerConfig) {
	s.replaced = append([]runtime.MCPServerConfig(nil), configs...)
}

func (s *mcpCatalogStub) ConfiguredServers() []runtime.MCPServerConfig {
	return append([]runtime.MCPServerConfig(nil), s.replaced...)
}

// TestToolsCatalogListAndSetActive covers the Settings tool surface RPCs:
// the catalog view reports the config default before any overlay write,
// set-active persists a whole-list replacement (including the legal
// chat-only empty list) and rejects unknown or duplicate names.
func TestToolsCatalogListAndSetActive(t *testing.T) {
	ctx := context.Background()
	backend, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "rpc-tools.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = backend.Close() })
	ts, err := tools.Builtin(backend).Resolve([]string{tools.EchoInfoName})
	if err != nil {
		t.Fatal(err)
	}
	engine, err := runtime.NewEngine(ctx, runtime.WrapModel(testsupport.NewEchoModel()), ts, runtime.EngineConfig{
		StreamBuffer: 8, MaxEventPayloadBytes: 64 << 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	bus := events.NewBus(8)
	service := runtime.NewService(engine, "test", "test-model", runtime.ServiceDeps{
		Journal: backend, Runs: backend, Messages: backend, Approvals: backend, Questions: backend,
		Sessions: backend, Crons: backend, Sink: bus, Truncations: backend,
	})
	var changes int
	handler, err := NewControlHandler(ControlDeps{
		Sessions: backend, Messages: backend, Runs: backend, Journal: backend,
		Approvals: backend, Questions: backend, Bus: bus, Service: service,
		Studio:             studio.NewService(backend),
		SettingsPath:       filepath.Join(t.TempDir(), "settings.yaml"),
		ToolCatalog:        tools.Builtin(backend).Specs(),
		ConfigToolsEnabled: []string{tools.EchoInfoName},
		OnSettingsChanged:  func() { changes++ },
	})
	if err != nil {
		t.Fatal(err)
	}

	listed, rpcErr := callControl(t, handler, "tools/list", nil)
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	view := listed.(toolsCatalogView)
	if view.OverlayWritten || len(view.Active) != 1 || view.Active[0] != tools.EchoInfoName {
		t.Fatalf("pre-overlay view = %+v", view)
	}
	if len(view.Tools) == 0 {
		t.Fatal("catalog must list the registered tools")
	}

	saved, rpcErr := callControl(t, handler, "tools/set-active", map[string]any{
		"tools": []string{tools.EchoInfoName, tools.ListNotesName},
	})
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	view = saved.(toolsCatalogView)
	if !view.OverlayWritten || len(view.Active) != 2 {
		t.Fatalf("post-set view = %+v", view)
	}
	if changes != 1 {
		t.Fatalf("OnSettingsChanged calls = %d, want 1", changes)
	}

	if _, rpcErr := callControl(t, handler, "tools/set-active", map[string]any{
		"tools": []string{"no_such_tool"},
	}); rpcErr == nil || rpcErr.Code != InvalidParams {
		t.Fatalf("unknown tool must be rejected, got %v", rpcErr)
	}
	if _, rpcErr := callControl(t, handler, "tools/set-active", map[string]any{
		"tools": []string{tools.EchoInfoName, tools.EchoInfoName},
	}); rpcErr == nil || rpcErr.Code != InvalidParams {
		t.Fatalf("duplicate tool must be rejected, got %v", rpcErr)
	}

	// An explicit empty list is the legal chat-only mode, not "use config".
	empty, rpcErr := callControl(t, handler, "tools/set-active", map[string]any{
		"tools": []string{},
	})
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	if view := empty.(toolsCatalogView); !view.OverlayWritten || len(view.Active) != 0 {
		t.Fatalf("chat-only view = %+v", view)
	}
}

func TestToolsCatalogLiveSnapshotStaysWholeDuringConcurrentRefresh(t *testing.T) {
	var snapshots atomic.Value
	first := []domain.ToolSpec{{Name: "mcp.docs.first", Description: "first", Readonly: true}}
	second := []domain.ToolSpec{{Name: "mcp.docs.second", Description: "second", Readonly: false}}
	snapshots.Store(first)
	env := newControlTestEnv(t, func(deps *ControlDeps) {
		deps.ConfigToolsEnabled = []string{"mcp.docs.first"}
		deps.ToolCatalogLive = func() []domain.ToolSpec {
			catalog := snapshots.Load().([]domain.ToolSpec)
			return append([]domain.ToolSpec(nil), catalog...)
		}
	})

	var failures atomic.Int32
	done := make(chan struct{})
	go func() {
		for index := 0; index < 500; index++ {
			if index%2 == 0 {
				snapshots.Store(first)
			} else {
				snapshots.Store(second)
			}
		}
		close(done)
	}()
	for index := 0; index < 500; index++ {
		listed, rpcErr := callControl(t, env.handler, "tools/list", nil)
		if rpcErr != nil {
			failures.Add(1)
			continue
		}
		view, ok := listed.(toolsCatalogView)
		if !ok || len(view.Tools) != 1 || (view.Tools[0].Name != "mcp.docs.first" && view.Tools[0].Name != "mcp.docs.second") {
			failures.Add(1)
		}
	}
	<-done
	if failures.Load() != 0 {
		t.Fatalf("live catalog refresh failures=%d", failures.Load())
	}
}

func TestToolsCatalogDropsStaleMCPSelectionWithoutCurrentProjection(t *testing.T) {
	catalog := []domain.ToolSpec{{Name: tools.EchoInfoName, Description: "echo", Readonly: true}}
	settingsPath := filepath.Join(t.TempDir(), "settings.yaml")
	env := newControlTestEnv(t, func(deps *ControlDeps) {
		deps.ConfigToolsEnabled = []string{"mcp.docs.retired", tools.EchoInfoName}
		deps.SettingsPath = settingsPath
		deps.ToolCatalogLive = func() []domain.ToolSpec {
			// The failed live MCP rebuild has fail-closed the current catalog;
			// no projection for docs exists in this snapshot.
			return append([]domain.ToolSpec(nil), catalog...)
		}
	})
	if _, err := settings.Save(settingsPath, settings.Settings{ToolsEnabled: &[]string{"mcp.docs.retired", tools.EchoInfoName}}); err != nil {
		t.Fatal(err)
	}
	listed, rpcErr := callControl(t, env.handler, "tools/list", nil)
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	view := listed.(toolsCatalogView)
	if slices.Contains(view.Active, "mcp.docs.retired") || slices.Contains(view.ConfigEnabled, "mcp.docs.retired") {
		t.Fatalf("stale MCP selection leaked into list view: %+v", view)
	}
	if len(view.Active) != 1 || view.Active[0] != tools.EchoInfoName || len(view.ConfigEnabled) != 1 || view.ConfigEnabled[0] != tools.EchoInfoName {
		t.Fatalf("non-MCP selection was not preserved: %+v", view)
	}
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
	engine, err := runtime.NewEngine(ctx, runtime.WrapModel(testsupport.NewEchoModel()), ts, runtime.EngineConfig{
		StreamBuffer: 8, MaxEventPayloadBytes: 64 << 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	bus := events.NewBus(8)
	service := runtime.NewService(engine, "test", "test-model", runtime.ServiceDeps{
		Journal: backend, Runs: backend, Messages: backend, Approvals: backend, Questions: backend,
		Sessions: backend, Crons: backend, Sink: bus, Truncations: backend,
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
	for _, unsafeName := range []string{" docs", "docs ", " docs "} {
		if _, rpcErr := callControl(t, handler, "settings/mcp/upsert", map[string]any{
			"name": unsafeName, "endpoint": "https://alias.example.com/mcp",
		}); rpcErr == nil || rpcErr.Code != InvalidParams {
			t.Fatalf("unsafe MCP name %q upsert error = %v, want invalid params", unsafeName, rpcErr)
		}
	}

	stdio, rpcErr := callControl(t, handler, "settings/mcp/upsert", map[string]any{
		"name": "local", "transport": "stdio", "command": "node",
		"args":     []string{"server.js", "--stdio"},
		"env_from": map[string]string{"MCP_TOKEN": "HOST_TOKEN"}, "cwd": "tools",
	})
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	stdioEntry := stdio.(mcpServerResult)
	if stdioEntry.Transport != "stdio" || stdioEntry.Command != "node" || len(stdioEntry.Args) != 2 || stdioEntry.EnvFrom["MCP_TOKEN"] != "HOST_TOKEN" || stdioEntry.Cwd != "tools" {
		t.Fatalf("stdio upsert result = %+v", stdioEntry)
	}
	if changes != 2 {
		t.Fatalf("OnSettingsChanged after stdio upsert = %d, want 2", changes)
	}

	if _, rpcErr := callControl(t, handler, "settings/mcp/upsert", map[string]any{
		"name": "bad", "endpoint": "ftp://example.com/mcp",
	}); rpcErr == nil || rpcErr.Code != InvalidParams {
		t.Fatalf("expected invalid endpoint, got %v", rpcErr)
	}
	for _, endpoint := range []string{
		"https://user:password@example.com/mcp",
		"https://example.com/mcp?api_key=secret",
		"https://example.com/mcp?access_token=secret",
	} {
		if _, rpcErr := callControl(t, handler, "settings/mcp/upsert", map[string]any{
			"name": "bad", "endpoint": endpoint,
		}); rpcErr == nil || rpcErr.Code != InvalidParams || strings.Contains(rpcErr.Message, "secret") || strings.Contains(rpcErr.Message, "password") {
			t.Fatalf("credential-bearing endpoint %q validation = %v", endpoint, rpcErr)
		}
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
	configCatalog := &mcpCatalogStub{
		replaced:  []runtime.MCPServerConfig{{Name: "config-docs", Endpoint: "https://config.example.com/mcp"}},
		resources: tools.MCPListResourcesResponse{Resources: []tools.MCPResource{{Server: "config-docs", URI: "docs://config"}}, Untrusted: true},
	}
	roConfig, err := NewControlHandler(ControlDeps{
		Sessions: backend, Messages: backend, Runs: backend, Journal: backend,
		Approvals: backend, Questions: backend, Bus: bus, Service: service,
		Studio: studio.NewService(backend), MCP: configCatalog,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, rpcErr := callControl(t, roConfig, "settings/mcp/resources", map[string]any{"name": "config-docs"}); rpcErr != nil {
		t.Fatalf("config-only MCP resource lookup: %v", rpcErr)
	}
	configList, rpcErr := callControl(t, roConfig, "settings/mcp", nil)
	if rpcErr != nil || len(configList.(mcpListResult).Servers) != 1 || configList.(mcpListResult).Servers[0].Name != "config-docs" {
		t.Fatalf("config-only MCP catalog = %#v err=%v", configList, rpcErr)
	}
	if _, rpcErr := callControl(t, roConfig, "settings/mcp/probe", map[string]any{"name": "config-docs"}); rpcErr != nil {
		t.Fatalf("config-only MCP probe: %v", rpcErr)
	}

	probed, rpcErr := callControl(t, handler, "settings/mcp/probe", map[string]any{"name": "docs"})
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	probe := probed.(mcpServerResult)
	if probe.Status != "ok" || probe.ToolCount != 1 {
		t.Fatalf("probe = %+v", probe)
	}

	catalog.resources = tools.MCPListResourcesResponse{
		Server: "docs",
		Resources: []tools.MCPResource{{
			Server: "docs", URI: "docs://guide", Name: "guide", Title: "Guide",
			Description: "remote guide", MIME: "text/markdown",
		}},
		Untrusted: true,
	}
	catalog.read = tools.MCPReadResourceResponse{
		Server: "docs", URI: "docs://guide",
		Contents:  []tools.MCPResourceContent{{URI: "docs://guide", MIME: "text/markdown", Text: stringPointer("# Hello")}},
		Untrusted: true,
	}
	resources, rpcErr := callControl(t, handler, "settings/mcp/resources", map[string]any{"name": "docs"})
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	resourcesResult, ok := resources.(tools.MCPListResourcesResponse)
	if !ok || !resourcesResult.Untrusted || resourcesResult.Server != "docs" || len(resourcesResult.Resources) != 1 || resourcesResult.Resources[0].URI != "docs://guide" {
		t.Fatalf("resources result = %#v", resources)
	}
	read, rpcErr := callControl(t, handler, "settings/mcp/read", map[string]any{"server": "docs", "uri": " docs://guide "})
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	readResult, ok := read.(tools.MCPReadResourceResponse)
	if !ok || !readResult.Untrusted || readResult.Server != "docs" || readResult.URI != "docs://guide" || len(readResult.Contents) != 1 || readResult.Contents[0].Text == nil || *readResult.Contents[0].Text != "# Hello" {
		t.Fatalf("read result = %#v", read)
	}
	if catalog.readRequest.Server != "docs" || catalog.readRequest.URI != " docs://guide " {
		t.Fatalf("read request = %+v", catalog.readRequest)
	}
	if _, rpcErr := callControl(t, handler, "settings/mcp/resources", map[string]any{"name": "missing"}); rpcErr == nil || rpcErr.Code != CodeNotFound {
		t.Fatalf("unknown resource server error = %v", rpcErr)
	}
	catalog.resourcesErr = errMCPResource
	if _, rpcErr := callControl(t, handler, "settings/mcp/resources", map[string]any{"name": "docs"}); rpcErr == nil || rpcErr.Code != InternalError || !strings.Contains(rpcErr.Message, errMCPResource.Error()) {
		t.Fatalf("resource cause error = %v", rpcErr)
	}
	catalog.resourcesErr = nil
	catalog.resourcesErr = errString("remote\x1b[31m\u009bfailure token sk-abcdefghijklmnop")
	if _, rpcErr := callControl(t, handler, "settings/mcp/resources", map[string]any{"name": "docs"}); rpcErr == nil || strings.ContainsAny(rpcErr.Message, "\x1b\u009b") || strings.Contains(rpcErr.Message, "sk-abcdefghijklmnop") {
		t.Fatalf("unsafe resource error = %v", rpcErr)
	}
	catalog.resourcesErr = nil
	catalog.resourcesErr = &tools.MCPRemoteError{Code: -32002, Message: "resource not found"}
	if _, rpcErr := callControl(t, handler, "settings/mcp/resources", map[string]any{"name": "docs"}); rpcErr == nil || rpcErr.Code != CodeNotFound {
		t.Fatalf("remote resource not-found mapping = %v", rpcErr)
	}
	catalog.resourcesErr = nil

	catalog.listErr = errMCPProbe
	failed, rpcErr := callControl(t, handler, "settings/mcp/probe", map[string]any{"name": "docs"})
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	if view := failed.(mcpServerResult); view.Status != "error" || view.Error == "" {
		t.Fatalf("failed probe = %+v", view)
	}

	if _, rpcErr := callControl(t, handler, "settings/mcp/delete", map[string]any{"name": "local"}); rpcErr != nil {
		t.Fatal(rpcErr)
	}
	if _, rpcErr := callControl(t, handler, "settings/mcp/delete", map[string]any{"name": "docs"}); rpcErr != nil {
		t.Fatal(rpcErr)
	}
	if changes != 4 {
		t.Fatalf("OnSettingsChanged after delete = %d, want 4", changes)
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

func TestMCPSettingsProjectsStdioTransportAndMissingChildEnv(t *testing.T) {
	base := &mcpCatalogStub{}
	statusCatalog := &mcpStatusCatalogStub{
		mcpCatalogStub: base,
		statuses: []runtime.MCPServerStatus{{
			Name: "local", Transport: "stdio", Error: "mcp: required environment variables are missing",
			EnvMissing: []string{"MCP_TOKEN"}, ToolCount: -1,
		}},
	}
	env, _ := newSettingsHandlerEnvWith(t, nil, func(deps *ControlDeps) {
		deps.MCP = statusCatalog
	})
	if _, rpcErr := callControl(t, env.handler, "settings/mcp/upsert", map[string]any{
		"name": "local", "transport": "stdio", "command": "node",
		"env_from": map[string]string{"MCP_TOKEN": "HOST_TOKEN"},
	}); rpcErr != nil {
		t.Fatal(rpcErr)
	}
	result, rpcErr := callControl(t, env.handler, "settings/mcp", nil)
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	view := result.(mcpListResult)
	if len(view.Servers) != 1 {
		t.Fatalf("mcp view = %+v", view)
	}
	entry := view.Servers[0]
	if entry.Transport != "stdio" || entry.Status != "error" || entry.Error == "" || len(entry.EnvMissing) != 1 || entry.EnvMissing[0] != "MCP_TOKEN" {
		t.Fatalf("stdio status projection = %+v", entry)
	}
	encoded, _ := json.Marshal(entry)
	if strings.Contains(string(encoded), "resolved-secret-value") {
		t.Fatalf("browser-facing MCP result leaked resolved env value: %s", encoded)
	}
}

func TestMCPSettingsProjectsCompleteSnapshotStatesWithoutActivation(t *testing.T) {
	base := &mcpCatalogStub{}
	catalog := &mcpStatusCatalogStub{
		mcpCatalogStub: base,
		statuses: []runtime.MCPServerStatus{
			{Name: "ready", Transport: "http", State: runtime.MCPStateReady, Initialized: true, ToolCount: 2},
			{Name: "unavailable", Transport: "http", State: runtime.MCPStateUnavailable, Error: "remote unavailable", ToolCount: -1},
			{Name: "deferred", Transport: "http", State: runtime.MCPStateDeferred, DeferredReason: "oauth is disabled", ToolCount: -1},
			{Name: "inactive", Transport: "http", State: runtime.MCPStateInactive, Error: "auth is missing", AuthMissing: true, ToolCount: -1},
			{Name: "disabled-deferred", Transport: "http", State: runtime.MCPStateDeferred, DeferredReason: "oauth is disabled", ToolCount: -1},
		},
	}
	compiled := true
	env, settingsPath := newSettingsHandlerEnvWith(t, nil, func(deps *ControlDeps) {
		deps.MCP = catalog
		deps.MCPCompiled = &compiled
	})
	list := []settings.MCPServer{
		{Name: "ready", Endpoint: "https://ready.example.com/mcp"},
		{Name: "unavailable", Endpoint: "https://unavailable.example.com/mcp"},
		{Name: "deferred", Endpoint: "https://deferred.example.com/mcp", DeferredReason: "oauth is disabled"},
		{Name: "inactive", Endpoint: "https://inactive.example.com/mcp", Enabled: settings.BoolPtr(false)},
		{Name: "disabled-deferred", Endpoint: "https://disabled-deferred.example.com/mcp", DeferredReason: "oauth is disabled", Enabled: settings.BoolPtr(false)},
	}
	if _, err := settings.Save(settingsPath, settings.Settings{MCPServers: &list}); err != nil {
		t.Fatal(err)
	}
	result, rpcErr := callControl(t, env.handler, "settings/mcp", nil)
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	view := result.(mcpListResult)
	states := make(map[string]string, len(view.Servers))
	for _, server := range view.Servers {
		states[server.Name] = server.State
	}
	want := map[string]string{
		"ready":             string(runtime.MCPStateReady),
		"unavailable":       string(runtime.MCPStateUnavailable),
		"deferred":          string(runtime.MCPStateDeferred),
		"inactive":          string(runtime.MCPStateInactive),
		"disabled-deferred": string(runtime.MCPStateInactive),
	}
	if !reflect.DeepEqual(states, want) {
		t.Fatalf("MCP states = %#v, want %#v", states, want)
	}
	for _, server := range view.Servers {
		if server.Name == "deferred" && server.DeferredReason != "oauth is disabled" {
			t.Fatalf("deferred reason was not projected: %+v", server)
		}
	}
	if base.listCalls != 0 {
		t.Fatalf("snapshot listing activated MCP %d times", base.listCalls)
	}
	deferredResult, rpcErr := callControl(t, env.handler, "settings/mcp/probe", map[string]any{"name": "deferred"})
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	if view := deferredResult.(mcpServerResult); view.State != string(runtime.MCPStateDeferred) {
		t.Fatalf("deferred probe state = %+v", view)
	}
	disabledDeferredResult, rpcErr := callControl(t, env.handler, "settings/mcp/probe", map[string]any{"name": "disabled-deferred"})
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	if view := disabledDeferredResult.(mcpServerResult); view.State != string(runtime.MCPStateInactive) {
		t.Fatalf("disabled deferred probe state = %+v", view)
	}
	if base.listCalls != 0 {
		t.Fatalf("deferred/disabled probe activated MCP %d times", base.listCalls)
	}
	if got := toMCPServerResult(settings.MCPServer{Name: "empty"}).State; got != string(runtime.MCPStateUnconfigured) {
		t.Fatalf("unconfigured state = %q", got)
	}

	compiled = false
	notCompiled, rpcErr := callControl(t, env.handler, "settings/mcp", nil)
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	for _, server := range notCompiled.(mcpListResult).Servers {
		if server.State != string(runtime.MCPStateNotCompiled) {
			t.Fatalf("not-compiled state for %q = %q", server.Name, server.State)
		}
	}
}

func TestMCPSettingsRejectsResourceBridgeWhenContextHostNotCompiled(t *testing.T) {
	compiled := false
	env, _ := newSettingsHandlerEnvWith(t, nil, func(deps *ControlDeps) {
		deps.ContextCompiled = &compiled
	})
	if _, rpcErr := callControl(t, env.handler, "settings/mcp/upsert", map[string]any{
		"name": "docs", "transport": "http", "endpoint": "https://docs.example.com/mcp", "resource_bridge": true,
	}); rpcErr == nil || !strings.Contains(rpcErr.Message, "compiled ContextHost") {
		t.Fatalf("resource bridge write error = %v, want compiled ContextHost rejection", rpcErr)
	}
}

var errMCPProbe = errString("remote down")
var errMCPResource = errString("resource remote down")

func stringPointer(value string) *string { return &value }

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
	engine, err := runtime.NewEngine(ctx, runtime.WrapModel(testsupport.NewEchoModel()), ts, runtime.EngineConfig{
		StreamBuffer: 8, MaxEventPayloadBytes: 64 << 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	bus := events.NewBus(8)
	service := runtime.NewService(engine, "test", "test-model", runtime.ServiceDeps{
		Journal: backend, Runs: backend, Messages: backend, Approvals: backend, Questions: backend,
		Sessions: backend, Crons: backend, Sink: bus, Truncations: backend,
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

func TestProviderRegistryUpsertMigratesRetiredMockSettings(t *testing.T) {
	probe := &settingsApplierProbe{}
	env, settingsPath := newSettingsHandlerEnv(t, probe)
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(settingsPath, []byte("provider: mock\ndefault_model: mock\nbase_url: \"\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, rpcErr := callControl(t, env.handler, "settings/providers/upsert", map[string]any{
		"id": "deepseek", "display_name": "DeepSeek", "bundle": "openai",
		"base_url": "https://api.deepseek.com/v1", "default_model": "deepseek-chat",
		"models": []string{"deepseek-chat"},
	}); rpcErr != nil {
		t.Fatalf("upsert should recover a retired mock settings document: %v", rpcErr)
	}
	loaded, err := settings.Load(settingsPath)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Provider != "" || loaded.DefaultModel != "" || loaded.BaseURL != "" {
		t.Fatalf("retired mock selection should not survive upsert: %+v", loaded)
	}
	if len(loaded.Providers) != 1 || loaded.Providers[0].ID != "deepseek" {
		t.Fatalf("upsert did not preserve the new provider: %+v", loaded.Providers)
	}
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

func TestProviderProfileStatusIsRedactedAndDeferredSelectionIsRejected(t *testing.T) {
	profiles := func() []modelhost.ProfileStatus {
		return []modelhost.ProfileStatus{
			{ID: "openai", AdapterFamily: "openai-compatible", EndpointClass: "native", ModelIDs: []string{"gpt-4o"}, State: modelhost.ProfileReady},
			{ID: "future", AdapterFamily: "native-future", EndpointClass: "native", ModelIDs: []string{"future-1"}, State: modelhost.ProfileDeferredIndefinite},
		}
	}
	env, _ := newSettingsHandlerEnvWith(t, nil, func(deps *ControlDeps) {
		deps.ProviderProfileStatuses = profiles
		deps.ProviderBundles = []provider.Bundle{
			{Name: "openai", Models: []string{"gpt-4o"}},
			{Name: "future", Models: []string{"future-1"}},
		}
	})

	result, rpcErr := callControl(t, env.handler, "settings/providers", nil)
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	view := result.(providersResult)
	if len(view.Profiles) != 2 || view.Profiles[1].State != modelhost.ProfileDeferredIndefinite {
		t.Fatalf("profile status view = %#v", view.Profiles)
	}
	body, err := json.Marshal(view.Profiles)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(body), "secret") || strings.Contains(string(body), "options_schema") {
		t.Fatalf("profile status leaked configuration detail: %s", body)
	}

	if _, rpcErr := callControl(t, env.handler, "settings/model/select", map[string]any{
		"provider": "future", "model": "future-1", "base_url": "https://fake.invalid/v1",
	}); rpcErr == nil || rpcErr.Code != InvalidParams {
		t.Fatalf("deferred model selection error = %v, want InvalidParams", rpcErr)
	}
	if _, rpcErr := callControl(t, env.handler, "settings/update", map[string]any{
		"provider": "future", "default_model": "future-1", "base_url": "https://fake.invalid/v1",
	}); rpcErr == nil || rpcErr.Code != InvalidParams {
		t.Fatalf("deferred settings update error = %v, want InvalidParams", rpcErr)
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

func TestSelectModelUsesCatalogAndPreservesUnrelatedSettings(t *testing.T) {
	probe := &settingsApplierProbe{}
	env, settingsPath := newSettingsHandlerEnvWith(t, probe, func(deps *ControlDeps) {
		deps.ProviderBundles = []provider.Bundle{{
			Name: "openai", DisplayName: "OpenAI", DefaultModel: "gpt-4o-mini",
			Models: []string{"gpt-4o-mini", "gpt-5"},
		}}
	})
	if _, rpcErr := callControl(t, env.handler, "settings/providers/upsert", map[string]any{
		"id": "custom-1", "display_name": "My Gateway", "bundle": "openai",
		"base_url": "https://gateway.example.com/v1", "default_model": "deepseek-chat",
		"models": []string{"deepseek-chat", "deepseek-reasoner"}, "api_key": "sk-entry",
	}); rpcErr != nil {
		t.Fatal(rpcErr)
	}
	toolsEnabled := []string{"read_file"}
	if _, err := settings.Update(settingsPath, func(cur settings.Settings) (settings.Settings, error) {
		cur.NetworkSearch.Provider = "wikipedia"
		cur.ToolsEnabled = &toolsEnabled
		return cur, nil
	}); err != nil {
		t.Fatal(err)
	}

	result, rpcErr := callControl(t, env.handler, "settings/model/select", map[string]any{
		"provider": "openai", "model": "deepseek-reasoner", "base_url": "https://gateway.example.com/v1",
	})
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	view := result.(providersResult)
	if view.ActiveProvider != "openai" || view.ActiveModel != "deepseek-reasoner" || view.ActiveBaseURL != "https://gateway.example.com/v1" {
		t.Fatalf("selection response = %+v", view)
	}
	if len(view.Bundles) != 1 || len(view.Bundles[0].Models) != 2 {
		t.Fatalf("pre-baked bundle catalog missing: %+v", view.Bundles)
	}
	loaded, err := settings.Load(settingsPath)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.NetworkSearch.Provider != "wikipedia" || loaded.ToolsEnabled == nil || len(*loaded.ToolsEnabled) != 1 || (*loaded.ToolsEnabled)[0] != "read_file" {
		t.Fatalf("model select replaced unrelated settings: %+v", loaded)
	}
	if len(loaded.Providers) != 1 || loaded.Providers[0].ApiKey != "sk-entry" {
		t.Fatalf("model select changed the provider registry: %+v", loaded.Providers)
	}
	if probe.n != 2 {
		t.Fatalf("OnSettingsChanged calls = %d, want 2 (upsert + select)", probe.n)
	}

	if _, rpcErr := callControl(t, env.handler, "settings/model/select", map[string]any{
		"provider": "openai", "model": "invented", "base_url": "https://gateway.example.com/v1",
	}); rpcErr == nil || rpcErr.Code != InvalidParams {
		t.Fatalf("unknown model must fail closed, got %v", rpcErr)
	}
	if probe.n != 2 {
		t.Fatalf("rejected selection notified listeners: %d", probe.n)
	}
	if _, rpcErr := callControl(t, env.handler, "settings/model/select", map[string]any{
		"provider": "openai", "model": "deepseek-reasoner", "base_url": "",
	}); rpcErr == nil || rpcErr.Code != InvalidParams {
		t.Fatalf("custom gateway model escaped as a direct bundle model: %v", rpcErr)
	}

	result, rpcErr = callControl(t, env.handler, "settings/model/select", map[string]any{
		"provider": "openai", "model": "gpt-5", "base_url": "",
	})
	if rpcErr != nil {
		t.Fatalf("pre-baked bundle model selection: %v", rpcErr)
	}
	if result.(providersResult).ActiveModel != "gpt-5" {
		t.Fatalf("bundle model response = %+v", result)
	}

	result, rpcErr = callControl(t, env.handler, "settings/model/select", map[string]any{
		"provider": "openai", "model": "gpt-4o-mini", "base_url": "",
	})
	if rpcErr != nil {
		t.Fatalf("config default selection: %v", rpcErr)
	}
	view = result.(providersResult)
	if view.ActiveModel != "gpt-4o-mini" || view.ActiveBaseURL != "" {
		t.Fatalf("config default response = %+v", view)
	}

	roEnv := newControlTestEnv(t)
	if _, rpcErr := callControl(t, roEnv.handler, "settings/model/select", map[string]any{
		"provider": "openai", "model": "gpt-4o-mini",
	}); rpcErr == nil || rpcErr.Code != CodeConflict {
		t.Fatalf("read-only model selection = %v", rpcErr)
	}
	frozenEnv, _ := newSettingsHandlerEnvWith(t, nil, func(deps *ControlDeps) { deps.Frozen = true })
	if _, rpcErr := callControl(t, frozenEnv.handler, "settings/model/select", map[string]any{
		"provider": "openai", "model": "gpt-4o-mini",
	}); rpcErr == nil || rpcErr.Code != CodeConflict {
		t.Fatalf("frozen model selection = %v", rpcErr)
	}

	initResult, rpcErr := callControl(t, env.handler, "initialize", nil)
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	initJSON, err := json.Marshal(initResult)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(initJSON), "settings.model.select") {
		t.Fatalf("capabilities missing settings.model.select: %s", initJSON)
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
		Service: runtime.NewService(nil, "test", "test-model", runtime.ServiceDeps{
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

func TestControlHandlerUpdatesTodo(t *testing.T) {
	env := newControlTestEnv(t)
	created, rpcErr := callControl(t, env.handler, "session/create", map[string]string{"title": "Todos"})
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	session := created.(sessionResult)

	now := time.Now().UnixMilli()
	if err := env.backend.CreateTodo(context.Background(), domain.Todo{
		ID: "1", SessionID: session.ID, Subject: "fix bug", Description: "fix issue 5",
		Status: domain.TodoPending, Blocks: []string{}, BlockedBy: []string{},
		Position: 0, CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatal(err)
	}

	// 1. Update pending -> completed
	updated, rpcErr := callControl(t, env.handler, "session/todo/update", map[string]string{
		"session_id": string(session.ID),
		"id":         "1",
		"status":     "completed",
	})
	if rpcErr != nil {
		t.Fatalf("unexpected error updating todo: %v", rpcErr)
	}
	updatedJSON, err := json.Marshal(updated)
	if err != nil {
		t.Fatal(err)
	}
	var out struct {
		Todo todoResult `json:"todo"`
	}
	if err := json.Unmarshal(updatedJSON, &out); err != nil {
		t.Fatal(err)
	}
	if out.Todo.ID != "1" || out.Todo.Status != domain.TodoCompleted {
		t.Fatalf("updated todo = %+v, want status completed", out.Todo)
	}

	// Verify in store
	stored, err := env.backend.GetTodo(context.Background(), session.ID, "1")
	if err != nil {
		t.Fatal(err)
	}
	if stored.Status != domain.TodoCompleted {
		t.Fatalf("stored status = %s, want completed", stored.Status)
	}

	// 2. Update completed -> pending
	if _, rpcErr := callControl(t, env.handler, "session/todo/update", map[string]string{
		"session_id": string(session.ID),
		"id":         "1",
		"status":     "pending",
	}); rpcErr != nil {
		t.Fatalf("unchecking todo error = %v", rpcErr)
	}

	// 3. Update pending -> cancelled
	if _, rpcErr := callControl(t, env.handler, "session/todo/update", map[string]string{
		"session_id": string(session.ID),
		"todo_id":    "1", // test todo_id alias
		"status":     "cancelled",
	}); rpcErr != nil {
		t.Fatalf("cancelling todo error = %v", rpcErr)
	}
	stored, _ = env.backend.GetTodo(context.Background(), session.ID, "1")
	if stored.Status != domain.TodoCancelled {
		t.Fatalf("stored status = %s, want cancelled", stored.Status)
	}

	// 4. Busy guard: active run rejects mutation with CodeConflict
	runID := domain.RunID("run-active-todo")
	if err := env.backend.CreateRun(context.Background(), domain.Run{
		ID:        runID,
		SessionID: session.ID,
		Status:    domain.RunActive,
		CreatedAt: time.Now().UnixMilli(),
	}); err != nil {
		t.Fatal(err)
	}
	if _, rpcErr := callControl(t, env.handler, "session/todo/update", map[string]string{
		"session_id": string(session.ID),
		"id":         "1",
		"status":     "completed",
	}); rpcErr == nil || rpcErr.Code != CodeConflict {
		t.Fatalf("expected CodeConflict during active run, got %v", rpcErr)
	}

	// Settle the run to allow mutations again
	if err := env.backend.SetRunStatus(context.Background(), runID, domain.RunCompleted); err != nil {
		t.Fatal(err)
	}

	// 5. Validation failures
	// Missing params
	if _, rpcErr := callControl(t, env.handler, "session/todo/update", map[string]string{
		"id":     "1",
		"status": "completed",
	}); rpcErr == nil || rpcErr.Code != InvalidParams {
		t.Fatalf("expected InvalidParams for missing session_id, got %v", rpcErr)
	}
	if _, rpcErr := callControl(t, env.handler, "session/todo/update", map[string]string{
		"session_id": string(session.ID),
		"status":     "completed",
	}); rpcErr == nil || rpcErr.Code != InvalidParams {
		t.Fatalf("expected InvalidParams for missing id, got %v", rpcErr)
	}
	if _, rpcErr := callControl(t, env.handler, "session/todo/update", map[string]string{
		"session_id": string(session.ID),
		"id":         "1",
	}); rpcErr == nil || rpcErr.Code != InvalidParams {
		t.Fatalf("expected InvalidParams for missing status, got %v", rpcErr)
	}
	// Unsupported status
	if _, rpcErr := callControl(t, env.handler, "session/todo/update", map[string]string{
		"session_id": string(session.ID),
		"id":         "1",
		"status":     "invalid_status",
	}); rpcErr == nil || rpcErr.Code != InvalidParams {
		t.Fatalf("expected InvalidParams for unsupported status, got %v", rpcErr)
	}
	// Missing session
	if _, rpcErr := callControl(t, env.handler, "session/todo/update", map[string]string{
		"session_id": "nonexistent-sess",
		"id":         "1",
		"status":     "completed",
	}); rpcErr == nil || rpcErr.Code != CodeNotFound {
		t.Fatalf("expected CodeNotFound for missing session, got %v", rpcErr)
	}
	// Missing todo
	if _, rpcErr := callControl(t, env.handler, "session/todo/update", map[string]string{
		"session_id": string(session.ID),
		"id":         "nonexistent-todo",
		"status":     "completed",
	}); rpcErr == nil || rpcErr.Code != CodeNotFound {
		t.Fatalf("expected CodeNotFound for missing todo, got %v", rpcErr)
	}

	// 6. Unwired store -> MethodNotFound
	unwiredBus := events.NewBus(8)
	unwired, err := NewControlHandler(ControlDeps{
		Sessions: env.backend, Messages: env.backend, Runs: env.backend, Journal: env.backend,
		Approvals: env.backend, Questions: env.backend, Bus: unwiredBus,
		Service: runtime.NewService(nil, "test", "test-model", runtime.ServiceDeps{
			Journal: env.backend, Runs: env.backend, Messages: env.backend, Approvals: env.backend, Questions: env.backend, Sink: unwiredBus,
		}),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, rpcErr := callControl(t, unwired, "session/todo/update", map[string]string{
		"session_id": string(session.ID),
		"id":         "1",
		"status":     "completed",
	}); rpcErr == nil || rpcErr.Code != MethodNotFound {
		t.Fatalf("expected MethodNotFound for unwired todo store, got %v", rpcErr)
	}
}

func TestControlHandlerListsSessionCompactions(t *testing.T) {
	env := newControlTestEnv(t, func(d *ControlDeps) {
		d.Compactions = d.Sessions.(storage.CompactionStore)
	})
	created, rpcErr := callControl(t, env.handler, "session/create", map[string]string{"title": "Compactions"})
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	createdJSON, _ := json.Marshal(created)
	var session sessionResult
	if err := json.Unmarshal(createdJSON, &session); err != nil {
		t.Fatal(err)
	}

	empty, rpcErr := callControl(t, env.handler, "session/compactions", map[string]string{"session_id": string(session.ID)})
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	emptyJSON, _ := json.Marshal(empty)
	var emptyOut struct {
		Compactions []compactionResult `json:"compactions"`
	}
	if err := json.Unmarshal(emptyJSON, &emptyOut); err != nil {
		t.Fatal(err)
	}
	if emptyOut.Compactions == nil || len(emptyOut.Compactions) != 0 {
		t.Fatalf("empty compactions = %+v, want []", emptyOut.Compactions)
	}

	ctx := context.Background()
	records := []storage.SessionCompaction{
		{SessionID: session.ID, RunID: "run-old", Summary: "older summary", TailFrom: 100, DroppedCount: 4, CreatedAt: 100},
		{SessionID: session.ID, RunID: "run-new", Summary: "newer summary", TailFrom: 200, DroppedCount: 7, CreatedAt: 200},
		{SessionID: "sess-elsewhere", RunID: "run-x", Summary: "elsewhere", TailFrom: 300, DroppedCount: 1, CreatedAt: 300},
	}
	for _, rec := range records {
		if err := env.backend.SaveSessionCompaction(ctx, rec); err != nil {
			t.Fatal(err)
		}
	}

	listed, rpcErr := callControl(t, env.handler, "session/compactions", map[string]string{"session_id": string(session.ID)})
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	listedJSON, _ := json.Marshal(listed)
	var out struct {
		Compactions []compactionResult `json:"compactions"`
	}
	if err := json.Unmarshal(listedJSON, &out); err != nil {
		t.Fatal(err)
	}
	if len(out.Compactions) != 2 || out.Compactions[0].RunID != "run-new" || out.Compactions[1].RunID != "run-old" {
		t.Fatalf("compactions = %+v, want [run-new run-old] newest first", out.Compactions)
	}
	newest := out.Compactions[0]
	if newest.Summary != "newer summary" || newest.TailFrom != 200 || newest.DroppedCount != 7 || newest.CreatedAt != 200 {
		t.Fatalf("newest record = %+v", newest)
	}

	capped, rpcErr := callControl(t, env.handler, "session/compactions", map[string]any{"session_id": string(session.ID), "limit": 1})
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	cappedJSON, _ := json.Marshal(capped)
	var cappedOut struct {
		Compactions []compactionResult `json:"compactions"`
	}
	if err := json.Unmarshal(cappedJSON, &cappedOut); err != nil {
		t.Fatal(err)
	}
	if len(cappedOut.Compactions) != 1 || cappedOut.Compactions[0].RunID != "run-new" {
		t.Fatalf("limit=1 = %+v, want only run-new", cappedOut.Compactions)
	}

	if _, rpcErr := callControl(t, env.handler, "session/compactions", map[string]string{"session_id": "missing"}); rpcErr == nil || rpcErr.Code != CodeNotFound {
		t.Fatalf("missing session error = %v", rpcErr)
	}
	if _, rpcErr := callControl(t, env.handler, "session/compactions", map[string]string{}); rpcErr == nil || rpcErr.Code != InvalidParams {
		t.Fatalf("missing session_id error = %v", rpcErr)
	}

	unwiredBus := events.NewBus(8)
	unwired, err := NewControlHandler(ControlDeps{
		Sessions: env.backend, Messages: env.backend, Runs: env.backend, Journal: env.backend,
		Approvals: env.backend, Questions: env.backend, Bus: unwiredBus,
		Service: runtime.NewService(nil, "test", "test-model", runtime.ServiceDeps{
			Journal: env.backend, Runs: env.backend, Messages: env.backend, Approvals: env.backend, Questions: env.backend, Sink: unwiredBus,
		}),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, rpcErr := callControl(t, unwired, "session/compactions", map[string]string{"session_id": string(session.ID)}); rpcErr == nil || rpcErr.Code != MethodNotFound {
		t.Fatalf("unwired compactions error = %v", rpcErr)
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
	doc := "---\nname: demo-skill\ndescription: A test skill\nuser-invocable: true\n---\n\nUse this carefully.\n"
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(doc), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "references", "guide.md"), []byte("reference content"), 0o600); err != nil {
		t.Fatal(err)
	}
	reservedDir := filepath.Join(root, "help")
	if err := os.MkdirAll(reservedDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(reservedDir, "SKILL.md"), []byte("---\nname: help\ndescription: must not shadow static help\nuser-invocable: true\n---\nreserved\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	backend, err := runtime.NewEinoSkillBackend(root, env.backend)
	if err != nil {
		t.Fatal(err)
	}
	wired, err := NewControlHandler(ControlDeps{
		Sessions: env.backend, Messages: env.backend, Runs: env.backend, Journal: env.backend,
		Approvals: env.backend, Questions: env.backend, Todos: env.backend, Skills: backend,
		Bus: events.NewBus(8), Service: runtime.NewService(nil, "test", "test-model", runtime.ServiceDeps{
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
	if len(catalog.Skills) != 2 || catalog.Skills[0].Name != "demo-skill" || !catalog.Skills[0].UserInvocable || catalog.Skills[0].Hash == "" {
		t.Fatalf("skills = %+v", catalog.Skills)
	}

	dynamicRaw, rpcErr := callControl(t, wired, "commands/list", nil)
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	dynamicJSON, _ := json.Marshal(dynamicRaw)
	var dynamic struct {
		Commands []dynamicCommandResult `json:"commands"`
	}
	if err := json.Unmarshal(dynamicJSON, &dynamic); err != nil {
		t.Fatal(err)
	}
	if len(dynamic.Commands) != 1 || dynamic.Commands[0].ID != "skill:demo-skill" || dynamic.Commands[0].Name != "demo-skill" || dynamic.Commands[0].Kind != "skill" {
		t.Fatalf("dynamic commands = %+v", dynamic.Commands)
	}
	expandedRaw, rpcErr := callControl(t, wired, "commands/expand", map[string]any{"id": "skill:demo-skill", "args": []string{"review this patch"}})
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	expandedJSON, _ := json.Marshal(expandedRaw)
	var expanded dynamicCommandExpansionResult
	if err := json.Unmarshal(expandedJSON, &expanded); err != nil {
		t.Fatal(err)
	}
	if expanded.ID != "skill:demo-skill" || !strings.Contains(expanded.Text, "Use this carefully") || !strings.Contains(expanded.Text, "review this patch") || strings.Contains(expanded.Text, root) {
		t.Fatalf("dynamic expansion = %+v", expanded)
	}
	if _, rpcErr := callControl(t, wired, "commands/expand", map[string]string{"id": "skill:missing"}); rpcErr == nil || rpcErr.Code != CodeNotFound {
		t.Fatalf("missing dynamic command error = %v", rpcErr)
	}
	if _, rpcErr := callControl(t, wired, "commands/expand", map[string]string{"id": "skill:help"}); rpcErr == nil || rpcErr.Code != CodeNotFound {
		t.Fatalf("reserved static command expansion error = %v", rpcErr)
	}
	if _, rpcErr := callControl(t, wired, "commands/expand", map[string]any{"id": "skill:demo-skill", "args": []string{strings.Repeat("x", maxDynamicCommandArgs+1)}}); rpcErr == nil || rpcErr.Code != InvalidParams {
		t.Fatalf("oversized dynamic arguments error = %v", rpcErr)
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
	if _, err := backend.SetSkillEnabled(context.Background(), "demo-skill", false, catalog.Skills[0].Hash); err != nil {
		t.Fatalf("disable dynamic skill: %v", err)
	}
	if _, rpcErr := callControl(t, wired, "commands/expand", map[string]string{"id": "skill:demo-skill"}); rpcErr == nil || rpcErr.Code != CodeNotFound {
		t.Fatalf("disabled dynamic command error = %v", rpcErr)
	}

	initResult, rpcErr := callControl(t, wired, "initialize", nil)
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	raw, _ := json.Marshal(initResult)
	if !containsFold(string(raw), "skills.list") || !containsFold(string(raw), "skills.get") || !containsFold(string(raw), "commands.list") || !containsFold(string(raw), "commands.expand") {
		t.Fatalf("skills capabilities not advertised: %s", raw)
	}
}

func TestControlHandlerMCPPromptCommands(t *testing.T) {
	stub := &mcpCatalogStub{
		prompts: tools.MCPListPromptsResponse{Untrusted: true, Prompts: []tools.MCPPrompt{{
			Server: "docs server", Name: "review/change", Title: "Review", Description: "Review a change",
			Arguments: []tools.MCPPromptArgument{{Name: "focus", Required: true}, {Name: "tone"}},
		}}},
		prompt: tools.MCPGetPromptResponse{Server: "docs server", Name: "review/change", Text: "Review the requested change", Untrusted: true},
	}
	env, _ := newSettingsHandlerEnvWith(t, nil, func(deps *ControlDeps) { deps.MCP = stub })

	listedRaw, rpcErr := callControl(t, env.handler, "commands/list", nil)
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	listedJSON, _ := json.Marshal(listedRaw)
	var listed struct {
		Commands []dynamicCommandResult `json:"commands"`
	}
	if err := json.Unmarshal(listedJSON, &listed); err != nil {
		t.Fatal(err)
	}
	if len(listed.Commands) != 1 || listed.Commands[0].Kind != "mcp_prompt" || !strings.Contains(listed.Commands[0].Usage, "focus=<value>") || len(listed.Commands[0].Arguments) != 2 {
		t.Fatalf("prompt commands = %+v", listed.Commands)
	}
	id := listed.Commands[0].ID
	expanded, rpcErr := callControl(t, env.handler, "commands/expand", map[string]any{"id": id, "args": []string{"focus=security", "tone=brief"}})
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	if got := expanded.(dynamicCommandExpansionResult); got.ID != id || got.Text != "Review the requested change" {
		t.Fatalf("expanded = %+v", got)
	}
	if stub.promptRequest.Server != "docs server" || stub.promptRequest.Name != "review/change" || stub.promptRequest.Arguments["focus"] != "security" {
		t.Fatalf("prompt request = %+v", stub.promptRequest)
	}
	for label, args := range map[string][]string{
		"missing":   nil,
		"unknown":   {"focus=security", "other=value"},
		"duplicate": {"focus=one", "focus=two"},
		"syntax":    {"focus"},
	} {
		if _, rpcErr := callControl(t, env.handler, "commands/expand", map[string]any{"id": id, "args": args}); rpcErr == nil || rpcErr.Code != InvalidParams {
			t.Fatalf("%s args error = %v", label, rpcErr)
		}
	}
	stub.prompts.Prompts = nil
	if _, rpcErr := callControl(t, env.handler, "commands/expand", map[string]any{"id": id, "args": []string{"focus=security"}}); rpcErr == nil || rpcErr.Code != CodeNotFound {
		t.Fatalf("stale prompt error = %v", rpcErr)
	}
	initResult, rpcErr := callControl(t, env.handler, "initialize", nil)
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	raw, _ := json.Marshal(initResult)
	if !containsFold(string(raw), "commands.list") || !containsFold(string(raw), "commands.expand") {
		t.Fatalf("MCP prompt command capabilities not advertised: %s", raw)
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
	engine, err := runtime.NewEngine(ctx, runtime.WrapModel(testsupport.NewEchoModel()), ts, runtime.EngineConfig{
		StreamBuffer: 8, MaxEventPayloadBytes: 64 << 10, MaxContextBytes: 1 << 20,
		Compaction: &runtime.CompactionPolicy{Enabled: true, MaxTokens: 128000, TriggerPercent: 80, KeepRecent: 12},
	})
	if err != nil {
		t.Fatal(err)
	}
	bus := events.NewBus(8)
	service := runtime.NewService(engine, "test", "test-model", runtime.ServiceDeps{
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
	if sc.ImageSupportKnown || sc.ImageSupported {
		t.Fatalf("unknown test model exposed image capability: %+v", sc)
	}

	// context/compact on an empty session reports nothing to do.
	compactResult, rpcErr := callControl(t, handler, "context/compact", map[string]any{"session_id": sessionID})
	if rpcErr != nil {
		t.Fatalf("compact empty session: %v", rpcErr)
	}
	if !compactResult.(runtime.CompactionResult).Skipped {
		t.Fatalf("expected skipped compaction for empty session, got %+v", compactResult)
	}

	// A disabled compaction policy also returns a nil RPC error by contract;
	// the wire result must still say skipped so a client cannot render a
	// zero-value success as an executed compaction.
	disabledEngine, err := runtime.NewEngine(ctx, runtime.WrapModel(testsupport.NewEchoModel()), ts, runtime.EngineConfig{
		StreamBuffer: 8, MaxEventPayloadBytes: 64 << 10, MaxContextBytes: 1 << 20,
		Compaction: nil,
	})
	if err != nil {
		t.Fatal(err)
	}
	disabledService := runtime.NewService(disabledEngine, "test", "test-model", runtime.ServiceDeps{
		Journal: backend, Runs: backend, Messages: backend, Approvals: backend, Questions: backend, Sink: bus, Compactions: backend,
	})
	disabledHandler, err := NewControlHandler(ControlDeps{
		Sessions: backend, Messages: backend, Runs: backend, Journal: backend,
		Approvals: backend, Questions: backend, Bus: bus, Service: disabledService,
		SettingsPath: settingsPath, ConfigCompaction: runtime.CompactionPolicy{Enabled: false},
	})
	if err != nil {
		t.Fatal(err)
	}
	disabledResult, rpcErr := callControl(t, disabledHandler, "context/compact", map[string]any{"session_id": sessionID})
	if rpcErr != nil {
		t.Fatalf("compact disabled policy: %v", rpcErr)
	}
	if result, ok := disabledResult.(runtime.CompactionResult); !ok || !result.Skipped {
		t.Fatalf("disabled compaction = %+v, want skipped", disabledResult)
	}
}

// TestChannelInspectRPC covers channel/inspect: the method is disabled
// without a ChannelHost, an empty host reports an empty list, and a
// populated host surfaces the compiled-in channel with its StartAll note,
// envelope state, and the token env NAME (value never crosses the wire).
func TestChannelInspectRPC(t *testing.T) {
	env := newControlTestEnv(t)
	if _, rpcErr := callControl(t, env.handler, "channel/inspect", nil); rpcErr == nil || rpcErr.Code != MethodNotFound {
		t.Fatalf("expected method-not-found without a channel host, got %v", rpcErr)
	}
	if _, rpcErr := callControl(t, env.handler, "channel/update", map[string]any{"name": "fake"}); rpcErr == nil || rpcErr.Code != MethodNotFound {
		t.Fatalf("expected method-not-found for update without a channel host, got %v", rpcErr)
	}

	// Empty host: the compiled-in set is empty and inspect returns [].
	emptyHost := channelhost.New(channelhost.Deps{})
	envEmpty, _ := newSettingsHandlerEnvWith(t, nil, func(deps *ControlDeps) { deps.Channels = emptyHost })
	got, rpcErr := callControl(t, envEmpty.handler, "channel/inspect", nil)
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	if statuses, ok := got.([]channelStatusResult); !ok || len(statuses) != 0 {
		t.Fatalf("empty host inspect = %+v, want []", got)
	}

	// Populated host: StartAll records its decision and inspect reports it.
	ctx := context.Background()
	backend, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "chan.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = backend.Close() })
	fakeCh := fake.New()
	host := channelhost.New(channelhost.Deps{
		Journal:  backend,
		Messages: backend,
		Sessions: backend,
		Run: func(context.Context, domain.SessionID, string, *domain.Provenance) (domain.RunID, error) {
			return "run-chan-inspect", nil
		},
		Channels: []plugin.Channel{fakeCh},
		Config:   config.Channels{"fake": {Enabled: false, AllowFrom: []string{"alice"}, TokenEnv: "VIVY_TEST_FAKE_CHANNEL_TOKEN"}},
	})
	if err := host.StartAll(ctx); err != nil {
		t.Fatalf("start all: %v", err)
	}
	envCh, _ := newSettingsHandlerEnvWith(t, nil, func(deps *ControlDeps) {
		deps.Channels = host
		deps.ConfigChannels = config.Channels{"fake": {Enabled: false, AllowFrom: []string{"alice"}, TokenEnv: "VIVY_TEST_FAKE_CHANNEL_TOKEN"}}
	})
	inspected, rpcErr := callControl(t, envCh.handler, "channel/inspect", nil)
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	statuses, ok := inspected.([]channelStatusResult)
	if !ok || len(statuses) != 1 {
		t.Fatalf("inspect = %+v, want one entry", inspected)
	}
	status := statuses[0]
	if status.Name != "fake" || !status.Configured || status.Enabled || status.Started {
		t.Fatalf("inspect status = %+v", status)
	}
	if status.Note != "disabled" {
		t.Fatalf("note = %q, want the disabled reason", status.Note)
	}
	if len(status.AllowFrom) != 1 || status.AllowFrom[0] != "alice" {
		t.Fatalf("allow_from = %v, want the startup-effective summary [alice]", status.AllowFrom)
	}
	if status.TokenEnv != "VIVY_TEST_FAKE_CHANNEL_TOKEN" || status.TokenEnvSet {
		t.Fatalf("token surface = %q set=%v, want the env NAME with set=false (unset variable)", status.TokenEnv, status.TokenEnvSet)
	}
	t.Setenv("VIVY_TEST_FAKE_CHANNEL_TOKEN", "dummy-not-a-secret")
	inspected, rpcErr = callControl(t, envCh.handler, "channel/inspect", nil)
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	if !inspected.([]channelStatusResult)[0].TokenEnvSet {
		t.Fatal("token_env_set = false for a set variable")
	}
}

// TestChannelInspectAllowFromAlwaysArray: an unconfigured channel has no
// allow list, but the wire still carries [] rather than null so the UI can
// compare document truth against it element-wise.
func TestChannelInspectAllowFromAlwaysArray(t *testing.T) {
	got := toChannelStatusResult(channelhost.ChannelStatus{Name: "fake"})
	if got.AllowFrom == nil || len(got.AllowFrom) != 0 {
		t.Fatalf("allow_from = %#v, want a non-nil empty array", got.AllowFrom)
	}
}

// TestChannelGetAndUpdateRPC covers channel/get and channel/update: get is
// NotFound for non-compiled-in names, update writes the settings overlay
// (never config.yaml) and echoes the folded envelope, and the guards
// (unknown name, "*" wildcard, frozen, read-only) all fail closed.
func TestChannelGetAndUpdateRPC(t *testing.T) {
	envelopeConfig := config.Channels{
		"fake": {Enabled: false, AllowFrom: []string{"alice"}, TokenEnv: "VIVY_TEST_FAKE_CHANNEL_TOKEN"},
	}
	host := channelhost.New(channelhost.Deps{
		Channels: []plugin.Channel{fake.New()},
		Config:   envelopeConfig,
	})
	probe := &settingsApplierProbe{}
	env, settingsPath := newSettingsHandlerEnvWith(t, probe, func(deps *ControlDeps) {
		deps.Channels = host
		deps.ConfigChannels = envelopeConfig
	})

	// get: known name reports document truth; unknown name is NotFound.
	got, rpcErr := callControl(t, env.handler, "channel/get", map[string]any{"name": "fake"})
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	envelope := got.(channelEnvelopeResult)
	if envelope.Name != "fake" || envelope.Enabled || !envelope.Configured ||
		len(envelope.AllowFrom) != 1 || envelope.AllowFrom[0] != "alice" ||
		envelope.TokenEnv != "VIVY_TEST_FAKE_CHANNEL_TOKEN" {
		t.Fatalf("get envelope = %+v", envelope)
	}
	_, rpcErr = callControl(t, env.handler, "channel/get", map[string]any{"name": "ghost"})
	if rpcErr == nil || rpcErr.Code != CodeNotFound || !strings.Contains(rpcErr.Message, `channel "ghost" is not compiled into this generation`) {
		t.Fatalf("unknown get error = %v", rpcErr)
	}

	// update: happy path persists the overlay entry and echoes the fold.
	saved, rpcErr := callControl(t, env.handler, "channel/update", map[string]any{
		"name": "fake", "enabled": true, "allow_from": []string{"bob", "carol"},
	})
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	envelope = saved.(channelEnvelopeResult)
	if !envelope.Enabled || !envelope.Configured || len(envelope.AllowFrom) != 2 ||
		envelope.AllowFrom[0] != "bob" || envelope.TokenEnv != "VIVY_TEST_FAKE_CHANNEL_TOKEN" {
		t.Fatalf("update envelope = %+v", envelope)
	}
	doc, err := settings.Load(settingsPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(doc.Channels) != 1 || doc.Channels[0].Name != "fake" || doc.Channels[0].Enabled == nil || !*doc.Channels[0].Enabled {
		t.Fatalf("overlay document = %+v", doc.Channels)
	}
	if doc.Channels[0].AllowFrom == nil || len(*doc.Channels[0].AllowFrom) != 2 {
		t.Fatalf("overlay allow_from = %+v", doc.Channels[0].AllowFrom)
	}
	if probe.n != 1 {
		t.Fatalf("OnSettingsChanged calls = %d, want 1", probe.n)
	}

	// update: unknown name is InvalidParams with the generation message.
	if _, rpcErr = callControl(t, env.handler, "channel/update", map[string]any{"name": "ghost", "enabled": true}); rpcErr == nil ||
		rpcErr.Code != InvalidParams || !strings.Contains(rpcErr.Message, "not compiled into this generation") {
		t.Fatalf("unknown update error = %v", rpcErr)
	}

	// update: the "*" wildcard sender is rejected by settings validation.
	if _, rpcErr = callControl(t, env.handler, "channel/update", map[string]any{
		"name": "fake", "allow_from": []string{"*"},
	}); rpcErr == nil || rpcErr.Code != InvalidParams {
		t.Fatalf("wildcard allow_from error = %v", rpcErr)
	}

	// update: a bad token_env name is rejected (env NAME, never a value).
	if _, rpcErr = callControl(t, env.handler, "channel/update", map[string]any{
		"name": "fake", "token_env": "not-an-env",
	}); rpcErr == nil || rpcErr.Code != InvalidParams {
		t.Fatalf("bad token_env error = %v", rpcErr)
	}

	// update: frozen deployments refuse channel writes.
	frozenEnv, _ := newSettingsHandlerEnvWith(t, nil, func(deps *ControlDeps) {
		deps.Channels = host
		deps.ConfigChannels = envelopeConfig
		deps.Frozen = true
	})
	if _, rpcErr = callControl(t, frozenEnv.handler, "channel/update", map[string]any{"name": "fake", "enabled": true}); rpcErr == nil || rpcErr.Code != CodeConflict {
		t.Fatalf("frozen update error = %v", rpcErr)
	}

	// update: read-only deployments (no settings document) refuse too.
	roEnv, _ := newSettingsHandlerEnvWith(t, nil, func(deps *ControlDeps) {
		deps.Channels = host
		deps.ConfigChannels = envelopeConfig
		deps.SettingsPath = ""
	})
	if _, rpcErr = callControl(t, roEnv.handler, "channel/update", map[string]any{"name": "fake", "enabled": true}); rpcErr == nil || rpcErr.Code != CodeConflict {
		t.Fatalf("read-only update error = %v", rpcErr)
	}

	// get on a frozen deployment still reads (document truth is visible).
	if _, rpcErr = callControl(t, frozenEnv.handler, "channel/get", map[string]any{"name": "fake"}); rpcErr != nil {
		t.Fatalf("frozen get must stay readable: %v", rpcErr)
	}

	// update on a compiled-in channel that config.yaml never configured:
	// the overlay entry alone marks it configured (the wizard-add path).
	addableHost := channelhost.New(channelhost.Deps{
		Channels: []plugin.Channel{fake.New()},
	})
	addableEnv, _ := newSettingsHandlerEnvWith(t, nil, func(deps *ControlDeps) {
		deps.Channels = addableHost
		// No ConfigChannels: nothing configured in config.yaml.
	})
	added, rpcErr := callControl(t, addableEnv.handler, "channel/update", map[string]any{
		"name": "fake", "enabled": true, "allow_from": []string{"carol"},
	})
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	if envelope := added.(channelEnvelopeResult); !envelope.Configured || !envelope.Enabled ||
		len(envelope.AllowFrom) != 1 || envelope.AllowFrom[0] != "carol" || envelope.TokenEnv != "" {
		t.Fatalf("overlay-only envelope = %+v, want configured+enabled with fresh fields", envelope)
	}
	got, rpcErr = callControl(t, addableEnv.handler, "channel/get", map[string]any{"name": "fake"})
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	if envelope := got.(channelEnvelopeResult); !envelope.Configured {
		t.Fatalf("get after overlay-only write = %+v, want configured", envelope)
	}
}

func TestTurnStartAttachmentsValidationAndRoundTrip(t *testing.T) {
	env := newControlTestEnv(t)
	created, rpcErr := callControl(t, env.handler, "session/create", map[string]string{"title": "att"})
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	createdJSON, _ := json.Marshal(created)
	var session sessionResult
	if err := json.Unmarshal(createdJSON, &session); err != nil {
		t.Fatal(err)
	}
	sessionID := string(session.ID)

	cases := []struct {
		name   string
		params map[string]any
	}{
		{"unsupported mime", map[string]any{
			"session_id": sessionID, "text": "hi",
			"attachments": []map[string]string{{"name": "x.txt", "mime_type": "text/plain", "data": "aGVsbG8="}},
		}},
		{"invalid base64", map[string]any{
			"session_id": sessionID, "text": "hi",
			"attachments": []map[string]string{{"mime_type": "image/png", "data": "not-base64!!"}},
		}},
		{"empty data", map[string]any{
			"session_id": sessionID, "text": "hi",
			"attachments": []map[string]string{{"mime_type": "image/png", "data": ""}},
		}},
		{"MIME spoof", map[string]any{
			"session_id": sessionID, "text": "hi",
			"attachments": []map[string]string{{"name": "notes.png", "mime_type": "image/png", "data": base64.StdEncoding.EncodeToString([]byte("plain text"))}},
		}},
		{"oversize", map[string]any{
			"session_id": sessionID, "text": "hi",
			"attachments": []map[string]string{{"mime_type": "image/png", "data": base64.StdEncoding.EncodeToString(make([]byte, maxAttachmentBytes+1))}},
		}},
		{"too many", map[string]any{
			"session_id": sessionID, "text": "hi",
			"attachments": []map[string]string{
				{"mime_type": "image/png", "data": "aGk="}, {"mime_type": "image/png", "data": "aGk="},
				{"mime_type": "image/png", "data": "aGk="}, {"mime_type": "image/png", "data": "aGk="},
				{"mime_type": "image/png", "data": "aGk="},
			},
		}},
	}
	for _, testCase := range cases {
		if _, rpcErr := callControl(t, env.handler, "turn/start", testCase.params); rpcErr == nil || rpcErr.Code != InvalidParams {
			t.Fatalf("%s: error = %v, want InvalidParams", testCase.name, rpcErr)
		}
	}

	started, rpcErr := callControl(t, env.handler, "turn/start", map[string]any{
		"session_id": sessionID, "text": "look at this",
		"attachments": []map[string]string{{"name": "dot.png", "mime_type": "image/png", "data": base64.StdEncoding.EncodeToString([]byte{0x89, 0x50, 0x4E, 0x47})}},
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
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		run, err := env.backend.GetRun(context.Background(), domain.RunID(accepted.RunID))
		if err == nil && run.Status.Terminal() {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	listed, rpcErr := callControl(t, env.handler, "session/messages", map[string]string{"session_id": sessionID})
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	listedJSON, _ := json.Marshal(listed)
	var messages struct {
		Messages []messageResult `json:"messages"`
	}
	if err := json.Unmarshal(listedJSON, &messages); err != nil {
		t.Fatal(err)
	}
	var user *messageResult
	for index := range messages.Messages {
		if messages.Messages[index].Role == domain.RoleUser && messages.Messages[index].Content == "look at this" {
			user = &messages.Messages[index]
		}
	}
	if user == nil {
		t.Fatalf("user message missing: %+v", messages.Messages)
	}
	if len(user.Attachments) != 1 {
		t.Fatalf("attachments = %+v, want one", user.Attachments)
	}
	if user.Attachments[0].Name != "dot.png" || user.Attachments[0].MimeType != "image/png" {
		t.Fatalf("attachment = %+v", user.Attachments[0])
	}
	wantURL := "data:image/png;base64," + base64.StdEncoding.EncodeToString([]byte{0x89, 0x50, 0x4E, 0x47})
	if user.Attachments[0].DataURL != wantURL {
		t.Fatalf("data_url = %q, want %q", user.Attachments[0].DataURL, wantURL)
	}
}

func TestAttachmentPathsFlowAndMessageDTOConsistency(t *testing.T) {
	projectRoot := t.TempDir()
	writeTestAttachment(t, filepath.Join(projectRoot, "look.png"), testPNGBytes())
	env := newControlTestEnv(t, func(deps *ControlDeps) {
		deps.ProjectRoot = projectRoot
	})
	created, rpcErr := callControl(t, env.handler, "session/create", map[string]string{"title": "paths"})
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	var session sessionResult
	createdJSON, _ := json.Marshal(created)
	if err := json.Unmarshal(createdJSON, &session); err != nil {
		t.Fatal(err)
	}
	sessionID := string(session.ID)

	resolved, rpcErr := callControl(t, env.handler, "attachments/resolve", map[string]any{
		"attachment_paths": []string{"look.png"},
	})
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	var resolvedEnvelope struct {
		Attachments []attachmentPathResult `json:"attachments"`
	}
	resolvedJSON, _ := json.Marshal(resolved)
	if err := json.Unmarshal(resolvedJSON, &resolvedEnvelope); err != nil {
		t.Fatal(err)
	}
	if len(resolvedEnvelope.Attachments) != 1 || resolvedEnvelope.Attachments[0].Path != "look.png" || resolvedEnvelope.Attachments[0].MimeType != "image/png" {
		t.Fatalf("resolve result = %+v", resolvedEnvelope)
	}

	started, rpcErr := callControl(t, env.handler, "turn/start", map[string]any{
		"session_id": sessionID, "text": "inspect", "attachment_paths": []string{"look.png"},
	})
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	var accepted struct {
		RunID string `json:"run_id"`
	}
	startedJSON, _ := json.Marshal(started)
	if err := json.Unmarshal(startedJSON, &accepted); err != nil || accepted.RunID == "" {
		t.Fatalf("turn/start = %s, err = %v", startedJSON, err)
	}
	waitForControlRunTerminal(t, env.backend, accepted.RunID)

	messages, rpcErr := callControl(t, env.handler, "session/messages", map[string]string{"session_id": sessionID})
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	gotMessages := decodeMessageResults(t, messages)
	fromMessages := findUserMessageWithContent(gotMessages, "inspect")
	if fromMessages == nil || len(fromMessages.Attachments) != 1 || fromMessages.Attachments[0].Size != int64(len(testPNGBytes())) {
		t.Fatalf("session/messages attachment = %+v", fromMessages)
	}

	gotSession, rpcErr := callControl(t, env.handler, "session/get", map[string]string{"session_id": sessionID})
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	fromSession := decodeSessionResultMessages(t, gotSession)
	fromGet := findUserMessageWithContent(fromSession, "inspect")
	if fromGet == nil || len(fromGet.Attachments) != 1 || fromGet.Attachments[0].Size != fromMessages.Attachments[0].Size || fromGet.Attachments[0].MimeType != fromMessages.Attachments[0].MimeType {
		t.Fatalf("session/get attachment = %+v, messages = %+v", fromGet, fromMessages)
	}
	metadataOnly, rpcErr := callControl(t, env.handler, "session/messages", map[string]any{
		"session_id": sessionID, "include_attachment_data": false,
	})
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	metadataJSON, _ := json.Marshal(metadataOnly)
	if strings.Contains(string(metadataJSON), "data_url") || strings.Contains(string(metadataJSON), base64.StdEncoding.EncodeToString(testPNGBytes())) {
		t.Fatalf("metadata-only history transported image bytes: %s", metadataJSON)
	}
	metadataMessages := decodeMessageResults(t, metadataOnly)
	metadataMessage := findUserMessageWithContent(metadataMessages, "inspect")
	if metadataMessage == nil || len(metadataMessage.Attachments) != 1 || metadataMessage.Attachments[0].Size != int64(len(testPNGBytes())) {
		t.Fatalf("metadata-only attachment = %+v", metadataMessage)
	}

	inline := make([]map[string]string, maxAttachmentCount)
	for index := range inline {
		inline[index] = map[string]string{
			"name":      fmt.Sprintf("inline-%d.png", index),
			"mime_type": "image/png",
			"data":      base64.StdEncoding.EncodeToString(testPNGBytes()),
		}
	}
	if _, rpcErr := callControl(t, env.handler, "turn/start", map[string]any{
		"session_id": sessionID, "text": "too many mixed", "attachments": inline, "attachment_paths": []string{"look.png"},
	}); rpcErr == nil || rpcErr.Code != InvalidParams {
		t.Fatalf("mixed attachment cap error = %+v", rpcErr)
	}
	if _, rpcErr := callControl(t, env.handler, "turn/start", map[string]any{
		"session_id": sessionID, "text": "missing path", "attachment_paths": []string{"missing.png"},
	}); rpcErr == nil || rpcErr.Code != InvalidParams {
		t.Fatalf("missing attachment error = %+v", rpcErr)
	}
	afterFailure, rpcErr := callControl(t, env.handler, "session/messages", map[string]string{"session_id": sessionID})
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	failedMessages := decodeMessageResults(t, afterFailure)
	if findUserMessageWithContent(failedMessages, "too many mixed") != nil || findUserMessageWithContent(failedMessages, "missing path") != nil {
		t.Fatalf("rejected attachment turn persisted a partial message: %+v", failedMessages)
	}
}

func TestAttachmentResolverCapabilityRequiresExplicitProjectRoot(t *testing.T) {
	withoutRoot := newControlTestEnv(t)
	result, rpcErr := callControl(t, withoutRoot.handler, "initialize", nil)
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	if containsCapability(result, "attachments.resolve") {
		t.Fatalf("attachment resolver advertised without project root: %v", result)
	}
	if containsCapability(result, "project-context.resolve") || containsCapability(result, "project-context.list") {
		t.Fatalf("project context resolver advertised without project root: %v", result)
	}

	withRoot := newControlTestEnv(t, func(deps *ControlDeps) { deps.ProjectRoot = t.TempDir() })
	result, rpcErr = callControl(t, withRoot.handler, "initialize", nil)
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	if !containsCapability(result, "attachments.resolve") {
		t.Fatalf("attachment resolver missing with explicit project root: %v", result)
	}
	if !containsCapability(result, "project-context.resolve") || !containsCapability(result, "project-context.list") {
		t.Fatalf("project context resolver missing with explicit project root: %v", result)
	}
}

func containsCapability(result any, want string) bool {
	raw, _ := json.Marshal(result)
	var envelope struct {
		Capabilities []string `json:"capabilities"`
	}
	if json.Unmarshal(raw, &envelope) != nil {
		return false
	}
	for _, capability := range envelope.Capabilities {
		if capability == want {
			return true
		}
	}
	return false
}

func waitForControlRunTerminal(t *testing.T, backend *sqlite.Backend, runID string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		run, err := backend.GetRun(context.Background(), domain.RunID(runID))
		if err == nil && run.Status.Terminal() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("run %s did not become terminal", runID)
}

func decodeMessageResults(t *testing.T, value any) []messageResult {
	t.Helper()
	raw, _ := json.Marshal(value)
	var envelope struct {
		Messages []messageResult `json:"messages"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		t.Fatal(err)
	}
	return envelope.Messages
}

func decodeSessionResultMessages(t *testing.T, value any) []messageResult {
	t.Helper()
	raw, _ := json.Marshal(value)
	var envelope struct {
		Messages []messageResult `json:"messages"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		t.Fatal(err)
	}
	return envelope.Messages
}

func findUserMessageWithContent(messages []messageResult, content string) *messageResult {
	for index := range messages {
		if messages[index].Role == domain.RoleUser && messages[index].Content == content {
			return &messages[index]
		}
	}
	return nil
}

// TestControlMessageProvenanceProjected (CH-C1-N3): channel turns project
// their world-entry provenance on both session/get and session/messages;
// ui turns (empty source, the legacy shape) project none.
func TestControlMessageProvenanceProjected(t *testing.T) {
	env := newControlTestEnv(t)
	created, rpcErr := callControl(t, env.handler, "session/create", map[string]string{"title": "prov"})
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	createdJSON, _ := json.Marshal(created)
	var session sessionResult
	if err := json.Unmarshal(createdJSON, &session); err != nil {
		t.Fatal(err)
	}
	sessionID := domain.SessionID(session.ID)

	uiMsg := domain.Message{ID: "m-ui", SessionID: sessionID, Role: domain.RoleUser, Content: "from the ui", CreatedAt: 1}
	channelMsg := domain.Message{ID: "m-ch", SessionID: sessionID, Role: domain.RoleUser, Content: "from telegram", CreatedAt: 2,
		Source: "channel", Channel: "telegram", ChatID: "chat-1", ChannelMessageID: "tg-42"}
	if err := env.backend.AppendMessage(context.Background(), uiMsg); err != nil {
		t.Fatal(err)
	}
	if err := env.backend.AppendMessage(context.Background(), channelMsg); err != nil {
		t.Fatal(err)
	}

	for _, method := range []string{"session/get", "session/messages"} {
		result, rpcErr := callControl(t, env.handler, method, map[string]string{"session_id": string(sessionID)})
		if rpcErr != nil {
			t.Fatal(rpcErr)
		}
		resultJSON, _ := json.Marshal(result)
		var decoded struct {
			Messages []messageResult `json:"messages"`
		}
		if err := json.Unmarshal(resultJSON, &decoded); err != nil {
			t.Fatal(err)
		}
		var ui, channel *messageResult
		for index := range decoded.Messages {
			switch decoded.Messages[index].ID {
			case "m-ui":
				ui = &decoded.Messages[index]
			case "m-ch":
				channel = &decoded.Messages[index]
			}
		}
		if ui == nil || channel == nil {
			t.Fatalf("%s: messages missing: %+v", method, decoded.Messages)
		}
		if ui.Provenance != nil {
			t.Fatalf("%s: ui message carries provenance: %+v", method, ui.Provenance)
		}
		if !strings.Contains(string(resultJSON), `"provenance"`) {
			t.Fatalf("%s: provenance key absent from payload", method)
		}
		if channel.Provenance == nil {
			t.Fatalf("%s: channel message lost provenance", method)
		}
		if channel.Provenance.Source != "channel" || channel.Provenance.Channel != "telegram" ||
			channel.Provenance.ChatID != "chat-1" || channel.Provenance.ChannelMessageID != "tg-42" {
			t.Fatalf("%s: provenance = %+v", method, channel.Provenance)
		}
	}
}

func TestControlHandlerHTTPSettingsSegment(t *testing.T) {
	probe := &settingsApplierProbe{}
	env, settingsPath := newSettingsHandlerEnvWith(t, probe, func(d *ControlDeps) {
		d.ConfigHTTPAllowedHosts = []string{"localhost", "127.0.0.1"}
		d.ConfigHTTPTimeoutSeconds = 10
	})

	// settings/get reports config defaults with no overlay.
	result, rpcErr := callControl(t, env.handler, "settings/get", nil)
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	get := result.(settingsResult)
	if strings.Join(get.HTTP.AllowedHosts, ",") != "localhost,127.0.0.1" ||
		get.HTTP.TimeoutSeconds != 10 || get.HTTP.OverlaySet ||
		strings.Join(get.HTTP.ConfigAllowedHosts, ",") != "localhost,127.0.0.1" ||
		get.HTTP.ConfigTimeoutSeconds != 10 {
		t.Fatalf("http defaults = %+v", get.HTTP)
	}

	// Update persists the overlay; hosts and timeout are echoed back.
	if _, rpcErr := callControl(t, env.handler, "settings/update", map[string]any{
		"http": map[string]any{"allowed_hosts": []string{"api.example.dev", "*.corp.dev"}, "timeout_seconds": 45},
	}); rpcErr != nil {
		t.Fatal(rpcErr)
	}
	result, rpcErr = callControl(t, env.handler, "settings/get", nil)
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	get = result.(settingsResult)
	if !get.HTTP.OverlaySet ||
		strings.Join(get.HTTP.AllowedHosts, ",") != "api.example.dev,*.corp.dev" ||
		get.HTTP.TimeoutSeconds != 45 {
		t.Fatalf("http overlay = %+v", get.HTTP)
	}
	loaded, err := settings.Load(settingsPath)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.HTTP == nil || loaded.HTTP.AllowedHosts == nil ||
		len(*loaded.HTTP.AllowedHosts) != 2 || loaded.HTTP.TimeoutSeconds != 45 {
		t.Fatalf("http overlay not persisted: %+v", loaded.HTTP)
	}

	// A present block with an explicit empty hosts list is the deny-all
	// surface; timeout 0 keeps the current value.
	if _, rpcErr := callControl(t, env.handler, "settings/update", map[string]any{
		"http": map[string]any{"allowed_hosts": []string{}, "timeout_seconds": 0},
	}); rpcErr != nil {
		t.Fatal(rpcErr)
	}
	result, rpcErr = callControl(t, env.handler, "settings/get", nil)
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	get = result.(settingsResult)
	if !get.HTTP.OverlaySet || len(get.HTTP.AllowedHosts) != 0 || get.HTTP.TimeoutSeconds != 45 {
		t.Fatalf("deny-all http overlay = %+v", get.HTTP)
	}

	// Restoring the config values keeps the overlay but the effective
	// surface matches the config defaults again.
	if _, rpcErr := callControl(t, env.handler, "settings/update", map[string]any{
		"http": map[string]any{"allowed_hosts": []string{"localhost", "127.0.0.1"}, "timeout_seconds": 10},
	}); rpcErr != nil {
		t.Fatal(rpcErr)
	}
	result, rpcErr = callControl(t, env.handler, "settings/get", nil)
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	get = result.(settingsResult)
	if strings.Join(get.HTTP.AllowedHosts, ",") != "localhost,127.0.0.1" || get.HTTP.TimeoutSeconds != 10 {
		t.Fatalf("restored http overlay = %+v", get.HTTP)
	}

	// An out-of-range timeout is rejected by document validation.
	if _, rpcErr := callControl(t, env.handler, "settings/update", map[string]any{
		"http": map[string]any{"timeout_seconds": 999},
	}); rpcErr == nil {
		t.Fatal("expected timeout 999 to be rejected")
	}
	if probe.n != 3 {
		t.Fatalf("OnSettingsChanged calls = %d, want 3", probe.n)
	}
}

type marketplaceFake struct {
	installID   string
	installMode tools.MarketplaceInstallMode
	installErr  error
	checkName   string
	check       tools.MarketplaceUpdateCheck
	checkErr    error
}

func (f *marketplaceFake) SearchMarketplace(context.Context, string, int) ([]tools.MarketplaceSkill, error) {
	return nil, nil
}

func (f *marketplaceFake) FeaturedMarketplace(context.Context) (tools.MarketplaceFeatured, error) {
	return tools.MarketplaceFeatured{}, nil
}

func (f *marketplaceFake) InstallMarketplace(_ context.Context, id string, mode tools.MarketplaceInstallMode) (tools.MarketplaceInstallResult, error) {
	f.installID, f.installMode = id, mode
	if f.installErr != nil {
		return tools.MarketplaceInstallResult{}, f.installErr
	}
	return tools.MarketplaceInstallResult{Outcome: tools.MarketplaceOutcomeCreated}, nil
}

func (f *marketplaceFake) CheckMarketplaceUpdate(_ context.Context, name string) (tools.MarketplaceUpdateCheck, error) {
	f.checkName = name
	if f.checkErr != nil {
		return tools.MarketplaceUpdateCheck{}, f.checkErr
	}
	return f.check, nil
}

func TestMarketplaceInstallModeAndCheckRoute(t *testing.T) {
	fake := &marketplaceFake{
		check: tools.MarketplaceUpdateCheck{Name: "demo-skill", Status: tools.MarketplaceUpdateUpgradeAvailable, MarketplaceID: "o/r/demo-skill"},
	}
	probe := &settingsApplierProbe{}
	env, _ := newSettingsHandlerEnvWith(t, probe, func(d *ControlDeps) { d.Marketplace = fake })

	if _, rpcErr := callControl(t, env.handler, "skills/marketplace/install", map[string]any{
		"id": "o/r/demo-skill", "mode": "replace",
	}); rpcErr == nil || rpcErr.Code != InvalidParams {
		t.Fatalf("bad mode error = %v", rpcErr)
	}
	if _, rpcErr := callControl(t, env.handler, "skills/marketplace/install", map[string]any{
		"id": "o/r/demo-skill", "mode": "upgrade",
	}); rpcErr != nil {
		t.Fatal(rpcErr)
	}
	if fake.installID != "o/r/demo-skill" || fake.installMode != tools.MarketplaceInstallUpgrade {
		t.Fatalf("install passthrough = %q mode %q", fake.installID, fake.installMode)
	}
	if _, rpcErr := callControl(t, env.handler, "skills/marketplace/install", map[string]any{
		"id": "o/r/demo-skill",
	}); rpcErr != nil || fake.installMode != tools.MarketplaceInstallCreate {
		t.Fatalf("default mode = %q err %v", fake.installMode, rpcErr)
	}
	fake.installErr = errString(`skills: skill "demo-skill" is not marketplace-managed; remove it and reinstall to upgrade`)
	if _, rpcErr := callControl(t, env.handler, "skills/marketplace/install", map[string]any{"id": "o/r/demo-skill"}); rpcErr == nil || rpcErr.Code != CodeConflict {
		t.Fatalf("unmanaged upgrade error = %v", rpcErr)
	}
	fake.installErr = nil

	result, rpcErr := callControl(t, env.handler, "skills/marketplace/check", map[string]any{"name": "demo-skill"})
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	check, ok := result.(tools.MarketplaceUpdateCheck)
	if !ok || check.Status != tools.MarketplaceUpdateUpgradeAvailable || check.Name != "demo-skill" || fake.checkName != "demo-skill" {
		t.Fatalf("check result = %+v ok=%v fakeName=%q", result, ok, fake.checkName)
	}
	if _, rpcErr := callControl(t, env.handler, "skills/marketplace/check", map[string]any{"name": " "}); rpcErr == nil || rpcErr.Code != InvalidParams {
		t.Fatalf("empty name error = %v", rpcErr)
	}

	bare, _ := newSettingsHandlerEnvWith(t, probe, nil)
	if _, rpcErr := callControl(t, bare.handler, "skills/marketplace/check", map[string]any{"name": "demo-skill"}); rpcErr == nil || rpcErr.Code != MethodNotFound {
		t.Fatalf("unconfigured marketplace error = %v", rpcErr)
	}
}

// TestTrajectorySessionRoute checks the trajectory/session route: invalid
// params without session_id and the projected shape over a seeded run.
func TestTrajectorySessionRoute(t *testing.T) {
	env := newControlTestEnv(t)
	ctx := context.Background()

	if _, rpcErr := callControl(t, env.handler, "trajectory/session", map[string]any{}); rpcErr == nil || rpcErr.Code != InvalidParams {
		t.Fatalf("missing session_id error = %v", rpcErr)
	}

	sessionID := domain.SessionID("sess-traj")
	runID := domain.RunID("run-traj")
	if err := env.backend.CreateRun(ctx, domain.Run{ID: runID, SessionID: sessionID, Status: domain.RunCompleted, CreatedAt: 1000}); err != nil {
		t.Fatal(err)
	}
	if err := env.backend.AppendMessage(ctx, domain.Message{ID: "m1", SessionID: sessionID, RunID: runID,
		Role: domain.RoleUser, CreatedAt: 1100, Content: "hello trajectory rpc"}); err != nil {
		t.Fatal(err)
	}
	if _, err := env.backend.Append(ctx, storage.Commit{RunID: runID, Events: []domain.RunEvent{{
		Type: domain.EventRunStarted, CreatedAt: 1200, PayloadVersion: 1,
		Payload: []byte(`{"provider":"test","model":"m1","mode":"chat"}`),
	}}}); err != nil {
		t.Fatal(err)
	}

	result, rpcErr := callControl(t, env.handler, "trajectory/session", map[string]any{"session_id": string(sessionID)})
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	session, ok := result.(runtime.TrajectorySession)
	if !ok {
		t.Fatalf("result type = %T", result)
	}
	if session.SessionID != string(sessionID) || session.Turns != 1 || len(session.Records) != 2 {
		t.Fatalf("session = %+v", session)
	}
	if session.Records[0].Kind != "system" || session.Records[0].Turn != nil {
		t.Fatalf("system record = %+v", session.Records[0])
	}
	if session.Records[1].Kind != "user" || session.Records[1].Turn == nil || *session.Records[1].Turn != 1 || !session.Records[1].OpensTurn {
		t.Fatalf("user record = %+v", session.Records[1])
	}
	if len(session.Requests) != 0 {
		t.Fatalf("requests = %+v", session.Requests)
	}
}

// TestTurnStartThinkingRoute pins the turn/start thinking boundary: an
// unknown preference is an InvalidParams before anything persists, a valid
// one starts the run, and session/context always reports the D9 gate flag.
func TestTurnStartThinkingRoute(t *testing.T) {
	env := newControlTestEnv(t)
	created, rpcErr := callControl(t, env.handler, "session/create", map[string]string{"title": "Thinking"})
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	createdJSON, _ := json.Marshal(created)
	var session sessionResult
	if err := json.Unmarshal(createdJSON, &session); err != nil {
		t.Fatal(err)
	}
	sessionID := string(session.ID)

	if _, rpcErr := callControl(t, env.handler, "turn/start", map[string]any{
		"session_id": sessionID, "text": "hi", "thinking": "execute",
	}); rpcErr == nil || rpcErr.Code != InvalidParams {
		t.Fatalf("error = %v, want InvalidParams", rpcErr)
	}

	started, rpcErr := callControl(t, env.handler, "turn/start", map[string]any{
		"session_id": sessionID, "text": "hi", "thinking": "on",
	})
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	startedJSON, _ := json.Marshal(started)
	var run struct {
		RunID string `json:"run_id"`
	}
	if err := json.Unmarshal(startedJSON, &run); err != nil || run.RunID == "" {
		t.Fatalf("turn/start result = %s (err %v)", startedJSON, err)
	}

	contextResult, rpcErr := callControl(t, env.handler, "session/context", map[string]string{"session_id": sessionID})
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	contextJSON, _ := json.Marshal(contextResult)
	var context struct {
		ThinkingSupported bool `json:"thinking_supported"`
	}
	if err := json.Unmarshal(contextJSON, &context); err != nil {
		t.Fatal(err)
	}
	if context.ThinkingSupported {
		t.Fatal("thinking_supported = true, want false without a thinking-capable route")
	}
}

func TestSessionRewindRoute(t *testing.T) {
	env := newControlTestEnv(t)
	ctx := context.Background()
	created, rpcErr := callControl(t, env.handler, "session/create", map[string]string{})
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	createdJSON, _ := json.Marshal(created)
	var session sessionResult
	if err := json.Unmarshal(createdJSON, &session); err != nil {
		t.Fatal(err)
	}
	for _, m := range []domain.Message{
		{ID: "msg-1", SessionID: session.ID, Role: domain.RoleUser, Content: "one"},
		{ID: "msg-2", SessionID: session.ID, Role: domain.RoleAssistant, Content: "two"},
		{ID: "msg-3", SessionID: session.ID, Role: domain.RoleUser, Content: "three"},
	} {
		if err := env.backend.AppendMessage(ctx, m); err != nil {
			t.Fatal(err)
		}
	}
	result, rpcErr := callControl(t, env.handler, "session/rewind", map[string]string{
		"session_id": string(session.ID), "message_id": "msg-2",
	})
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	resultJSON, _ := json.Marshal(result)
	var rewound struct {
		CutoffMessageID string `json:"cutoff_message_id"`
		RemainingCount  int    `json:"remaining_count"`
	}
	if err := json.Unmarshal(resultJSON, &rewound); err != nil {
		t.Fatal(err)
	}
	if rewound.CutoffMessageID != "msg-2" || rewound.RemainingCount != 1 {
		t.Fatalf("rewind result = %s, want cutoff msg-2 with 1 remaining", resultJSON)
	}

	listed, rpcErr := callControl(t, env.handler, "session/messages", map[string]string{"session_id": string(session.ID)})
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	listedJSON, _ := json.Marshal(listed)
	if !strings.Contains(string(listedJSON), `"one"`) || strings.Contains(string(listedJSON), `"two"`) || strings.Contains(string(listedJSON), `"three"`) {
		t.Fatalf("session/messages after rewind = %s, want only msg-1", listedJSON)
	}
	gotSession, rpcErr := callControl(t, env.handler, "session/get", map[string]string{"session_id": string(session.ID)})
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	gotSessionJSON, _ := json.Marshal(gotSession)
	if !strings.Contains(string(gotSessionJSON), `"one"`) || strings.Contains(string(gotSessionJSON), `"two"`) || strings.Contains(string(gotSessionJSON), `"three"`) {
		t.Fatalf("session/get after rewind = %s, want same projection as session/messages", gotSessionJSON)
	}
	// A turn started after the rewind (the edit flow) must stay visible.
	if err := env.backend.AppendMessage(ctx, domain.Message{ID: "msg-4", SessionID: session.ID, Role: domain.RoleUser, Content: "fresh"}); err != nil {
		t.Fatal(err)
	}
	relisted, rpcErr := callControl(t, env.handler, "session/messages", map[string]string{"session_id": string(session.ID)})
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	relistedJSON, _ := json.Marshal(relisted)
	if !strings.Contains(string(relistedJSON), `"one"`) || !strings.Contains(string(relistedJSON), `"fresh"`) || strings.Contains(string(relistedJSON), `"two"`) {
		t.Fatalf("session/messages after post-rewind turn = %s, want msg-1 + fresh", relistedJSON)
	}

	if _, rpcErr := callControl(t, env.handler, "session/rewind", map[string]string{"session_id": string(session.ID), "message_id": "msg-9"}); rpcErr == nil || rpcErr.Code != CodeNotFound {
		t.Fatalf("unknown cutoff err = %v, want CodeNotFound", rpcErr)
	}
	if _, rpcErr := callControl(t, env.handler, "session/rewind", map[string]string{}); rpcErr == nil || rpcErr.Code != InvalidParams {
		t.Fatalf("missing fields err = %v, want InvalidParams", rpcErr)
	}
	if err := env.backend.CreateRun(ctx, domain.Run{ID: "run-busy", SessionID: session.ID, Status: domain.RunActive, CreatedAt: 1}); err != nil {
		t.Fatal(err)
	}
	if _, rpcErr := callControl(t, env.handler, "session/rewind", map[string]string{"session_id": string(session.ID), "message_id": "msg-1"}); rpcErr == nil || rpcErr.Code != CodeConflict {
		t.Fatalf("busy session err = %v, want CodeConflict", rpcErr)
	}
}

func TestSessionForkRoute(t *testing.T) {
	env := newControlTestEnv(t)
	ctx := context.Background()
	created, rpcErr := callControl(t, env.handler, "session/create", map[string]string{"title": "origin"})
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	createdJSON, _ := json.Marshal(created)
	var session sessionResult
	if err := json.Unmarshal(createdJSON, &session); err != nil {
		t.Fatal(err)
	}
	for _, m := range []domain.Message{
		{ID: "msg-1", SessionID: session.ID, Role: domain.RoleUser, Content: "one"},
		{ID: "msg-2", SessionID: session.ID, Role: domain.RoleAssistant, Content: "two"},
	} {
		if err := env.backend.AppendMessage(ctx, m); err != nil {
			t.Fatal(err)
		}
	}
	result, rpcErr := callControl(t, env.handler, "session/fork", map[string]string{
		"session_id": string(session.ID), "message_id": "msg-2", "title": "branch",
	})
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	resultJSON, _ := json.Marshal(result)
	var forked struct {
		SessionID          string `json:"session_id"`
		ForkPointMessageID string `json:"fork_point_message_id"`
		CopiedCount        int    `json:"copied_count"`
	}
	if err := json.Unmarshal(resultJSON, &forked); err != nil {
		t.Fatal(err)
	}
	if forked.SessionID == "" || forked.SessionID == string(session.ID) || forked.ForkPointMessageID != "msg-2" || forked.CopiedCount != 2 {
		t.Fatalf("fork result = %s, want new session with msg-2 point and 2 copied rows", resultJSON)
	}
	// The child carries the requested title and the copied history.
	childGet, rpcErr := callControl(t, env.handler, "session/get", map[string]string{"session_id": forked.SessionID})
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	childJSON, _ := json.Marshal(childGet)
	var childEnvelope struct {
		Session sessionResult `json:"session"`
	}
	if err := json.Unmarshal(childJSON, &childEnvelope); err != nil {
		t.Fatal(err)
	}
	child := childEnvelope.Session
	if child.Title != "branch" {
		t.Fatalf("child session = %s, want title branch", childJSON)
	}
	childMsgs, rpcErr := callControl(t, env.handler, "session/messages", map[string]string{"session_id": forked.SessionID})
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	childMsgsJSON, _ := json.Marshal(childMsgs)
	if !strings.Contains(string(childMsgsJSON), `"one"`) || !strings.Contains(string(childMsgsJSON), `"two"`) {
		t.Fatalf("child messages = %s, want copied history", childMsgsJSON)
	}
	// The parent keeps its full view.
	parentMsgs, rpcErr := callControl(t, env.handler, "session/messages", map[string]string{"session_id": string(session.ID)})
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	parentMsgsJSON, _ := json.Marshal(parentMsgs)
	if !strings.Contains(string(parentMsgsJSON), `"one"`) || !strings.Contains(string(parentMsgsJSON), `"two"`) {
		t.Fatalf("parent messages = %s, want unfiltered original view", parentMsgsJSON)
	}
	if _, rpcErr := callControl(t, env.handler, "session/fork", map[string]string{"session_id": string(session.ID), "message_id": "msg-9"}); rpcErr == nil || rpcErr.Code != CodeNotFound {
		t.Fatalf("unknown fork point err = %v, want CodeNotFound", rpcErr)
	}
	if _, rpcErr := callControl(t, env.handler, "session/fork", map[string]string{"session_id": string(session.ID)}); rpcErr == nil || rpcErr.Code != InvalidParams {
		t.Fatalf("missing message_id err = %v, want InvalidParams", rpcErr)
	}
}
