package bml

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// Build the released schema directly; Open must not create the fixture using
// the new schema and accidentally skip the real existing-Home upgrade path.
func issue58SchemaOne(t *testing.T, path, workspace string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, ddl := range schemaDDL {
		if _, err := db.Exec(ddl); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.Exec(ftsDDL); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(insertMetaSQL, component, 1, workspace); err != nil {
		t.Fatal(err)
	}
	r := storeRecord("legacy", workspace, "legacy evidence")
	if workspace == machineMemoryScope {
		r.Scope = machineScope(nil)
	}
	raw, err := marshalCanonical(r)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(insertRecordSQL, r.ID, r.Kind, r.Scope.TenantID, r.Scope.WorkspaceID, r.Scope.SessionID, r.Trust, r.Sensitivity, rfc3339UTC(r.CreatedAt), rfc3339UTC(r.EffectiveAt), nil, 0, len(r.Content), string(raw)); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(insertFTSSQL, r.ID, r.Scope.TenantID, workspace, nil, r.Content); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE schema_meta SET store_revision=1,record_count=1,content_bytes=?`, len(r.Content)); err != nil {
		t.Fatal(err)
	}
}

func TestIssue58SchemaTwoFreshAndRealHomeUpgrade(t *testing.T) {
	ctx := context.Background()
	fresh, err := Open(ctx, t.TempDir(), "fresh")
	if err != nil {
		t.Fatal(err)
	}
	defer fresh.Close()
	if m, err := fresh.Metadata(ctx); err != nil || m.SchemaVersion != 2 {
		t.Errorf("fresh schema not version 2: %+v %v", m, err)
	}
	root := t.TempDir()
	h := NewHome(root)
	issue58SchemaOne(t, h.DatabasePath(), machineMemoryScope)
	if err := h.Warmup(ctx); err != nil {
		t.Fatal(err)
	}
	s, err := h.existingStore(ctx)
	if err != nil {
		t.Fatal(err)
	}
	m, err := s.Metadata(ctx)
	if err != nil || m.SchemaVersion != 2 || m.StoreRevision != 1 || m.RecordCount != 1 {
		t.Fatalf("real Home did not preserve/upgrade v1: %+v %v", m, err)
	}
	legacy, err := h.GetRecord(ctx, "legacy")
	if err != nil || legacy == nil || legacy.Record.Content != "legacy evidence" {
		t.Fatalf("lost legacy record: %+v %v", legacy, err)
	}
	if err := h.Close(); err != nil {
		t.Fatal(err)
	}
	h = NewHome(root)
	defer h.Close()
	if err := h.Warmup(ctx); err != nil {
		t.Fatal(err)
	}
	s, err = h.existingStore(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if after, err := s.Metadata(ctx); err != nil || after != m {
		t.Fatalf("reopen repeated migration: %+v %v", after, err)
	}
}

func TestIssue58SchemaOneReadOnlyAndUpgradeRollback(t *testing.T) {
	ctx := context.Background()
	for _, failUpgrade := range []bool{false, true} {
		t.Run(map[bool]string{false: "read_only", true: "failed_upgrade"}[failUpgrade], func(t *testing.T) {
			root := t.TempDir()
			h := NewHome(root)
			path := h.DatabasePath()
			issue58SchemaOne(t, path, machineMemoryScope)
			if failUpgrade {
				db, err := sql.Open("sqlite", "file:"+path)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := db.Exec(`CREATE TRIGGER reject_schema_two BEFORE UPDATE OF schema_version ON schema_meta WHEN NEW.schema_version=2 BEGIN SELECT RAISE(ABORT,'injected schema upgrade failure'); END`); err != nil {
					t.Fatal(err)
				}
				if err := db.Close(); err != nil {
					t.Fatal(err)
				}
			}
			before, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if failUpgrade {
				if err := h.Warmup(ctx); err == nil {
					t.Error("Home ignored failed v1 -> v2 upgrade")
				}
				_ = h.Close()
			}
			ro, err := OpenExisting(ctx, filepath.Dir(path), machineMemoryScope)
			if err != nil {
				t.Fatal(err)
			}
			m, err := ro.Metadata(ctx)
			if err != nil || m.SchemaVersion != 1 || m.RecordCount != 1 {
				t.Fatalf("legacy schema changed: %+v %v", m, err)
			}
			if got, err := ro.Get(ctx, "legacy"); err != nil || got == nil {
				t.Fatalf("readonly legacy unavailable: %+v %v", got, err)
			}
			var n int
			if err := ro.db.QueryRow(`SELECT count(*) FROM sqlite_master WHERE name IN ('memory_ownership','memory_operations')`).Scan(&n); err != nil || n != 0 {
				t.Fatalf("partial schema migration survived: %d %v", n, err)
			}
			if err := ro.Close(); err != nil {
				t.Fatal(err)
			}
			if !failUpgrade {
				after, err := os.ReadFile(path)
				if err != nil || !reflect.DeepEqual(before, after) {
					t.Fatalf("read-only open changed database bytes: %v", err)
				}
			}
		})
	}
}

func TestIssue58SchemaOneCannotAdoptPreexistingOwnership(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	path := filepath.Join(dir, storeFileName)
	issue58SchemaOne(t, path, machineMemoryScope)
	db, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatal(err)
	}
	for _, ddl := range boundSchemaDDL {
		if _, err := db.Exec(ddl); err != nil {
			t.Fatal(err)
		}
	}
	owner, err := marshalCanonical(issue58Owner)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO memory_ownership(memory_id,binding_json) VALUES('legacy',?)`, string(owner)); err != nil {
		t.Fatal(err)
	}
	_ = db.Close()
	if opened, err := Open(ctx, dir, machineMemoryScope); err == nil {
		_ = opened.Close()
		t.Fatal("schema1 adopted an unqualified external ownership table")
	}
	ro, err := OpenExisting(ctx, dir, machineMemoryScope)
	if err != nil {
		t.Fatal(err)
	}
	defer ro.Close()
	if meta, err := ro.Metadata(ctx); err != nil || meta.SchemaVersion != 1 {
		t.Fatalf("unsupported v1 shape was upgraded: %+v %v", meta, err)
	}
}
