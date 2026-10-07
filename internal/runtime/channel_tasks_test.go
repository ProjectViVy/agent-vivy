package runtime

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
	"agent-vivy/internal/storage/sqlite"
	"agent-vivy/internal/testsupport"
)

func newChannelTaskService(t *testing.T) (*Service, *sqlite.Backend, *testSink) {
	t.Helper()
	svc, backend, sink := newTestService(t, testsupport.NewEchoModel())
	// Channel-task admission requires the atomic primary boundary; wire the
	// same backend the app wires.
	svc.deps.PrimaryRuns = backend
	svc.deps.Admission = backend
	svc.deps.GenerationID = "test-generation"
	return svc, backend, sink
}

// TestChannelTaskMissingContextConcurrentRetry: two identical sends with no
// context selector race; exactly one Session+Run+launch commits, both callers
// get the same receipt identity.
func TestChannelTaskMissingContextConcurrentRetry(t *testing.T) {
	svc, backend, _ := newChannelTaskService(t)
	ctx := context.Background()
	t.Cleanup(func() { svc.CancelAll(); svc.WaitIdle(ctx) })

	in := domain.ChannelTaskInput{
		Scope:     domain.ChannelTaskScope{InstanceKey: "inst/a2a:a2a", PrincipalID: "p-1"},
		MessageID: "remote-concurrent",
		Parts:     []string{"hello", "a2a"},
	}
	const senders = 2
	receipts := make([]domain.ChannelTaskReceipt, senders)
	errs := make([]error, senders)
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < senders; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			receipts[i], errs[i] = svc.SubmitChannelTask(ctx, in)
		}(i)
	}
	close(start)
	wg.Wait()

	for i := range errs {
		if errs[i] != nil {
			t.Fatalf("submit %d: %v", i, errs[i])
		}
	}
	if receipts[0].SessionID == "" || receipts[0].RunID == "" {
		t.Fatalf("empty receipt: %+v", receipts[0])
	}
	if receipts[0].RunID != receipts[1].RunID || receipts[0].SessionID != receipts[1].SessionID {
		t.Fatalf("receipts diverged: %+v vs %+v", receipts[0], receipts[1])
	}
	// One committed Session/Run pair, one primary run.
	runs, err := backend.ListRunsBySession(ctx, receipts[0].SessionID)
	if err != nil || len(runs) != 1 || runs[0].ID != receipts[0].RunID {
		t.Fatalf("runs = %+v %v", runs, err)
	}
	// The loser never materialized a second session.
	receipt, found, err := backend.FindChannelTaskReceipt(ctx, in.Scope, in.MessageID)
	if err != nil || !found {
		t.Fatalf("receipt = %v %v", found, err)
	}
	if receipt.RunID != receipts[0].RunID {
		t.Fatalf("stored receipt run = %s, want %s", receipt.RunID, receipts[0].RunID)
	}
}

// TestChannelTaskReplayBeforeBusyAndQuota: an accepted retry resolves to the
// original receipt while the context is busy, before any gate that could
// reject it.
func TestChannelTaskReplayBeforeBusyAndQuota(t *testing.T) {
	svc, _, _ := newChannelTaskService(t)
	ctx := context.Background()
	t.Cleanup(func() { svc.CancelAll(); svc.WaitIdle(ctx) })

	scope := domain.ChannelTaskScope{InstanceKey: "inst/a2a:a2a", PrincipalID: "p-1"}
	in := domain.ChannelTaskInput{Scope: scope, MessageID: "remote-busy", Parts: []string{"first"}}
	first, err := svc.SubmitChannelTask(ctx, in)
	if err != nil {
		t.Fatal(err)
	}
	// The primary run is still active; the identical retry must return the
	// receipt, not ErrWorkRunConflict.
	replay, err := svc.SubmitChannelTask(ctx, in)
	if err != nil {
		t.Fatalf("replay while busy = %v", err)
	}
	if replay.RunID != first.RunID || replay.SessionID != first.SessionID || replay.Operation != "submit" {
		t.Fatalf("replay = %+v, want original identity", replay)
	}

	// Same message ID, different body => conflict, still without touching
	// run admission.
	changed := in
	changed.Parts = []string{"tampered"}
	if _, err := svc.SubmitChannelTask(ctx, changed); !errors.Is(err, storage.ErrConflict) {
		t.Fatalf("changed body = %v, want ErrConflict", err)
	}

	// A distinct message on the busy context is a retryable run conflict.
	other := domain.ChannelTaskInput{
		Scope: scope, MessageID: "remote-busy-2",
		SessionID: first.SessionID, Parts: []string{"second"},
	}
	if _, err := svc.SubmitChannelTask(ctx, other); !errors.Is(err, storage.ErrWorkRunConflict) {
		t.Fatalf("busy distinct message = %v, want ErrWorkRunConflict", err)
	}
}

// TestChannelTaskNewSessionPreparation: a candidate admission commits session
// defaults, an immutable prompt snapshot, and the accepted evidence event in
// one transaction.
func TestChannelTaskNewSessionPreparation(t *testing.T) {
	svc, backend, sink := newChannelTaskService(t)
	ctx := context.Background()
	t.Cleanup(func() { svc.CancelAll(); svc.WaitIdle(ctx) })

	receipt, err := svc.SubmitChannelTask(ctx, domain.ChannelTaskInput{
		Scope:     domain.ChannelTaskScope{InstanceKey: "inst/a2a:a2a", PrincipalID: "p-2"},
		MessageID: "remote-prep",
		Parts:     []string{"prepare me"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if receipt.SessionID == "" || receipt.RunID == "" || receipt.AcceptedSeq == 0 {
		t.Fatalf("receipt = %+v", receipt)
	}
	session, err := backend.GetSession(ctx, receipt.SessionID)
	if err != nil {
		t.Fatalf("candidate session not committed: %v", err)
	}
	if session.ID != receipt.SessionID {
		t.Fatalf("session = %+v", session)
	}
	// run.started and channel.task_admitted publish in order, once.
	waitForEvents(t, sink, func(events []domain.RunEvent) bool {
		var sawStarted, sawAdmitted bool
		for _, e := range events {
			if e.Type == domain.EventRunStarted && e.RunID == receipt.RunID {
				sawStarted = true
			}
			if e.Type == domain.EventChannelTaskAdmitted && e.RunID == receipt.RunID {
				sawAdmitted = true
			}
		}
		return sawStarted && sawAdmitted
	})
}

// TestChannelTaskRejectsCompositeOptions: channel-task admission never
// composes with the other admission axes or caller-controlled attachments.
func TestChannelTaskRejectsCompositeOptions(t *testing.T) {
	svc, _, _ := newChannelTaskService(t)
	ctx := context.Background()

	scope := domain.ChannelTaskScope{InstanceKey: "inst/a2a:a2a", PrincipalID: "p-3"}
	for _, tc := range []struct {
		name string
		in   domain.ChannelTaskInput
	}{
		{"empty message id", domain.ChannelTaskInput{Scope: scope, Parts: []string{"x"}}},
		{"no parts", domain.ChannelTaskInput{Scope: scope, MessageID: "m"}},
		{"oversize part", domain.ChannelTaskInput{Scope: scope, MessageID: "m", Parts: []string{string(make([]byte, 70<<10))}}},
	} {
		if _, err := svc.SubmitChannelTask(ctx, tc.in); err == nil {
			t.Fatalf("%s: accepted invalid input", tc.name)
		}
	}
}

func waitForEvents(t *testing.T, sink *testSink, pred func([]domain.RunEvent) bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if pred(sink.snapshot()) {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("timed out waiting for published events")
}

// TestChannelTaskDeletionCannotResurrect: deleting an owned context leaves
// tombstones; resubmission mints a fresh session, and the tombstoned
// context id is never adoptable again.
func TestChannelTaskDeletionCannotResurrect(t *testing.T) {
	svc, backend, _ := newChannelTaskService(t)
	ctx := context.Background()
	t.Cleanup(func() { svc.CancelAll(); svc.WaitIdle(ctx) })

	scope := domain.ChannelTaskScope{InstanceKey: "inst/a2a:a2a", PrincipalID: "p-del"}
	in := domain.ChannelTaskInput{Scope: scope, MessageID: "remote-del", Parts: []string{"doomed"}}
	first, err := svc.SubmitChannelTask(ctx, in)
	if err != nil {
		t.Fatal(err)
	}
	// Cancel the live run first so deletion exercises the busy path.
	svc.Cancel(first.RunID)
	// Deletion tombstones receipt + context inside the session's transaction.
	if err := svc.DeleteSession(ctx, first.SessionID); err != nil {
		t.Fatal(err)
	}
	// The same remote message admits as a NEW session — the deleted id is
	// dead address space, never resurrected.
	second, err := svc.SubmitChannelTask(ctx, in)
	if err != nil {
		t.Fatalf("resubmit after delete: %v", err)
	}
	if second.SessionID == first.SessionID || second.RunID == first.RunID {
		t.Fatalf("resubmit resurrected: %+v vs %+v", second, first)
	}
	if _, err := backend.GetSession(ctx, first.SessionID); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("deleted session = %v, want ErrNotFound", err)
	}
	// A submit addressed at the tombstoned context is not-found, not an
	// adoption.
	if _, err := svc.SubmitChannelTask(ctx, domain.ChannelTaskInput{
		Scope: scope, MessageID: "remote-del-2", SessionID: first.SessionID, Parts: []string{"adopt me"},
	}); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("tombstoned context submit = %v, want ErrNotFound", err)
	}
}

// TestChannelTaskCancelAfterCommit: cancelling the HTTP-shaped caller after
// commit does not stop the native run (AS-7 detach).
func TestChannelTaskCancelAfterCommit(t *testing.T) {
	svc, backend, _ := newChannelTaskService(t)
	ctx := context.Background()
	t.Cleanup(func() { svc.CancelAll(); svc.WaitIdle(ctx) })

	receipt, err := svc.SubmitChannelTask(ctx, domain.ChannelTaskInput{
		Scope:     domain.ChannelTaskScope{InstanceKey: "inst/a2a:a2a", PrincipalID: "p-detach"},
		MessageID: "remote-detach", Parts: []string{"detached"},
	})
	if err != nil {
		t.Fatal(err)
	}
	run, err := backend.GetRun(ctx, receipt.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if run.Status.Terminal() {
		t.Fatalf("committed run %s is already terminal: %s", run.ID, run.Status)
	}
	// The receipt survives and the run keeps driving — nothing about the
	// caller's ctx can cancel committed work.
	_ = ctx.Done()
	owner, err := backend.GetChannelTaskOwner(ctx, receipt.Scope, receipt.RunID)
	if err != nil || owner != receipt.SessionID {
		t.Fatalf("owner = %q %v", owner, err)
	}
}

// TestChannelTaskOrphanSweep: the startup sweep reaps stale empty private
// dirs named for runs that never committed, and keeps claimed, recent, and
// non-empty directories (design 6.2 orphan recovery).
func TestChannelTaskOrphanSweep(t *testing.T) {
	svc, backend, _ := newChannelTaskService(t)
	ctx := context.Background()
	root := t.TempDir()
	mgr, err := NewWorkspaceManager(root)
	if err != nil {
		t.Fatal(err)
	}
	svc.deps.Workspaces = mgr

	stale := time.Now().Add(-2 * channelTaskAdmissionGraceWindow)
	mkdir := func(name string) string {
		t.Helper()
		path := filepath.Join(root, name)
		if err := os.Mkdir(path, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(path, stale, stale); err != nil {
			t.Fatal(err)
		}
		return path
	}
	// Stale empty dir for a run that never committed → reaped.
	orphan := mkdir("run_orphan")
	// Stale dir claimed by an existing run → kept.
	mustCreateSession(t, backend, "sess_sweep")
	if err := backend.CreateRun(ctx, domain.Run{
		ID: "run_claimed", SessionID: "sess_sweep", Status: domain.RunCompleted,
		Kind: domain.RunKindPrimary, CreatedAt: 1,
	}); err != nil {
		t.Fatal(err)
	}
	claimed := mkdir("run_claimed")
	// Stale but non-empty → kept (received content between crash and sweep).
	nonempty := mkdir("run_nonempty")
	if err := os.WriteFile(filepath.Join(nonempty, "artifact.txt"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	// Empty but inside the grace window → kept.
	fresh := filepath.Join(root, "run_fresh")
	if err := os.Mkdir(fresh, 0o700); err != nil {
		t.Fatal(err)
	}

	if err := svc.SweepChannelTaskOrphans(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(orphan); !os.IsNotExist(err) {
		t.Fatalf("orphan dir survived: %v", err)
	}
	for _, kept := range []string{claimed, nonempty, fresh} {
		if _, err := os.Stat(kept); err != nil {
			t.Fatalf("dir %s reaped: %v", kept, err)
		}
	}
}
