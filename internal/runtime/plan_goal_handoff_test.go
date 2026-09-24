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
	"agent-vivy/internal/storage/sqlite"
	"agent-vivy/internal/tools"
)

type heldPlanResumeModel struct {
	inner         model.ToolCallingChatModel
	calls         atomic.Int32
	resumeEntered chan struct{}
	release       chan struct{}
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
