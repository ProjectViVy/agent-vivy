
CREATE TABLE IF NOT EXISTS file_versions (
	id BIGSERIAL PRIMARY KEY,
	session_id TEXT NOT NULL,
	run_id TEXT NOT NULL DEFAULT '',
	path TEXT NOT NULL,
	version BIGINT NOT NULL,
	content_hash TEXT NOT NULL,
	content BYTEA NOT NULL,
	created_at BIGINT NOT NULL,
	UNIQUE(session_id, path, version)
);
CREATE INDEX IF NOT EXISTS file_versions_session_path_idx ON file_versions(session_id, path, version DESC);

CREATE TABLE IF NOT EXISTS file_reads (
	session_id TEXT NOT NULL,
	path TEXT NOT NULL,
	read_at BIGINT NOT NULL,
	PRIMARY KEY(session_id, path)
);
