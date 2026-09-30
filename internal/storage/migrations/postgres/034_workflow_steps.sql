-- S11-C: INOFY workflow-step durable state (G4). Paired with the SQLite
-- migration of the same number.
ALTER TABLE workflow_revisions ADD COLUMN program_digest TEXT;
ALTER TABLE workflow_revisions ADD COLUMN catalog_digest TEXT;
ALTER TABLE workflow_revisions ADD COLUMN compiler_version TEXT;
ALTER TABLE workflow_revisions ADD COLUMN eino_build TEXT;
ALTER TABLE workflow_revisions ADD COLUMN input_digest TEXT;
ALTER TABLE workflow_revisions ADD COLUMN effective_limits BYTEA;
ALTER TABLE workflow_revisions ADD COLUMN host_binding_id TEXT;

CREATE TABLE workflow_executions (
	workflow_run_id TEXT PRIMARY KEY,
	status TEXT NOT NULL,
	epoch BIGINT NOT NULL,
	revision BIGINT NOT NULL,
	usage BYTEA,
	checkpoint BYTEA,
	checkpoint_blob_id TEXT,
	checkpoint_digest TEXT,
	waits BYTEA,
	interrupts BYTEA,
	gates BYTEA,
	resume_key TEXT,
	resume_answers_digest TEXT,
	unresolved BYTEA,
	updated_at BIGINT NOT NULL,
	FOREIGN KEY (workflow_run_id) REFERENCES runs(id) ON DELETE CASCADE
);

CREATE TABLE workflow_commits (
	workflow_run_id TEXT NOT NULL,
	commit_id TEXT NOT NULL,
	digest TEXT NOT NULL,
	first_seq BIGINT NOT NULL,
	last_seq BIGINT NOT NULL,
	revision BIGINT NOT NULL,
	created_at BIGINT NOT NULL,
	PRIMARY KEY (workflow_run_id, commit_id),
	FOREIGN KEY (workflow_run_id) REFERENCES runs(id) ON DELETE CASCADE
);

CREATE TABLE workflow_results (
	workflow_run_id TEXT NOT NULL,
	path TEXT NOT NULL,
	attempt BIGINT NOT NULL,
	digest TEXT NOT NULL,
	blob_id TEXT NOT NULL,
	bytes BIGINT NOT NULL,
	created_at BIGINT NOT NULL,
	PRIMARY KEY (workflow_run_id, path, attempt),
	FOREIGN KEY (workflow_run_id) REFERENCES runs(id) ON DELETE CASCADE
);
