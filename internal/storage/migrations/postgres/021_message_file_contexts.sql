
CREATE TABLE IF NOT EXISTS message_file_contexts (
	id BIGSERIAL PRIMARY KEY,
	message_id TEXT NOT NULL,
	position INTEGER NOT NULL,
	path TEXT NOT NULL,
	name TEXT NOT NULL DEFAULT '',
	size BIGINT NOT NULL,
	content BYTEA NOT NULL
);
CREATE INDEX IF NOT EXISTS message_file_contexts_message_idx ON message_file_contexts(message_id);
