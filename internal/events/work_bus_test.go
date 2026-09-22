package events

import (
	"testing"

	"agent-vivy/internal/domain"
)

func workTestEvent(sessionID domain.SessionID, seq domain.WorkSeq) domain.WorkEvent {
	return domain.WorkEvent{
		SessionID: sessionID, Seq: seq, PayloadVersion: domain.WorkPayloadVersion,
		Kind: domain.WorkEventGoalCreated,
	}
}

func TestWorkBusPublishesOnlyToMatchingSession(t *testing.T) {
	bus := NewWorkBus(2)
	matching, cancelMatching := bus.Subscribe("session-1")
	other, cancelOther := bus.Subscribe("session-2")
	defer cancelMatching()
	defer cancelOther()

	bus.Publish(workTestEvent("session-1", 1))
	select {
	case event := <-matching:
		if event.SessionID != "session-1" || event.Seq != 1 {
			t.Fatalf("matching event = %+v", event)
		}
	default:
		t.Fatal("matching subscriber did not receive event")
	}
	select {
	case event := <-other:
		t.Fatalf("other session received event %+v", event)
	default:
	}
}

func TestWorkBusDropsSlowSubscribersForDurableReplay(t *testing.T) {
	bus := NewWorkBus(1)
	ch, _ := bus.Subscribe("session-1")
	bus.Publish(workTestEvent("session-1", 1))
	bus.Publish(workTestEvent("session-1", 2))

	if got := bus.Subscribers("session-1"); got != 0 {
		t.Fatalf("Subscribers() = %d, want dropped subscriber removed", got)
	}
	if event := <-ch; event.Seq != 1 {
		t.Fatalf("buffered event seq = %d, want 1", event.Seq)
	}
	if _, ok := <-ch; ok {
		t.Fatal("dropped subscriber channel remains open")
	}
}
