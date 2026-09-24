package runtime

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cloudwego/eino/schema"

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
		SessionID: sessionID, ExpectedVersion: 0, RequestID: "seed-armed-goal", RequestHash: "seed-armed-goal",
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
		schema.AssistantMessage("", []schema.ToolCall{{ID: "enter-while-armed", Function: schema.FunctionCall{Name: tools.EnterPlanModeName, Arguments: `{}`}}}),
		schema.AssistantMessage("The Goal remains armed.", nil),
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
		SessionID: sessionID, ExpectedVersion: 0, RequestID: "seed-current-goal", RequestHash: "seed-current-goal",
		Kind: domain.WorkEventGoalCreated, Goal: domain.GoalRef{ID: "goal-current", Revision: 1},
		Objective: "owner's bounded work", MaxRounds: 2,
	}); err != nil {
		t.Fatal(err)
	}
	toolset, err := tools.NewRegistry(tools.NewReportGoal()).Resolve([]string{tools.ReportGoalName})
	if err != nil {
		t.Fatal(err)
	}
	engine, err := NewEngine(ctx, NewScriptedModel(schema.AssistantMessage("", []schema.ToolCall{{
		ID: "report-unadmitted", Function: schema.FunctionCall{Name: tools.ReportGoalName,
			Arguments: `{"goal_id":"goal-current","revision":1,"status":"completed","reason":"I claim this Goal"}`},
	}})), toolset, EngineConfig{StreamBuffer: 8, MaxEventPayloadBytes: 64 << 10,
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
	waitForRunStatus(t, backend, runID, domain.RunFailed)
	work, err := backend.ReadWork(ctx, sessionID)
	if err != nil || work.Goal == nil || work.Goal.Phase != domain.WorkPhaseActive || work.Version != 1 ||
		hasWorkKind(t, backend, sessionID, domain.WorkEventGoalCompleted) {
		t.Fatalf("unadmitted report changed Goal: %+v / %v", work, err)
	}
	var failed bool
	for _, event := range replayAll(t, backend, runID) {
		if event.Type == domain.EventRunFailed {
			failed = true
		}
		if event.Type == domain.EventToolFinished {
			t.Fatalf("unadmitted Goal report unexpectedly finished: %s", event.Payload)
		}
	}
	if !failed {
		t.Fatal("unadmitted run Journal has no terminal failure")
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
		RequestID: "seed-goal", RequestHash: "seed-goal", Kind: domain.WorkEventGoalCreated,
		Goal: ref, Objective: "bounded work", MaxRounds: 2}); err != nil {
		t.Fatal(err)
	}
	if _, err := backend.CommitWork(ctx, domain.WorkMutation{SessionID: sessionID, ExpectedVersion: 1,
		RequestID: "human-pause", RequestHash: "human-pause", Kind: domain.WorkEventGoalPaused,
		Goal: ref, Reason: "human paused"}); err != nil {
		t.Fatal(err)
	}
	toolset, err := tools.NewRegistry(tools.NewGetGoal()).Resolve([]string{tools.GetGoalName})
	if err != nil {
		t.Fatal(err)
	}
	engine, err := NewEngine(ctx, NewScriptedModel(schema.AssistantMessage("", []schema.ToolCall{{
		ID: "forged-resume", Function: schema.FunctionCall{Name: "resume_goal", Arguments: `{"goal_id":"goal-human-paused","revision":1}`},
	}})), toolset, EngineConfig{StreamBuffer: 8, MaxEventPayloadBytes: 64 << 10})
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
	id1, hash1, err := modelWorkIdentity("run-1", "submit-plan", "call-1", map[string]string{"markdown": "# plan"})
	if err != nil {
		t.Fatalf("modelWorkIdentity() error = %v", err)
	}
	id2, hash2, err := modelWorkIdentity("run-1", "submit-plan", "call-1", map[string]string{"markdown": "# plan"})
	if err != nil {
		t.Fatalf("modelWorkIdentity() repeat error = %v", err)
	}
	if id1 != id2 || hash1 != hash2 {
		t.Fatalf("identity changed across retries: (%q, %q) vs (%q, %q)", id1, hash1, id2, hash2)
	}
	id3, hash3, err := modelWorkIdentity("run-2", "submit-plan", "call-1", map[string]string{"markdown": "# plan"})
	if err != nil {
		t.Fatalf("modelWorkIdentity() second run error = %v", err)
	}
	if id1 == id3 || hash1 == hash3 {
		t.Fatal("different run identities must not share an idempotency key")
	}
}

func TestModelWorkIdentityDistinguishesCallsAndRejectsChangedArgs(t *testing.T) {
	payload := map[string]string{"objective": "ship the change"}
	requestID, requestHash, err := modelWorkIdentity("run-1", "create-goal", "call-1", payload)
	if err != nil {
		t.Fatalf("modelWorkIdentity() error = %v", err)
	}
	retryID, retryHash, err := modelWorkIdentity("run-1", "create-goal", "call-1", payload)
	if err != nil {
		t.Fatalf("modelWorkIdentity() retry error = %v", err)
	}
	if requestID != retryID || requestHash != retryHash {
		t.Fatalf("exact same-call retry changed identity: (%q, %q) vs (%q, %q)", requestID, requestHash, retryID, retryHash)
	}

	distinctID, distinctHash, err := modelWorkIdentity("run-1", "create-goal", "call-2", payload)
	if err != nil {
		t.Fatalf("modelWorkIdentity() distinct call error = %v", err)
	}
	if distinctID == requestID || distinctHash == requestHash {
		t.Fatal("distinct Eino tool-call IDs must identify distinct model work requests")
	}

	changedID, changedHash, err := modelWorkIdentity("run-1", "create-goal", "call-1", map[string]string{"objective": "different change"})
	if err != nil {
		t.Fatalf("modelWorkIdentity() changed arguments error = %v", err)
	}
	if changedID != requestID || changedHash == requestHash {
		t.Fatalf("same call ID with changed args must retain request ID but change payload hash: (%q, %q) vs (%q, %q)", requestID, requestHash, changedID, changedHash)
	}
}
