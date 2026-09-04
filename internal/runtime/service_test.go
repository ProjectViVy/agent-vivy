package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/provider"
	"agent-vivy/internal/storage"
	"agent-vivy/internal/storage/sqlite"
	"agent-vivy/internal/testsupport"
	"agent-vivy/internal/tools"
)

func newTestService(t *testing.T, model domain.ChatModel) (*Service, *sqlite.Backend, *testSink) {
	t.Helper()
	ctx := context.Background()

	backend, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "journal.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = backend.Close() })

	ts, err := tools.Builtin(backend).Resolve([]string{tools.EchoInfoName})
	if err != nil {
		t.Fatalf("resolve tools: %v", err)
	}
	eng, err := NewEngine(ctx, WrapModel(model), ts, EngineConfig{StreamBuffer: 8, MaxEventPayloadBytes: 64 << 10})
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}
	sink := newTestSink()
	svc := NewService(eng, "test", "test-model", ServiceDeps{
		Journal: backend, Runs: backend, Messages: backend, Notes: backend, Sessions: backend, Sink: sink, Truncations: backend,
	})
	return svc, backend, sink
}

func TestChangeModelWhenIdleCommitsUnderRunStartupFence(t *testing.T) {
	svc := NewService(nil, "openai", "old-model", ServiceDeps{})
	persisted := false
	if err := svc.ChangeModelWhenIdle("anthropic", "new-model", func() error {
		persisted = true
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if !persisted {
		t.Fatal("model persistence callback was not called")
	}
	if providerName, modelID := svc.CurrentModel(); providerName != "anthropic" || modelID != "new-model" {
		t.Fatalf("current model = %q/%q", providerName, modelID)
	}

	sentinel := errors.New("save failed")
	if err := svc.ChangeModelWhenIdle("openai", "broken", func() error { return sentinel }); !errors.Is(err, sentinel) {
		t.Fatalf("persist failure = %v", err)
	}
	if providerName, modelID := svc.CurrentModel(); providerName != "anthropic" || modelID != "new-model" {
		t.Fatalf("failed persistence changed model = %q/%q", providerName, modelID)
	}

	svc.mu.Lock()
	svc.active["run-active"] = func() {}
	svc.mu.Unlock()
	called := false
	if err := svc.ChangeModelWhenIdle("openai", "later", func() error { called = true; return nil }); !errors.Is(err, ErrModelChangeBusy) {
		t.Fatalf("active run model change = %v", err)
	}
	if called {
		t.Fatal("busy model change called persistence")
	}
	svc.mu.Lock()
	delete(svc.active, "run-active")
	svc.snapshots["child-active"] = domain.PolicySnapshot{Profile: domain.PolicyProfileDefault, Hash: "hash"}
	svc.mu.Unlock()
	if err := svc.ChangeModelWhenIdle("openai", "later", func() error { return nil }); !errors.Is(err, ErrModelChangeBusy) {
		t.Fatalf("child worker model change = %v", err)
	}
	svc.cleanupRunState("child-active")
	if err := svc.ChangeModelWhenIdle("openai", "after-cleanup", func() error { return nil }); err != nil {
		t.Fatalf("cleanup left model selection permanently busy: %v", err)
	}
}

// testSink collects published events. The bus never delivers terminal
// events (it closes its subscribers instead), so the snapshot must stay
// terminal-free; the journal remains the place to assert the close.
type testSink struct {
	mu     sync.Mutex
	events []domain.RunEvent
}

func newTestSink() *testSink { return &testSink{} }

func (s *testSink) Publish(ev domain.RunEvent) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = append(s.events, ev)
}

func (s *testSink) snapshot() []domain.RunEvent {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]domain.RunEvent, len(s.events))
	copy(out, s.events)
	return out
}

func countTerminal(events []domain.RunEvent) int {
	n := 0
	for _, ev := range events {
		if ev.Type.Terminal() {
			n++
		}
	}
	return n
}

// waitForRunStatus polls the run row until it reaches want (bounded).
func waitForRunStatus(t *testing.T, runs storage.RunStore, runID domain.RunID, want domain.RunStatus) {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	var last domain.RunStatus
	for time.Now().Before(deadline) {
		r, err := runs.GetRun(context.Background(), runID)
		if err == nil {
			last = r.Status
			if r.Status == want {
				return
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("run %s never reached status %s (last status %s)", runID, want, last)
}

// replayAll drains the journal for one run.
func replayAll(t *testing.T, j storage.Journal, runID domain.RunID) []domain.RunEvent {
	t.Helper()
	it, err := j.Replay(context.Background(), runID, 0)
	if err != nil {
		t.Fatalf("replay: %v", err)
	}
	defer it.Close()
	var out []domain.RunEvent
	for it.Next() {
		out = append(out, it.Value().Event)
	}
	if err := it.Err(); err != nil {
		t.Fatalf("replay iteration: %v", err)
	}
	return out
}

func TestServiceRunHappyPath(t *testing.T) {
	svc, backend, sink := newTestService(t, testsupport.NewEchoModel())
	runID, err := svc.Run(context.Background(), "sess-1", "hello vivy")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	waitForRunStatus(t, backend, runID, domain.RunCompleted)

	// The terminal publish lands right after the status flip; wait for it
	// so the snapshot is complete.
	deadline := time.Now().Add(5 * time.Second)
	for countTerminal(sink.snapshot()) == 0 {
		if time.Now().After(deadline) {
			t.Fatal("terminal event was never published to the sink")
		}
		time.Sleep(10 * time.Millisecond)
	}

	live := sink.snapshot()
	if len(live) == 0 || live[0].Type != domain.EventRunStarted {
		t.Fatalf("first published event = %+v, want run.started", live)
	}
	if n := countTerminal(live); n != 1 {
		t.Fatalf("terminal events published to the sink = %d, want 1", n)
	}

	// The journal holds the full sequence including the single terminal.
	events := replayAll(t, backend, runID)
	if len(events) < 4 {
		t.Fatalf("expected at least 4 events, got %d", len(events))
	}
	if events[0].Type != domain.EventRunStarted {
		t.Fatalf("first event = %s, want run.started", events[0].Type)
	}
	last := events[len(events)-1]
	if last.Type != domain.EventRunCompleted {
		t.Fatalf("last event = %s, want run.completed", last.Type)
	}
	if n := countTerminal(events); n != 1 {
		t.Fatalf("terminal events = %d, want exactly 1", n)
	}

	// Seq monotonic 1..K with no gaps. model.completed is the one v2 payload:
	// its text is the preceding delta stream, while all other events remain
	// on the v1 envelope.
	for i, ev := range events {
		if ev.Seq != domain.EventSeq(i+1) {
			t.Fatalf("event %d has seq %d, want %d", i, ev.Seq, i+1)
		}
		wantVersion := 1
		if ev.Type == domain.EventModelCompleted {
			wantVersion = 2
		}
		if ev.PayloadVersion != wantVersion {
			t.Fatalf("event %d (%s) payload version = %d, want %d", i, ev.Type, ev.PayloadVersion, wantVersion)
		}
	}

	// Middle shape: delta* then metadata-only model.completed before the
	// terminal. Reassembly is byte-for-byte and independently checked against
	// the completion digest/length.
	var deltas strings.Builder
	var completed payloadModelCompletedV2
	for _, ev := range events[1 : len(events)-1] {
		switch ev.Type {
		case domain.EventModelRequest:
			// Request digest is recorded before the model stream.
		case domain.EventModelDelta:
			deltas.WriteString(payloadDeltaOf(t, ev.Payload))
		case domain.EventModelCompleted:
			completed = payloadCompletedOf(t, ev.Payload)
		default:
			t.Fatalf("unexpected mid-run event %s", ev.Type)
		}
	}
	want := "test response to: hello vivy"
	if deltas.String() != want {
		t.Fatalf("reassembled deltas = %q, want %q", deltas.String(), want)
	}
	if completed.ContentSHA256 != sha256Hex([]byte(want)) || completed.ByteLen != len([]byte(want)) {
		t.Fatalf("model.completed metadata = %+v, want sha/bytes for %q", completed, want)
	}

	// The conversation log mirrors the user turn and the assistant reply.
	msgs, err := backend.ListMessages(context.Background(), "sess-1")
	if err != nil {
		t.Fatalf("list messages: %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("messages = %d, want 2 (user + assistant)", len(msgs))
	}
	if msgs[0].Role != domain.RoleUser || msgs[0].Content != "hello vivy" || msgs[0].RunID != runID {
		t.Fatalf("user message = %+v", msgs[0])
	}
	if msgs[1].Role != domain.RoleAssistant || msgs[1].Content != want || msgs[1].RunID != runID {
		t.Fatalf("assistant message = %+v", msgs[1])
	}
}

// TestServiceRunBindsToolsOnKeywordlessRequest is the regression gate for
// the retired keyword tool selector: a keyword-less (here: Chinese) user
// message must still bind the full active tool surface on the outgoing
// model request, not an empty selection.
func TestServiceRunBindsToolsOnKeywordlessRequest(t *testing.T) {
	svc, backend, _ := newTestService(t, testsupport.NewEchoModel())
	ctx := context.Background()

	runID, err := svc.Run(ctx, "sess-zh-tools", "你现在有什么工具？")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	waitForRunStatus(t, backend, runID, domain.RunCompleted)

	var req payloadModelRequest
	found := false
	for _, ev := range replayAll(t, backend, runID) {
		if ev.Type == domain.EventModelRequest {
			mustUnmarshal(t, ev.Payload, &req)
			found = true
			break
		}
	}
	if !found {
		t.Fatal("run missing model.request")
	}
	if len(req.SelectedTools) == 0 {
		t.Fatal("keyword-less request bound no tools")
	}
	sawEcho := false
	for _, name := range req.SelectedTools {
		if name == tools.EchoInfoName {
			sawEcho = true
		}
	}
	if !sawEcho {
		t.Fatalf("selected tools %v missing %s", req.SelectedTools, tools.EchoInfoName)
	}
}

// blockingModel blocks until ctx is done, then surfaces the cancellation.
type blockingModel struct{}

func (blockingModel) Stream(ctx context.Context, _ []*domain.Message) (domain.Stream[*domain.Message], error) {
	<-ctx.Done()
	return nil, ctx.Err()
}

func TestServiceRunCancelled(t *testing.T) {
	svc, backend, _ := newTestService(t, blockingModel{})
	runID, err := svc.Run(context.Background(), "sess-1", "never finishes")
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	if !svc.Cancel(runID) {
		t.Fatal("cancel of an active run must report true")
	}
	if svc.Cancel("run-unknown") {
		t.Fatal("cancel of an unknown run must report false")
	}
	waitForRunStatus(t, backend, runID, domain.RunCancelled)

	events := replayAll(t, backend, runID)
	last := events[len(events)-1]
	if last.Type != domain.EventRunCancelled {
		t.Fatalf("last event = %s, want run.cancelled", last.Type)
	}
	if reason := payloadReasonOf(t, last.Payload); reason != reasonUserRequested {
		t.Fatalf("cancel reason = %q, want %q", reason, reasonUserRequested)
	}
	if n := countTerminal(events); n != 1 {
		t.Fatalf("terminal events = %d, want exactly 1", n)
	}

	// The journal freezes at the close: no event may be appended after the
	// terminal lands (AS-5).
	time.Sleep(100 * time.Millisecond)
	if again := replayAll(t, backend, runID); len(again) != len(events) {
		t.Fatalf("journal grew from %d to %d events after the terminal", len(events), len(again))
	}
}

func TestDeleteSessionSealsActiveRunAgainstResurrection(t *testing.T) {
	svc, backend, _ := newTestService(t, blockingModel{})
	ctx := context.Background()
	sessionID := domain.SessionID("sess-delete-active")
	if err := backend.CreateSession(ctx, domain.Session{ID: sessionID, Title: "delete me", CreatedAt: time.Now().UnixMilli()}); err != nil {
		t.Fatal(err)
	}
	runID, err := svc.Run(ctx, sessionID, "never finishes")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if err := svc.DeleteSession(ctx, sessionID); err != nil {
		t.Fatalf("delete session: %v", err)
	}
	drainCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if !svc.WaitIdle(drainCtx) {
		t.Fatal("deleted session run did not drain")
	}
	if _, err := backend.GetRun(ctx, runID); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("deleted run lookup error = %v, want not found", err)
	}
	msgs, err := backend.ListMessages(ctx, sessionID)
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 0 {
		t.Fatalf("deleted session was resurrected with messages: %+v", msgs)
	}
	if _, err := svc.Run(ctx, sessionID, "must stay deleted"); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("run after deletion error = %v, want not found", err)
	}
}

func TestDeleteSessionRejectsLateExternalWorkerEvent(t *testing.T) {
	svc, backend, _ := newTestService(t, testsupport.NewEchoModel())
	ctx := context.Background()
	sessionID := domain.SessionID("sess-delete-worker")
	if err := backend.CreateSession(ctx, domain.Session{ID: sessionID, Title: "worker", CreatedAt: time.Now().UnixMilli()}); err != nil {
		t.Fatal(err)
	}
	child := domain.Run{
		ID: "child-delete-late", SessionID: sessionID, Status: domain.RunActive,
		CreatedAt: time.Now().UnixMilli(), Kind: domain.RunKindChild,
	}
	if err := svc.CreateWorkerRun(ctx, child); err != nil {
		t.Fatal(err)
	}
	if err := svc.DeleteSession(ctx, sessionID); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.RecordExternalRunEvent(ctx, child.ID, domain.EventChildCompleted, map[string]any{"status": "late"}); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("late child event error = %v, want not found", err)
	}
	if err := svc.CreateWorkerRun(ctx, domain.Run{ID: "child-after-delete", SessionID: sessionID, Status: domain.RunAccepted, CreatedAt: time.Now().UnixMilli(), Kind: domain.RunKindChild}); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("child creation after delete error = %v, want not found", err)
	}
}

func TestDeleteSessionRacesWorkerCreationWithoutOrphans(t *testing.T) {
	svc, backend, _ := newTestService(t, testsupport.NewEchoModel())
	ctx := context.Background()
	sessionID := domain.SessionID("sess-delete-worker-race")
	if err := backend.CreateSession(ctx, domain.Session{ID: sessionID, Title: "worker race", CreatedAt: time.Now().UnixMilli()}); err != nil {
		t.Fatal(err)
	}
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			_ = svc.CreateWorkerRun(ctx, domain.Run{
				ID: domain.RunID(fmt.Sprintf("child-delete-race-%02d", i)), SessionID: sessionID,
				Status: domain.RunAccepted, CreatedAt: time.Now().UnixMilli(), Kind: domain.RunKindChild,
			})
		}(i)
	}
	close(start)
	if err := svc.DeleteSession(ctx, sessionID); err != nil {
		t.Fatal(err)
	}
	wg.Wait()
	runs, err := backend.ListRunsBySession(ctx, sessionID)
	if err != nil {
		t.Fatal(err)
	}
	if len(runs) != 0 {
		t.Fatalf("worker/delete race left orphan runs: %+v", runs)
	}
}

// The request context must not own the run: cancelling it (RPC disconnect,
// page refresh) leaves the run alive until Cancel is called (AS-7).
func TestServiceRunSurvivesRequestCancellation(t *testing.T) {
	svc, backend, _ := newTestService(t, blockingModel{})
	ctx, cancel := context.WithCancel(context.Background())
	runID, err := svc.Run(ctx, "sess-1", "keep going")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	cancel()

	time.Sleep(50 * time.Millisecond)
	r, err := backend.GetRun(context.Background(), runID)
	if err != nil {
		t.Fatalf("get run: %v", err)
	}
	if r.Status != domain.RunActive {
		t.Fatalf("run status after request cancel = %s, want active", r.Status)
	}

	if !svc.Cancel(runID) {
		t.Fatal("cancel of an active run must report true")
	}
	waitForRunStatus(t, backend, runID, domain.RunCancelled)
}

// errorModel fails immediately, driving the run.failed path.
type errorModel struct{}

var errProviderBoom = errors.New("provider exploded")

func (errorModel) Stream(_ context.Context, _ []*domain.Message) (domain.Stream[*domain.Message], error) {
	return nil, errProviderBoom
}

func TestServiceRunFailed(t *testing.T) {
	svc, backend, _ := newTestService(t, errorModel{})
	runID, err := svc.Run(context.Background(), "sess-1", "boom")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	waitForRunStatus(t, backend, runID, domain.RunFailed)

	events := replayAll(t, backend, runID)
	last := events[len(events)-1]
	if last.Type != domain.EventRunFailed {
		t.Fatalf("last event = %s, want run.failed", last.Type)
	}
	cat, msg := payloadFailureOf(t, last.Payload)
	if cat != causeInternalError {
		t.Fatalf("cause category = %q, want %q", cat, causeInternalError)
	}
	if msg == "" {
		t.Fatal("run.failed must carry a user-visible message")
	}
	if strings.Contains(msg, "provider exploded") {
		t.Fatalf("failure message leaks internals: %q", msg)
	}
	if n := countTerminal(events); n != 1 {
		t.Fatalf("terminal events = %d, want exactly 1", n)
	}
}

// keyMissingModel fails with the provider's typed KeyMissingError, which the
// engine may wrap on the way out; the terminal must still classify it as a
// provider failure with an actionable message.
type keyMissingModel struct{}

func (keyMissingModel) Stream(_ context.Context, _ []*domain.Message) (domain.Stream[*domain.Message], error) {
	return nil, &provider.KeyMissingError{Provider: "openai"}
}

type unconfiguredModel struct{}

func (unconfiguredModel) Stream(_ context.Context, _ []*domain.Message) (domain.Stream[*domain.Message], error) {
	return nil, fmt.Errorf("resolve active model: %w", provider.ErrModelNotConfigured)
}

func TestServiceRunFailedWithoutProvider(t *testing.T) {
	svc, backend, _ := newTestService(t, unconfiguredModel{})
	runID, err := svc.Run(context.Background(), "sess-1", "hello")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	waitForRunStatus(t, backend, runID, domain.RunFailed)

	events := replayAll(t, backend, runID)
	last := events[len(events)-1]
	if last.Type != domain.EventRunFailed {
		t.Fatalf("last event = %s, want run.failed", last.Type)
	}
	cat, msg := payloadFailureOf(t, last.Payload)
	if cat != causeProviderError {
		t.Fatalf("cause category = %q, want %q", cat, causeProviderError)
	}
	if msg != providerUnavailableMessage {
		t.Fatalf("failure message = %q, want %q", msg, providerUnavailableMessage)
	}
}

func TestServiceRunFailedKeyMissing(t *testing.T) {
	svc, backend, _ := newTestService(t, keyMissingModel{})
	runID, err := svc.Run(context.Background(), "sess-1", "hello")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	waitForRunStatus(t, backend, runID, domain.RunFailed)

	events := replayAll(t, backend, runID)
	last := events[len(events)-1]
	if last.Type != domain.EventRunFailed {
		t.Fatalf("last event = %s, want run.failed", last.Type)
	}
	cat, msg := payloadFailureOf(t, last.Payload)
	if cat != causeProviderError {
		t.Fatalf("cause category = %q, want %q", cat, causeProviderError)
	}
	if msg != providerUnavailableMessage {
		t.Fatalf("failure message = %q, want %q", msg, providerUnavailableMessage)
	}
	if strings.Contains(msg, "sk-") || strings.Contains(msg, "api_key") {
		t.Fatalf("failure message leaks a key value or field: %q", msg)
	}
}

// transportErrorModel fails with a wrapped network error; the terminal must
// classify it as provider transport, not an internal mystery.
type transportErrorModel struct{}

func (transportErrorModel) Stream(_ context.Context, _ []*domain.Message) (domain.Stream[*domain.Message], error) {
	return nil, fmt.Errorf("model stream recv: %w", errors.New("dial tcp 127.0.0.1:9999: connect: connection refused"))
}

func TestServiceRunFailedProviderTransport(t *testing.T) {
	svc, backend, _ := newTestService(t, transportErrorModel{})
	runID, err := svc.Run(context.Background(), "sess-1", "hello")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	waitForRunStatus(t, backend, runID, domain.RunFailed)

	events := replayAll(t, backend, runID)
	last := events[len(events)-1]
	if last.Type != domain.EventRunFailed {
		t.Fatalf("last event = %s, want run.failed", last.Type)
	}
	cat, msg := payloadFailureOf(t, last.Payload)
	if cat != causeProviderError {
		t.Fatalf("cause category = %q, want %q", cat, causeProviderError)
	}
	if msg != providerUnavailableMessage {
		t.Fatalf("failure message = %q, want %q", msg, providerUnavailableMessage)
	}
	if strings.Contains(msg, "127.0.0.1") || strings.Contains(msg, "dial tcp") {
		t.Fatalf("failure message leaks transport internals: %q", msg)
	}
}

func TestMapperToolCallAndResult(t *testing.T) {
	m := newEventMapper("run-test", 0)

	callEvents, err := m.onEvent(&adk.AgentEvent{
		Output: &adk.AgentOutput{MessageOutput: &adk.TypedMessageVariant[*schema.Message]{
			Message: &schema.Message{
				Role: schema.Assistant,
				ToolCalls: []schema.ToolCall{{
					ID:       "call-1",
					Function: schema.FunctionCall{Name: "echo_info", Arguments: `{"text":"hi"}`},
				}},
			},
		}},
	})
	if err != nil {
		t.Fatalf("map tool call: %v", err)
	}
	if len(callEvents) != 1 || callEvents[0].Type != domain.EventToolRequested {
		t.Fatalf("expected one tool.requested, got %+v", callEvents)
	}
	if !strings.Contains(string(callEvents[0].Payload), `"tool_name":"echo_info"`) {
		t.Fatalf("tool.requested payload missing tool name: %s", callEvents[0].Payload)
	}
	if !strings.Contains(string(callEvents[0].Payload), `"text":"hi"`) {
		t.Fatalf("tool.requested payload missing parsed args: %s", callEvents[0].Payload)
	}

	resultEvents, err := m.onEvent(&adk.AgentEvent{
		Output: &adk.AgentOutput{MessageOutput: &adk.TypedMessageVariant[*schema.Message]{
			Message: &schema.Message{Role: schema.Tool, Content: "hi", ToolCallID: "call-1"},
		}},
	})
	if err != nil {
		t.Fatalf("map tool result: %v", err)
	}
	if len(resultEvents) != 2 {
		t.Fatalf("expected tool.started + tool.finished, got %d events", len(resultEvents))
	}
	if resultEvents[0].Type != domain.EventToolStarted || resultEvents[1].Type != domain.EventToolFinished {
		t.Fatalf("unexpected tool event order: %s, %s", resultEvents[0].Type, resultEvents[1].Type)
	}
	if !strings.Contains(string(resultEvents[1].Payload), `"result":"hi"`) {
		t.Fatalf("tool.finished payload missing result: %s", resultEvents[1].Payload)
	}
}

func TestMapperTurnEndFlushesModelCompleted(t *testing.T) {
	m := newEventMapper("run-test", 0)
	m.pendingText.WriteString("partial")
	m.hasPending = true

	events := m.onTurnEnd()
	if len(events) != 1 || events[0].Type != domain.EventModelCompleted {
		t.Fatalf("expected flushed model.completed, got %+v", events)
	}
	if events[0].PayloadVersion != 2 {
		t.Fatalf("flushed model.completed payload version = %d, want 2", events[0].PayloadVersion)
	}
	completed := payloadCompletedOf(t, events[0].Payload)
	if completed.ContentSHA256 != sha256Hex([]byte("partial")) || completed.ByteLen != len([]byte("partial")) {
		t.Fatalf("model.completed payload wrong: %+v", completed)
	}
	if again := m.onTurnEnd(); len(again) != 0 {
		t.Fatalf("second flush must be empty, got %+v", again)
	}
}

func TestMapperFlushesAssistantTextBeforeToolAndFencesNextRound(t *testing.T) {
	m := newEventMapper("run-test", 0)
	m.pendingText.WriteString("before tool")
	m.hasPending = true
	events, err := m.onMessageEvent(&adk.TypedMessageVariant[*schema.Message]{Message: &schema.Message{
		Role:      schema.Assistant,
		ToolCalls: []schema.ToolCall{{ID: "call-1", Function: schema.FunctionCall{Name: "echo_info", Arguments: `{}`}}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].Type != domain.EventToolRequested {
		t.Fatalf("tool boundary events = %+v", events)
	}
	if m.hasPending || m.pendingText.Len() != 0 {
		t.Fatalf("already-durable tool preamble was not fenced: pending=%q", m.pendingText.String())
	}

	next, err := m.onMessageEvent(&adk.TypedMessageVariant[*schema.Message]{Message: &schema.Message{Role: schema.Assistant, Content: "final"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(next) != 2 || next[0].Type != domain.EventModelDelta || next[1].Type != domain.EventModelCompleted || strings.Contains(string(next[1].Payload), "before tool") {
		t.Fatalf("next round was contaminated: %+v", next)
	}
	if payloadDeltaOf(t, next[0].Payload) != "final" {
		t.Fatalf("next round delta = %q, want final", payloadDeltaOf(t, next[0].Payload))
	}
	completed := payloadCompletedOf(t, next[1].Payload)
	if completed.ContentSHA256 != sha256Hex([]byte("final")) || completed.ByteLen != len([]byte("final")) {
		t.Fatalf("next round completion = %+v", completed)
	}
}

func TestClampText(t *testing.T) {
	if got := clampText("hello", 1024); got != "hello" {
		t.Fatalf("small text must pass through, got %q", got)
	}
	long := strings.Repeat("x", 10000)
	got := clampText(long, 128)
	if len(got) >= len(long) {
		t.Fatal("clampText must shrink oversized input")
	}
}

func TestEventMapperClampsJSONEscapedToolResult(t *testing.T) {
	const budget = 4096
	m := newEventMapper("run-escaped", budget)
	result := strings.Repeat(`\"\\`, 8192)
	event := m.build(domain.EventToolFinished, payloadToolFinished{ToolCallID: "call", ToolName: tools.BashName, Result: result})
	if len(event.Payload) > budget {
		t.Fatalf("encoded payload = %d bytes, want <= %d", len(event.Payload), budget)
	}
}

func TestEventMapperBoundsCompleteToolFinishedPayload(t *testing.T) {
	const budget = 4096
	parts := []json.RawMessage{
		json.RawMessage(`{"type":"text","text":"` + strings.Repeat("a", 5000) + `"}`),
		json.RawMessage(`{"type":"text","text":"` + strings.Repeat("b", 5000) + `"}`),
	}
	event := newEventMapper("run-parts", budget).build(domain.EventToolFinished, payloadToolFinished{
		ToolCallID: "call", ToolName: "read_file", Result: "ok", Parts: parts,
	})
	if len(event.Payload) > budget {
		t.Fatalf("tool.finished payload = %d bytes, want <= %d", len(event.Payload), budget)
	}
	var payload payloadToolFinished
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Parts) != 0 || !strings.Contains(payload.Result, "parts omitted") {
		t.Fatalf("oversized parts were not explicitly omitted: %+v", payload)
	}
}

func TestEventMapperBoundsApprovalReviewPayloadBeforeJournal(t *testing.T) {
	const budget = 8192
	m := newEventMapper("run-approval-bound", budget)
	event := m.build(domain.EventToolApprovalRequired, payloadToolApprovalRequired{
		ApprovalID: "approval-1", ToolCallID: "call-1", ToolName: "write_file", Action: "write_file",
		Args:    map[string]any{"content": strings.Repeat(`"\\`, 10000)},
		Preview: strings.Repeat("界", 100000), RiskFindings: []string{strings.Repeat("risk", 10000)},
	})
	if len(event.Payload) > budget {
		t.Fatalf("approval payload = %d bytes, want <= %d", len(event.Payload), budget)
	}
	var payload payloadToolApprovalRequired
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		t.Fatal(err)
	}
	if payload.ApprovalID != "approval-1" || payload.ToolCallID != "call-1" {
		t.Fatalf("approval identity was lost: %+v", payload)
	}
	if !strings.Contains(payload.Preview, "omitted") && !strings.Contains(payload.Preview, "truncated") {
		t.Fatalf("oversized preview was not marked: %q", payload.Preview)
	}
}

func TestBoundedApprovalReviewMatchesStoredAndJournalProjection(t *testing.T) {
	const budget = 64 << 10
	proposal := boundToolProposalReview(domain.ToolProposal{
		Action: "write_file", Target: "src/sk-live-abcdefghijkl/main.go", PreconditionHash: strings.Repeat("a", 64),
		Preview: "token sk-live-abcdefghijkl\n" + strings.Repeat("界", 50000), RiskFindings: []string{"api_key='private value' " + strings.Repeat("risk", 10000)},
	}, budget)
	if strings.Contains(proposal.Target+proposal.Preview+strings.Join(proposal.RiskFindings, ""), "sk-live") || strings.Contains(strings.Join(proposal.RiskFindings, ""), "private value") {
		t.Fatalf("stored approval review retained secret canary: %+v", proposal)
	}
	event := newEventMapper("run-review-parity", budget).build(domain.EventToolApprovalRequired, payloadToolApprovalRequired{
		ApprovalID: "approval-1", ToolCallID: "call-1", ToolName: "write_file", Args: map[string]any{"content": strings.Repeat("x", 100000)},
		Action: proposal.Action, Target: proposal.Target, PreconditionHash: proposal.PreconditionHash,
		Preview: proposal.Preview, RiskFindings: proposal.RiskFindings,
	})
	var journal payloadToolApprovalRequired
	if err := json.Unmarshal(event.Payload, &journal); err != nil {
		t.Fatal(err)
	}
	if journal.Preview != proposal.Preview || !reflect.DeepEqual(journal.RiskFindings, proposal.RiskFindings) {
		t.Fatalf("stored review and Journal diverged: stored preview=%d risks=%#v journal preview=%d risks=%#v", len(proposal.Preview), proposal.RiskFindings, len(journal.Preview), journal.RiskFindings)
	}
}

func TestEventMapperHonorsMinimumConfiguredPayloadBudget(t *testing.T) {
	const budget = 1024
	m := newEventMapper("run-min-budget", budget)
	events := []domain.RunEvent{
		m.build(domain.EventToolFinished, payloadToolFinished{ToolCallID: strings.Repeat("c", 500), ToolName: strings.Repeat("t", 5000), Result: strings.Repeat("r", 5000), Parts: []json.RawMessage{json.RawMessage(`{"text":"` + strings.Repeat("p", 5000) + `"}`)}}),
		m.build(domain.EventToolApprovalRequired, payloadToolApprovalRequired{ApprovalID: strings.Repeat("a", 5000), ToolCallID: strings.Repeat("c", 5000), ToolName: strings.Repeat("w", 5000), Face: strings.Repeat("f", 5000), Args: map[string]any{"secret": strings.Repeat("x", 5000)}, Preview: strings.Repeat("p", 5000), RiskFindings: []string{strings.Repeat("r", 5000)}}),
	}
	for _, event := range events {
		if len(event.Payload) > budget {
			t.Fatalf("%s payload = %d bytes, want <= %d: %s", event.Type, len(event.Payload), budget, event.Payload)
		}
	}
}

// capturingModel records the message list of every Stream call and
// answers a fixed reply, so tests can assert exactly what the engine fed
// the model (MA-1).
type capturingModel struct {
	mu     sync.Mutex
	inputs [][]domain.Message
}

func (c *capturingModel) Stream(_ context.Context, in []*domain.Message) (domain.Stream[*domain.Message], error) {
	c.mu.Lock()
	cp := make([]domain.Message, 0, len(in))
	for _, m := range in {
		cp = append(cp, *m)
	}
	c.inputs = append(c.inputs, cp)
	c.mu.Unlock()
	return &captureStream{chunks: []*domain.Message{{Role: domain.RoleAssistant, Content: "captured reply"}}}, nil
}

func (c *capturingModel) calls() [][]domain.Message {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([][]domain.Message, len(c.inputs))
	copy(out, c.inputs)
	return out
}

// captureStream replays fixed reply chunks for the stream tests.
type captureStream struct {
	chunks []*domain.Message
	next   int
}

type gatedIncrementalModel struct {
	secondRecv chan struct{}
	release    chan struct{}
}

func (m *gatedIncrementalModel) Stream(context.Context, []*domain.Message) (domain.Stream[*domain.Message], error) {
	return &gatedIncrementalStream{secondRecv: m.secondRecv, release: m.release}, nil
}

type gatedIncrementalStream struct {
	next       int
	secondRecv chan struct{}
	release    chan struct{}
}

func (s *gatedIncrementalStream) Recv() (*domain.Message, error) {
	if s.next < 9 {
		content := "x"
		if s.next == 0 {
			content = "这"
		}
		s.next++
		return &domain.Message{Role: domain.RoleAssistant, Content: content}, nil
	}
	switch s.next {
	case 9:
		s.next++
		close(s.secondRecv)
		<-s.release
		return &domain.Message{Role: domain.RoleAssistant, Content: "是一句话"}, nil
	default:
		return nil, io.EOF
	}
}

func TestServicePersistsFirstModelChunkBeforeProviderEOF(t *testing.T) {
	model := &gatedIncrementalModel{secondRecv: make(chan struct{}), release: make(chan struct{})}
	released := false
	defer func() {
		if !released {
			close(model.release)
		}
	}()
	svc, backend, _ := newTestService(t, model)
	runID, err := svc.Run(context.Background(), "sess-incremental", "stream")
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-model.secondRecv:
	case <-time.After(2 * time.Second):
		t.Fatal("provider never requested its second chunk")
	}
	seenFirst := false
	var events []domain.RunEvent
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && !seenFirst {
		events = replayAll(t, backend, runID)
		for _, event := range events {
			if event.Type == domain.EventModelDelta && payloadDeltaOf(t, event.Payload) == "这" {
				seenFirst = true
			}
		}
		if !seenFirst {
			time.Sleep(5 * time.Millisecond)
		}
	}
	if !seenFirst {
		t.Fatalf("first chunk was not durable while provider remained open: %+v", events)
	}
	close(model.release)
	released = true
	waitForRunStatus(t, backend, runID, domain.RunCompleted)
	var got strings.Builder
	for _, event := range replayAll(t, backend, runID) {
		if event.Type == domain.EventModelDelta {
			got.WriteString(payloadDeltaOf(t, event.Payload))
		}
	}
	want := "这" + strings.Repeat("x", 8) + "是一句话"
	if got.String() != want {
		t.Fatalf("durable deltas = %q, want exactly-once provider text %q", got.String(), want)
	}
}

func (s *captureStream) Recv() (*domain.Message, error) {
	if s.next >= len(s.chunks) {
		return nil, io.EOF
	}
	chunk := s.chunks[s.next]
	s.next++
	return chunk, nil
}

// userAssistantPairs strips the leading run context: the static
// instruction and the per-run preamble both cross the adapter boundary
// as system messages, which the three-role domain vocabulary collapses
// to the assistant role, so every assistant entry before the first user
// message is leading context, not transcript.
func userAssistantPairs(msgs []domain.Message) [][2]string {
	var out [][2]string
	for _, m := range msgs {
		switch m.Role {
		case domain.RoleUser, domain.RoleAssistant:
			out = append(out, [2]string{string(m.Role), m.Content})
		}
	}
	start := 0
	for start < len(out) && out[start][0] == string(domain.RoleAssistant) {
		start++
	}
	return out[start:]
}

// The second run of a session must carry the first turn's transcript:
// without the feed every turn is stateless (docs/v1-minimal-agent-proposal.md §1).
func TestServiceFeedsSessionHistory(t *testing.T) {
	cm := &capturingModel{}
	svc, backend, _ := newTestService(t, cm)

	run1, err := svc.Run(context.Background(), "sess-h", "remember the code word bluebird")
	if err != nil {
		t.Fatalf("run 1: %v", err)
	}
	waitForRunStatus(t, backend, run1, domain.RunCompleted)

	run2, err := svc.Run(context.Background(), "sess-h", "what is the code word?")
	if err != nil {
		t.Fatalf("run 2: %v", err)
	}
	waitForRunStatus(t, backend, run2, domain.RunCompleted)

	calls := cm.calls()
	if len(calls) != 2 {
		t.Fatalf("model calls = %d, want 2", len(calls))
	}
	first := userAssistantPairs(calls[0])
	wantFirst := [][2]string{{"user", "remember the code word bluebird"}}
	if len(first) != len(wantFirst) || first[0] != wantFirst[0] {
		t.Fatalf("first run feed = %v, want %v", first, wantFirst)
	}
	second := userAssistantPairs(calls[1])
	wantSecond := [][2]string{
		{"user", "remember the code word bluebird"},
		{"assistant", "captured reply"},
		{"user", "what is the code word?"},
	}
	if len(second) != len(wantSecond) {
		t.Fatalf("second run feed = %v, want %v", second, wantSecond)
	}
	for i := range wantSecond {
		if second[i] != wantSecond[i] {
			t.Fatalf("second run feed[%d] = %v, want %v", i, second[i], wantSecond[i])
		}
	}
}

// History feeds are per-session: another session's transcript must never
// leak into the feed (multi-session isolation).
func TestServiceHistoryIsolatedAcrossSessions(t *testing.T) {
	cm := &capturingModel{}
	svc, backend, _ := newTestService(t, cm)

	run1, err := svc.Run(context.Background(), "sess-a", "a speaks first")
	if err != nil {
		t.Fatalf("run sess-a: %v", err)
	}
	waitForRunStatus(t, backend, run1, domain.RunCompleted)

	run2, err := svc.Run(context.Background(), "sess-b", "b speaks second")
	if err != nil {
		t.Fatalf("run sess-b: %v", err)
	}
	waitForRunStatus(t, backend, run2, domain.RunCompleted)

	calls := cm.calls()
	if len(calls) != 2 {
		t.Fatalf("model calls = %d, want 2", len(calls))
	}
	second := userAssistantPairs(calls[1])
	want := [][2]string{{"user", "b speaks second"}}
	if len(second) != len(want) || second[0] != want[0] {
		t.Fatalf("sess-b feed = %v, want exactly %v (no sess-a leakage)", second, want)
	}
}

// Every run's feed must be led by the per-run preamble (MA-2): persona,
// current date, and the resolved tool set, ahead of any history.
func TestServiceRunLeadsWithPreamble(t *testing.T) {
	cm := &capturingModel{}
	svc, backend, _ := newTestService(t, cm)

	runID, err := svc.Run(context.Background(), "sess-p", "echo hello")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	waitForRunStatus(t, backend, runID, domain.RunCompleted)

	calls := cm.calls()
	if len(calls) != 1 {
		t.Fatalf("model calls = %d, want 1", len(calls))
	}
	feed := calls[0]
	if len(feed) < 2 {
		t.Fatalf("feed too short: %+v", feed)
	}
	// The adapter collapses system roles to assistant. The stable instruction
	// must precede the dynamic per-run preamble and the first user message.
	static := feed[0]
	if static.Role != domain.RoleAssistant || !strings.HasPrefix(static.Content, preamblePersona) {
		t.Fatalf("feed[0] = %+v, want stable instruction leading with %q", static, preamblePersona)
	}
	dynamic := feed[1]
	if !strings.Contains(dynamic.Content, "Today's date: ") {
		t.Fatalf("dynamic preamble missing date: %q", dynamic.Content)
	}
	if strings.Contains(static.Content, "echo_info") {
		t.Fatalf("static instruction must not contain request-scoped tool names: %q", static.Content)
	}
	for _, marker := range []string{"echo_info", "read-only; runs automatically"} {
		if !strings.Contains(dynamic.Content, marker) {
			t.Fatalf("dynamic preamble missing %q: %q", marker, dynamic.Content)
		}
	}
	last := feed[len(feed)-1]
	if last.Role != domain.RoleUser || last.Content != "echo hello" {
		t.Fatalf("feed must end with the user message, got %+v", last)
	}
}

// Saved notes surface in the preamble as a bounded digest (MA-3): the
// model sees recent note ids and first lines without asking.
func TestServicePreambleCarriesNotesDigest(t *testing.T) {
	cm := &capturingModel{}
	svc, backend, _ := newTestService(t, cm)

	if err := backend.AppendNote(context.Background(), domain.Note{
		ID: "note_digest", Content: "code word is bluebird\nsecond line", CreatedAt: time.Now().UnixMilli(),
	}); err != nil {
		t.Fatalf("seed note: %v", err)
	}

	runID, err := svc.Run(context.Background(), "sess-n", "hello")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	waitForRunStatus(t, backend, runID, domain.RunCompleted)

	calls := cm.calls()
	if len(calls) != 1 {
		t.Fatalf("model calls = %d, want 1", len(calls))
	}
	preamble := calls[0][1].Content
	for _, marker := range []string{"Recent notes from the user's notebook:", "note_digest", "code word is bluebird"} {
		if !strings.Contains(preamble, marker) {
			t.Fatalf("preamble missing %q: %q", marker, preamble)
		}
	}
	if strings.Contains(preamble, "second line") {
		t.Fatalf("digest must collapse each note to its first line: %q", preamble)
	}
}

// payload decode helpers keep the tests readable.

func payloadDeltaOf(t *testing.T, b []byte) string {
	t.Helper()
	var p payloadModelDelta
	mustUnmarshal(t, b, &p)
	return p.Delta
}

func payloadCompletedOf(t *testing.T, b []byte) payloadModelCompletedV2 {
	t.Helper()
	var p payloadModelCompletedV2
	mustUnmarshal(t, b, &p)
	return p
}

func payloadReasonOf(t *testing.T, b []byte) string {
	t.Helper()
	var p payloadRunCancelled
	mustUnmarshal(t, b, &p)
	return p.Reason
}

func payloadFailureOf(t *testing.T, b []byte) (string, string) {
	t.Helper()
	var p payloadRunFailed
	mustUnmarshal(t, b, &p)
	return p.CauseCategory, p.Message
}

func mustUnmarshal(t *testing.T, b []byte, v any) {
	t.Helper()
	if err := json.Unmarshal(b, v); err != nil {
		t.Fatalf("decode payload %s: %v", b, err)
	}
}

type recordingChatModel struct {
	inner  model.ToolCallingChatModel
	mu     sync.Mutex
	inputs [][]*schema.Message
}

func (m *recordingChatModel) Generate(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	m.record(input)
	return m.inner.Generate(ctx, input, opts...)
}

func (m *recordingChatModel) Stream(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	m.record(input)
	return m.inner.Stream(ctx, input, opts...)
}

func (m *recordingChatModel) WithTools(tools []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	next, err := m.inner.WithTools(tools)
	if err != nil {
		return nil, err
	}
	m.inner = next
	return m, nil
}

func (m *recordingChatModel) record(input []*schema.Message) {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := make([]*schema.Message, 0, len(input))
	for _, msg := range input {
		if msg == nil {
			continue
		}
		clone := *msg
		if len(msg.ToolCalls) > 0 {
			clone.ToolCalls = append([]schema.ToolCall(nil), msg.ToolCalls...)
		}
		cp = append(cp, &clone)
	}
	m.inputs = append(m.inputs, cp)
}

func (m *recordingChatModel) lastInput() []*schema.Message {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.inputs) == 0 {
		return nil
	}
	return m.inputs[len(m.inputs)-1]
}

func TestServiceFeedsToolTraceAndRequestDigest(t *testing.T) {
	ctx := context.Background()
	backend, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "tool-feed.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = backend.Close() })

	recorder := &recordingChatModel{inner: NewScriptedModel(
		schema.AssistantMessage("", []schema.ToolCall{{
			ID:       "call-echo-1",
			Function: schema.FunctionCall{Name: tools.EchoInfoName, Arguments: `{"text":"hi"}`},
		}}),
		schema.AssistantMessage("echoed", nil),
		schema.AssistantMessage("second turn", nil),
	)}
	ts, err := tools.Builtin(backend).Resolve([]string{tools.EchoInfoName})
	if err != nil {
		t.Fatalf("resolve tools: %v", err)
	}
	eng, err := NewEngine(ctx, recorder, ts, EngineConfig{StreamBuffer: 8, MaxEventPayloadBytes: 64 << 10})
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}
	svc := NewService(eng, "scripted", "scripted-v0", ServiceDeps{
		Journal: backend, Runs: backend, Messages: backend, Notes: backend, Sink: newTestSink(),
	})

	run1, err := svc.Run(ctx, "sess-tools", "please echo")
	if err != nil {
		t.Fatalf("run 1: %v", err)
	}
	waitForRunStatus(t, backend, run1, domain.RunCompleted)

	stored, err := backend.ListMessages(ctx, "sess-tools")
	if err != nil {
		t.Fatalf("list messages: %v", err)
	}
	var sawCall, sawResult bool
	for _, msg := range stored {
		if msg.Role == domain.RoleAssistant && msg.ToolCallID == "call-echo-1" {
			sawCall = true
		}
		if msg.Role == domain.RoleTool && msg.ToolCallID == "call-echo-1" {
			sawResult = true
		}
	}
	if !sawCall || !sawResult {
		t.Fatalf("message projection missing tool turn: %+v", stored)
	}

	run2, err := svc.Run(ctx, "sess-tools", "what did you echo?")
	if err != nil {
		t.Fatalf("run 2: %v", err)
	}
	waitForRunStatus(t, backend, run2, domain.RunCompleted)

	feed := recorder.lastInput()
	var sawToolRole bool
	for _, msg := range feed {
		if msg.Role == schema.Tool && msg.ToolCallID == "call-echo-1" {
			sawToolRole = true
		}
	}
	if !sawToolRole {
		t.Fatalf("second-run feed missing tool result: %+v", feed)
	}

	rebuilt, _, err := buildRunContext(ContextPolicy{}, "unused-preamble", stored, "what did you echo?")
	if err != nil {
		t.Fatalf("rebuild context: %v", err)
	}

	events := replayAll(t, backend, run2)
	var req payloadModelRequest
	found := false
	for _, ev := range events {
		if ev.Type == domain.EventModelRequest {
			mustUnmarshal(t, ev.Payload, &req)
			found = true
			break
		}
	}
	if !found {
		t.Fatal("second run missing model.request")
	}
	gotBody := stripSystemRequestRows(req.Messages)
	wantBody := stripSystemRequestRows(digestModelRequest(rebuilt, nil).Messages)
	if len(gotBody) != len(wantBody) {
		t.Fatalf("model.request body = %+v, want %+v", gotBody, wantBody)
	}
	for i := range wantBody {
		if gotBody[i].Role != wantBody[i].Role || gotBody[i].ContentSHA256 != wantBody[i].ContentSHA256 || gotBody[i].ToolCallID != wantBody[i].ToolCallID {
			t.Fatalf("model.request body[%d] = %+v, want %+v", i, gotBody[i], wantBody[i])
		}
	}
}

func stripSystemRequestRows(in []payloadModelRequestMessage) []payloadModelRequestMessage {
	out := make([]payloadModelRequestMessage, 0, len(in))
	for _, row := range in {
		if row.Role == "system" {
			continue
		}
		out = append(out, row)
	}
	return out
}

// Streaming chunk events must not consume the run events budget: one mapped
// event per streamed chunk makes any substantive reply exceed MaxEvents on
// its own (TT-4). Semantic events keep charging, and the model-call /
// tool-call budgets remain the runaway guard.
func TestReserveMappedBudgetSkipsStreamingDeltas(t *testing.T) {
	ledger, err := NewBudgetLedger(BudgetPolicy{MaxEvents: 5, MaxModelCalls: 5, MaxToolCalls: 5, MaxRetries: 5})
	if err != nil {
		t.Fatal(err)
	}
	deltas := make([]domain.RunEvent, 0, 600)
	for i := 0; i < 600; i++ {
		deltas = append(deltas, domain.RunEvent{Type: domain.EventModelDelta})
	}
	if err := reserveMappedBudget(ledger, deltas); err != nil {
		t.Fatalf("600 streamed deltas must not consume the events budget: %v", err)
	}
	if err := reserveMappedBudget(ledger, []domain.RunEvent{{Type: domain.EventModelCompleted}}); err != nil {
		t.Fatalf("semantic events keep charging: %v", err)
	}
	semantic := make([]domain.RunEvent, 0, 6)
	for i := 0; i < 6; i++ {
		semantic = append(semantic, domain.RunEvent{Type: domain.EventModelUsage})
	}
	if err := reserveMappedBudget(ledger, semantic); !errors.Is(err, ErrBudgetExceeded) {
		t.Fatalf("events budget must still trip on semantic events, got: %v", err)
	}
}
