package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cloudwego/eino/schema"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage/sqlite"
	"agent-vivy/internal/tools"
)

func TestPlanGoalProbePlanGuidanceAfterEnterPlanMode(t *testing.T) {
	ctx := context.Background()
	backend, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "plan-goal-probe.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = backend.Close() })

	script := &recordingChatModel{inner: NewScriptedModel(
		schema.AssistantMessage("", []schema.ToolCall{{
			ID:       "enter-plan",
			Function: schema.FunctionCall{Name: tools.EnterPlanModeName, Arguments: `{}`},
		}}),
		schema.AssistantMessage("", []schema.ToolCall{{
			ID:       "echo-after-plan",
			Function: schema.FunctionCall{Name: tools.EchoInfoName, Arguments: `{"text":"planning"}`},
		}}),
		schema.AssistantMessage("I will outline a plan.", nil),
	)}
	toolset, err := tools.NewRegistry(tools.NewEnterPlanMode(), tools.NewEchoInfo()).Resolve([]string{
		tools.EnterPlanModeName, tools.EchoInfoName,
	})
	if err != nil {
		t.Fatalf("resolve tools: %v", err)
	}
	engine, err := NewEngine(ctx, script, toolset, EngineConfig{
		StreamBuffer: 8, MaxEventPayloadBytes: 64 << 10,
		AutoApproveTools: []string{tools.EnterPlanModeName},
	})
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}
	if err := backend.CreateSession(ctx, domain.Session{
		ID: "sess-plan-goal-probe", Title: "Plan probe", CreatedAt: 1,
		SandboxMode: string(domain.SandboxModeWorkspaceWrite), ApprovalPolicy: string(domain.ApprovalPolicyAuto),
	}); err != nil {
		t.Fatalf("create session: %v", err)
	}
	svc := NewService(engine, "scripted", "scripted-v0", ServiceDeps{
		Journal: backend, Runs: backend, Messages: backend, Sessions: backend,
		PrimaryRuns: backend, Work: backend, Sink: newTestSink(),
	})
	runID, err := svc.Run(ctx, "sess-plan-goal-probe", "Help me plan this change.")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	waitForRunStatus(t, backend, runID, domain.RunCompleted)

	script.mu.Lock()
	inputs := append([][]*schema.Message(nil), script.inputs...)
	script.mu.Unlock()
	if len(inputs) < 3 {
		t.Fatalf("model calls = %d, want two tool follow-up calls", len(inputs))
	}
	const planningGuidance = "Planning collaboration is active. Treat planning as advisory guidance only; do not claim human approval, widen permissions, or change the execution policy."
	for call := 1; call < 3; call++ {
		var nextInput strings.Builder
		for _, msg := range inputs[call] {
			nextInput.WriteString(msg.Content)
			for _, part := range msg.MultiContent {
				if part.Text != "" {
					nextInput.WriteString(part.Text)
				}
			}
		}
		if got := strings.Count(nextInput.String(), planningGuidance); got != 1 {
			t.Fatalf("model request %d contains planning guidance %d times, want exactly once; input: %s", call+1, got, nextInput.String())
		}
	}
}

type goalFenceEffectTool struct {
	calls atomic.Int64
}

func (t *goalFenceEffectTool) Spec() domain.ToolSpec {
	return domain.ToolSpec{Name: tools.WriteFileName, Description: "Count a test write.", Readonly: false}
}

func (t *goalFenceEffectTool) InvokableRun(context.Context, json.RawMessage) (string, error) {
	t.calls.Add(1)
	return "write completed", nil
}

func TestPlanGoalProbeReportFencesLaterToolInSameBatch(t *testing.T) {
	ctx := context.Background()
	backend, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "goal-fence-probe.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = backend.Close() })

	const sessions = 1
	script := make([]*schema.Message, 0, sessions*2)
	for i := 0; i < sessions; i++ {
		goalID := fmt.Sprintf("goal-%03d", i)
		script = append(script,
			schema.AssistantMessage("", []schema.ToolCall{
				{ID: fmt.Sprintf("report-%03d", i), Function: schema.FunctionCall{
					Name:      tools.ReportGoalName,
					Arguments: fmt.Sprintf(`{"goal_id":%q,"revision":1,"status":"completed","reason":"done"}`, goalID),
				}},
				{ID: fmt.Sprintf("write-%03d", i), Function: schema.FunctionCall{
					Name: tools.WriteFileName, Arguments: `{}`,
				}},
			}),
			schema.AssistantMessage("Goal work finished.", nil),
		)
	}
	effect := &goalFenceEffectTool{}
	toolset, err := tools.NewRegistry(tools.NewReportGoal(), effect).Resolve([]string{tools.ReportGoalName, tools.WriteFileName})
	if err != nil {
		t.Fatalf("resolve tools: %v", err)
	}
	engine, err := NewEngine(ctx, NewScriptedModel(script...), toolset, EngineConfig{
		StreamBuffer: 8, MaxEventPayloadBytes: 64 << 10,
		AutoApproveTools: []string{tools.ReportGoalName, tools.WriteFileName},
	})
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}
	svc := NewService(engine, "scripted", "scripted-v0", ServiceDeps{
		Journal: backend, Runs: backend, Messages: backend, Sessions: backend,
		PrimaryRuns: backend, GoalRuns: backend, Work: backend, Sink: newTestSink(),
	})

	for i := 0; i < sessions; i++ {
		sessionID := domain.SessionID(fmt.Sprintf("sess-goal-fence-%03d", i))
		goalID := fmt.Sprintf("goal-%03d", i)
		if err := backend.CreateSession(ctx, domain.Session{
			ID: sessionID, Title: "Goal fence probe", CreatedAt: 1,
			SandboxMode: string(domain.SandboxModeWorkspaceWrite), ApprovalPolicy: string(domain.ApprovalPolicyAuto),
		}); err != nil {
			t.Fatalf("create session %s: %v", sessionID, err)
		}
		if _, err := backend.CommitWork(ctx, domain.WorkMutation{
			SessionID: sessionID, ExpectedVersion: 0, RequestID: "seed-goal", RequestHash: "seed-goal",
			Kind: domain.WorkEventGoalCreated, Goal: domain.GoalRef{ID: goalID, Revision: 1},
			Objective: "finish a bounded test task", MaxRounds: 1,
		}); err != nil {
			t.Fatalf("create Goal %s: %v", goalID, err)
		}
		svc.WakeGoal(sessionID)
		runID := waitForProbeGoalRun(t, backend, sessionID)
		waitForRunStatus(t, backend, runID, domain.RunCompleted)
		state, err := backend.ReadWork(ctx, sessionID)
		if err != nil || state.Goal == nil || state.Goal.Phase != domain.WorkPhaseCompleted {
			t.Fatalf("Goal state for %s = %+v / %v, want completed", sessionID, state.Goal, err)
		}
	}
	if calls := effect.calls.Load(); calls != 0 {
		t.Fatalf("effectful sibling executed %d times after terminal Goal reports", calls)
	}
}

func TestPlanGoalProbeSubmissionFencesLaterToolInSameBatch(t *testing.T) {
	ctx := context.Background()
	backend, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "plan-submit-fence-probe.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = backend.Close() })

	const sessionID = domain.SessionID("sess-plan-submit-fence")
	if err := backend.CreateSession(ctx, domain.Session{
		ID: sessionID, Title: "Plan submission fence", CreatedAt: 1,
		SandboxMode: string(domain.SandboxModeWorkspaceWrite), ApprovalPolicy: string(domain.ApprovalPolicyAuto),
	}); err != nil {
		t.Fatalf("create session: %v", err)
	}
	if _, err := backend.CommitWork(ctx, domain.WorkMutation{
		SessionID: sessionID, ExpectedVersion: 0, RequestID: "enter-plan", RequestHash: "enter-plan",
		Kind: domain.WorkEventPlanEntered,
	}); err != nil {
		t.Fatalf("enter Plan: %v", err)
	}
	effect := &goalFenceEffectTool{}
	toolset, err := tools.NewRegistry(tools.NewSubmitPlan(), effect).Resolve([]string{tools.SubmitPlanName, tools.WriteFileName})
	if err != nil {
		t.Fatalf("resolve tools: %v", err)
	}
	checkpoints, err := NewVersionedCheckpointStore(backend.Blobs(), "plan-goal-probe")
	if err != nil {
		t.Fatalf("checkpoint store: %v", err)
	}
	script := &recordingChatModel{inner: NewScriptedModel(
		schema.AssistantMessage("", []schema.ToolCall{
			{ID: "submit-plan", Function: schema.FunctionCall{Name: tools.SubmitPlanName, Arguments: `{"markdown":"1. inspect\n2. implement"}`}},
			{ID: "write-after-submit", Function: schema.FunctionCall{Name: tools.WriteFileName, Arguments: `{}`}},
		}),
		schema.AssistantMessage("The plan is ready for review.", nil),
	)}
	engine, err := NewEngine(ctx, script, toolset, EngineConfig{
		StreamBuffer: 8, MaxEventPayloadBytes: 64 << 10, Checkpoints: checkpoints,
		AutoApproveTools: []string{tools.SubmitPlanName, tools.WriteFileName},
	})
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}
	svc := NewService(engine, "scripted", "scripted-v0", ServiceDeps{
		Journal: backend, Runs: backend, Messages: backend, Sessions: backend,
		PrimaryRuns: backend, Work: backend, Sink: newTestSink(),
	})
	runID, err := svc.Run(ctx, sessionID, "Prepare and review a plan.")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	deadline := time.Now().Add(3 * time.Second)
	var state domain.WorkState
	for time.Now().Before(deadline) {
		state, err = backend.ReadWork(ctx, sessionID)
		if err != nil {
			t.Fatalf("read work: %v", err)
		}
		run, runErr := backend.GetRun(ctx, runID)
		if runErr != nil {
			t.Fatalf("get run: %v", runErr)
		}
		if run.Status.Terminal() {
			t.Fatalf("run reached terminal status %s while Plan review is pending", run.Status)
		}
		if state.Plan.ReviewStatus == domain.PlanReviewPending && state.Plan.ResumeTarget != "" {
			settleUntil := time.Now().Add(150 * time.Millisecond)
			for time.Now().Before(settleUntil) {
				run, runErr = backend.GetRun(ctx, runID)
				if runErr != nil {
					t.Fatalf("get run while review is pending: %v", runErr)
				}
				if run.Status.Terminal() {
					t.Fatalf("run reached terminal status %s before a Plan decision", run.Status)
				}
				time.Sleep(5 * time.Millisecond)
			}
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if !state.Plan.Active || state.Plan.ReviewStatus != domain.PlanReviewPending || state.Plan.Markdown != "1. inspect\n2. implement" || state.Plan.SubmissionID == "" {
		t.Fatalf("Plan state = %+v, want exact pending submission while run waits", state.Plan)
	}
	if state.Plan.OriginRunID != runID || state.Plan.OriginToolCallID != "submit-plan" || state.Plan.ResumeTarget == "" {
		t.Fatalf("Plan resume anchor = %+v, want originating run, exact tool call and Eino target", state.Plan)
	}
	if len(state.Plan.BlockedToolCalls) != 1 || state.Plan.BlockedToolCalls[0] != "write-after-submit" {
		t.Fatalf("Plan blocked calls = %v, want the unreviewed sibling", state.Plan.BlockedToolCalls)
	}
	if calls := effect.calls.Load(); calls != 0 {
		t.Fatalf("effectful sibling executed %d times after Plan submission", calls)
	}
	decision := domain.WorkMutation{
		SessionID: sessionID, ExpectedVersion: state.Version,
		RequestID: "decide-plan-once", RequestHash: "decide-plan-once",
		Kind: domain.WorkEventPlanDecided, PlanSubmissionID: state.Plan.SubmissionID,
		PlanAction: domain.PlanDecisionRevise, PlanFeedback: "clarify the rollback step",
	}
	restarted := NewService(engine, "scripted", "scripted-v0", ServiceDeps{
		Journal: backend, Runs: backend, Messages: backend, Sessions: backend,
		PrimaryRuns: backend, Work: backend, Sink: newTestSink(),
	})
	if err := restarted.RecoverBackground(ctx); err != nil {
		t.Fatalf("recover Plan review after restart: %v", err)
	}
	committed, err := restarted.DecidePlan(ctx, decision)
	if err != nil {
		t.Fatalf("decide Plan: %v", err)
	}
	if committed.Replayed || committed.State.Plan.DecisionAction != domain.PlanDecisionRevise {
		t.Fatalf("first Plan decision = %+v, want a newly committed revision request", committed)
	}
	replayed, err := restarted.DecidePlan(ctx, decision)
	if err != nil {
		t.Fatalf("replay Plan decision: %v", err)
	}
	if !replayed.Replayed {
		t.Fatalf("replayed Plan decision = %+v, want original result without a second resume", replayed)
	}
	waitForRunStatus(t, backend, runID, domain.RunCompleted)
	script.mu.Lock()
	modelCalls := len(script.inputs)
	script.mu.Unlock()
	if modelCalls != 2 {
		t.Fatalf("model calls = %d, want one initial call and one exact resume", modelCalls)
	}
	finishedPlanCalls := 0
	for _, event := range replayAll(t, backend, runID) {
		if event.Type != domain.EventToolFinished {
			continue
		}
		var payload payloadToolFinished
		if err := json.Unmarshal(event.Payload, &payload); err != nil {
			t.Fatalf("decode tool.finished: %v", err)
		}
		if payload.ToolCallID == "submit-plan" {
			finishedPlanCalls++
		}
	}
	if finishedPlanCalls != 1 {
		t.Fatalf("submit_plan result count = %d, want exact call resumed once", finishedPlanCalls)
	}
	if calls := effect.calls.Load(); calls != 0 {
		t.Fatalf("unreviewed sibling executed %d times across Plan resume", calls)
	}
}

func waitForProbeGoalRun(t *testing.T, backend *sqlite.Backend, sessionID domain.SessionID) domain.RunID {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		runs, err := backend.ListRunsBySession(context.Background(), sessionID)
		if err != nil {
			t.Fatalf("list runs for %s: %v", sessionID, err)
		}
		if len(runs) > 0 {
			return runs[0].ID
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("Goal run for %s was not admitted", sessionID)
	return ""
}
