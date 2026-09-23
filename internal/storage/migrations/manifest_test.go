package migrations

import (
	"io/fs"
	"testing"
	"testing/fstest"
)

func TestManifestValidatesPairedCatalog(t *testing.T) {
	files := fstest.MapFS{
		"sqlite/001_initial.sql":   &fstest.MapFile{Data: []byte("CREATE TABLE sessions (id TEXT PRIMARY KEY);\n")},
		"sqlite/002_notes.sql":     &fstest.MapFile{Data: []byte("CREATE TABLE notes (id TEXT PRIMARY KEY);\n")},
		"postgres/001_initial.sql": &fstest.MapFile{Data: []byte("CREATE TABLE sessions (id TEXT PRIMARY KEY);\n")},
		"postgres/002_notes.sql":   &fstest.MapFile{Data: []byte("CREATE TABLE notes (id TEXT PRIMARY KEY);\n")},
	}

	manifest, err := Load(files)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if err := manifest.Validate(); err != nil {
		t.Fatalf("Manifest.Validate: %v", err)
	}

	got := manifest.Migrations(SQLite)
	if len(got) != 2 || got[0].Version != 1 || got[0].Name != "initial" || got[1].Version != 2 || got[1].Name != "notes" {
		t.Fatalf("SQLite migrations = %+v", got)
	}
	if got[0].Checksum == "" || got[0].Checksum != manifest.Migrations(Postgres)[0].Checksum {
		t.Fatalf("paired checksum = %q / %q", got[0].Checksum, manifest.Migrations(Postgres)[0].Checksum)
	}
}

func TestEmbeddedManifestHasCanonicalPairs(t *testing.T) {
	manifest, err := Embedded()
	if err != nil {
		t.Fatalf("Embedded: %v", err)
	}
	if got := len(manifest.Migrations(SQLite)); got != 25 {
		t.Fatalf("embedded SQLite migration count = %d, want 25", got)
	}
	if got := len(manifest.Migrations(Postgres)); got != 25 {
		t.Fatalf("embedded PostgreSQL migration count = %d, want 25", got)
	}
	for i, migration := range manifest.Migrations(SQLite) {
		postgres := manifest.Migrations(Postgres)[i]
		if migration.Version != postgres.Version || migration.Name != postgres.Name {
			t.Fatalf("migration pair %d = %d/%s vs %d/%s", i, migration.Version, migration.Name, postgres.Version, postgres.Name)
		}
		if migration.Checksum == "" || postgres.Checksum == "" {
			t.Fatalf("migration pair %d has empty checksum", migration.Version)
		}
	}
}

func TestManifestRejectsDuplicateVersion(t *testing.T) {
	files := pairedFiles(
		"sqlite/001_initial.sql", "sqlite/001_other.sql",
		"postgres/001_initial.sql", "postgres/001_other.sql",
	)

	err := ValidateFS(files)
	if err == nil || !containsError(err, "duplicate migration version 1") {
		t.Fatalf("ValidateFS error = %v, want duplicate version diagnostic", err)
	}
}

func TestManifestRejectsMissingVersion(t *testing.T) {
	files := pairedFiles(
		"sqlite/001_initial.sql", "sqlite/003_third.sql",
		"postgres/001_initial.sql", "postgres/003_third.sql",
	)

	err := ValidateFS(files)
	if err == nil || !containsError(err, "missing migration version 2") {
		t.Fatalf("ValidateFS error = %v, want missing version diagnostic", err)
	}
}

func TestManifestRejectsInvalidName(t *testing.T) {
	files := pairedFiles(
		"sqlite/001-Initial.sql",
		"postgres/001_initial.sql",
	)

	err := ValidateFS(files)
	if err == nil || !containsError(err, "invalid migration filename") {
		t.Fatalf("ValidateFS error = %v, want invalid filename diagnostic", err)
	}
}

func TestManifestRejectsMissingDialectPair(t *testing.T) {
	files := pairedFiles(
		"sqlite/001_initial.sql", "sqlite/002_notes.sql",
		"postgres/001_initial.sql",
	)

	err := ValidateFS(files)
	if err == nil || !containsError(err, "missing postgres migration 2") {
		t.Fatalf("ValidateFS error = %v, want missing dialect diagnostic", err)
	}
}

func TestManifestRejectsMismatchedDialectName(t *testing.T) {
	files := pairedFiles(
		"sqlite/001_initial.sql",
		"postgres/001_bootstrap.sql",
	)

	err := ValidateFS(files)
	if err == nil || !containsError(err, "migration 1 name mismatch") {
		t.Fatalf("ValidateFS error = %v, want name mismatch diagnostic", err)
	}
}

func pairedFiles(paths ...string) fstest.MapFS {
	files := fstest.MapFS{}
	for _, path := range paths {
		files[path] = &fstest.MapFile{Data: []byte(path)}
	}
	return files
}

func containsError(err error, want string) bool {
	return err != nil && stringsContains(err.Error(), want)
}

func stringsContains(value, want string) bool {
	for i := 0; i+len(want) <= len(value); i++ {
		if value[i:i+len(want)] == want {
			return true
		}
	}
	return false
}

var _ fs.FS = fstest.MapFS{}
