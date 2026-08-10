package events

import (
	"testing"
	"time"

	"agent-vivy/internal/domain"
)

func runEv(typ domain.EventType, seq int64) domain.RunEvent {
	return domain.RunEvent{
		RunID:          "run-t",
		Seq:            domain.EventSeq(seq),
		Type:           typ,
		CreatedAt:      seq,
		PayloadVersion: 1,
		Payload:        []byte(`{}`),
	}
}

func TestBusPublishSubscribeOrder(t *testing.T) {
	bus := NewBus(8)
	live, cancel := bus.Subscribe("run-t")
	defer cancel()

	bus.Publish(runEv(domain.EventRunStarted, 1))
	bus.Publish(runEv(domain.EventModelDelta, 2))

	for want := domain.EventSeq(1); want <= 2; want++ {
		select {
		case ev := <-live:
			if ev.Seq != want {
				t.Fatalf("got seq %d, want %d", ev.Seq, want)
			}
		case <-time.After(time.Second):
			t.Fatalf("timeout waiting for seq %d", want)
		}
	}

	// Terminal publish delivers nothing more and closes the stream.
	bus.Publish(runEv(domain.EventRunCompleted, 3))
	if _, ok := <-live; ok {
		t.Fatal("subscription must close on terminal publish")
	}
	if n := bus.Subscribers("run-t"); n != 0 {
		t.Fatalf("subscribers after terminal = %d, want 0", n)
	}
}

func TestBusSlowSubscriberDropped(t *testing.T) {
	bus := NewBus(1)
	live, _ := bus.Subscribe("run-t")

	bus.Publish(runEv(domain.EventModelDelta, 1)) // fills the buffer
	bus.Publish(runEv(domain.EventModelDelta, 2)) // drops + closes the slow sub

	ev, ok := <-live
	if !ok || ev.Seq != 1 {
		t.Fatalf("first buffered event lost: %+v, %v", ev, ok)
	}
	if _, ok := <-live; ok {
		t.Fatal("slow subscriber channel must be closed")
	}
	if n := bus.Subscribers("run-t"); n != 0 {
		t.Fatalf("subscribers after drop = %d, want 0", n)
	}
}

func TestBusCancelUnregisters(t *testing.T) {
	bus := NewBus(4)
	live, cancel := bus.Subscribe("run-t")
	cancel()
	cancel() // double cancel must not panic

	if _, ok := <-live; ok {
		t.Fatal("cancelled subscription must be closed")
	}
	if n := bus.Subscribers("run-t"); n != 0 {
		t.Fatalf("subscribers after cancel = %d, want 0", n)
	}

	// Publishing to a run with no subscribers is a no-op.
	bus.Publish(runEv(domain.EventModelDelta, 1))
}

func TestBusTerminalBeforeSubscribe(t *testing.T) {
	bus := NewBus(4)
	bus.Publish(runEv(domain.EventRunCompleted, 1))

	// Late subscriber: no panic, empty stream is fine; history comes
	// from the journal replay on the RPC path.
	_, cancel := bus.Subscribe("run-t")
	defer cancel()
	if n := bus.Subscribers("run-t"); n != 1 {
		t.Fatalf("late subscriber not registered: %d", n)
	}
}
