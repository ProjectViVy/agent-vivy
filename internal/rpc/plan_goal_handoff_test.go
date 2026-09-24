package rpc

import (
	"context"
	"errors"
	"testing"
	"time"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
)

// Removing the durable pause, draining after Plan commit, or recreating a
// deleted session must fail these end-to-end RPC and Journal assertions.
func TestHumanPlanEntryDrainsGoalBeforeBecomingEffective(t *testing.T) {
	ctx := context.Background()
	model := &holdCancellationModel{entered: make(chan struct{}), release: make(chan struct{}), cancelled: make(chan struct{})}
	env, svc := newGoalLifecycleControlEnv(t, model, nil)
	defer func() { model.unblock(); svc.StopAutomaticWork(); svc.CancelAll(); waitForGoalControlIdle(t, svc) }()
	const sessionID domain.SessionID = "sess-plan-handoff"
	if err := env.backend.CreateSession(ctx, domain.Session{ID: sessionID, CreatedAt: 1}); err != nil {
		t.Fatal(err)
	}
	if _, rpcErr := callControl(t, env.handler, "goal/create", map[string]any{
		"session_id": string(sessionID), "request_id": "goal", "expected_version": 0,
		"goal_id": "goal-plan", "objective": "finish work", "max_rounds": 2,
	}); rpcErr != nil {
		t.Fatal(rpcErr)
	}
	select {
	case <-model.entered:
	case <-time.After(5 * time.Second):
		t.Fatal("Goal run did not start")
	}
	type response struct {
		value any
		err   *Error
	}
	done := make(chan response, 1)
	go func() {
		value, rpcErr := callControl(t, env.handler, "plan/enter", map[string]any{
			"session_id": string(sessionID), "request_id": "enter", "expected_version": 2,
		})
		done <- response{value, rpcErr}
	}()
	deadline := time.Now().Add(5 * time.Second)
	for {
		state, err := env.backend.ReadWork(ctx, sessionID)
		if err != nil {
			t.Fatal(err)
		}
		if state.Goal != nil && state.Goal.Phase == domain.WorkPhasePaused {
			if state.Plan.Active || state.Version != 3 {
				t.Fatalf("Plan effective before drain: %+v", state)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("Goal was not durably paused: %+v", state)
		}
		time.Sleep(5 * time.Millisecond)
	}
	select {
	case <-model.cancelled:
	case <-time.After(5 * time.Second):
		t.Fatal("Goal run was not cancelled")
	}
	select {
	case got := <-done:
		t.Fatalf("Plan returned before drain: %+v", got)
	default:
	}
	model.unblock()
	select {
	case got := <-done:
		if got.err != nil {
			t.Fatalf("plan/enter: %v", got.err)
		}
		view := got.value.(workCommitResult).Work
		if !view.Plan.Active || view.Goal == nil || view.Goal.Phase != string(domain.WorkPhasePaused) {
			t.Fatalf("Plan view: %+v", view)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Plan did not finish after drain")
	}
	state, err := env.backend.ReadWork(ctx, sessionID)
	if err != nil || !state.Plan.Active || state.Goal == nil || state.Goal.Phase != domain.WorkPhasePaused || state.Version != 4 {
		t.Fatalf("persisted state: %+v / %v", state, err)
	}
	events, _, err := env.backend.ReplayWork(ctx, sessionID, domain.WorkState{SessionID: sessionID}, 10)
	if err != nil || len(events) != 4 || events[2].Kind != domain.WorkEventGoalPaused || events[3].Kind != domain.WorkEventPlanEntered {
		t.Fatalf("work events: %+v / %v", events, err)
	}
}

func TestDeleteDuringGoalDrainCannotResurrectPlan(t *testing.T) {
	ctx := context.Background()
	model := &holdCancellationModel{entered: make(chan struct{}), release: make(chan struct{}), cancelled: make(chan struct{})}
	env, svc := newGoalLifecycleControlEnv(t, model, nil)
	defer func() { model.unblock(); svc.StopAutomaticWork(); svc.CancelAll(); waitForGoalControlIdle(t, svc) }()
	const sessionID domain.SessionID = "sess-plan-deleted"
	if err := env.backend.CreateSession(ctx, domain.Session{ID: sessionID, CreatedAt: 1}); err != nil {
		t.Fatal(err)
	}
	if _, rpcErr := callControl(t, env.handler, "goal/create", map[string]any{
		"session_id": string(sessionID), "request_id": "goal", "expected_version": 0,
		"goal_id": "goal-delete", "objective": "finish work", "max_rounds": 2,
	}); rpcErr != nil {
		t.Fatal(rpcErr)
	}
	select {
	case <-model.entered:
	case <-time.After(5 * time.Second):
		t.Fatal("Goal run did not start")
	}
	done := make(chan *Error, 1)
	go func() {
		_, rpcErr := callControl(t, env.handler, "plan/enter", map[string]any{
			"session_id": string(sessionID), "request_id": "enter", "expected_version": 2,
		})
		done <- rpcErr
	}()
	deadline := time.Now().Add(5 * time.Second)
	for {
		state, err := env.backend.ReadWork(ctx, sessionID)
		if err != nil {
			t.Fatal(err)
		}
		if state.Goal != nil && state.Goal.Phase == domain.WorkPhasePaused {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("Goal was not paused: %+v", state)
		}
		time.Sleep(5 * time.Millisecond)
	}
	if err := svc.DeleteSession(ctx, sessionID); err != nil {
		t.Fatalf("delete during drain: %v", err)
	}
	model.unblock()
	select {
	case rpcErr := <-done:
		if rpcErr == nil {
			t.Fatal("deleted session reported effective Plan")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Plan entry did not stop after delete")
	}
	if _, err := env.backend.GetSession(ctx, sessionID); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("session resurrected: %v", err)
	}
	if _, err := env.backend.ReadWork(ctx, sessionID); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("work resurrected: %v", err)
	}
}

func TestPlanEntryDrainCancellationLeavesGoalPausedAndPlanIneffective(t *testing.T) {
	ctx := context.Background()
	model := &holdCancellationModel{entered: make(chan struct{}), release: make(chan struct{}), cancelled: make(chan struct{})}
	env, svc := newGoalLifecycleControlEnv(t, model, nil)
	defer func() { model.unblock(); svc.StopAutomaticWork(); svc.CancelAll(); waitForGoalControlIdle(t, svc) }()
	const sessionID domain.SessionID = "sess-plan-drain-timeout"
	if err := env.backend.CreateSession(ctx, domain.Session{ID: sessionID, CreatedAt: 1}); err != nil {
		t.Fatal(err)
	}
	if _, rpcErr := callControl(t, env.handler, "goal/create", map[string]any{
		"session_id": string(sessionID), "request_id": "goal", "expected_version": 0,
		"goal_id": "goal-timeout", "objective": "finish work", "max_rounds": 2,
	}); rpcErr != nil {
		t.Fatal(rpcErr)
	}
	select {
	case <-model.entered:
	case <-time.After(5 * time.Second):
		t.Fatal("Goal run did not start")
	}
	mutation, rpcErr := buildWorkMutation("plan/enter", domain.WorkEventPlanEntered, workParams{
		SessionID: string(sessionID), RequestID: "enter", ExpectedVersion: 2,
	})
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	drainCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	drainDone := make(chan error, 1)
	go func() { _, err := svc.EnterPlan(drainCtx, mutation); drainDone <- err }()
	deadline := time.Now().Add(5 * time.Second)
	var state domain.WorkState
	var err error
	for {
		state, err = env.backend.ReadWork(ctx, sessionID)
		if err != nil {
			t.Fatal(err)
		}
		if state.Goal != nil && state.Goal.Phase == domain.WorkPhasePaused {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("Goal was not paused: %+v", state)
		}
		time.Sleep(5 * time.Millisecond)
	}
	cancel()
	if err := <-drainDone; !errors.Is(err, context.Canceled) {
		t.Fatalf("undrained transition = %v, want cancellation", err)
	}
	if err != nil || state.Plan.Active || state.Goal == nil || state.Goal.Phase != domain.WorkPhasePaused || state.Version != 3 {
		t.Fatalf("failed drain state = %+v / %v", state, err)
	}
	if activation, _ := svc.GoalActivation(sessionID); activation != "disarmed" {
		t.Fatalf("failed drain activation = %q", activation)
	}
	retry, rpcErr := buildWorkMutation("plan/enter", domain.WorkEventPlanEntered, workParams{
		SessionID: string(sessionID), RequestID: "retry-enter", ExpectedVersion: 3,
	})
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	retryCtx, retryCancel := context.WithCancel(ctx)
	defer retryCancel()
	retryDone := make(chan error, 1)
	go func() { _, err := svc.EnterPlan(retryCtx, retry); retryDone <- err }()
	select {
	case err := <-retryDone:
		t.Fatalf("retry bypassed old run drain: %v", err)
	case <-time.After(100 * time.Millisecond):
	}
	retryCancel()
	if err := <-retryDone; !errors.Is(err, context.Canceled) {
		t.Fatalf("retry drain = %v, want cancellation", err)
	}
	events, _, err := env.backend.ReplayWork(ctx, sessionID, domain.WorkState{SessionID: sessionID}, 10)
	if err != nil || len(events) != 3 || events[2].Kind != domain.WorkEventGoalPaused {
		t.Fatalf("failed drain events = %+v / %v", events, err)
	}
}

func TestReplayedStartGoalDecisionDoesNotRearmAfterRestart(t *testing.T) {
	ctx := context.Background()
	model := &holdCancellationModel{entered: make(chan struct{}), release: make(chan struct{})}
	env, svc := newGoalLifecycleControlEnv(t, model, nil)
	defer func() { model.unblock(); svc.StopAutomaticWork(); svc.CancelAll(); waitForGoalControlIdle(t, svc) }()
	const sessionID domain.SessionID = "sess-plan-replay-wake"
	const origin domain.RunID = "run-plan-replay-wake"
	if err := env.backend.CreateSession(ctx, domain.Session{ID: sessionID, CreatedAt: 1}); err != nil {
		t.Fatal(err)
	}
	if err := env.backend.CreateRun(ctx, domain.Run{ID: origin, SessionID: sessionID, Kind: domain.RunKindPrimary, Status: domain.RunActive, CreatedAt: 2}); err != nil {
		t.Fatal(err)
	}
	seed := []domain.WorkMutation{
		{SessionID: sessionID, ExpectedVersion: 0, RequestID: "enter", RequestHash: "enter", Kind: domain.WorkEventPlanEntered},
		{SessionID: sessionID, ExpectedVersion: 1, RequestID: "submit", RequestHash: "submit", Kind: domain.WorkEventPlanSubmitted,
			PlanSubmissionID: "submission", PlanMarkdown: "1. inspect", PlanOriginRunID: origin, PlanOriginToolCallID: "submit-call"},
		{SessionID: sessionID, ExpectedVersion: 2, RequestID: "suspend", RequestHash: "suspend", Kind: domain.WorkEventPlanReviewSuspended,
			PlanSubmissionID: "submission", PlanOriginRunID: origin, PlanOriginToolCallID: "submit-call", PlanResumeTarget: "exact-target"},
	}
	for _, mutation := range seed {
		if _, err := env.backend.CommitWork(ctx, mutation); err != nil {
			t.Fatalf("seed %s: %v", mutation.Kind, err)
		}
	}
	params := workParams{
		SessionID: string(sessionID), ExpectedVersion: 3, RequestID: "decision",
		PlanSubmissionID: "submission", PlanAction: string(domain.PlanDecisionStartGoal),
		GoalID: "new-goal", Objective: "finish reviewed work", MaxRounds: 1,
	}
	decision, rpcErr := buildWorkMutation("plan/decide", domain.WorkEventPlanDecided, params)
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	if _, err := env.backend.CommitWork(ctx, decision); err != nil {
		t.Fatalf("seed accepted decision: %v", err)
	}
	if err := env.backend.SetRunStatus(ctx, origin, domain.RunCompleted); err != nil {
		t.Fatal(err)
	}
	got, rpcErr := callControl(t, env.handler, "plan/decide", map[string]any{
		"session_id": string(sessionID), "expected_version": 3, "request_id": "decision",
		"submission_id": "submission", "action": string(domain.PlanDecisionStartGoal),
		"goal_id": "new-goal", "objective": "finish reviewed work", "max_rounds": 1,
	})
	if rpcErr != nil || !got.(workCommitResult).Replayed {
		t.Fatalf("identical decision replay = %+v / %v", got, rpcErr)
	}
	select {
	case <-model.entered:
		t.Fatal("replayed decision armed a new Goal run")
	case <-time.After(300 * time.Millisecond):
	}
	state, err := env.backend.ReadWork(ctx, sessionID)
	if err != nil || state.Goal == nil || state.Goal.RoundsStarted != 0 {
		t.Fatalf("replayed decision admitted Goal: %+v / %v", state.Goal, err)
	}
}
