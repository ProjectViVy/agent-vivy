
	CREATE TABLE IF NOT EXISTS message_attachments (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		message_id TEXT NOT NULL,
		position INTEGER NOT NULL,
		name TEXT NOT NULL DEFAULT '',
		mime_type TEXT NOT NULL DEFAULT '',
		data BLOB NOT NULL
	);
	CREATE INDEX IF NOT EXISTS message_attachments_message_idx ON message_attachments(message_id);
