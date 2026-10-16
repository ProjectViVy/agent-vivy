package bml

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// Regression derived from the independent reviewer_live_upgrade_test probe.
// Keep the original external probe immutable; this adds native positive,
// pre-limit, reopen/Home and concurrent supported-upgrade coverage.
func TestIssue58ReadOnlyLegacyHandleAcrossLiveUpgrade(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	home := NewHome(root)
	issue58SchemaOne(t, home.DatabasePath(), machineMemoryScope)
	dir := filepath.Dir(home.DatabasePath())
	before, err := os.ReadFile(home.DatabasePath())
	if err != nil {
		t.Fatal(err)
	}
	ro, err := OpenExisting(ctx, dir, machineMemoryScope)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := ro.Get(ctx, "legacy"); err != nil || got == nil {
		t.Fatalf("v1 Get unavailable: %+v %v", got, err)
	}
	if rows, err := ro.List(ctx, 1); err != nil || len(rows) != 1 || rows[0].Record.ID != "legacy" {
		t.Fatalf("v1 List unavailable: %+v %v", rows, err)
	}
	query := SearchQuery{Text: "evidence", Scope: machineScope(nil), Limit: 1}
	for _, search := range []func(context.Context, SearchQuery) ([]SearchHit, error){ro.Search, ro.SearchVisible} {
		if rows, err := search(ctx, query); err != nil || len(rows) != 1 || rows[0].Record.Record.ID != "legacy" {
			t.Fatalf("v1 FTS unavailable: %+v %v", rows, err)
		}
	}
	if err := ro.Close(); err != nil {
		t.Fatal(err)
	}
	if after, err := os.ReadFile(home.DatabasePath()); err != nil || !reflect.DeepEqual(before, after) {
		t.Fatalf("unchanged readonly v1 mutated bytes: %v", err)
	}
	ro, err = OpenExisting(ctx, dir, machineMemoryScope)
	if err != nil {
		t.Fatal(err)
	}
	defer ro.Close()
	rw, err := Open(ctx, dir, machineMemoryScope)
	if err != nil {
		t.Fatal(err)
	}
	defer rw.Close()
	request := issue58Create("a-private-row")
	if _, err := rw.MutateBound(ctx, issue58Owner, request); err != nil {
		t.Fatal(err)
	}
	checks := []struct {
		name string
		call func() error
	}{
		{"Get", func() error {
			got, err := ro.Get(ctx, request.Record.ID)
			if got != nil {
				t.Errorf("stale v1 Get disclosed %s", got.Record.ID)
			}
			return err
		}},
		{"List", func() error {
			rows, err := ro.List(ctx, 1)
			for _, row := range rows {
				if row.Record.ID == request.Record.ID {
					t.Error("stale v1 List disclosed private row before LIMIT")
				}
			}
			return err
		}},
		{"Search", func() error {
			rows, err := ro.Search(ctx, SearchQuery{Text: "needle", Scope: machineScope(nil), Limit: 1})
			if len(rows) > 0 {
				t.Error("stale v1 Search disclosed private FTS")
			}
			return err
		}},
		{"SearchVisible", func() error {
			rows, err := ro.SearchVisible(ctx, SearchQuery{Text: "needle", Scope: machineScope(nil), Limit: 1})
			if len(rows) > 0 {
				t.Error("stale v1 SearchVisible disclosed private FTS")
			}
			return err
		}},
		{"SupersededTargetIDs", func() error { _, err := ro.SupersededTargetIDs(ctx); return err }},
	}
	for _, check := range checks {
		var storeErr *StoreError
		if err := check.call(); !errors.As(err, &storeErr) || storeErr.Code != "schema_changed" {
			t.Errorf("%s did not explicitly fence upgraded v1 handle: %v", check.name, err)
		}
	}
	if caps := ro.BoundCapabilities(); caps.Mutate || caps.Export || caps.Search || caps.MutationLookup {
		t.Fatalf("old readonly handle inflated bound capabilities: %+v", caps)
	}
	if err := ro.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := OpenExisting(ctx, dir, machineMemoryScope)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if rows, err := reopened.List(ctx, 1); err != nil || len(rows) != 1 || rows[0].Record.ID != "legacy" {
		t.Fatalf("reopened v2 limit lost legacy: %+v %v", rows, err)
	}
	if rows, err := reopened.SearchVisible(ctx, SearchQuery{Text: "needle", Scope: machineScope(nil), Limit: 1}); err != nil || len(rows) != 0 {
		t.Fatalf("reopened v2 leaked private FTS: %+v %v", rows, err)
	}
	if got, err := reopened.GetBound(ctx, issue58Owner, request.Record.ID, 1); err != nil || got.Record.Content != request.Record.Content {
		t.Fatalf("reopened exact bound control failed: %+v %v", got, err)
	}
	defer home.Close()
	if err := home.Warmup(ctx); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(home.StartupMarkdown(), "legacy evidence") || strings.Contains(home.StartupMarkdown(), "private continuity") {
		t.Fatalf("reopened Home L1 isolation failed: %s", home.StartupMarkdown())
	}
}

func TestIssue58ReadOnlyReadsOverlapSupportedUpgrade(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	path := filepath.Join(dir, storeFileName)
	issue58SchemaOne(t, path, machineMemoryScope)
	// Released schema 1 used WAL too; allow the actual migration/writer to
	// commit while the reader is active instead of relying on a DDL lock.
	db, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`PRAGMA journal_mode=WAL`); err != nil {
		t.Fatal(err)
	}
	_ = db.Close()
	ro, err := OpenExisting(ctx, dir, machineMemoryScope)
	if err != nil {
		t.Fatal(err)
	}
	defer ro.Close()
	ready, done := make(chan struct{}), make(chan error, 1)
	writerDone := make(chan struct{})
	go func() {
		defer close(ready)
		for i := 0; ; i++ {
			got, err := ro.Get(ctx, "a-private")
			if err == nil && got != nil {
				done <- fmt.Errorf("overlap Get leaked %s", got.Record.ID)
				return
			}
			rows, err := ro.List(ctx, 1)
			if err == nil {
				for _, r := range rows {
					if r.Record.ID != "legacy" {
						done <- fmt.Errorf("overlap List leaked %s", r.Record.ID)
						return
					}
				}
			}
			for _, search := range []func(context.Context, SearchQuery) ([]SearchHit, error){ro.Search, ro.SearchVisible} {
				if hits, err := search(ctx, SearchQuery{Text: "needle", Scope: machineScope(nil), Limit: 1}); err == nil && len(hits) > 0 {
					done <- fmt.Errorf("overlap FTS leaked %s", hits[0].Record.Record.ID)
					return
				}
			}
			if i == 0 {
				ready <- struct{}{}
			}
			if i >= 255 {
				select {
				case <-writerDone:
					done <- nil
					return
				default:
				}
			}
		}
	}()
	<-ready
	rw, err := Open(ctx, dir, machineMemoryScope)
	if err != nil {
		close(writerDone)
		t.Fatal(err)
	}
	defer rw.Close()
	if _, err := rw.MutateBound(ctx, issue58Owner, issue58Create("a-private")); err != nil {
		close(writerDone)
		t.Fatal(err)
	}
	close(writerDone)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	// The post-upgrade call must also fence after the overlap finishes.
	if got, err := ro.Get(ctx, "a-private"); err == nil || got != nil {
		t.Fatalf("post-upgrade read was not fenced: %+v %v", got, err)
	}
}

// Regression derived from reviewer_target_cas_test. The original external
// probe remains unchanged; verify every affected canonical/derived table.
func TestIssue58BoundTombstoneHasOnlyItsGuardedTarget(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t, machineMemoryScope)
	a, b := issue58Create("a"), issue58Create("b")
	oa, err := s.MutateBound(ctx, issue58Owner, a)
	if err != nil {
		t.Fatal(err)
	}
	ob, err := s.MutateBound(ctx, issue58Owner, b)
	if err != nil {
		t.Fatal(err)
	}
	update := b
	update.Operation = "update"
	update.OperationID = "update-b"
	update.ExpectedRecordRevision = &ob.RecordRevision
	update.Record.Content = "independently corrected needle"
	update.Record.Provenance.ContentDigest = MemoryContentDigest([]byte(update.Record.Content))
	if _, err := s.MutateBound(ctx, issue58Owner, update); err != nil {
		t.Fatal(err)
	}
	marker := tombstoneRecord("marker", machineMemoryScope, "a")
	marker.Scope = machineScope(nil)
	marker.Supersedes = []string{"a", "b"}
	request := BoundMutation{Operation: "tombstone", OperationID: "remove-a", PayloadDigest: "same-caller-digest", Record: marker, ExpectedTargetRevision: &oa.RecordRevision}
	before := issue58MutationTables(t, s)
	meta, err := s.Metadata(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if out, err := s.MutateBound(ctx, issue58Owner, request); err == nil {
		t.Errorf("single-target CAS suppressed extra target: %+v", out)
	}
	if after := issue58MutationTables(t, s); !reflect.DeepEqual(before, after) {
		t.Error("rejected extra target changed rows/ownership/outcomes/FTS/supersedes")
	}
	if after, err := s.Metadata(ctx); err != nil || after != meta {
		t.Fatalf("extra target changed native counters/CAS: %+v %v", after, err)
	}
	if receipt, err := s.LookupBoundMutation(ctx, issue58Owner, request.OperationID, request.PayloadDigest); err != nil || receipt != nil {
		t.Fatalf("extra target retained false receipt: %+v %v", receipt, err)
	}
	request.Record.Supersedes = []string{"a"}
	out, err := s.MutateBound(ctx, issue58Owner, request)
	if err != nil || out.TargetID != "a" {
		t.Fatalf("single-target positive rejected: %+v %v", out, err)
	}
	if _, err := s.GetBound(ctx, issue58Owner, "a", 1); err == nil {
		t.Fatal("single guarded target remained visible")
	}
	if got, err := s.GetBound(ctx, issue58Owner, "b", 2); err != nil || got.Record.Content != update.Record.Content {
		t.Fatalf("independently updated B lost: %+v %v", got, err)
	}
}

func TestIssue58LegacyMultipleSupersedesStillWorks(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t, machineMemoryScope)
	for i, id := range []string{"a", "b"} {
		r := storeRecord(id, machineMemoryScope, "legacy needle")
		r.Scope = machineScope(nil)
		if _, err := s.Put(ctx, r, int64(i), nil); err != nil {
			t.Fatal(err)
		}
	}
	marker := tombstoneRecord("legacy-marker", machineMemoryScope, "a")
	marker.Scope = machineScope(nil)
	marker.Supersedes = []string{"a", "b"}
	if err := s.PutTombstone(ctx, marker, 2, "a", 1); err != nil {
		t.Fatal(err)
	}
	if rows, err := s.SearchVisible(ctx, SearchQuery{Text: "needle", Scope: machineScope(nil), Limit: 10}); err != nil || len(rows) != 0 {
		t.Fatalf("legacy multi-target suppression changed: %+v %v", rows, err)
	}
	if targets, err := s.SupersededTargetIDs(ctx); err != nil || len(targets) != 2 {
		t.Fatalf("legacy multi-target edge lost: %+v %v", targets, err)
	}
}

func issue58MutationTables(t *testing.T, s *Store) map[string][]string {
	t.Helper()
	out := map[string][]string{}
	queries := map[string]string{
		"records":    `SELECT memory_id||':'||record_revision||':'||record_json FROM memory_records ORDER BY memory_id`,
		"ownership":  `SELECT memory_id||':'||binding_json FROM memory_ownership ORDER BY memory_id`,
		"operations": `SELECT operation_id||':'||binding_json||':'||payload_digest||':'||request_fingerprint||':'||outcome_json FROM memory_operations ORDER BY operation_id`,
		"fts":        `SELECT memory_id||':'||tenant_id||':'||workspace_id||':'||COALESCE(session_id,'')||':'||content FROM memory_fts ORDER BY memory_id`,
		"supersedes": `SELECT memory_id||':'||superseded_id FROM memory_supersedes ORDER BY memory_id,superseded_id`,
	}
	for name, query := range queries {
		rows, err := s.db.Query(query)
		if err != nil {
			t.Fatal(err)
		}
		for rows.Next() {
			var raw string
			if err := rows.Scan(&raw); err != nil {
				t.Fatal(err)
			}
			out[name] = append(out[name], raw)
		}
		err = rows.Err()
		_ = rows.Close()
		if err != nil {
			t.Fatal(err)
		}
	}
	return out
}
