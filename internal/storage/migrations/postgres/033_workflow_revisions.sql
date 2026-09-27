CREATE TABLE workflow_revisions (
	workflow_run_id TEXT PRIMARY KEY,
	parent_run_id TEXT NOT NULL,
	parent_session_id TEXT NOT NULL,
	root_run_id TEXT NOT NULL,
	operation_key TEXT NOT NULL,
	descriptor_digest TEXT NOT NULL CHECK (length(descriptor_digest) = 64),
	authority_digest TEXT NOT NULL CHECK (length(authority_digest) = 64),
	descriptor_json BYTEA NOT NULL,
	authority_json BYTEA NOT NULL,
	schema_version BIGINT NOT NULL CHECK (schema_version > 0),
	created_at BIGINT NOT NULL,
	UNIQUE (parent_run_id, operation_key),
	FOREIGN KEY (workflow_run_id) REFERENCES runs(id) ON DELETE CASCADE,
	FOREIGN KEY (parent_run_id) REFERENCES runs(id) ON DELETE CASCADE,
	FOREIGN KEY (parent_session_id) REFERENCES sessions(id) ON DELETE CASCADE
);
CREATE INDEX workflow_revisions_parent_idx ON workflow_revisions(parent_run_id, created_at, workflow_run_id);
