package rpc

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/events"
	"agent-vivy/internal/storage"
)

func TestBuildGoalEditMutationCarriesCurrentReference(t *testing.T) {
	mutation, err := buildWorkMutation("goal/edit", domain.WorkEventGoalEdited, workParams{
		SessionID: "session-1", ExpectedVersion: 4, RequestID: "edit-1",
		GoalID: "goal-1", GoalRevision: 7, Objective: "revised objective", MaxRounds: 3,
	})
	if err != nil {
		t.Fatalf("buildWorkMutation: %v", err)
	}
	if mutation.Goal != (domain.GoalRef{ID: "goal-1", Revision: 7}) {
		t.Fatalf("edit GoalRef = %+v, want exact current revision 7", mutation.Goal)
	}
}

func TestPlanGetCarriesReplayStateAcrossPages(t *testing.T) {
	env := newControlTestEnv(t, func(deps *ControlDeps) {
		deps.Work = deps.Sessions.(storage.WorkStore)
	})
	ctx := context.Background()
	const sessionID domain.SessionID = "sess-plan-pages"
	if err := env.backend.CreateSession(ctx, domain.Session{ID: sessionID, CreatedAt: 1}); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	version := domain.WorkVersion(0)
	commit := func(kind domain.WorkEventKind, submissionID, markdown string) {
		t.Helper()
		requestID := fmt.Sprintf("plan-page-%d", version+1)
		mutation := domain.WorkMutation{
			SessionID: sessionID, ExpectedVersion: version, RequestID: requestID,
			RequestHash: requestID, Kind: kind, PlanSubmissionID: submissionID,
			PlanMarkdown: markdown,
		}
		if kind == domain.WorkEventPlanDecided {
			mutation.PlanAction = domain.PlanDecisionRevise
		}
		result, err := env.backend.CommitWork(ctx, mutation)
		if err != nil {
			t.Fatalf("CommitWork %s at %d: %v", kind, version, err)
		}
		version = result.State.Version
	}
	commit(domain.WorkEventPlanEntered, "", "")
	for i := 0; i < 128; i++ {
		id := fmt.Sprintf("submission-%d", i)
		commit(domain.WorkEventPlanSubmitted, id, "# plan")
		commit(domain.WorkEventPlanDecided, id, "")
	}
	if version != 257 {
		t.Fatalf("work version = %d, want decision past first 256-event page", version)
	}
	result, rpcErr := callControl(t, env.handler, "plan/get", map[string]string{
		"session_id": string(sessionID), "submission_id": "submission-127",
	})
	if rpcErr != nil {
		t.Fatalf("plan/get: %v", rpcErr)
	}
	plan, ok := result.(workPlanResult)
	if !ok || plan.SubmissionID != "submission-127" || plan.ReviewStatus != string(domain.PlanReviewRejected) {
		t.Fatalf("plan/get = %+v, want last submission rejected on second page", result)
	}
}

func TestWorkSubscriptionReplaysFromDerivedStateAndContinuesLive(t *testing.T) {
	bus := events.NewWorkBus(8)
	env := newControlTestEnv(t, func(deps *ControlDeps) {
		deps.Work = deps.Sessions.(storage.WorkStore)
		deps.WorkBus = bus
	})
	ctx := context.Background()
	const sessionID domain.SessionID = "sess-work-subscription"
	if err := env.backend.CreateSession(ctx, domain.Session{ID: sessionID, CreatedAt: 1}); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	ref := domain.GoalRef{ID: "goal-1", Revision: 1}
	commit := func(version domain.WorkVersion, kind domain.WorkEventKind) domain.WorkEvent {
		t.Helper()
		requestID := fmt.Sprintf("work-sub-%d", version+1)
		mutation := domain.WorkMutation{
			SessionID: sessionID, ExpectedVersion: version,
			RequestID: requestID, RequestHash: requestID, Kind: kind,
			Goal: ref,
		}
		if kind == domain.WorkEventGoalCreated {
			mutation.Objective, mutation.MaxRounds = "ship it", 2
		}
		result, err := env.backend.CommitWork(ctx, mutation)
		if err != nil {
			t.Fatalf("CommitWork %s: %v", kind, err)
		}
		return result.Event
	}
	commit(0, domain.WorkEventGoalCreated)
	commit(1, domain.WorkEventGoalPaused)
	peer := NewPeer(nil, nil, Options{OutgoingBuffer: 8})
	streamCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() {
		env.handler.(*controlHandler).streamWork(streamCtx, peer, "sub-1", sessionID, 1)
		close(done)
	}()
	defer func() {
		cancel()
		<-done
	}()
	readSeq := func() int64 {
		t.Helper()
		select {
		case frame := <-peer.out:
			var notification struct {
				Method string `json:"method"`
				Params struct {
					Event struct {
						Seq int64 `json:"Seq"`
					} `json:"event"`
				} `json:"params"`
			}
			if err := json.Unmarshal(frame, &notification); err != nil {
				t.Fatalf("decode work notification: %v", err)
			}
			if notification.Method != "session/work/event" {
				t.Fatalf("notification method = %q", notification.Method)
			}
			return notification.Params.Event.Seq
		case <-time.After(2 * time.Second):
			t.Fatal("timed out waiting for work event")
			return 0
		}
	}
	if seq := readSeq(); seq != 2 {
		t.Fatalf("replayed seq = %d, want 2 after after_seq=1", seq)
	}
	bus.Publish(commit(2, domain.WorkEventGoalResumed))
	if seq := readSeq(); seq != 3 {
		t.Fatalf("live seq = %d, want 3", seq)
	}
	commit(3, domain.WorkEventGoalPaused) // a dropped bus notification is recovered from storage
	bus.Publish(commit(4, domain.WorkEventGoalResumed))
	if seq := readSeq(); seq != 4 {
		t.Fatalf("repaired seq = %d, want missing durable event 4", seq)
	}
	if seq := readSeq(); seq != 5 {
		t.Fatalf("live seq after repair = %d, want 5", seq)
	}
}
