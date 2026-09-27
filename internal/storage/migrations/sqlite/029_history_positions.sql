ALTER TABLE sessions ADD COLUMN next_message_position INTEGER NOT NULL DEFAULT 1;
ALTER TABLE messages ADD COLUMN position INTEGER NOT NULL DEFAULT 0;

WITH ordered AS (
  SELECT id, ROW_NUMBER() OVER (PARTITION BY session_id ORDER BY created_at, id) AS position
  FROM messages
)
UPDATE messages
SET position = (SELECT ordered.position FROM ordered WHERE ordered.id = messages.id);

UPDATE sessions
SET next_message_position = COALESCE((SELECT MAX(position) + 1 FROM messages WHERE messages.session_id = sessions.id), 1);

CREATE UNIQUE INDEX messages_session_position_idx ON messages(session_id, position);
