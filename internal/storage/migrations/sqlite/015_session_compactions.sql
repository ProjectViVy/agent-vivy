
CREATE TABLE session_compactions (
	session_id TEXT NOT NULL,
	run_id TEXT NOT NULL,
	summary BLOB NOT NULL,
	tail_from INTEGER NOT NULL,
	dropped_count INTEGER NOT NULL,
	created_at INTEGER NOT NULL,
	PRIMARY KEY(session_id, created_at, run_id),
	FOREIGN KEY(session_id) REFERENCES sessions(id)
);
CREATE INDEX session_compactions_session_idx ON session_compactions(session_id, created_at DESC);
