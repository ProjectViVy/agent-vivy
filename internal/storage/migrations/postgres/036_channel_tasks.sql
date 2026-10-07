-- A2A-02: scoped channel task ownership and admission receipts. Paired with
-- the SQLite migration of the same number; same constraints, no cascades.
CREATE TABLE channel_task_scopes (
    instance_key TEXT NOT NULL,
    principal_id TEXT NOT NULL,
    created_at BIGINT NOT NULL,
    PRIMARY KEY (instance_key, principal_id)
);

CREATE TABLE channel_task_contexts (
    session_id TEXT PRIMARY KEY,
    instance_key TEXT NOT NULL,
    principal_id TEXT NOT NULL,
    created_at BIGINT NOT NULL,
    deleted_at BIGINT
);
CREATE INDEX idx_channel_task_contexts_scope
    ON channel_task_contexts (instance_key, principal_id, session_id);

CREATE TABLE channel_task_receipts (
    instance_key TEXT NOT NULL,
    principal_id TEXT NOT NULL,
    message_id TEXT NOT NULL,
    input_hash BYTEA NOT NULL,
    operation TEXT NOT NULL,
    session_id TEXT NOT NULL,
    run_id TEXT NOT NULL,
    question_id TEXT,
    accepted_seq BIGINT NOT NULL,
    created_at BIGINT NOT NULL,
    deleted_at BIGINT,
    PRIMARY KEY (instance_key, principal_id, message_id)
);
CREATE UNIQUE INDEX idx_channel_task_receipts_submit_run
    ON channel_task_receipts (run_id)
    WHERE operation = 'submit' AND deleted_at IS NULL;
CREATE INDEX idx_channel_task_receipts_scope_run
    ON channel_task_receipts (instance_key, principal_id, operation, run_id);
