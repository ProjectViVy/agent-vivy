package rpc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"sync"
	"testing"
	"time"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/events"
	"agent-vivy/internal/runtime"
	"agent-vivy/internal/storage"
	"agent-vivy/internal/storage/sqlite"
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

func TestWorkViewDisarmsDurablyPausedOrClearedGoalDuringCancellation(t *testing.T) {
	for _, tc := range []struct {
		name string
		goal *domain.GoalState
	}{
		{name: "paused", goal: &domain.GoalState{Ref: domain.GoalRef{ID: "goal-1", Revision: 1}, Phase: domain.WorkPhasePaused}},
		{name: "cleared"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			state := domain.WorkState{SessionID: "sess-work-view", Version: 3, Goal: tc.goal}
			view := workStateView(state, "armed", "run-being-cancelled")
			if view.Activation != "disarmed" || view.CurrentRunID != "run-being-cancelled" {
				t.Fatalf("WorkView after durable %s = activation %q, current run %q; want disarmed with owned RunID", tc.name, view.Activation, view.CurrentRunID)
			}
		})
	}
}

func TestPauseAndClearReturnDisarmedWorkViewWhileOwnedRunIsCancelling(t *testing.T) {
	for _, operation := range []string{"goal/pause", "goal/clear"} {
		t.Run(operation, func(t *testing.T) {
			ctx := context.Background()
			model := &holdCancellationModel{entered: make(chan struct{}), release: make(chan struct{})}
			env, svc := newGoalLifecycleControlEnv(t, model, nil)
			defer func() {
				model.unblock()
				if svc != nil {
					svc.CancelAll()
					idleCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
					defer cancel()
					if !svc.WaitIdle(idleCtx) {
						t.Error("Goal run did not become idle")
					}
				}
			}()
			const sessionID domain.SessionID = "sess-work-disarm"
			if err := env.backend.CreateSession(ctx, domain.Session{ID: sessionID, CreatedAt: 1}); err != nil {
				t.Fatalf("create session: %v", err)
			}
			created, rpcErr := callControl(t, env.handler, "goal/create", map[string]any{
				"session_id": string(sessionID), "request_id": "create-goal", "expected_version": 0,
				"goal_id": "goal-disarm", "goal_revision": 1, "objective": "finish task", "max_rounds": 2,
			})
			if rpcErr != nil {
				t.Fatalf("goal/create: %v", rpcErr)
			}
			if result := created.(workCommitResult); result.Work.Goal == nil || result.Work.Goal.Phase != string(domain.WorkPhaseActive) {
				t.Fatalf("created WorkView = %+v", result.Work)
			}
			select {
			case <-model.entered:
			case <-time.After(5 * time.Second):
				t.Fatal("owned Goal run did not enter model barrier")
			}
			activation, runID := svc.GoalActivation(sessionID)
			if activation != "armed" || runID == "" {
				t.Fatalf("pre-mutation activation = %q / %q", activation, runID)
			}
			changed, rpcErr := callControl(t, env.handler, operation, map[string]any{
				"session_id": string(sessionID), "request_id": "stop-goal", "expected_version": 2,
				"goal_id": "goal-disarm", "goal_revision": 1, "reason": "user stopped it",
			})
			if rpcErr != nil {
				t.Fatalf("%s: %v", operation, rpcErr)
			}
			mutationView := changed.(workCommitResult).Work
			if mutationView.Activation != "disarmed" || mutationView.CurrentRunID != string(runID) {
				t.Fatalf("mutation WorkView = %+v", mutationView)
			}
			activation, current := svc.GoalActivation(sessionID)
			if activation != "armed" || current != runID {
				t.Fatalf("cancelling run no longer held in process map = %q / %q, want old RunID %q", activation, current, runID)
			}
			got, rpcErr := callControl(t, env.handler, "session/work/get", map[string]any{"session_id": string(sessionID)})
			if rpcErr != nil {
				t.Fatalf("session/work/get: %v", rpcErr)
			}
			getView := got.(workStateResult)
			if getView.Activation != "disarmed" || getView.CurrentRunID != string(runID) {
				t.Fatalf("get WorkView = %+v", getView)
			}
			state, err := env.backend.ReadWork(ctx, sessionID)
			if err != nil || state.Version != 3 {
				t.Fatalf("durable Work state = %+v / %v", state, err)
			}
			if operation == "goal/pause" && (state.Goal == nil || state.Goal.Phase != domain.WorkPhasePaused) {
				t.Fatalf("paused Work state = %+v", state.Goal)
			}
			if operation == "goal/clear" && state.Goal != nil {
				t.Fatalf("cleared Work state = %+v", state.Goal)
			}
			model.unblock()
		})
	}
}

func newGoalLifecycleControlEnv(t *testing.T, model domain.ChatModel, wrapWork func(storage.WorkStore) storage.WorkStore) (*controlTestEnv, *runtime.Service) {
	t.Helper()
	var svc *runtime.Service
	env := newControlTestEnv(t, func(deps *ControlDeps) {
		backend := deps.Sessions.(*sqlite.Backend)
		engine, err := runtime.NewEngine(context.Background(), runtime.WrapModel(model), nil, runtime.EngineConfig{
			StreamBuffer: 8, MaxEventPayloadBytes: 64 << 10,
		})
		if err != nil {
			t.Fatalf("new engine: %v", err)
		}
		var work storage.WorkStore = backend
		if wrapWork != nil {
			work = wrapWork(work)
		}
		svc = runtime.NewService(engine, "test", "test-model", runtime.ServiceDeps{
			Journal: backend, Runs: backend, Messages: backend, Sessions: backend,
			PrimaryRuns: backend, GoalRuns: backend, Work: work, Sink: deps.Bus,
		})
		deps.Service = svc
		deps.Work = work
	})
	return env, svc
}

type failGoalPauseWorkStore struct{ storage.WorkStore }

func (s *failGoalPauseWorkStore) CommitWork(ctx context.Context, mutation domain.WorkMutation) (storage.WorkCommitResult, error) {
	if mutation.Kind == domain.WorkEventGoalPaused {
		return storage.WorkCommitResult{}, errors.New("injected pause storage failure")
	}
	return s.WorkStore.CommitWork(ctx, mutation)
}

func TestGoalEditResponseReflectsRearmedOwnedRun(t *testing.T) {
	ctx := context.Background()
	model := &holdCancellationModel{entered: make(chan struct{}), release: make(chan struct{})}
	env, svc := newGoalLifecycleControlEnv(t, model, func(inner storage.WorkStore) storage.WorkStore {
		return &failGoalPauseWorkStore{WorkStore: inner}
	})
	defer func() { svc.StopAutomaticWork(); model.unblock(); svc.CancelAll(); waitForGoalControlIdle(t, svc) }()
	const sessionID domain.SessionID = "sess-edit-rearmed-response"
	if err := env.backend.CreateSession(ctx, domain.Session{ID: sessionID, CreatedAt: 1}); err != nil {
		t.Fatalf("create session: %v", err)
	}
	if _, rpcErr := callControl(t, env.handler, "goal/create", map[string]any{
		"session_id": string(sessionID), "request_id": "create", "expected_version": 0,
		"goal_id": "goal-edit-rearm", "goal_revision": 1, "objective": "original", "max_rounds": 2,
	}); rpcErr != nil {
		t.Fatalf("goal/create: %v", rpcErr)
	}
	select {
	case <-model.entered:
	case <-time.After(5 * time.Second):
		t.Fatal("Goal run did not reach model barrier")
	}
	_, runID := svc.GoalActivation(sessionID)
	if runID == "" {
		t.Fatal("Goal run has no RunID")
	}
	if _, rpcErr := callControl(t, env.handler, "goal/pause", map[string]any{
		"session_id": string(sessionID), "request_id": "pause-error", "expected_version": 2,
		"goal_id": "goal-edit-rearm", "goal_revision": 1, "reason": "stop",
	}); rpcErr == nil {
		t.Fatal("failed pause write returned success")
	}
	if activation, current := svc.GoalActivation(sessionID); activation != "disarmed" || current != runID {
		t.Fatalf("activation after failed pause = %q/%q", activation, current)
	}
	got, rpcErr := callControl(t, env.handler, "goal/edit", map[string]any{
		"session_id": string(sessionID), "request_id": "edit", "expected_version": 2,
		"goal_id": "goal-edit-rearm", "goal_revision": 1, "objective": "revised", "max_rounds": 2,
	})
	if rpcErr != nil {
		t.Fatalf("goal/edit: %v", rpcErr)
	}
	view := got.(workCommitResult).Work
	if view.Activation != "armed" || view.CurrentRunID != string(runID) {
		t.Fatalf("edit response after explicit re-arm = %+v", view)
	}
	if activation, current := svc.GoalActivation(sessionID); activation != "armed" || current != runID {
		t.Fatalf("live activation after edit = %q/%q", activation, current)
	}
}

type holdCancellationModel struct {
	entered    chan struct{}
	release    chan struct{}
	cancelled  chan struct{}
	start      sync.Once
	done       sync.Once
	cancelOnce sync.Once
}

func (m *holdCancellationModel) Stream(ctx context.Context, _ []*domain.Message) (domain.Stream[*domain.Message], error) {
	m.start.Do(func() { close(m.entered) })
	if m.cancelled != nil {
		context.AfterFunc(ctx, func() { m.cancelOnce.Do(func() { close(m.cancelled) }) })
	}
	<-m.release
	return nil, ctx.Err()
}
func (m *holdCancellationModel) unblock() { m.done.Do(func() { close(m.release) }) }

func TestEditedGoalStartsNewRevisionAfterOldRunCleanup(t *testing.T) {
	ctx := context.Background()
	model := &heldTwoRoundModel{
		firstEntered: make(chan struct{}), firstRelease: make(chan struct{}),
		secondEntered: make(chan struct{}), secondRelease: make(chan struct{}),
	}
	env, svc := newGoalLifecycleControlEnv(t, model, nil)
	defer func() {
		model.releaseAll()
		svc.CancelAll()
		idleCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		if !svc.WaitIdle(idleCtx) {
			t.Error("edited Goal did not become idle")
		}
	}()
	const sessionID domain.SessionID = "sess-edit-live-goal"
	if err := env.backend.CreateSession(ctx, domain.Session{ID: sessionID, CreatedAt: 1}); err != nil {
		t.Fatalf("create session: %v", err)
	}
	if _, rpcErr := callControl(t, env.handler, "goal/create", map[string]any{
		"session_id": string(sessionID), "request_id": "create", "expected_version": 0,
		"goal_id": "goal-edit-live", "goal_revision": 1, "objective": "original", "max_rounds": 2,
	}); rpcErr != nil {
		t.Fatalf("goal/create: %v", rpcErr)
	}
	select {
	case <-model.firstEntered:
	case <-time.After(5 * time.Second):
		t.Fatal("first Goal run did not enter model barrier")
	}
	activation, oldRunID := svc.GoalActivation(sessionID)
	if activation != "armed" || oldRunID == "" {
		t.Fatalf("old Goal run = %q / %q", activation, oldRunID)
	}
	if _, rpcErr := callControl(t, env.handler, "goal/edit", map[string]any{
		"session_id": string(sessionID), "request_id": "edit", "expected_version": 2,
		"goal_id": "goal-edit-live", "goal_revision": 1, "objective": "revised", "max_rounds": 2,
	}); rpcErr != nil {
		t.Fatalf("goal/edit: %v", rpcErr)
	}
	// The old run still owns the session, and its report cannot cross the
	// durable GoalRef CAS into revision 2.
	_, err := env.backend.CommitWork(ctx, domain.WorkMutation{
		SessionID: sessionID, ExpectedVersion: 3, RequestID: "old-report", RequestHash: "old-report",
		Kind: domain.WorkEventGoalCompleted, Goal: domain.GoalRef{ID: "goal-edit-live", Revision: 1},
		Reason: "old run done", EvidenceRunID: oldRunID,
	})
	if !errors.Is(err, domain.ErrStaleGoalReference) {
		t.Fatalf("old-revision report = %v, want stale Goal reference", err)
	}
	runs, err := env.backend.ListRunsBySession(ctx, sessionID)
	if err != nil || len(runs) != 1 || runs[0].ID != oldRunID {
		t.Fatalf("runs before old cleanup = %+v / %v", runs, err)
	}
	model.releaseFirst()
	select {
	case <-model.secondEntered:
	case <-time.After(5 * time.Second):
		t.Fatal("edited active Goal did not start a new revision after old run cleanup")
	}
	runs, err = env.backend.ListRunsBySession(ctx, sessionID)
	if err != nil || len(runs) != 2 {
		t.Fatalf("runs after old cleanup = %+v / %v", runs, err)
	}
	var revisedRun domain.Run
	for _, run := range runs {
		if run.ID == oldRunID {
			if run.Status != domain.RunCompleted {
				t.Fatalf("old run status = %s, want completed before revised start", run.Status)
			}
		} else {
			revisedRun = run
		}
	}
	if revisedRun.ID == "" || revisedRun.Status != domain.RunActive {
		t.Fatalf("revised run = %+v", revisedRun)
	}
	state, err := env.backend.ReadWork(ctx, sessionID)
	if err != nil || state.Goal == nil || state.Goal.Ref.Revision != 2 || state.Goal.RoundsStarted != 2 || state.Goal.EvidenceRunID != revisedRun.ID {
		t.Fatalf("revised Goal state = %+v / %v", state, err)
	}
	messages, err := env.backend.ListMessages(ctx, sessionID)
	if err != nil {
		t.Fatalf("list messages: %v", err)
	}
	userRuns := make(map[domain.RunID]bool)
	for _, message := range messages {
		if message.Role == domain.RoleUser {
			userRuns[message.RunID] = true
		}
	}
	if len(userRuns) != 2 || !userRuns[oldRunID] || !userRuns[revisedRun.ID] {
		t.Fatalf("user message RunIDs = %+v, want both admitted runs", userRuns)
	}
	model.releaseSecond()
}

type heldTwoRoundModel struct {
	mu            sync.Mutex
	calls         int
	firstEntered  chan struct{}
	firstRelease  chan struct{}
	secondEntered chan struct{}
	secondRelease chan struct{}
	firstDone     sync.Once
	secondDone    sync.Once
}

func (m *heldTwoRoundModel) Stream(ctx context.Context, _ []*domain.Message) (domain.Stream[*domain.Message], error) {
	m.mu.Lock()
	m.calls++
	call := m.calls
	m.mu.Unlock()
	switch call {
	case 1:
		close(m.firstEntered)
		select {
		case <-m.firstRelease:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	case 2:
		close(m.secondEntered)
		select {
		case <-m.secondRelease:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	default:
		return nil, fmt.Errorf("unexpected Goal model call %d", call)
	}
	return &singleGoalMessage{message: &domain.Message{Role: domain.RoleAssistant, Content: "round completed"}}, nil
}
func (m *heldTwoRoundModel) releaseFirst()  { m.firstDone.Do(func() { close(m.firstRelease) }) }
func (m *heldTwoRoundModel) releaseSecond() { m.secondDone.Do(func() { close(m.secondRelease) }) }
func (m *heldTwoRoundModel) releaseAll()    { m.releaseFirst(); m.releaseSecond() }

type singleGoalMessage struct{ message *domain.Message }

func (s *singleGoalMessage) Recv() (*domain.Message, error) {
	if s.message == nil {
		return nil, io.EOF
	}
	message := s.message
	s.message = nil
	return message, nil
}

func TestRunCancelDisarmsOwnedGoalBeforeSignallingRun(t *testing.T) {
	ctx := context.Background()
	model := &holdCancellationModel{entered: make(chan struct{}), release: make(chan struct{}), cancelled: make(chan struct{})}
	env, svc := newGoalLifecycleControlEnv(t, model, nil)
	defer func() { model.unblock(); svc.CancelAll(); waitForGoalControlIdle(t, svc) }()
	const sessionID domain.SessionID = "sess-direct-goal-cancel"
	if err := env.backend.CreateSession(ctx, domain.Session{ID: sessionID, CreatedAt: 1}); err != nil {
		t.Fatalf("create session: %v", err)
	}
	if _, rpcErr := callControl(t, env.handler, "goal/create", map[string]any{
		"session_id": string(sessionID), "request_id": "create", "expected_version": 0,
		"goal_id": "goal-direct-cancel", "goal_revision": 1, "objective": "run", "max_rounds": 2,
	}); rpcErr != nil {
		t.Fatalf("goal/create: %v", rpcErr)
	}
	select {
	case <-model.entered:
	case <-time.After(5 * time.Second):
		t.Fatal("Goal run did not enter model barrier")
	}
	_, runID := svc.GoalActivation(sessionID)
	if runID == "" {
		t.Fatal("Goal has no admitted RunID")
	}
	if _, rpcErr := callControl(t, env.handler, "run/cancel", map[string]any{"run_id": string(runID)}); rpcErr != nil {
		t.Fatalf("run/cancel: %v", rpcErr)
	}
	state, err := env.backend.ReadWork(ctx, sessionID)
	if err != nil || state.Goal == nil || state.Goal.Phase != domain.WorkPhaseBlocked || state.Goal.Reason != "goal round cancelled" || state.Goal.EvidenceRunID != runID || state.Goal.RoundsStarted != 1 {
		t.Fatalf("durable Goal after run/cancel = %+v / %v", state, err)
	}
	select {
	case <-model.cancelled:
	case <-time.After(5 * time.Second):
		t.Fatal("run/cancel did not signal the model after durable disarm")
	}
	got, rpcErr := callControl(t, env.handler, "session/work/get", map[string]any{"session_id": string(sessionID)})
	if rpcErr != nil {
		t.Fatalf("session/work/get: %v", rpcErr)
	}
	view := got.(workStateResult)
	if view.Activation != "disarmed" || view.CurrentRunID != string(runID) {
		t.Fatalf("WorkView while blocked run is cancelling = %+v", view)
	}
	run, err := env.backend.GetRun(ctx, runID)
	if err != nil || run.Status != domain.RunActive {
		t.Fatalf("held cancelling run = %+v / %v", run, err)
	}
	model.unblock()
	waitForGoalControlIdle(t, svc)
	run, err = env.backend.GetRun(ctx, runID)
	if err != nil || run.Status != domain.RunCancelled {
		t.Fatalf("terminal cancelled run = %+v / %v", run, err)
	}
}

func TestRunCancelPersistenceFailureDoesNotSignalOwnedGoal(t *testing.T) {
	ctx := context.Background()
	model := &holdCancellationModel{entered: make(chan struct{}), release: make(chan struct{}), cancelled: make(chan struct{})}
	store := &failOneGoalBlockStore{err: errors.New("injected Goal block persistence failure")}
	env, svc := newGoalLifecycleControlEnv(t, model, func(inner storage.WorkStore) storage.WorkStore {
		store.WorkStore = inner
		return store
	})
	defer func() { model.unblock(); svc.CancelAll(); waitForGoalControlIdle(t, svc) }()
	const sessionID domain.SessionID = "sess-goal-cancel-failure"
	if err := env.backend.CreateSession(ctx, domain.Session{ID: sessionID, CreatedAt: 1}); err != nil {
		t.Fatalf("create session: %v", err)
	}
	if _, rpcErr := callControl(t, env.handler, "goal/create", map[string]any{
		"session_id": string(sessionID), "request_id": "create", "expected_version": 0,
		"goal_id": "goal-cancel-failure", "goal_revision": 1, "objective": "run", "max_rounds": 2,
	}); rpcErr != nil {
		t.Fatalf("goal/create: %v", rpcErr)
	}
	select {
	case <-model.entered:
	case <-time.After(5 * time.Second):
		t.Fatal("Goal run did not enter model barrier")
	}
	_, runID := svc.GoalActivation(sessionID)
	if runID == "" {
		t.Fatal("Goal has no admitted RunID")
	}
	if _, rpcErr := callControl(t, env.handler, "run/cancel", map[string]any{"run_id": string(runID)}); rpcErr == nil {
		t.Fatal("run/cancel accepted a Goal whose durable block failed")
	}
	select {
	case <-model.cancelled:
		t.Fatal("run/cancel signalled the run despite block persistence failure")
	default:
	}
	state, err := env.backend.ReadWork(ctx, sessionID)
	if err != nil || state.Goal == nil || state.Goal.Phase != domain.WorkPhaseActive || state.Goal.RoundsStarted != 1 {
		t.Fatalf("Goal after failed disarm = %+v / %v", state, err)
	}
	run, err := env.backend.GetRun(ctx, runID)
	if err != nil || run.Status != domain.RunActive {
		t.Fatalf("run after failed disarm = %+v / %v", run, err)
	}
}

func TestRunCancelPreservesOrdinaryNonGoalCancellation(t *testing.T) {
	ctx := context.Background()
	model := &holdCancellationModel{entered: make(chan struct{}), release: make(chan struct{}), cancelled: make(chan struct{})}
	env, svc := newGoalLifecycleControlEnv(t, model, nil)
	defer func() { model.unblock(); svc.CancelAll(); waitForGoalControlIdle(t, svc) }()
	const sessionID domain.SessionID = "sess-ordinary-cancel"
	if err := env.backend.CreateSession(ctx, domain.Session{ID: sessionID, CreatedAt: 1}); err != nil {
		t.Fatalf("create session: %v", err)
	}
	started, rpcErr := callControl(t, env.handler, "turn/start", map[string]any{"session_id": string(sessionID), "text": "hello"})
	if rpcErr != nil {
		t.Fatalf("turn/start: %v", rpcErr)
	}
	accepted := started.(map[string]any)
	runID := accepted["run_id"].(domain.RunID)
	select {
	case <-model.entered:
	case <-time.After(5 * time.Second):
		t.Fatal("ordinary run did not enter model barrier")
	}
	if _, rpcErr := callControl(t, env.handler, "run/cancel", map[string]any{"run_id": string(runID)}); rpcErr != nil {
		t.Fatalf("ordinary run/cancel: %v", rpcErr)
	}
	select {
	case <-model.cancelled:
	case <-time.After(5 * time.Second):
		t.Fatal("ordinary run/cancel did not signal the model")
	}
	state, err := env.backend.ReadWork(ctx, sessionID)
	if err != nil || state.Goal != nil || state.Version != 0 {
		t.Fatalf("ordinary cancellation changed Goal work = %+v / %v", state, err)
	}
	model.unblock()
	waitForGoalControlIdle(t, svc)
	run, err := env.backend.GetRun(ctx, runID)
	if err != nil || run.Status != domain.RunCancelled {
		t.Fatalf("ordinary cancelled run = %+v / %v", run, err)
	}
}

type goalCancelReadKey struct{}

type heldGoalCancelReadStore struct {
	storage.WorkStore
	cancelEntered     chan struct{}
	cancelRelease     chan struct{}
	settlementEntered chan struct{}
	settlementRelease chan struct{}
	cancelOnce        sync.Once
	settlementOnce    sync.Once
}

func (s *heldGoalCancelReadStore) ReadWork(ctx context.Context, sessionID domain.SessionID) (domain.WorkState, error) {
	state, err := s.WorkStore.ReadWork(ctx, sessionID)
	if ctx.Value(goalCancelReadKey{}) == true {
		s.cancelOnce.Do(func() { close(s.cancelEntered) })
		<-s.cancelRelease
	} else if state.Goal != nil && state.Goal.RoundsStarted == 1 {
		select {
		case <-s.cancelEntered:
			s.settlementOnce.Do(func() { close(s.settlementEntered) })
			<-s.settlementRelease
		default:
		}
	}
	return state, err
}

func TestRunCancelDoesNotBlockGoalAfterItsRunCompleted(t *testing.T) {
	ctx := context.Background()
	model := &heldTwoRoundModel{
		firstEntered: make(chan struct{}), firstRelease: make(chan struct{}),
		secondEntered: make(chan struct{}), secondRelease: make(chan struct{}),
	}
	readBarrier := &heldGoalCancelReadStore{
		cancelEntered: make(chan struct{}), cancelRelease: make(chan struct{}),
		settlementEntered: make(chan struct{}), settlementRelease: make(chan struct{}),
	}
	env, svc := newGoalLifecycleControlEnv(t, model, func(inner storage.WorkStore) storage.WorkStore {
		readBarrier.WorkStore = inner
		return readBarrier
	})
	var readReleaseOnce, settlementReleaseOnce sync.Once
	releaseRead := func() { readReleaseOnce.Do(func() { close(readBarrier.cancelRelease) }) }
	releaseSettlement := func() { settlementReleaseOnce.Do(func() { close(readBarrier.settlementRelease) }) }
	defer func() {
		releaseRead()
		releaseSettlement()
		model.releaseAll()
		svc.CancelAll()
		waitForGoalControlIdle(t, svc)
	}()
	const sessionID domain.SessionID = "sess-complete-races-cancel"
	if err := env.backend.CreateSession(ctx, domain.Session{ID: sessionID, CreatedAt: 1}); err != nil {
		t.Fatalf("create session: %v", err)
	}
	if _, rpcErr := callControl(t, env.handler, "goal/create", map[string]any{
		"session_id": string(sessionID), "request_id": "create", "expected_version": 0,
		"goal_id": "goal-complete-races-cancel", "goal_revision": 1, "objective": "run", "max_rounds": 2,
	}); rpcErr != nil {
		t.Fatalf("goal/create: %v", rpcErr)
	}
	select {
	case <-model.firstEntered:
	case <-time.After(5 * time.Second):
		t.Fatal("first Goal run did not enter model barrier")
	}
	_, oldRunID := svc.GoalActivation(sessionID)
	if oldRunID == "" {
		t.Fatal("Goal has no admitted RunID")
	}
	params, err := json.Marshal(map[string]any{"run_id": string(oldRunID)})
	if err != nil {
		t.Fatalf("encode cancel request: %v", err)
	}
	type cancelResult struct {
		result any
		rpcErr *Error
	}
	cancelled := make(chan cancelResult, 1)
	go func() {
		result, rpcErr := env.handler.Handle(context.WithValue(ctx, goalCancelReadKey{}, true), nil, Request{
			JSONRPC: "2.0", Method: "run/cancel", Params: params,
		})
		cancelled <- cancelResult{result: result, rpcErr: rpcErr}
	}()
	select {
	case <-readBarrier.cancelEntered:
	case <-time.After(5 * time.Second):
		t.Fatal("cancel did not capture old Goal Work state")
	}
	model.releaseFirst()
	select {
	case <-readBarrier.settlementEntered:
	case <-time.After(5 * time.Second):
		t.Fatal("completed run did not reach post-cleanup settlement")
	}
	oldRun, err := env.backend.GetRun(ctx, oldRunID)
	if err != nil || oldRun.Status != domain.RunCompleted {
		t.Fatalf("old run before cancel resumes = %+v / %v", oldRun, err)
	}
	releaseRead()
	select {
	case result := <-cancelled:
		if result.rpcErr == nil || result.rpcErr.Code != CodeNotFound || result.result != nil {
			t.Fatalf("late run/cancel = %+v, want not-active error without Work side effect", result)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("late run/cancel did not return")
	}
	releaseSettlement()
	model.releaseSecond()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		state, readErr := env.backend.ReadWork(ctx, sessionID)
		if readErr != nil {
			t.Fatalf("read settled Goal: %v", readErr)
		}
		if state.Goal != nil && state.Goal.Phase == domain.WorkPhaseBlocked {
			break
		}
		time.Sleep(time.Millisecond)
	}
	waitForGoalControlIdle(t, svc)
	state, err := env.backend.ReadWork(ctx, sessionID)
	if err != nil || state.Goal == nil || state.Goal.Reason != "goal round limit reached" || state.Goal.RoundsStarted != 2 {
		t.Fatalf("Goal after completion won cancellation = %+v / %v", state, err)
	}
	runs, err := env.backend.ListRunsBySession(ctx, sessionID)
	if err != nil || len(runs) != 2 || runs[0].ID == runs[1].ID {
		t.Fatalf("Goal runs after completion won cancellation = %+v / %v", runs, err)
	}
	for _, run := range runs {
		if run.Status != domain.RunCompleted {
			t.Fatalf("Goal run = %+v, want completed", run)
		}
	}
}

type failOneGoalBlockStore struct {
	storage.WorkStore
	mu     sync.Mutex
	failed bool
	err    error
}

func (s *failOneGoalBlockStore) CommitWork(ctx context.Context, mutation domain.WorkMutation) (storage.WorkCommitResult, error) {
	s.mu.Lock()
	fail := mutation.Kind == domain.WorkEventGoalBlocked && !s.failed
	if fail {
		s.failed = true
	}
	s.mu.Unlock()
	if fail {
		return storage.WorkCommitResult{}, s.err
	}
	return s.WorkStore.CommitWork(ctx, mutation)
}

func waitForGoalControlIdle(t *testing.T, svc *runtime.Service) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if !svc.WaitIdle(ctx) {
		t.Fatal("Goal control service did not become idle")
	}
}

func TestBuildWorkMutationRejectsModelOnlyPlanSubmit(t *testing.T) {
	if _, rpcErr := buildWorkMutation("plan/submit", domain.WorkEventPlanSubmitted, workParams{
		SessionID: "session-1", RequestID: "model-only-submit",
	}); rpcErr == nil || rpcErr.Code != InvalidParams {
		t.Fatalf("buildWorkMutation PlanSubmitted error = %v, want invalid params", rpcErr)
	}
}

func TestPublicPlanSubmitRejectsCallerOriginWithoutMutation(t *testing.T) {
	ctx := context.Background()
	env := newControlTestEnv(t, func(deps *ControlDeps) {
		deps.Work = deps.Sessions.(storage.WorkStore)
	})
	const sessionID domain.SessionID = "sess-plan-submit-origin"
	const runID domain.RunID = "run-plan-submit-origin"
	if err := env.backend.CreateSession(ctx, domain.Session{ID: sessionID, CreatedAt: 1}); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	if err := env.backend.CreateRun(ctx, domain.Run{
		ID: runID, SessionID: sessionID, Status: domain.RunActive, Kind: domain.RunKindPrimary, CreatedAt: 2,
	}); err != nil {
		t.Fatalf("CreateRun: %v", err)
	}
	if _, rpcErr := callControl(t, env.handler, "plan/enter", map[string]any{
		"session_id": string(sessionID), "request_id": "enter-plan", "expected_version": 0,
	}); rpcErr != nil {
		t.Fatalf("plan/enter: %v", rpcErr)
	}
	beforeState, err := env.backend.ReadWork(ctx, sessionID)
	if err != nil {
		t.Fatalf("ReadWork before submit: %v", err)
	}
	beforeReplay, _, err := env.backend.ReplayWork(ctx, sessionID, domain.WorkState{SessionID: sessionID}, 100)
	if err != nil {
		t.Fatalf("ReplayWork before submit: %v", err)
	}
	_, rpcErr := callControl(t, env.handler, "plan/submit", map[string]any{
		"session_id": string(sessionID), "request_id": "forged-submit", "expected_version": int64(beforeState.Version),
		"submission_id": "forged-submission", "markdown": "# forged plan",
		"origin_run_id": string(runID), "origin_tool_call_id": "forged-tool-call",
	})
	if rpcErr == nil || rpcErr.Code != MethodNotFound || rpcErr.Message != "method not found: plan/submit" {
		t.Fatalf("plan/submit error = %v, want an unregistered model-only operation", rpcErr)
	}
	afterState, err := env.backend.ReadWork(ctx, sessionID)
	if err != nil {
		t.Fatalf("ReadWork after submit: %v", err)
	}
	afterReplay, _, err := env.backend.ReplayWork(ctx, sessionID, domain.WorkState{SessionID: sessionID}, 100)
	if err != nil {
		t.Fatalf("ReplayWork after submit: %v", err)
	}
	if !reflect.DeepEqual(afterState, beforeState) {
		t.Fatalf("work state changed after rejected plan/submit: before=%+v after=%+v", beforeState, afterState)
	}
	if !reflect.DeepEqual(afterReplay, beforeReplay) {
		t.Fatalf("work replay changed after rejected plan/submit: before=%+v after=%+v", beforeReplay, afterReplay)
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
	if err := env.backend.CreateRun(ctx, domain.Run{
		ID: "run-plan-pages", SessionID: sessionID, Status: domain.RunActive,
		Kind: domain.RunKindPrimary, CreatedAt: 2,
	}); err != nil {
		t.Fatalf("CreateRun: %v", err)
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
		if kind == domain.WorkEventPlanSubmitted {
			mutation.PlanOriginRunID = "run-plan-pages"
			mutation.PlanOriginToolCallID = fmt.Sprintf("tool-%d", version)
		}
		if kind == domain.WorkEventPlanReviewSuspended {
			mutation.PlanOriginRunID = "run-plan-pages"
			mutation.PlanOriginToolCallID = fmt.Sprintf("tool-%d", version-1)
			mutation.PlanResumeTarget = fmt.Sprintf("resume-%d", version)
		}
		result, err := env.backend.CommitWork(ctx, mutation)
		if err != nil {
			t.Fatalf("CommitWork %s at %d: %v", kind, version, err)
		}
		version = result.State.Version
	}
	commit(domain.WorkEventPlanEntered, "", "")
	for i := 0; i < 86; i++ {
		id := fmt.Sprintf("submission-%d", i)
		commit(domain.WorkEventPlanSubmitted, id, "# plan")
		commit(domain.WorkEventPlanReviewSuspended, id, "")
		commit(domain.WorkEventPlanDecided, id, "")
	}
	if version != 259 {
		t.Fatalf("work version = %d, want decision past first 256-event page", version)
	}
	result, rpcErr := callControl(t, env.handler, "plan/get", map[string]string{
		"session_id": string(sessionID), "submission_id": "submission-85",
	})
	if rpcErr != nil {
		t.Fatalf("plan/get: %v", rpcErr)
	}
	plan, ok := result.(workPlanResult)
	if !ok || plan.SubmissionID != "submission-85" || plan.ReviewStatus != string(domain.PlanReviewRejected) {
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
	ch, stopLive := bus.Subscribe(sessionID)
	work, err := env.backend.ReadWork(ctx, sessionID)
	if err != nil {
		t.Fatalf("ReadWork watermark: %v", err)
	}
	done := make(chan struct{})
	go func() {
		env.handler.(*controlHandler).streamWorkBuffered(streamCtx, peer, "sub-1", sessionID, 1, work.Version, ch, stopLive)
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
