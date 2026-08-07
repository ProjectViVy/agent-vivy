package runtime

import (
	"context"
	"testing"
	"time"
)

// E4: shutdown cancels every run and then drains the service before
// storage closes, so each run.cancelled terminal persists while the
// journal is still open.

// CancelAll followed by WaitIdle must drain a mid-tool run: the blocked
// drive exits once its cancel lands, and the terminal closes durably.
func TestServiceWaitIdleDrains(t *testing.T) {
	wait := &blockingTool{entered: make(chan struct{})}
	svc, backend := newCancelService(t, wait)
	ctx := context.Background()

	runID, err := svc.Run(ctx, "sess-1", "wait for me")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	select {
	case <-wait.entered:
	case <-time.After(5 * time.Second):
		t.Fatal("the blocking tool never started")
	}

	svc.CancelAll()
	drainCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if !svc.WaitIdle(drainCtx) {
		t.Fatal("WaitIdle must return true once CancelAll has drained the run")
	}
	assertCancelledClose(t, backend, runID)
}

// WaitIdle must report false while a drive goroutine is still in flight,
// so shutdown never closes storage underneath a live run.
func TestServiceWaitIdleTimesOut(t *testing.T) {
	wait := &blockingTool{entered: make(chan struct{})}
	svc, backend := newCancelService(t, wait)
	ctx := context.Background()

	runID, err := svc.Run(ctx, "sess-1", "wait for me")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	select {
	case <-wait.entered:
	case <-time.After(5 * time.Second):
		t.Fatal("the blocking tool never started")
	}

	shortCtx, shortCancel := context.WithTimeout(ctx, 50*time.Millisecond)
	defer shortCancel()
	if svc.WaitIdle(shortCtx) {
		t.Fatal("WaitIdle must report false while a run is still executing")
	}

	// Cleanup: cancel and drain so the test leaves no live goroutine.
	svc.CancelAll()
	drainCtx, drainCancel := context.WithTimeout(ctx, 5*time.Second)
	defer drainCancel()
	if !svc.WaitIdle(drainCtx) {
		t.Fatal("WaitIdle must drain after the cleanup cancel")
	}
	assertCancelledClose(t, backend, runID)
}
