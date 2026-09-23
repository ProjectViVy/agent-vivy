package runtime

import (
	"context"
	"errors"
	"path/filepath"
	goruntime "runtime"
	"sync"
	"testing"
	"time"

	"github.com/cloudwego/eino/schema"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
	"agent-vivy/internal/storage/sqlite"
)

func TestAutomaticPrimaryCannotOvertakeRegisteredHumanIntent(t *testing.T) {
	ctx := context.Background()
	backend, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "primary-admission.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = backend.Close() })
	const sessionID domain.SessionID = "sess-primary-admission"
	if err := backend.CreateSession(ctx, domain.Session{ID: sessionID, Title: "Primary admission", CreatedAt: 1}); err != nil {
		t.Fatalf("create session: %v", err)
	}
	engine, err := NewEngine(ctx, NewScriptedModel(
		schema.AssistantMessage("Human turn completed.", nil),
		schema.AssistantMessage("Automatic turn completed.", nil),
	), nil, EngineConfig{StreamBuffer: 8, MaxEventPayloadBytes: 64 << 10})
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}
	svc := NewService(engine, "scripted", "scripted-v0", ServiceDeps{
		Journal: backend, Runs: backend, Messages: backend, Sessions: backend,
		PrimaryRuns: backend, Work: backend, Sink: newTestSink(),
	})
	gate := svc.sessionAdmission(sessionID)
	gate.Lock()
	gateHeld := true
	defer func() {
		if gateHeld {
			gate.Unlock()
		}
		idleCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if !svc.WaitIdle(idleCtx) {
			t.Error("service did not become idle")
		}
	}()

	type admissionResult struct {
		runID domain.RunID
		err   error
	}
	humanDone := make(chan admissionResult, 1)
	go func() {
		runID, err := svc.RunWithOptions(ctx, sessionID, "Human intent first.", RunOptions{HumanAdmission: true})
		humanDone <- admissionResult{runID: runID, err: err}
	}()
	waitForHumanIntent(t, svc, sessionID)

	autoDone := make(chan admissionResult, 1)
	go func() {
		runID, err := svc.RunWithOptions(ctx, sessionID, "Automatic turn.", RunOptions{})
		autoDone <- admissionResult{runID: runID, err: err}
	}()
	gate.Unlock()
	gateHeld = false
	var human admissionResult
	select {
	case human = <-humanDone:
	case <-time.After(5 * time.Second):
		t.Fatal("human admission did not leave the startup gate")
	}
	if human.err != nil || human.runID == "" {
		t.Fatalf("human admission = %+v, want committed run", human)
	}
	select {
	case auto := <-autoDone:
		if auto.runID != "" || !errors.Is(auto.err, storage.ErrWorkRunConflict) {
			t.Fatalf("automatic producer overtook registered human intent: result=%+v, want no RunID and existing run conflict", auto)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("automatic producer did not resolve after human admission")
	}
	waitForTerminalRun(t, backend, human.runID)

	messages, err := backend.ListMessages(ctx, sessionID)
	if err != nil {
		t.Fatalf("list session messages: %v", err)
	}
	var userMessages []domain.Message
	for _, message := range messages {
		if message.Role == domain.RoleUser {
			userMessages = append(userMessages, message)
		}
	}
	if len(userMessages) != 1 || userMessages[0].Content != "Human intent first." {
		t.Fatalf("persisted user messages = %+v, want only the registered human turn", userMessages)
	}
}

func waitForHumanIntent(t *testing.T, svc *Service, sessionID domain.SessionID) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		svc.mu.Lock()
		pending := svc.humanPending[sessionID]
		svc.mu.Unlock()
		if pending > 0 {
			return
		}
		goruntime.Gosched()
	}
	t.Fatal("human intent was not registered before the startup gate was released")
}

func TestWakeGoalCoalescesSignalsWithoutLosingInFlightEligibility(t *testing.T) {
	ctx := context.Background()
	backend, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "coalesced-goal-wake.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = backend.Close() })
	const sessionID domain.SessionID = "sess-coalesced-goal-wake"
	if err := backend.CreateSession(ctx, domain.Session{ID: sessionID, Title: "Coalesced wake", CreatedAt: 1}); err != nil {
		t.Fatalf("create session: %v", err)
	}
	ref := domain.GoalRef{ID: "goal-wake", Revision: 1}
	if _, err := backend.CommitWork(ctx, domain.WorkMutation{
		SessionID: sessionID, RequestID: "create-goal", RequestHash: "create-goal",
		Kind: domain.WorkEventGoalCreated, Goal: ref, Objective: "complete the bounded task", MaxRounds: 1,
	}); err != nil {
		t.Fatalf("create Goal: %v", err)
	}
	if _, err := backend.CommitWork(ctx, domain.WorkMutation{
		SessionID: sessionID, ExpectedVersion: 1, RequestID: "pause-goal", RequestHash: "pause-goal",
		Kind: domain.WorkEventGoalPaused, Goal: ref,
	}); err != nil {
		t.Fatalf("pause Goal: %v", err)
	}

	work := &barrierReadWorkStore{
		WorkStore: backend, firstRead: make(chan domain.WorkState, 1), releaseFirst: make(chan struct{}),
	}
	engine, err := NewEngine(ctx, NewScriptedModel(schema.AssistantMessage("Goal round finished.", nil)), nil, EngineConfig{
		StreamBuffer: 8, MaxEventPayloadBytes: 64 << 10,
	})
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}
	goalRuns := &notifyingGoalRunStore{GoalRunStore: backend, committed: make(chan storage.GoalRunCommitResult, 1)}
	svc := NewService(engine, "scripted", "scripted-v0", ServiceDeps{
		Journal: backend, Runs: backend, Messages: backend, Sessions: backend,
		PrimaryRuns: backend, GoalRuns: goalRuns, Work: work, Sink: newTestSink(),
	})
	defer func() {
		work.release()
		idleCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if !svc.WaitIdle(idleCtx) {
			t.Error("coalesced Goal wake worker did not become idle")
		}
	}()
	svc.WakeGoal(sessionID)
	select {
	case state := <-work.firstRead:
		if state.Goal == nil || state.Goal.Phase != domain.WorkPhasePaused {
			t.Fatalf("barrier read Goal=%+v, want paused candidate snapshot", state.Goal)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Goal candidate did not reach the controlled work-state barrier")
	}
	if _, err := backend.CommitWork(ctx, domain.WorkMutation{
		SessionID: sessionID, ExpectedVersion: 2, RequestID: "resume-goal", RequestHash: "resume-goal",
		Kind: domain.WorkEventGoalResumed, Goal: ref,
	}); err != nil {
		t.Fatalf("resume Goal during candidate: %v", err)
	}
	wakesDone := make(chan struct{})
	go func() {
		for i := 0; i < 64; i++ {
			svc.WakeGoal(sessionID)
		}
		close(wakesDone)
	}()
	select {
	case <-wakesDone:
	case <-time.After(2 * time.Second):
		t.Fatal("WakeGoal burst blocked behind an in-flight admission candidate")
	}
	svc.mu.Lock()
	_, workerRunning := svc.goalWakeRunning[sessionID]
	_, wakePending := svc.goalWakePending[sessionID]
	svc.mu.Unlock()
	if !workerRunning || !wakePending {
		t.Fatalf("coalesced wake state running=%v pending=%v, want one worker with one pending bit", workerRunning, wakePending)
	}
	work.release()
	var goalRun storage.GoalRunCommitResult
	select {
	case goalRun = <-goalRuns.committed:
	case <-time.After(5 * time.Second):
		t.Fatal("in-flight eligibility wake was lost before Goal admission")
	}
	waitForRunStatus(t, backend, goalRun.Run.ID, domain.RunCompleted)

	idleCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if !svc.WaitIdle(idleCtx) {
		t.Fatal("coalesced Goal wake worker did not become idle")
	}
	state, err := backend.ReadWork(ctx, sessionID)
	if err != nil {
		t.Fatalf("read final Goal state: %v", err)
	}
	if state.Goal == nil || state.Goal.RoundsStarted != 1 {
		t.Fatalf("Goal state after in-flight eligibility wake = %+v, want exactly one admitted round", state.Goal)
	}
	if got := countWorkEvents(t, backend, sessionID, domain.WorkEventGoalRoundAdmitted); got != 1 {
		t.Fatalf("durable Goal round admissions = %d, want exactly one", got)
	}
}

type barrierReadWorkStore struct {
	storage.WorkStore
	mu           sync.Mutex
	reads        int
	firstRead    chan domain.WorkState
	releaseFirst chan struct{}
	releaseOnce  sync.Once
}

type notifyingGoalRunStore struct {
	storage.GoalRunStore
	committed chan storage.GoalRunCommitResult
}

func (s *notifyingGoalRunStore) CommitGoalRun(ctx context.Context, admission storage.GoalRunCommit) (storage.GoalRunCommitResult, error) {
	result, err := s.GoalRunStore.CommitGoalRun(ctx, admission)
	if err == nil && !result.Work.Replayed {
		s.committed <- result
	}
	return result, err
}

func (s *barrierReadWorkStore) ReadWork(ctx context.Context, sessionID domain.SessionID) (domain.WorkState, error) {
	state, err := s.WorkStore.ReadWork(ctx, sessionID)
	s.mu.Lock()
	s.reads++
	first := s.reads == 1
	s.mu.Unlock()
	if first {
		s.firstRead <- state
		<-s.releaseFirst
	}
	return state, err
}

func (s *barrierReadWorkStore) readCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.reads
}

func (s *barrierReadWorkStore) release() {
	s.releaseOnce.Do(func() { close(s.releaseFirst) })
}

func countWorkEvents(t *testing.T, backend *sqlite.Backend, sessionID domain.SessionID, kind domain.WorkEventKind) int {
	t.Helper()
	events, _, err := backend.ReplayWork(context.Background(), sessionID, domain.WorkState{SessionID: sessionID}, 100)
	if err != nil {
		t.Fatalf("replay work: %v", err)
	}
	count := 0
	for _, event := range events {
		if event.Kind == kind {
			count++
		}
	}
	return count
}
