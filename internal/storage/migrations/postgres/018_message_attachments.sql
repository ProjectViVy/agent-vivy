
CREATE TABLE IF NOT EXISTS message_attachments (
	id BIGSERIAL PRIMARY KEY,
	message_id TEXT NOT NULL,
	position INTEGER NOT NULL,
	name TEXT NOT NULL DEFAULT '',
	mime_type TEXT NOT NULL DEFAULT '',
	data BYTEA NOT NULL
);
CREATE INDEX IF NOT EXISTS message_attachments_message_idx ON message_attachments(message_id);
