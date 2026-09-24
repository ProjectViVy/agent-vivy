package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"runtime"
	"testing"
	"time"

	"github.com/cloudwego/eino/schema"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
	"agent-vivy/internal/tools"
)

func TestGoalWakeRacingShutdownDoesNotAdmit(t *testing.T) {
	ctx := context.Background()
	backend := openLifecycleBackend(t)
	const sessionID domain.SessionID = "sess-goal-shutdown-race"
	createLifecycleGoal(t, backend, sessionID, 2)
	workspace := &precommitBarrierWorkspaceAllocator{entered: make(chan struct{}), release: make(chan struct{}), path: t.TempDir()}
	svc := newGoalLifecycleService(t, backend, NewScriptedModel(schema.AssistantMessage("unexpected", nil)), workspace, backend, nil)
	defer func() { workspace.releaseBarrier(); waitLifecycleIdle(t, svc) }()
	svc.WakeGoal(sessionID)
	select {
	case <-workspace.entered:
	case <-time.After(5 * time.Second):
		t.Fatal("Goal candidate did not reach precommit barrier")
	}
	stopped := make(chan struct{})
	go func() { svc.StopAutomaticWork(); close(stopped) }()
	deadline := time.After(5 * time.Second)
	for {
		svc.mu.Lock()
		stopping := svc.stopping
		svc.mu.Unlock()
		if stopping {
			break
		}
		select {
		case <-deadline:
			t.Fatal("shutdown did not close automatic admission")
		default:
			runtime.Gosched()
		}
	}
	workspace.releaseBarrier()
	select {
	case <-stopped:
	case <-time.After(5 * time.Second):
		t.Fatal("Goal shutdown did not drain the candidate")
	}
	if got := countWorkEvents(t, backend, sessionID, domain.WorkEventGoalRoundAdmitted); got != 0 {
		t.Fatalf("Goal admissions after shutdown began = %d", got)
	}
	runs, err := backend.ListRunsBySession(ctx, sessionID)
	if err != nil || len(runs) != 0 {
		t.Fatalf("runs after shutdown = %+v / %v", runs, err)
	}
}

type failGoalPauseStore struct {
	storage.WorkStore
	err error
}

func (s *failGoalPauseStore) CommitWork(ctx context.Context, mutation domain.WorkMutation) (storage.WorkCommitResult, error) {
	if mutation.Kind == domain.WorkEventGoalPaused {
		return storage.WorkCommitResult{}, s.err
	}
	return s.WorkStore.CommitWork(ctx, mutation)
}

func TestGoalPausePersistenceFailureDisarmsWithoutDurablePause(t *testing.T) {
	ctx := context.Background()
	backend := openLifecycleBackend(t)
	const sessionID domain.SessionID = "sess-goal-pause-error"
	ref := createLifecycleGoal(t, backend, sessionID, 2)
	goalRuns := &notifyingGoalRunStore{GoalRunStore: backend, committed: make(chan storage.GoalRunCommitResult, 1)}
	svc := newGoalLifecycleService(t, backend, &blockingEinoModel{}, nil, goalRuns, nil)
	pauseErr := errors.New("pause storage unavailable")
	svc.deps.Work = &failGoalPauseStore{WorkStore: backend, err: pauseErr}
	t.Cleanup(func() { svc.StopAutomaticWork(); svc.CancelAll(); waitLifecycleIdle(t, svc) })
	svc.WakeGoal(sessionID)
	var admitted storage.GoalRunCommitResult
	select {
	case admitted = <-goalRuns.committed:
	case <-time.After(5 * time.Second):
		t.Fatal("Goal run was not admitted")
	}
	if _, err := svc.CommitWork(ctx, domain.WorkMutation{
		SessionID: sessionID, ExpectedVersion: 2, RequestID: "pause-error", RequestHash: "pause-error",
		Kind: domain.WorkEventGoalPaused, Goal: ref, Reason: "stop now",
	}); !errors.Is(err, pauseErr) {
		t.Fatalf("pause persistence error = %v", err)
	}
	if activation, current := svc.GoalActivation(sessionID); activation != "disarmed" || current != admitted.Run.ID {
		t.Fatalf("activation after failed pause = %q/%q", activation, current)
	}
	state, err := backend.ReadWork(ctx, sessionID)
	if err != nil || state.Goal == nil || state.Goal.Phase != domain.WorkPhaseActive || state.Version != 2 {
		t.Fatalf("durable Goal after failed pause = %+v / %v", state, err)
	}
	if got := countWorkEvents(t, backend, sessionID, domain.WorkEventGoalPaused); got != 0 {
		t.Fatalf("durable pause events after failed write = %d", got)
	}
}

func TestGoalRecoveryKeepsPendingQuestionDisarmed(t *testing.T) {
	ctx := context.Background()
	backend := openLifecycleBackend(t)
	const sessionID domain.SessionID = "sess-goal-question-restart"
	createLifecycleGoal(t, backend, sessionID, 2)
	toolset, err := tools.Builtin(backend).Resolve([]string{tools.AskUserName})
	if err != nil {
		t.Fatalf("resolve ask_user: %v", err)
	}
	checkpoints, err := NewVersionedCheckpointStore(backend.Blobs(), "test-engine")
	if err != nil {
		t.Fatalf("checkpoint store: %v", err)
	}
	engine, err := NewEngine(ctx, NewQuestionFlowModel(), toolset, EngineConfig{StreamBuffer: 8, MaxEventPayloadBytes: 64 << 10, Checkpoints: checkpoints})
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}
	deps := ServiceDeps{Journal: backend, Runs: backend, Messages: backend, Sessions: backend, PrimaryRuns: backend, GoalRuns: backend, Work: backend, Questions: backend, ApprovalExpiration: 5 * time.Minute, Sink: newTestSink()}
	initial := NewService(engine, "scripted", "scripted-v0", deps)
	initial.WakeGoal(sessionID)
	var runID domain.RunID
	deadline := time.After(5 * time.Second)
	for runID == "" {
		_, runID = initial.GoalActivation(sessionID)
		select {
		case <-deadline:
			t.Fatal("Goal question run was not admitted")
		default:
			runtime.Gosched()
		}
	}
	question := waitForPendingQuestion(t, backend, runID)
	if question.Status != domain.QuestionPending {
		t.Fatalf("question = %+v", question)
	}
	resumedEngine, err := NewEngine(ctx, NewScriptedModel(schema.AssistantMessage("Recovered answer.", nil)), toolset, EngineConfig{StreamBuffer: 8, MaxEventPayloadBytes: 64 << 10, Checkpoints: checkpoints})
	if err != nil {
		t.Fatalf("new resumed engine: %v", err)
	}
	deps.Sink = newTestSink()
	restarted := NewService(resumedEngine, "scripted", "scripted-v0", deps)
	if err := restarted.Recover(ctx); err != nil {
		t.Fatalf("recover: %v", err)
	}
	defer func() { restarted.StopAutomaticWork(); restarted.CancelAll(); waitLifecycleIdle(t, restarted) }()
	if activation, current := restarted.GoalActivation(sessionID); activation != "disarmed" || current != runID {
		t.Fatalf("recovered Goal activation = %q/%q", activation, current)
	}
	stored, err := backend.GetQuestion(ctx, question.ID)
	if err != nil || stored.Status != domain.QuestionPending {
		t.Fatalf("recovered question = %+v / %v", stored, err)
	}
	run, err := backend.GetRun(ctx, runID)
	if err != nil || run.Status != domain.RunActive {
		t.Fatalf("recovered run = %+v / %v", run, err)
	}
	state, err := backend.ReadWork(ctx, sessionID)
	if err != nil || state.Goal == nil || state.Goal.Phase != domain.WorkPhaseActive || state.Goal.RoundsStarted != 1 {
		t.Fatalf("recovered Work = %+v / %v", state, err)
	}
	if err := restarted.AnswerQuestion(ctx, question.ID, "blue"); err != nil {
		t.Fatalf("answer recovered question: %v", err)
	}
	waitForRunStatus(t, backend, runID, domain.RunCompleted)
	waitLifecycleIdle(t, restarted)
	if activation, current := restarted.GoalActivation(sessionID); activation != "disarmed" || current != "" {
		t.Fatalf("activation after recovered answer = %q/%q", activation, current)
	}
	if got := countWorkEvents(t, backend, sessionID, domain.WorkEventGoalRoundAdmitted); got != 1 {
		t.Fatalf("automatic admissions after recovered answer = %d", got)
	}
	runs, err := backend.ListRunsBySession(ctx, sessionID)
	if err != nil || len(runs) != 1 || runs[0].ID != runID {
		t.Fatalf("runs after recovered answer = %+v / %v", runs, err)
	}
}

func TestGoalRecoveryHumanQuestionDoesNotStartGoalRound(t *testing.T) {
	ctx := context.Background()
	backend := openLifecycleBackend(t)
	const sessionID domain.SessionID = "sess-human-question-goal-restart"
	if err := backend.CreateSession(ctx, domain.Session{ID: sessionID, CreatedAt: 1}); err != nil {
		t.Fatalf("create session: %v", err)
	}
	toolset, err := tools.Builtin(backend).Resolve([]string{tools.AskUserName})
	if err != nil {
		t.Fatalf("resolve ask_user: %v", err)
	}
	checkpoints, err := NewVersionedCheckpointStore(backend.Blobs(), "test-engine")
	if err != nil {
		t.Fatalf("checkpoint store: %v", err)
	}
	deps := ServiceDeps{Journal: backend, Runs: backend, Messages: backend, Sessions: backend, PrimaryRuns: backend, GoalRuns: backend, Work: backend, Questions: backend, ApprovalExpiration: 5 * time.Minute, Sink: newTestSink()}
	engine, err := NewEngine(ctx, NewQuestionFlowModel(), toolset, EngineConfig{StreamBuffer: 8, MaxEventPayloadBytes: 64 << 10, Checkpoints: checkpoints})
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}
	initial := NewService(engine, "scripted", "scripted-v0", deps)
	runID, err := initial.Run(ctx, sessionID, "ask before continuing")
	if err != nil {
		t.Fatalf("start human run: %v", err)
	}
	question := waitForPendingQuestion(t, backend, runID)
	if _, err := backend.CommitWork(ctx, domain.WorkMutation{
		SessionID: sessionID, RequestID: "human-created-goal", RequestHash: "human-created-goal",
		Kind: domain.WorkEventGoalCreated, Goal: domain.GoalRef{ID: "goal-after-question", Revision: 1},
		Objective: "continue after the answer", MaxRounds: 2, EvidenceRunID: runID,
	}); err != nil {
		t.Fatalf("record Goal created by human run: %v", err)
	}
	resumedEngine, err := NewEngine(ctx, NewScriptedModel(schema.AssistantMessage("The answer is noted.", nil)), toolset, EngineConfig{StreamBuffer: 8, MaxEventPayloadBytes: 64 << 10, Checkpoints: checkpoints})
	if err != nil {
		t.Fatalf("new resumed engine: %v", err)
	}
	deps.Sink = newTestSink()
	restarted := NewService(resumedEngine, "scripted", "scripted-v0", deps)
	if err := restarted.Recover(ctx); err != nil {
		t.Fatalf("recover: %v", err)
	}
	defer func() { restarted.StopAutomaticWork(); restarted.CancelAll(); waitLifecycleIdle(t, restarted) }()
	if got, err := backend.GetQuestion(ctx, question.ID); err != nil || got.Status != domain.QuestionPending {
		t.Fatalf("recovered Question = %+v / %v", got, err)
	}
	if activation, current := restarted.GoalActivation(sessionID); activation != "disarmed" || current != "" {
		t.Fatalf("recovered human-run Goal activation = %q/%q", activation, current)
	}
	if err := restarted.AnswerQuestion(ctx, question.ID, "blue"); err != nil {
		t.Fatalf("answer recovered Question: %v", err)
	}
	waitForRunStatus(t, backend, runID, domain.RunCompleted)
	waitLifecycleIdle(t, restarted)
	if got := countWorkEvents(t, backend, sessionID, domain.WorkEventGoalRoundAdmitted); got != 0 {
		t.Fatalf("automatic Goal rounds after recovered human answer = %d", got)
	}
	runs, err := backend.ListRunsBySession(ctx, sessionID)
	if err != nil || len(runs) != 1 || runs[0].ID != runID || runs[0].Status != domain.RunCompleted {
		t.Fatalf("runs after recovered human answer = %+v / %v", runs, err)
	}
	if got, err := backend.GetQuestion(ctx, question.ID); err != nil || got.Status != domain.QuestionAnswered {
		t.Fatalf("answered Question = %+v / %v", got, err)
	}
	state, err := backend.ReadWork(ctx, sessionID)
	if err != nil || state.Goal == nil || state.Goal.Phase != domain.WorkPhaseActive || state.Goal.RoundsStarted != 0 {
		t.Fatalf("durable Goal after recovered human answer = %+v / %v", state, err)
	}
}

func TestGoalRecoveryPreservesWorkerLostChild(t *testing.T) {
	ctx := context.Background()
	backend := openLifecycleBackend(t)
	const sessionID domain.SessionID = "sess-worker-lost-restart"
	if err := backend.CreateSession(ctx, domain.Session{ID: sessionID, CreatedAt: 1}); err != nil {
		t.Fatalf("create session: %v", err)
	}
	child := domain.Run{ID: "run-lost-child", SessionID: sessionID, Status: domain.RunActive, Kind: domain.RunKindChild, CreatedAt: 1}
	if err := backend.CreateRun(ctx, child); err != nil {
		t.Fatalf("create child: %v", err)
	}
	svc := newGoalLifecycleService(t, backend, NewScriptedModel(schema.AssistantMessage("must not execute", nil)), nil, backend, nil)
	if err := svc.Recover(ctx); err != nil {
		t.Fatalf("recover: %v", err)
	}
	run, err := backend.GetRun(ctx, child.ID)
	if err != nil || run.Status != domain.RunFailed {
		t.Fatalf("recovered child = %+v / %v", run, err)
	}
	events := replayAll(t, backend, child.ID)
	if len(events) != 1 || events[0].Type != domain.EventChildFailed {
		t.Fatalf("child events = %+v", events)
	}
	var failed payloadRunFailed
	if err := json.Unmarshal(events[0].Payload, &failed); err != nil {
		t.Fatalf("decode failed child: %v", err)
	}
	if failed.CauseCategory != "worker_lost_after_restart" {
		t.Fatalf("child failure cause = %q", failed.CauseCategory)
	}
}
