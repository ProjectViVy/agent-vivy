package tui

import (
	"context"
	"errors"
	"strings"
	"testing"

	"example.com/vivy/faces/tui/surface"
)

func TestLiveKeepsReasoningContinuousAcrossEmptyDelta(t *testing.T) {
	live := &Live{
		messages:  map[string][]surface.Message{"sess_1": nil},
		activeID:  "sess_1",
		ctx:       context.Background(),
		eventWake: make(chan struct{}, 1),
	}
	live.applyNotice(eventNotice{Kind: "reasoning", Delta: "这"})
	live.applyNotice(eventNotice{Kind: "delta", Delta: ""})
	live.applyNotice(eventNotice{Kind: "reasoning", Delta: "是一句"})
	live.applyNotice(eventNotice{Kind: "delta", Delta: ""})
	live.applyNotice(eventNotice{Kind: "reasoning", Delta: "话🙂\n继续"})

	msgs := live.ActiveMessages()
	if len(msgs) != 1 || !msgs[0].Reasoning || msgs[0].Content != "这是一句话🙂\n继续" {
		t.Fatalf("reasoning messages = %+v", msgs)
	}

	live.applyNotice(eventNotice{Kind: "delta", Delta: "答案"})
	msgs = live.ActiveMessages()
	if len(msgs) != 2 || msgs[0].Streaming || !msgs[0].Reasoning || msgs[1].Reasoning || msgs[1].Content != "答案" {
		t.Fatalf("reasoning/answer boundary = %+v", msgs)
	}
}

func TestLiveEventInboxDoesNotDropBurst(t *testing.T) {
	live := &Live{
		messages:  map[string][]surface.Message{"sess_1": nil},
		activeID:  "sess_1",
		ctx:       context.Background(),
		eventWake: make(chan struct{}, 1),
	}
	for i := 0; i < 257; i++ {
		live.enqueueNotice(eventNotice{Kind: "reasoning", Delta: "界"})
	}
	_, _ = live.drainEvents()
	msgs := live.ActiveMessages()
	if len(msgs) != 1 {
		t.Fatalf("burst messages = %d, want 1", len(msgs))
	}
	if msgs[0].Content != strings.Repeat("界", 257) {
		t.Fatalf("burst runes = %d, want 257", len([]rune(msgs[0].Content)))
	}
}

func TestLiveDetectsSequenceGapAndSuppressesReplayDuplicates(t *testing.T) {
	live := &Live{
		messages:  map[string][]surface.Message{"sess_1": nil},
		activeID:  "sess_1",
		runID:     "run_1",
		busy:      true,
		ctx:       context.Background(),
		eventWake: make(chan struct{}, 1),
	}
	live.enqueueNotice(eventNotice{RunID: "run_1", Seq: 1})
	live.enqueueNotice(eventNotice{RunID: "run_1", Seq: 3, Kind: "reasoning", Delta: "丢"})
	finished, gap := live.drainEvents()
	if finished || !gap || live.cursor.LastSeq != 1 || !live.cursor.Recovering {
		t.Fatalf("finished=%v gap=%v seq=%d recovering=%v", finished, gap, live.cursor.LastSeq, live.cursor.Recovering)
	}
	live.recoveryInFlight = true
	live.applySubscribed(liveSubscribedMsg{RunID: "run_1", AfterSeq: 0})
	if !live.recoveryInFlight {
		t.Fatal("delayed initial subscription result cleared recovery state")
	}
	live.applySubscribed(liveSubscribedMsg{RunID: "run_1", AfterSeq: 1, Recovery: true, Err: errors.New("temporary")})
	if !live.Meta().Busy || live.runID != "run_1" {
		t.Fatal("transient replay failure cancelled the active run")
	}
	_, gap = live.drainEvents()
	if !gap || live.inbox.Len() != 1 {
		t.Fatalf("failed replay lost retained gap: gap=%v pending=%d", gap, live.inbox.Len())
	}
	live.enqueueNotice(eventNotice{RunID: "run_1", Seq: 2, Kind: "reasoning", Delta: "补"})
	live.enqueueNotice(eventNotice{RunID: "run_1", Seq: 3, Kind: "reasoning", Delta: "丢"})
	_, gap = live.drainEvents()
	if gap || live.cursor.LastSeq != 3 {
		t.Fatalf("replay gap=%v seq=%d", gap, live.cursor.LastSeq)
	}
	if msgs := live.ActiveMessages(); len(msgs) != 1 || msgs[0].Content != "补丢" {
		t.Fatalf("retained replay messages = %+v", msgs)
	}
	if accept, missing := live.acceptSequence(eventNotice{RunID: "run_1", Seq: 3}); accept || missing {
		t.Fatalf("duplicate seq 3 accept=%v gap=%v", accept, missing)
	}
	live.applySubscribed(liveSubscribedMsg{RunID: "run_1", AfterSeq: 1, Err: errors.New("stale")})
	if !live.Meta().Busy || live.runID != "run_1" {
		t.Fatal("stale subscription failure cancelled an advanced run")
	}
}

func TestLiveWaitsForReplayAfterSuccessfulRecoverySubscription(t *testing.T) {
	live := &Live{
		messages:  map[string][]surface.Message{"sess_1": nil},
		activeID:  "sess_1",
		runID:     "run_1",
		busy:      true,
		ctx:       context.Background(),
		eventWake: make(chan struct{}, 1),
	}
	live.cursor.LastSeq = 1
	live.cursor.Recovering = true
	live.recoveryInFlight = true
	live.applySubscribed(liveSubscribedMsg{RunID: "run_1", AfterSeq: 1, Recovery: true})
	if !live.replayPending || live.recoveryInFlight || live.recoverSubscriptionCmd() != nil {
		t.Fatalf("replay fence pending=%v in_flight=%v", live.replayPending, live.recoveryInFlight)
	}
	live.enqueueNotice(eventNotice{RunID: "run_1", Seq: 2})
	_, gap := live.drainEvents()
	if gap || live.replayPending {
		t.Fatalf("contiguous replay gap=%v pending=%v", gap, live.replayPending)
	}
}
