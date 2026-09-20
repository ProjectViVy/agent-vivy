package migrations

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

func TestApplyFreshAndReapplyIsNoOp(t *testing.T) {
	db := openMigrationTestDB(t)
	ctx := context.Background()

	if err := Apply(ctx, db, SQLite); err != nil {
		t.Fatalf("Apply fresh: %v", err)
	}

	var count int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM schema_migrations`).Scan(&count); err != nil {
		t.Fatalf("count migrations: %v", err)
	}
	if count != 23 {
		t.Fatalf("migration count = %d, want 23", count)
	}
	var name, checksum string
	if err := db.QueryRowContext(ctx,
		`SELECT name, checksum FROM schema_migrations WHERE version = 23`).Scan(&name, &checksum); err != nil {
		t.Fatalf("read migration 23: %v", err)
	}
	if name != "workspace_path" || len(checksum) != 64 {
		t.Fatalf("migration 23 metadata = %q/%q", name, checksum)
	}

	if err := Apply(ctx, db, SQLite); err != nil {
		t.Fatalf("Apply again: %v", err)
	}
	var reapplied int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM schema_migrations`).Scan(&reapplied); err != nil {
		t.Fatalf("count re-applied migrations: %v", err)
	}
	if reapplied != count {
		t.Fatalf("re-applied migration count = %d, want %d", reapplied, count)
	}
}

func TestApplyRejectsChecksumDrift(t *testing.T) {
	db := openMigrationTestDB(t)
	ctx := context.Background()
	if err := Apply(ctx, db, SQLite); err != nil {
		t.Fatalf("Apply fresh: %v", err)
	}
	if _, err := db.ExecContext(ctx, `UPDATE schema_migrations SET checksum = 'drift' WHERE version = 1`); err != nil {
		t.Fatalf("change checksum: %v", err)
	}

	err := Apply(ctx, db, SQLite)
	if err == nil || !strings.Contains(err.Error(), "checksum drift") {
		t.Fatalf("Apply after checksum drift = %v, want checksum drift error", err)
	}
}

func TestApplyRollsBackFailedMigration(t *testing.T) {
	db := openMigrationTestDB(t)
	ctx := context.Background()
	manifest := testManifest(
		Migration{Version: 1, Name: "initial", SQL: "CREATE TABLE runner_base (id INTEGER);", Checksum: "base"},
		Migration{Version: 2, Name: "failure", SQL: "CREATE TABLE runner_partial (id INTEGER); SELECT * FROM missing_table;", Checksum: "failure"},
	)

	err := ApplyManifest(ctx, db, SQLite, manifest)
	if err == nil || !strings.Contains(err.Error(), "apply migration 2") {
		t.Fatalf("ApplyManifest = %v, want migration failure", err)
	}
	if tableExists(t, db, "runner_partial") {
		t.Fatal("failed migration table survived rollback")
	}
	var count int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM schema_migrations`).Scan(&count); err != nil {
		t.Fatalf("read migration markers: %v", err)
	}
	if count != 1 {
		t.Fatalf("migration marker count = %d, want 1", count)
	}
}

func TestApplyBackfillsLegacySQLiteMetadata(t *testing.T) {
	db := openMigrationTestDB(t)
	ctx := context.Background()
	if _, err := db.ExecContext(ctx, `
		CREATE TABLE schema_migrations (
			version INTEGER PRIMARY KEY,
			applied_at INTEGER NOT NULL
		);
		INSERT INTO schema_migrations (version, applied_at) VALUES (1, 1);
	`); err != nil {
		t.Fatalf("create legacy metadata: %v", err)
	}
	manifest := testManifest(
		Migration{Version: 1, Name: "initial", SQL: "SELECT 1;", Checksum: "legacy-checksum"},
		Migration{Version: 2, Name: "pending", SQL: "CREATE TABLE legacy_pending (id INTEGER);", Checksum: "pending-checksum"},
	)

	if err := ApplyManifest(ctx, db, SQLite, manifest); err != nil {
		t.Fatalf("ApplyManifest legacy: %v", err)
	}
	var name, checksum string
	if err := db.QueryRowContext(ctx,
		`SELECT name, checksum FROM schema_migrations WHERE version = 1`).Scan(&name, &checksum); err != nil {
		t.Fatalf("read backfilled metadata: %v", err)
	}
	if name != "initial" || checksum != "legacy-checksum" {
		t.Fatalf("backfilled metadata = %q/%q", name, checksum)
	}
	if !tableExists(t, db, "legacy_pending") {
		t.Fatal("pending migration was not applied")
	}
}

func openMigrationTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", "file:"+filepath.Join(t.TempDir(), "migrations.db"))
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func testManifest(items ...Migration) Manifest {
	return Manifest{byDialect: map[Dialect][]Migration{
		SQLite:   append([]Migration(nil), items...),
		Postgres: append([]Migration(nil), items...),
	}}
}

func tableExists(t *testing.T, db *sql.DB, name string) bool {
	t.Helper()
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?`, name).Scan(&count); err != nil {
		t.Fatalf("check table %s: %v", name, err)
	}
	return count == 1
}
