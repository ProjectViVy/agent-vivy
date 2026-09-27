package bml

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

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

func openTestStore(t *testing.T, workspace string) *Store {
	t.Helper()
	s, err := Open(context.Background(), t.TempDir(), workspace)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func storeRecord(id, workspace, content string) Record {
	now := time.Now().UTC().Add(-time.Minute)
	return Record{
		ID:      id,
		Kind:    KindLongTerm,
		Content: content,
		Provenance: Provenance{
			Source:        ProvenanceSourceUserInput,
			SourceID:      "user-1",
			ContentDigest: MemoryContentDigest([]byte(content)),
			CapturedAt:    now,
			Correlation: AuditCorrelation{
				RequestID: "request-1",
				TurnID:    "turn-1",
				SessionID: "session-1",
			},
		},
		EvidenceRefs:  []EvidenceRef{},
		ConfidenceBPS: 9_000,
		Sensitivity:   SensitivityInternal,
		Trust:         TrustUserAsserted,
		Scope:         Scope{TenantID: "tenant-1", WorkspaceID: workspace},
		CreatedAt:     now,
		EffectiveAt:   now,
		Supersedes:    []string{},
	}
}

func tombstoneRecord(id, workspace, target string) Record {
	rec := storeRecord(id, workspace, "")
	rec.Provenance.ContentDigest = MemoryContentDigest(nil)
	rec.Supersedes = []string{target}
	rec.Tombstone = &Tombstone{
		TargetRecordID: target,
		ReasonDigest:   MemoryContentDigest([]byte("removed")),
		ActorID:        "user-1",
		CreatedAt:      time.Now().UTC().Add(-time.Minute),
	}
	return rec
}

func ftsRows(t *testing.T, s *Store, memoryID string) int {
	t.Helper()
	var n int
	err := s.db.QueryRowContext(context.Background(),
		"SELECT COUNT(*) FROM memory_fts WHERE memory_id = ?", memoryID).Scan(&n)
	if err != nil {
		t.Fatalf("count fts rows for %s: %v", memoryID, err)
	}
	return n
}

func TestPutGetListRoundTrip(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t, "workspace-1")

	storedB, err := s.Put(ctx, storeRecord("b", "workspace-1", "beta"), 0, nil)
	if err != nil {
		t.Fatalf("Put b: %v", err)
	}
	if storedB.Revision != 1 {
		t.Fatalf("b revision = %d, want 1", storedB.Revision)
	}
	storedA, err := s.Put(ctx, storeRecord("a", "workspace-1", "alpha"), 1, nil)
	if err != nil {
		t.Fatalf("Put a: %v", err)
	}
	if storedA.Revision != 1 {
		t.Fatalf("a revision = %d, want 1", storedA.Revision)
	}

	got, err := s.Get(ctx, "b")
	if err != nil {
		t.Fatalf("Get b: %v", err)
	}
	if got == nil || got.Record.Content != "beta" || got.Revision != 1 {
		t.Fatalf("Get b = %+v", got)
	}
	missing, err := s.Get(ctx, "absent")
	if err != nil {
		t.Fatalf("Get absent: %v", err)
	}
	if missing != nil {
		t.Fatalf("Get absent = %+v, want nil", missing)
	}

	list, err := s.List(ctx, 10)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	ids := make([]string, len(list))
	for i, r := range list {
		ids[i] = r.Record.ID
	}
	if !reflect.DeepEqual(ids, []string{"a", "b"}) {
		t.Fatalf("List order = %v, want [a b]", ids)
	}
	limited, err := s.List(ctx, 1)
	if err != nil {
		t.Fatalf("List limit: %v", err)
	}
	if len(limited) != 1 || limited[0].Record.ID != "a" {
		t.Fatalf("List limit = %+v", limited)
	}

	m, err := s.Metadata(ctx)
	if err != nil {
		t.Fatalf("Metadata: %v", err)
	}
	wantBytes := int64(len("beta") + len("alpha"))
	if m.StoreRevision != 2 || m.RecordCount != 2 || m.ContentBytes != wantBytes {
		t.Fatalf("metadata = %+v, want revision 2, count 2, bytes %d", m, wantBytes)
	}
	if ftsRows(t, s, "a") != 1 || ftsRows(t, s, "b") != 1 {
		t.Fatal("missing FTS rows after put")
	}
}

func TestPutUpdateAndRevisionConflicts(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t, "workspace-1")

	if _, err := s.Put(ctx, storeRecord("a", "workspace-1", "alpha"), 0, nil); err != nil {
		t.Fatalf("Put a: %v", err)
	}
	one := int64(1)
	updated, err := s.Put(ctx, storeRecord("a", "workspace-1", "alpha v2"), 1, &one)
	if err != nil {
		t.Fatalf("update a: %v", err)
	}
	if updated.Revision != 2 {
		t.Fatalf("updated revision = %d, want 2", updated.Revision)
	}
	got, err := s.Get(ctx, "a")
	if err != nil || got.Record.Content != "alpha v2" {
		t.Fatalf("Get a = %+v, %v", got, err)
	}

	stale, err := s.Put(ctx, storeRecord("a", "workspace-1", "alpha v3"), 0, nil)
	if code := storeErrCode(t, err); code != ErrStoreRevisionConflict {
		t.Fatalf("stale store revision: code = %q (stored %+v)", code, stale)
	}
	_, err = s.Put(ctx, storeRecord("a", "workspace-1", "alpha v3"), 2, nil)
	if code := storeErrCode(t, err); code != ErrRecordRevisionConflict {
		t.Fatalf("expected-revision none on existing row: code = %q", code)
	}
	zero := int64(0)
	_, err = s.Put(ctx, storeRecord("a", "workspace-1", "alpha v3"), 2, &zero)
	if code := storeErrCode(t, err); code != ErrRecordRevisionConflict {
		t.Fatalf("stale record revision: code = %q", code)
	}
	m, err := s.Metadata(ctx)
	if err != nil {
		t.Fatalf("Metadata: %v", err)
	}
	if m.StoreRevision != 2 || m.RecordCount != 1 {
		t.Fatalf("metadata after failed puts = %+v, want revision 2 count 1", m)
	}
	if got, _ := s.Get(ctx, "a"); got.Record.Content != "alpha v2" {
		t.Fatalf("content after failed puts = %q", got.Record.Content)
	}
}

func TestPutWorkspaceMismatchAndInvalidRecord(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t, "workspace-1")

	_, err := s.Put(ctx, storeRecord("x", "workspace-2", "forbidden"), 0, nil)
	if code := storeErrCode(t, err); code != ErrWorkspaceMismatch {
		t.Fatalf("cross-workspace put: code = %q", code)
	}

	invalid := storeRecord("bad", "workspace-1", "secret sentinel content")
	invalid.Provenance.ContentDigest = MemoryContentDigest([]byte("different"))
	_, err = s.Put(ctx, invalid, 0, nil)
	if code := storeErrCode(t, err); code != ErrInvalidRecord {
		t.Fatalf("invalid record: code = %q", code)
	}
	if strings.Contains(err.Error(), "secret sentinel content") {
		t.Fatalf("error leaked content: %q", err.Error())
	}
	m, err := s.Metadata(ctx)
	if err != nil {
		t.Fatalf("Metadata: %v", err)
	}
	if m.RecordCount != 0 || m.StoreRevision != 0 {
		t.Fatalf("invalid write was not atomic: %+v", m)
	}
}

func TestTombstoneHidesContentAndIndexesSupersede(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t, "workspace-1")

	if _, err := s.Put(ctx, storeRecord("old", "workspace-1", "needle private value"), 0, nil); err != nil {
		t.Fatalf("Put old: %v", err)
	}
	if ftsRows(t, s, "old") != 1 {
		t.Fatal("precondition: old should be indexed")
	}
	if err := s.PutTombstone(ctx, tombstoneRecord("forget-old", "workspace-1", "old"), 1, "old", 1); err != nil {
		t.Fatalf("PutTombstone: %v", err)
	}

	got, err := s.Get(ctx, "old")
	if err != nil || got == nil {
		t.Fatalf("Get old: %+v, %v", got, err)
	}
	tomb, err := s.Get(ctx, "forget-old")
	if err != nil || tomb == nil {
		t.Fatalf("Get tombstone: %+v, %v", tomb, err)
	}
	if tomb.Record.Tombstone == nil || tomb.Record.Content != "" {
		t.Fatalf("tombstone record = %+v", tomb.Record)
	}
	if ftsRows(t, s, "old") != 0 {
		t.Fatal("tombstone left searchable FTS content for target")
	}
	if ftsRows(t, s, "forget-old") != 0 {
		t.Fatal("tombstone record itself got an FTS row")
	}
	targets, err := s.SupersededTargetIDs(ctx)
	if err != nil {
		t.Fatalf("SupersededTargetIDs: %v", err)
	}
	if _, ok := targets["old"]; !ok || len(targets) != 1 {
		t.Fatalf("SupersededTargetIDs = %v, want {old}", targets)
	}
	list, err := s.List(ctx, 10)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("List len = %d, want 2 (tombstone keeps record row)", len(list))
	}
}

func TestPutTombstoneGuardsTargetRevision(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t, "workspace-1")

	if _, err := s.Put(ctx, storeRecord("old", "workspace-1", "value"), 0, nil); err != nil {
		t.Fatalf("Put old: %v", err)
	}
	err := s.PutTombstone(ctx, tombstoneRecord("t1", "workspace-1", "old"), 1, "old", 99)
	if code := storeErrCode(t, err); code != ErrRecordRevisionConflict {
		t.Fatalf("stale target revision: code = %q", code)
	}
	err = s.PutTombstone(ctx, tombstoneRecord("t1", "workspace-1", "missing"), 1, "missing", 1)
	if code := storeErrCode(t, err); code != ErrRecordRevisionConflict {
		t.Fatalf("missing target: code = %q", code)
	}
	if err := s.PutTombstone(ctx, tombstoneRecord("t1", "workspace-1", "old"), 1, "old", 1); err != nil {
		t.Fatalf("guarded PutTombstone: %v", err)
	}
}

func TestTombstoneSuppressionSurvivesTargetRewrite(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t, "workspace-1")

	if _, err := s.Put(ctx, storeRecord("old", "workspace-1", "needle"), 0, nil); err != nil {
		t.Fatalf("Put old: %v", err)
	}
	if _, err := s.Put(ctx, tombstoneRecord("forget-old", "workspace-1", "old"), 1, nil); err != nil {
		t.Fatalf("Put tombstone: %v", err)
	}
	one := int64(1)
	if _, err := s.Put(ctx, storeRecord("old", "workspace-1", "needle rewritten"), 2, &one); err != nil {
		t.Fatalf("rewrite old: %v", err)
	}
	if ftsRows(t, s, "old") != 0 {
		t.Fatal("rewritten record regained FTS row despite superseding tombstone")
	}
}

func TestCapacityLimitsEnforced(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t, "workspace-1")

	if _, err := s.db.ExecContext(ctx,
		"UPDATE schema_meta SET record_count = ? WHERE component = ?", MaxMemoryRecords, component); err != nil {
		t.Fatalf("seed record_count: %v", err)
	}
	_, err := s.Put(ctx, storeRecord("over-count", "workspace-1", "x"), 0, nil)
	if code := storeErrCode(t, err); code != ErrCapacityExceeded {
		t.Fatalf("record capacity: code = %q", code)
	}

	if _, err := s.db.ExecContext(ctx,
		"UPDATE schema_meta SET record_count = 0, content_bytes = ? WHERE component = ?", MaxMemoryContentBytes, component); err != nil {
		t.Fatalf("seed content_bytes: %v", err)
	}
	_, err = s.Put(ctx, storeRecord("over-bytes", "workspace-1", "x"), 0, nil)
	if code := storeErrCode(t, err); code != ErrCapacityExceeded {
		t.Fatalf("content capacity: code = %q", code)
	}

	if _, err := s.db.ExecContext(ctx,
		"UPDATE schema_meta SET content_bytes = 0 WHERE component = ?", component); err != nil {
		t.Fatalf("reset content_bytes: %v", err)
	}
	oversized := storeRecord("oversized", "workspace-1", strings.Repeat("x", int(MaxMemoryContentBytes)+1))
	oversized.Provenance.ContentDigest = MemoryContentDigest([]byte(oversized.Content))
	_, err = s.Put(ctx, oversized, 0, nil)
	if code := storeErrCode(t, err); code != ErrCapacityExceeded {
		t.Fatalf("oversized record: code = %q", code)
	}
}

func TestConcurrentPutHasOneWinner(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t, "workspace-1")

	type result struct{ err error }
	results := make(chan result, 2)
	go func() {
		_, err := s.Put(ctx, storeRecord("left", "workspace-1", "left"), 0, nil)
		results <- result{err}
	}()
	go func() {
		_, err := s.Put(ctx, storeRecord("right", "workspace-1", "right"), 0, nil)
		results <- result{err}
	}()
	var ok, conflicts int
	for i := 0; i < 2; i++ {
		r := <-results
		if r.err == nil {
			ok++
			continue
		}
		var se *StoreError
		if errors.As(r.err, &se) && se.Code == ErrStoreRevisionConflict {
			conflicts++
		} else {
			t.Fatalf("unexpected error: %v", r.err)
		}
	}
	if ok != 1 || conflicts != 1 {
		t.Fatalf("concurrent put: ok=%d conflicts=%d", ok, conflicts)
	}
	m, err := s.Metadata(ctx)
	if err != nil {
		t.Fatalf("Metadata: %v", err)
	}
	if m.StoreRevision != 1 || m.RecordCount != 1 {
		t.Fatalf("metadata = %+v, want revision 1 count 1", m)
	}
}

func TestImportRecords(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t, "workspace-1")

	batch := []Record{
		storeRecord("i-1", "workspace-1", "first"),
		storeRecord("i-2", "workspace-1", "second"),
	}
	m, err := s.ImportRecords(ctx, batch, 0)
	if err != nil {
		t.Fatalf("ImportRecords: %v", err)
	}
	if m.StoreRevision != 2 || m.RecordCount != 2 || m.ContentBytes != int64(len("first")+len("second")) {
		t.Fatalf("metadata after import = %+v", m)
	}
	got, err := s.Get(ctx, "i-1")
	if err != nil || got == nil || got.Revision != 1 || got.Record.Content != "first" {
		t.Fatalf("Get i-1 = %+v, %v", got, err)
	}
	if ftsRows(t, s, "i-1") != 1 {
		t.Fatal("imported record missing FTS row")
	}

	// Idempotent replay: identical canonical JSON is a no-op.
	m, err = s.ImportRecords(ctx, batch, 2)
	if err != nil {
		t.Fatalf("idempotent ImportRecords: %v", err)
	}
	if m.StoreRevision != 2 || m.RecordCount != 2 {
		t.Fatalf("idempotent replay mutated metadata: %+v", m)
	}

	// Store-revision CAS on import.
	_, err = s.ImportRecords(ctx, []Record{storeRecord("i-9", "workspace-1", "nine")}, 1)
	if code := storeErrCode(t, err); code != ErrStoreRevisionConflict {
		t.Fatalf("stale store revision import: code = %q", code)
	}

	// Conflicting id aborts the whole batch.
	conflict := storeRecord("i-1", "workspace-1", "different content")
	fresh := storeRecord("i-3", "workspace-1", "third")
	_, err = s.ImportRecords(ctx, []Record{conflict, fresh}, 2)
	if code := storeErrCode(t, err); code != ErrImportConflict {
		t.Fatalf("conflicting import: code = %q", code)
	}
	m, _ = s.Metadata(ctx)
	if m.RecordCount != 2 {
		t.Fatalf("conflict was not atomic: %+v", m)
	}
	if got, _ := s.Get(ctx, "i-3"); got != nil {
		t.Fatal("conflicting batch partially applied")
	}

	// One invalid record aborts the batch.
	invalid := storeRecord("i-4", "workspace-1", "fourth")
	invalid.Provenance.ContentDigest = MemoryContentDigest([]byte("tampered"))
	_, err = s.ImportRecords(ctx, []Record{invalid, storeRecord("i-5", "workspace-1", "fifth")}, 2)
	if code := storeErrCode(t, err); code != ErrInvalidRecord {
		t.Fatalf("invalid import: code = %q", code)
	}
	m, _ = s.Metadata(ctx)
	if m.RecordCount != 2 {
		t.Fatalf("invalid batch was not atomic: %+v", m)
	}

	// Cross-workspace record aborts the batch.
	_, err = s.ImportRecords(ctx, []Record{storeRecord("i-6", "workspace-2", "sixth")}, 2)
	if code := storeErrCode(t, err); code != ErrWorkspaceMismatch {
		t.Fatalf("cross-workspace import: code = %q", code)
	}

	// Capacity: seeded counters plus batch must not exceed the bound.
	if _, err := s.db.ExecContext(ctx,
		"UPDATE schema_meta SET record_count = ? WHERE component = ?", MaxMemoryRecords-1, component); err != nil {
		t.Fatalf("seed counters: %v", err)
	}
	_, err = s.ImportRecords(ctx, []Record{
		storeRecord("i-7", "workspace-1", "seven"),
		storeRecord("i-8", "workspace-1", "eight"),
	}, 2)
	if code := storeErrCode(t, err); code != ErrCapacityExceeded {
		t.Fatalf("capacity import: code = %q", code)
	}
	m, _ = s.Metadata(ctx)
	if m.RecordCount != MaxMemoryRecords-1 {
		t.Fatalf("capacity failure was not atomic: %+v", m)
	}
}

func TestConcurrentPutAcrossConnections(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	s1, err := Open(ctx, dir, "workspace-1")
	if err != nil {
		t.Fatalf("Open s1: %v", err)
	}
	defer s1.Close()
	s2, err := Open(ctx, dir, "workspace-1")
	if err != nil {
		t.Fatalf("Open s2: %v", err)
	}
	defer s2.Close()

	// Two independent connections race at the same expected store revision;
	// whichever loses the write CAS must surface store_revision_conflict and
	// leave exactly one committed record.
	start := make(chan struct{})
	errs := make([]error, 2)
	var wg sync.WaitGroup
	race := func(s *Store, id string, i int) {
		defer wg.Done()
		<-start
		_, errs[i] = s.Put(ctx, storeRecord(id, "workspace-1", id), 0, nil)
	}
	wg.Add(2)
	go race(s1, "left", 0)
	go race(s2, "right", 1)
	close(start)
	wg.Wait()

	var ok, conflicts int
	for _, err := range errs {
		if err == nil {
			ok++
			continue
		}
		var se *StoreError
		if errors.As(err, &se) && se.Code == ErrStoreRevisionConflict {
			conflicts++
		} else {
			t.Fatalf("unexpected error: %v", err)
		}
	}
	if ok != 1 || conflicts != 1 {
		t.Fatalf("cross-connection put: ok=%d conflicts=%d", ok, conflicts)
	}
	m, err := s1.Metadata(ctx)
	if err != nil {
		t.Fatalf("Metadata: %v", err)
	}
	if m.StoreRevision != 1 || m.RecordCount != 1 {
		t.Fatalf("metadata = %+v, want revision 1 count 1", m)
	}
}

func TestGetCorruptRecord(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t, "workspace-1")

	if _, err := s.Put(ctx, storeRecord("corrupt-me", "workspace-1", "private"), 0, nil); err != nil {
		t.Fatalf("Put: %v", err)
	}
	if _, err := s.db.ExecContext(ctx,
		"UPDATE memory_records SET record_json = '{invalid' WHERE memory_id = ?", "corrupt-me"); err != nil {
		t.Fatalf("corrupt row: %v", err)
	}
	_, err := s.Get(ctx, "corrupt-me")
	if code := storeErrCode(t, err); code != ErrCorruptRecord {
		t.Fatalf("corrupt get: code = %q", code)
	}
	if strings.Contains(err.Error(), "private") {
		t.Fatalf("error leaked content: %q", err.Error())
	}
	_, err = s.List(ctx, 10)
	if code := storeErrCode(t, err); code != ErrCorruptRecord {
		t.Fatalf("corrupt list: code = %q", code)
	}
}

func TestCanonicalJSONMatchesSerdeBytes(t *testing.T) {
	// Hand-written serde_json::to_string(&record) output: struct declaration
	// field order, no HTML escaping, chrono "Z" timestamps with AutoSi
	// fractional groups, and U+2028 preserved verbatim.
	content := "a<b>&\"q\"\u2028end"
	reasonDigest := MemoryContentDigest([]byte("rm"))
	expires := time.Date(2026, 7, 30, 0, 0, 0, 123_456_000, time.UTC)
	rec := Record{
		ID:      "t-1",
		Kind:    KindJournal,
		Content: content,
		Provenance: Provenance{
			Source:        ProvenanceSourceUserInput,
			SourceID:      "u-1",
			ContentDigest: MemoryContentDigest([]byte(content)),
			CapturedAt:    time.Date(2026, 7, 29, 12, 0, 1, 123_000_000, time.UTC),
			Correlation: AuditCorrelation{
				RequestID: "r-1", TurnID: "tn-1", SessionID: "s-1", TraceID: strptr("tr-1"),
			},
		},
		EvidenceRefs: []EvidenceRef{{
			ID: "e-1", Source: EvidenceSourceFile, URI: "file://x",
			Excerpt:   strptr("ex<c"),
			CreatedAt: time.Date(2026, 7, 29, 12, 0, 3, 999_999_999, time.UTC),
		}},
		ConfidenceBPS: 9_000,
		Sensitivity:   SensitivityInternal,
		Trust:         TrustUserAsserted,
		Scope:         Scope{TenantID: "ten-1", WorkspaceID: "ws-1", SessionID: strptr("sess-1")},
		CreatedAt:     time.Date(2026, 7, 29, 12, 0, 1, 123_456_789, time.UTC),
		EffectiveAt:   time.Date(2026, 7, 29, 12, 0, 2, 0, time.UTC),
		ExpiresAt:     &expires,
		Supersedes:    []string{"old-1"},
		Tombstone: &Tombstone{
			TargetRecordID: "old-1",
			ReasonDigest:   reasonDigest,
			ActorID:        "u-1",
			CreatedAt:      time.Date(2026, 7, 29, 13, 0, 0, 500_000_000, time.UTC),
		},
	}

	want := `{"id":"t-1","kind":"journal","content":"a<b>&\"q\"` + "\u2028" + `end",` +
		`"provenance":{"source":"user_input","source_id":"u-1",` +
		`"content_digest":{"algorithm":"sha256","value":"` + MemoryContentDigest([]byte(content)).Value + `"},` +
		`"captured_at":"2026-07-29T12:00:01.123Z",` +
		`"correlation":{"request_id":"r-1","turn_id":"tn-1","session_id":"s-1","trace_id":"tr-1"}},` +
		`"evidence_refs":[{"id":"e-1","source":"file","uri":"file://x","excerpt":"ex<c","hash":null,"created_at":"2026-07-29T12:00:03.999999999Z"}],` +
		`"confidence_bps":9000,"sensitivity":"internal","trust":"user_asserted",` +
		`"scope":{"tenant_id":"ten-1","workspace_id":"ws-1","session_id":"sess-1"},` +
		`"created_at":"2026-07-29T12:00:01.123456789Z","effective_at":"2026-07-29T12:00:02Z","expires_at":"2026-07-30T00:00:00.123456Z",` +
		`"supersedes":["old-1"],` +
		`"tombstone":{"target_record_id":"old-1","reason_digest":{"algorithm":"sha256","value":"` + reasonDigest.Value + `"},"actor_id":"u-1","created_at":"2026-07-29T13:00:00.500Z"}}`

	got, err := marshalCanonical(rec)
	if err != nil {
		t.Fatalf("marshalCanonical: %v", err)
	}
	if string(got) != want {
		t.Fatalf("canonical bytes mismatch:\n got: %s\nwant: %s", got, want)
	}

	// The stored record_json round-trips: decode restores the exact record.
	var decoded Record
	if err := json.Unmarshal(got, &decoded); err != nil {
		t.Fatalf("decode canonical: %v", err)
	}
	if !reflect.DeepEqual(decoded, rec) {
		t.Fatalf("canonical round trip mismatch:\n got: %+v\nwant: %+v", decoded, rec)
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
