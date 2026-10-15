package bml

import (
	"context"
	"path/filepath"
	"testing"
)

type issue58OperationReader interface {
	LookupBoundOperation(context.Context, OwnershipBinding, string) (*MutationOutcome, error)
}

func operationReader(t *testing.T, value any) issue58OperationReader {
	t.Helper()
	r, ok := value.(issue58OperationReader)
	if !ok {
		t.Fatal("native operation-only lookup unavailable; Garden status cannot recover its retained digest")
	}
	return r
}
func TestIssue58BoundOperationLookupDurable(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	s, err := Open(ctx, dir, machineMemoryScope)
	if err != nil {
		t.Fatal(err)
	}
	create := issue58Create("lookup")
	first, err := s.MutateBound(ctx, issue58Owner, create)
	if err != nil {
		t.Fatal(err)
	}
	update := create
	update.Operation = "update"
	update.OperationID = "update-lookup"
	update.ExpectedRecordRevision = &first.RecordRevision
	update.Record.Content = "changed"
	update.Record.Provenance.ContentDigest = MemoryContentDigest([]byte("changed"))
	second, err := s.MutateBound(ctx, issue58Owner, update)
	if err != nil {
		t.Fatal(err)
	}
	tomb := BoundMutation{Operation: "tombstone", OperationID: "remove-lookup", PayloadDigest: "remove", Record: tombstoneRecord("lookup-removed", machineMemoryScope, "lookup"), ExpectedTargetRevision: &second.RecordRevision}
	tomb.Record.Scope = machineScope(nil)
	removed, err := s.MutateBound(ctx, issue58Owner, tomb)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	ro, err := OpenExisting(ctx, dir, machineMemoryScope)
	if err != nil {
		t.Fatal(err)
	}
	defer ro.Close()
	reader := operationReader(t, ro)
	for _, want := range []MutationOutcome{first, second, removed} {
		got, err := reader.LookupBoundOperation(ctx, issue58Owner, want.OperationID)
		if err != nil || got == nil || *got != want {
			t.Fatalf("immutable status=%+v %v want %+v", got, err, want)
		}
	}
	if got, err := reader.LookupBoundOperation(ctx, issue58Owner, "absent"); err != nil || got != nil {
		t.Fatalf("absence=%+v %v", got, err)
	}
	for _, owner := range []OwnershipBinding{{SubjectID: "other", DomainID: issue58Owner.DomainID, ScopeKind: "personal", DestinationID: issue58Owner.DestinationID}, {SubjectID: issue58Owner.SubjectID, DomainID: issue58Owner.DomainID, ScopeKind: "personal", DestinationID: "other"}} {
		if got, err := reader.LookupBoundOperation(ctx, owner, first.OperationID); err == nil || got != nil {
			t.Fatalf("foreign outcome disclosed=%+v %v", got, err)
		}
	}
	if _, err := ro.LookupBoundMutation(ctx, issue58Owner, first.OperationID, "wrong"); err == nil {
		t.Fatal("strict wrong digest accepted")
	}
	if _, err := ro.LookupBoundMutation(ctx, issue58Owner, first.OperationID, ""); err == nil {
		t.Fatal("strict empty digest became wildcard")
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if got, err := reader.LookupBoundOperation(canceled, issue58Owner, first.OperationID); err == nil || got != nil {
		t.Fatalf("canceled=%+v %v", got, err)
	}
	for _, id := range []string{"", string([]byte{0xff})} {
		if got, err := reader.LookupBoundOperation(ctx, issue58Owner, id); err == nil || got != nil {
			t.Fatalf("invalid id=%+v %v", got, err)
		}
	}
	bad := issue58Owner
	bad.DomainID = string([]byte{0xff})
	if got, err := reader.LookupBoundOperation(ctx, bad, first.OperationID); err == nil || got != nil {
		t.Fatalf("invalid owner=%+v %v", got, err)
	}
}
func TestIssue58BoundOperationLookupLegacyAndCorrupt(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	issue58SchemaOne(t, filepath.Join(dir, storeFileName), machineMemoryScope)
	ro, err := OpenExisting(ctx, dir, machineMemoryScope)
	if err != nil {
		t.Fatal(err)
	}
	defer ro.Close()
	if got, err := operationReader(t, ro).LookupBoundOperation(ctx, issue58Owner, "old"); err == nil || got != nil {
		t.Fatalf("legacy fabricated capability=%+v %v", got, err)
	}
	s := openTestStore(t, machineMemoryScope)
	m := issue58Create("corrupt-lookup")
	if _, err := s.MutateBound(ctx, issue58Owner, m); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(`UPDATE memory_operations SET outcome_json='{}' WHERE operation_id=?`, m.OperationID); err != nil {
		t.Fatal(err)
	}
	if got, err := operationReader(t, s).LookupBoundOperation(ctx, issue58Owner, m.OperationID); err == nil || got != nil {
		t.Fatalf("corrupt outcome=%+v %v", got, err)
	}
}
func TestIssue58BoundOperationLookupHome(t *testing.T) {
	ctx := context.Background()
	h := NewHome(t.TempDir())
	defer h.Close()
	reader := operationReader(t, h)
	if got, err := reader.LookupBoundOperation(ctx, issue58Owner, "missing"); err == nil || got != nil {
		t.Fatalf("missing database=%+v %v", got, err)
	}
	m := issue58Create("home-lookup")
	want, err := h.MutateBound(ctx, issue58Owner, m)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := reader.LookupBoundOperation(ctx, issue58Owner, m.OperationID); err != nil || got == nil || *got != want {
		t.Fatalf("Home status=%+v %v", got, err)
	}
}
