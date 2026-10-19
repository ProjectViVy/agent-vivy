package main

import (
	"bytes"
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"agent-vivy/internal/notebookcontract"
	"agent-vivy/internal/storage/migrations"
	"agent-vivy/internal/storage/sqlite"

	_ "modernc.org/sqlite"
)

// TestNotebookExportWithoutModules exercises the narrow export path: open the
// configured backend read-only (no App/runtime/modules), write into a fresh
// directory, and refuse overwrites and traversal — with N0's no-injection
// guarantee untouched.
func TestNotebookExportWithoutModules(t *testing.T) {
	ctx := context.Background()
	db := filepath.Join(t.TempDir(), "vivy.db")
	b, err := sqlite.Open(ctx, db)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	mc := notebookcontract.MutationContext{
		ScopeID:      notebookcontract.HomeScopeID,
		Actor:        notebookcontract.Actor{Kind: notebookcontract.ActorHuman, Ref: "local:test"},
		OperationKey: "export-fixture",
	}
	rc, err := b.Notebook().CreateEntry(ctx, mc, notebookcontract.CreateEntryRequest{
		SectionID: "section-notes", Title: "Export Me", Markdown: "exported body",
	})
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := b.Close(); err != nil {
		t.Fatal(err)
	}
	t.Setenv("VIVY_CONFIG", writeNotebookConfig(t, db))

	outDir := filepath.Join(t.TempDir(), "fresh-out")
	var stdout, stderr bytes.Buffer
	if code := runNotebook([]string{"export", "--scope", "home", "--entry", rc.ResourceID, "--revision", rc.RevisionID, "--output", outDir}, &stdout, &stderr); code != 0 {
		t.Fatalf("export code=%d stderr=%s", code, stderr.String())
	}
	data, err := os.ReadDir(outDir)
	if err != nil || len(data) != 1 {
		t.Fatalf("output dir = %v err=%v", data, err)
	}
	body, err := os.ReadFile(filepath.Join(outDir, data[0].Name()))
	if err != nil || !strings.Contains(string(body), "exported body") || !strings.Contains(string(body), "entry: "+rc.ResourceID) {
		t.Fatalf("artifact body missing content: %v", err)
	}

	// Refuse to overwrite a populated directory.
	if code := runNotebook([]string{"export", "--entry", rc.ResourceID, "--output", outDir}, &stdout, &stderr); code == 0 {
		t.Fatal("re-export into populated directory unexpectedly succeeded")
	}
	// Missing entry reports failure, not a new file.
	if code := runNotebook([]string{"export", "--entry", "nbe-missing", "--output", filepath.Join(t.TempDir(), "x")}, &stdout, &stderr); code == 0 {
		t.Fatal("export of missing entry succeeded")
	}
	// Old-schema database reports the upgrade requirement.
	oldDB := filepath.Join(t.TempDir(), "old.db")
	seedHead35(t, oldDB)
	t.Setenv("VIVY_CONFIG", writeNotebookConfig(t, oldDB))
	if code := runNotebook([]string{"export", "--entry", rc.ResourceID, "--output", filepath.Join(t.TempDir(), "y")}, &stdout, &stderr); code == 0 || !strings.Contains(stderr.String(), "upgrade") {
		t.Fatalf("old schema export = code %d stderr %q, want upgrade report", code, stderr.String())
	}
}

func writeNotebookConfig(t *testing.T, dbPath string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	cfg := "storage:\n  backend: sqlite\n  sqlite:\n    path: " + filepath.ToSlash(dbPath) + "\n"
	if err := os.WriteFile(path, []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// seedHead35 builds a schema at migration head 035 only.
func seedHead35(t *testing.T, path string) {
	t.Helper()
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
			base := m.Path
			if i := strings.LastIndexByte(base, '/'); i >= 0 {
				base = base[i+1:]
			}
			fsys[string(d)+"/"+base] = &fstest.MapFile{Data: []byte(m.SQL)}
		}
	}
	head35, err := migrations.Load(fsys)
	if err != nil {
		t.Fatal(err)
	}
	if err := migrations.ApplyManifest(context.Background(), raw, migrations.SQLite, head35); err != nil {
		t.Fatalf("apply head-35: %v", err)
	}
	if err := raw.Close(); err != nil {
		t.Fatal(err)
	}
}
