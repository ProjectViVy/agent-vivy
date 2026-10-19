package runtime

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
	"agent-vivy/internal/storage/sqlite"
	"agent-vivy/internal/tools"
)

func appendJournalEvents(t *testing.T, backend *sqlite.Backend, runID domain.RunID, events ...domain.RunEvent) {
	t.Helper()
	if _, err := backend.Append(context.Background(), storage.Commit{RunID: runID, Events: events}); err != nil {
		t.Fatalf("append journal events: %v", err)
	}
}

func toolRequestEvent(t *testing.T, id, name string) domain.RunEvent {
	t.Helper()
	payload, err := json.Marshal(payloadToolRequested{ToolCallID: id, ToolName: name})
	if err != nil {
		t.Fatalf("marshal tool request: %v", err)
	}
	return domain.RunEvent{Type: domain.EventToolRequested, CreatedAt: time.Now().UnixMilli(), PayloadVersion: 1, Payload: payload}
}

func toolFinishedEvent(t *testing.T, id, name string) domain.RunEvent {
	t.Helper()
	payload, err := json.Marshal(payloadToolFinished{ToolCallID: id, ToolName: name})
	if err != nil {
		t.Fatalf("marshal tool finished: %v", err)
	}
	return domain.RunEvent{Type: domain.EventToolFinished, CreatedAt: time.Now().UnixMilli(), PayloadVersion: 1, Payload: payload}
}

func plainEvent(t *testing.T, eventType domain.EventType) domain.RunEvent {
	t.Helper()
	return domain.RunEvent{Type: eventType, CreatedAt: time.Now().UnixMilli(), PayloadVersion: 1, Payload: json.RawMessage(`{}`)}
}

func newJournalProbeService(t *testing.T) (*Service, *sqlite.Backend) {
	t.Helper()
	backend, err := sqlite.Open(context.Background(), filepath.Join(t.TempDir(), "journal-order.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = backend.Close() })
	svc := NewService(nil, "scripted", "scripted-v0", ServiceDeps{Journal: backend})
	return svc, backend
}

// Regression for the resume batch split: admission-time events
// (policy.evaluated, tool.operation, approval lifecycle, tool.finished)
// interleave between a batch's tool.requested records, so grouping must be
// bounded by the model turn, not event adjacency.
func TestResumeBatchIDsFromJournalInterleavedEvents(t *testing.T) {
	svc, backend := newJournalProbeService(t)
	runID := domain.RunID("run-interleaved")
	appendJournalEvents(t, backend, runID,
		plainEvent(t, domain.EventRunStarted),
		plainEvent(t, domain.EventModelRequest),
		toolRequestEvent(t, "call-a", tools.WriteFileName),
		plainEvent(t, domain.EventPolicyEvaluated),
		toolRequestEvent(t, "call-b", tools.WriteFileName),
		plainEvent(t, domain.EventToolOperation),
		plainEvent(t, domain.EventToolApprovalRequired),
	)
	for _, call := range []string{"call-a", "call-b"} {
		got, err := svc.resumeBatchIDsFromJournal(context.Background(), runID, call)
		if err != nil {
			t.Fatalf("derive batch for %s: %v", call, err)
		}
		if len(got) != 2 || got[0] != "call-a" || got[1] != "call-b" {
			t.Fatalf("batch for %s = %v, want [call-a call-b]", call, got)
		}
	}
}

// A finished sibling and an approval event mid-batch must not split it, and
// the result keeps request order even when finishes land out of order.
func TestResumeBatchIDsFromJournalMixedSettledAndPending(t *testing.T) {
	svc, backend := newJournalProbeService(t)
	runID := domain.RunID("run-mixed")
	appendJournalEvents(t, backend, runID,
		plainEvent(t, domain.EventModelRequest),
		toolRequestEvent(t, "call-a", tools.EchoInfoName),
		toolRequestEvent(t, "call-b", tools.WriteFileName),
		toolFinishedEvent(t, "call-b", tools.WriteFileName),
		plainEvent(t, domain.EventPolicyEvaluated),
		plainEvent(t, domain.EventToolApprovalRequired),
	)
	got, err := svc.resumeBatchIDsFromJournal(context.Background(), runID, "call-a")
	if err != nil {
		t.Fatalf("derive batch: %v", err)
	}
	if len(got) != 2 || got[0] != "call-a" || got[1] != "call-b" {
		t.Fatalf("batch = %v, want [call-a call-b]", got)
	}
}

// model.request seals a turn: the suspended call resolves to its own turn's
// batch, never to an earlier turn's.
func TestResumeBatchIDsFromJournalTurnBoundary(t *testing.T) {
	svc, backend := newJournalProbeService(t)
	runID := domain.RunID("run-turns")
	appendJournalEvents(t, backend, runID,
		plainEvent(t, domain.EventModelRequest),
		toolRequestEvent(t, "old-a", tools.EchoInfoName),
		toolRequestEvent(t, "old-b", tools.EchoInfoName),
		toolFinishedEvent(t, "old-a", tools.EchoInfoName),
		toolFinishedEvent(t, "old-b", tools.EchoInfoName),
		plainEvent(t, domain.EventModelRequest),
		toolRequestEvent(t, "call-x", tools.WriteFileName),
		plainEvent(t, domain.EventPolicyEvaluated),
		toolRequestEvent(t, "call-y", tools.WriteFileName),
	)
	got, err := svc.resumeBatchIDsFromJournal(context.Background(), runID, "call-y")
	if err != nil {
		t.Fatalf("derive batch: %v", err)
	}
	if len(got) != 2 || got[0] != "call-x" || got[1] != "call-y" {
		t.Fatalf("batch = %v, want [call-x call-y]", got)
	}
	// A call names the most recent turn containing it: old-a resolves to its
	// own sealed batch, which is correct — a suspended call can only come
	// from the open turn, and a sealed turn's calls all settled.
	if got, err := svc.resumeBatchIDsFromJournal(context.Background(), runID, "old-a"); err != nil || len(got) != 2 || got[0] != "old-a" {
		t.Fatalf("sealed turn call = %v / %v, want [old-a old-b]", got, err)
	}
}

// Consecutive suspends inside one batch (approval chains) emit approval
// events but no model.request between resume legs; the batch stays whole.
func TestResumeBatchIDsFromJournalConsecutiveSuspends(t *testing.T) {
	svc, backend := newJournalProbeService(t)
	runID := domain.RunID("run-consecutive")
	appendJournalEvents(t, backend, runID,
		plainEvent(t, domain.EventModelRequest),
		toolRequestEvent(t, "call-a", tools.WriteFileName),
		toolRequestEvent(t, "call-b", tools.WriteFileName),
		plainEvent(t, domain.EventToolApprovalRequired),
		plainEvent(t, domain.EventToolApprovalDecided),
		toolFinishedEvent(t, "call-a", tools.WriteFileName),
		plainEvent(t, domain.EventToolApprovalRequired),
	)
	for _, call := range []string{"call-a", "call-b"} {
		got, err := svc.resumeBatchIDsFromJournal(context.Background(), runID, call)
		if err != nil {
			t.Fatalf("derive batch for %s: %v", call, err)
		}
		if len(got) != 2 || got[0] != "call-a" || got[1] != "call-b" {
			t.Fatalf("batch for %s = %v, want [call-a call-b]", call, got)
		}
	}
}

func TestResumeBatchIDsFromJournalMissingCall(t *testing.T) {
	svc, backend := newJournalProbeService(t)
	runID := domain.RunID("run-missing")
	appendJournalEvents(t, backend, runID,
		plainEvent(t, domain.EventModelRequest),
		toolRequestEvent(t, "call-a", tools.EchoInfoName),
	)
	got, err := svc.resumeBatchIDsFromJournal(context.Background(), runID, "call-unknown")
	if err != nil {
		t.Fatalf("derive batch: %v", err)
	}
	if got != nil {
		t.Fatalf("batch for unknown call = %v, want nil", got)
	}
}

func TestToolBatchOrderAwaitsRegistration(t *testing.T) {
	order := newToolBatchOrder()
	ctx := context.Background()
	done := make(chan []orderedToolCall, 1)
	go func() {
		earlier, err := order.awaitEarlier(ctx, "call-b")
		if err != nil {
			done <- nil
			return
		}
		done <- earlier
	}()
	select {
	case earlier := <-done:
		t.Fatalf("awaitEarlier returned %v before the batch was registered", earlier)
	case <-time.After(50 * time.Millisecond):
	}
	order.noteBatch([]orderedToolCall{
		{id: "call-a", name: tools.SubmitPlanName},
		{id: "call-b", name: tools.WriteFileName},
	})
	select {
	case earlier := <-done:
		if len(earlier) != 1 || earlier[0].id != "call-a" {
			t.Fatalf("earlier calls = %v, want [call-a]", earlier)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("awaitEarlier did not unblock after noteBatch")
	}
}

func TestToolBatchOrderDoneBarrier(t *testing.T) {
	order := newToolBatchOrder()
	order.noteBatch([]orderedToolCall{
		{id: "call-a", name: tools.EchoInfoName},
		{id: "call-b", name: tools.WriteFileName},
	})
	ctx := context.Background()
	done := make(chan error, 1)
	go func() { done <- order.awaitDone(ctx, "call-a") }()
	select {
	case err := <-done:
		t.Fatalf("awaitDone returned %v before signalDone", err)
	case <-time.After(50 * time.Millisecond):
	}
	order.signalDone("call-a")
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("awaitDone = %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("awaitDone did not unblock after signalDone")
	}
	// A waiter arriving after the signal must not block.
	if err := order.awaitDone(ctx, "call-a"); err != nil {
		t.Fatalf("awaitDone after settle = %v", err)
	}
}

func TestToolBatchOrderWaitsReleaseOnCancel(t *testing.T) {
	order := newToolBatchOrder()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := order.awaitEarlier(ctx, "never-registered")
		done <- err
	}()
	go func() { done <- order.awaitDone(ctx, "never-settled") }()
	cancel()
	for i := 0; i < 2; i++ {
		select {
		case err := <-done:
			if err == nil {
				t.Fatal("wait released without cancellation error")
			}
		case <-time.After(2 * time.Second):
			t.Fatal("wait not released on cancel")
		}
	}
}
