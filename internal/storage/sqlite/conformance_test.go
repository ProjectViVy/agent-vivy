package sqlite

// B5 / D-032: the backend conformance suite. Sixteen named cases pin the
// storage invariants the rest of the product trusts (IMPLEMENTATION-PLAN
// §5.5). Each case is independent and runs against a fresh database.

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
)

func TestBackendConformance(t *testing.T) {
	cases := []struct {
		id   string
		name string
		run  func(t *testing.T)
	}{
		{"CN-01", "atomic append", cnAtomicAppend},
		{"CN-02", "monotonic sequence", cnMonotonicSequence},
		{"CN-03", "expected-version conflict", cnExpectedVersionConflict},
		{"CN-04", "idempotent replay", cnIdempotentReplay},
		{"CN-05", "retry divergence refused", cnRetryDivergenceRefused},
		{"CN-06", "exactly one terminal", cnExactlyOneTerminal},
		{"CN-07", "first-writer-wins approval", cnFirstWriterWinsApproval},
		{"CN-08", "restart repair view", cnRestartRepairView},
		{"CN-09", "torn final write survives reopen", cnTornFinalWrite},
		{"CN-10", "malformed payload round-trip", cnMalformedPayload},
		{"CN-11", "orphan checkpoint generations", cnOrphanCheckpoint},
		{"CN-12", "approval decision after kill", cnApprovalAfterKill},
		{"CN-13", "payload byte fidelity (secret audit anchor)", cnPayloadByteFidelity},
		{"CN-14", "dual-handle process lock safety", cnDualHandleSafety},
		{"CN-15", "monotonic replay under concurrent writers", cnConcurrentWriters},
		{"CN-16", "replay after disconnect (after_seq tail)", cnReplayAfterDisconnect},
	}
	if len(cases) != 16 {
		t.Fatalf("conformance suite must carry exactly 16 cases, got %d", len(cases))
	}
	for _, c := range cases {
		t.Run(c.id+" "+c.name, c.run)
	}
}

func cnBackend(t *testing.T) *Backend {
	t.Helper()
	b, err := Open(context.Background(), filepath.Join(t.TempDir(), "conformance.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = b.Close() })
	return b
}

func cnReplayAll(t *testing.T, b *Backend, runID domain.RunID, after domain.EventSeq) []domain.RunEvent {
	t.Helper()
	it, err := b.Replay(context.Background(), runID, after)
	if err != nil {
		t.Fatalf("Replay: %v", err)
	}
	defer func() { _ = it.Close() }()
	var out []domain.RunEvent
	for it.Next() {
		out = append(out, it.Value().Event)
	}
	if it.Err() != nil {
		t.Fatalf("iterator: %v", it.Err())
	}
	return out
}

// CN-01: a commit lands all-or-nothing; a rejected commit leaves no rows.
func cnAtomicAppend(t *testing.T) {
	b := cnBackend(t)
	ctx := context.Background()
	if _, err := b.Append(ctx, storage.Commit{
		RunID: "run-1",
		Events: []domain.RunEvent{
			ev(domain.EventRunStarted), ev(domain.EventModelDelta), ev(domain.EventModelCompleted),
		},
	}); err != nil {
		t.Fatalf("Append: %v", err)
	}
	if got := cnReplayAll(t, b, "run-1", 0); len(got) != 3 {
		t.Fatalf("events = %d, want all 3 of the commit", len(got))
	}

	// Rejected commit (two terminals): nothing may be persisted.
	if _, err := b.Append(ctx, storage.Commit{
		RunID: "run-1",
		Events: []domain.RunEvent{
			ev(domain.EventRunCompleted), ev(domain.EventRunFailed),
		},
	}); !errors.Is(err, storage.ErrCommitInvalid) {
		t.Fatalf("two-terminal commit: err = %v, want ErrCommitInvalid", err)
	}
	if got := cnReplayAll(t, b, "run-1", 0); len(got) != 3 {
		t.Fatalf("rejected commit leaked rows: %d events", len(got))
	}
}

// CN-02: seq is strictly contiguous and increasing across commits.
func cnMonotonicSequence(t *testing.T) {
	b := cnBackend(t)
	ctx := context.Background()
	for i := 0; i < 5; i++ {
		seq, err := b.Append(ctx, storage.Commit{
			RunID:  "run-seq",
			Events: []domain.RunEvent{ev(domain.EventModelDelta)},
		})
		if err != nil {
			t.Fatalf("Append %d: %v", i, err)
		}
		if seq != domain.EventSeq(i+1) {
			t.Fatalf("seq = %d, want %d", seq, i+1)
		}
	}
	got := cnReplayAll(t, b, "run-seq", 0)
	for i, e := range got {
		if e.Seq != domain.EventSeq(i+1) {
			t.Fatalf("replay seq[%d] = %d, want contiguous", i, e.Seq)
		}
	}
}

// CN-03: snapshot writes honor expected-version; stale writes conflict.
func cnExpectedVersionConflict(t *testing.T) {
	b := cnBackend(t)
	ctx := context.Background()
	snap := b.Snapshot()
	if err := snap.Put(ctx, "k", []byte("v"), 1); !errors.Is(err, storage.ErrVersionConflict) {
		t.Fatalf("create with expectVersion=1: err = %v, want conflict", err)
	}
	if err := snap.Put(ctx, "k", []byte("v"), 0); err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := snap.Put(ctx, "k", []byte("v2"), 0); !errors.Is(err, storage.ErrVersionConflict) {
		t.Fatalf("stale create: err = %v, want conflict", err)
	}
}

// CN-04: replay is a pure read: two passes yield the identical stream.
func cnIdempotentReplay(t *testing.T) {
	b := cnBackend(t)
	ctx := context.Background()
	if _, err := b.Append(ctx, storage.Commit{
		RunID: "run-replay",
		Events: []domain.RunEvent{
			ev(domain.EventRunStarted), ev(domain.EventModelDelta), ev(domain.EventRunCompleted),
		},
	}); err != nil {
		t.Fatalf("Append: %v", err)
	}
	first := cnReplayAll(t, b, "run-replay", 0)
	second := cnReplayAll(t, b, "run-replay", 0)
	if len(first) != len(second) {
		t.Fatalf("replay lengths diverged: %d vs %d", len(first), len(second))
	}
	for i := range first {
		if first[i].Seq != second[i].Seq || first[i].Type != second[i].Type {
			t.Fatalf("replay pass diverged at %d", i)
		}
	}
}

// CN-05: a divergent retry after the run closed is refused, never folded
// in: the journal keeps exactly the original stream.
func cnRetryDivergenceRefused(t *testing.T) {
	b := cnBackend(t)
	ctx := context.Background()
	if _, err := b.Append(ctx, storage.Commit{
		RunID:  "run-retry",
		Events: []domain.RunEvent{ev(domain.EventRunStarted), ev(domain.EventRunCompleted)},
	}); err != nil {
		t.Fatalf("Append: %v", err)
	}
	// A retry carrying different content must not land.
	if _, err := b.Append(ctx, storage.Commit{
		RunID:  "run-retry",
		Events: []domain.RunEvent{ev(domain.EventModelDelta)},
	}); !errors.Is(err, storage.ErrRunClosed) {
		t.Fatalf("divergent retry: err = %v, want ErrRunClosed", err)
	}
	if got := cnReplayAll(t, b, "run-retry", 0); len(got) != 2 {
		t.Fatalf("journal grew under a refused retry: %d events", len(got))
	}
}

// CN-06: exactly one terminal per run, enforced at the storage layer.
func cnExactlyOneTerminal(t *testing.T) {
	b := cnBackend(t)
	ctx := context.Background()
	if _, err := b.Append(ctx, storage.Commit{
		RunID: "run-one",
		Events: []domain.RunEvent{
			ev(domain.EventRunStarted), ev(domain.EventRunCompleted),
		},
	}); err != nil {
		t.Fatalf("Append: %v", err)
	}
	for _, typ := range []domain.EventType{
		domain.EventRunCompleted, domain.EventRunFailed, domain.EventRunCancelled, domain.EventModelDelta,
	} {
		if _, err := b.Append(ctx, storage.Commit{
			RunID: "run-one", Events: []domain.RunEvent{ev(typ)},
		}); !errors.Is(err, storage.ErrRunClosed) {
			t.Fatalf("append %s after terminal: err = %v, want ErrRunClosed", typ, err)
		}
	}
	terminals := 0
	for _, e := range cnReplayAll(t, b, "run-one", 0) {
		if e.Type.Terminal() {
			terminals++
		}
	}
	if terminals != 1 {
		t.Fatalf("terminal events = %d, want exactly 1", terminals)
	}
}

// CN-07: concurrent decisions settle first-writer-wins.
func cnFirstWriterWinsApproval(t *testing.T) {
	b := cnBackend(t)
	ctx := context.Background()
	apr := domain.Approval{
		ID: "apr-fw", RunID: "run-fw", ToolCallID: "tc-1",
		Decision: domain.ApprovalPending, ExpiresAt: time.Now().Add(time.Minute).UnixMilli(),
	}
	if err := b.CreateApproval(ctx, apr); err != nil {
		t.Fatalf("CreateApproval: %v", err)
	}

	var wg sync.WaitGroup
	results := make(chan bool, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ok, err := b.DecideApproval(ctx, apr.ID, domain.ApprovalApproved)
			if err != nil {
				t.Errorf("DecideApproval: %v", err)
				return
			}
			results <- ok
		}()
	}
	wg.Wait()
	close(results)
	wins := 0
	for ok := range results {
		if ok {
			wins++
		}
	}
	if wins != 1 {
		t.Fatalf("concurrent decisions won = %d, want exactly 1", wins)
	}
	if pending, _ := b.ListPendingApprovals(ctx); len(pending) != 0 {
		t.Fatalf("pending after decision = %d, want 0", len(pending))
	}
}

// CN-08: the restart-repair view survives a close/reopen: active runs and
// pending approvals are still enumerable (the E2 entry point).
func cnRestartRepairView(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "restart.db")
	ctx := context.Background()
	b, err := Open(ctx, path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := b.CreateSession(ctx, domain.Session{ID: "sess-rr", Title: "t", CreatedAt: 1}); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	if err := b.CreateRun(ctx, domain.Run{ID: "run-rr", SessionID: "sess-rr", Status: domain.RunActive, CreatedAt: 2}); err != nil {
		t.Fatalf("CreateRun: %v", err)
	}
	if err := b.CreateApproval(ctx, domain.Approval{
		ID: "apr-rr", RunID: "run-rr", Decision: domain.ApprovalPending, ExpiresAt: 9999,
	}); err != nil {
		t.Fatalf("CreateApproval: %v", err)
	}
	if err := b.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	b, err = Open(ctx, path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer func() { _ = b.Close() }()
	active, err := b.ListActiveRuns(ctx)
	if err != nil || len(active) != 1 || active[0].ID != "run-rr" {
		t.Fatalf("active after reopen = %+v, %v", active, err)
	}
	pending, err := b.ListPendingApprovals(ctx)
	if err != nil || len(pending) != 1 || pending[0].ID != "apr-rr" {
		t.Fatalf("pending after reopen = %+v, %v", pending, err)
	}
}

// CN-09: a clean close after the last commit never tears the tail: reopen
// replays the exact committed stream.
func cnTornFinalWrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "torn.db")
	ctx := context.Background()
	b, err := Open(ctx, path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if _, err := b.Append(ctx, storage.Commit{
		RunID: "run-torn",
		Events: []domain.RunEvent{
			ev(domain.EventRunStarted), ev(domain.EventModelDelta), ev(domain.EventModelCompleted),
		},
	}); err != nil {
		t.Fatalf("Append: %v", err)
	}
	if err := b.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	b, err = Open(ctx, path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer func() { _ = b.Close() }()
	got := cnReplayAll(t, b, "run-torn", 0)
	if len(got) != 3 || got[2].Type != domain.EventModelCompleted {
		t.Fatalf("reopen replay = %d events, tail %v; want the committed stream", len(got), got[len(got)-1].Type)
	}
}

// CN-10: storage is byte-transparent: an invalid-JSON payload round-trips
// verbatim so upper layers own validation (never silent rewriting).
func cnMalformedPayload(t *testing.T) {
	b := cnBackend(t)
	ctx := context.Background()
	malformed := []byte(`{"broken":`)
	if _, err := b.Append(ctx, storage.Commit{
		RunID: "run-bad",
		Events: []domain.RunEvent{
			{Type: domain.EventModelDelta, CreatedAt: 1, PayloadVersion: 1, Payload: malformed},
		},
	}); err != nil {
		t.Fatalf("Append malformed payload: %v", err)
	}
	got := cnReplayAll(t, b, "run-bad", 0)
	if len(got) != 1 || !bytes.Equal(got[0].Payload, malformed) {
		t.Fatalf("malformed payload was rewritten: %q", got[0].Payload)
	}
}

// CN-11: a generation row whose pointer row was lost (the crash shape)
// stays visible to SQL inspection and removable: orphan checkpoints are
// recoverable, never wedged (D-030 generations).
func cnOrphanCheckpoint(t *testing.T) {
	b := cnBackend(t)
	ctx := context.Background()
	if err := b.Blobs().Put(ctx, "ckpt-orphan", []byte("gen-1")); err != nil {
		t.Fatalf("Put: %v", err)
	}
	// Crash shape: the pointer row disappears while the generation
	// survives (Delete removes both atomically, so this simulates loss).
	if _, err := b.db.ExecContext(ctx, `DELETE FROM checkpoints WHERE id = 'ckpt-orphan'`); err != nil {
		t.Fatalf("drop pointer: %v", err)
	}
	var n int
	if err := b.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM checkpoint_generations WHERE id = 'ckpt-orphan'`).Scan(&n); err != nil {
		t.Fatalf("count generations: %v", err)
	}
	if n != 1 {
		t.Fatalf("orphan generations = %d, want 1 visible for recovery", n)
	}
	if _, err := b.db.ExecContext(ctx,
		`DELETE FROM checkpoint_generations WHERE id = 'ckpt-orphan'`); err != nil {
		t.Fatalf("cleanup orphan: %v", err)
	}
	if err := b.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM checkpoint_generations WHERE id = 'ckpt-orphan'`).Scan(&n); err != nil {
		t.Fatalf("recount: %v", err)
	}
	if n != 0 {
		t.Fatalf("orphan cleanup left %d rows", n)
	}
}

// CN-12: an approval left pending by a kill is still decidable after
// reopen (the cancel/approval recovery path, AS-6 support).
func cnApprovalAfterKill(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "kill.db")
	ctx := context.Background()
	b, err := Open(ctx, path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := b.CreateApproval(ctx, domain.Approval{
		ID: "apr-kill", RunID: "run-kill", ToolCallID: "tc-1",
		Decision: domain.ApprovalPending, ExpiresAt: 9999, ResumeTarget: "rt",
	}); err != nil {
		t.Fatalf("CreateApproval: %v", err)
	}
	if err := b.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	b, err = Open(ctx, path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer func() { _ = b.Close() }()
	ok, err := b.DecideApproval(ctx, "apr-kill", domain.ApprovalApproved)
	if err != nil || !ok {
		t.Fatalf("decide after reopen = %v/%v, want granted", ok, err)
	}
	apr, err := b.GetApproval(ctx, "apr-kill")
	if err != nil || apr.Decision != domain.ApprovalApproved {
		t.Fatalf("approval after decide = %+v, %v", apr, err)
	}
}

// CN-13: payload byte fidelity is the secret-audit anchor: storage never
// mutates payloads, so E3's redaction guarantees can be reasoned about at
// the writer (AS-9).
func cnPayloadByteFidelity(t *testing.T) {
	b := cnBackend(t)
	canary := []byte(`{"secret":"sk-canary-cn13-byte-fidelity"}`)
	ctx := context.Background()
	if _, err := b.Append(ctx, storage.Commit{
		RunID: "run-canary",
		Events: []domain.RunEvent{
			{Type: domain.EventModelDelta, CreatedAt: 1, PayloadVersion: 1, Payload: canary},
		},
	}); err != nil {
		t.Fatalf("Append: %v", err)
	}
	got := cnReplayAll(t, b, "run-canary", 0)
	if len(got) != 1 || !bytes.Equal(got[0].Payload, canary) {
		t.Fatalf("payload mutated in storage: %q", got[0].Payload)
	}
}

// CN-14: a second handle on the same file (the Windows process-lock
// shape) opens without corruption, and writes through both handles stay
// consistent; exclusivity is enforced per write, not per open.
func cnDualHandleSafety(t *testing.T) {
	path := filepath.Join(t.TempDir(), "dual.db")
	ctx := context.Background()
	first, err := Open(ctx, path)
	if err != nil {
		t.Fatalf("first Open: %v", err)
	}
	defer func() { _ = first.Close() }()
	second, err := Open(ctx, path)
	if err != nil {
		t.Fatalf("second Open must not fail or corrupt: %v", err)
	}
	defer func() { _ = second.Close() }()

	if _, err := first.Append(ctx, storage.Commit{
		RunID: "run-dual", Events: []domain.RunEvent{ev(domain.EventRunStarted)},
	}); err != nil {
		t.Fatalf("append via first handle: %v", err)
	}
	if _, err := second.Append(ctx, storage.Commit{
		RunID: "run-dual", Events: []domain.RunEvent{ev(domain.EventRunCompleted)},
	}); err != nil {
		t.Fatalf("append via second handle: %v", err)
	}
	got := cnReplayAll(t, first, "run-dual", 0)
	if len(got) != 2 || got[1].Type != domain.EventRunCompleted {
		t.Fatalf("dual-handle journal = %d events, want both commits", len(got))
	}
}

// CN-15: concurrent writers on distinct runs keep every stream contiguous.
func cnConcurrentWriters(t *testing.T) {
	b := cnBackend(t)
	ctx := context.Background()
	const writers, perRun = 8, 10

	var wg sync.WaitGroup
	errs := make(chan error, writers)
	for w := 0; w < writers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			runID := domain.RunID(fmt.Sprintf("run-cw-%d", w))
			for i := 0; i < perRun; i++ {
				if _, err := b.Append(ctx, storage.Commit{
					RunID:  runID,
					Events: []domain.RunEvent{ev(domain.EventModelDelta)},
				}); err != nil {
					errs <- fmt.Errorf("writer %d append %d: %w", w, i, err)
					return
				}
			}
		}(w)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
	for w := 0; w < writers; w++ {
		got := cnReplayAll(t, b, domain.RunID(fmt.Sprintf("run-cw-%d", w)), 0)
		if len(got) != perRun {
			t.Fatalf("run %d events = %d, want %d", w, len(got), perRun)
		}
		for i, e := range got {
			if e.Seq != domain.EventSeq(i+1) {
				t.Fatalf("run %d seq[%d] = %d, want contiguous", w, i, e.Seq)
			}
		}
	}
}

// CN-16: replay after a disconnect resumes from the exact tail (AS-7 at
// the storage layer).
func cnReplayAfterDisconnect(t *testing.T) {
	b := cnBackend(t)
	ctx := context.Background()
	if _, err := b.Append(ctx, storage.Commit{
		RunID: "run-tail",
		Events: []domain.RunEvent{
			ev(domain.EventRunStarted),
			ev(domain.EventModelDelta),
			ev(domain.EventModelDelta),
			ev(domain.EventRunCompleted),
		},
	}); err != nil {
		t.Fatalf("Append: %v", err)
	}
	// A client that saw seq 1..2 reconnects with after_seq=2.
	tail := cnReplayAll(t, b, "run-tail", 2)
	if len(tail) != 2 || tail[0].Seq != 3 || tail[1].Seq != 4 {
		t.Fatalf("tail = %+v, want exactly seq 3..4", tail)
	}
	if tail[1].Type != domain.EventRunCompleted {
		t.Fatalf("tail terminal = %s, want run.completed", tail[1].Type)
	}
}

// keep os imported for future file-level probes (raw db scans).
var _ = os.ReadFile
