package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/testsupport"
)

func followUpRestorePayloadSize(item domain.QueuedTurn) int {
	item.Track = domain.QueueTrackFollowUp
	return len(newEventMapper(item.EnqueuedOn, 0).build(domain.EventTurnDequeued, payloadTurnDequeued{
		QueueID: item.ID, Track: item.Track, Text: item.Text, Reason: "dequeued", Turn: &item,
	}).Payload)
}

func steerTextAtRestoreBoundary(sid domain.SessionID, rid domain.RunID, limit int) int {
	sample := domain.QueuedTurn{ID: "q-0000000000000000", SessionID: sid, EnqueuedOn: rid, Text: "x", CreatedAt: time.Now()}
	// Text appears in both the legacy payload and the full DTO. Probe around
	// the estimated boundary to allow the clock's fractional-second encoding.
	return (limit - followUpRestorePayloadSize(sample) + 2) / 2
}

func TestSteerEnqueueReservesFutureRestorePayloadBoundary(t *testing.T) {
	svc, backend := newQueueTestService(t, testsupport.NewEchoModel())
	ctx := context.Background()
	const sid domain.SessionID = "payload-steer"
	const rid domain.RunID = "run_0000000000000000"
	mustCreateSession(t, backend, sid)
	if err := backend.CreateRun(ctx, domain.Run{ID: rid, SessionID: sid, Status: domain.RunActive}); err != nil {
		t.Fatal(err)
	}
	limit := svc.MaxEventPayloadBytes()
	middle := steerTextAtRestoreBoundary(sid, rid, limit)
	accepted, rejected := 0, 0
	for n := middle - 5; n <= middle+5; n++ {
		before := len(journalEvents(t, backend, rid))
		item, err := svc.enqueueTurn(ctx, sid, rid, domain.QueuedTurn{Text: strings.Repeat("x", n)}, domain.QueueTrackSteer)
		if err != nil {
			if !errors.Is(err, ErrQueuePayloadTooLarge) {
				t.Fatal(err)
			}
			rejected++
			state := mustQueueState(t, svc, ctx, sid, "")
			if len(state.Steering)+len(state.FollowUps) != 0 {
				t.Fatalf("rejected item visible: %+v", state)
			}
			if after := len(journalEvents(t, backend, rid)); after != before {
				t.Fatalf("rejection changed journal: %d -> %d", before, after)
			}
			svc.dropSessionQueue(sid)
			if state := mustQueueState(t, svc, ctx, sid, ""); len(state.Steering)+len(state.FollowUps) != 0 {
				t.Fatal("rejection changed durable queue")
			}
			continue
		}
		accepted++
		if future := followUpRestorePayloadSize(item); future > limit {
			t.Fatalf("acknowledged steer cannot restore after fallback: future=%d limit=%d", future, limit)
		}
		events := journalEvents(t, backend, rid)
		var queued payloadTurnQueued
		if err := json.Unmarshal(events[len(events)-1].Payload, &queued); err != nil {
			t.Fatal(err)
		}
		if item.Track != domain.QueueTrackSteer || queued.Track != domain.QueueTrackSteer || queued.Turn == nil || queued.Turn.Track != domain.QueueTrackSteer {
			t.Fatal("reservation changed actual persisted track")
		}
		if _, ok, err := svc.QueueRemove(ctx, sid, item.ID); err != nil || !ok {
			t.Fatalf("remove accepted probe: %v %v", ok, err)
		}
	}
	if accepted == 0 || rejected == 0 {
		t.Fatalf("did not exercise both sides of boundary: accepted=%d rejected=%d", accepted, rejected)
	}
}

func TestBoundedSteerPayloadAdmitsAfterMissingCheckpointFallback(t *testing.T) {
	model := newScriptedToolModel()
	svc, backend := newScriptedService(t, model)
	ctx := context.Background()
	const sid domain.SessionID = "payload-fallback"
	mustCreateSession(t, backend, sid)
	rid, err := svc.Run(ctx, sid, "initial")
	if err != nil {
		t.Fatal(err)
	}
	<-model.entered
	limit := svc.MaxEventPayloadBytes()
	text := strings.Repeat("x", steerTextAtRestoreBoundary(sid, rid, limit)-8)
	item, err := svc.Steer(ctx, sid, text)
	if err != nil || item.Track != domain.QueueTrackSteer {
		t.Fatalf("bounded steer rejected: %+v %v", item, err)
	}
	if future := followUpRestorePayloadSize(item); future > limit {
		t.Fatalf("bounded setup exceeds limit: %d > %d", future, limit)
	}
	svc.engine.cfg.Checkpoints = nil
	close(model.release)
	waitFor(t, "bounded fallback completed", func() bool {
		runs, _ := backend.ListRunsBySession(ctx, sid)
		return len(runs) == 2 && runs[1].Status.Terminal()
	})
	svc.WaitIdle(ctx)
	runs, err := backend.ListRunsBySession(ctx, sid)
	if err != nil || len(runs) != 2 {
		t.Fatalf("fallback admissions=%+v %v", runs, err)
	}
	found := false
	for _, event := range journalEvents(t, backend, runs[1].ID) {
		if event.Type == domain.EventTurnDequeued {
			var p payloadTurnDequeued
			_ = json.Unmarshal(event.Payload, &p)
			found = found || p.QueueID == item.ID
		}
	}
	if !found {
		t.Fatal("bounded steer was not durably admitted")
	}
	if state := mustQueueState(t, svc, ctx, sid, ""); len(state.Steering)+len(state.FollowUps) != 0 {
		t.Fatal("bounded fallback retained consumed turn")
	}
}
