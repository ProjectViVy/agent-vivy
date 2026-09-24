package rpc

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/events"
	"agent-vivy/internal/storage"
)

func TestWorkSubscriptionBuffersCommitAfterCapturedWatermark(t *testing.T) {
	bus := events.NewWorkBus(8)
	env := newControlTestEnv(t, func(deps *ControlDeps) {
		deps.Work = deps.Sessions.(storage.WorkStore)
		deps.WorkBus = bus
	})
	const sessionID domain.SessionID = "sess-watermark"
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := env.backend.CreateSession(ctx, domain.Session{ID: sessionID, CreatedAt: 1}); err != nil {
		t.Fatal(err)
	}
	commit := func(version domain.WorkVersion, kind domain.WorkEventKind) domain.WorkEvent {
		t.Helper()
		mutation := domain.WorkMutation{SessionID: sessionID, ExpectedVersion: version,
			RequestID: string(kind), RequestHash: string(kind), Kind: kind,
			Goal: domain.GoalRef{ID: "goal-watermark", Revision: 1}}
		if kind == domain.WorkEventGoalCreated {
			mutation.Objective, mutation.MaxRounds = "deliver", 2
		}
		result, err := env.backend.CommitWork(ctx, mutation)
		if err != nil {
			t.Fatal(err)
		}
		return result.Event
	}
	commit(0, domain.WorkEventGoalCreated)
	peer := NewPeer(nil, nil, Options{OutgoingBuffer: 8})
	request := Request{JSONRPC: "2.0", ID: json.RawMessage(`"watermark"`), Method: "session/work/subscribe",
		Params: json.RawMessage(`{"session_id":"sess-watermark","after_seq":0}`)}
	result, rpcErr := env.handler.Handle(ctx, peer, request)
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	response := result.(map[string]any)
	if response["watermark_seq"] != int64(1) {
		t.Fatalf("watermark = %v, want durable WorkSeq 1", response["watermark_seq"])
	}
	if response["process_epoch"] == "" {
		t.Fatal("subscription response has no process epoch")
	}
	if bus.Subscribers(sessionID) != 1 {
		t.Fatal("live buffer was not attached before watermark response")
	}
	bus.Publish(commit(1, domain.WorkEventGoalPaused))
	stored, err := env.backend.ReadWork(ctx, sessionID)
	if err != nil || stored.Version != 2 || stored.Goal == nil || stored.Goal.Phase != domain.WorkPhasePaused {
		t.Fatalf("durable work after interleaved commit = %+v, %v", stored, err)
	}
	peer.runAfterResponse(request.ID)
	for _, want := range []int64{1, 2} {
		select {
		case frame := <-peer.out:
			var notification struct {
				Method string `json:"method"`
				Params struct {
					WorkVersion  int64           `json:"work_version"`
					ProcessEpoch string          `json:"process_epoch"`
					Event        workEventResult `json:"event"`
				} `json:"params"`
			}
			if err := json.Unmarshal(frame, &notification); err != nil {
				t.Fatal(err)
			}
			if notification.Params.Event.Seq != want {
				t.Fatalf("event seq = %d, want %d", notification.Params.Event.Seq, want)
			}
			if notification.Method != "session/work/event" || notification.Params.WorkVersion != want || notification.Params.ProcessEpoch != response["process_epoch"] {
				t.Fatalf("notification metadata = %+v", notification)
			}
		case <-time.After(2 * time.Second):
			t.Fatalf("missing work seq %d", want)
		}
	}
	select {
	case frame := <-peer.out:
		t.Fatalf("duplicate work event: %s", frame)
	case <-time.After(50 * time.Millisecond):
	}
}

func TestWorkViewUsesNewEpochAfterControlRestart(t *testing.T) {
	env := newControlTestEnv(t, func(deps *ControlDeps) {
		deps.Work = deps.Sessions.(storage.WorkStore)
	})
	const sessionID domain.SessionID = "sess-restart-epoch"
	if err := env.backend.CreateSession(context.Background(), domain.Session{ID: sessionID, CreatedAt: 1}); err != nil {
		t.Fatal(err)
	}
	first, rpcErr := callControl(t, env.handler, "session/work/get", map[string]any{"session_id": sessionID})
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	secondHandler, err := NewControlHandler(env.handler.(*controlHandler).deps)
	if err != nil {
		t.Fatal(err)
	}
	second, rpcErr := callControl(t, secondHandler, "session/work/get", map[string]any{"session_id": sessionID})
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	oldView, newView := first.(workStateResult), second.(workStateResult)
	if oldView.ProcessEpoch == "" || newView.ProcessEpoch == "" || oldView.ProcessEpoch == newView.ProcessEpoch || oldView.Version != newView.Version {
		t.Fatalf("work epochs or durable version after restart = %+v / %+v", oldView, newView)
	}
}
