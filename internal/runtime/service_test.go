package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/provider"
	"agent-vivy/internal/storage"
	"agent-vivy/internal/storage/sqlite"
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

	ts, err := tools.Builtin().Resolve([]string{tools.EchoInfoName})
	if err != nil {
		t.Fatalf("resolve tools: %v", err)
	}
	eng, err := NewEngine(ctx, WrapModel(model), ts, EngineConfig{StreamBuffer: 8, MaxEventPayloadBytes: 64 << 10})
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}
	sink := newTestSink()
	svc := NewService(eng, "mock", "mock-v0", ServiceDeps{
		Journal: backend, Runs: backend, Messages: backend, Sink: sink,
	})
	return svc, backend, sink
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
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		r, err := runs.GetRun(context.Background(), runID)
		if err == nil && r.Status == want {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("run %s never reached status %s", runID, want)
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
	svc, backend, sink := newTestService(t, provider.NewMock())
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

	// Seq monotonic 1..K with no gaps.
	for i, ev := range events {
		if ev.Seq != domain.EventSeq(i+1) {
			t.Fatalf("event %d has seq %d, want %d", i, ev.Seq, i+1)
		}
		if ev.PayloadVersion != 1 {
			t.Fatalf("event %d payload version = %d, want 1", i, ev.PayloadVersion)
		}
	}

	// Middle shape: delta* then model.completed before the terminal.
	var deltas strings.Builder
	var completedContent string
	for _, ev := range events[1 : len(events)-1] {
		switch ev.Type {
		case domain.EventModelDelta:
			deltas.WriteString(payloadDeltaOf(t, ev.Payload))
		case domain.EventModelCompleted:
			completedContent = payloadContentOf(t, ev.Payload)
		default:
			t.Fatalf("unexpected mid-run event %s", ev.Type)
		}
	}
	want := "mock reply to: hello vivy"
	if deltas.String() != want {
		t.Fatalf("reassembled deltas = %q, want %q", deltas.String(), want)
	}
	if completedContent != want {
		t.Fatalf("model.completed content = %q, want %q", completedContent, want)
	}

	// The conversation log mirrors the user turn and the assistant reply.
	msgs, err := backend.ListMessages(context.Background(), "sess-1")
	if err != nil {
		t.Fatalf("list messages: %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("messages = %d, want 2 (user + assistant)", len(msgs))
	}
	if msgs[0].Role != domain.RoleUser || msgs[0].Content != "hello vivy" || msgs[0].RunID != "" {
		t.Fatalf("user message = %+v", msgs[0])
	}
	if msgs[1].Role != domain.RoleAssistant || msgs[1].Content != want || msgs[1].RunID != runID {
		t.Fatalf("assistant message = %+v", msgs[1])
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

// The request context must not own the run: cancelling it (SSE disconnect,
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
	if !strings.Contains(string(events[0].Payload), `"content":"partial"`) {
		t.Fatalf("model.completed payload wrong: %s", events[0].Payload)
	}
	if again := m.onTurnEnd(); len(again) != 0 {
		t.Fatalf("second flush must be empty, got %+v", again)
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

// payload decode helpers keep the tests readable.

func payloadDeltaOf(t *testing.T, b []byte) string {
	t.Helper()
	var p payloadModelDelta
	mustUnmarshal(t, b, &p)
	return p.Delta
}

func payloadContentOf(t *testing.T, b []byte) string {
	t.Helper()
	var p payloadModelCompleted
	mustUnmarshal(t, b, &p)
	return p.Content
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
