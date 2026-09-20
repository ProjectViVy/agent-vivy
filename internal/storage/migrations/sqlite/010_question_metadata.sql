
ALTER TABLE approvals ADD COLUMN tool_name TEXT NOT NULL DEFAULT '';
ALTER TABLE approvals ADD COLUMN created_at INTEGER NOT NULL DEFAULT 0;
ALTER TABLE approvals ADD COLUMN decided_at INTEGER NOT NULL DEFAULT 0;
ALTER TABLE approvals ADD COLUMN actor TEXT NOT NULL DEFAULT '';
ALTER TABLE approvals ADD COLUMN decision_reason BLOB NOT NULL DEFAULT '';
ALTER TABLE approvals ADD COLUMN stale_reason BLOB NOT NULL DEFAULT '';
ALTER TABLE questions ADD COLUMN created_at INTEGER NOT NULL DEFAULT 0;
ALTER TABLE questions ADD COLUMN answered_at INTEGER NOT NULL DEFAULT 0;
ALTER TABLE questions ADD COLUMN actor TEXT NOT NULL DEFAULT '';
ALTER TABLE questions ADD COLUMN decision_reason BLOB NOT NULL DEFAULT '';
CREATE INDEX approvals_status_expiry_idx ON approvals(decision, expires_at, id);
CREATE INDEX questions_status_expiry_idx ON questions(status, expires_at, id);
