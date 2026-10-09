package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"testing/fstest"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	nb "agent-vivy/internal/notebookcontract"
	"agent-vivy/internal/storage"
	"agent-vivy/internal/storage/conformance"
	"agent-vivy/internal/storage/migrations"
)

var pgNotebookSeq atomic.Int64

func notebookSchema(t *testing.T) string {
	t.Helper()
	return fmt.Sprintf("test_nb_%d_%d", time.Now().UnixNano(), pgNotebookSeq.Add(1))
}

func notebookSlot(t *testing.T) (conformance.Slot, bool) {
	t.Helper()
	dsn := os.Getenv("VIVY_POSTGRES_TEST_DSN")
	if dsn == "" {
		return conformance.Slot{}, false
	}
	schema := notebookSchema(t)
	open := func() (storage.Engine, error) { return OpenSchema(context.Background(), dsn, schema) }
	b, err := open()
	if err != nil {
		t.Fatalf("OpenSchema: %v", err)
	}
	t.Cleanup(func() { _ = b.Close() })
	return conformance.Slot{
		Engine: b,
		Reopen: func() (storage.Engine, error) {
			_ = b.Close() // release the organism lease before the reopen
			reopened, err := open()
			if err != nil {
				return nil, err
			}
			t.Cleanup(func() { _ = reopened.Close() })
			return reopened, nil
		},
	}, true
}

func TestNotebookContract(t *testing.T) {
	slot, ok := notebookSlot(t)
	if !ok {
		t.Skip("VIVY_POSTGRES_TEST_DSN not set")
	}
	conformance.AssertNotebookContract(t, slot)
}

// TestNotebookLegacyImport applies head-035 schema on a fresh Postgres schema,
// seeds legacy notes (including one above the new write bound), then applies
// the real catalog and verifies exact import and scope isolation.
func TestNotebookLegacyImport(t *testing.T) {
	dsn := os.Getenv("VIVY_POSTGRES_TEST_DSN")
	if dsn == "" {
		t.Skip("VIVY_POSTGRES_TEST_DSN not set")
	}
	ctx := context.Background()
	schema := notebookSchema(t)
	root, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := root.ExecContext(ctx, fmt.Sprintf(`CREATE SCHEMA %s`, schema)); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = root.ExecContext(context.Background(), fmt.Sprintf(`DROP SCHEMA %s CASCADE`, schema))
		_ = root.Close()
	})
	raw, err := sql.Open("pgx", dsn+"&search_path="+schema)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := raw.ExecContext(ctx, `SET search_path TO `+schema); err != nil {
		t.Fatal(err)
	}
	manifest, err := migrations.Embedded()
	if err != nil {
		t.Fatal(err)
	}
	fsys := fstest.MapFS{}
	for _, d := range []migrations.Dialect{migrations.SQLite, migrations.Postgres} {
		for _, m := range manifest.Migrations(d) {
			if m.Version > 35 {
				continue
			}
			base := m.Path
			if i := strings.LastIndexByte(base, '/'); i >= 0 {
				base = base[i+1:]
			}
			fsys[string(d)+"/"+base] = &fstest.MapFile{Data: []byte(m.SQL)}
		}
	}
	head35, err := migrations.Load(fsys)
	if err != nil {
		t.Fatalf("head-35 manifest: %v", err)
	}
	if err := migrations.ApplyManifest(ctx, raw, migrations.Postgres, head35); err != nil {
		t.Fatalf("apply head-35: %v", err)
	}
	big := strings.Repeat("z", nb.MaxBodyBytes+2048)
	for _, n := range []struct {
		id, body string
		at       int64
	}{
		{"note-a", "alpha body", 111},
		{"note-big", big, 222},
	} {
		if _, err := raw.ExecContext(ctx, `INSERT INTO notes (id, content, created_at) VALUES ($1,$2,$3)`, n.id, []byte(n.body), n.at); err != nil {
			t.Fatal(err)
		}
	}
	// Apply the real migration tail inside the schema.
	if err := migrations.ApplyManifest(ctx, raw, migrations.Postgres, manifest); err != nil {
		t.Fatalf("apply 036: %v", err)
	}
	_ = raw.Close()

	b, err := OpenSchema(ctx, dsn, schema)
	if err != nil {
		t.Fatalf("OpenSchema: %v", err)
	}
	defer func() { _ = b.Close() }()
	s := b.Notebook()
	view, err := s.GetEntry(ctx, nb.HomeScopeID, nb.GetEntryRequest{EntryID: "note-a"})
	if err != nil {
		t.Fatalf("imported get: %v", err)
	}
	if view.Revision.ID != "legacy:note-a" || view.Revision.Markdown != "alpha body" ||
		view.Revision.Origin != nb.OriginLegacy || view.Revision.CreatedAt != 111 ||
		view.Entry.SectionID != "section-notes" {
		t.Fatalf("imported view = %+v", view)
	}
	bigView, err := s.GetEntry(ctx, nb.HomeScopeID, nb.GetEntryRequest{EntryID: "note-big"})
	if err != nil || bigView.Revision.Markdown != big {
		t.Fatalf("oversized import lost bytes: err=%v len=%d", err, len(bigView.Revision.Markdown))
	}
	page, err := s.ListEntries(ctx, nb.WorkspaceScope("other"), nb.ListEntriesRequest{})
	if err != nil || len(page.Entries) != 0 {
		t.Fatalf("workspace leaked: %+v err=%v", page, err)
	}
	// sqlite parity: identical export shape is asserted by the CLI tests; here
	// the parity contract is same IDs/receipts under the same calls.
	_ = view
}

func TestNotebookMissingScopeIsolation(t *testing.T) {
	slot, ok := notebookSlot(t)
	if !ok {
		t.Skip("VIVY_POSTGRES_TEST_DSN not set")
	}
	s := slot.Engine.Notebook()
	ctx := context.Background()
	if _, err := s.GetEntry(ctx, nb.HomeScopeID, nb.GetEntryRequest{EntryID: "nope"}); !errors.Is(err, nb.ErrNotFound) {
		t.Fatalf("missing = %v, want not_found", err)
	}
}
