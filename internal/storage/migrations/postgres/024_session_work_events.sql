
CREATE TABLE session_work_events (
	session_id TEXT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
	work_seq BIGINT NOT NULL,
	kind TEXT NOT NULL,
	payload_version BIGINT NOT NULL,
	request_id TEXT NOT NULL,
	request_hash TEXT NOT NULL,
	created_at BIGINT NOT NULL,
	payload BYTEA NOT NULL,
	PRIMARY KEY (session_id, work_seq),
	UNIQUE (session_id, request_id)
);
CREATE INDEX session_work_events_request_idx ON session_work_events(session_id, request_id);
