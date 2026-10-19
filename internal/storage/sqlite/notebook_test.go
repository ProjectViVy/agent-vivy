package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"agent-vivy/internal/storage"

	nb "agent-vivy/internal/notebookcontract"
	"agent-vivy/internal/storage/conformance"
	"agent-vivy/internal/storage/migrations"
)

func notebookSlot(t *testing.T) conformance.Slot {
	t.Helper()
	path := filepath.Join(t.TempDir(), "vivy-nb.db")
	open := func() (storage.Engine, error) { return Open(context.Background(), path) }
	b, err := open()
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = b.Close() })
	return conformance.Slot{
		Engine: b,
		Reopen: func() (storage.Engine, error) {
			reopened, err := open()
			if err != nil {
				return nil, err
			}
			t.Cleanup(func() { _ = reopened.Close() })
			return reopened, nil
		},
	}
}

func TestNotebookContract(t *testing.T) {
	conformance.AssertNotebookContract(t, notebookSlot(t))
}

// TestNotebookLegacyImport seeds legacy notes at migration head 035, applies
// the production catalog and verifies exact ID/content/timestamp import into
// the home scope — including a row larger than the new write bound.
func TestNotebookLegacyImport(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "vivy-legacy.db")
	raw, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatalf("open raw: %v", err)
	}
	manifest, err := migrations.Embedded()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := raw.ExecContext(ctx, `CREATE TABLE schema_migrations (
		version BIGINT PRIMARY KEY, name TEXT NOT NULL, checksum TEXT NOT NULL, applied_at BIGINT NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	var applied int64
	for _, m := range manifest.Migrations(migrations.SQLite) {
		if m.Version > 35 {
			break
		}
		if _, err := raw.ExecContext(ctx, m.SQL); err != nil {
			t.Fatalf("apply %d %s: %v", m.Version, m.Name, err)
		}
		if _, err := raw.ExecContext(ctx,
			`INSERT INTO schema_migrations (version, name, checksum, applied_at) VALUES (?,?,?,?)`,
			m.Version, m.Name, m.Checksum, time.Now().UnixMilli()); err != nil {
			t.Fatal(err)
		}
		applied = m.Version
	}
	if applied != 35 {
		t.Fatalf("fixture stopped at %d, want 35", applied)
	}
	big := strings.Repeat("z", nb.MaxBodyBytes+2048)
	for _, n := range []struct {
		id, body string
		at       int64
	}{
		{"note-a", "alpha body", 111},
		{"note-big", big, 222},
	} {
		if _, err := raw.ExecContext(ctx, `INSERT INTO notes (id, content, created_at) VALUES (?,?,?)`, n.id, n.body, n.at); err != nil {
			t.Fatal(err)
		}
	}
	if err := raw.Close(); err != nil {
		t.Fatal(err)
	}

	b, err := Open(ctx, path)
	if err != nil {
		t.Fatalf("Open with import: %v", err)
	}
	defer func() { _ = b.Close() }()
	s := b.Notebook()
	view, err := s.GetEntry(ctx, nb.HomeScopeID, nb.GetEntryRequest{EntryID: "note-a"})
	if err != nil {
		t.Fatalf("imported get: %v", err)
	}
	if view.Revision.ID != "legacy:note-a" || view.Revision.Markdown != "alpha body" ||
		view.Revision.Origin != nb.OriginLegacy || view.Revision.CreatedAt != 111 ||
		view.Entry.SectionID != "section-notes" || view.Entry.Kind != nb.EntryNote {
		t.Fatalf("imported view = %+v", view)
	}
	bigView, err := s.GetEntry(ctx, nb.HomeScopeID, nb.GetEntryRequest{EntryID: "note-big"})
	if err != nil {
		t.Fatalf("oversized import get: %v", err)
	}
	if bigView.Revision.Markdown != big {
		t.Fatalf("oversized imported body lost bytes (len=%d)", len(bigView.Revision.Markdown))
	}
	// Workspace scopes never receive legacy copies.
	ws := nb.WorkspaceScope("any")
	page, err := s.ListEntries(ctx, ws, nb.ListEntriesRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Entries) != 0 {
		t.Fatalf("legacy rows leaked into workspace scope: %+v", page.Entries)
	}
	// Reopen stays stable; no duplicate import.
	_ = b.Close()
	b2, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = b2.Close() }()
	revs, err := b2.Notebook().ListRevisions(ctx, nb.HomeScopeID, nb.ListRevisionsRequest{EntryID: "note-a"})
	if err != nil || len(revs.Revisions) != 1 {
		t.Fatalf("reopen duplicated import: %+v err=%v", revs, err)
	}
}

// TestNotebookInterruptedMigration proves a failed migration leaves no partial
// state and the real catalog then applies and imports cleanly.
func TestNotebookInterruptedMigration(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "vivy-interrupt.db")
	raw, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatal(err)
	}
	full, err := migrations.Embedded()
	if err != nil {
		t.Fatal(err)
	}
	fsys := fstest.MapFS{}
	for _, d := range []migrations.Dialect{migrations.SQLite, migrations.Postgres} {
		for _, m := range full.Migrations(d) {
			if m.Version > 35 {
				continue
			}
			fsys[string(d)+"/"+fileBase(m.Path)] = &fstest.MapFile{Data: []byte(m.SQL)}
		}
		fsys[string(d)+"/036_notebook_content.sql"] = &fstest.MapFile{Data: []byte("CREATE TABLE broken_notebook(")}
	}
	partial, err := migrations.Load(fsys)
	if err != nil {
		t.Fatalf("partial manifest: %v", err)
	}
	if err := migrations.ApplyManifest(ctx, raw, migrations.SQLite, partial); err == nil {
		t.Fatal("broken migration unexpectedly applied")
	}
	var count int
	if err := raw.QueryRowContext(ctx, `SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='notebook_entries'`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatal("interrupted migration leaked partial notebook tables")
	}
	// Seed legacy content on the last good head, then apply the real catalog.
	if _, err := raw.ExecContext(ctx, `INSERT INTO notes (id, content, created_at) VALUES ('n-irq','body',7)`); err != nil {
		t.Fatal(err)
	}
	if err := migrations.Apply(ctx, raw, migrations.SQLite); err != nil {
		t.Fatalf("real apply after interruption: %v", err)
	}
	if err := raw.Close(); err != nil {
		t.Fatal(err)
	}
	b, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = b.Close() }()
	view, err := b.Notebook().GetEntry(ctx, nb.HomeScopeID, nb.GetEntryRequest{EntryID: "n-irq"})
	if err != nil || view.Revision.ID != "legacy:n-irq" {
		t.Fatalf("post-interruption import = %+v err=%v", view, err)
	}
	var dup int
	if err := b.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM notebook_mutations`).Scan(&dup); err != nil || dup != 0 {
		t.Fatalf("partial receipts leaked: %d err=%v", dup, err)
	}
}

func fileBase(p string) string {
	if i := strings.LastIndexByte(p, '/'); i >= 0 {
		return p[i+1:]
	}
	return p
}

func TestNotebookEmptyDatabaseHasNoEntries(t *testing.T) {
	s := notebookSlot(t).Engine.Notebook()
	page, err := s.ListEntries(context.Background(), nb.HomeScopeID, nb.ListEntriesRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Entries) != 0 {
		t.Fatalf("empty database listed %d entries", len(page.Entries))
	}
	secs, err := s.ListSections(context.Background(), nb.HomeScopeID, nb.ListSectionsRequest{})
	if err != nil || len(secs.Sections) != 4 {
		t.Fatalf("empty database sections = %+v err=%v", secs, err)
	}
	if _, err := s.GetEntry(context.Background(), nb.HomeScopeID, nb.GetEntryRequest{EntryID: "nope"}); !errors.Is(err, nb.ErrNotFound) {
		t.Fatalf("missing entry = %v, want not_found", err)
	}
}
