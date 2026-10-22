package bml

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestHomeWarmupAfterCancelledFirstInitialization(t *testing.T) {
	root := t.TempDir()
	h := NewHome(root)
	path := h.DatabasePath()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, ddl := range schemaDDL {
		if _, err := tx.ExecContext(ctx, ddl); err != nil {
			t.Fatal(err)
		}
	}
	// Observer shutdown or its delivery deadline can cancel the first append
	// after SQLite creates the file but before the schema transaction commits.
	cancel()
	if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, pristine, err := inspectStore(context.Background(), path, machineMemoryScope); err != nil || !pristine {
		t.Fatalf("rollback did not leave a pristine store: %v %v", pristine, err)
	}
	for range 2 {
		if err := h.Warmup(context.Background()); err != nil {
			t.Fatalf("warmup after cancelled initialization: %v", err)
		}
		if records, err := h.ListRecords(context.Background(), 10); err != nil || len(records) != 0 {
			t.Fatalf("empty read: %+v %v", records, err)
		}
		if err := h.Close(); err != nil {
			t.Fatal(err)
		}
		after, err := os.ReadFile(path)
		if err != nil || !bytes.Equal(before, after) {
			t.Fatalf("warmup changed pristine source: %v", err)
		}
		h = NewHome(root)
	}
	input := AppendHistoryInput{ID: "first-record", SourceID: "run:1", Content: "first committed history"}
	stored, applied, err := h.AppendHistory(context.Background(), input)
	if err != nil || !applied {
		t.Fatalf("subsequent append did not initialize normally: %+v %v %v", stored, applied, err)
	}
	if err := h.Close(); err != nil {
		t.Fatal(err)
	}
	h = NewHome(root)
	t.Cleanup(func() { _ = h.Close() })
	if err := h.Warmup(context.Background()); err != nil {
		t.Fatalf("initialized store did not reopen: %v", err)
	}
	replayed, applied, err := h.AppendHistory(context.Background(), input)
	if err != nil || applied || replayed.Record.ID != stored.Record.ID || replayed.Revision != stored.Revision {
		t.Fatalf("reopened history changed: %+v %v %v", replayed, applied, err)
	}
}

func TestHomeWarmupRefusesNonPristineSourcesWithoutMutation(t *testing.T) {
	ctx := context.Background()
	for _, kind := range []string{"unknown", "schema1"} {
		t.Run(kind, func(t *testing.T) {
			h := NewHome(t.TempDir())
			t.Cleanup(func() { _ = h.Close() })
			if err := os.MkdirAll(h.MemoryDir(), 0o700); err != nil {
				t.Fatal(err)
			}
			want := ErrUnsupportedSourceFormat
			if kind == "schema1" {
				store, err := Open(ctx, h.MemoryDir(), machineMemoryScope)
				if err != nil {
					t.Fatal(err)
				}
				_, err = store.db.Exec(`CREATE TABLE memory_apply_journal(idempotency_key TEXT PRIMARY KEY,proposal_id TEXT NOT NULL,request_id TEXT NOT NULL,content_digest TEXT NOT NULL,record_id TEXT NOT NULL,store_revision INTEGER NOT NULL,record_revision INTEGER NOT NULL,actor_id TEXT NOT NULL,applied_at TEXT NOT NULL); UPDATE schema_meta SET schema_version=1`)
				if err != nil {
					t.Fatal(err)
				}
				if err := store.Close(); err != nil {
					t.Fatal(err)
				}
				want = ErrUpgradeRequired
			} else {
				db, err := sql.Open("sqlite", h.DatabasePath())
				if err != nil {
					t.Fatal(err)
				}
				if _, err := db.Exec(`CREATE TABLE foreign_data(value TEXT); INSERT INTO foreign_data VALUES('preserve unknown history')`); err != nil {
					t.Fatal(err)
				}
				if err := db.Close(); err != nil {
					t.Fatal(err)
				}
			}
			before, err := os.ReadFile(h.DatabasePath())
			if err != nil {
				t.Fatal(err)
			}
			var se *StoreError
			if err := h.Warmup(ctx); !errors.As(err, &se) || se.Code != want {
				t.Fatalf("non-pristine source admitted: %v", err)
			}
			_, _, err = h.AppendHistory(ctx, AppendHistoryInput{ID: "must-not-write", SourceID: "run:1", Content: "must not replace source"})
			if !errors.As(err, &se) || se.Code != want {
				t.Fatalf("write adopted refused source: %v", err)
			}
			after, err := os.ReadFile(h.DatabasePath())
			if err != nil || !bytes.Equal(before, after) {
				t.Fatalf("refused source changed: %v", err)
			}
		})
	}
}
