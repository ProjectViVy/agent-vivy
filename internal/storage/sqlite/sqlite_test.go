package sqlite

import (
	"bytes"
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
)

func openBackend(t *testing.T) *Backend {
	t.Helper()
	b, err := Open(context.Background(), filepath.Join(t.TempDir(), "vivy.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = b.Close() })
	return b
}

func ev(typ domain.EventType) domain.RunEvent {
	return domain.RunEvent{Type: typ, CreatedAt: 1, PayloadVersion: 1, Payload: []byte(`{}`)}
}

func TestJournalAppendReplay(t *testing.T) {
	b := openBackend(t)
	ctx := context.Background()

	last, err := b.Append(ctx, storage.Commit{
		RunID: "run-1",
		Events: []domain.RunEvent{
			ev(domain.EventRunStarted),
			ev(domain.EventModelDelta),
		},
	})
	if err != nil || last != 2 {
		t.Fatalf("Append = %d, %v; want seq 2", last, err)
	}

	last, err = b.Append(ctx, storage.Commit{
		RunID:  "run-1",
		Events: []domain.RunEvent{ev(domain.EventRunCompleted)},
	})
	if err != nil || last != 3 {
		t.Fatalf("Append = %d, %v; want seq 3", last, err)
	}

	it, err := b.Replay(ctx, "run-1", 0)
	if err != nil {
		t.Fatalf("Replay: %v", err)
	}
	defer func() { _ = it.Close() }()

	var seqs []domain.EventSeq
	var types []domain.EventType
	for it.Next() {
		e := it.Value().Event
		seqs = append(seqs, e.Seq)
		types = append(types, e.Type)
		if e.RunID != "run-1" {
			t.Errorf("run_id = %q", e.RunID)
		}
	}
	if it.Err() != nil {
		t.Fatalf("iterator: %v", it.Err())
	}
	if len(seqs) != 3 || seqs[0] != 1 || seqs[1] != 2 || seqs[2] != 3 {
		t.Errorf("seqs = %v, want monotonic [1 2 3]", seqs)
	}
	if types[0] != domain.EventRunStarted || types[2] != domain.EventRunCompleted {
		t.Errorf("types = %v", types)
	}

	// after_seq filtering (RPC resume, AS-7).
	it2, err := b.Replay(ctx, "run-1", 2)
	if err != nil {
		t.Fatalf("Replay after 2: %v", err)
	}
	defer func() { _ = it2.Close() }()
	count := 0
	for it2.Next() {
		count++
		if got := it2.Value().Event.Seq; got != 3 {
			t.Errorf("seq after filter = %d, want 3", got)
		}
	}
	if count != 1 {
		t.Errorf("entries after seq 2 = %d, want 1", count)
	}
}

// D-008: exactly one terminal event per run.
func TestJournalTerminalGuards(t *testing.T) {
	b := openBackend(t)
	ctx := context.Background()

	if _, err := b.Append(ctx, storage.Commit{
		RunID: "run-2",
		Events: []domain.RunEvent{
			ev(domain.EventRunCompleted),
			ev(domain.EventRunFailed),
		},
	}); !errors.Is(err, storage.ErrCommitInvalid) {
		t.Errorf("two terminals in one commit: err = %v, want ErrCommitInvalid", err)
	}

	if _, err := b.Append(ctx, storage.Commit{
		RunID:  "run-2",
		Events: []domain.RunEvent{ev(domain.EventRunCompleted)},
	}); err != nil {
		t.Fatalf("Append terminal: %v", err)
	}

	if _, err := b.Append(ctx, storage.Commit{
		RunID:  "run-2",
		Events: []domain.RunEvent{ev(domain.EventModelDelta)},
	}); !errors.Is(err, storage.ErrRunClosed) {
		t.Errorf("append after terminal: err = %v, want ErrRunClosed", err)
	}

	if _, err := b.Append(ctx, storage.Commit{RunID: "run-2"}); !errors.Is(err, storage.ErrCommitInvalid) {
		t.Errorf("empty commit: err = %v, want ErrCommitInvalid", err)
	}
}

func TestSnapshotVersions(t *testing.T) {
	b := openBackend(t)
	ctx := context.Background()
	snap := b.Snapshot()

	if val, ver, err := snap.Get(ctx, "missing"); err != nil || val != nil || ver != 0 {
		t.Errorf("Get missing = %v/%d/%v", val, ver, err)
	}

	if err := snap.Put(ctx, "k", []byte("v1"), 1); !errors.Is(err, storage.ErrVersionConflict) {
		t.Errorf("create with expectVersion=1: err = %v, want conflict", err)
	}
	if err := snap.Put(ctx, "k", []byte("v1"), 0); err != nil {
		t.Fatalf("create: %v", err)
	}

	if err := snap.Put(ctx, "k", []byte("v2"), 0); !errors.Is(err, storage.ErrVersionConflict) {
		t.Errorf("stale create: err = %v, want conflict", err)
	}
	if err := snap.Put(ctx, "k", []byte("v2"), 1); err != nil {
		t.Fatalf("update: %v", err)
	}

	val, ver, err := snap.Get(ctx, "k")
	if err != nil || ver != 2 || !bytes.Equal(val, []byte("v2")) {
		t.Errorf("Get = %q/%d/%v", val, ver, err)
	}
}

// D-030: same-id writes append a generation; old bytes stay intact.
func TestBlobGenerations(t *testing.T) {
	b := openBackend(t)
	ctx := context.Background()
	blobs := b.Blobs()

	if _, ok, err := blobs.Get(ctx, "ckpt"); err != nil || ok {
		t.Errorf("Get absent = ok=%v err=%v", ok, err)
	}

	if err := blobs.Put(ctx, "ckpt", []byte("gen-1")); err != nil {
		t.Fatalf("Put gen1: %v", err)
	}
	if err := blobs.Put(ctx, "ckpt", []byte("gen-2")); err != nil {
		t.Fatalf("Put gen2: %v", err)
	}

	data, ok, err := blobs.Get(ctx, "ckpt")
	if err != nil || !ok || !bytes.Equal(data, []byte("gen-2")) {
		t.Errorf("Get = %q/%v/%v, want gen-2", data, ok, err)
	}

	// The first generation must still exist — no in-place overwrite.
	var n int
	if err := b.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM checkpoint_generations WHERE id = 'ckpt'`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Errorf("generations stored = %d, want 2", n)
	}

	if err := blobs.Delete(ctx, "ckpt"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, ok, err := blobs.Get(ctx, "ckpt"); err != nil || ok {
		t.Errorf("Get after delete = ok=%v err=%v", ok, err)
	}
}

func TestLeaseExclusivityAndExpiry(t *testing.T) {
	b := openBackend(t)
	ctx := context.Background()

	ok, err := b.Acquire(ctx, "session-lock", "owner-a", time.Minute)
	if err != nil || !ok {
		t.Fatalf("first acquire = %v/%v", ok, err)
	}

	ok, err = b.Acquire(ctx, "session-lock", "owner-b", time.Minute)
	if err != nil || ok {
		t.Errorf("second acquire = %v/%v, want denied", ok, err)
	}

	// Re-acquire by the same owner must succeed.
	ok, err = b.Acquire(ctx, "session-lock", "owner-a", time.Minute)
	if err != nil || !ok {
		t.Errorf("re-acquire by owner = %v/%v", ok, err)
	}

	// Expiry hands the lease to the next acquirer.
	ok, err = b.Acquire(ctx, "short-lock", "owner-a", time.Millisecond)
	if err != nil || !ok {
		t.Fatalf("short acquire = %v/%v", ok, err)
	}
	time.Sleep(5 * time.Millisecond)
	ok, err = b.Acquire(ctx, "short-lock", "owner-b", time.Minute)
	if err != nil || !ok {
		t.Errorf("acquire after expiry = %v/%v, want granted", ok, err)
	}

	// Release is owner-checked.
	if err := b.Release(ctx, "short-lock", "owner-a"); err != nil {
		t.Errorf("release by non-owner: %v", err)
	}
	if ok, _ := b.Acquire(ctx, "short-lock", "owner-c", time.Minute); ok {
		t.Error("lease must survive a release by the wrong owner")
	}
	if err := b.Release(ctx, "short-lock", "owner-b"); err != nil {
		t.Errorf("release by owner: %v", err)
	}
	if ok, _ := b.Acquire(ctx, "short-lock", "owner-c", time.Minute); !ok {
		t.Error("released lease must be acquirable")
	}
}

// Migrations are idempotent: reopening the same file must not fail.
func TestReopenIsIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vivy.db")
	b, err := Open(context.Background(), path)
	if err != nil {
		t.Fatalf("first Open: %v", err)
	}
	if err := b.Close(); err != nil {
		t.Fatal(err)
	}
	b, err = Open(context.Background(), path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	_ = b.Close()
}

func TestReopenRepairsCronTableAfterMigration016WasRecorded(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vivy.db")
	ctx := context.Background()

	b, err := Open(ctx, path)
	if err != nil {
		t.Fatalf("initial Open: %v", err)
	}
	if _, err := b.db.ExecContext(ctx, `DROP TABLE cron_jobs`); err != nil {
		t.Fatalf("remove cron_jobs from legacy shape: %v", err)
	}
	if _, err := b.db.ExecContext(ctx,
		`DELETE FROM schema_migrations WHERE version = 17`); err != nil {
		t.Fatalf("remove repair marker: %v", err)
	}
	if err := b.Close(); err != nil {
		t.Fatalf("close legacy shape: %v", err)
	}

	b, err = Open(ctx, path)
	if err != nil {
		t.Fatalf("Open after migration016-only shape: %v", err)
	}
	defer func() { _ = b.Close() }()

	if _, err := b.ListCronJobs(ctx); err != nil {
		t.Fatalf("ListCronJobs after repair: %v", err)
	}
	var n int
	if err := b.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM schema_migrations WHERE version = 17`).Scan(&n); err != nil {
		t.Fatalf("read repair marker: %v", err)
	}
	if n != 1 {
		t.Fatalf("migration017 marker count = %d, want 1", n)
	}
}
