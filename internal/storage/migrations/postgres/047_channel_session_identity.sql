-- The original platform tuple belongs to its existing session row. NULL is
-- an ordinary non-channel session; the finite converter binds old channel
-- sessions from proven source tuples without renaming any historical ID.
ALTER TABLE sessions ADD COLUMN channel_name TEXT;
ALTER TABLE sessions ADD COLUMN channel_chat_id TEXT;
ALTER TABLE sessions ADD COLUMN channel_topic_id TEXT CHECK (
  (channel_name IS NULL AND channel_chat_id IS NULL AND channel_topic_id IS NULL)
  OR (channel_name IS NOT NULL AND channel_name <> ''
      AND channel_chat_id IS NOT NULL AND channel_chat_id <> '' AND channel_topic_id IS NOT NULL)
);
CREATE UNIQUE INDEX sessions_channel_identity ON sessions(channel_name, channel_chat_id, channel_topic_id)
WHERE channel_name IS NOT NULL;
