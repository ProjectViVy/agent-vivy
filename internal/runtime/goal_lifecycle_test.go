package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
	"agent-vivy/internal/storage/sqlite"
	"agent-vivy/internal/tools"
)

func newGoalLifecycleService(t *testing.T, backend *sqlite.Backend, chat model.ToolCallingChatModel, workspace WorkspaceAllocator, goalRuns storage.GoalRunStore, toolset []tools.Tool) *Service {
	t.Helper()
	ctx := context.Background()
	engine, err := NewEngine(ctx, chat, toolset, EngineConfig{StreamBuffer: 8, MaxEventPayloadBytes: 64 << 10})
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}
	return NewService(engine, "scripted", "scripted-v0", ServiceDeps{
		Journal: backend, Runs: backend, Messages: backend, Sessions: backend,
		PrimaryRuns: backend, GoalRuns: goalRuns, Work: backend, Workspaces: workspace,
		Approvals: backend, Questions: backend, Sink: newTestSink(),
	})
}

func createLifecycleGoal(t *testing.T, backend *sqlite.Backend, sessionID domain.SessionID, maxRounds int) domain.GoalRef {
	return createLifecycleGoalWithSession(t, backend, domain.Session{ID: sessionID, Title: "Goal lifecycle", CreatedAt: 1}, maxRounds)
}

func createLifecycleGoalWithSession(t *testing.T, backend *sqlite.Backend, session domain.Session, maxRounds int) domain.GoalRef {
	t.Helper()
	ctx := context.Background()
	if err := backend.CreateSession(ctx, session); err != nil {
		t.Fatalf("create session: %v", err)
	}
	ref := domain.GoalRef{ID: "goal-lifecycle", Revision: 1}
	if _, err := backend.CommitWork(ctx, domain.WorkMutation{
		SessionID: session.ID, RequestID: "create-goal", RequestHash: "create-goal",
		Kind: domain.WorkEventGoalCreated, Goal: ref, Objective: "complete the lifecycle task", MaxRounds: maxRounds,
	}); err != nil {
		t.Fatalf("create Goal: %v", err)
	}
	return ref
}

func openLifecycleBackend(t *testing.T) *sqlite.Backend {
	t.Helper()
	backend, err := sqlite.Open(context.Background(), filepath.Join(t.TempDir(), "goal-lifecycle.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = backend.Close() })
	return backend
}

func waitLifecycleIdle(t *testing.T, svc *Service) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if !svc.WaitIdle(ctx) {
		t.Fatal("Goal lifecycle service did not become idle")
	}
}

func TestGoalReservationCannotCommitAfterPauseOrEdit(t *testing.T) {
	for _, kind := range []domain.WorkEventKind{domain.WorkEventGoalPaused, domain.WorkEventGoalEdited} {
		t.Run(string(kind), func(t *testing.T) {
			ctx := context.Background()
			backend := openLifecycleBackend(t)
			const sessionID domain.SessionID = "sess-goal-reservation"
			ref := createLifecycleGoal(t, backend, sessionID, 2)
			workspace := &precommitBarrierWorkspaceAllocator{entered: make(chan struct{}), release: make(chan struct{}), path: t.TempDir()}
			svc := newGoalLifecycleService(t, backend, NewScriptedModel(schema.AssistantMessage("unexpected", nil)), workspace, backend, nil)
			defer func() { workspace.releaseBarrier(); waitLifecycleIdle(t, svc) }()
			svc.WakeGoal(sessionID)
			select {
			case <-workspace.entered:
			case <-time.After(5 * time.Second):
				t.Fatal("reserved candidate never reached startup barrier")
			}
			mutation := domain.WorkMutation{
				SessionID: sessionID, ExpectedVersion: 1, RequestID: "change-goal", RequestHash: "change-goal",
				Kind: kind, Goal: ref, Reason: "user changed direction",
			}
			if kind == domain.WorkEventGoalEdited {
				mutation.Objective, mutation.MaxRounds = "revised objective", 2
			}
			if _, err := backend.CommitWork(ctx, mutation); err != nil {
				t.Fatalf("durable %s: %v", kind, err)
			}
			workspace.releaseBarrier()
			waitLifecycleIdle(t, svc)
			state, err := backend.ReadWork(ctx, sessionID)
			if err != nil || state.Goal == nil || state.Goal.RoundsStarted != 0 {
				t.Fatalf("work after reserved candidate invalidation = %+v / %v", state, err)
			}
			if kind == domain.WorkEventGoalPaused && state.Goal.Phase != domain.WorkPhasePaused {
				t.Fatalf("paused candidate phase = %s", state.Goal.Phase)
			}
			if kind == domain.WorkEventGoalEdited && (state.Goal.Ref.Revision != 2 || state.Goal.Objective != "revised objective") {
				t.Fatalf("edited candidate Goal = %+v", state.Goal)
			}
			if got := countWorkEvents(t, backend, sessionID, domain.WorkEventGoalRoundAdmitted); got != 0 {
				t.Fatalf("admitted rounds = %d, want zero", got)
			}
			runs, err := backend.ListRunsBySession(ctx, sessionID)
			if err != nil || len(runs) != 0 {
				t.Fatalf("runs after invalidation = %+v / %v", runs, err)
			}
			messages, err := backend.ListMessages(ctx, sessionID)
			if err != nil || len(messages) != 0 {
				t.Fatalf("messages after invalidation = %+v / %v", messages, err)
			}
		})
	}
}

func TestCancelGoalRunDoesNotImmediatelyReplaceIt(t *testing.T) {
	ctx := context.Background()
	backend := openLifecycleBackend(t)
	const sessionID domain.SessionID = "sess-goal-cancel"
	createLifecycleGoal(t, backend, sessionID, 2)
	goalRuns := &notifyingGoalRunStore{GoalRunStore: backend, committed: make(chan storage.GoalRunCommitResult, 1)}
	svc := newGoalLifecycleService(t, backend, &blockingEinoModel{}, nil, goalRuns, nil)
	t.Cleanup(func() { svc.CancelAll(); waitLifecycleIdle(t, svc) })
	svc.WakeGoal(sessionID)
	var admitted storage.GoalRunCommitResult
	select {
	case admitted = <-goalRuns.committed:
	case <-time.After(5 * time.Second):
		t.Fatal("Goal run was not admitted")
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		activation, current := svc.GoalActivation(sessionID)
		if activation == "armed" && current == admitted.Run.ID {
			break
		}
		time.Sleep(time.Millisecond)
	}
	if activation, current := svc.GoalActivation(sessionID); activation != "armed" || current != admitted.Run.ID {
		t.Fatalf("committed Goal run never became cancellable: activation=%q run=%q", activation, current)
	}
	if !svc.Cancel(admitted.Run.ID) {
		t.Fatal("cancel of admitted Goal run returned false")
	}
	waitForRunStatus(t, backend, admitted.Run.ID, domain.RunCancelled)
	waitLifecycleIdle(t, svc)
	state, err := backend.ReadWork(ctx, sessionID)
	if err != nil || state.Goal == nil || state.Goal.Phase != domain.WorkPhaseBlocked || state.Goal.RoundsStarted != 1 || state.Goal.Reason != "goal round cancelled" || state.Goal.EvidenceRunID != admitted.Run.ID {
		t.Fatalf("cancelled Goal state = %+v / %v", state, err)
	}
	runs, err := backend.ListRunsBySession(ctx, sessionID)
	if err != nil || len(runs) != 1 || runs[0].ID != admitted.Run.ID {
		t.Fatalf("runs after cancel = %+v / %v, want only cancelled run", runs, err)
	}
	messages, err := backend.ListMessages(ctx, sessionID)
	if err != nil || len(messages) != 1 || messages[0].RunID != admitted.Run.ID {
		t.Fatalf("messages after cancel = %+v / %v", messages, err)
	}
	if got := countWorkEvents(t, backend, sessionID, domain.WorkEventGoalRoundAdmitted); got != 1 {
		t.Fatalf("admitted rounds after cancel = %d", got)
	}
}

func TestGoalRoundCapPersistsExplicitBlock(t *testing.T) {
	ctx := context.Background()
	backend := openLifecycleBackend(t)
	const sessionID domain.SessionID = "sess-goal-cap"
	createLifecycleGoal(t, backend, sessionID, 1)
	goalRuns := &notifyingGoalRunStore{GoalRunStore: backend, committed: make(chan storage.GoalRunCommitResult, 1)}
	svc := newGoalLifecycleService(t, backend, NewScriptedModel(schema.AssistantMessage("one round finished", nil)), nil, goalRuns, nil)
	t.Cleanup(func() { svc.CancelAll(); waitLifecycleIdle(t, svc) })
	svc.WakeGoal(sessionID)
	var admitted storage.GoalRunCommitResult
	select {
	case admitted = <-goalRuns.committed:
	case <-time.After(5 * time.Second):
		t.Fatal("capped Goal round was not admitted")
	}
	waitForRunStatus(t, backend, admitted.Run.ID, domain.RunCompleted)
	waitLifecycleIdle(t, svc)
	state, err := backend.ReadWork(ctx, sessionID)
	if err != nil || state.Goal == nil || state.Goal.Phase != domain.WorkPhaseBlocked || state.Goal.RoundsStarted != 1 || state.Goal.Reason != "goal round limit reached" {
		t.Fatalf("capped Goal state = %+v / %v", state, err)
	}
	runs, err := backend.ListRunsBySession(ctx, sessionID)
	if err != nil || len(runs) != 1 || runs[0].Status != domain.RunCompleted || state.Goal.EvidenceRunID != runs[0].ID {
		t.Fatalf("capped Goal run = %+v / %v", runs, err)
	}
	if got := countWorkEvents(t, backend, sessionID, domain.WorkEventGoalBlocked); got != 1 {
		t.Fatalf("durable cap block events = %d", got)
	}
}

func TestOldRevisionGoalRunCannotReportIntoEditedGoal(t *testing.T) {
	ctx := context.Background()
	backend := openLifecycleBackend(t)
	const sessionID domain.SessionID = "sess-goal-old-report"
	ref := createLifecycleGoalWithSession(t, backend, domain.Session{
		ID: sessionID, Title: "Old report", CreatedAt: 1,
		SandboxMode: string(domain.SandboxModeWorkspaceWrite), ApprovalPolicy: string(domain.ApprovalPolicyAuto),
	}, 2)
	toolset, err := tools.NewRegistry(tools.NewReportGoal()).Resolve([]string{tools.ReportGoalName})
	if err != nil {
		t.Fatalf("resolve report_goal: %v", err)
	}
	model := &gatedLifecycleModel{
		inner: NewScriptedModel(
			schema.AssistantMessage("", []schema.ToolCall{{ID: "report-old-revision", Function: schema.FunctionCall{
				Name: tools.ReportGoalName, Arguments: `{"goal_id":"goal-lifecycle","revision":1,"status":"completed","reason":"old run done"}`,
			}}}),
			schema.AssistantMessage("Old report was rejected.", nil),
		),
		entered: make(chan struct{}), release: make(chan struct{}),
	}
	engine, err := NewEngine(ctx, model, toolset, EngineConfig{
		StreamBuffer: 8, MaxEventPayloadBytes: 64 << 10, AutoApproveTools: []string{tools.ReportGoalName},
	})
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}
	svc := NewService(engine, "scripted", "scripted-v0", ServiceDeps{
		Journal: backend, Runs: backend, Messages: backend, Sessions: backend,
		PrimaryRuns: backend, GoalRuns: backend, Work: backend, Sink: newTestSink(),
	})
	defer func() { model.unblock(); svc.CancelAll(); waitLifecycleIdle(t, svc) }()
	svc.WakeGoal(sessionID)
	select {
	case <-model.entered:
	case <-time.After(5 * time.Second):
		t.Fatal("admitted run did not enter model barrier")
	}
	runs, err := backend.ListRunsBySession(ctx, sessionID)
	if err != nil || len(runs) != 1 || runs[0].Status != domain.RunActive {
		t.Fatalf("admitted old-revision run = %+v / %v", runs, err)
	}
	if _, err := backend.CommitWork(ctx, domain.WorkMutation{
		SessionID: sessionID, ExpectedVersion: 2, RequestID: "edit-live-goal", RequestHash: "edit-live-goal",
		Kind: domain.WorkEventGoalEdited, Goal: ref, Objective: "revised live goal", MaxRounds: 2,
	}); err != nil {
		t.Fatalf("edit Goal while old run is live: %v", err)
	}
	model.unblock()
	waitForTerminalRun(t, backend, runs[0].ID)
	waitLifecycleIdle(t, svc)
	state, err := backend.ReadWork(ctx, sessionID)
	if err != nil || state.Goal == nil || state.Goal.Ref.Revision != 2 || state.Goal.Phase != domain.WorkPhaseActive || state.Goal.RoundsStarted != 1 {
		t.Fatalf("Goal after stale run report = %+v / %v", state, err)
	}
	if got := countWorkEvents(t, backend, sessionID, domain.WorkEventGoalCompleted); got != 0 {
		t.Fatalf("stale report completed revised Goal %d times", got)
	}
	if got := countWorkEvents(t, backend, sessionID, domain.WorkEventGoalBlocked); got != 0 {
		t.Fatalf("stale run settled revised Goal %d times", got)
	}
	staleReportRejected := false
	for _, event := range replayAll(t, backend, runs[0].ID) {
		if event.Type != domain.EventToolFinished {
			continue
		}
		var finished payloadToolFinished
		if err := json.Unmarshal(event.Payload, &finished); err != nil {
			t.Fatalf("decode tool.finished: %v", err)
		}
		if finished.ToolCallID == "report-old-revision" && strings.Contains(finished.Error+" "+finished.Result, "stale goal reference") {
			staleReportRejected = true
		}
	}
	if !staleReportRejected {
		t.Fatal("old-revision report_goal call did not produce a stale Goal rejection")
	}
	messages, err := backend.ListMessages(ctx, sessionID)
	if err != nil || len(messages) == 0 || messages[0].RunID != runs[0].ID {
		t.Fatalf("old run message provenance = %+v / %v", messages, err)
	}
}

func TestPendingGoalQuestionKeepsSessionBusy(t *testing.T) {
	ctx := context.Background()
	backend := openLifecycleBackend(t)
	const sessionID domain.SessionID = "sess-goal-question"
	createLifecycleGoal(t, backend, sessionID, 2)
	toolset, err := tools.Builtin(backend).Resolve([]string{tools.AskUserName})
	if err != nil {
		t.Fatalf("resolve ask_user: %v", err)
	}
	checkpoints, err := NewVersionedCheckpointStore(backend.Blobs(), "test-engine")
	if err != nil {
		t.Fatalf("checkpoint store: %v", err)
	}
	engine, err := NewEngine(ctx, NewQuestionFlowModel(), toolset, EngineConfig{
		StreamBuffer: 8, MaxEventPayloadBytes: 64 << 10, Checkpoints: checkpoints,
	})
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}
	goalRuns := &notifyingGoalRunStore{GoalRunStore: backend, committed: make(chan storage.GoalRunCommitResult, 1)}
	svc := NewService(engine, "scripted", "scripted-v0", ServiceDeps{
		Journal: backend, Runs: backend, Messages: backend, Sessions: backend,
		PrimaryRuns: backend, GoalRuns: goalRuns, Work: backend, Questions: backend,
		ApprovalExpiration: 5 * time.Minute, Sink: newTestSink(),
	})
	t.Cleanup(func() { svc.CancelAll(); waitLifecycleIdle(t, svc) })
	svc.WakeGoal(sessionID)
	var admitted storage.GoalRunCommitResult
	select {
	case admitted = <-goalRuns.committed:
	case <-time.After(5 * time.Second):
		t.Fatal("question Goal run was not admitted")
	}
	question := waitForPendingQuestion(t, backend, admitted.Run.ID)
	if question.Status != domain.QuestionPending {
		t.Fatalf("question status = %s", question.Status)
	}
	svc.WakeGoal(sessionID)
	waitLifecycleIdle(t, svc)
	runs, err := backend.ListRunsBySession(ctx, sessionID)
	if err != nil || len(runs) != 1 || runs[0].Status != domain.RunActive {
		t.Fatalf("runs while Goal question pending = %+v / %v", runs, err)
	}
	state, err := backend.ReadWork(ctx, sessionID)
	if err != nil || state.Goal == nil || state.Goal.RoundsStarted != 1 || state.Goal.Phase != domain.WorkPhaseActive {
		t.Fatalf("Goal while question pending = %+v / %v", state, err)
	}
	if !svc.Cancel(admitted.Run.ID) {
		t.Fatal("cancel pending Goal question returned false")
	}
	waitForRunStatus(t, backend, admitted.Run.ID, domain.RunCancelled)
	waitLifecycleIdle(t, svc)
	runs, err = backend.ListRunsBySession(ctx, sessionID)
	if err != nil || len(runs) != 1 {
		t.Fatalf("runs after pending question cancellation = %+v / %v", runs, err)
	}
}

func TestPendingGoalApprovalKeepsSessionBusy(t *testing.T) {
	ctx := context.Background()
	backend := openLifecycleBackend(t)
	const sessionID domain.SessionID = "sess-goal-approval"
	createLifecycleGoal(t, backend, sessionID, 2)
	toolset, err := tools.Builtin(backend).Resolve([]string{tools.WriteNoteName})
	if err != nil {
		t.Fatalf("resolve write_note: %v", err)
	}
	checkpoints, err := NewVersionedCheckpointStore(backend.Blobs(), "test-engine")
	if err != nil {
		t.Fatalf("checkpoint store: %v", err)
	}
	engine, err := NewEngine(ctx, NewApprovalFlowModel(), toolset, EngineConfig{
		StreamBuffer: 8, MaxEventPayloadBytes: 64 << 10, Checkpoints: checkpoints,
	})
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}
	goalRuns := &notifyingGoalRunStore{GoalRunStore: backend, committed: make(chan storage.GoalRunCommitResult, 1)}
	svc := NewService(engine, "scripted", "scripted-v0", ServiceDeps{
		Journal: backend, Runs: backend, Messages: backend, Sessions: backend,
		PrimaryRuns: backend, GoalRuns: goalRuns, Work: backend, Notes: backend, Approvals: backend,
		ApprovalExpiration: 5 * time.Minute, Sink: newTestSink(),
	})
	t.Cleanup(func() { svc.CancelAll(); waitLifecycleIdle(t, svc) })
	svc.WakeGoal(sessionID)
	var admitted storage.GoalRunCommitResult
	select {
	case admitted = <-goalRuns.committed:
	case <-time.After(5 * time.Second):
		t.Fatal("approval Goal run was not admitted")
	}
	approval := waitForPendingApproval(t, backend, admitted.Run.ID)
	if approval.RunID != admitted.Run.ID {
		t.Fatalf("pending approval run = %s, want %s", approval.RunID, admitted.Run.ID)
	}
	svc.WakeGoal(sessionID)
	waitLifecycleIdle(t, svc)
	runs, err := backend.ListRunsBySession(ctx, sessionID)
	if err != nil || len(runs) != 1 || runs[0].Status != domain.RunActive {
		t.Fatalf("runs while Goal approval pending = %+v / %v", runs, err)
	}
	state, err := backend.ReadWork(ctx, sessionID)
	if err != nil || state.Goal == nil || state.Goal.RoundsStarted != 1 || state.Goal.Phase != domain.WorkPhaseActive {
		t.Fatalf("Goal while approval pending = %+v / %v", state, err)
	}
	if !svc.Cancel(admitted.Run.ID) {
		t.Fatal("cancel pending Goal approval returned false")
	}
	waitForRunStatus(t, backend, admitted.Run.ID, domain.RunCancelled)
	waitLifecycleIdle(t, svc)
	runs, err = backend.ListRunsBySession(ctx, sessionID)
	if err != nil || len(runs) != 1 {
		t.Fatalf("runs after pending approval cancellation = %+v / %v", runs, err)
	}
}

type gatedLifecycleModel struct {
	inner   *ScriptedModel
	entered chan struct{}
	release chan struct{}
	once    sync.Once
	done    sync.Once
}

func (m *gatedLifecycleModel) wait(ctx context.Context) error {
	m.once.Do(func() { close(m.entered) })
	select {
	case <-m.release:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
func (m *gatedLifecycleModel) unblock() { m.done.Do(func() { close(m.release) }) }
func (m *gatedLifecycleModel) Generate(ctx context.Context, in []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	if err := m.wait(ctx); err != nil {
		return nil, err
	}
	return m.inner.Generate(ctx, in, opts...)
}
func (m *gatedLifecycleModel) Stream(ctx context.Context, in []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	if err := m.wait(ctx); err != nil {
		return nil, err
	}
	return m.inner.Stream(ctx, in, opts...)
}
func (m *gatedLifecycleModel) WithTools(_ []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	return m, nil
}

// This model never returns a model response; cancellation must terminate the
// already committed run without permitting a replacement round.
type blockingEinoModel struct{}

func (*blockingEinoModel) Generate(ctx context.Context, _ []*schema.Message, _ ...model.Option) (*schema.Message, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}
func (m *blockingEinoModel) Stream(ctx context.Context, in []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	_, err := m.Generate(ctx, in, opts...)
	return nil, err
}
func (m *blockingEinoModel) WithTools(_ []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	return m, nil
}

type commitThenLoseResultStore struct {
	storage.GoalRunStore
	committed chan struct{}
}

func (s *commitThenLoseResultStore) CommitGoalRun(ctx context.Context, admission storage.GoalRunCommit) (storage.GoalRunCommitResult, error) {
	if _, err := s.GoalRunStore.CommitGoalRun(ctx, admission); err != nil {
		return storage.GoalRunCommitResult{}, err
	}
	close(s.committed)
	return storage.GoalRunCommitResult{}, errors.New("simulated process loss after durable GoalRun commit")
}

func TestCrashAfterGoalRunCommitNeverRefundsOrReplaysRound(t *testing.T) {
	ctx := context.Background()
	backend := openLifecycleBackend(t)
	const sessionID domain.SessionID = "sess-goal-commit-crash"
	createLifecycleGoal(t, backend, sessionID, 1)
	store := &commitThenLoseResultStore{GoalRunStore: backend, committed: make(chan struct{})}
	svc := newGoalLifecycleService(t, backend, NewScriptedModel(schema.AssistantMessage("must not execute", nil)), nil, store, nil)
	svc.WakeGoal(sessionID)
	select {
	case <-store.committed:
	case <-time.After(5 * time.Second):
		t.Fatal("crash-window Goal run was not committed")
	}
	waitLifecycleIdle(t, svc)
	state, err := backend.ReadWork(ctx, sessionID)
	if err != nil || state.Goal == nil || state.Goal.RoundsStarted != 1 {
		t.Fatalf("work after committed crash window = %+v / %v", state, err)
	}
	runs, err := backend.ListRunsBySession(ctx, sessionID)
	if err != nil || len(runs) != 1 || runs[0].Status != domain.RunActive {
		t.Fatalf("committed crash-window run = %+v / %v", runs, err)
	}
	messages, err := backend.ListMessages(ctx, sessionID)
	if err != nil || len(messages) != 1 || messages[0].RunID != runs[0].ID {
		t.Fatalf("committed crash-window message = %+v / %v", messages, err)
	}
	restarted := newGoalLifecycleService(t, backend, NewScriptedModel(schema.AssistantMessage("must not replay", nil)), nil, backend, nil)
	if err := restarted.Recover(ctx); err != nil {
		t.Fatalf("recover committed Goal run: %v", err)
	}
	waitForRunStatus(t, backend, runs[0].ID, domain.RunFailed)
	restarted.WakeGoal(sessionID)
	waitLifecycleIdle(t, restarted)
	state, err = backend.ReadWork(ctx, sessionID)
	if err != nil || state.Goal == nil || state.Goal.RoundsStarted != 1 || state.Goal.Phase != domain.WorkPhaseBlocked || !strings.Contains(state.Goal.Reason, "failed") {
		t.Fatalf("recovered Goal state = %+v / %v", state, err)
	}
	afterRuns, err := backend.ListRunsBySession(ctx, sessionID)
	if err != nil || len(afterRuns) != 1 || afterRuns[0].ID != runs[0].ID {
		t.Fatalf("runs after recovery = %+v / %v, want original only", afterRuns, err)
	}
	if got := countWorkEvents(t, backend, sessionID, domain.WorkEventGoalRoundAdmitted); got != 1 {
		t.Fatalf("durable admissions after recovery = %d", got)
	}
}
