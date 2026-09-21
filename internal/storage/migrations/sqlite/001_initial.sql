
CREATE TABLE sessions (
	id TEXT PRIMARY KEY,
	title TEXT NOT NULL,
	created_at INTEGER NOT NULL
);

CREATE TABLE messages (
	id TEXT PRIMARY KEY,
	session_id TEXT NOT NULL,
	run_id TEXT NOT NULL DEFAULT '',
	role TEXT NOT NULL,
	created_at INTEGER NOT NULL,
	content BLOB NOT NULL,
	FOREIGN KEY(session_id) REFERENCES sessions(id)
);

CREATE TABLE runs (
	id TEXT PRIMARY KEY,
	session_id TEXT NOT NULL,
	status TEXT NOT NULL,
	created_at INTEGER NOT NULL,
	FOREIGN KEY(session_id) REFERENCES sessions(id)
);

CREATE TABLE run_events (
	run_id TEXT NOT NULL,
	seq INTEGER NOT NULL,
	type TEXT NOT NULL,
	created_at INTEGER NOT NULL,
	payload_version INTEGER NOT NULL,
	payload BLOB NOT NULL,
	PRIMARY KEY(run_id, seq)
);

CREATE TABLE approvals (
	id TEXT PRIMARY KEY,
	run_id TEXT NOT NULL,
	tool_call_id TEXT NOT NULL,
	decision TEXT NOT NULL,
	expires_at INTEGER NOT NULL
);

CREATE TABLE snapshots (
	key TEXT PRIMARY KEY,
	value BLOB NOT NULL,
	version INTEGER NOT NULL
);

CREATE TABLE checkpoints (
	id TEXT PRIMARY KEY,
	generation INTEGER NOT NULL,
	checksum TEXT NOT NULL,
	created_at INTEGER NOT NULL
);

CREATE TABLE checkpoint_generations (
	id TEXT NOT NULL,
	generation INTEGER NOT NULL,
	blob BLOB NOT NULL,
	PRIMARY KEY(id, generation)
);

CREATE TABLE leases (
	key TEXT PRIMARY KEY,
	owner TEXT NOT NULL,
	expires_at INTEGER NOT NULL
);
