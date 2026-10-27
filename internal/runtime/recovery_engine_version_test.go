package runtime

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage/sqlite"
	"agent-vivy/internal/tools"
)

// A restart under a different Eino engine version (upgrade or rollback)
// cannot resume a paused run's checkpoint. Recovery closes the run with the
// upgrade message and settles its pending interaction, so no actionable card
// outlives it. Checkpoints are never converted across versions.

// restartServiceAt mirrors restartService with an explicit engine version, so
// a test can simulate a Vivy upgrade across a paused run.
func restartServiceAt(t *testing.T, backend *sqlite.Backend, engineVersion string) (*Service, *testSink) {
	t.Helper()
	ctx := context.Background()

	ts, err := tools.Builtin(backend).Resolve([]string{tools.EchoInfoName, tools.WriteNoteName})
	if err != nil {
		t.Fatalf("resolve tools: %v", err)
	}
	checkpoints, err := NewVersionedCheckpointStore(backend.Blobs(), engineVersion)
	if err != nil {
		t.Fatalf("checkpoint store: %v", err)
	}
	resumed := NewScriptedModel(testAssistant("Done: the note has been handled."))
	eng, err := NewEngine(ctx, resumed, ts, EngineConfig{
		StreamBuffer: 8, MaxEventPayloadBytes: 64 << 10, Checkpoints: checkpoints,
	})
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}
	sink := newTestSink()
	svc := NewService(eng, "scripted", "scripted-v0", ServiceDeps{Admission: backend, GenerationID: "test-generation",
		Journal: backend, Runs: backend, Messages: backend, Notes: backend, Approvals: backend, Questions: backend,
		ApprovalExpiration: 5 * time.Minute, Sink: sink,
	})
	return svc, sink
}

// restartQuestionServiceAt is restartQuestionService with an explicit engine
// version.
func restartQuestionServiceAt(t *testing.T, backend *sqlite.Backend, engineVersion string) *Service {
	t.Helper()
	ctx := context.Background()
	ts, err := tools.Builtin(backend).Resolve([]string{tools.AskUserName})
	if err != nil {
		t.Fatalf("resolve ask_user: %v", err)
	}
	checkpoints, err := NewVersionedCheckpointStore(backend.Blobs(), engineVersion)
	if err != nil {
		t.Fatalf("checkpoint store: %v", err)
	}
	eng, err := NewEngine(ctx, NewScriptedModel(testAssistant("Recovered answer.")), ts, EngineConfig{
		StreamBuffer: 8, MaxEventPayloadBytes: 64 << 10, Checkpoints: checkpoints,
	})
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}
	return NewService(eng, "scripted", "scripted-v0", ServiceDeps{Admission: backend, GenerationID: "test-generation",
		Journal: backend, Runs: backend, Messages: backend, Questions: backend,
		ApprovalExpiration: 5 * time.Minute, Sink: newTestSink(),
	})
}

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

func TestRecoverAcrossEngineVersionCancelsPendingPlanReview(t *testing.T) {
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
		SessionID: sessionID, RequestID: "enter-plan", RequestInput: "enter-plan", Kind: domain.WorkEventPlanEntered,
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
		})
		if err != nil {
			t.Fatalf("new engine: %v", err)
		}
		return NewService(eng, "scripted", "scripted-v0", ServiceDeps{Admission: backend, GenerationID: "test-generation",
			Journal: backend, Runs: backend, Messages: backend, Sessions: backend,
			PrimaryRuns: backend, Work: backend, Sink: newTestSink(),
		})
	}
	initial := planService("test-engine", NewScriptedModel(
		testToolCalls(testCall("submit-plan", tools.SubmitPlanName, `{"markdown":"1. inspect"}`)),
		testAssistant("The plan is ready for review."),
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

	restarted := planService("test-engine-next", NewScriptedModel(testAssistant("unused")))
	if err := restarted.Recover(ctx); err != nil {
		t.Fatalf("recover: %v", err)
	}
	assertEngineChangedFailure(t, backend, runID)
	state, err := backend.ReadWork(ctx, sessionID)
	if err != nil || state.Plan.ReviewStatus != domain.PlanReviewCancelled {
		t.Fatalf("Plan review after recovery = %q, err=%v; want cancelled", state.Plan.ReviewStatus, err)
	}
}

// A restart after the model-generation migration cannot resume a checkpoint
// written by the previous generation: the persisted ADK state holds a different
// message type. Recovery closes the paused run through the same lost-checkpoint
// path (with the engine-changed message) and settles the pending approval.
func TestRecoverAcrossModelGenerationCancelsPendingApproval(t *testing.T) {
	svc, backend, _ := newApprovalService(t, 5*time.Minute)
	ctx := context.Background()
	mustCreateSession(t, backend, "sess-generation")
	runID, err := svc.Run(ctx, "sess-generation", "note that I need milk")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	approval := waitForPendingApproval(t, backend, runID)
	waitForApprovalEvent(t, backend, runID)

	// Rewrite the stored envelope as a pre-migration one: it carries no
	// generation field at all, exactly like a text-generation checkpoint.
	downgradeCheckpointGeneration(t, ctx, backend, checkpointIDFor(runID))

	restarted, _ := restartService(t, backend)
	if err := restarted.Recover(ctx); err != nil {
		t.Fatalf("recover: %v", err)
	}
	assertEngineChangedFailure(t, backend, runID)
	got, err := backend.GetApproval(ctx, approval.ID)
	if err != nil || got.Decision != domain.ApprovalCancelled {
		t.Fatalf("approval after recovery = %q, err=%v; want cancelled", got.Decision, err)
	}
	// Idempotent: the second pass has nothing left to recover.
	if err := restarted.Recover(ctx); err != nil {
		t.Fatalf("second recover: %v", err)
	}
	if n := countTerminal(replayAll(t, backend, runID)); n != 1 {
		t.Fatalf("terminal events after second recovery = %d, want 1", n)
	}
}

// downgradeCheckpointGeneration rewrites one stored checkpoint as a
// text-generation envelope, keeping the payload and prompt identity intact so
// only the generation check can reject it.
func downgradeCheckpointGeneration(t *testing.T, ctx context.Context, backend *sqlite.Backend, id string) {
	t.Helper()
	raw, ok, err := backend.Blobs().Get(ctx, id)
	if err != nil || !ok {
		t.Fatalf("read checkpoint %q: ok=%v err=%v", id, ok, err)
	}
	payload, env, err := decodeCheckpointEnvelope(raw)
	if err != nil {
		t.Fatalf("decode checkpoint envelope: %v", err)
	}
	env.Generation = ""
	header, err := json.Marshal(env)
	if err != nil {
		t.Fatalf("marshal legacy envelope: %v", err)
	}
	blob := binary.BigEndian.AppendUint32(nil, uint32(len(header)))
	blob = append(blob, header...)
	blob = append(blob, payload...)
	if err := backend.Blobs().Put(ctx, id, blob); err != nil {
		t.Fatalf("write legacy checkpoint: %v", err)
	}
}
