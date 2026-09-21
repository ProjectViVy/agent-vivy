
CREATE TABLE IF NOT EXISTS session_compactions (
	session_id TEXT NOT NULL,
	run_id TEXT NOT NULL,
	summary BYTEA NOT NULL,
	tail_from BIGINT NOT NULL,
	dropped_count BIGINT NOT NULL,
	created_at BIGINT NOT NULL,
	PRIMARY KEY(session_id, created_at, run_id),
	FOREIGN KEY(session_id) REFERENCES sessions(id)
);
CREATE INDEX IF NOT EXISTS session_compactions_session_idx ON session_compactions(session_id, created_at DESC);
