package embedded

// W1 Task 4: context-aware bounded close. Successful release is
// distinguished from a timed-out wait whose teardown still owns resources.

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestCloseContextTimeoutNamesComponent(t *testing.T) {
	h := newQueueHost(8)
	release := make(chan struct{})
	var released atomic.Bool
	h.teardownHook = func() { <-release; released.Store(true) }

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Millisecond)
	defer cancel()
	err := h.CloseContext(ctx)
	if err == nil {
		t.Fatal("CloseContext returned before the deadline")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected deadline error, got %v", err)
	}
	if !strings.Contains(err.Error(), "control peer") {
		t.Fatalf("timeout must name the unfinished component, got %v", err)
	}
	// The host is closed to callers while teardown still runs.
	if _, callErr := h.Call(context.Background(), "system/ping", nil); !errors.Is(callErr, ErrClosed) {
		t.Fatalf("Call during teardown should fail closed, got %v", callErr)
	}
	// Releasing the hook lets a later Close observe completion.
	close(release)
	if err := h.Close(); err != nil {
		t.Fatalf("Close after released teardown: %v", err)
	}
	if !released.Load() {
		t.Fatal("teardown hook never ran")
	}
}

func TestCloseContextSuccessIsIdempotent(t *testing.T) {
	h := newQueueHost(8)
	if err := h.CloseContext(context.Background()); err != nil {
		t.Fatalf("CloseContext: %v", err)
	}
	if err := h.CloseContext(context.Background()); err != nil {
		t.Fatalf("second CloseContext: %v", err)
	}
	if err := h.Close(); err != nil {
		t.Fatalf("Close after CloseContext: %v", err)
	}
}

func TestCloseContextHonoursAnAlreadyExpiredDeadline(t *testing.T) {
	h := newQueueHost(8)
	release := make(chan struct{})
	defer close(release)
	h.teardownHook = func() { <-release }
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := h.CloseContext(ctx)
	if err == nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("expected cancellation, got %v", err)
	}
}
