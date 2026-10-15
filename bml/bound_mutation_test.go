package bml

import (
	"context"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
)

var issue58Owner = OwnershipBinding{SubjectID: "subject-a", DomainID: "domain-a", ScopeKind: "personal", DestinationID: "bml-a"}

func issue58Create(id string) BoundMutation {
	r := storeRecord(id, machineMemoryScope, "private continuity needle")
	r.Scope = machineScope(nil)
	r.Kind = KindHistory
	return BoundMutation{Operation: "create", OperationID: "op-" + id, PayloadDigest: "caller-digest-" + id, Record: r}
}

func TestIssue58BoundImmutableNativeOutcomes(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	s, err := Open(ctx, dir, machineMemoryScope)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()
	create := issue58Create("history")
	first, err := s.MutateBound(ctx, issue58Owner, create)
	if err != nil {
		t.Fatal(err)
	}
	if first.RecordID != create.Record.ID || first.RecordRevision != 1 || first.StoreRevision != 1 || first.CanonicalStatus != "completed" || first.IndexStatus != "ready" || first.Binding != issue58Owner {
		t.Fatalf("not actual native outcome: %+v", first)
	}
	got, err := s.GetBound(ctx, issue58Owner, create.Record.ID, 1)
	if err != nil || got.Record.Content != create.Record.Content || !reflect.DeepEqual(got.Record.Provenance, create.Record.Provenance) {
		t.Fatalf("native source lost: %+v %v", got, err)
	}
	update := create
	update.Operation, update.OperationID = "update", "update-history"
	update.Record.Content = "corrected continuity needle"
	update.Record.Provenance.ContentDigest = MemoryContentDigest([]byte(update.Record.Content))
	update.ExpectedRecordRevision = &first.RecordRevision
	second, err := s.MutateBound(ctx, issue58Owner, update)
	if err != nil || second.RecordRevision != 2 {
		t.Fatalf("update: %+v %v", second, err)
	}
	if _, err := s.GetBound(ctx, issue58Owner, create.Record.ID, 1); err == nil {
		t.Fatal("stale export served current bytes")
	}
	if replay, err := s.MutateBound(ctx, issue58Owner, create); err != nil || replay != first {
		t.Fatalf("replay rechecked stale CAS/current row: %+v %v", replay, err)
	}
	tomb := BoundMutation{Operation: "tombstone", OperationID: "remove-history", PayloadDigest: "remove-digest", Record: tombstoneRecord("history-removed", machineMemoryScope, "history"), ExpectedTargetRevision: &second.RecordRevision}
	tomb.Record.Scope = machineScope(nil)
	removed, err := s.MutateBound(ctx, issue58Owner, tomb)
	if err != nil || removed.RecordID != tomb.Record.ID || removed.TargetID != "history" || removed.RecordRevision != 1 || removed.StoreRevision != 3 {
		t.Fatalf("false tombstone result: %+v %v", removed, err)
	}
	if _, err := s.GetBound(ctx, issue58Owner, "history", 2); err == nil {
		t.Fatal("tombstoned original still exported")
	}
	if hits, err := s.SearchBound(ctx, issue58Owner, SearchQuery{Text: "needle", Scope: machineScope(nil), Limit: 10}); err != nil || len(hits) != 0 {
		t.Fatalf("tombstoned original recalled: %+v %v", hits, err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = Open(ctx, dir, machineMemoryScope)
	if err != nil {
		t.Fatal(err)
	}
	for _, original := range []MutationOutcome{first, second, removed} {
		if got, err := s.LookupBoundMutation(ctx, issue58Owner, original.OperationID, original.PayloadDigest); err != nil || got == nil || *got != original {
			t.Fatalf("immutable outcome lost after reopen: %+v %v", got, err)
		}
	}
	if replay, err := s.MutateBound(ctx, issue58Owner, create); err != nil || replay != first {
		t.Fatalf("tombstone/reopen changed original outcome: %+v %v", replay, err)
	}
	if missing, err := s.LookupBoundMutation(ctx, issue58Owner, "never-applied", "opaque"); err != nil || missing != nil {
		t.Fatalf("fabricated missing outcome: %+v %v", missing, err)
	}
}

func TestIssue58BoundMutationAtomicFailureAndConcurrentReplay(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t, machineMemoryScope)
	request := issue58Create("atomic")
	if _, err := s.db.Exec(`CREATE TRIGGER fail_native_outcome BEFORE INSERT ON memory_operations BEGIN SELECT RAISE(ABORT,'injected receipt write failure'); END`); err != nil {
		t.Fatal(err)
	}
	before, err := s.Metadata(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.MutateBound(ctx, issue58Owner, request); err == nil {
		t.Fatal("receipt failure acknowledged")
	}
	if after, err := s.Metadata(ctx); err != nil || after != before {
		t.Fatalf("failed receipt committed counters/CAS: %+v %v", after, err)
	}
	for _, table := range []string{"memory_records", "memory_ownership", "memory_operations", "memory_fts", "memory_supersedes"} {
		if n := countTableRows(t, s, table); n != 0 {
			t.Fatalf("receipt failure left %s rows: %d", table, n)
		}
	}
	if _, err := s.db.Exec(`DROP TRIGGER fail_native_outcome`); err != nil {
		t.Fatal(err)
	}
	other, err := Open(ctx, filepath.Dir(s.Path()), machineMemoryScope)
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	var wg sync.WaitGroup
	results := make(chan MutationOutcome, 8)
	errs := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			handle := s
			if i%2 != 0 {
				handle = other
			}
			out, err := handle.MutateBound(ctx, issue58Owner, request)
			results <- out
			errs <- err
		}(i)
	}
	wg.Wait()
	close(results)
	close(errs)
	for err := range errs {
		if err != nil {
			t.Errorf("same-operation replay failed: %v", err)
		}
	}
	for got := range results {
		if got.RecordRevision != 1 || got.StoreRevision != 1 {
			t.Errorf("same operation wrote twice: %+v", got)
		}
	}
	for _, table := range []string{"memory_records", "memory_ownership", "memory_operations", "memory_fts"} {
		if n := countTableRows(t, s, table); n != 1 {
			t.Errorf("%s count=%d", table, n)
		}
	}
}
