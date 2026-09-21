
CREATE TABLE skill_revisions (
	 id TEXT PRIMARY KEY,
	 run_id TEXT NOT NULL,
	 skill_name TEXT NOT NULL,
	 action TEXT NOT NULL,
	 target_path TEXT NOT NULL,
	 payload BLOB NOT NULL,
	 base_hash TEXT NOT NULL,
	 content_hash TEXT NOT NULL,
	 preview BLOB NOT NULL,
	 warnings_json BLOB NOT NULL,
	 status TEXT NOT NULL,
	 created_at INTEGER NOT NULL,
	 applied_at INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX skill_revisions_status_idx ON skill_revisions(status, created_at, id);
