package embedded

// W1 Task 3: blocking Next semantics on the bounded notification queue.
// These tests drive Handle (the control-peer producer) and Next directly so
// no app composition is needed.

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	controlrpc "agent-vivy/internal/rpc"
)

func newQueueHost(capacity int) *Host {
	return &Host{
		events:    make(chan Notification, capacity),
		notify:    make(chan struct{}, 1),
		closeCh:   make(chan struct{}),
		closeDone: make(chan struct{}),
		cancel:    func() {},
	}
}

func produce(t *testing.T, h *Host, method string) {
	t.Helper()
	if _, rpcErr := h.Handle(context.Background(), nil, controlrpc.Request{
		Method: method,
		Params: json.RawMessage(`{"seq":1}`),
	}); rpcErr != nil {
		t.Fatalf("Handle rejected %q: %v", method, rpcErr)
	}
}

func TestNextBlocksUntilFirstNotification(t *testing.T) {
	h := newQueueHost(8)
	type result struct {
		out PollResult
		err error
	}
	done := make(chan result, 1)
	go func() {
		out, err := h.Next(context.Background(), 0)
		done <- result{out, err}
	}()
	select {
	case <-done:
		t.Fatal("Next returned before any notification")
	case <-time.After(60 * time.Millisecond):
	}
	produce(t, h, "run/event")
	select {
	case got := <-done:
		if got.err != nil {
			t.Fatalf("Next: %v", got.err)
		}
		if len(got.out.Events) != 1 || got.out.Events[0].Method != "run/event" {
			t.Fatalf("unexpected batch %+v", got.out)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Next did not wake for the first notification")
	}
}

func TestNextEmptyWaitCancelledByContext(t *testing.T) {
	h := newQueueHost(8)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, err := h.Next(ctx, 10)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected deadline error, got %v", err)
	}
	if time.Since(start) > 2*time.Second {
		t.Fatal("Next ignored the caller deadline")
	}
}

func TestNextDrainsMaxBatch(t *testing.T) {
	h := newQueueHost(64)
	for i := 0; i < 10; i++ {
		produce(t, h, "run/event")
	}
	out, err := h.Next(context.Background(), 3)
	if err != nil {
		t.Fatalf("Next: %v", err)
	}
	if len(out.Events) != 3 {
		t.Fatalf("expected 3 events, got %d", len(out.Events))
	}
	out, err = h.Next(context.Background(), 0)
	if err != nil {
		t.Fatalf("Next: %v", err)
	}
	if len(out.Events) != 7 {
		t.Fatalf("expected remaining 7 events, got %d", len(out.Events))
	}
}

func TestNextOverflowReportsStickyGap(t *testing.T) {
	h := newQueueHost(2)
	produce(t, h, "first")
	produce(t, h, "second")
	produce(t, h, "third")
	produce(t, h, "fourth")
	out, err := h.Next(context.Background(), 0)
	if err != nil {
		t.Fatalf("Next: %v", err)
	}
	if !out.Gap {
		t.Fatal("overflow did not latch the gap flag")
	}
	if len(out.Events) != 2 || out.Events[0].Method != "third" || out.Events[1].Method != "fourth" {
		t.Fatalf("expected oldest events dropped, got %+v", out.Events)
	}
	// The gap is reported exactly once.
	if h.gap.Load() {
		t.Fatal("gap flag was not consumed by the drain")
	}
	produce(t, h, "fifth")
	out, err = h.Next(context.Background(), 0)
	if err != nil {
		t.Fatalf("Next: %v", err)
	}
	if out.Gap {
		t.Fatal("gap reported twice")
	}
}

func TestNextGapOnlyWakeup(t *testing.T) {
	h := newQueueHost(8)
	h.gap.Store(true)
	h.wake()
	out, err := h.Next(context.Background(), 0)
	if err != nil {
		t.Fatalf("Next: %v", err)
	}
	if !out.Gap || len(out.Events) != 0 {
		t.Fatalf("expected gap-only batch, got %+v", out)
	}
}

func TestNextCloseWhileWaiting(t *testing.T) {
	h := newQueueHost(8)
	done := make(chan error, 1)
	go func() {
		_, err := h.Next(context.Background(), 0)
		done <- err
	}()
	select {
	case <-done:
		t.Fatal("Next returned before close")
	case <-time.After(60 * time.Millisecond):
	}
	if err := h.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	select {
	case err := <-done:
		if !errors.Is(err, ErrClosed) {
			t.Fatalf("expected ErrClosed, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("blocked Next was not released by Close")
	}
}

func TestNextDrainsBufferedEventsOnClose(t *testing.T) {
	h := newQueueHost(8)
	produce(t, h, "pending")
	if err := h.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	out, err := h.Next(context.Background(), 0)
	if err != nil {
		t.Fatalf("Next after close should deliver buffered events, got %v", err)
	}
	if len(out.Events) != 1 || out.Events[0].Method != "pending" {
		t.Fatalf("expected buffered event, got %+v", out.Events)
	}
	if _, err := h.Next(context.Background(), 0); !errors.Is(err, ErrClosed) {
		t.Fatalf("expected ErrClosed once drained, got %v", err)
	}
}

func TestNextConcurrentProducerDrainNeverStalls(t *testing.T) {
	h := newQueueHost(512)
	const total = 300
	var wg sync.WaitGroup
	received := make(chan int, 1)
	wg.Add(1)
	go func() {
		defer wg.Done()
		seen := 0
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		for {
			out, err := h.Next(ctx, 50)
			if err != nil {
				received <- seen
				return
			}
			seen += len(out.Events)
			if seen >= total {
				received <- seen
				return
			}
		}
	}()
	producerDone := make(chan struct{})
	go func() {
		defer close(producerDone)
		for i := 0; i < total; i++ {
			h.Handle(context.Background(), nil, controlrpc.Request{
				Method: "run/event", Params: json.RawMessage(`{"seq":1}`),
			})
		}
	}()
	select {
	case <-producerDone:
	case <-time.After(5 * time.Second):
		t.Fatal("producer stalled: a full queue must never block dispatch")
	}
	select {
	case seen := <-received:
		if seen != total {
			t.Fatalf("expected %d events, got %d", total, seen)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("drain did not finish")
	}
	wg.Wait()
}

func TestPollRemainsNonBlocking(t *testing.T) {
	h := newQueueHost(8)
	start := time.Now()
	out, err := h.Poll(context.Background(), 0)
	if err != nil {
		t.Fatalf("Poll: %v", err)
	}
	if len(out.Events) != 0 || out.Gap {
		t.Fatalf("expected empty poll result, got %+v", out)
	}
	if time.Since(start) > time.Second {
		t.Fatal("Poll blocked")
	}
}
