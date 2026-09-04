package postgres

import (
	"context"
	"fmt"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"agent-vivy/internal/domain"
)

// schemaV14Fixture freezes the pre-provenance Journal DDL (the schemaV14
// body as of commit 82ecf14, when schemaV15Upgrade was introduced). It is
// copied verbatim on purpose: future DDL drift must not silently invalidate
// the v14 -> v15 in-place upgrade test below.
const schemaV14Fixture = `
CREATE TABLE sessions (
	id TEXT PRIMARY KEY,
	title TEXT NOT NULL,
	created_at BIGINT NOT NULL,
	sandbox_mode TEXT NOT NULL DEFAULT 'workspace_write',
	approval_policy TEXT NOT NULL DEFAULT 'ask'
);

CREATE TABLE messages (
	id TEXT PRIMARY KEY,
	session_id TEXT NOT NULL,
	run_id TEXT NOT NULL DEFAULT '',
	role TEXT NOT NULL,
	created_at BIGINT NOT NULL,
	content BYTEA NOT NULL,
	tool_call_id TEXT NOT NULL DEFAULT '',
	tool_name TEXT NOT NULL DEFAULT '',
	tool_args BYTEA NOT NULL DEFAULT ''::bytea
);

CREATE TABLE runs (
	id TEXT PRIMARY KEY,
	session_id TEXT NOT NULL,
	status TEXT NOT NULL,
	created_at BIGINT NOT NULL,
	kind TEXT NOT NULL DEFAULT 'primary',
	parent_run_id TEXT NOT NULL DEFAULT '',
	root_run_id TEXT NOT NULL DEFAULT '',
	depth BIGINT NOT NULL DEFAULT 0
);
CREATE INDEX runs_parent_idx ON runs(parent_run_id, created_at, id);
CREATE INDEX runs_root_idx ON runs(root_run_id, created_at, id);

CREATE TABLE run_events (
	run_id TEXT NOT NULL,
	seq BIGINT NOT NULL,
	type TEXT NOT NULL,
	created_at BIGINT NOT NULL,
	payload_version BIGINT NOT NULL,
	payload BYTEA NOT NULL,
	PRIMARY KEY(run_id, seq)
);

CREATE TABLE approvals (
	id TEXT PRIMARY KEY,
	run_id TEXT NOT NULL,
	tool_call_id TEXT NOT NULL,
	decision TEXT NOT NULL,
	expires_at BIGINT NOT NULL,
	resume_target TEXT NOT NULL DEFAULT '',
	kind TEXT NOT NULL DEFAULT 'run',
	action TEXT NOT NULL DEFAULT '',
	target TEXT NOT NULL DEFAULT '',
	precondition_hash TEXT NOT NULL DEFAULT '',
	preview BYTEA NOT NULL DEFAULT ''::bytea,
	risk_findings_json BYTEA NOT NULL DEFAULT '[]'::bytea,
	proposal_data BYTEA NOT NULL DEFAULT ''::bytea,
	tool_name TEXT NOT NULL DEFAULT '',
	created_at BIGINT NOT NULL DEFAULT 0,
	decided_at BIGINT NOT NULL DEFAULT 0,
	actor TEXT NOT NULL DEFAULT '',
	decision_reason BYTEA NOT NULL DEFAULT ''::bytea,
	stale_reason BYTEA NOT NULL DEFAULT ''::bytea,
	sandbox_mode TEXT NOT NULL DEFAULT 'workspace_write',
	approval_policy TEXT NOT NULL DEFAULT 'ask',
	timeout_at BIGINT NOT NULL DEFAULT 0
);
CREATE INDEX approvals_status_expiry_idx ON approvals(decision, expires_at, id);

CREATE TABLE snapshots (
	key TEXT PRIMARY KEY,
	value BYTEA NOT NULL,
	version BIGINT NOT NULL
);

CREATE TABLE checkpoints (
	id TEXT PRIMARY KEY,
	generation BIGINT NOT NULL,
	checksum TEXT NOT NULL,
	created_at BIGINT NOT NULL
);

CREATE TABLE checkpoint_generations (
	id TEXT NOT NULL,
	generation BIGINT NOT NULL,
	blob BYTEA NOT NULL,
	PRIMARY KEY(id, generation)
);

CREATE TABLE leases (
	key TEXT PRIMARY KEY,
	owner TEXT NOT NULL,
	expires_at BIGINT NOT NULL
);

CREATE TABLE notes (
	id TEXT PRIMARY KEY,
	content BYTEA NOT NULL,
	created_at BIGINT NOT NULL
);

CREATE TABLE questions (
	id TEXT PRIMARY KEY,
	run_id TEXT NOT NULL,
	tool_call_id TEXT NOT NULL,
	prompt BYTEA NOT NULL,
	answer BYTEA NOT NULL DEFAULT ''::bytea,
	status TEXT NOT NULL,
	expires_at BIGINT NOT NULL,
	resume_target TEXT NOT NULL DEFAULT '',
	created_at BIGINT NOT NULL DEFAULT 0,
	answered_at BIGINT NOT NULL DEFAULT 0,
	actor TEXT NOT NULL DEFAULT '',
	decision_reason BYTEA NOT NULL DEFAULT ''::bytea
);
CREATE INDEX questions_status_expiry_idx ON questions(status, expires_at, id);

CREATE TABLE skill_revisions (
	id TEXT PRIMARY KEY,
	run_id TEXT NOT NULL,
	skill_name TEXT NOT NULL,
	action TEXT NOT NULL,
	target_path TEXT NOT NULL,
	payload BYTEA NOT NULL,
	base_hash TEXT NOT NULL,
	content_hash TEXT NOT NULL,
	preview BYTEA NOT NULL,
	warnings_json BYTEA NOT NULL,
	status TEXT NOT NULL,
	created_at BIGINT NOT NULL,
	applied_at BIGINT NOT NULL DEFAULT 0,
	before_payload BYTEA NOT NULL DEFAULT ''::bytea
);
CREATE INDEX skill_revisions_status_idx ON skill_revisions(status, created_at, id);

CREATE TABLE todos (
	id TEXT NOT NULL,
	session_id TEXT NOT NULL,
	subject TEXT NOT NULL,
	description TEXT NOT NULL,
	status TEXT NOT NULL,
	blocks_json BYTEA NOT NULL,
	blocked_by_json BYTEA NOT NULL,
	active_form TEXT NOT NULL,
	owner TEXT NOT NULL,
	metadata_json BYTEA NOT NULL,
	position BIGINT NOT NULL,
	created_at BIGINT NOT NULL,
	updated_at BIGINT NOT NULL,
	PRIMARY KEY(session_id, id)
);
CREATE INDEX todos_session_position_idx ON todos(session_id, position, id);

CREATE TABLE generations (
	id TEXT PRIMARY KEY,
	parent_id TEXT NOT NULL DEFAULT '',
	artifact_sha256 TEXT NOT NULL,
	source_ref TEXT NOT NULL DEFAULT '',
	recipe_json BYTEA NOT NULL,
	phase TEXT NOT NULL,
	created_at BIGINT NOT NULL
);

CREATE TABLE eval_runs (
	id TEXT PRIMARY KEY,
	candidate_id TEXT NOT NULL,
	baseline_id TEXT NOT NULL DEFAULT '',
	suite TEXT NOT NULL,
	verdict TEXT NOT NULL,
	journal_ref TEXT NOT NULL DEFAULT '',
	created_at BIGINT NOT NULL
);
CREATE INDEX eval_runs_candidate_idx ON eval_runs(candidate_id, created_at, id);

CREATE TABLE promotions (
	id TEXT PRIMARY KEY,
	from_id TEXT NOT NULL,
	to_id TEXT NOT NULL,
	eval_id TEXT NOT NULL,
	actor TEXT NOT NULL,
	phase TEXT NOT NULL,
	applies_at TEXT NOT NULL,
	created_at BIGINT NOT NULL
);
CREATE UNIQUE INDEX promotions_from_accepted_idx ON promotions(from_id) WHERE phase = 'accepted';

CREATE TABLE studio_events (
	seq BIGINT GENERATED BY DEFAULT AS IDENTITY PRIMARY KEY,
	type TEXT NOT NULL,
	object_id TEXT NOT NULL,
	created_at BIGINT NOT NULL,
	payload BYTEA NOT NULL
);

CREATE INDEX run_events_type_created_idx ON run_events(type, created_at);

CREATE TABLE session_compactions (
	session_id TEXT NOT NULL,
	run_id TEXT NOT NULL,
	summary BYTEA NOT NULL,
	tail_from BIGINT NOT NULL,
	dropped_count BIGINT NOT NULL,
	created_at BIGINT NOT NULL,
	PRIMARY KEY(session_id, created_at, run_id),
	FOREIGN KEY(session_id) REFERENCES sessions(id)
);
CREATE INDEX session_compactions_session_idx ON session_compactions(session_id, created_at DESC);
`

var upgradeSeq atomic.Uint64

// TestMigrateUpgradesV14InPlace drives the production OpenSchema path over a
// hand-built version-14 database: the upgrade must ALTER messages in place
// (legacy rows survive with empty provenance), record all pending versions, and keep
// accepting provenance-bearing appends. Conformance always bootstraps fresh
// schemas, so without this test the upgrade branch never executes in CI.
func TestMigrateUpgradesV14InPlace(t *testing.T) {
	dsn := os.Getenv("VIVY_POSTGRES_TEST_DSN")
	if dsn == "" {
		t.Skip("VIVY_POSTGRES_TEST_DSN not set")
	}
	ctx := context.Background()
	schema := fmt.Sprintf("upg_%d_%d", time.Now().UnixNano(), upgradeSeq.Add(1))

	admin, err := openPool(dsn, "")
	if err != nil {
		t.Fatalf("open admin pool: %v", err)
	}
	t.Cleanup(func() {
		_, _ = admin.ExecContext(context.Background(), `DROP SCHEMA IF EXISTS `+schema+` CASCADE`)
		_ = admin.Close()
	})
	if _, err := admin.ExecContext(ctx, `CREATE SCHEMA IF NOT EXISTS `+schema); err != nil {
		t.Fatalf("create schema %s: %v", schema, err)
	}

	// Build the version-14 shape by hand: schema_migrations (created by
	// migrate() itself, so it is not part of the frozen DDL), the frozen v14
	// DDL, the version-14 marker, and one legacy 9-column message row.
	setup, err := openPool(dsn, schema)
	if err != nil {
		t.Fatalf("open setup pool: %v", err)
	}
	if _, err := setup.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version BIGINT PRIMARY KEY,
			applied_at BIGINT NOT NULL
		)`); err != nil {
		t.Fatalf("create schema_migrations: %v", err)
	}
	if _, err := setup.ExecContext(ctx, schemaV14Fixture); err != nil {
		t.Fatalf("apply v14 fixture DDL: %v", err)
	}
	if _, err := setup.ExecContext(ctx,
		`INSERT INTO schema_migrations (version, applied_at) VALUES ($1, $2)`,
		14, time.Now().UnixMilli()); err != nil {
		t.Fatalf("record version 14: %v", err)
	}
	if _, err := setup.ExecContext(ctx,
		`INSERT INTO messages (id, session_id, run_id, role, created_at, content, tool_call_id, tool_name, tool_args)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		"msg-upg-legacy", "sess-upg", "", "user", int64(1), []byte("hello from v14"),
		"", "", []byte{}); err != nil {
		t.Fatalf("insert legacy message row: %v", err)
	}
	_ = setup.Close()

	b, err := OpenSchema(ctx, dsn, schema)
	if err != nil {
		t.Fatalf("OpenSchema over v14 database: %v", err)
	}
	t.Cleanup(func() { _ = b.Close() })

	// (a) migrate recorded versions 15 through 19 alongside the pre-existing 14.
	var versions []int64
	rows, err := admin.QueryContext(ctx,
		`SELECT version FROM `+schema+`.schema_migrations ORDER BY version`)
	if err != nil {
		t.Fatalf("read schema_migrations: %v", err)
	}
	for rows.Next() {
		var v int64
		if err := rows.Scan(&v); err != nil {
			t.Fatalf("scan version: %v", err)
		}
		versions = append(versions, v)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate schema_migrations: %v", err)
	}
	wantVersions := []int64{14, 15, 16, 17, 18, 19}
	if len(versions) != len(wantVersions) {
		t.Fatalf("schema_migrations = %v, want %v", versions, wantVersions)
	}
	for i, want := range wantVersions {
		if versions[i] != want {
			t.Fatalf("schema_migrations = %v, want %v", versions, wantVersions)
		}
	}

	var cronTables int
	if err := admin.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM information_schema.tables
		 WHERE table_schema = $1 AND table_name = 'cron_jobs'`, schema).Scan(&cronTables); err != nil {
		t.Fatalf("check cron_jobs table: %v", err)
	}
	if cronTables != 1 {
		t.Fatalf("cron_jobs tables = %d, want 1", cronTables)
	}

	// (b) information_schema reports the 4 new columns as NOT NULL DEFAULT ''.
	for _, col := range []string{"source", "channel", "chat_id", "channel_message_id"} {
		var nullable, def string
		err := admin.QueryRowContext(ctx,
			`SELECT is_nullable, column_default
			 FROM information_schema.columns
			 WHERE table_schema = $1 AND table_name = 'messages' AND column_name = $2`,
			schema, col).Scan(&nullable, &def)
		if err != nil {
			t.Fatalf("information_schema column %s: %v", col, err)
		}
		if nullable != "NO" || def != "''" {
			t.Fatalf("column %s = nullable %q default %q, want NO / ''", col, nullable, def)
		}
	}

	// (c) the legacy row survives the in-place upgrade with empty provenance.
	got, err := b.ListMessages(ctx, "sess-upg")
	if err != nil {
		t.Fatalf("ListMessages: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("messages after upgrade = %d, want the 1 legacy row", len(got))
	}
	legacy := got[0]
	if legacy.ID != "msg-upg-legacy" || legacy.Role != domain.RoleUser || legacy.Content != "hello from v14" {
		t.Fatalf("legacy row drifted: %+v", legacy)
	}
	if legacy.Source != "" || legacy.Channel != "" || legacy.ChatID != "" || legacy.ChannelMessageID != "" {
		t.Fatalf("legacy provenance = %+v, want all empty", legacy)
	}
	if legacy.EffectiveSource() != "ui" {
		t.Fatalf("legacy EffectiveSource = %q, want ui", legacy.EffectiveSource())
	}

	// (d) provenance-bearing appends round-trip through the upgraded table.
	if err := b.AppendMessage(ctx, domain.Message{
		ID: "msg-upg-channel", SessionID: "sess-upg", Role: domain.RoleUser,
		CreatedAt: 2, Content: "hello from v15",
		Source: "channel", Channel: "telegram", ChatID: "chat-123", ChannelMessageID: "tg-456",
	}); err != nil {
		t.Fatalf("AppendMessage: %v", err)
	}
	got, err = b.ListMessages(ctx, "sess-upg")
	if err != nil {
		t.Fatalf("ListMessages after append: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("messages = %d, want 2", len(got))
	}
	c := got[1]
	if c.ID != "msg-upg-channel" || c.Source != "channel" || c.Channel != "telegram" ||
		c.ChatID != "chat-123" || c.ChannelMessageID != "tg-456" || c.Content != "hello from v15" {
		t.Fatalf("channel row did not round-trip: %+v", c)
	}
	if c.EffectiveSource() != "channel" {
		t.Fatalf("channel EffectiveSource = %q, want channel", c.EffectiveSource())
	}
	if got[0].ID != "msg-upg-legacy" {
		t.Fatalf("legacy row displaced: %+v", got[0])
	}
}
