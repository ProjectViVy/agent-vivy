
CREATE TABLE IF NOT EXISTS session_truncations (
	id BIGSERIAL PRIMARY KEY,
	session_id TEXT NOT NULL,
	cutoff_message_id TEXT NOT NULL,
	tail_message_id TEXT NOT NULL DEFAULT '',
	reason TEXT NOT NULL,
	fork_session_id TEXT NOT NULL DEFAULT '',
	created_at BIGINT NOT NULL,
	FOREIGN KEY(session_id) REFERENCES sessions(id)
);
CREATE INDEX IF NOT EXISTS session_truncations_session_idx ON session_truncations(session_id, id DESC);
