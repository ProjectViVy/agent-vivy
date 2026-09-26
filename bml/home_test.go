package bml

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func homeErrCode(t *testing.T, err error) string {
	t.Helper()
	var he *HomeError
	if !errors.As(err, &he) {
		t.Fatalf("expected *HomeError, got %T: %v", err, err)
	}
	return he.Code()
}

// newSessionRecord builds a valid session-scoped record for GC tests,
// standing in for the checkpoint write path that stays out of Task 6.
func newSessionRecord(t *testing.T, id, sessionID string) Record {
	t.Helper()
	now := time.Now().UTC()
	return Record{
		ID:      id,
		Kind:    KindSessionCheckpoint,
		Content: "checkpoint state",
		Provenance: Provenance{
			Source:        ProvenanceSourceSessionSync,
			SourceID:      "session_checkpoint",
			ContentDigest: MemoryContentDigest([]byte("checkpoint state")),
			CapturedAt:    now,
			Correlation:   correlation("session_checkpoint", sessionID),
		},
		EvidenceRefs:  []EvidenceRef{},
		ConfidenceBPS: MaxConfidenceBPS,
		Sensitivity:   SensitivityInternal,
		Trust:         TrustObserved,
		Scope:         machineScope(&sessionID),
		CreatedAt:     now,
		EffectiveAt:   now,
		Supersedes:    []string{},
	}
}

func putSessionRecord(t *testing.T, h *Home, id, sessionID string) {
	t.Helper()
	store, err := h.writableStore(context.Background())
	if err != nil {
		t.Fatalf("writableStore: %v", err)
	}
	meta, err := store.Metadata(context.Background())
	if err != nil {
		t.Fatalf("metadata: %v", err)
	}
	if _, err := store.Put(context.Background(), newSessionRecord(t, id, sessionID), meta.StoreRevision, nil); err != nil {
		t.Fatalf("put session record: %v", err)
	}
}

func TestHomePathsAndLazyStore(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	config := filepath.Join(root, "config")
	h := NewHome(config)

	if h.ConfigDir() != config {
		t.Fatalf("config dir: %q", h.ConfigDir())
	}
	if want := filepath.Join(config, "memory"); h.MemoryDir() != want {
		t.Fatalf("memory dir: %q want %q", h.MemoryDir(), want)
	}
	if want := filepath.Join(config, "memory", "memory.sqlite3"); h.DatabasePath() != want {
		t.Fatalf("database path: %q want %q", h.DatabasePath(), want)
	}
	if want := filepath.Join(config, "memory", "MEMRULES.MD"); h.MemRulesPath() != want {
		t.Fatalf("memrules path: %q want %q", h.MemRulesPath(), want)
	}

	// No database yet: reads project empty, writes would create it.
	records, err := h.ListRecords(ctx, 20)
	if err != nil || len(records) != 0 {
		t.Fatalf("list on absent store: %v %v", records, err)
	}
	got, err := h.GetRecord(ctx, "memory-1-deadbeef")
	if err != nil || got != nil {
		t.Fatalf("get on absent store: %v %v", got, err)
	}
	if n, err := h.RunStartupGC(ctx, nil); err != nil || n != 0 {
		t.Fatalf("startup GC on absent store: %d %v", n, err)
	}
	if n, err := h.ClearSessionCheckpoint(ctx, "s1"); err != nil || n != 0 {
		t.Fatalf("clear checkpoint on absent store: %d %v", n, err)
	}
	if isFile(h.DatabasePath()) {
		t.Fatal("read paths must not create the database")
	}

	if _, err := h.AddRecord(ctx, KindLongTerm, "remember this", nil); err != nil {
		t.Fatalf("add: %v", err)
	}
	if !isFile(h.DatabasePath()) {
		t.Fatal("add must create the database")
	}
}

func TestHomeLongTermCRUDIsRevisionedAndTombstoned(t *testing.T) {
	ctx := context.Background()
	h := NewHome(t.TempDir())

	added, err := h.AddRecord(ctx, KindLongTerm, "first", nil)
	if err != nil {
		t.Fatalf("add: %v", err)
	}
	if added.Revision != 1 {
		t.Fatalf("first revision: %d", added.Revision)
	}
	if !strings.HasPrefix(added.Record.ID, "memory-") {
		t.Fatalf("id format: %q", added.Record.ID)
	}
	if added.Record.Kind != KindLongTerm ||
		added.Record.Provenance.Source != ProvenanceSourceUserInput ||
		added.Record.Provenance.SourceID != "memory_add" ||
		added.Record.Scope.TenantID != "local" ||
		added.Record.Scope.WorkspaceID != machineMemoryScope ||
		added.Record.Scope.SessionID != nil ||
		added.Record.Trust != TrustUserAsserted ||
		added.Record.Sensitivity != SensitivityInternal ||
		added.Record.ConfidenceBPS != MaxConfidenceBPS {
		t.Fatalf("record shape: %+v", added.Record)
	}
	if added.Record.Provenance.Correlation.RequestID != "memory_add-"+added.Record.ID {
		t.Fatalf("correlation: %+v", added.Record.Provenance.Correlation)
	}

	updated, err := h.UpdateRecord(ctx, added.Record.ID, "second", added.Revision, nil)
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated.Revision != 2 || updated.Record.Content != "second" {
		t.Fatalf("updated: rev=%d content=%q", updated.Revision, updated.Record.Content)
	}
	if updated.Record.Provenance.SourceID != "memory_update" ||
		updated.Record.Provenance.Source != ProvenanceSourceUserInput {
		t.Fatalf("update provenance: %+v", updated.Record.Provenance)
	}
	if !updated.Record.CreatedAt.Equal(added.Record.CreatedAt) {
		t.Fatalf("created_at must be preserved: %v vs %v", updated.Record.CreatedAt, added.Record.CreatedAt)
	}

	err = func() error {
		_, err := h.UpdateRecord(ctx, added.Record.ID, "stale", added.Revision, nil)
		return err
	}()
	if code := homeErrCode(t, err); code != HomeCodeRevisionConflict {
		t.Fatalf("stale update code: %s", code)
	}
	var he *HomeError
	if errors.As(err, &he) && (he.Expected != 1 || he.Actual == nil || *he.Actual != 2) {
		t.Fatalf("revision conflict fields: expected=%d actual=%v", he.Expected, he.Actual)
	}

	if _, err := h.UpdateRecord(ctx, "memory-missing", "x", 1, nil); homeErrCode(t, err) != HomeCodeNotFound {
		t.Fatalf("update missing: %v", err)
	}
	if _, err := h.RemoveRecord(ctx, "memory-missing", "cleanup", 1); homeErrCode(t, err) != HomeCodeNotFound {
		t.Fatalf("remove missing: %v", err)
	}

	removed, err := h.RemoveRecord(ctx, added.Record.ID, "done", updated.Revision)
	if err != nil {
		t.Fatalf("remove: %v", err)
	}
	if removed.Revision != updated.Revision || removed.Record.ID != added.Record.ID {
		t.Fatalf("remove returns the deposed record: %+v", removed)
	}
	if got, err := h.GetRecord(ctx, added.Record.ID); err != nil || got != nil {
		t.Fatalf("get after remove: %v %v", got, err)
	}
	if records, err := h.ListRecords(ctx, 20); err != nil || len(records) != 0 {
		t.Fatalf("list after remove: %v %v", records, err)
	}
	// The deposed target keeps its revision, so a stale base revision still
	// conflicts, while replaying the observed revision is Rust-faithful.
	if _, err := h.RemoveRecord(ctx, added.Record.ID, "again", added.Revision); homeErrCode(t, err) != HomeCodeRevisionConflict {
		t.Fatalf("stale re-remove: %v", err)
	}
	if _, err := h.RemoveRecord(ctx, added.Record.ID, "again", updated.Revision); err != nil {
		t.Fatalf("re-remove at observed revision: %v", err)
	}
}

func TestHomeKindForbiddenAndInvalidRequests(t *testing.T) {
	ctx := context.Background()
	h := NewHome(t.TempDir())

	if _, err := h.AddRecord(ctx, KindIdentity, "persona", nil); homeErrCode(t, err) != HomeCodeKindForbidden {
		t.Fatalf("identity add code: %v", err)
	}
	if isFile(h.DatabasePath()) {
		t.Fatal("forbidden write must not create the database")
	}
	for _, kind := range []Kind{KindPreference, KindSessionCheckpoint, KindDaily} {
		if _, err := h.AddRecord(ctx, kind, "x", nil); homeErrCode(t, err) != HomeCodeKindForbidden {
			t.Fatalf("kind %s code: %v", kind, err)
		}
	}
	if _, err := h.AddRecord(ctx, KindLongTerm, "   \n ", nil); homeErrCode(t, err) != HomeCodeInvalidRequest {
		t.Fatalf("empty content code: %v", err)
	}
	if _, err := h.UpdateRecord(ctx, "memory-x", "  ", 1, nil); homeErrCode(t, err) != HomeCodeInvalidRequest {
		t.Fatalf("empty update content code: %v", err)
	}
	if _, err := h.RemoveRecord(ctx, "memory-x", " ", 1); homeErrCode(t, err) != HomeCodeInvalidRequest {
		t.Fatalf("empty reason code: %v", err)
	}
}

func TestHomeMemRulesDefaultThenFile(t *testing.T) {
	h := NewHome(t.TempDir())

	doc, err := h.ReadMemRules()
	if err != nil {
		t.Fatalf("read default: %v", err)
	}
	if doc.Source != MemRulesSourceDefault || doc.Content != DefaultMemRulesText {
		t.Fatalf("default doc: %+v", doc)
	}
	if isFile(h.MemRulesPath()) {
		t.Fatal("default read must not materialize the file")
	}

	written, err := h.WriteMemRules("# User rules\r\nsecond line\r\n")
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	if written.Source != MemRulesSourceFile || strings.Contains(written.Content, "\r\n") {
		t.Fatalf("written doc: %+v", written)
	}
	reread, err := h.ReadMemRules()
	if err != nil {
		t.Fatalf("re-read: %v", err)
	}
	if reread.Source != MemRulesSourceFile || reread.Content != written.Content {
		t.Fatalf("re-read doc: %+v", reread)
	}

	if _, err := h.WriteMemRules(" \n\t "); homeErrCode(t, err) != HomeCodeInvalidRequest {
		t.Fatalf("empty write code: %v", err)
	}
}

func TestHomeStartupGCAndClearSessionCheckpoint(t *testing.T) {
	ctx := context.Background()
	h := NewHome(t.TempDir())

	putSessionRecord(t, h, "session-checkpoint-a", "gui:active")
	putSessionRecord(t, h, "session-checkpoint-b", "gui:stale")

	store, err := h.writableStore(ctx)
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	alive := func(id string) bool {
		stored, err := store.Get(ctx, id)
		if err != nil {
			t.Fatalf("get %s: %v", id, err)
		}
		return stored != nil
	}

	if n, err := h.RunStartupGC(ctx, []string{"gui:active"}); err != nil || n != 1 {
		t.Fatalf("startup GC: %d %v", n, err)
	}
	if !alive("session-checkpoint-a") || alive("session-checkpoint-b") {
		t.Fatal("startup GC must keep active sessions and remove stale ones")
	}

	// Corrected contract: empty active list means "no active sessions" and
	// removes every session-scoped record.
	putSessionRecord(t, h, "session-checkpoint-c", "gui:orphan")
	if n, err := h.RunStartupGC(ctx, nil); err != nil || n != 2 {
		t.Fatalf("empty startup GC: %d %v", n, err)
	}
	if alive("session-checkpoint-a") || alive("session-checkpoint-c") {
		t.Fatal("empty startup GC must remove all session-scoped records")
	}

	putSessionRecord(t, h, "session-checkpoint-d", "gui:gone")
	if n, err := h.ClearSessionCheckpoint(ctx, "gui:gone"); err != nil || n != 1 {
		t.Fatalf("clear checkpoint: %d %v", n, err)
	}
	if alive("session-checkpoint-d") {
		t.Fatal("clear checkpoint must remove the session record")
	}
	if n, err := h.ClearSessionCheckpoint(ctx, "gui:none"); err != nil || n != 0 {
		t.Fatalf("clear unknown session: %d %v", n, err)
	}
}

func TestHomeListFiltersToVisibleLongTerm(t *testing.T) {
	ctx := context.Background()
	h := NewHome(t.TempDir())

	if _, err := h.AddRecord(ctx, KindLongTerm, "machine fact", nil); err != nil {
		t.Fatalf("add: %v", err)
	}
	putSessionRecord(t, h, "session-checkpoint-s", "gui:s1")

	records, err := h.ListRecords(ctx, 20)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(records) != 1 || records[0].Record.Content != "machine fact" {
		t.Fatalf("list must expose only visible long-term records: %+v", records)
	}
	if got, err := h.GetRecord(ctx, "session-checkpoint-s"); err != nil || got != nil {
		t.Fatalf("session-scoped get must be invisible: %v %v", got, err)
	}
}

func TestHomeStartupIndexTracksAuthority(t *testing.T) {
	ctx := context.Background()
	h := NewHome(t.TempDir())

	pointer := memrulesPointer()
	if got := h.StartupMarkdown(); got != pointer {
		t.Fatalf("initial markdown: %q", got)
	}
	if h.StartupRevision() != 0 {
		t.Fatalf("initial revision: %d", h.StartupRevision())
	}
	if err := h.Warmup(ctx); err != nil {
		t.Fatalf("warmup: %v", err)
	}
	if h.StartupMarkdown() != pointer || h.StartupRevision() != 0 {
		t.Fatal("warmup on empty authority must not change the projection")
	}

	added, err := h.AddRecord(ctx, KindLongTerm, "first memory line\nsecond line", nil)
	if err != nil {
		t.Fatalf("add: %v", err)
	}
	markdown := h.StartupMarkdown()
	if !strings.HasPrefix(markdown, pointer) ||
		!strings.Contains(markdown, "## Long-term Memory Index") ||
		!strings.Contains(markdown, "- ["+added.Record.ID+"] first memory line") {
		t.Fatalf("markdown after add: %q", markdown)
	}
	if h.StartupRevision() != 1 {
		t.Fatalf("revision after add: %d", h.StartupRevision())
	}

	if _, err := h.RemoveRecord(ctx, added.Record.ID, "done", added.Revision); err != nil {
		t.Fatalf("remove: %v", err)
	}
	if h.StartupMarkdown() != pointer {
		t.Fatalf("markdown after remove: %q", h.StartupMarkdown())
	}
	if h.StartupRevision() != 2 {
		t.Fatalf("revision after remove: %d", h.StartupRevision())
	}
}

func TestHomeL1BudgetCapsIndexLines(t *testing.T) {
	ctx := context.Background()
	h := NewHomeWithL1Budget(t.TempDir(), 1)
	for _, content := range []string{"alpha", "beta"} {
		if _, err := h.AddRecord(ctx, KindLongTerm, content, nil); err != nil {
			t.Fatalf("add %q: %v", content, err)
		}
	}
	index := h.StartupMarkdown()
	if strings.Count(index, "- [memory-") != 1 {
		t.Fatalf("L1 budget must cap index lines at 1: %q", index)
	}
}

func TestHomeStoreIdentityMismatchIsBmlUnavailable(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	h := NewHome(dir)
	// Pre-create a store bound to a different workspace identity.
	other, err := Open(ctx, h.MemoryDir(), "other-workspace")
	if err != nil {
		t.Fatalf("seed store: %v", err)
	}
	if err := other.Close(); err != nil {
		t.Fatalf("close seed: %v", err)
	}
	_, err = h.GetRecord(ctx, "memory-x")
	if homeErrCode(t, err) != HomeCodeBmlUnavailable {
		t.Fatalf("identity mismatch code: %v", err)
	}
	var he *HomeError
	if !errors.As(err, &he) {
		t.Fatalf("expected *HomeError: %v", err)
	}
	var se *StoreError
	if !errors.As(he, &se) {
		t.Fatalf("bml_unavailable must wrap the store error: %v", err)
	}
}

func TestHomeOpenExistingDoesNotRunSchemaOnForeignFile(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	h := NewHome(dir)
	if err := os.MkdirAll(h.MemoryDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(h.DatabasePath(), []byte("not sqlite"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := h.ListRecords(ctx, 10); err == nil {
		t.Fatal("corrupt database file must surface an error")
	}
}
