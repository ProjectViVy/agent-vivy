package memory_test

import (
	"context"
	"os"
	"testing"

	"agent-vivy/internal/modules/memory"
	"github.com/ProjectViVy/agent-vivy/bml"
)

// requireSingleEntry asserts a listed outcome with exactly one entry carrying
// the expected native record id.
func requireSingleEntry(t *testing.T, outcome bml.MemoryCrudOutcome, id string) bml.MemoryEntry {
	t.Helper()
	wantStatus(t, outcome, bml.CrudOutcomeListed)
	if len(outcome.Entries) != 1 {
		t.Fatalf("listed entries = %+v, want exactly one record %q", outcome.Entries, id)
	}
	if outcome.Entries[0].ID != id {
		t.Fatalf("listed entry id = %q, want %q", outcome.Entries[0].ID, id)
	}
	return outcome.Entries[0]
}

// requireNoEntry asserts a listed projection does not expose the record id.
func requireNoEntry(t *testing.T, outcome bml.MemoryCrudOutcome, id string) {
	t.Helper()
	wantStatus(t, outcome, bml.CrudOutcomeListed)
	for _, entry := range outcome.Entries {
		if entry.ID == id {
			t.Fatalf("listed entries still expose %q: %+v", id, outcome.Entries)
		}
	}
}

// TestBMLCloseoutPublicDiskLifecycle closes BML-R1/R3 through the public
// action plane: one disk-backed root, real ownership close/reopen between
// every phase, CAS correction, stale-conflict rejection and targeted
// removal. Fixtures stay serial because memory.Active() is process-global.
func TestBMLCloseoutPublicDiskLifecycle(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	cfg := testConfig(root)

	if active := memory.Active(); active != nil {
		t.Fatalf("memory service already active: %p", active)
	}
	open := func() *memory.Service {
		t.Helper()
		service, err := memory.Open(ctx, cfg)
		if err != nil {
			t.Fatalf("Open() error = %v", err)
		}
		if service == nil {
			t.Fatal("Open() returned a nil service")
		}
		return service
	}
	reopen := func(previous *memory.Service) *memory.Service {
		t.Helper()
		if err := memory.Close(); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
		if active := memory.Active(); active != nil {
			t.Fatalf("Active() after Close() = %p, want nil", active)
		}
		next := open()
		if next == previous {
			t.Fatalf("reopen returned the closed service pointer %p", next)
		}
		return next
	}

	first := open()
	t.Cleanup(func() { _ = memory.Close() })

	const (
		oldContent = "bmlcloseout oldvalue synthetic record"
		newContent = "bmlcloseout newvalue synthetic record"
	)

	added := invokeOutcome(t, memory.ActionAdd, `{"kind":"long_term","content":"`+oldContent+`"}`)
	wantStatus(t, added, bml.CrudOutcomeApplied)
	if added.Entry == nil {
		t.Fatalf("add outcome = %+v, want an entry", added)
	}
	id := added.Entry.ID
	if id == "" {
		t.Fatalf("add outcome has an empty record id: %+v", added.Entry)
	}
	if added.Entry.Revision != 1 {
		t.Fatalf("first revision = %d, want 1", added.Entry.Revision)
	}
	if added.Entry.Content != oldContent {
		t.Fatalf("first content = %q, want %q", added.Entry.Content, oldContent)
	}

	databasePath := first.Home().DatabasePath()
	if info, err := os.Stat(databasePath); err != nil || info.IsDir() {
		t.Fatalf("database path %q is not a file: %v", databasePath, err)
	}

	// The closed service must report the explicit unavailable outcome instead
	// of silently reopening the store.
	second := reopen(first)
	staleGet := first.Get(ctx, bml.MemoryGetRequest{RecordID: id})
	wantFailure(t, staleGet, bml.HomeCodeBmlUnavailable)
	if second.Home().DatabasePath() != databasePath {
		t.Fatalf("reopened database path = %q, want %q", second.Home().DatabasePath(), databasePath)
	}

	// No writes between reopen and reads: the durable record must already be
	// visible with its identity, content and revision intact.
	got := requireSingleEntry(t, second.Get(ctx, bml.MemoryGetRequest{RecordID: id}), id)
	if got.Revision != 1 || got.Content != oldContent {
		t.Fatalf("post-reopen get = %+v, want revision 1 with the original content", got)
	}
	searchOld := requireSingleEntry(t, second.Search(ctx, bml.MemorySearchRequest{Query: "oldvalue"}), id)
	if searchOld.Revision != 1 || searchOld.Content != oldContent {
		t.Fatalf("post-reopen search = %+v, want revision 1 with the original content", searchOld)
	}

	// CAS correction at the observed revision wins; both stale mutations must
	// be rejected with the stable conflict reason.
	updated := invokeOutcome(t, memory.ActionUpdate, `{"id":"`+id+`","content":"`+newContent+`","base_revision":1}`)
	wantStatus(t, updated, bml.CrudOutcomeApplied)
	if updated.Entry == nil || updated.Entry.ID != id || updated.Entry.Revision != 2 || updated.Entry.Content != newContent {
		t.Fatalf("update outcome = %+v, want revision 2 with the corrected content", updated)
	}
	wantFailure(t, invokeOutcome(t, memory.ActionUpdate, `{"id":"`+id+`","content":"bmlcloseout stale value","base_revision":1}`), bml.HomeCodeRevisionConflict)
	wantFailure(t, invokeOutcome(t, memory.ActionRemove, `{"id":"`+id+`","reason":"stale removal","base_revision":1}`), bml.HomeCodeRevisionConflict)

	// Reopen again: the persisted winner and the FTS projection must agree.
	third := reopen(second)
	corrected := requireSingleEntry(t, third.Get(ctx, bml.MemoryGetRequest{RecordID: id}), id)
	if corrected.Revision != 2 || corrected.Content != newContent {
		t.Fatalf("post-reopen corrected get = %+v, want revision 2 with the corrected content", corrected)
	}
	requireNoEntry(t, third.Search(ctx, bml.MemorySearchRequest{Query: "oldvalue"}), id)
	if entry := requireSingleEntry(t, third.Search(ctx, bml.MemorySearchRequest{Query: "newvalue"}), id); entry.Revision != 2 {
		t.Fatalf("corrected search revision = %d, want 2", entry.Revision)
	}

	// Targeted removal tombstones the selected long-term record and returns
	// the deposed revision-2 entry, not a fabricated tombstone revision.
	removed := invokeOutcome(t, memory.ActionRemove, `{"id":"`+id+`","reason":"bml closeout synthetic removal","base_revision":2}`)
	wantStatus(t, removed, bml.CrudOutcomeApplied)
	if removed.Entry == nil || removed.Entry.ID != id || removed.Entry.Revision != 2 {
		t.Fatalf("remove outcome = %+v, want the deposed revision-2 entry", removed)
	}

	fourth := reopen(third)
	wantFailure(t, fourth.Get(ctx, bml.MemoryGetRequest{RecordID: id}), bml.HomeCodeNotFound)
	requireNoEntry(t, fourth.List(ctx, bml.MemoryListRequest{}), id)
	requireNoEntry(t, fourth.Search(ctx, bml.MemorySearchRequest{Query: "bmlcloseout"}), id)
	requireNoEntry(t, fourth.Search(ctx, bml.MemorySearchRequest{Query: "newvalue"}), id)
}
