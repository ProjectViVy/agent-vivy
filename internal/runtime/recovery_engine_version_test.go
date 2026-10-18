package runtime

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/cloudwego/eino/schema"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage/sqlite"
	"agent-vivy/internal/tools"
)

// A restart under a different Eino engine version (upgrade or rollback)
// cannot resume a paused run's checkpoint. Recovery closes the run with the
// upgrade message and settles its pending interaction, so no actionable card
// outlives it. Checkpoints are never converted across versions.

func assertEngineChangedFailure(t *testing.T, backend *sqlite.Backend, runID domain.RunID) {
	t.Helper()
	waitForRunStatus(t, backend, runID, domain.RunFailed)
	events := replayAll(t, backend, runID)
	if n := countTerminal(events); n != 1 {
		t.Fatalf("terminal events = %d, want 1", n)
	}
	last := events[len(events)-1]
	var p payloadRunFailed
	if last.Type != domain.EventRunFailed || json.Unmarshal(last.Payload, &p) != nil {
		t.Fatalf("last event = %s %s, want run.failed", last.Type, last.Payload)
	}
	if p.CauseCategory != causeInternalError || p.Message != engineChangedMessage {
		t.Fatalf("run.failed = %+v, want the engine-changed message", p)
	}
}

func TestRecoverAcrossEngineVersionCancelsPendingApproval(t *testing.T) {
	svc, backend, _ := newApprovalService(t, 5*time.Minute)
	ctx := context.Background()
	mustCreateSession(t, backend, "sess-upgrade")
	runID, err := svc.Run(ctx, "sess-upgrade", "note that I need milk")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	approval := waitForPendingApproval(t, backend, runID)
	waitForApprovalEvent(t, backend, runID)

	restarted, _ := restartServiceAt(t, backend, "test-engine-next")
	if err := restarted.Recover(ctx); err != nil {
		t.Fatalf("recover: %v", err)
	}
	assertEngineChangedFailure(t, backend, runID)
	got, err := backend.GetApproval(ctx, approval.ID)
	if err != nil || got.Decision != domain.ApprovalCancelled {
		t.Fatalf("approval after recovery = %q, err=%v; want cancelled", got.Decision, err)
	}
	events := replayAll(t, backend, runID)
	if i, j := indexOfType(events, domain.EventToolApprovalCancelled), indexOfType(events, domain.EventRunFailed); i < 0 || i > j {
		t.Fatalf("approval_cancelled at %d, run.failed at %d; want cancel before terminal", i, j)
	}

	// Idempotent: a second pass finds nothing to recover.
	if err := restarted.Recover(ctx); err != nil {
		t.Fatalf("second recover: %v", err)
	}
	if n := countTerminal(replayAll(t, backend, runID)); n != 1 {
		t.Fatalf("terminal events after second recovery = %d, want 1", n)
	}
}

func TestRecoverAcrossEngineVersionCancelsPendingQuestion(t *testing.T) {
	initial, backend, _ := newQuestionService(t)
	ctx := context.Background()
	mustCreateSession(t, backend, "sess-upgrade-q")
	runID, err := initial.Run(ctx, "sess-upgrade-q", "ask me for a color")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	question := waitForPendingQuestion(t, backend, runID)

	restarted := restartQuestionServiceAt(t, backend, "test-engine-next")
	if err := restarted.Recover(ctx); err != nil {
		t.Fatalf("recover: %v", err)
	}
	assertEngineChangedFailure(t, backend, runID)
	got, err := backend.GetQuestion(ctx, question.ID)
	if err != nil || got.Status != domain.QuestionCancelled {
		t.Fatalf("question after recovery = %q, err=%v; want cancelled", got.Status, err)
	}
}

// A same-version restart with a missing checkpoint keeps the generic restart
// message, but its pending approval is still settled.
func TestRecoverWithMissingCheckpointCancelsPendingApproval(t *testing.T) {
	svc, backend, _ := newApprovalService(t, 5*time.Minute)
	ctx := context.Background()
	mustCreateSession(t, backend, "sess-lost")
	runID, err := svc.Run(ctx, "sess-lost", "note that I need milk")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	approval := waitForPendingApproval(t, backend, runID)
	waitForApprovalEvent(t, backend, runID)
	if err := backend.Blobs().Delete(ctx, checkpointIDFor(runID)); err != nil {
		t.Fatalf("delete checkpoint: %v", err)
	}

	restarted, _ := restartService(t, backend)
	if err := restarted.Recover(ctx); err != nil {
		t.Fatalf("recover: %v", err)
	}
	waitForRunStatus(t, backend, runID, domain.RunFailed)
	lastRunFailed(t, backend, runID)
	if got, err := backend.GetApproval(ctx, approval.ID); err != nil || got.Decision != domain.ApprovalCancelled {
		t.Fatalf("approval after recovery = %q, err=%v; want cancelled", got.Decision, err)
	}
}

func newPendingPlanRecovery(t *testing.T) (*sqlite.Backend, domain.RunID, domain.SessionID, func() *Service) {
	ctx := context.Background()
	backend, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "plan-upgrade.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = backend.Close() })
	const sessionID = domain.SessionID("sess-plan-upgrade")
	if err := backend.CreateSession(ctx, domain.Session{
		ID: sessionID, Title: "Plan upgrade", CreatedAt: 1,
		SandboxMode: string(domain.SandboxModeWorkspaceWrite), ApprovalPolicy: string(domain.ApprovalPolicyAuto),
	}); err != nil {
		t.Fatalf("create session: %v", err)
	}
	if _, err := backend.CommitWork(ctx, domain.WorkMutation{
		SessionID: sessionID, RequestID: "enter-plan", RequestHash: "enter-plan", Kind: domain.WorkEventPlanEntered,
	}); err != nil {
		t.Fatalf("enter Plan: %v", err)
	}
	planService := func(engineVersion string, model *ScriptedModel) *Service {
		toolset, err := tools.NewRegistry(tools.NewSubmitPlan()).Resolve([]string{tools.SubmitPlanName})
		if err != nil {
			t.Fatalf("resolve tools: %v", err)
		}
		checkpoints, err := NewVersionedCheckpointStore(backend.Blobs(), engineVersion)
		if err != nil {
			t.Fatalf("checkpoint store: %v", err)
		}
		eng, err := NewEngine(ctx, model, toolset, EngineConfig{
			StreamBuffer: 8, MaxEventPayloadBytes: 64 << 10, Checkpoints: checkpoints,
			AutoApproveTools: []string{tools.SubmitPlanName},
		})
		if err != nil {
			t.Fatalf("new engine: %v", err)
		}
		return NewService(eng, "scripted", "scripted-v0", ServiceDeps{
			Journal: backend, Runs: backend, Messages: backend, Sessions: backend,
			PrimaryRuns: backend, Work: backend, Sink: newTestSink(),
		})
	}
	initial := planService("test-engine", NewScriptedModel(
		schema.AssistantMessage("", []schema.ToolCall{
			{ID: "submit-plan", Function: schema.FunctionCall{Name: tools.SubmitPlanName, Arguments: `{"markdown":"1. inspect"}`}},
		}),
		schema.AssistantMessage("The plan is ready for review.", nil),
	))
	runID, err := initial.Run(ctx, sessionID, "Prepare a plan.")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		state, err := backend.ReadWork(ctx, sessionID)
		if err != nil {
			t.Fatalf("read work: %v", err)
		}
		if state.Plan.ReviewStatus == domain.PlanReviewPending && state.Plan.ResumeTarget != "" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("Plan review never became pending: %+v", state.Plan)
		}
		time.Sleep(10 * time.Millisecond)
	}

	return backend, runID, sessionID, func() *Service {
		return planService("test-engine-next", NewScriptedModel(schema.AssistantMessage("unused", nil)))
	}
}

func TestRecoverAcrossEngineVersionCancelsPendingPlanReview(t *testing.T) {
	ctx := context.Background()
	backend, runID, sessionID, restart := newPendingPlanRecovery(t)
	restarted := restart()
	if err := restarted.Recover(ctx); err != nil {
		t.Fatalf("recover: %v", err)
	}
	assertEngineChangedFailure(t, backend, runID)
	state, err := backend.ReadWork(ctx, sessionID)
	if err != nil || state.Plan.ReviewStatus != domain.PlanReviewCancelled {
		t.Fatalf("Plan review after recovery = %q, err=%v; want cancelled", state.Plan.ReviewStatus, err)
	}
}
