package bml

import (
	"context"
	"reflect"
	"strings"
	"testing"
)

func TestIssue58BoundLegacyReadWriteAndL1Isolation(t *testing.T) {
	ctx := context.Background()
	h := NewHomeWithL1Budget(t.TempDir(), 1)
	defer h.Close()
	request := issue58Create("a-bound")
	request.Record.Kind = KindLongTerm
	if _, err := h.MutateBound(ctx, issue58Owner, request); err != nil {
		t.Fatal(err)
	}
	s, err := h.existingStore(ctx)
	if err != nil {
		t.Fatal(err)
	}
	legacy := storeRecord("z-legacy", machineMemoryScope, "legacy needle")
	legacy.Scope = machineScope(nil)
	meta, err := s.Metadata(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Put(ctx, legacy, meta.StoreRevision, nil); err != nil {
		t.Fatal(err)
	}
	if err := h.Warmup(ctx); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(h.StartupMarkdown(), "private continuity") || !strings.Contains(h.StartupMarkdown(), "legacy needle") {
		t.Fatalf("L1 leaked or limit consumed by bound record: %s", h.StartupMarkdown())
	}
	for _, read := range []func() (*StoredRecord, error){func() (*StoredRecord, error) { return s.Get(ctx, request.Record.ID) }, func() (*StoredRecord, error) { return h.GetRecord(ctx, request.Record.ID) }} {
		if got, err := read(); err != nil || got != nil {
			t.Fatalf("legacy Get leaked: %+v %v", got, err)
		}
	}
	for _, list := range []func() ([]StoredRecord, error){func() ([]StoredRecord, error) { return s.List(ctx, 1) }, func() ([]StoredRecord, error) { return h.ListRecords(ctx, 1) }} {
		got, err := list()
		if err != nil || len(got) != 1 || got[0].Record.ID != "z-legacy" {
			t.Fatalf("legacy List filter after LIMIT: %+v %v", got, err)
		}
	}
	query := SearchQuery{Text: "needle", Scope: machineScope(nil), Limit: 1}
	for _, search := range []func() ([]SearchHit, error){func() ([]SearchHit, error) { return s.Search(ctx, query) }, func() ([]SearchHit, error) { return s.SearchVisible(ctx, query) }, func() ([]SearchHit, error) { return h.SearchVisible(ctx, query) }} {
		got, err := search()
		if err != nil || len(got) != 1 || got[0].Record.Record.ID != "z-legacy" {
			t.Fatalf("legacy search leaked/filtered after LIMIT: %+v %v", got, err)
		}
	}
	if got, err := h.SearchBound(ctx, issue58Owner, query); err != nil || len(got) != 1 || got[0].Record.Record.ID != request.Record.ID {
		t.Fatalf("bound search mixed legacy: %+v %v", got, err)
	}
	if got, err := h.ExportBoundSource(ctx, issue58Owner, request.Record.ID, 1); err != nil || got.Record.ID != request.Record.ID {
		t.Fatalf("bound export: %+v %v", got, err)
	}
	if _, err := h.ExportBoundSource(ctx, issue58Owner, legacy.ID, 1); err == nil {
		t.Fatal("unknown legacy ownership promoted")
	}
	if _, err := h.ExportSource(ctx, request.Record.ID, 1); err == nil {
		t.Fatal("unscoped export disclosed bound record")
	}
	if got, err := h.ExportSource(ctx, legacy.ID, 1); err != nil || got.Record.ID != legacy.ID {
		t.Fatalf("legacy exact export unavailable: %+v %v", got, err)
	}
	before, err := s.Metadata(ctx)
	if err != nil {
		t.Fatal(err)
	}
	one := int64(1)
	if _, err := s.Put(ctx, request.Record, before.StoreRevision, &one); err == nil {
		t.Fatal("legacy Put overwrote bound record")
	}
	if _, err := s.ImportRecords(ctx, []Record{request.Record}, before.StoreRevision); err == nil {
		t.Fatal("legacy import claimed owned record")
	}
	attack := tombstoneRecord("legacy-tombstone", machineMemoryScope, request.Record.ID)
	attack.Scope = machineScope(nil)
	if err := s.PutTombstone(ctx, attack, before.StoreRevision, request.Record.ID, 1); err == nil {
		t.Fatal("legacy tombstone suppressed owned record")
	}
	if _, err := h.UpdateRecord(ctx, request.Record.ID, "overwrite", 1, nil); err == nil {
		t.Fatal("legacy Home update overwrote owned record")
	}
	if _, err := h.RemoveRecord(ctx, request.Record.ID, "remove", 1); err == nil {
		t.Fatal("legacy Home remove suppressed owned record")
	}
	if after, err := s.Metadata(ctx); err != nil || after != before {
		t.Fatalf("denied legacy write mutated metadata: %+v %v", after, err)
	}
}

func TestIssue58BoundExactIdentityAndFingerprint(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t, machineMemoryScope)
	request := issue58Create("bound")
	first, err := s.MutateBound(ctx, issue58Owner, request)
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"subject", "domain", "kind", "workspace", "destination"} {
		t.Run(field, func(t *testing.T) {
			other := issue58Owner
			switch field {
			case "subject":
				other.SubjectID = "other"
			case "domain":
				other.DomainID = "other"
			case "kind":
				other.ScopeKind = "workspace"
				other.WorkspaceID = "w"
			case "workspace":
				other.ScopeKind = "workspace"
				other.WorkspaceID = "other"
			case "destination":
				other.DestinationID = "other"
			}
			if _, err := s.LookupBoundMutation(ctx, other, request.OperationID, request.PayloadDigest); err == nil {
				t.Fatal("foreign outcome disclosed")
			}
			if _, err := s.GetBound(ctx, other, request.Record.ID, 1); err == nil {
				t.Fatal("foreign record disclosed")
			}
			if _, err := s.MutateBound(ctx, other, request); err == nil {
				t.Fatal("foreign operation reused")
			}
			mutation := request
			mutation.OperationID = "foreign-create"
			if _, err := s.MutateBound(ctx, other, mutation); err == nil {
				t.Fatal("foreign record ownership claimed")
			}
		})
	}
	for _, change := range []string{"body", "sources", "store_cas_presence", "digest", "record_cas", "operation"} {
		t.Run(change, func(t *testing.T) {
			altered := request
			switch change {
			case "body":
				altered.Record.Content = "altered"
				altered.Record.Provenance.ContentDigest = MemoryContentDigest([]byte("altered"))
			case "sources":
				altered.Record.Provenance.SourceID = "changed-source"
			case "store_cas_presence":
				zero := int64(0)
				altered.ExpectedStoreRevision = &zero
			case "digest":
				altered.PayloadDigest = "different"
			case "record_cas":
				one := int64(1)
				altered.ExpectedRecordRevision = &one
			case "operation":
				altered.Operation = "update"
			}
			if _, err := s.MutateBound(ctx, issue58Owner, altered); err == nil {
				t.Fatal("changed request replayed unchanged caller digest")
			}
		})
	}
	if got, err := s.LookupBoundMutation(ctx, issue58Owner, request.OperationID, request.PayloadDigest); err != nil || got == nil || *got != first {
		t.Fatalf("conflict overwrote immutable result: %+v %v", got, err)
	}
	legacy := storeRecord("unbound", machineMemoryScope, "unknown old owner")
	legacy.Scope = machineScope(nil)
	meta, err := s.Metadata(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Put(ctx, legacy, meta.StoreRevision, nil); err != nil {
		t.Fatal(err)
	}
	claim := issue58Create("unbound")
	claim.Operation = "update"
	one := int64(1)
	claim.ExpectedRecordRevision = &one
	if _, err := s.MutateBound(ctx, issue58Owner, claim); err == nil {
		t.Fatal("legacy row gained subject ownership")
	}
	if got, err := s.Get(ctx, "unbound"); err != nil || got == nil || !reflect.DeepEqual(got.Record, legacy) {
		t.Fatalf("unknown legacy record changed: %+v %v", got, err)
	}
}

func TestIssue58BoundRejectsInvalidUTF8BeforeFingerprint(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t, machineMemoryScope)
	bad := string([]byte{0xff})
	for _, field := range []string{"owner", "operation", "digest", "id", "body", "source", "correlation", "scope", "supersedes", "evidence_uri", "evidence_excerpt"} {
		t.Run(field, func(t *testing.T) {
			binding := issue58Owner
			request := issue58Create("utf8")
			switch field {
			case "owner":
				binding.DomainID = bad
			case "operation":
				request.OperationID = bad
			case "digest":
				request.PayloadDigest = bad
			case "id":
				request.Record.ID = bad
			case "body":
				request.Record.Content = bad
				request.Record.Provenance.ContentDigest = MemoryContentDigest([]byte(bad))
			case "source":
				request.Record.Provenance.SourceID = bad
			case "correlation":
				request.Record.Provenance.Correlation.RequestID = bad
			case "scope":
				request.Record.Scope.TenantID = bad
			case "supersedes":
				request.Record.Supersedes = []string{bad}
			case "evidence_uri":
				request.Record.EvidenceRefs = []EvidenceRef{{ID: "e", Source: EvidenceSourceFile, URI: bad}}
			case "evidence_excerpt":
				request.Record.EvidenceRefs = []EvidenceRef{{ID: "e", Source: EvidenceSourceFile, URI: "file:test", Excerpt: &bad}}
			}
			if _, err := s.MutateBound(ctx, binding, request); err == nil {
				t.Fatal("UTF8 normalized into durable fingerprint")
			}
			if m, err := s.Metadata(ctx); err != nil || m.StoreRevision != 0 || m.RecordCount != 0 {
				t.Fatalf("invalid UTF8 changed store: %+v %v", m, err)
			}
		})
	}
}
