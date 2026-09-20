
ALTER TABLE messages ADD COLUMN IF NOT EXISTS source TEXT NOT NULL DEFAULT '';
ALTER TABLE messages ADD COLUMN IF NOT EXISTS channel TEXT NOT NULL DEFAULT '';
ALTER TABLE messages ADD COLUMN IF NOT EXISTS chat_id TEXT NOT NULL DEFAULT '';
ALTER TABLE messages ADD COLUMN IF NOT EXISTS channel_message_id TEXT NOT NULL DEFAULT '';
CREATE TABLE IF NOT EXISTS cron_jobs (
	id TEXT PRIMARY KEY,
	name TEXT NOT NULL,
	enabled BOOLEAN NOT NULL,
	schedule_json BYTEA NOT NULL,
	payload_json BYTEA NOT NULL,
	session_id TEXT NOT NULL DEFAULT '',
	next_run_at_ms BIGINT NOT NULL DEFAULT 0,
	last_run_at_ms BIGINT NOT NULL DEFAULT 0,
	last_status TEXT NOT NULL DEFAULT '',
	last_error TEXT NOT NULL DEFAULT '',
	delete_after_run BOOLEAN NOT NULL DEFAULT FALSE,
	created_at_ms BIGINT NOT NULL,
	updated_at_ms BIGINT NOT NULL
);
CREATE INDEX IF NOT EXISTS cron_jobs_next_run_idx ON cron_jobs(enabled, next_run_at_ms);
