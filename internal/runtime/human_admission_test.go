package runtime

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/cloudwego/eino/schema"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
	"agent-vivy/internal/storage/sqlite"
)

func TestHumanAdmissionRegistersBeforeWaitingForSessionGate(t *testing.T) {
	ctx := context.Background()
	backend, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "human-admission.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = backend.Close() })
	const sessionID domain.SessionID = "sess-human-intent"
	if err := backend.CreateSession(ctx, domain.Session{ID: sessionID, Title: "Human intent", CreatedAt: 1}); err != nil {
		t.Fatalf("create session: %v", err)
	}
	if _, err := backend.CommitWork(ctx, domain.WorkMutation{
		SessionID: sessionID, RequestID: "create-goal", RequestHash: "create-goal",
		Kind: domain.WorkEventGoalCreated, Goal: domain.GoalRef{ID: "goal-1", Revision: 1},
		Objective: "finish the current task", MaxRounds: 1,
	}); err != nil {
		t.Fatalf("create Goal: %v", err)
	}
	goalRuns := &notifyingGoalRunStore{GoalRunStore: backend, committed: make(chan storage.GoalRunCommitResult, 1)}
	engine, err := NewEngine(ctx, NewScriptedModel(
		schema.AssistantMessage("Human request completed.", nil),
		schema.AssistantMessage("The one Goal round completed.", nil),
	), nil, EngineConfig{
		StreamBuffer: 8, MaxEventPayloadBytes: 64 << 10,
	})
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}
	svc := NewService(engine, "scripted", "scripted-v0", ServiceDeps{
		Journal: backend, Runs: backend, Messages: backend, Sessions: backend,
		PrimaryRuns: backend, GoalRuns: goalRuns, Work: backend, Sink: newTestSink(),
	})
	svcCtx, cancel := context.WithCancel(ctx)
	t.Cleanup(cancel)
	gate := svc.sessionAdmission(sessionID)
	gate.Lock()
	gateLocked := true
	defer func() {
		if gateLocked {
			gate.Unlock()
		}
	}()
	finished := make(chan error, 1)
	go func() {
		_, err := svc.RunWithOptions(svcCtx, sessionID, "Please handle this before the Goal continues.", RunOptions{HumanAdmission: true})
		finished <- err
	}()

	waitForHumanIntent(t, svc, sessionID)
	svc.mu.Lock()
	pending := svc.humanPending[sessionID]
	svc.mu.Unlock()
	if pending != 1 {
		cancel()
		t.Fatalf("human pending registrations = %d while session gate is held, want 1", pending)
	}

	// A Goal contender may acquire the gate before the human run, but it must
	// observe the registered intent and leave the round uncharged.
	svc.WakeGoal(sessionID)
	gate.Unlock()
	gateLocked = false
	select {
	case err := <-finished:
		if err != nil {
			t.Fatalf("start human run: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("human admission did not leave the session gate")
	}

	runs, err := backend.ListRunsBySession(ctx, sessionID)
	if err != nil {
		t.Fatalf("list runs: %v", err)
	}
	if len(runs) != 1 || runs[0].Kind != domain.RunKindPrimary {
		t.Fatalf("runs = %+v, want only the human primary run", runs)
	}
	state, err := backend.ReadWork(ctx, sessionID)
	if err != nil {
		t.Fatalf("read work: %v", err)
	}
	if state.Goal == nil || state.Goal.RoundsStarted != 0 {
		t.Fatalf("Goal state = %+v, want no round admitted ahead of the registered human request", state.Goal)
	}
	waitForRunStatus(t, backend, runs[0].ID, domain.RunCompleted)
	var goalRun storage.GoalRunCommitResult
	select {
	case goalRun = <-goalRuns.committed:
	case <-time.After(5 * time.Second):
		t.Fatal("terminal human turn did not wake the eligible Goal")
	}
	waitForRunStatus(t, backend, goalRun.Run.ID, domain.RunCompleted)
	if !svc.WaitIdle(context.Background()) {
		t.Fatal("service did not become idle")
	}
}

func TestCancelledHumanAdmissionLeavesNoRunOrMessage(t *testing.T) {
	ctx := context.Background()
	backend, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "cancelled-human-admission.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = backend.Close() })
	const sessionID domain.SessionID = "sess-cancelled-human-intent"
	if err := backend.CreateSession(ctx, domain.Session{ID: sessionID, Title: "Cancelled intent", CreatedAt: 1}); err != nil {
		t.Fatalf("create session: %v", err)
	}
	engine, err := NewEngine(ctx, NewScriptedModel(schema.AssistantMessage("unexpected", nil)), nil, EngineConfig{
		StreamBuffer: 8, MaxEventPayloadBytes: 64 << 10,
	})
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}
	svc := NewService(engine, "scripted", "scripted-v0", ServiceDeps{
		Journal: backend, Runs: backend, Messages: backend, Sessions: backend,
		PrimaryRuns: backend, Work: backend, Sink: newTestSink(),
	})
	runCtx, cancel := context.WithCancel(ctx)
	gate := svc.sessionAdmission(sessionID)
	gate.Lock()
	finished := make(chan error, 1)
	go func() {
		_, err := svc.RunWithOptions(runCtx, sessionID, "cancel before admission", RunOptions{HumanAdmission: true})
		finished <- err
	}()
	waitForHumanIntent(t, svc, sessionID)
	cancel()
	gate.Unlock()
	select {
	case err := <-finished:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("cancelled admission error = %v, want context.Canceled", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("cancelled admission did not leave the session gate")
	}
	runs, err := backend.ListRunsBySession(ctx, sessionID)
	if err != nil || len(runs) != 0 {
		t.Fatalf("runs after cancellation = %+v / %v, want none", runs, err)
	}
	messages, err := backend.ListMessages(ctx, sessionID)
	if err != nil || len(messages) != 0 {
		t.Fatalf("messages after cancellation = %+v / %v, want none", messages, err)
	}
}
