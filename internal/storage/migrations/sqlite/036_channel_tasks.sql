-- A2A-02: scoped channel task ownership and admission receipts.
-- Authorization/idempotency index only — no task status, outputs, or
-- execution live here. Tombstones survive Session/Run deletion on purpose:
-- no cascade-delete foreign keys. Paired with the PostgreSQL migration of
-- the same number.
CREATE TABLE channel_task_scopes (
    instance_key TEXT NOT NULL,
    principal_id TEXT NOT NULL,
    created_at INTEGER NOT NULL,
    PRIMARY KEY (instance_key, principal_id)
);

CREATE TABLE channel_task_contexts (
    session_id TEXT PRIMARY KEY,
    instance_key TEXT NOT NULL,
    principal_id TEXT NOT NULL,
    created_at INTEGER NOT NULL,
    deleted_at INTEGER
);
CREATE INDEX idx_channel_task_contexts_scope
    ON channel_task_contexts (instance_key, principal_id, session_id);

CREATE TABLE channel_task_receipts (
    instance_key TEXT NOT NULL,
    principal_id TEXT NOT NULL,
    message_id TEXT NOT NULL,
    input_hash BLOB NOT NULL,
    operation TEXT NOT NULL,
    session_id TEXT NOT NULL,
    run_id TEXT NOT NULL,
    question_id TEXT,
    accepted_seq INTEGER NOT NULL,
    created_at INTEGER NOT NULL,
    deleted_at INTEGER,
    PRIMARY KEY (instance_key, principal_id, message_id)
);
CREATE UNIQUE INDEX idx_channel_task_receipts_submit_run
    ON channel_task_receipts (run_id)
    WHERE operation = 'submit' AND deleted_at IS NULL;
CREATE INDEX idx_channel_task_receipts_scope_run
    ON channel_task_receipts (instance_key, principal_id, operation, run_id);
