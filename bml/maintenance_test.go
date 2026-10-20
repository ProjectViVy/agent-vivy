package bml

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func mustPut(t *testing.T, s *Store, rec Record, expectedStoreRevision int64) {
	t.Helper()
	if _, err := s.Put(context.Background(), rec, expectedStoreRevision, nil); err != nil {
		t.Fatalf("Put %s: %v", rec.ID, err)
	}
}

func countTableRows(t *testing.T, s *Store, table string) int {
	t.Helper()
	var n int
	if err := s.db.QueryRowContext(context.Background(),
		"SELECT COUNT(*) FROM "+table).Scan(&n); err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	return n
}

func sessionTombstoneRecord(id, workspace, sessionID, target string) Record {
	rec := tombstoneRecord(id, workspace, target)
	rec.Scope.SessionID = strptr(sessionID)
	return rec
}

// --- gc_session_scoped / gc_stale_session_scoped (typed_store.rs :686, :715)

func TestGCSessionScopedRemovesOnlyTargetSession(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t, "ws-1")

	mustPut(t, s, sessionScopedRecord("a-1", "ws-1", "sess-a", "alpha one"), 0)
	mustPut(t, s, sessionScopedRecord("a-2", "ws-1", "sess-a", "alpha two"), 1)
	mustPut(t, s, sessionScopedRecord("b-1", "ws-1", "sess-b", "beta one"), 2)
	mustPut(t, s, storeRecord("lt-1", "ws-1", "long term one"), 3)
	if err := s.PutTombstone(ctx,
		sessionTombstoneRecord("a-t", "ws-1", "sess-a", "lt-1"), 4, "lt-1", 1); err != nil {
		t.Fatalf("PutTombstone: %v", err)
	}

	deleted, err := s.GCSessionScoped(ctx, "sess-a")
	if err != nil {
		t.Fatalf("GCSessionScoped: %v", err)
	}
	if deleted != 3 {
		t.Fatalf("deleted = %d, want 3 (a-1, a-2, a-t)", deleted)
	}

	if n := countTableRows(t, s, "memory_records"); n != 2 {
		t.Fatalf("memory_records = %d, want 2 (b-1, lt-1)", n)
	}
	if n := countTableRows(t, s, "memory_supersedes"); n != 0 {
		t.Fatalf("memory_supersedes = %d, want 0", n)
	}
	for _, id := range []string{"a-1", "a-2", "a-t", "lt-1"} {
		if n := ftsRows(t, s, id); n != 0 {
			t.Fatalf("fts rows for %s = %d, want 0", id, n)
		}
	}
	if n := ftsRows(t, s, "b-1"); n != 1 {
		t.Fatalf("fts rows for b-1 = %d, want 1", n)
	}
	got, err := s.Get(ctx, "a-1")
	if err != nil || got != nil {
		t.Fatalf("Get a-1 = %v, %v; want absent", got, err)
	}
	if got, err := s.Get(ctx, "b-1"); err != nil || got == nil {
		t.Fatalf("Get b-1 = %v, %v; want present", got, err)
	}
}

func TestGCSessionScopedNoMatchDeletesNothing(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t, "ws-1")
	mustPut(t, s, sessionScopedRecord("a-1", "ws-1", "sess-a", "alpha"), 0)
	mustPut(t, s, storeRecord("lt-1", "ws-1", "long term"), 1)

	deleted, err := s.GCSessionScoped(ctx, "sess-x")
	if err != nil {
		t.Fatalf("GCSessionScoped: %v", err)
	}
	if deleted != 0 {
		t.Fatalf("deleted = %d, want 0", deleted)
	}
	if n := countTableRows(t, s, "memory_records"); n != 2 {
		t.Fatalf("memory_records = %d, want 2", n)
	}
}

func TestGCStaleSessionScopedRemovesInactiveOnly(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t, "ws-1")

	mustPut(t, s, sessionScopedRecord("a-1", "ws-1", "sess-a", "alpha one"), 0)
	mustPut(t, s, sessionScopedRecord("b-1", "ws-1", "sess-b", "beta one"), 1)
	mustPut(t, s, sessionScopedRecord("c-1", "ws-1", "sess-c", "gamma one"), 2)
	mustPut(t, s, storeRecord("lt-1", "ws-1", "long term one"), 3)

	deleted, err := s.GCStaleSessionScoped(ctx, []string{"sess-b"})
	if err != nil {
		t.Fatalf("GCStaleSessionScoped: %v", err)
	}
	if deleted != 2 {
		t.Fatalf("deleted = %d, want 2 (a-1, c-1)", deleted)
	}
	if n := countTableRows(t, s, "memory_records"); n != 2 {
		t.Fatalf("memory_records = %d, want 2 (b-1, lt-1)", n)
	}
	if n := ftsRows(t, s, "b-1"); n != 1 {
		t.Fatalf("fts rows for b-1 = %d, want 1", n)
	}
	for _, id := range []string{"a-1", "c-1"} {
		if n := ftsRows(t, s, id); n != 0 {
			t.Fatalf("fts rows for %s = %d, want 0", id, n)
		}
	}
}

func TestGCStaleSessionScopedEmptyActiveList(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t, "ws-1")

	mustPut(t, s, sessionScopedRecord("a-1", "ws-1", "sess-a", "alpha"), 0)
	mustPut(t, s, sessionScopedRecord("b-1", "ws-1", "sess-b", "beta"), 1)
	mustPut(t, s, storeRecord("lt-1", "ws-1", "long term"), 2)

	deleted, err := s.GCStaleSessionScoped(ctx, nil)
	if err != nil {
		t.Fatalf("GCStaleSessionScoped: %v", err)
	}
	if deleted != 2 {
		t.Fatalf("deleted = %d, want 2", deleted)
	}
	if n := countTableRows(t, s, "memory_records"); n != 1 {
		t.Fatalf("memory_records = %d, want 1 (lt-1)", n)
	}
}

func TestGCStaleSessionScopedAllActiveDeletesNothing(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t, "ws-1")
	mustPut(t, s, sessionScopedRecord("a-1", "ws-1", "sess-a", "alpha"), 0)
	mustPut(t, s, sessionScopedRecord("b-1", "ws-1", "sess-b", "beta"), 1)

	deleted, err := s.GCStaleSessionScoped(ctx, []string{"sess-a", "sess-b"})
	if err != nil {
		t.Fatalf("GCStaleSessionScoped: %v", err)
	}
	if deleted != 0 {
		t.Fatalf("deleted = %d, want 0", deleted)
	}
	if n := countTableRows(t, s, "memory_records"); n != 2 {
		t.Fatalf("memory_records = %d, want 2", n)
	}
}

// --- integrity (typed_store.rs :1340)

func TestIntegrityEmptyStore(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t, "ws-1")

	report, err := s.Integrity(ctx)
	if err != nil {
		t.Fatalf("Integrity: %v", err)
	}
	want := StoreIntegrity{
		SchemaVersion: 2, StoreRevision: 0, RecordCount: 0,
		TombstoneCount: 0, ContentBytes: 0, FTSRowCount: 0,
		SupersedesEdgeCount: 0, CorruptRecordIDs: nil, OrphanFTSRows: 0,
	}
	if !reflect.DeepEqual(report, want) {
		t.Fatalf("integrity = %+v, want %+v", report, want)
	}
}

func TestIntegrityCounts(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t, "ws-1")

	mustPut(t, s, storeRecord("lt-1", "ws-1", "long term one"), 0)
	mustPut(t, s, sessionScopedRecord("s-1", "ws-1", "sess-a", "scoped one"), 1)
	if err := s.PutTombstone(ctx,
		tombstoneRecord("t-1", "ws-1", "s-1"), 2, "s-1", 1); err != nil {
		t.Fatalf("PutTombstone: %v", err)
	}

	report, err := s.Integrity(ctx)
	if err != nil {
		t.Fatalf("Integrity: %v", err)
	}
	want := StoreIntegrity{
		SchemaVersion:       2,
		StoreRevision:       3,
		RecordCount:         3,
		TombstoneCount:      1,
		ContentBytes:        int64(len("long term one") + len("scoped one")),
		FTSRowCount:         1, // only lt-1; s-1's fts row is removed by the tombstone
		SupersedesEdgeCount: 1,
		CorruptRecordIDs:    nil,
		OrphanFTSRows:       0,
	}
	if !reflect.DeepEqual(report, want) {
		t.Fatalf("integrity = %+v, want %+v", report, want)
	}
}

func TestIntegrityFlagsCorruptRow(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t, "ws-1")
	mustPut(t, s, storeRecord("lt-1", "ws-1", "private"), 0)

	if _, err := s.db.ExecContext(ctx,
		"UPDATE memory_records SET record_json = '{invalid' WHERE memory_id = ?", "lt-1"); err != nil {
		t.Fatalf("corrupt row: %v", err)
	}

	report, err := s.Integrity(ctx)
	if err != nil {
		t.Fatalf("Integrity: %v", err)
	}
	if len(report.CorruptRecordIDs) != 1 || report.CorruptRecordIDs[0] != "lt-1" {
		t.Fatalf("corrupt ids = %v, want [lt-1]", report.CorruptRecordIDs)
	}
	if strings.Contains(report.CorruptRecordIDs[0], "private") {
		t.Fatalf("corrupt id leaked content: %v", report.CorruptRecordIDs)
	}
}

func TestIntegrityFlagsWorkspaceColumnMismatch(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t, "ws-1")
	mustPut(t, s, storeRecord("lt-1", "ws-1", "body"), 0)

	// Column no longer agrees with the JSON scope: corrupt even though the
	// record itself parses.
	if _, err := s.db.ExecContext(ctx,
		"UPDATE memory_records SET workspace_id = 'ws-else' WHERE memory_id = ?", "lt-1"); err != nil {
		t.Fatalf("tamper column: %v", err)
	}
	report, err := s.Integrity(ctx)
	if err != nil {
		t.Fatalf("Integrity: %v", err)
	}
	if len(report.CorruptRecordIDs) != 1 || report.CorruptRecordIDs[0] != "lt-1" {
		t.Fatalf("corrupt ids = %v, want [lt-1]", report.CorruptRecordIDs)
	}
}

func TestIntegrityFlagsOrphanFTSRows(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t, "ws-1")
	mustPut(t, s, storeRecord("lt-1", "ws-1", "body"), 0)
	if err := s.PutTombstone(ctx, tombstoneRecord("t-1", "ws-1", "lt-1"), 1, "lt-1", 1); err != nil {
		t.Fatalf("PutTombstone: %v", err)
	}

	// An fts row with no backing record, plus one re-added for the tombstone
	// record itself, are both orphans per the Rust predicate
	// (r.memory_id IS NULL OR r.tombstone = 1).
	for _, id := range []string{"ghost", "t-1"} {
		if _, err := s.db.ExecContext(ctx,
			`INSERT INTO memory_fts(memory_id, tenant_id, workspace_id, session_id, content)
			 VALUES (?, ?, ?, NULL, ?)`, id, "tenant-1", "ws-1", "x"); err != nil {
			t.Fatalf("insert orphan fts %s: %v", id, err)
		}
	}

	report, err := s.Integrity(ctx)
	if err != nil {
		t.Fatalf("Integrity: %v", err)
	}
	if report.OrphanFTSRows != 2 {
		t.Fatalf("orphan fts = %d, want 2", report.OrphanFTSRows)
	}
}

// --- backup / restore (typed_store.rs :1405, :1420)

func TestBackupRejectsExistingDestination(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t, "ws-1")
	dest := filepath.Join(t.TempDir(), "backup.sqlite3")
	if err := os.WriteFile(dest, []byte("occupied"), 0o644); err != nil {
		t.Fatalf("seed dest: %v", err)
	}
	err := s.Backup(ctx, dest)
	if code := storeErrCode(t, err); code != ErrBackupExists {
		t.Fatalf("backup over existing: code = %q", code)
	}
}

func TestBackupRestoreRoundTrip(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	s, err := Open(ctx, dir, "ws-1")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	mustPut(t, s, storeRecord("before-1", "ws-1", "kept"), 0)

	// Space in the destination exercises the VACUUM INTO quoting.
	backup := filepath.Join(t.TempDir(), "memory backup.sqlite3")
	if err := s.Backup(ctx, backup); err != nil {
		t.Fatalf("Backup: %v", err)
	}
	if _, err := os.Stat(backup); err != nil {
		t.Fatalf("backup file missing: %v", err)
	}
	mustPut(t, s, storeRecord("after-1", "ws-1", "lost on restore"), 1)

	restored, err := s.Restore(ctx, backup)
	if err != nil {
		t.Fatalf("Restore: %v", err)
	}
	defer restored.Close()
	if restored.Path() != filepath.Join(dir, "memory.sqlite3") {
		t.Fatalf("restored path = %q", restored.Path())
	}
	if got, err := restored.Get(ctx, "before-1"); err != nil || got == nil {
		t.Fatalf("Get before-1 = %v, %v; want present", got, err)
	}
	if got, err := restored.Get(ctx, "after-1"); err != nil || got != nil {
		t.Fatalf("Get after-1 = %v, %v; want absent", got, err)
	}
	// The restored store keeps the backup's revision and accepts writes.
	if _, err := restored.Put(ctx, storeRecord("post-1", "ws-1", "after restore"), 1, nil); err != nil {
		t.Fatalf("Put after restore: %v", err)
	}
}

func TestRestoreRejectsMissingAndGarbageBackup(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t, "ws-1")

	if _, err := s.Restore(ctx, filepath.Join(t.TempDir(), "missing.sqlite3")); err == nil {
		t.Fatal("Restore missing backup: expected error")
	} else if code := storeErrCode(t, err); code != ErrInvalidBackup {
		t.Fatalf("missing backup: code = %q", code)
	}
	// Restore consumes the store (Rust `restore(self)`); reopen for the next case.
	s = openTestStore(t, "ws-1")
	garbage := filepath.Join(t.TempDir(), "garbage.sqlite3")
	if err := os.WriteFile(garbage, []byte("not a sqlite database"), 0o644); err != nil {
		t.Fatalf("write garbage: %v", err)
	}
	if _, err := s.Restore(ctx, garbage); err == nil {
		t.Fatal("Restore garbage backup: expected error")
	} else if code := storeErrCode(t, err); code != ErrInvalidBackup {
		t.Fatalf("garbage backup: code = %q", code)
	}
}

// --- canonical workspace identity (typed_store.rs :220, :233, :256, :345)
