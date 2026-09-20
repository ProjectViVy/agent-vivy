
ALTER TABLE sessions ADD COLUMN IF NOT EXISTS updated_at BIGINT NOT NULL DEFAULT 0;
UPDATE sessions SET updated_at = created_at WHERE updated_at = 0;
