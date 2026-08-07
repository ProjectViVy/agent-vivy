package runtime

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/cloudwego/eino/schema"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage/sqlite"
	"agent-vivy/internal/tools"
)

// restartService builds a fresh engine + service over an existing
// backend, exactly what app.New does after a process restart: same
// checkpoint bridge version, new in-memory state. The model carries only
// the resumed turn's reply: the real provider is stateless (checkpoint
// history drives it), whereas ScriptedModel's call counter is a test
// artifact that a restart resets.
func restartService(t *testing.T, backend *sqlite.Backend) (*Service, *testSink) {
	t.Helper()
	ctx := context.Background()

	ts, err := tools.Builtin().Resolve([]string{tools.EchoInfoName, tools.WriteNoteName})
	if err != nil {
		t.Fatalf("resolve tools: %v", err)
	}
	checkpoints, err := NewVersionedCheckpointStore(backend.Blobs(), "test-engine")
	if err != nil {
		t.Fatalf("checkpoint store: %v", err)
	}
	resumed := NewScriptedModel(schema.AssistantMessage("Done: the note has been handled.", nil))
	eng, err := NewEngine(ctx, resumed, ts, EngineConfig{
		StreamBuffer: 8, MaxEventPayloadBytes: 64 << 10, Checkpoints: checkpoints,
	})
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}
	sink := newTestSink()
	svc := NewService(eng, "scripted", "scripted-v0", ServiceDeps{
		Journal: backend, Runs: backend, Messages: backend, Approvals: backend,
		ApprovalExpiration: 5 * time.Minute, Sink: sink,
	})
	return svc, sink
}

// waitForApprovalEvent polls until the suspend commit is durable: the
// approval row exists before its journal event (D-029 order), and a
// crash simulation must start from the fully-suspended state.
func waitForApprovalEvent(t *testing.T, backend *sqlite.Backend, runID domain.RunID) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if indexOfType(replayAll(t, backend, runID), domain.EventToolApprovalRequired) >= 0 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("run %s never journaled tool.approval_required", runID)
}

// lastRunFailed decodes the run's terminal event and asserts it is a
// run.failed carrying the restart-recovery message.
func lastRunFailed(t *testing.T, backend *sqlite.Backend, runID domain.RunID) {
	t.Helper()
	events := replayAll(t, backend, runID)
	last := events[len(events)-1]
	if last.Type != domain.EventRunFailed {
		t.Fatalf("last event = %s, want run.failed", last.Type)
	}
	var p payloadRunFailed
	if err := json.Unmarshal(last.Payload, &p); err != nil {
		t.Fatalf("decode run.failed payload: %v", err)
	}
	if p.CauseCategory != causeInternalError {
		t.Fatalf("cause_category = %q, want %q", p.CauseCategory, causeInternalError)
	}
	if !strings.Contains(p.Message, "server restart") {
		t.Fatalf("run.failed message = %q, want the restart-recovery wording", p.Message)
	}
}

func TestServiceRecoverResumableApproval(t *testing.T) {
	svc, backend, _ := newApprovalService(t, 5*time.Minute)
	ctx := context.Background()

	runID, err := svc.Run(ctx, "sess-1", "note that I need milk")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	approval := waitForPendingApproval(t, backend, runID)
	waitForApprovalEvent(t, backend, runID)

	// Restart: fresh engine + service over the same durable state.
	restarted, _ := restartService(t, backend)
	if err := restarted.Recover(ctx); err != nil {
		t.Fatalf("recover: %v", err)
	}

	// The run stays active: recovery rebuilt the pending registration
	// instead of closing it.
	r, err := backend.GetRun(ctx, runID)
	if err != nil {
		t.Fatalf("get run: %v", err)
	}
	if r.Status != domain.RunActive {
		t.Fatalf("recovered run status = %s, want active", r.Status)
	}

	// Deciding the approval resumes the recovered run to completion.
	if err := restarted.DecideApproval(ctx, approval.ID, domain.ApprovalApproved); err != nil {
		t.Fatalf("decide after recovery: %v", err)
	}
	waitForRunStatus(t, backend, runID, domain.RunCompleted)

	events := replayAll(t, backend, runID)
	if n := countTerminal(events); n != 1 {
		t.Fatalf("terminal events = %d, want exactly 1", n)
	}
	if events[len(events)-1].Type != domain.EventRunCompleted {
		t.Fatalf("last event = %s, want run.completed", events[len(events)-1].Type)
	}
}

func TestServiceRecoverExpiredApprovalFails(t *testing.T) {
	// One-millisecond expiry: the approval is already stale by the time
	// the restarted process looks at it.
	svc, backend, _ := newApprovalService(t, time.Millisecond)
	ctx := context.Background()

	runID, err := svc.Run(ctx, "sess-1", "note that I need milk")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	waitForPendingApproval(t, backend, runID)
	waitForApprovalEvent(t, backend, runID)
	time.Sleep(10 * time.Millisecond)

	restarted, _ := restartService(t, backend)
	if err := restarted.Recover(ctx); err != nil {
		t.Fatalf("recover: %v", err)
	}

	waitForRunStatus(t, backend, runID, domain.RunFailed)
	lastRunFailed(t, backend, runID)
}

func TestServiceRecoverOrphanRunFails(t *testing.T) {
	svc, backend, _ := newApprovalService(t, 5*time.Minute)
	_ = svc
	ctx := context.Background()

	// An active run row with no pending approval: the crash window
	// between run.started and any suspend point.
	if err := backend.CreateSession(ctx, domain.Session{ID: "sess-orphan", Title: "orphan", CreatedAt: time.Now().UnixMilli()}); err != nil {
		t.Fatalf("create session: %v", err)
	}
	orphan := domain.Run{ID: "run-orphan", SessionID: "sess-orphan", Status: domain.RunActive, CreatedAt: time.Now().UnixMilli()}
	if err := backend.CreateRun(ctx, orphan); err != nil {
		t.Fatalf("create orphan run: %v", err)
	}

	restarted, _ := restartService(t, backend)
	if err := restarted.Recover(ctx); err != nil {
		t.Fatalf("recover: %v", err)
	}

	waitForRunStatus(t, backend, orphan.ID, domain.RunFailed)
	lastRunFailed(t, backend, orphan.ID)

	// Idempotence: a second recovery pass sees no non-terminal runs and
	// the journal still holds exactly one terminal (D-008).
	if err := restarted.Recover(ctx); err != nil {
		t.Fatalf("second recover: %v", err)
	}
	if n := countTerminal(replayAll(t, backend, orphan.ID)); n != 1 {
		t.Fatalf("terminal events after double recovery = %d, want 1", n)
	}
}

func TestServiceRecoverNothingToDo(t *testing.T) {
	svc, backend, _ := newApprovalService(t, 5*time.Minute)
	ctx := context.Background()

	// The scripted model suspends the run on an approval; nothing else is
	// in flight. Recovery on live process state must not close the run or
	// append events (it only rebuilds the pending registration).
	runID, err := svc.Run(ctx, "sess-1", "note that I need milk")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	approval := waitForPendingApproval(t, backend, runID)
	waitForApprovalEvent(t, backend, runID)

	before := replayAll(t, backend, runID)
	if err := svc.Recover(ctx); err != nil {
		t.Fatalf("recover: %v", err)
	}
	r, err := backend.GetRun(ctx, runID)
	if err != nil {
		t.Fatalf("get run: %v", err)
	}
	if r.Status != domain.RunActive {
		t.Fatalf("run status after no-op recovery = %s, want active", r.Status)
	}
	after := replayAll(t, backend, runID)
	if len(after) != len(before) {
		t.Fatalf("no-op recovery appended events: before=%d after=%d", len(before), len(after))
	}

	// The original in-process registration survives: the decision still
	// resumes the run.
	if err := svc.DecideApproval(ctx, approval.ID, domain.ApprovalApproved); err != nil {
		t.Fatalf("decide: %v", err)
	}
	waitForRunStatus(t, backend, runID, domain.RunCompleted)
}
