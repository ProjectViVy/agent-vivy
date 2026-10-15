package bml

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestIssue58BoundBackupRestoreAndReadOnlyCapabilities(t *testing.T) {
	ctx := context.Background()
	for _, version := range []int{1, 2} {
		t.Run(map[int]string{1: "legacy_backup", 2: "bound_backup"}[version], func(t *testing.T) {
			dir := t.TempDir()
			backup := filepath.Join(t.TempDir(), storeFileName)
			s, err := Open(ctx, dir, machineMemoryScope)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = s.Close() }()
			var original MutationOutcome
			if version == 1 {
				issue58SchemaOne(t, backup, machineMemoryScope)
			} else {
				original, err = s.MutateBound(ctx, issue58Owner, issue58Create("backup-record"))
				if err != nil {
					t.Fatal(err)
				}
				if err := s.Backup(ctx, backup); err != nil {
					t.Fatal(err)
				}
			}
			before, err := os.ReadFile(backup)
			if err != nil {
				t.Fatal(err)
			}
			ro, err := OpenExisting(ctx, filepath.Dir(backup), machineMemoryScope)
			if err != nil {
				t.Fatal(err)
			}
			caps := ro.BoundCapabilities()
			if caps.Mutate || caps.Export != (version == 2) || caps.MutationLookup != (version == 2) || caps.Search != (version == 2) {
				t.Fatalf("untruthful readonly capability: %+v", caps)
			}
			if _, err := ro.MutateBound(ctx, issue58Owner, issue58Create("readonly-write")); err == nil {
				t.Fatal("readonly native mutation claimed success")
			}
			if version == 1 {
				if _, err := ro.LookupBoundMutation(ctx, issue58Owner, "old", "digest"); err == nil {
					t.Fatal("schema1 claimed durable lookup capability")
				}
			}
			if version == 2 {
				if got, err := ro.LookupBoundMutation(ctx, issue58Owner, original.OperationID, original.PayloadDigest); err != nil || got == nil || *got != original {
					t.Fatalf("readonly outcome lost: %+v %v", got, err)
				}
			}
			if err := ro.Close(); err != nil {
				t.Fatal(err)
			}
			if after, err := os.ReadFile(backup); err != nil || !reflect.DeepEqual(before, after) {
				t.Fatalf("readonly validation changed backup: %v", err)
			}
			s, err = s.Restore(ctx, backup)
			if err != nil {
				t.Fatal(err)
			}
			if meta, err := s.Metadata(ctx); err != nil || meta.SchemaVersion != 2 {
				t.Fatalf("restored copy not upgraded: %+v %v", meta, err)
			}
			if after, err := os.ReadFile(backup); err != nil || !reflect.DeepEqual(before, after) {
				t.Fatalf("restore upgraded/rewrote backup bytes: %v", err)
			}
			if version == 2 {
				if got, err := s.LookupBoundMutation(ctx, issue58Owner, original.OperationID, original.PayloadDigest); err != nil || got == nil || *got != original {
					t.Fatalf("backup/restore lost immutable result: %+v %v", got, err)
				}
			}
		})
	}
}

func TestIssue58BoundIdentityMigrationRejectedBeforeWrites(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	legacy := LegacyPathWorkspaceID(dir)
	s, err := Open(ctx, dir, legacy)
	if err != nil {
		t.Fatal(err)
	}
	request := issue58Create("bound-identity")
	request.Record.Scope.WorkspaceID = legacy
	out, err := s.MutateBound(ctx, issue58Owner, request)
	if err != nil {
		t.Fatal(err)
	}
	// A historical outcome alone must retain the original identity even when
	// there are no active records/ownership rows left (e.g. external retention).
	for _, onlyHistory := range []bool{false, true} {
		t.Run(map[bool]string{false: "owned_records", true: "historical_outcome"}[onlyHistory], func(t *testing.T) {
			if onlyHistory {
				if _, err := s.db.Exec(`DELETE FROM memory_ownership; DELETE FROM memory_fts; DELETE FROM memory_records`); err != nil {
					t.Fatal(err)
				}
			}
			if err := s.Close(); err != nil {
				t.Fatal(err)
			}
			before, err := os.ReadFile(filepath.Join(dir, storeFileName))
			if err != nil {
				t.Fatal(err)
			}
			if migrated, err := OpenExistingCanonical(ctx, dir); err == nil {
				_ = migrated.Close()
				t.Fatal("canonical migration rewrote bound immutable state")
			}
			if after, err := os.ReadFile(filepath.Join(dir, storeFileName)); err != nil || !reflect.DeepEqual(before, after) {
				t.Fatalf("rejected migration changed database bytes: %v", err)
			}
			if _, err := os.Stat(filepath.Join(dir, "migrations", migrationDirName)); !os.IsNotExist(err) {
				t.Fatalf("rejected migration created backup/manifest: %v", err)
			}
			s, err = Open(ctx, dir, legacy)
			if err != nil {
				t.Fatal(err)
			}
			if got, err := s.LookupBoundMutation(ctx, issue58Owner, out.OperationID, out.PayloadDigest); err != nil || got == nil || *got != out {
				t.Fatalf("identity rejection lost receipt: %+v %v", got, err)
			}
		})
	}
	_ = s.Close()
}

func TestIssue58BoundGCAndCommitFailurePreserveAtomicState(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t, machineMemoryScope)
	if _, err := s.db.Exec(`CREATE TABLE issue58_parent(id INTEGER PRIMARY KEY); CREATE TABLE issue58_commit_guard(parent_id INTEGER REFERENCES issue58_parent(id) DEFERRABLE INITIALLY DEFERRED); CREATE TRIGGER issue58_commit_failure AFTER INSERT ON memory_operations BEGIN INSERT INTO issue58_commit_guard(parent_id) VALUES(999); END`); err != nil {
		t.Fatal(err)
	}
	request := issue58Create("bound-session")
	session := "session-owned"
	request.Record.Scope.SessionID = &session
	if _, err := s.MutateBound(ctx, issue58Owner, request); err == nil {
		t.Fatal("deferred constraint commit failure acknowledged")
	}
	if m, err := s.Metadata(ctx); err != nil || m.StoreRevision != 0 || m.RecordCount != 0 || m.ContentBytes != 0 {
		t.Fatalf("commit failure retained counters/revision: %+v %v", m, err)
	}
	for _, table := range []string{"memory_records", "memory_ownership", "memory_operations", "memory_fts", "memory_supersedes", "issue58_commit_guard"} {
		if n := countTableRows(t, s, table); n != 0 {
			t.Fatalf("commit failure retained %s: %d", table, n)
		}
	}
	if _, err := s.db.Exec(`DROP TRIGGER issue58_commit_failure`); err != nil {
		t.Fatal(err)
	}
	out, err := s.MutateBound(ctx, issue58Owner, request)
	if err != nil {
		t.Fatal(err)
	}
	legacy := sessionScopedRecord("legacy-session", machineMemoryScope, session, "legacy")
	legacy.Scope.TenantID = "local"
	if _, err := s.Put(ctx, legacy, 1, nil); err != nil {
		t.Fatal(err)
	}
	if n, err := s.GCSessionScoped(ctx, session); err != nil || n != 1 {
		t.Fatalf("GC touched owned rows: %d %v", n, err)
	}
	if n, err := s.GCStaleSessionScoped(ctx, nil); err != nil || n != 0 {
		t.Fatalf("stale GC touched owned rows: %d %v", n, err)
	}
	if got, err := s.GetBound(ctx, issue58Owner, request.Record.ID, 1); err != nil || got.Record.ID != out.RecordID {
		t.Fatalf("GC lost bound original: %+v %v", got, err)
	}
	if hits, err := s.SearchBound(ctx, issue58Owner, SearchQuery{Text: "needle", Scope: machineScope(&session), Limit: 10}); err != nil || len(hits) != 1 {
		t.Fatalf("GC lost bound FTS: %+v %v", hits, err)
	}
}

func TestIssue58ReadOnlyHandleCannotRestoreOverDatabase(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	s, err := Open(ctx, dir, machineMemoryScope)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.MutateBound(ctx, issue58Owner, issue58Create("read-only")); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	backup := filepath.Join(t.TempDir(), storeFileName)
	issue58SchemaOne(t, backup, machineMemoryScope)
	path := filepath.Join(dir, storeFileName)
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	ro, err := OpenExisting(ctx, dir, machineMemoryScope)
	if err != nil {
		t.Fatal(err)
	}
	if restored, err := ro.Restore(ctx, backup); err == nil {
		_ = restored.Close()
		t.Error("read-only handle replaced/upgraded its database")
	}
	_ = ro.Close()
	if after, err := os.ReadFile(path); err != nil || !reflect.DeepEqual(before, after) {
		t.Fatalf("readonly restore mutated original database: %v", err)
	}
}
