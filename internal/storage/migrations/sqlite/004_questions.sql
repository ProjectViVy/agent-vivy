
CREATE TABLE questions (
	id TEXT PRIMARY KEY,
	run_id TEXT NOT NULL,
	tool_call_id TEXT NOT NULL,
	prompt BLOB NOT NULL,
	answer BLOB NOT NULL DEFAULT '',
	status TEXT NOT NULL,
	expires_at INTEGER NOT NULL,
	resume_target TEXT NOT NULL DEFAULT '',
	FOREIGN KEY(run_id) REFERENCES runs(id)
);
