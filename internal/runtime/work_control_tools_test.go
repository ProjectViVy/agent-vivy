package runtime

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage/sqlite"
	"agent-vivy/internal/tools"
)

func TestModelEnterPlanWhileGoalArmedReportsGoalArmedWithoutPersistingPlan(t *testing.T) {
	ctx := context.Background()
	backend, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "armed-goal.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = backend.Close() })
	const sessionID = domain.SessionID("sess-armed-goal-plan-tool")
	if err := backend.CreateSession(ctx, domain.Session{ID: sessionID, Title: "Armed Goal", CreatedAt: 1,
		SandboxMode: string(domain.SandboxModeWorkspaceWrite), ApprovalPolicy: string(domain.ApprovalPolicyAuto)}); err != nil {
		t.Fatal(err)
	}
	if _, err := backend.CommitWork(ctx, domain.WorkMutation{
		SessionID: sessionID, ExpectedVersion: 0, RequestID: "seed-armed-goal", RequestInput: "seed-armed-goal",
		Kind: domain.WorkEventGoalCreated, Goal: domain.GoalRef{ID: "goal-armed", Revision: 1},
		Objective: "bounded objective", MaxRounds: 1,
	}); err != nil {
		t.Fatal(err)
	}
	toolset, err := tools.NewRegistry(tools.NewEnterPlanMode()).Resolve([]string{tools.EnterPlanModeName})
	if err != nil {
		t.Fatal(err)
	}
	model := NewScriptedModel(
		testToolCalls(testCall("enter-while-armed", tools.EnterPlanModeName, `{}`)),
		testAssistant("The Goal remains armed."),
	)
	engine, err := NewEngine(ctx, model, toolset, EngineConfig{StreamBuffer: 8, MaxEventPayloadBytes: 64 << 10,
		AutoApproveTools: []string{tools.EnterPlanModeName}})
	if err != nil {
		t.Fatal(err)
	}
	svc := NewService(engine, "scripted", "scripted-v0", ServiceDeps{
		Journal: backend, Work: backend, Runs: backend, Messages: backend, Sessions: backend,
		PrimaryRuns: backend, Sink: newTestSink(), PolicyDefaultProfile: domain.PolicyProfileFullAuto,
	})
	runID, err := svc.Run(ctx, sessionID, "Make a plan.")
	if err != nil {
		t.Fatal(err)
	}
	waitForRunStatus(t, backend, runID, domain.RunCompleted)
	state, err := backend.ReadWork(ctx, sessionID)
	if err != nil {
		t.Fatal(err)
	}
	if state.Goal == nil || state.Goal.Phase != domain.WorkPhaseActive || state.Plan.Active || state.Version != 1 {
		t.Fatalf("persisted work changed despite armed Goal: %+v", state)
	}
	var sawGoalArmed bool
	var toolEvents []string
	for _, event := range replayAll(t, backend, runID) {
		if event.Type == domain.EventToolFinished {
			toolEvents = append(toolEvents, string(event.Payload))
		}
		if event.Type == domain.EventToolFinished && strings.Contains(string(event.Payload), "goal_armed") {
			sawGoalArmed = true
		}
	}
	if !sawGoalArmed {
		t.Fatalf("model did not receive the goal_armed refusal: tool events = %v", toolEvents)
	}
}

func TestChildRunCannotEnterPlanThroughWorkTool(t *testing.T) {
	ctx := context.Background()
	backend, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "child-work-tool.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = backend.Close() })
	const sessionID = domain.SessionID("sess-child-work-tool")
	const childID = domain.RunID("run-child-work-tool")
	if err := backend.CreateSession(ctx, domain.Session{ID: sessionID, Title: "Child work tool", CreatedAt: 1}); err != nil {
		t.Fatal(err)
	}
	svc := NewService(nil, "scripted", "scripted-v0", ServiceDeps{Journal: backend, Work: backend, Runs: backend, Sessions: backend})
	if err := svc.CreateWorkerRun(ctx, domain.Run{ID: childID, SessionID: sessionID, Kind: domain.RunKindChild, Status: domain.RunActive, CreatedAt: 1}); err != nil {
		t.Fatal(err)
	}
	// Even an erroneously registered live child must fail the durable run-kind check.
	svc.runSessions[childID] = sessionID
	toolCtx := tools.WithWorkControl(tools.WithRunID(tools.WithSessionID(ctx, sessionID), childID), svc)
	if _, err := tools.NewEnterPlanMode().InvokableRun(toolCtx, json.RawMessage(`{}`)); err == nil || !strings.Contains(err.Error(), "not the session primary run") {
		t.Fatalf("child enter_plan_mode = %v, want primary-run refusal", err)
	}
	work, err := backend.ReadWork(ctx, sessionID)
	if err != nil || work.Version != 0 || work.Plan.Active || work.Goal != nil || hasWorkKind(t, backend, sessionID, domain.WorkEventPlanEntered) {
		t.Fatalf("child changed durable work: %+v / %v", work, err)
	}
	run, err := backend.GetRun(ctx, childID)
	if err != nil || run.Kind != domain.RunKindChild || run.Status != domain.RunActive {
		t.Fatalf("child run provenance = %+v / %v", run, err)
	}
	if events := replayAll(t, backend, childID); len(events) != 0 {
		t.Fatalf("rejected child tool wrote Journal events: %+v", events)
	}
}

// An unadmitted report is a per-call refusal: the Goal is untouched, the
// model sees that it did not run, and the run still completes.
func TestUnadmittedRunCannotReportCurrentGoalFromSameSession(t *testing.T) {
	ctx := context.Background()
	backend, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "unadmitted-goal-report.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = backend.Close() })
	const sessionID = domain.SessionID("sess-unadmitted-report")
	if err := backend.CreateSession(ctx, domain.Session{ID: sessionID, Title: "Unadmitted report", CreatedAt: 1,
		SandboxMode: string(domain.SandboxModeWorkspaceWrite), ApprovalPolicy: string(domain.ApprovalPolicyAuto)}); err != nil {
		t.Fatal(err)
	}
	if _, err := backend.CommitWork(ctx, domain.WorkMutation{
		SessionID: sessionID, ExpectedVersion: 0, RequestID: "seed-current-goal", RequestInput: "seed-current-goal",
		Kind: domain.WorkEventGoalCreated, Goal: domain.GoalRef{ID: "goal-current", Revision: 1},
		Objective: "owner's bounded work", MaxRounds: 10,
	}); err != nil {
		t.Fatal(err)
	}
	toolset, err := tools.NewRegistry(tools.NewReportGoal()).Resolve([]string{tools.ReportGoalName})
	if err != nil {
		t.Fatal(err)
	}
	engine, err := NewEngine(ctx, NewScriptedModel(
		testToolCalls(testCall("report-unadmitted", tools.ReportGoalName, `{"goal_id":"goal-current","revision":1,"status":"completed","reason":"I claim this Goal"}`)),
		testAssistant("Cannot report from this run."),
		testAssistant("Leaving the Goal to its owner."),
		testAssistant("Nothing further to report."),
		testAssistant("Done."),
	), toolset, EngineConfig{StreamBuffer: 8, MaxEventPayloadBytes: 64 << 10,
		AutoApproveTools: []string{tools.ReportGoalName}})
	if err != nil {
		t.Fatal(err)
	}
	svc := NewService(engine, "scripted", "scripted-v0", ServiceDeps{
		Journal: backend, Work: backend, Runs: backend, Messages: backend, Sessions: backend,
		PrimaryRuns: backend, GoalRuns: backend, Sink: newTestSink(), PolicyDefaultProfile: domain.PolicyProfileFullAuto,
	})
	runID, err := svc.Run(ctx, sessionID, "Report the current Goal without admission.")
	if err != nil {
		t.Fatal(err)
	}
	waitForRunStatus(t, backend, runID, domain.RunCompleted)
	// The Goal stays with its owner: no completion or block was recorded. The
	// version may advance from ordinary round accounting while the Goal lives.
	work, err := backend.ReadWork(ctx, sessionID)
	if err != nil || work.Goal == nil || work.Goal.Phase != domain.WorkPhaseActive ||
		hasWorkKind(t, backend, sessionID, domain.WorkEventGoalCompleted) ||
		hasWorkKind(t, backend, sessionID, domain.WorkEventGoalBlocked) {
		t.Fatalf("unadmitted report changed Goal: %+v / %v", work, err)
	}
	var refusal payloadToolFinished
	var refusals int
	for _, event := range replayAll(t, backend, runID) {
		if event.Type == domain.EventRunFailed {
			t.Fatalf("unadmitted refusal failed the run: %s", event.Payload)
		}
		if event.Type != domain.EventToolFinished {
			continue
		}
		if err := json.Unmarshal(event.Payload, &refusal); err != nil {
			t.Fatalf("decode tool.finished: %v", err)
		}
		refusals++
	}
	if refusals != 1 {
		t.Fatalf("tool.finished rows = %d, want the single refused report", refusals)
	}
	if refusal.Outcome != toolFailureStatusRefused || refusal.Reason != toolFailureReasonPolicyDenied ||
		refusal.Effects != toolEffectsNotExecuted {
		t.Fatalf("unadmitted report row = %s/%s/%s, want refused/policy_denied/not_executed",
			refusal.Outcome, refusal.Reason, refusal.Effects)
	}
	if !strings.Contains(refusal.Result, "did not run") || !strings.Contains(refusal.Result, "admitted Goal run") {
		t.Fatalf("unadmitted report result = %q", refusal.Result)
	}
}

func TestModelResumeGoalToolIsUnavailableAfterHumanPause(t *testing.T) {
	ctx := context.Background()
	backend, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "model-resume-goal.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = backend.Close() })
	const sessionID = domain.SessionID("sess-model-resume-goal")
	if err := backend.CreateSession(ctx, domain.Session{ID: sessionID, Title: "Paused Goal", CreatedAt: 1,
		SandboxMode: string(domain.SandboxModeWorkspaceWrite), ApprovalPolicy: string(domain.ApprovalPolicyAuto)}); err != nil {
		t.Fatal(err)
	}
	ref := domain.GoalRef{ID: "goal-human-paused", Revision: 1}
	if _, err := backend.CommitWork(ctx, domain.WorkMutation{SessionID: sessionID, ExpectedVersion: 0,
		RequestID: "seed-goal", RequestInput: "seed-goal", Kind: domain.WorkEventGoalCreated,
		Goal: ref, Objective: "bounded work", MaxRounds: 2}); err != nil {
		t.Fatal(err)
	}
	if _, err := backend.CommitWork(ctx, domain.WorkMutation{SessionID: sessionID, ExpectedVersion: 1,
		RequestID: "human-pause", RequestInput: "human-pause", Kind: domain.WorkEventGoalPaused,
		Goal: ref, Reason: "human paused"}); err != nil {
		t.Fatal(err)
	}
	toolset, err := tools.NewRegistry(tools.NewGetGoal()).Resolve([]string{tools.GetGoalName})
	if err != nil {
		t.Fatal(err)
	}
	engine, err := NewEngine(ctx, NewScriptedModel(testToolCalls(testCall("forged-resume", "resume_goal", `{"goal_id":"goal-human-paused","revision":1}`))), toolset, EngineConfig{StreamBuffer: 8, MaxEventPayloadBytes: 64 << 10})
	if err != nil {
		t.Fatal(err)
	}
	svc := NewService(engine, "scripted", "scripted-v0", ServiceDeps{
		Journal: backend, Work: backend, Runs: backend, Messages: backend, Sessions: backend,
		PrimaryRuns: backend, Sink: newTestSink(), PolicyDefaultProfile: domain.PolicyProfileFullAuto,
	})
	runID, err := svc.Run(ctx, sessionID, "Try to resume my paused Goal.")
	if err != nil {
		t.Fatal(err)
	}
	waitForRunStatus(t, backend, runID, domain.RunFailed)
	work, err := backend.ReadWork(ctx, sessionID)
	if err != nil || work.Goal == nil || work.Goal.Phase != domain.WorkPhasePaused || work.Version != 2 ||
		hasWorkKind(t, backend, sessionID, domain.WorkEventGoalResumed, domain.WorkEventGoalRoundAdmitted) {
		t.Fatalf("model resumed a human-paused Goal: %+v / %v", work, err)
	}
	var failed bool
	for _, event := range replayAll(t, backend, runID) {
		if event.Type == domain.EventRunFailed {
			failed = true
		}
		if event.Type == domain.EventToolFinished {
			t.Fatalf("model resume unexpectedly finished: %s", event.Payload)
		}
	}
	if !failed {
		t.Fatal("model resume attempt has no terminal Journal failure")
	}
}

func TestModelWorkIdentityIsDeterministicAndRunScoped(t *testing.T) {
	id1, input1, err := modelWorkIdentity("run-1", "submit-plan", "call-1", map[string]string{"markdown": "# plan"})
	if err != nil {
		t.Fatalf("modelWorkIdentity() error = %v", err)
	}
	id2, input2, err := modelWorkIdentity("run-1", "submit-plan", "call-1", map[string]string{"markdown": "# plan"})
	if err != nil {
		t.Fatalf("modelWorkIdentity() repeat error = %v", err)
	}
	if id1 != id2 || input1 != input2 {
		t.Fatalf("identity changed across retries: (%q, %q) vs (%q, %q)", id1, input1, id2, input2)
	}
	id3, input3, err := modelWorkIdentity("run-2", "submit-plan", "call-1", map[string]string{"markdown": "# plan"})
	if err != nil {
		t.Fatalf("modelWorkIdentity() second run error = %v", err)
	}
	if id1 == id3 || input1 == input3 {
		t.Fatal("different run identities must not share an idempotency key")
	}
}

func TestModelWorkIdentityDistinguishesCallsAndRejectsChangedArgs(t *testing.T) {
	payload := map[string]string{"objective": "ship the change"}
	requestID, requestInput, err := modelWorkIdentity("run-1", "create-goal", "call-1", payload)
	if err != nil {
		t.Fatalf("modelWorkIdentity() error = %v", err)
	}
	retryID, retryInput, err := modelWorkIdentity("run-1", "create-goal", "call-1", payload)
	if err != nil {
		t.Fatalf("modelWorkIdentity() retry error = %v", err)
	}
	if requestID != retryID || requestInput != retryInput {
		t.Fatalf("exact same-call retry changed identity: (%q, %q) vs (%q, %q)", requestID, requestInput, retryID, retryInput)
	}

	distinctID, distinctInput, err := modelWorkIdentity("run-1", "create-goal", "call-2", payload)
	if err != nil {
		t.Fatalf("modelWorkIdentity() distinct call error = %v", err)
	}
	if distinctID == requestID || distinctInput == requestInput {
		t.Fatal("distinct Eino tool-call IDs must identify distinct model work requests")
	}

	changedID, changedInput, err := modelWorkIdentity("run-1", "create-goal", "call-1", map[string]string{"objective": "different change"})
	if err != nil {
		t.Fatalf("modelWorkIdentity() changed arguments error = %v", err)
	}
	if changedID != requestID || changedInput == requestInput {
		t.Fatalf("same call ID with changed args must retain request ID but change intended input: (%q, %q) vs (%q, %q)", requestID, requestInput, changedID, changedInput)
	}
}
