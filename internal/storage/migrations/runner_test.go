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
	if count != 27 {
		t.Fatalf("migration count = %d, want 27", count)
	}
	var name, checksum string
	if err := db.QueryRowContext(ctx,
		`SELECT name, checksum FROM schema_migrations WHERE version = 25`).Scan(&name, &checksum); err != nil {
		t.Fatalf("read migration 25: %v", err)
	}
	if name != "truncation_run_id" || len(checksum) != 64 {
		t.Fatalf("migration 25 metadata = %q/%q", name, checksum)
	}
	for _, table := range []string{"mask_definitions", "session_mask_selections", "run_prompt_snapshots"} {
		if !tableExists(t, db, table) {
			t.Fatalf("migration 24 did not create %s", table)
		}
	}
	for version, want := range map[int]string{26: "session_work_events", 27: "history_work_anchors"} {
		if err := db.QueryRowContext(ctx,
			`SELECT name FROM schema_migrations WHERE version = ?`, version).Scan(&name); err != nil {
			t.Fatalf("read migration %d: %v", version, err)
		}
		if name != want {
			t.Fatalf("migration %d name = %q, want %q", version, name, want)
		}
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

func TestApplyUpgradesSQLite23To27AndReopens(t *testing.T) {
	ctx := context.Background()
	manifest, err := Embedded()
	if err != nil {
		t.Fatalf("Embedded: %v", err)
	}
	through23 := Manifest{byDialect: map[Dialect][]Migration{
		SQLite:   manifest.Migrations(SQLite)[:23],
		Postgres: manifest.Migrations(Postgres)[:23],
	}}
	path := filepath.Join(t.TempDir(), "upgrade-23.db")
	open := func() *sql.DB {
		db, err := sql.Open("sqlite", "file:"+path)
		if err != nil {
			t.Fatalf("sql.Open: %v", err)
		}
		db.SetMaxOpenConns(1)
		return db
	}

	db := open()
	if err := ApplyManifest(ctx, db, SQLite, through23); err != nil {
		t.Fatalf("apply through migration 23: %v", err)
	}
	if _, err := db.ExecContext(ctx, `
		INSERT INTO sessions (id, title, created_at) VALUES ('upgrade-session', 'preserved', 11);
		INSERT INTO runs (id, session_id, status, created_at) VALUES ('upgrade-run', 'upgrade-session', 'completed', 12);
	`); err != nil {
		t.Fatalf("seed version 23 data: %v", err)
	}
	if err := Apply(ctx, db, SQLite); err != nil {
		t.Fatalf("upgrade 23 to 27: %v", err)
	}
	assertSQLiteMaskMigration24(t, db)
	assertSQLiteUpgradeRows(t, db)
	if err := db.Close(); err != nil {
		t.Fatalf("close upgraded database: %v", err)
	}

	db = open()
	t.Cleanup(func() { _ = db.Close() })
	if err := Apply(ctx, db, SQLite); err != nil {
		t.Fatalf("apply after reopen: %v", err)
	}
	assertSQLiteMaskMigration24(t, db)
	assertSQLiteUpgradeRows(t, db)
}

func assertSQLiteMaskMigration24(t *testing.T, db *sql.DB) {
	t.Helper()
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM schema_migrations`).Scan(&count); err != nil {
		t.Fatalf("count upgraded migrations: %v", err)
	}
	if count != 27 {
		t.Fatalf("upgraded migration count = %d, want 27", count)
	}
	var name, checksum string
	if err := db.QueryRow(`SELECT name, checksum FROM schema_migrations WHERE version = 25`).Scan(&name, &checksum); err != nil {
		t.Fatalf("read upgraded migration 25 metadata: %v", err)
	}
	if name != "truncation_run_id" || len(checksum) != 64 {
		t.Fatalf("upgraded migration 25 metadata = %q/%q", name, checksum)
	}
	for _, table := range []string{"mask_definitions", "session_mask_selections", "run_prompt_snapshots"} {
		if !tableExists(t, db, table) {
			t.Fatalf("upgrade did not create %s", table)
		}
	}
}

func assertSQLiteUpgradeRows(t *testing.T, db *sql.DB) {
	t.Helper()
	var sessionTitle, runSession, runStatus string
	if err := db.QueryRow(`SELECT title FROM sessions WHERE id = 'upgrade-session'`).Scan(&sessionTitle); err != nil {
		t.Fatalf("read upgraded session: %v", err)
	}
	if err := db.QueryRow(`SELECT session_id, status FROM runs WHERE id = 'upgrade-run'`).Scan(&runSession, &runStatus); err != nil {
		t.Fatalf("read upgraded run: %v", err)
	}
	if sessionTitle != "preserved" || runSession != "upgrade-session" || runStatus != "completed" {
		t.Fatalf("upgraded rows = %q/%q/%q", sessionTitle, runSession, runStatus)
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
