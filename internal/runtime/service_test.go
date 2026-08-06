package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/provider"
	"agent-vivy/internal/storage"
	"agent-vivy/internal/storage/sqlite"
	"agent-vivy/internal/tools"
)

func newTestService(t *testing.T, model domain.ChatModel) (*Service, storage.Journal) {
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
	eng, err := NewEngine(ctx, model, ts, EngineConfig{StreamBuffer: 8, MaxEventPayloadBytes: 64 << 10})
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}
	return NewService(eng, backend, "mock", "mock-v0"), backend
}

func drainHandle(t *testing.T, h *RunHandle) []domain.RunEvent {
	t.Helper()
	var out []domain.RunEvent
	for ev := range h.Events {
		out = append(out, ev)
	}
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

func TestServiceRunHappyPath(t *testing.T) {
	svc, journal := newTestService(t, provider.NewMock())
	h, err := svc.Run(context.Background(), "sess-1", "hello vivy")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	events := drainHandle(t, h)

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

	// Replay from the journal must agree with the fanned-out channel.
	it, err := journal.Replay(context.Background(), h.RunID, 0)
	if err != nil {
		t.Fatalf("replay: %v", err)
	}
	defer it.Close()
	var replayed []domain.RunEvent
	for it.Next() {
		replayed = append(replayed, it.Value().Event)
	}
	if err := it.Err(); err != nil {
		t.Fatalf("replay iteration: %v", err)
	}
	if len(replayed) != len(events) {
		t.Fatalf("replay has %d events, channel had %d", len(replayed), len(events))
	}
	for i := range events {
		if replayed[i].Type != events[i].Type || replayed[i].Seq != events[i].Seq {
			t.Fatalf("replay/channel diverge at %d: %s/%d vs %s/%d",
				i, replayed[i].Type, replayed[i].Seq, events[i].Type, events[i].Seq)
		}
		if string(replayed[i].Payload) != string(events[i].Payload) {
			t.Fatalf("replay/channel payload diverge at %d", i)
		}
	}
}

// blockingModel blocks until ctx is done, then surfaces the cancellation.
type blockingModel struct{}

func (blockingModel) Stream(ctx context.Context, _ []*domain.Message) (domain.Stream[*domain.Message], error) {
	<-ctx.Done()
	return nil, ctx.Err()
}

func TestServiceRunCancelled(t *testing.T) {
	svc, journal := newTestService(t, blockingModel{})
	ctx, cancel := context.WithCancel(context.Background())

	h, err := svc.Run(ctx, "sess-1", "never finishes")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	cancel()

	events := drainHandle(t, h)
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

	// The journal must hold the same close.
	it, err := journal.Replay(context.Background(), h.RunID, 0)
	if err != nil {
		t.Fatalf("replay: %v", err)
	}
	defer it.Close()
	var lastType domain.EventType
	for it.Next() {
		lastType = it.Value().Event.Type
	}
	if lastType != domain.EventRunCancelled {
		t.Fatalf("journal last event = %s, want run.cancelled", lastType)
	}
}

// errorModel fails immediately, driving the run.failed path.
type errorModel struct{}

var errProviderBoom = errors.New("provider exploded")

func (errorModel) Stream(_ context.Context, _ []*domain.Message) (domain.Stream[*domain.Message], error) {
	return nil, errProviderBoom
}

func TestServiceRunFailed(t *testing.T) {
	svc, _ := newTestService(t, errorModel{})
	h, err := svc.Run(context.Background(), "sess-1", "boom")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	events := drainHandle(t, h)
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
