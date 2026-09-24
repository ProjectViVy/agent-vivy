package runtime

import (
	"context"
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
