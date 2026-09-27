CREATE TABLE tool_operations (
    run_id TEXT NOT NULL,
    operation_id TEXT NOT NULL,
    tool_name TEXT NOT NULL,
    request_digest TEXT NOT NULL,
    middleware_input_arguments BYTEA NOT NULL,
    arguments_digest TEXT NOT NULL,
    effective_arguments BYTEA NOT NULL,
    state TEXT NOT NULL CHECK (state IN ('admitted', 'claimed', 'completed')),
    claim_owner TEXT NOT NULL DEFAULT '',
    result TEXT NOT NULL DEFAULT '',
    failure TEXT NOT NULL DEFAULT '',
    created_at BIGINT NOT NULL,
    updated_at BIGINT NOT NULL,
    PRIMARY KEY (run_id, operation_id),
    FOREIGN KEY (run_id) REFERENCES runs(id) ON DELETE CASCADE
);
