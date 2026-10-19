package bml

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
)

func TestIssue58BoundDeletedTargetCannotBeUpdatedOrDeletedAgain(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t, machineMemoryScope)
	create := issue58Create("target")
	out, err := s.MutateBound(ctx, issue58Owner, create)
	if err != nil {
		t.Fatal(err)
	}
	remove := BoundMutation{Operation: "tombstone", OperationID: "remove-1", PayloadDigest: "digest", Record: tombstoneRecord("marker-1", machineMemoryScope, "target"), ExpectedTargetRevision: &out.RecordRevision}
	remove.Record.Scope = machineScope(nil)
	if _, err := s.MutateBound(ctx, issue58Owner, remove); err != nil {
		t.Fatal(err)
	}
	before, err := s.Metadata(ctx)
	if err != nil {
		t.Fatal(err)
	}
	update := create
	update.Operation = "update"
	update.OperationID = "update-removed"
	update.ExpectedRecordRevision = &out.RecordRevision
	if _, err := s.MutateBound(ctx, issue58Owner, update); err == nil {
		t.Error("update reused CAS of removed target")
	}
	remove.OperationID = "remove-2"
	remove.Record.ID = "marker-2"
	if _, err := s.MutateBound(ctx, issue58Owner, remove); err == nil {
		t.Error("second tombstone reused removed target CAS")
	}
	if after, err := s.Metadata(ctx); err != nil || after != before {
		t.Fatalf("failed deleted-target mutation changed counters/revision: %+v %v", after, err)
	}
}

func TestIssue58BoundCannotInheritForeignSuppression(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t, machineMemoryScope)
	legacy := tombstoneRecord("old-marker", machineMemoryScope, "future-id")
	legacy.Scope = machineScope(nil)
	if _, err := s.Put(ctx, legacy, 0, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := s.MutateBound(ctx, issue58Owner, issue58Create("future-id")); err == nil {
		t.Fatal("bound record inherited foreign legacy tombstone")
	}
	if n := countTableRows(t, s, "memory_operations"); n != 0 {
		t.Fatal("foreign suppression produced successful native outcome")
	}
}

func TestIssue58BoundDistinctOperationsAndRecordCASConcurrency(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t, machineMemoryScope)
	other, err := Open(ctx, filepath.Dir(s.Path()), machineMemoryScope)
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	var wg sync.WaitGroup
	errs := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			handle := s
			if i%2 == 1 {
				handle = other
			}
			r := issue58Create(string(rune('a' + i)))
			_, err := handle.MutateBound(ctx, issue58Owner, r)
			errs <- err
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	if meta, err := s.Metadata(ctx); err != nil || meta.StoreRevision != 8 || meta.RecordCount != 8 {
		t.Fatalf("distinct operations lost commits: %+v %v", meta, err)
	}
	results := make(chan error, 2)
	one := int64(1)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			r := issue58Create("a")
			r.Operation = "update"
			r.OperationID = "update-" + string(rune('a'+i))
			r.ExpectedRecordRevision = &one
			handle := s
			if i == 1 {
				handle = other
			}
			_, err := handle.MutateBound(ctx, issue58Owner, r)
			results <- err
		}(i)
	}
	wg.Wait()
	close(results)
	wins, conflicts := 0, 0
	for err := range results {
		if err == nil {
			wins++
		} else if storeErrCode(t, err) == ErrRecordRevisionConflict {
			conflicts++
		} else {
			t.Fatal(err)
		}
	}
	if wins != 1 || conflicts != 1 {
		t.Fatalf("record CAS winners=%d conflicts=%d", wins, conflicts)
	}
}

func TestIssue58BoundSchemaShapeCannotBeSilentlyRecreated(t *testing.T) {
	ctx := context.Background()
	for _, version := range []int{1, 2} {
		t.Run(map[int]string{1: "v1_collision", 2: "v2_missing_operations"}[version], func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, storeFileName)
			if version == 1 {
				issue58SchemaOne(t, path, machineMemoryScope)
			} else {
				s, err := Open(ctx, dir, machineMemoryScope)
				if err != nil {
					t.Fatal(err)
				}
				if err := s.Close(); err != nil {
					t.Fatal(err)
				}
			}
			db, err := sql.Open("sqlite", "file:"+path)
			if err != nil {
				t.Fatal(err)
			}
			if version == 1 {
				_, err = db.Exec(`CREATE TABLE memory_operations(wrong_column TEXT)`)
			} else {
				_, err = db.Exec(`DROP TABLE memory_operations`)
			}
			if err != nil {
				t.Fatal(err)
			}
			_ = db.Close()
			before, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if s, err := Open(ctx, dir, machineMemoryScope); err == nil {
				_ = s.Close()
				t.Error("invalid native schema marked usable/recreated")
			}
			ro, err := openReadOnlyPath(ctx, path, machineMemoryScope)
			if err == nil {
				_ = ro.Close()
				if version == 2 {
					t.Error("readonly damaged schema advertised bound support")
				}
			}
			db, err = sql.Open("sqlite", "file:"+path)
			if err != nil {
				t.Fatal(err)
			}
			var actual int
			if err := db.QueryRow(`SELECT schema_version FROM schema_meta`).Scan(&actual); err != nil {
				t.Fatal(err)
			}
			_ = db.Close()
			if actual != version {
				t.Fatalf("failed schema validation changed version: %d", actual)
			}
			if version == 2 {
				after, err := os.ReadFile(path)
				if err != nil || !reflect.DeepEqual(before, after) {
					t.Fatalf("damaged v2 open changed bytes: %v", err)
				}
			}
		})
	}
}

func TestIssue58BoundRestoreUpgradeFailurePreservesLiveAndBackup(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t, machineMemoryScope)
	out, err := s.MutateBound(ctx, issue58Owner, issue58Create("live"))
	if err != nil {
		t.Fatal(err)
	}
	backup := filepath.Join(t.TempDir(), storeFileName)
	issue58SchemaOne(t, backup, machineMemoryScope)
	db, err := sql.Open("sqlite", "file:"+backup)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TRIGGER reject_upgrade BEFORE UPDATE OF schema_version ON schema_meta WHEN NEW.schema_version=2 BEGIN SELECT RAISE(ABORT,'restore upgrade failure'); END`); err != nil {
		t.Fatal(err)
	}
	_ = db.Close()
	before, err := os.ReadFile(backup)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Restore(ctx, backup); err == nil {
		t.Fatal("faulted restore claimed success")
	}
	reopened, err := Open(ctx, filepath.Dir(s.Path()), machineMemoryScope)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if got, err := reopened.LookupBoundMutation(ctx, issue58Owner, out.OperationID, out.PayloadDigest); err != nil || got == nil || *got != out {
		t.Fatalf("failed restore lost original live outcome: %+v %v", got, err)
	}
	if after, err := os.ReadFile(backup); err != nil || !reflect.DeepEqual(before, after) {
		t.Fatalf("failed restore changed backup: %v", err)
	}
}

func TestIssue58BoundRawEnumsCannotChangeDuringCanonicalDecode(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t, machineMemoryScope)
	for _, field := range []string{"kind", "provenance", "trust", "sensitivity", "evidence", "empty_kind", "empty_provenance", "empty_trust", "empty_sensitivity"} {
		t.Run(field, func(t *testing.T) {
			m := issue58Create("raw-enum")
			switch field {
			case "empty_kind":
				m.Record.Kind = ""
			case "empty_provenance":
				m.Record.Provenance.Source = ""
			case "empty_trust":
				m.Record.Trust = ""
			case "empty_sensitivity":
				m.Record.Sensitivity = ""
			case "kind":
				m.Record.Kind = "not-a-kind"
			case "provenance":
				m.Record.Provenance.Source = "not-a-source"
			case "trust":
				m.Record.Trust = "not-a-trust"
			case "sensitivity":
				m.Record.Sensitivity = "not-a-sensitivity"
			case "evidence":
				m.Record.EvidenceRefs = []EvidenceRef{{ID: "e", Source: "not-evidence", URI: "file:exact"}}
			}
			if _, err := s.MutateBound(ctx, issue58Owner, m); err == nil {
				t.Error("raw enum committed but will decode to different native material")
			}
			if meta, err := s.Metadata(ctx); err != nil || meta.StoreRevision != 0 {
				t.Fatalf("invalid raw enum committed: %+v %v", meta, err)
			}
		})
	}
}

func TestIssue58BoundHomeProjectionFailureRetainsOutcome(t *testing.T) {
	ctx := context.Background()
	h := NewHome(t.TempDir())
	defer h.Close()
	legacy, err := h.AddLongTerm(ctx, "ordinary legacy", nil)
	if err != nil {
		t.Fatal(err)
	}
	s, err := h.existingStore(ctx)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := marshalCanonical(legacy.Record)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(`UPDATE memory_records SET record_json='{' WHERE memory_id=?`, legacy.Record.ID); err != nil {
		t.Fatal(err)
	}
	request := issue58Create("projection-retry")
	if _, err := h.MutateBound(ctx, issue58Owner, request); err == nil {
		t.Fatal("failed L1 refresh reported fully completed Home operation")
	}
	actual, err := h.LookupBoundMutation(ctx, issue58Owner, request.OperationID, request.PayloadDigest)
	if err != nil || actual == nil || actual.StoreRevision != 2 {
		t.Fatalf("refresh error erased committed outcome: %+v %v", actual, err)
	}
	if _, err := s.db.Exec(`UPDATE memory_records SET record_json=? WHERE memory_id=?`, string(raw), legacy.Record.ID); err != nil {
		t.Fatal(err)
	}
	if replay, err := h.MutateBound(ctx, issue58Owner, request); err != nil || replay != *actual {
		t.Fatalf("projection retry rewrote native effect: %+v %v", replay, err)
	}
}
