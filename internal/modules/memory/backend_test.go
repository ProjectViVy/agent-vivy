package memory

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"agent-vivy/internal/config"
	"github.com/ProjectViVy/agent-vivy/bml"
	mentlebackend "github.com/ProjectViVy/laputa/garden/backends/mentle"
	gardenmemory "github.com/ProjectViVy/laputa/garden/memory"
	"github.com/ProjectViVy/laputa/garden/memory/memorytest"
	"github.com/ProjectViVy/laputa/laputa/evolution"
	"github.com/ProjectViVy/laputa/mentle/facade"
)

func TestIssue58BMLBackendConformance(t *testing.T) {
	svc, err := Open(context.Background(), config.Config{Storage: config.Storage{DataDir: t.TempDir()}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = Close() })
	memorytest.BackendConformance(t, func(t testing.TB, scope evolution.Scope, dest string) gardenmemory.Backend {
		b, err := NewBoundBackend(svc, bml.OwnershipBinding{SubjectID: scope.SubjectID, DomainID: "test-domain", ScopeKind: string(scope.Kind), WorkspaceID: scope.WorkspaceID, DestinationID: dest})
		if err != nil {
			t.Fatal(err)
		}
		return b
	})
}
func TestIssue58MentleBackendConformance(t *testing.T) {
	svc, err := facade.OpenCatalogService(context.Background(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = svc.Close() })
	memorytest.BackendConformance(t, func(t testing.TB, scope evolution.Scope, dest string) gardenmemory.Backend {
		b, err := mentlebackend.New(svc, scope, dest, []evolution.Scope{{SubjectID: scope.SubjectID, Kind: evolution.ScopePersonal}, scope})
		if err != nil {
			t.Fatal(err)
		}
		return b
	})
}
func TestIssue58BMLNativeCRUDSearchExport(t *testing.T) {
	ctx := context.Background()
	cfg := config.Config{Storage: config.Storage{DataDir: t.TempDir()}}
	svc, err := Open(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = Close() })
	binding := bml.OwnershipBinding{SubjectID: "trusted-subject", DomainID: "trusted-domain", ScopeKind: "personal", DestinationID: "bml"}
	b, err := NewBoundBackend(svc, binding)
	if err != nil {
		t.Fatal(err)
	}
	scope := evolution.Scope{SubjectID: binding.SubjectID, Kind: evolution.ScopePersonal}
	req := gardenmemory.AuthorizedMutation{Scope: scope, DestinationID: binding.DestinationID, OperationID: "create-history", PayloadDigest: "caller", Operation: evolution.MutationCreate, RecordID: "native-history", ExpectedAbsent: true, Body: "exact private needle 😀", Inference: evolution.InferenceObserved, Sources: []evolution.SourceRef{{SourceID: "core-c2", RecordID: "run:7", Revision: 7, Scope: scope}}}
	first, err := b.Mutate(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	native, err := svc.home.ExportBoundSource(ctx, binding, req.RecordID, 1)
	if err != nil {
		t.Fatal(err)
	}
	if first.TargetRef != native.Record.ID || first.Revision != uint64(native.Revision) || native.Record.Content != req.Body || len(native.Record.EvidenceRefs) != 1 {
		t.Fatalf("native material lost: %+v %+v", first, native)
	}
	if !native.Record.CreatedAt.IsZero() || !native.Record.Provenance.CapturedAt.IsZero() {
		t.Fatal("adapter invented source clocks")
	}
	page, err := b.Search(ctx, gardenmemory.AuthorizedSearch{Scopes: []evolution.Scope{scope}, Query: "needle", Limit: 1})
	if err != nil || len(page.Items) != 1 {
		t.Fatalf("search=%+v %v", page, err)
	}
	if strings.Contains(page.Items[0].Summary, "private") || strings.Contains(page.Items[0].Title, "private") {
		t.Fatal("discovery card leaked source body")
	}
	expanded, err := b.Expand(ctx, gardenmemory.AuthorizedExpansion{Scopes: []evolution.Scope{scope}, CardID: req.RecordID, ExpectedRevision: 1, BudgetChars: 100})
	if err != nil || len(expanded.Items) != 1 || expanded.Items[0].Excerpt != req.Body {
		t.Fatalf("exact expansion=%+v %v", expanded, err)
	}
	update := req
	update.Operation = evolution.MutationUpdate
	update.OperationID = "update-history"
	update.ExpectedAbsent = false
	update.ExpectedRevision = 1
	update.Body = "corrected needle"
	second, err := b.Mutate(ctx, update)
	if err != nil || second.Revision != 2 {
		t.Fatalf("update=%+v %v", second, err)
	}
	if _, err := b.Mutate(ctx, gardenmemory.AuthorizedMutation{Scope: scope, DestinationID: "bml", OperationID: "stale", PayloadDigest: "x", Operation: evolution.MutationUpdate, RecordID: req.RecordID, ExpectedRevision: 1, Body: "bad", Inference: evolution.InferenceObserved}); err == nil {
		t.Fatal("stale CAS accepted")
	}
	tomb := update
	tomb.Operation = evolution.MutationTombstone
	tomb.OperationID = "remove-history"
	tomb.ExpectedRevision = 2
	tomb.Body = "removed"
	removed, err := b.Mutate(ctx, tomb)
	if err != nil {
		t.Fatal(err)
	}
	out, err := svc.home.LookupBoundOperation(ctx, binding, tomb.OperationID)
	if err != nil || out == nil || removed.TargetRef != out.RecordID || removed.Revision != uint64(out.RecordRevision) || out.TargetID != req.RecordID {
		t.Fatalf("tombstone result invented: %+v %+v %v", removed, out, err)
	}
	if _, err := b.Expand(ctx, gardenmemory.AuthorizedExpansion{Scopes: []evolution.Scope{scope}, CardID: req.RecordID, ExpectedRevision: 2}); err == nil {
		t.Fatal("deleted original expanded")
	}
	if err := b.Close(); err != nil || !svc.live() {
		t.Fatal("borrowed Close closed App Home")
	}
	if err := Close(); err != nil {
		t.Fatal(err)
	}
	svc, err = Open(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	b, err = NewBoundBackend(svc, binding)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []gardenmemory.MutationReceipt{first, second, removed} {
		got, err := b.MutationStatus(ctx, want.OperationID)
		if err != nil || !reflect.DeepEqual(got, want) {
			t.Fatalf("retained result=%+v %v want %+v", got, err, want)
		}
	}
	if got, err := b.Mutate(ctx, req); err != nil || !reflect.DeepEqual(got, first) {
		t.Fatalf("replay after tombstone/reopen=%+v %v", got, err)
	}
	changed := req
	changed.Body = "changed same digest"
	if _, err := b.Mutate(ctx, changed); err == nil {
		t.Fatal("same digest changed body replay")
	}
	changed = req
	changed.Sources = append([]evolution.SourceRef(nil), req.Sources...)
	changed.Sources[0].Revision++
	if _, err := b.Mutate(ctx, changed); err == nil {
		t.Fatal("same digest changed source replay")
	}
	changed = req
	changed.Inference = evolution.InferenceInferred
	if _, err := b.Mutate(ctx, changed); err == nil {
		t.Fatal("same digest changed inference replay")
	}
	changed = req
	changed.ExpectedRevision = 1
	if _, err := b.Mutate(ctx, changed); err == nil {
		t.Fatal("changed CAS replay")
	}
	bad := req
	bad.RecordID = "invalid-utf8"
	bad.OperationID = "bad"
	bad.Sources = append([]evolution.SourceRef(nil), req.Sources...)
	bad.Sources[0].SourceID = string([]byte{0xff})
	if _, err := b.Mutate(ctx, bad); err == nil {
		t.Fatal("invalid UTF8 normalized")
	}
	foreign := binding
	foreign.DomainID = "foreign"
	other, err := NewBoundBackend(svc, foreign)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := other.MutationStatus(ctx, req.OperationID); err == nil {
		t.Fatal("foreign result disclosed")
	}
}

func TestIssue58BMLAdapterRawUTF8BeforeEncoding(t *testing.T) {
	ctx := context.Background()
	svc, err := Open(ctx, config.Config{Storage: config.Storage{DataDir: t.TempDir()}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = Close() })
	if out := svc.Add(ctx, bml.MemoryAddRequest{Content: "legacy anchor"}); out.Status != bml.CrudOutcomeApplied {
		t.Fatal(out)
	}
	binding := bml.OwnershipBinding{SubjectID: "subject", DomainID: "domain", ScopeKind: "personal", DestinationID: "bml"}
	b, err := NewBoundBackend(svc, binding)
	if err != nil {
		t.Fatal(err)
	}
	scope := evolution.Scope{SubjectID: "subject", Kind: evolution.ScopePersonal}
	base := gardenmemory.AuthorizedMutation{Scope: scope, DestinationID: "bml", OperationID: "raw", PayloadDigest: "digest", Operation: evolution.MutationCreate, RecordID: "raw-record", ExpectedAbsent: true, Body: "original", Inference: evolution.InferenceObserved, Sources: []evolution.SourceRef{{SourceID: "source", RecordID: "evidence", Revision: 1, Scope: scope}}}
	bad := string([]byte{0xff})
	cases := map[string]func(*gardenmemory.AuthorizedMutation){
		"body": func(r *gardenmemory.AuthorizedMutation) { r.Body = bad }, "operationID": func(r *gardenmemory.AuthorizedMutation) { r.OperationID = bad }, "digest": func(r *gardenmemory.AuthorizedMutation) { r.PayloadDigest = bad }, "recordID": func(r *gardenmemory.AuthorizedMutation) { r.RecordID = bad }, "subject": func(r *gardenmemory.AuthorizedMutation) { r.Scope.SubjectID = bad }, "scopeKind": func(r *gardenmemory.AuthorizedMutation) { r.Scope.Kind = evolution.ScopeKind(bad) }, "workspace": func(r *gardenmemory.AuthorizedMutation) { r.Scope.WorkspaceID = bad }, "destination": func(r *gardenmemory.AuthorizedMutation) { r.DestinationID = bad }, "operation": func(r *gardenmemory.AuthorizedMutation) { r.Operation = evolution.MutationOperation(bad) }, "inference": func(r *gardenmemory.AuthorizedMutation) { r.Inference = evolution.InferenceClass(bad) }, "sourceID": func(r *gardenmemory.AuthorizedMutation) { r.Sources[0].SourceID = bad }, "sourceRecord": func(r *gardenmemory.AuthorizedMutation) { r.Sources[0].RecordID = bad }, "sourceSubject": func(r *gardenmemory.AuthorizedMutation) { r.Sources[0].Scope.SubjectID = bad }, "sourceKind": func(r *gardenmemory.AuthorizedMutation) { r.Sources[0].Scope.Kind = evolution.ScopeKind(bad) }, "sourceWorkspace": func(r *gardenmemory.AuthorizedMutation) { r.Sources[0].Scope.WorkspaceID = bad },
	}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			r := base
			r.Sources = append([]evolution.SourceRef(nil), base.Sources...)
			change(&r)
			if _, err := b.Mutate(ctx, r); err == nil {
				t.Fatal("invalid raw UTF8 accepted")
			}
			if out, err := svc.home.LookupBoundOperation(ctx, binding, base.OperationID); err != nil || out != nil {
				t.Fatalf("invalid request committed original operation: %+v %v", out, err)
			}
		})
	}
	base.Body = "explicit replacement rune \ufffd"
	if _, err := b.Mutate(ctx, base); err != nil {
		t.Fatalf("valid U+FFFD control rejected: %v", err)
	}
	got, err := svc.home.ExportBoundSource(ctx, binding, base.RecordID, 1)
	if err != nil || got.Record.Content != base.Body {
		t.Fatalf("explicit U+FFFD bytes lost: %+v %v", got, err)
	}
}

func TestIssue58BMLBackendRejectsIncompleteNativeBinding(t *testing.T) {
	s, err := Open(context.Background(), config.Config{Storage: config.Storage{DataDir: t.TempDir()}})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	for _, binding := range []bml.OwnershipBinding{
		{SubjectID: " ", DomainID: "domain", ScopeKind: "personal", DestinationID: "bml"},
		{SubjectID: "subject", DomainID: "domain", ScopeKind: "workspace", WorkspaceID: " ", DestinationID: "bml"},
	} {
		if _, err := NewBoundBackend(s, binding); err == nil {
			t.Fatalf("incomplete native binding accepted: %+v", binding)
		}
	}
}
