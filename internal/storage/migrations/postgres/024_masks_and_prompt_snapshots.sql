CREATE TABLE mask_definitions (
	 id TEXT PRIMARY KEY,
	 name TEXT NOT NULL,
	 description TEXT NOT NULL DEFAULT '',
	 body BYTEA NOT NULL,
	 revision BIGINT NOT NULL CHECK (revision > 0),
	 digest TEXT NOT NULL,
	 created_at BIGINT NOT NULL,
	 updated_at BIGINT NOT NULL,
	 create_operation_id TEXT NOT NULL UNIQUE,
	 create_request_digest TEXT NOT NULL
);

CREATE TABLE session_mask_selections (
	 session_id TEXT PRIMARY KEY,
	 mask_id TEXT NOT NULL DEFAULT '',
	 revision BIGINT NOT NULL CHECK (revision > 0),
	 FOREIGN KEY (session_id) REFERENCES sessions(id) ON DELETE CASCADE
);
CREATE INDEX session_mask_selections_mask_idx ON session_mask_selections(mask_id);

CREATE TABLE run_prompt_snapshots (
	 run_id TEXT PRIMARY KEY,
	 schema_version BIGINT NOT NULL CHECK (schema_version > 0),
	 composer_version TEXT NOT NULL,
	 generation_id TEXT NOT NULL,
	 payload BYTEA NOT NULL,
	 payload_sha256 TEXT NOT NULL,
	 FOREIGN KEY (run_id) REFERENCES runs(id) ON DELETE CASCADE
);
