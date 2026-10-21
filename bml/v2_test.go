package bml

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestV2RecordContainsNoDigestFields(t *testing.T) {
	raw, err := json.Marshal(testRecord())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "digest") || strings.Contains(string(raw), `"hash"`) {
		t.Fatalf("retired fields in record: %s", raw)
	}
}

func TestV2FreshSchema(t *testing.T) {
	s, err := Open(context.Background(), t.TempDir(), "owner")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	meta, err := s.Metadata(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if meta.SchemaVersion != 2 {
		t.Fatalf("schema = %d, want 2", meta.SchemaVersion)
	}
	var count int
	if err := s.db.QueryRow("SELECT count(*) FROM sqlite_master WHERE name = 'memory_apply_journal'").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatal("obsolete journal exists")
	}
}

func TestV2RefusesOldAndUnknownStoresBeforeWrite(t *testing.T) {
	for _, tc := range []struct{ name, ddl, code string }{
		{"partial_old", "CREATE TABLE schema_meta(component TEXT PRIMARY KEY, schema_version INTEGER, store_revision INTEGER, record_count INTEGER, content_bytes INTEGER, workspace_id TEXT); INSERT INTO schema_meta VALUES ('embedded_laputa',1,0,0,0,'owner')", "unsupported_source_format"},
		{"unknown", "CREATE TABLE foreign_data(value TEXT)", "unsupported_source_format"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, storeFileName)
			db, err := sql.Open("sqlite", path)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := db.Exec(tc.ddl); err != nil {
				t.Fatal(err)
			}
			db.Close()
			before, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			s, err := Open(context.Background(), dir, "owner")
			if s != nil {
				s.Close()
			}
			var se *StoreError
			if !errors.As(err, &se) || string(se.Code) != tc.code {
				t.Fatalf("got %v, want %s", err, tc.code)
			}
			after, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if string(before) != string(after) {
				t.Fatal("refused source was modified")
			}
		})
	}
}

func TestV2CanonicalOwnershipSurvivesRelocation(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	first := filepath.Join(root, "before")
	s, err := OpenCanonical(ctx, first)
	if err != nil {
		t.Fatal(err)
	}
	meta, err := s.Metadata(ctx)
	if err != nil {
		t.Fatal(err)
	}
	s.Close()
	second := filepath.Join(root, "after")
	if err := os.Rename(first, second); err != nil {
		t.Fatal(err)
	}
	s, err = OpenExistingCanonical(ctx, second)
	if err != nil {
		t.Fatalf("relocated home: %v", err)
	}
	defer s.Close()
	moved, err := s.Metadata(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if moved.WorkspaceID != meta.WorkspaceID {
		t.Fatal("relocation changed owner")
	}
}

func TestV2ImportAllocationsSurviveRestartAndFailure(t *testing.T) {
	root := t.TempDir()
	ctx := migrationContext()
	migration := NewMemoryMigration(root)
	records := AdaptLegacyMarkdown("MEMORY.md", "body", KindLongTerm, ctx, true).Records
	plan, err := migration.DryRun("retry", []byte("raw\r\nbody"), records, ts(0))
	if err != nil {
		t.Fatal(err)
	}
	failure := MemoryMigrationTestFailureAfterRecords
	if _, err := migration.ExecuteWithFailure(plan, &failure); err == nil {
		t.Fatal("fault not injected")
	}
	restarted := NewMemoryMigration(root)
	nextRecords := AdaptLegacyMarkdown("MEMORY.md", "body", KindLongTerm, ctx, true).Records
	next, err := restarted.DryRun("retry", []byte("raw\r\nbody"), nextRecords, ts(5))
	if err != nil {
		t.Fatal(err)
	}
	if next.Records[0].ID != plan.Records[0].ID {
		t.Fatal("retry allocated a second identity")
	}
	if _, err := restarted.Execute(next); err != nil {
		t.Fatal(err)
	}
	if _, err := restarted.DryRun("retry", []byte("raw\nbody"), nextRecords, ts(0)); err == nil {
		t.Fatal("changed raw input reused migration")
	}
	nextRecords[0].Content = "changed"
	if _, err := restarted.DryRun("retry", []byte("raw\r\nbody"), nextRecords, ts(0)); err == nil {
		t.Fatal("changed normalized input reused migration")
	}
}

func TestV2ImportDoesNotOverwriteUnownedOrPartialOutputs(t *testing.T) {
	root := t.TempDir()
	migration := NewMemoryMigration(root)
	plan, err := migration.DryRun("taken", []byte("input"), nil, ts(0))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(plan.Directory, 0700); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(plan.Directory, "records.json")
	if err := os.WriteFile(marker, []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := migration.Execute(plan); err == nil {
		t.Fatal("unowned target was overwritten")
	}
	raw, err := os.ReadFile(marker)
	if err != nil || string(raw) != "keep" {
		t.Fatal("unowned target changed")
	}
}

func TestV2RefusesPartialAndFutureTargetShapes(t *testing.T) {
	for _, mutation := range []string{"DROP TABLE memory_supersedes", "ALTER TABLE schema_meta ADD COLUMN checksum TEXT", "UPDATE schema_meta SET schema_version=99", "CREATE TABLE unexplained(value TEXT)"} {
		t.Run(mutation, func(t *testing.T) {
			dir := t.TempDir()
			s, err := Open(context.Background(), dir, "owner")
			if err != nil {
				t.Fatal(err)
			}
			if _, err := s.db.Exec(mutation); err != nil {
				t.Fatal(err)
			}
			s.Close()
			reopened, err := Open(context.Background(), dir, "owner")
			if reopened != nil {
				reopened.Close()
			}
			var se *StoreError
			if !errors.As(err, &se) || se.Code != ErrUnsupportedSourceFormat {
				t.Fatalf("unexpected refusal: %v", err)
			}
		})
	}
}

func TestV2RecognizedOldFormatIsUpgradeRequired(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	s, err := Open(ctx, dir, "owner")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(`CREATE TABLE memory_apply_journal(idempotency_key TEXT PRIMARY KEY,proposal_id TEXT NOT NULL,request_id TEXT NOT NULL,content_digest TEXT NOT NULL,record_id TEXT NOT NULL,store_revision INTEGER NOT NULL,record_revision INTEGER NOT NULL,actor_id TEXT NOT NULL,applied_at TEXT NOT NULL); UPDATE schema_meta SET schema_version=1`); err != nil {
		t.Fatal(err)
	}
	s.Close()
	before, err := os.ReadFile(filepath.Join(dir, storeFileName))
	if err != nil {
		t.Fatal(err)
	}
	for _, open := range []func() (*Store, error){func() (*Store, error) { return Open(ctx, dir, "owner") }, func() (*Store, error) { return OpenExistingCanonical(ctx, dir) }} {
		opened, err := open()
		if opened != nil {
			opened.Close()
		}
		var se *StoreError
		if !errors.As(err, &se) || se.Code != ErrUpgradeRequired {
			t.Fatalf("got %v", err)
		}
	}
	after, err := os.ReadFile(filepath.Join(dir, storeFileName))
	if err != nil || string(before) != string(after) {
		t.Fatal("old source changed")
	}
}

func TestV2ForeignOwnerIsNotAdopted(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(context.Background(), dir, "original-owner")
	if err != nil {
		t.Fatal(err)
	}
	s.Close()
	_, err = OpenExisting(context.Background(), dir, "different-owner")
	var se *StoreError
	if !errors.As(err, &se) || se.Code != ErrDatabaseWorkspaceMismatch {
		t.Fatalf("got %v", err)
	}
}

func TestV2LegacyNestedFieldsAreRejected(t *testing.T) {
	for _, retired := range []string{"content_digest", "reason_digest", "hash"} {
		var value any
		switch retired {
		case "content_digest":
			value = &Provenance{}
		case "reason_digest":
			value = &Tombstone{}
		case "hash":
			value = &EvidenceRef{}
		}
		if err := json.Unmarshal([]byte(`{"`+retired+`":null}`), value); err == nil {
			t.Fatalf("legacy %s accepted", retired)
		}
	}
}

func TestV2ArtifactRollbackRejectsLinkedDirectory(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	migration := NewMemoryMigration(root)
	plan, err := NewMemoryMigration(filepath.Dir(outside)).DryRun(filepath.Base(outside), []byte("input"), nil, ts(0))
	if err != nil {
		t.Fatal(err)
	}
	// Populate a valid-looking receipt in an unrelated directory.
	plan.Rollback.MigrationID = "linked"
	if err := atomicWriteJSON(filepath.Join(outside, "rollback.json"), plan.Rollback); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(outside, "records.json")
	if err := os.WriteFile(marker, []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "linked")); err != nil {
		t.Skipf("symlink unavailable on this host: %v", err)
	}
	if err := migration.Rollback("linked"); err == nil {
		t.Fatal("linked rollback succeeded")
	}
	if raw, err := os.ReadFile(marker); err != nil || string(raw) != "keep" {
		t.Fatal("foreign home modified")
	}
}

func TestV2CompletedImportVerifiesRollbackProjection(t *testing.T) {
	migration := NewMemoryMigration(t.TempDir())
	plan, err := migration.DryRun("complete", []byte("input"), nil, ts(0))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := migration.Execute(plan); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(plan.Directory, "rollback.json"), []byte(`{"schema_version":"2.0.0","migration_id":"complete","outputs":["../foreign"]}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := migration.Execute(plan); err == nil {
		t.Fatal("corrupt rollback projection accepted")
	}
}

func TestV2RejectsIncompleteTargetBeforeWritableOpen(t *testing.T) {
	for _, mutation := range []string{
		`UPDATE memory_records SET record_json=json_set(record_json,'$.provenance.content_digest',json('{"algorithm":"sha256","value":"old"}'))`,
		`DROP TABLE memory_fts; CREATE TABLE memory_fts(memory_id,tenant_id,workspace_id,session_id,content)`,
		`DROP TABLE memory_supersedes; CREATE TABLE memory_supersedes(memory_id TEXT NOT NULL,superseded_id TEXT NOT NULL)`,
	} {
		t.Run(mutation, func(t *testing.T) {
			dir := t.TempDir()
			ctx := context.Background()
			s, err := Open(ctx, dir, "owner")
			if err != nil {
				t.Fatal(err)
			}
			mustPut(t, s, storeRecord("one", "owner", "body"), 0)
			if _, err := s.db.Exec(mutation); err != nil {
				t.Fatal(err)
			}
			s.Close()
			path := filepath.Join(dir, storeFileName)
			db, err := sql.Open("sqlite", path)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := db.Exec("PRAGMA journal_mode=DELETE"); err != nil {
				t.Fatal(err)
			}
			db.Close()
			before, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			reopened, err := Open(ctx, dir, "owner")
			if reopened != nil {
				reopened.Close()
			}
			if err == nil {
				t.Fatal("partial target accepted")
			}
			after, err := os.ReadFile(path)
			if err != nil || string(before) != string(after) {
				t.Fatal("refused target mutated")
			}
		})
	}
}

func TestV2RollbackAfterInterruptedPublicationAndDeletion(t *testing.T) {
	for _, failure := range []MemoryMigrationTestFailure{MemoryMigrationTestFailureAfterRecords, MemoryMigrationTestFailureAfterRollbackManifest} {
		migration := NewMemoryMigration(t.TempDir())
		plan, err := migration.DryRun("rollback", []byte("input"), nil, ts(0))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := migration.ExecuteWithFailure(plan, &failure); err == nil {
			t.Fatal("fault missing")
		}
		if err := migration.Rollback("rollback"); err != nil {
			t.Fatalf("rollback allocated: %v", err)
		}
	}
	migration := NewMemoryMigration(t.TempDir())
	plan, err := migration.DryRun("rollback", []byte("input"), nil, ts(0))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := migration.Execute(plan); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(plan.Directory, "records.json")); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(plan.Directory, "rollback.json")); err != nil {
		t.Fatal(err)
	}
	if err := migration.Rollback("rollback"); err != nil {
		t.Fatalf("rollback deletion retry: %v", err)
	}
}
