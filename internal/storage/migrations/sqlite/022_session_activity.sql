
	ALTER TABLE sessions ADD COLUMN updated_at INTEGER NOT NULL DEFAULT 0;
	UPDATE sessions SET updated_at = created_at WHERE updated_at = 0;
