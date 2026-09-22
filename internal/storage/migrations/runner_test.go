package migrations

import (
	"context"
	"database/sql"
	"path/filepath"
	"strconv"
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
	if count != 24 {
		t.Fatalf("migration count = %d, want 24", count)
	}
	var name, checksum string
	if err := db.QueryRowContext(ctx,
		`SELECT name, checksum FROM schema_migrations WHERE version = 24`).Scan(&name, &checksum); err != nil {
		t.Fatalf("read migration 24: %v", err)
	}
	if name != "history_positions" || len(checksum) != 64 {
		t.Fatalf("migration 24 metadata = %q/%q", name, checksum)
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

func TestContinuityMigrationHistoryPositionsBackfillsLegacyAndReopens(t *testing.T) {
	db := openMigrationTestDB(t)
	ctx := context.Background()
	full, err := Embedded()
	if err != nil { t.Fatal(err) }
	legacy := Manifest{byDialect: map[Dialect][]Migration{
		SQLite: append([]Migration(nil), full.Migrations(SQLite)[:23]...),
		Postgres: append([]Migration(nil), full.Migrations(Postgres)[:23]...),
	}}
	if err := ApplyManifest(ctx, db, SQLite, legacy); err != nil { t.Fatalf("Apply legacy: %v", err) }
	if _, err := db.ExecContext(ctx, `INSERT INTO sessions (id,title,created_at,updated_at,sandbox_mode,approval_policy,workspace_path) VALUES ('s','s',1,1,'workspace_write','ask','')`); err != nil { t.Fatal(err) }
	for _, row := range []struct{ id string; at int64 }{{"late-id", 10}, {"early-z", 1}, {"early-a", 1}} {
		if _, err := db.ExecContext(ctx, `INSERT INTO messages (id,session_id,run_id,role,created_at,content,tool_call_id,tool_name,tool_args,source,channel,chat_id,channel_message_id) VALUES (?,?, '', 'user', ?, x'', '', '', x'', '', '', '', '')`, row.id, "s", row.at); err != nil { t.Fatal(err) }
	}
	if err := Apply(ctx, db, SQLite); err != nil { t.Fatalf("Apply 024: %v", err) }
	rows, err := db.QueryContext(ctx, `SELECT id, position FROM messages WHERE session_id = 's' ORDER BY position`)
	if err != nil { t.Fatal(err) }
	defer rows.Close()
	var got []string
	for rows.Next() { var id string; var position int; if err := rows.Scan(&id, &position); err != nil { t.Fatal(err) }; got = append(got, id+":"+strconv.Itoa(position)) }
	if strings.Join(got, ",") != "early-a:1,early-z:2,late-id:3" { t.Fatalf("legacy backfill = %v", got) }
	var next int
	if err := db.QueryRowContext(ctx, `SELECT next_message_position FROM sessions WHERE id = 's'`).Scan(&next); err != nil || next != 4 { t.Fatalf("next position = %d, %v", next, err) }
	if err := Apply(ctx, db, SQLite); err != nil { t.Fatalf("reapply 024: %v", err) }
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
