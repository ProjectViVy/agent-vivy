
CREATE TABLE session_work_events (
	session_id TEXT NOT NULL,
	work_seq INTEGER NOT NULL,
	kind TEXT NOT NULL,
	payload_version INTEGER NOT NULL,
	request_id TEXT NOT NULL,
	request_hash TEXT NOT NULL,
	created_at INTEGER NOT NULL,
	payload BLOB NOT NULL,
	PRIMARY KEY (session_id, work_seq),
	UNIQUE (session_id, request_id),
	FOREIGN KEY (session_id) REFERENCES sessions(id)
);
CREATE INDEX session_work_events_request_idx ON session_work_events(session_id, request_id);
