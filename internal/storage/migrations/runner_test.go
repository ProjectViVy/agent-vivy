package migrations

import (
	"bytes"
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
	if count != 33 {
		t.Fatalf("migration count = %d, want 33", count)
	}
	var name, checksum string
	if err := db.QueryRowContext(ctx,
		`SELECT name, checksum FROM schema_migrations WHERE version = 27`).Scan(&name, &checksum); err != nil {
		t.Fatalf("read migration 27: %v", err)
	}
	if name != "child_sessions" || len(checksum) != 64 {
		t.Fatalf("migration 27 metadata = %q/%q", name, checksum)
	}
	if err := db.QueryRowContext(ctx,
		`SELECT name, checksum FROM schema_migrations WHERE version = 28`).Scan(&name, &checksum); err != nil {
		t.Fatalf("read migration 28: %v", err)
	}
	if name != "workflow_revisions" || len(checksum) != 64 {
		t.Fatalf("migration 28 metadata = %q/%q", name, checksum)
	}
	for _, table := range []string{"mask_definitions", "session_mask_selections", "run_prompt_snapshots", "child_sessions", "child_mailbox_messages", "child_message_receipts", "workflow_revisions"} {
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
func TestApplyUpgradesSQLite23ToLatestAndReopens(t *testing.T) {
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
		INSERT INTO run_events (run_id, seq, type, created_at, payload_version, payload)
		VALUES ('upgrade-run', 1, 'run.started', 13, 1, '{"provider":"legacy","model":"legacy-model"}');
		INSERT INTO runs (id, session_id, status, created_at, kind, parent_run_id, root_run_id, depth)
		VALUES ('upgrade-child-run', 'upgrade-session', 'completed', 13, 'child', 'upgrade-run', 'upgrade-run', 1);
	`); err != nil {
		t.Fatalf("seed version 23 data: %v", err)
	}
	if err := Apply(ctx, db, SQLite); err != nil {
		t.Fatalf("upgrade 23 to 27: %v", err)
		t.Fatalf("upgrade 23 to latest: %v", err)
	}
	assertSQLiteLatestMigrations(t, db)
	assertSQLiteUpgradeRows(t, db)
	if err := db.Close(); err != nil {
		t.Fatalf("close upgraded database: %v", err)
	}

	db = open()
	t.Cleanup(func() { _ = db.Close() })
	if err := Apply(ctx, db, SQLite); err != nil {
		t.Fatalf("apply after reopen: %v", err)
	}
	assertSQLiteLatestMigrations(t, db)
	assertSQLiteUpgradeRows(t, db)
}

func assertSQLiteLatestMigrations(t *testing.T, db *sql.DB) {
	t.Helper()
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM schema_migrations`).Scan(&count); err != nil {
		t.Fatalf("count upgraded migrations: %v", err)
	}
	if count != 28 {
		t.Fatalf("upgraded migration count = %d, want 28", count)
	}
	var name, checksum string
	if err := db.QueryRow(`SELECT name, checksum FROM schema_migrations WHERE version = 28`).Scan(&name, &checksum); err != nil {
		t.Fatalf("read upgraded migration 28 metadata: %v", err)
	}
	if name != "workflow_revisions" || len(checksum) != 64 {
		t.Fatalf("upgraded migration 28 metadata = %q/%q", name, checksum)
	}
	for _, table := range []string{"mask_definitions", "session_mask_selections", "run_prompt_snapshots", "tool_operations", "child_sessions", "child_mailbox_messages", "child_message_receipts", "workflow_revisions"} {
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
	var eventRunID, eventType string
	var eventSeq, eventCreatedAt, payloadVersion int64
	var payload []byte
	if err := db.QueryRow(`SELECT run_id, seq, type, created_at, payload_version, payload FROM run_events WHERE run_id = 'upgrade-run' AND seq = 1`).
		Scan(&eventRunID, &eventSeq, &eventType, &eventCreatedAt, &payloadVersion, &payload); err != nil {
		t.Fatalf("read preserved run.started: %v", err)
	}
	wantPayload := []byte(`{"provider":"legacy","model":"legacy-model"}`)
	if eventRunID != "upgrade-run" || eventSeq != 1 || eventType != "run.started" || eventCreatedAt != 13 || payloadVersion != 1 || !bytes.Equal(payload, wantPayload) {
		t.Fatalf("preserved run.started = %q/%d/%q/%d/v%d/%s, want exact legacy event payload %s", eventRunID, eventSeq, eventType, eventCreatedAt, payloadVersion, payload, wantPayload)
	}
}

func TestContinuityMigrationHistoryPositionsBackfillsLegacyAndReopens(t *testing.T) {
	db := openMigrationTestDB(t)
	ctx := context.Background()
	full, err := Embedded()
	if err != nil {
		t.Fatal(err)
	}
	legacy := Manifest{byDialect: map[Dialect][]Migration{
		SQLite:   append([]Migration(nil), full.Migrations(SQLite)[:23]...),
		Postgres: append([]Migration(nil), full.Migrations(Postgres)[:23]...),
	}}
	if err := ApplyManifest(ctx, db, SQLite, legacy); err != nil {
		t.Fatalf("Apply legacy: %v", err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO sessions (id,title,created_at,updated_at,sandbox_mode,approval_policy,workspace_path) VALUES ('s','s',1,1,'workspace_write','ask','')`); err != nil {
		t.Fatal(err)
	}
	for _, row := range []struct {
		id string
		at int64
	}{{"late-id", 10}, {"early-z", 1}, {"early-a", 1}} {
		if _, err := db.ExecContext(ctx, `INSERT INTO messages (id,session_id,run_id,role,created_at,content,tool_call_id,tool_name,tool_args,source,channel,chat_id,channel_message_id) VALUES (?,?, '', 'user', ?, x'', '', '', x'', '', '', '', '')`, row.id, "s", row.at); err != nil {
			t.Fatal(err)
		}
	}
	if err := Apply(ctx, db, SQLite); err != nil {
		t.Fatalf("Apply 024: %v", err)
	}
	rows, err := db.QueryContext(ctx, `SELECT id, position FROM messages WHERE session_id = 's' ORDER BY position`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var got []string
	for rows.Next() {
		var id string
		var position int
		if err := rows.Scan(&id, &position); err != nil {
			t.Fatal(err)
		}
		got = append(got, id+":"+strconv.Itoa(position))
	}
	if strings.Join(got, ",") != "early-a:1,early-z:2,late-id:3" {
		t.Fatalf("legacy backfill = %v", got)
	}
	var next int
	if err := db.QueryRowContext(ctx, `SELECT next_message_position FROM sessions WHERE id = 's'`).Scan(&next); err != nil || next != 4 {
		t.Fatalf("next position = %d, %v", next, err)
	}
	if err := Apply(ctx, db, SQLite); err != nil {
		t.Fatalf("reapply 024: %v", err)
	}
	var childMode string
	if err := db.QueryRow(`SELECT child_mode FROM runs WHERE id = 'upgrade-child-run'`).Scan(&childMode); err != nil {
		t.Fatalf("read migrated child mode: %v", err)
	}
	if childMode != "one-shot" {
		t.Fatalf("migrated historical child mode = %q, want one-shot", childMode)
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
