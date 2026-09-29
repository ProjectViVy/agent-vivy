-- S11-C: INOFY workflow-step durable state (G4).
-- workflow_revisions gains the nullable INOFY admission identity columns
-- (schema_version 2 rows must populate all of them; legacy rows keep NULLs).
ALTER TABLE workflow_revisions ADD COLUMN program_digest TEXT;
ALTER TABLE workflow_revisions ADD COLUMN catalog_digest TEXT;
ALTER TABLE workflow_revisions ADD COLUMN compiler_version TEXT;
ALTER TABLE workflow_revisions ADD COLUMN eino_build TEXT;
ALTER TABLE workflow_revisions ADD COLUMN input_digest TEXT;
ALTER TABLE workflow_revisions ADD COLUMN effective_limits BLOB;
ALTER TABLE workflow_revisions ADD COLUMN host_binding_id TEXT;

-- One row per admitted INOFY execution: the mutable projection the atomic
-- step commits advance (status, writer epoch, waits, checkpoint references).
CREATE TABLE workflow_executions (
	workflow_run_id TEXT PRIMARY KEY,
	status TEXT NOT NULL,
	epoch INTEGER NOT NULL,
	revision INTEGER NOT NULL,
	usage BLOB,
	checkpoint BLOB,
	checkpoint_blob_id TEXT,
	checkpoint_digest TEXT,
	waits BLOB,
	interrupts BLOB,
	gates BLOB,
	resume_key TEXT,
	resume_answers_digest TEXT,
	unresolved BLOB,
	updated_at INTEGER NOT NULL,
	FOREIGN KEY (workflow_run_id) REFERENCES runs(id) ON DELETE CASCADE
);

-- Commit ledger for idempotent replay: same (run, commit_id) with the same
-- content digest replays to the stored receipt; a different digest conflicts.
CREATE TABLE workflow_commits (
	workflow_run_id TEXT NOT NULL,
	commit_id TEXT NOT NULL,
	digest TEXT NOT NULL,
	first_seq INTEGER NOT NULL,
	last_seq INTEGER NOT NULL,
	revision INTEGER NOT NULL,
	created_at INTEGER NOT NULL,
	PRIMARY KEY (workflow_run_id, commit_id),
	FOREIGN KEY (workflow_run_id) REFERENCES runs(id) ON DELETE CASCADE
);

-- Protected result references: output bytes live in workflow-scoped
-- content-addressed blobs; this row is the transaction-local index.
CREATE TABLE workflow_results (
	workflow_run_id TEXT NOT NULL,
	path TEXT NOT NULL,
	attempt INTEGER NOT NULL,
	digest TEXT NOT NULL,
	blob_id TEXT NOT NULL,
	bytes INTEGER NOT NULL,
	created_at INTEGER NOT NULL,
	PRIMARY KEY (workflow_run_id, path, attempt),
	FOREIGN KEY (workflow_run_id) REFERENCES runs(id) ON DELETE CASCADE
);
