package postgres

// schemaV15 is the current logical Journal schema (SQLite migration 16.
// Postgres bootstraps here in one step; later versions increment both engines.
const schemaV15 = `
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
	tool_args BYTEA NOT NULL DEFAULT ''::bytea,
	source TEXT NOT NULL DEFAULT '',
	channel TEXT NOT NULL DEFAULT '',
	chat_id TEXT NOT NULL DEFAULT '',
	channel_message_id TEXT NOT NULL DEFAULT ''
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

CREATE TABLE cron_jobs (
	id TEXT PRIMARY KEY,
	name TEXT NOT NULL,
	enabled BOOLEAN NOT NULL,
	schedule_json BYTEA NOT NULL,
	payload_json BYTEA NOT NULL,
	session_id TEXT NOT NULL DEFAULT '',
	next_run_at_ms BIGINT NOT NULL DEFAULT 0,
	last_run_at_ms BIGINT NOT NULL DEFAULT 0,
	last_status TEXT NOT NULL DEFAULT '',
	last_error TEXT NOT NULL DEFAULT '',
	delete_after_run BOOLEAN NOT NULL DEFAULT FALSE,
	created_at_ms BIGINT NOT NULL,
	updated_at_ms BIGINT NOT NULL
);
CREATE INDEX cron_jobs_next_run_idx ON cron_jobs(enabled, next_run_at_ms);
`

// schemaV15Upgrade upgrades a version-14 database in place: the same
// channel-provenance columns SQLite migration016 adds. Defaults keep
// existing rows valid — an empty source reads as ui.
const schemaV15Upgrade = `
ALTER TABLE messages ADD COLUMN source TEXT NOT NULL DEFAULT '';
ALTER TABLE messages ADD COLUMN channel TEXT NOT NULL DEFAULT '';
ALTER TABLE messages ADD COLUMN chat_id TEXT NOT NULL DEFAULT '';
ALTER TABLE messages ADD COLUMN channel_message_id TEXT NOT NULL DEFAULT '';
CREATE TABLE IF NOT EXISTS cron_jobs (
	id TEXT PRIMARY KEY,
	name TEXT NOT NULL,
	enabled BOOLEAN NOT NULL,
	schedule_json BYTEA NOT NULL,
	payload_json BYTEA NOT NULL,
	session_id TEXT NOT NULL DEFAULT '',
	next_run_at_ms BIGINT NOT NULL DEFAULT 0,
	last_run_at_ms BIGINT NOT NULL DEFAULT 0,
	last_status TEXT NOT NULL DEFAULT '',
	last_error TEXT NOT NULL DEFAULT '',
	delete_after_run BOOLEAN NOT NULL DEFAULT FALSE,
	created_at_ms BIGINT NOT NULL,
	updated_at_ms BIGINT NOT NULL
);
CREATE INDEX IF NOT EXISTS cron_jobs_next_run_idx ON cron_jobs(enabled, next_run_at_ms);
`
