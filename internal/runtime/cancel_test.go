package runtime

import (
	"context"
	"encoding/json"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/cloudwego/eino/schema"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage/sqlite"
	"agent-vivy/internal/tools"
)

// E1 (AS-5): cancellation must produce exactly one run.cancelled terminal
// event in every lifecycle phase — pre-start, mid-stream, mid-tool — and
// no run.completed may ever follow. Pending-phase cancellation lives in
// approval_test.go.

const waitForeverName = "wait_forever"

// blockingTool is a readonly test tool that signals entry and then blocks
// until its context ends, giving the test a deterministic mid-tool window.
type blockingTool struct {
	entered chan struct{}
	once    sync.Once
}

func (b *blockingTool) Spec() domain.ToolSpec {
	return domain.ToolSpec{
		Name:        waitForeverName,
		Description: "Test-only: blocks until cancelled.",
		Readonly:    true,
	}
}

func (b *blockingTool) InvokableRun(ctx context.Context, _ json.RawMessage) (string, error) {
	b.once.Do(func() { close(b.entered) })
	<-ctx.Done()
	return "", ctx.Err()
}

// newCancelService wires a scripted model whose first turn requests the
// blocking readonly tool, over a fresh sqlite backend.
func newCancelService(t *testing.T, wait *blockingTool) (*Service, *sqlite.Backend) {
	t.Helper()
	ctx := context.Background()

	backend, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "cancel.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = backend.Close() })

	model := NewScriptedModel(
		schema.AssistantMessage("", []schema.ToolCall{{
			ID:       "call-wait-1",
			Function: schema.FunctionCall{Name: waitForeverName, Arguments: `{}`},
		}}),
		schema.AssistantMessage("unreachable", nil),
	)
	eng, err := NewEngine(ctx, model, []tools.Tool{wait}, EngineConfig{
		StreamBuffer: 8, MaxEventPayloadBytes: 64 << 10,
	})
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}
	svc := NewService(eng, "scripted", "scripted-v0", ServiceDeps{
		Journal: backend, Runs: backend, Messages: backend, Sink: newTestSink(),
	})
	return svc, backend
}

// assertCancelledClose verifies the AS-5 shape of one run's journal: the
// last event is run.cancelled with the user-requested reason, exactly one
// terminal exists, and no run.completed is present.
func assertCancelledClose(t *testing.T, backend *sqlite.Backend, runID domain.RunID) []domain.RunEvent {
	t.Helper()
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
	for _, ev := range events {
		if ev.Type == domain.EventRunCompleted {
			t.Fatal("run.completed must never follow a cancellation")
		}
	}
	return events
}

func TestServiceCancelMidTool(t *testing.T) {
	wait := &blockingTool{entered: make(chan struct{})}
	svc, backend := newCancelService(t, wait)
	ctx := context.Background()

	runID, err := svc.Run(ctx, "sess-1", "wait for me")
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	// Wait until the tool is actually executing before cancelling.
	select {
	case <-wait.entered:
	case <-time.After(5 * time.Second):
		t.Fatal("the blocking tool never started")
	}

	if !svc.Cancel(runID) {
		t.Fatal("cancel of a run executing a tool must report true")
	}
	waitForRunStatus(t, backend, runID, domain.RunCancelled)

	// The entered channel above is the deterministic mid-tool witness; the
	// journal may legitimately lose the tool.requested event when the
	// cancel races the persist of its preceding model delta.
	assertCancelledClose(t, backend, runID)
}

// Cancellation in the accepted/pre-start window: the run is registered in
// the active map before Run returns, so an immediate Cancel must still
// close it as cancelled (no model output can land first).
func TestServiceCancelPreStart(t *testing.T) {
	svc, backend, _ := newTestService(t, blockingModel{})
	runID, err := svc.Run(context.Background(), "sess-1", "cancel me instantly")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if !svc.Cancel(runID) {
		t.Fatal("cancel right after Run must report true")
	}
	waitForRunStatus(t, backend, runID, domain.RunCancelled)

	events := assertCancelledClose(t, backend, runID)
	for _, ev := range events {
		if ev.Type == domain.EventModelDelta || ev.Type == domain.EventModelCompleted {
			t.Fatalf("pre-start cancel leaked model output: %s", ev.Type)
		}
	}
}

// Concurrent cancels race the drive's own close: the journal guard keeps
// exactly one terminal no matter how many callers win the map (D-008).
func TestServiceCancelConcurrentIdempotent(t *testing.T) {
	svc, backend, _ := newTestService(t, blockingModel{})
	runID, err := svc.Run(context.Background(), "sess-1", "race the cancel")
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			svc.Cancel(runID)
		}()
	}
	wg.Wait()
	waitForRunStatus(t, backend, runID, domain.RunCancelled)
	assertCancelledClose(t, backend, runID)

	// A late cancel after the close reports false and appends nothing.
	if svc.Cancel(runID) {
		t.Fatal("cancel of an already-cancelled run must report false")
	}
	assertCancelledClose(t, backend, runID)
}
