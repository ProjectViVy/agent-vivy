package tui

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"agent-vivy/sdk/tui/stream"
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
	for live.inbox.Len() > 0 {
		_, _ = live.drainEvents()
	}
	msgs := live.ActiveMessages()
	if len(msgs) != 1 {
		t.Fatalf("burst messages = %d, want 1", len(msgs))
	}
	if msgs[0].Content != strings.Repeat("界", 257) {
		t.Fatalf("burst runes = %d, want 257", len([]rune(msgs[0].Content)))
	}
}

func TestLiveInboxOverflowRequestsReplayFromLastAppliedSequence(t *testing.T) {
	live := &Live{
		messages:  map[string][]surface.Message{"sess_1": nil},
		activeID:  "sess_1",
		runID:     "run_1",
		busy:      true,
		inbox:     stream.NewInbox(2),
		ctx:       context.Background(),
		eventWake: make(chan struct{}, 1),
	}
	for seq := 1; seq <= 3; seq++ {
		live.enqueueNotice(eventNotice{RunID: "run_1", Seq: seq, Kind: "reasoning", Delta: fmt.Sprint(seq)})
	}
	finished, replay := live.drainEvents()
	if finished || !replay || live.cursor.LastSeq != 0 || live.inbox.Len() != 0 {
		t.Fatalf("finished=%v replay=%v seq=%d pending=%d", finished, replay, live.cursor.LastSeq, live.inbox.Len())
	}
	if msgs := live.ActiveMessages(); len(msgs) != 0 {
		t.Fatalf("bounded prefix = %+v", msgs)
	}
}

func TestLiveRecoveryReplacesAndUnsubscribesPreviousStream(t *testing.T) {
	unsubscribed := ""
	script := baseScript()
	script["run/subscribe"] = func(json.RawMessage) (any, error) {
		return map[string]string{"subscription_id": "sub_new"}, nil
	}
	script["run/unsubscribe"] = func(raw json.RawMessage) (any, error) {
		var params struct {
			SubscriptionID string `json:"subscription_id"`
		}
		_ = json.Unmarshal(raw, &params)
		unsubscribed = params.SubscriptionID
		return map[string]bool{"unsubscribed": true}, nil
	}
	env := &fakeEnv{script: script}
	live := NewLive(newClient(env), LiveOptions{})
	live.runID = "run_1"
	live.busy = true
	live.subscriptionID = "sub_old"
	live.cursor.LastSeq = 4
	msg := mustMsg[liveSubscribedMsg](t, live.subscribeCmd("run_1", 4, true))
	cmd := live.applySubscribed(msg)
	if cmd == nil {
		t.Fatal("replacement did not schedule old subscription cleanup")
	}
	_ = cmd()
	if unsubscribed != "sub_old" || live.subscriptionID != "sub_new" {
		t.Fatalf("unsubscribed=%q current=%q", unsubscribed, live.subscriptionID)
	}
	live.applyNotice(eventNotice{RunID: "run_1", Seq: 5, Kind: "done", Done: true})
	if live.subscriptionID != "" {
		t.Fatalf("terminal retained subscription id %q", live.subscriptionID)
	}
	live.Close()
}

func TestLiveCloseUnsubscribesActiveStream(t *testing.T) {
	unsubscribed := ""
	script := baseScript()
	script["run/unsubscribe"] = func(raw json.RawMessage) (any, error) {
		var params struct {
			SubscriptionID string `json:"subscription_id"`
		}
		_ = json.Unmarshal(raw, &params)
		unsubscribed = params.SubscriptionID
		return map[string]bool{"unsubscribed": true}, nil
	}
	live := NewLive(newClient(&fakeEnv{script: script}), LiveOptions{})
	live.subscriptionID = "sub_close"
	live.Close()
	if unsubscribed != "sub_close" {
		t.Fatalf("close unsubscribed %q", unsubscribed)
	}
}

func TestLiveStreamErrorRecoversAndRejectsRetiredTraffic(t *testing.T) {
	live := &Live{
		messages:       map[string][]surface.Message{"sess_1": nil},
		activeID:       "sess_1",
		runID:          "run_1",
		busy:           true,
		subscriptionID: "sub_old",
		ctx:            context.Background(),
		eventWake:      make(chan struct{}, 1),
	}
	live.recordStreamError(stream.StreamError{SubscriptionID: "sub_old", Message: "replay failed"})
	if !live.recoveryNeeded || live.subscriptionID != "" || !live.Meta().Busy {
		t.Fatalf("stream failure state recovery=%v subscription=%q meta=%+v", live.recoveryNeeded, live.subscriptionID, live.Meta())
	}
	live.enqueueNotice(eventNotice{SubscriptionID: "sub_old", RunID: "run_1", Seq: 1, Kind: "delta", Delta: "stale"})
	live.enqueueNotice(eventNotice{SubscriptionID: "sub_new", RunID: "run_1", Seq: 1, Kind: "delta", Delta: "fresh"})
	_, _ = live.drainEvents()
	if msgs := live.ActiveMessages(); len(msgs) != 1 || msgs[0].Content != "fresh" {
		t.Fatalf("retired stream polluted transcript: %+v", msgs)
	}
	pending := &Live{runID: "run_2", busy: true, recoveryInFlight: true}
	pending.recordStreamError(stream.StreamError{SubscriptionID: "sub_pending", Message: "early failure"})
	pending.applySubscribed(liveSubscribedMsg{RunID: "run_2", SubscriptionID: "sub_pending", Recovery: true})
	if pending.subscriptionID != "" || !pending.recoveryNeeded || pending.replayPending || pending.recoveryInFlight {
		t.Fatalf("early stream error state subscription=%q recovery=%v replay=%v in_flight=%v", pending.subscriptionID, pending.recoveryNeeded, pending.replayPending, pending.recoveryInFlight)
	}
}

func TestLiveLateSubscriptionAfterCloseIsCleanedUp(t *testing.T) {
	unsubscribed := ""
	script := baseScript()
	script["run/unsubscribe"] = func(raw json.RawMessage) (any, error) {
		var params struct {
			SubscriptionID string `json:"subscription_id"`
		}
		_ = json.Unmarshal(raw, &params)
		unsubscribed = params.SubscriptionID
		return map[string]bool{"unsubscribed": true}, nil
	}
	live := NewLive(newClient(&fakeEnv{script: script}), LiveOptions{})
	live.runID = "run_1"
	live.Close()
	cmd := live.applySubscribed(liveSubscribedMsg{RunID: "run_1", SubscriptionID: "sub_late"})
	if cmd == nil {
		t.Fatal("late subscription was not scheduled for cleanup")
	}
	_ = cmd()
	if unsubscribed != "sub_late" {
		t.Fatalf("late subscription cleanup = %q", unsubscribed)
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
	live.nextRecoveryAt = time.Now().Add(-time.Second)
	if live.recoverSubscriptionCmd() != nil {
		t.Fatal("empty successful replay scheduled a duplicate subscription")
	}
	live.enqueueNotice(eventNotice{RunID: "run_1", Seq: 2})
	_, gap := live.drainEvents()
	if gap || live.replayPending {
		t.Fatalf("contiguous replay gap=%v pending=%v", gap, live.replayPending)
	}
}

func TestLiveNewGapBreaksSuccessfulReplayFence(t *testing.T) {
	live := &Live{
		messages:  map[string][]surface.Message{"sess_1": nil},
		activeID:  "sess_1",
		runID:     "run_1",
		busy:      true,
		ctx:       context.Background(),
		eventWake: make(chan struct{}, 1),
	}
	live.cursor.LastSeq = 1
	live.recoveryInFlight = true
	live.recoveryNeeded = true
	live.applySubscribed(liveSubscribedMsg{RunID: "run_1", AfterSeq: 1, Recovery: true})
	live.enqueueNotice(eventNotice{RunID: "run_1", Seq: 3})
	_, gap := live.drainEvents()
	if !gap || live.replayPending || live.recoverSubscriptionCmd() == nil {
		t.Fatalf("new gap did not reopen recovery: gap=%v replay=%v needed=%v", gap, live.replayPending, live.recoveryNeeded)
	}
}
