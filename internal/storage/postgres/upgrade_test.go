package postgres

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"agent-vivy/internal/storage/migrations"
)

// schemaV14Fixture freezes the pre-provenance Journal DDL (the historical
// version-14 body as of commit 82ecf14). It is
// copied verbatim on purpose: future DDL drift must not silently invalidate
// the v14 -> latest in-place upgrade test below.
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

// TestMigrateRefusesV14BeforeWrites proves the runtime refuses a known legacy
// store without changing its metadata or historical event bytes. Executable
// normalization and preservation coverage lives in the finite artifact module.
func TestMigrateRefusesV14BeforeWrites(t *testing.T) {
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
	// the migration runner, so it is not part of the frozen DDL), the frozen v14
	// DDL, the version-14 marker, one legacy 9-column message row, and an
	// existing run.started event whose bytes must survive every later migration.
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
	if _, err := setup.ExecContext(ctx, `
		INSERT INTO sessions (id, title, created_at) VALUES ('sess-upg', 'upgrade', 1);
		INSERT INTO runs (id, session_id, status, created_at) VALUES ('run-upg', 'sess-upg', 'completed', 2);
		INSERT INTO run_events (run_id, seq, type, created_at, payload_version, payload)
		VALUES ('run-upg', 1, 'run.started', 3, 1, '{"provider":"legacy","model":"legacy-model"}');
	`); err != nil {
		t.Fatalf("insert legacy run.started fixture: %v", err)
	}
	if _, err := setup.ExecContext(ctx,
		`INSERT INTO messages (id, session_id, run_id, role, created_at, content, tool_call_id, tool_name, tool_args)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		"msg-upg-legacy", "sess-upg", "run-upg", "user", int64(1), []byte("hello from v14"),
		"", "", []byte{}); err != nil {
		t.Fatalf("insert legacy message row: %v", err)
	}
	if err := migrations.Apply(ctx, setup, migrations.Postgres); !errors.Is(err, migrations.ErrUpgradeRequired) {
		t.Fatalf("runtime old source = %v", err)
	}
	var count int
	if err := setup.QueryRow(`SELECT COUNT(*) FROM schema_migrations`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("refusal changed metadata: %d %v", count, err)
	}
	var cols int
	if err := setup.QueryRow(`SELECT COUNT(*) FROM information_schema.columns WHERE table_schema=current_schema() AND table_name='schema_migrations'`).Scan(&cols); err != nil || cols != 2 {
		t.Fatalf("refusal changed schema: %d %v", cols, err)
	}
	var payload []byte
	if err := setup.QueryRow(`SELECT payload FROM run_events WHERE run_id='run-upg' AND seq=1`).Scan(&payload); err != nil || string(payload) != `{"provider":"legacy","model":"legacy-model"}` {
		t.Fatalf("historical event changed: %v", err)
	}
	if err := setup.Close(); err != nil {
		t.Fatal(err)
	}
}
