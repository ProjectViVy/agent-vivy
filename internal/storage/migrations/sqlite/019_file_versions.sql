
	CREATE TABLE IF NOT EXISTS file_versions (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		session_id TEXT NOT NULL,
		run_id TEXT NOT NULL DEFAULT '',
		path TEXT NOT NULL,
		version INTEGER NOT NULL,
		content_hash TEXT NOT NULL,
		content BLOB NOT NULL,
		created_at INTEGER NOT NULL,
		UNIQUE(session_id, path, version)
	);
	CREATE INDEX IF NOT EXISTS file_versions_session_path_idx ON file_versions(session_id, path, version DESC);
	CREATE TABLE IF NOT EXISTS file_reads (
		session_id TEXT NOT NULL,
		path TEXT NOT NULL,
		read_at INTEGER NOT NULL,
		PRIMARY KEY(session_id, path)
	);
