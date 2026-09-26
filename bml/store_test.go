package bml

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

func openStoreTables(t *testing.T, path string) map[string]string {
	t.Helper()
	db, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatalf("sql.Open %s: %v", path, err)
	}
	defer db.Close()
	rows, err := db.Query(
		"SELECT name, sql FROM sqlite_master WHERE name IN " +
			"('schema_meta','memory_records','memory_supersedes','memory_apply_journal','memory_fts')")
	if err != nil {
		t.Fatalf("query sqlite_master: %v", err)
	}
	defer rows.Close()
	found := map[string]string{}
	for rows.Next() {
		var name, ddl string
		if err := rows.Scan(&name, &ddl); err != nil {
			t.Fatalf("scan sqlite_master: %v", err)
		}
		found[name] = ddl
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate sqlite_master: %v", err)
	}
	return found
}

func storeErrCode(t *testing.T, err error) StoreErrorCode {
	t.Helper()
	var se *StoreError
	if !errors.As(err, &se) {
		t.Fatalf("expected *StoreError, got %T: %v", err, err)
	}
	return se.Code
}

func TestOpenCreatesDatabaseWithAllTables(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(context.Background(), dir, "ws-1")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer s.Close()

	want := filepath.Join(dir, "memory.sqlite3")
	if s.Path() != want {
		t.Fatalf("Path() = %q, want %q", s.Path(), want)
	}
	if _, err := os.Stat(want); err != nil {
		t.Fatalf("memory.sqlite3 not created: %v", err)
	}

	tables := openStoreTables(t, want)
	for _, name := range []string{
		"schema_meta", "memory_records", "memory_supersedes",
		"memory_apply_journal", "memory_fts",
	} {
		if _, ok := tables[name]; !ok {
			t.Errorf("missing table %s; found %v", name, tables)
		}
	}
	if ddl := tables["memory_fts"]; !strings.Contains(ddl, "fts5") {
		t.Errorf("memory_fts is not an FTS5 table: %q", ddl)
	}
	if ddl := tables["memory_fts"]; !strings.Contains(ddl, "tokenize='unicode61'") {
		t.Errorf("memory_fts missing unicode61 tokenizer: %q", ddl)
	}
}

func TestOpenInitializesMetadata(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(context.Background(), dir, "ws-1")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer s.Close()

	m, err := s.Metadata(context.Background())
	if err != nil {
		t.Fatalf("Metadata: %v", err)
	}
	if m.SchemaVersion != 1 {
		t.Errorf("SchemaVersion = %d, want 1", m.SchemaVersion)
	}
	if m.StoreRevision != 0 {
		t.Errorf("StoreRevision = %d, want 0", m.StoreRevision)
	}
	if m.RecordCount != 0 || m.ContentBytes != 0 {
		t.Errorf("RecordCount/ContentBytes = %d/%d, want 0/0", m.RecordCount, m.ContentBytes)
	}
	if m.WorkspaceID != "ws-1" {
		t.Errorf("WorkspaceID = %q, want ws-1", m.WorkspaceID)
	}
}

func TestReopenExistingDatabase(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(context.Background(), dir, "ws-1")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	reopened, err := Open(context.Background(), dir, "ws-1")
	if err != nil {
		t.Fatalf("reopen Open: %v", err)
	}
	defer reopened.Close()
	m, err := reopened.Metadata(context.Background())
	if err != nil {
		t.Fatalf("Metadata: %v", err)
	}
	if m.SchemaVersion != 1 || m.WorkspaceID != "ws-1" {
		t.Errorf("metadata after reopen = %+v", m)
	}
}

func TestOpenWorkspaceMismatch(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(context.Background(), dir, "ws-1")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	_, err = Open(context.Background(), dir, "ws-2")
	if code := storeErrCode(t, err); code != ErrDatabaseWorkspaceMismatch {
		t.Fatalf("Open with wrong workspace: code = %q, want %q", code, ErrDatabaseWorkspaceMismatch)
	}

	_, err = OpenExisting(context.Background(), dir, "ws-2")
	if code := storeErrCode(t, err); code != ErrDatabaseWorkspaceMismatch {
		t.Fatalf("OpenExisting with wrong workspace: code = %q, want %q", code, ErrDatabaseWorkspaceMismatch)
	}
}

func TestOpenUnsupportedSchema(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(context.Background(), dir, "ws-1")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	db, err := sql.Open("sqlite", "file:"+filepath.Join(dir, "memory.sqlite3"))
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	if _, err := db.Exec(
		"UPDATE schema_meta SET schema_version = 99 WHERE component = 'embedded_laputa'"); err != nil {
		t.Fatalf("bump schema_version: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close raw db: %v", err)
	}

	_, err = Open(context.Background(), dir, "ws-1")
	if code := storeErrCode(t, err); code != ErrUnsupportedSchema {
		t.Fatalf("Open on schema 99: code = %q, want %q", code, ErrUnsupportedSchema)
	}
	_, err = OpenExisting(context.Background(), dir, "ws-1")
	if code := storeErrCode(t, err); code != ErrUnsupportedSchema {
		t.Fatalf("OpenExisting on schema 99: code = %q, want %q", code, ErrUnsupportedSchema)
	}
}

func TestOpenExistingRequiresFile(t *testing.T) {
	dir := t.TempDir()
	_, err := OpenExisting(context.Background(), dir, "ws-1")
	if code := storeErrCode(t, err); code != ErrIO {
		t.Fatalf("OpenExisting on missing db: code = %q, want %q", code, ErrIO)
	}
}

func TestOpenExistingReadsExistingStore(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(context.Background(), dir, "ws-1")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	s, err = OpenExisting(context.Background(), dir, "ws-1")
	if err != nil {
		t.Fatalf("OpenExisting: %v", err)
	}
	defer s.Close()
	m, err := s.Metadata(context.Background())
	if err != nil {
		t.Fatalf("Metadata: %v", err)
	}
	if m.WorkspaceID != "ws-1" || m.SchemaVersion != 1 {
		t.Errorf("metadata = %+v", m)
	}
}

func TestMemoryFTSAcceptsWritesAndQueries(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(context.Background(), dir, "ws-1")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer s.Close()

	ctx := context.Background()
	_, err = s.db.ExecContext(ctx,
		"INSERT INTO memory_fts(memory_id, tenant_id, workspace_id, session_id, content) "+
			"VALUES ('m-1','t-1','ws-1','s-1','hello memory layer')")
	if err != nil {
		t.Fatalf("insert into memory_fts: %v", err)
	}
	var id string
	err = s.db.QueryRowContext(ctx,
		"SELECT memory_id FROM memory_fts WHERE memory_fts MATCH 'memory'").Scan(&id)
	if err != nil {
		t.Fatalf("fts5 MATCH query: %v", err)
	}
	if id != "m-1" {
		t.Fatalf("fts5 hit = %q, want m-1", id)
	}
}

func TestWALAndBusyTimeoutPragmas(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(context.Background(), dir, "ws-1")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer s.Close()

	ctx := context.Background()
	var journal string
	if err := s.db.QueryRowContext(ctx, "PRAGMA journal_mode").Scan(&journal); err != nil {
		t.Fatalf("journal_mode: %v", err)
	}
	if journal != "wal" {
		t.Fatalf("journal_mode = %q, want wal", journal)
	}
	var busy int
	if err := s.db.QueryRowContext(ctx, "PRAGMA busy_timeout").Scan(&busy); err != nil {
		t.Fatalf("busy_timeout: %v", err)
	}
	if busy != 5000 {
		t.Fatalf("busy_timeout = %d, want 5000", busy)
	}
}
