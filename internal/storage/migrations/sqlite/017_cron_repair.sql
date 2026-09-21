
	CREATE TABLE IF NOT EXISTS cron_jobs (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		enabled INTEGER NOT NULL,
		schedule_json BLOB NOT NULL,
		payload_json BLOB NOT NULL,
		session_id TEXT NOT NULL DEFAULT '',
		next_run_at_ms INTEGER NOT NULL DEFAULT 0,
		last_run_at_ms INTEGER NOT NULL DEFAULT 0,
		last_status TEXT NOT NULL DEFAULT '',
		last_error TEXT NOT NULL DEFAULT '',
		delete_after_run INTEGER NOT NULL DEFAULT 0,
		created_at_ms INTEGER NOT NULL,
		updated_at_ms INTEGER NOT NULL
	);
	CREATE INDEX IF NOT EXISTS cron_jobs_next_run_idx ON cron_jobs(enabled, next_run_at_ms);
