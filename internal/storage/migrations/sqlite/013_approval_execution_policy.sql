
ALTER TABLE approvals ADD COLUMN sandbox_mode TEXT NOT NULL DEFAULT 'workspace_write';
ALTER TABLE approvals ADD COLUMN approval_policy TEXT NOT NULL DEFAULT 'ask';
ALTER TABLE approvals ADD COLUMN timeout_at INTEGER NOT NULL DEFAULT 0;
ALTER TABLE sessions ADD COLUMN sandbox_mode TEXT NOT NULL DEFAULT 'workspace_write';
ALTER TABLE sessions ADD COLUMN approval_policy TEXT NOT NULL DEFAULT 'ask';
