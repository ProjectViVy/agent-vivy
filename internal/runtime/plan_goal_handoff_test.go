package runtime

import (
	"context"
	"errors"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
	"agent-vivy/internal/storage/sqlite"
	"agent-vivy/internal/tools"
)

type heldPlanResumeModel struct {
	inner         model.ToolCallingChatModel
	calls         atomic.Int32
	resumeEntered chan struct{}
	release       chan struct{}
}

type heldCancellationEinoModel struct {
	entered   chan struct{}
	cancelled chan struct{}
	release   chan struct{}
	once      sync.Once
}

func (m *heldCancellationEinoModel) Generate(ctx context.Context, _ []*schema.Message, _ ...model.Option) (*schema.Message, error) {
	m.once.Do(func() {
		close(m.entered)
		context.AfterFunc(ctx, func() { close(m.cancelled) })
	})
	<-m.release
	return nil, ctx.Err()
}

func (m *heldCancellationEinoModel) Stream(ctx context.Context, in []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	_, err := m.Generate(ctx, in, opts...)
	return nil, err
}

func (m *heldCancellationEinoModel) WithTools(_ []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	return m, nil
}

func TestResumeBeforePlanCancellationLeavesOwnedRunAlive(t *testing.T) {
	ctx := context.Background()
	backend := openLifecycleBackend(t)
	const sessionID domain.SessionID = "sess-resume-before-plan-cancel"
	ref := createLifecycleGoal(t, backend, sessionID, 2)
	model := &gatedLifecycleModel{
		inner:   NewScriptedModel(schema.AssistantMessage("held Goal run", nil)),
		entered: make(chan struct{}), release: make(chan struct{}),
	}
	svc := newGoalLifecycleService(t, backend, model, nil, backend, nil)
	defer func() { model.unblock(); svc.CancelAll(); waitLifecycleIdle(t, svc) }()
	svc.WakeGoal(sessionID)
	select {
	case <-model.entered:
	case <-time.After(5 * time.Second):
		t.Fatal("Goal run did not start")
	}
	_, runID := svc.GoalActivation(sessionID)
	if runID == "" {
		t.Fatal("Goal run has no ID")
	}
	state, err := backend.ReadWork(ctx, sessionID)
	if err != nil || state.Version != 2 {
		t.Fatalf("initial Work = %+v / %v", state, err)
	}
	mutation := domain.WorkMutation{
		SessionID: sessionID, ExpectedVersion: state.Version,
		RequestID: "enter-plan", RequestHash: "enter-plan", Kind: domain.WorkEventPlanEntered,
	}
	transition, _, err := svc.pauseGoalForPlan(ctx, mutation)
	if err != nil || transition == nil {
		t.Fatalf("durable Plan pause = %+v / %v", transition, err)
	}
	paused, err := backend.ReadWork(ctx, sessionID)
	if err != nil || paused.Version != 3 || paused.Goal == nil || paused.Goal.Phase != domain.WorkPhasePaused {
		t.Fatalf("durable Goal pause = %+v / %v", paused, err)
	}
	resumed, resumeErr := svc.ResumeGoal(ctx, domain.WorkMutation{
		SessionID: sessionID, ExpectedVersion: 3,
		RequestID: "human-resume", RequestHash: "human-resume",
		Kind: domain.WorkEventGoalResumed, Goal: ref,
	})
	if resumeErr != nil || resumed.State.Goal == nil || resumed.State.Goal.Phase != domain.WorkPhaseActive {
		t.Fatalf("human resume = %+v / %v", resumed, resumeErr)
	}
	result, err := svc.finishPlanTransition(ctx, mutation, *transition)
	if !errors.Is(err, storage.ErrWorkVersionConflict) || result.Event.Kind != "" {
		t.Fatalf("stale Plan = %+v / %v", result, err)
	}
	run, err := backend.GetRun(ctx, runID)
	if err != nil || run.Status != domain.RunActive {
		t.Fatalf("resumed Goal run was cancelled: %+v / %v", run, err)
	}
	state, err = backend.ReadWork(ctx, sessionID)
	if err != nil || state.Version != 4 || state.Plan.Active || state.Goal == nil || state.Goal.Phase != domain.WorkPhaseActive {
		t.Fatalf("Work after resume = %+v / %v", state, err)
	}
	events, _, err := backend.ReplayWork(ctx, sessionID, domain.WorkState{SessionID: sessionID}, 10)
	if err != nil || len(events) != 4 || events[2].Kind != domain.WorkEventGoalPaused || events[3].Kind != domain.WorkEventGoalResumed {
		t.Fatalf("Journal after resume = %+v / %v", events, err)
	}
}

func TestResumeCannotCommitAfterPlanClaimsCancellation(t *testing.T) {
	ctx := context.Background()
	backend := openLifecycleBackend(t)
	const sessionID domain.SessionID = "sess-resume-during-plan-drain"
	ref := createLifecycleGoal(t, backend, sessionID, 2)
	model := &heldCancellationEinoModel{entered: make(chan struct{}), cancelled: make(chan struct{}), release: make(chan struct{})}
	svc := newGoalLifecycleService(t, backend, model, nil, backend, nil)
	var releaseOnce sync.Once
	release := func() { releaseOnce.Do(func() { close(model.release) }) }
	defer func() { release(); svc.CancelAll(); waitLifecycleIdle(t, svc) }()
	svc.WakeGoal(sessionID)
	select {
	case <-model.entered:
	case <-time.After(5 * time.Second):
		t.Fatal("Goal run did not start")
	}
	state, err := backend.ReadWork(ctx, sessionID)
	if err != nil || state.Version != 2 {
		t.Fatalf("initial Work = %+v / %v", state, err)
	}
	type planOutcome struct {
		result storage.WorkCommitResult
		err    error
	}
	planDone := make(chan planOutcome, 1)
	go func() {
		result, err := svc.EnterPlan(ctx, domain.WorkMutation{
			SessionID: sessionID, ExpectedVersion: state.Version,
			RequestID: "enter-plan", RequestHash: "enter-plan", Kind: domain.WorkEventPlanEntered,
		})
		planDone <- planOutcome{result: result, err: err}
	}()
	select {
	case <-model.cancelled:
	case <-time.After(5 * time.Second):
		t.Fatal("Plan did not signal cancellation")
	}
	resumeCtx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	resume, err := svc.ResumeGoal(resumeCtx, domain.WorkMutation{
		SessionID: sessionID, ExpectedVersion: 3,
		RequestID: "resume-during-drain", RequestHash: "resume-during-drain",
		Kind: domain.WorkEventGoalResumed, Goal: ref,
	})
	if !errors.Is(err, storage.ErrWorkVersionConflict) || resume.Event.Kind != "" {
		t.Fatalf("resume during Plan drain = %+v / %v", resume, err)
	}
	state, err = backend.ReadWork(ctx, sessionID)
	if err != nil || state.Version != 3 || state.Goal == nil || state.Goal.Phase != domain.WorkPhasePaused || state.Plan.Active {
		t.Fatalf("Work during drain = %+v / %v", state, err)
	}
	release()
	select {
	case outcome := <-planDone:
		if outcome.err != nil || outcome.result.State.Version != 4 || !outcome.result.State.Plan.Active {
			t.Fatalf("Plan after drain = %+v / %v", outcome.result, outcome.err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Plan transition did not settle")
	}
	events, _, err := backend.ReplayWork(ctx, sessionID, domain.WorkState{SessionID: sessionID}, 10)
	if err != nil || len(events) != 4 || events[2].Kind != domain.WorkEventGoalPaused || events[3].Kind != domain.WorkEventPlanEntered {
		t.Fatalf("Journal after Plan drain = %+v / %v", events, err)
	}
}

func (m *heldPlanResumeModel) Stream(ctx context.Context, messages []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	if m.calls.Add(1) == 2 {
		close(m.resumeEntered)
		select {
		case <-m.release:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	return m.inner.Stream(ctx, messages, opts...)
}

func (m *heldPlanResumeModel) Generate(ctx context.Context, messages []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	return m.inner.Generate(ctx, messages, opts...)
}

func (m *heldPlanResumeModel) WithTools(infos []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	inner, err := m.inner.WithTools(infos)
	if err != nil {
		return nil, err
	}
	m.inner = inner
	return m, nil
}

func TestPlanDecisionHandoffUsesOriginTerminalBeforeGoalAdmission(t *testing.T) {
	for _, tc := range []struct {
		name       string
		action     domain.PlanDecisionAction
		pausedGoal bool
	}{
		{name: "execute once", action: domain.PlanDecisionExecuteOnce},
		{name: "start Goal", action: domain.PlanDecisionStartGoal},
		{name: "paused Goal conflicts", action: domain.PlanDecisionStartGoal, pausedGoal: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			backend, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "handoff.db"))
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = backend.Close() })
			const sessionID domain.SessionID = "sess-review-handoff"
			if err := backend.CreateSession(ctx, domain.Session{
				ID: sessionID, CreatedAt: 1,
				SandboxMode: string(domain.SandboxModeWorkspaceWrite), ApprovalPolicy: string(domain.ApprovalPolicyAuto),
			}); err != nil {
				t.Fatal(err)
			}
			version := domain.WorkVersion(0)
			if tc.pausedGoal {
				created, err := backend.CommitWork(ctx, domain.WorkMutation{
					SessionID: sessionID, ExpectedVersion: version, RequestID: "old-goal", RequestHash: "old-goal",
					Kind: domain.WorkEventGoalCreated, Goal: domain.GoalRef{ID: "unfinished", Revision: 1},
					Objective: "unfinished task", MaxRounds: 2,
				})
				if err != nil {
					t.Fatal(err)
				}
				version = created.State.Version
				paused, err := backend.CommitWork(ctx, domain.WorkMutation{
					SessionID: sessionID, ExpectedVersion: version, RequestID: "old-pause", RequestHash: "old-pause",
					Kind: domain.WorkEventGoalPaused, Goal: domain.GoalRef{ID: "unfinished", Revision: 1},
				})
				if err != nil {
					t.Fatal(err)
				}
				version = paused.State.Version
			}
			if _, err := backend.CommitWork(ctx, domain.WorkMutation{
				SessionID: sessionID, ExpectedVersion: version, RequestID: "enter", RequestHash: "enter", Kind: domain.WorkEventPlanEntered,
			}); err != nil {
				t.Fatal(err)
			}
			model := &heldPlanResumeModel{
				inner: NewScriptedModel(
					schema.AssistantMessage("", []schema.ToolCall{{ID: "submit", Function: schema.FunctionCall{Name: tools.SubmitPlanName, Arguments: `{"markdown":"1. inspect\n2. implement"}`}}}),
					schema.AssistantMessage("I will proceed.", nil),
					schema.AssistantMessage("Goal round done.", nil),
				),
				resumeEntered: make(chan struct{}), release: make(chan struct{}),
			}
			var releaseOnce sync.Once
			release := func() { releaseOnce.Do(func() { close(model.release) }) }
			toolset, err := tools.NewRegistry(tools.NewSubmitPlan()).Resolve([]string{tools.SubmitPlanName})
			if err != nil {
				t.Fatal(err)
			}
			checkpoints, err := NewVersionedCheckpointStore(backend.Blobs(), "handoff")
			if err != nil {
				t.Fatal(err)
			}
			engine, err := NewEngine(ctx, model, toolset, EngineConfig{
				StreamBuffer: 8, MaxEventPayloadBytes: 64 << 10, Checkpoints: checkpoints,
				AutoApproveTools: []string{tools.SubmitPlanName},
			})
			if err != nil {
				t.Fatal(err)
			}
			svc := NewService(engine, "scripted", "scripted-v0", ServiceDeps{
				Journal: backend, Runs: backend, Messages: backend, Sessions: backend,
				PrimaryRuns: backend, GoalRuns: backend, Work: backend, Sink: newTestSink(),
			})
			defer func() {
				release()
				svc.StopAutomaticWork()
				svc.CancelAll()
				waitCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
				defer cancel()
				if !svc.WaitIdle(waitCtx) {
					t.Error("service did not drain")
				}
			}()
			origin, err := svc.Run(ctx, sessionID, "review a Plan")
			if err != nil {
				t.Fatal(err)
			}
			deadline := time.Now().Add(5 * time.Second)
			var state domain.WorkState
			for {
				state, err = backend.ReadWork(ctx, sessionID)
				if err != nil {
					t.Fatal(err)
				}
				if state.Plan.ReviewStatus == domain.PlanReviewPending && state.Plan.ResumeTarget != "" {
					break
				}
				if time.Now().After(deadline) {
					t.Fatalf("review did not suspend: %+v", state.Plan)
				}
				time.Sleep(5 * time.Millisecond)
			}
			decision := domain.WorkMutation{
				SessionID: sessionID, ExpectedVersion: state.Version, RequestID: "decide", RequestHash: "decide",
				Kind: domain.WorkEventPlanDecided, PlanSubmissionID: state.Plan.SubmissionID,
				PlanAction: tc.action,
			}
			if tc.action == domain.PlanDecisionStartGoal {
				decision.Goal = domain.GoalRef{ID: "new-goal", Revision: 1}
				decision.Objective, decision.MaxRounds = "complete reviewed plan", 1
			}
			result, err := svc.DecidePlan(ctx, decision)
			if tc.pausedGoal {
				if !errors.Is(err, domain.ErrStaleGoalReference) {
					t.Fatalf("start_goal with paused Goal = %+v / %v", result, err)
				}
				persisted, readErr := backend.ReadWork(ctx, sessionID)
				if readErr != nil || persisted.Goal == nil || persisted.Goal.Ref.ID != "unfinished" || persisted.Plan.ReviewStatus != domain.PlanReviewPending {
					t.Fatalf("unfinished Goal overwritten: %+v / %v", persisted, readErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("decide: %v", err)
			}
			replayed, err := svc.DecidePlan(ctx, decision)
			if err != nil || !replayed.Replayed || replayed.Event.Seq != result.Event.Seq {
				t.Fatalf("double decision: %+v / %v", replayed, err)
			}
			if tc.action == domain.PlanDecisionStartGoal {
				svc.WakeGoal(sessionID)
			}
			select {
			case <-model.resumeEntered:
			case <-time.After(5 * time.Second):
				t.Fatal("origin did not resume")
			}
			interim, err := backend.ReadWork(ctx, sessionID)
			if err != nil {
				t.Fatal(err)
			}
			if tc.action == domain.PlanDecisionExecuteOnce {
				if interim.Goal != nil {
					t.Fatalf("execute_once created Goal: %+v", interim.Goal)
				}
			} else if interim.Goal == nil || interim.Goal.Ref.ID != "new-goal" || interim.Goal.RoundsStarted != 0 {
				t.Fatalf("new Goal admitted before origin terminal: %+v", interim.Goal)
			}
			release()
			waitForRunStatus(t, backend, origin, domain.RunCompleted)
			if tc.action == domain.PlanDecisionStartGoal {
				deadline := time.Now().Add(5 * time.Second)
				for {
					state, err = backend.ReadWork(ctx, sessionID)
					if err != nil {
						t.Fatal(err)
					}
					if state.Goal != nil && state.Goal.RoundsStarted == 1 {
						break
					}
					if time.Now().After(deadline) {
						t.Fatalf("Goal did not start after origin cleanup: %+v", state.Goal)
					}
					time.Sleep(5 * time.Millisecond)
				}
			} else {
				state, err = backend.ReadWork(ctx, sessionID)
				if err != nil || state.Goal != nil {
					t.Fatalf("execute_once persisted Goal: %+v / %v", state.Goal, err)
				}
			}
			events, _, err := backend.ReplayWork(ctx, sessionID, domain.WorkState{SessionID: sessionID}, 20)
			if err != nil {
				t.Fatal(err)
			}
			decisions, admissions := 0, 0
			for _, event := range events {
				if event.Kind == domain.WorkEventPlanDecided {
					decisions++
				}
				if event.Kind == domain.WorkEventGoalRoundAdmitted {
					admissions++
				}
			}
			wantAdmissions := 0
			if tc.action == domain.PlanDecisionStartGoal {
				wantAdmissions = 1
			}
			if decisions != 1 || admissions != wantAdmissions {
				t.Fatalf("Journal decisions/admissions = %d/%d, want 1/%d", decisions, admissions, wantAdmissions)
			}
		})
	}
}
