ALTER TABLE runs
    ADD COLUMN child_mode TEXT NOT NULL DEFAULT ''
    CHECK (child_mode IN ('', 'one-shot', 'continuable'));

UPDATE runs SET child_mode = 'one-shot' WHERE kind = 'child';

CREATE TABLE child_sessions (
    child_session_id TEXT PRIMARY KEY,
    origin_parent_session_id TEXT NOT NULL,
    origin_parent_run_id TEXT NOT NULL,
    authorizer_run_id TEXT NOT NULL,
    initial_activation_run_id TEXT NOT NULL,
    activation_run_id TEXT NOT NULL,
    operation_key TEXT NOT NULL,
    request_digest TEXT NOT NULL,
    authority_ceiling_digest TEXT NOT NULL,
    authority_ceiling_json TEXT NOT NULL,
    activation_operation_key TEXT NOT NULL,
    activation_request_digest TEXT NOT NULL,
    activation_tools_json TEXT NOT NULL,
    state TEXT NOT NULL CHECK (state IN ('open', 'closed')),
    next_message_sequence INTEGER NOT NULL DEFAULT 1 CHECK (next_message_sequence >= 1),
    consumed_message_sequence INTEGER NOT NULL DEFAULT 0 CHECK (consumed_message_sequence >= 0),
    next_parent_message_sequence INTEGER NOT NULL DEFAULT 1 CHECK (next_parent_message_sequence >= 1),
    consumed_parent_message_sequence INTEGER NOT NULL DEFAULT 0 CHECK (consumed_parent_message_sequence >= 0),
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    UNIQUE (origin_parent_session_id, operation_key),
    FOREIGN KEY (child_session_id) REFERENCES sessions(id) ON DELETE CASCADE,
    FOREIGN KEY (origin_parent_session_id) REFERENCES sessions(id) ON DELETE CASCADE
);
CREATE INDEX child_sessions_parent_idx ON child_sessions(origin_parent_session_id, created_at, child_session_id);

CREATE TABLE child_session_activations (
    child_session_id TEXT NOT NULL,
    activation_run_id TEXT NOT NULL UNIQUE,
    authorizer_run_id TEXT NOT NULL,
    operation_key TEXT NOT NULL,
    request_digest TEXT NOT NULL,
    tool_names_json TEXT NOT NULL,
    created_at INTEGER NOT NULL,
    PRIMARY KEY (child_session_id, operation_key),
    FOREIGN KEY (child_session_id) REFERENCES child_sessions(child_session_id) ON DELETE CASCADE,
    FOREIGN KEY (activation_run_id) REFERENCES runs(id) ON DELETE CASCADE
);

CREATE TABLE child_mailbox_messages (
    message_id TEXT PRIMARY KEY,
    child_session_id TEXT NOT NULL,
    sender_session_id TEXT NOT NULL,
    recipient_session_id TEXT NOT NULL,
    idempotency_key TEXT NOT NULL,
    sequence INTEGER NOT NULL CHECK (sequence >= 1),
    body BLOB NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('pending', 'consumed', 'rejected', 'expired')),
    created_at INTEGER NOT NULL,
    consumed_at INTEGER NOT NULL DEFAULT 0,
    consumed_by_run_id TEXT NOT NULL DEFAULT '',
    UNIQUE (child_session_id, message_id),
    UNIQUE (child_session_id, recipient_session_id, sequence),
    UNIQUE (child_session_id, sender_session_id, idempotency_key),
    FOREIGN KEY (child_session_id) REFERENCES child_sessions(child_session_id) ON DELETE CASCADE,
    FOREIGN KEY (sender_session_id) REFERENCES sessions(id) ON DELETE CASCADE,
    FOREIGN KEY (recipient_session_id) REFERENCES sessions(id) ON DELETE CASCADE
);
CREATE INDEX child_mailbox_pending_idx ON child_mailbox_messages(child_session_id, recipient_session_id, status, sequence);

CREATE TABLE child_message_receipts (
    child_session_id TEXT NOT NULL,
    message_id TEXT NOT NULL,
    consumer_run_id TEXT NOT NULL,
    state TEXT NOT NULL CHECK (state IN ('in-progress', 'consumed', 'failed')),
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    PRIMARY KEY (message_id, consumer_run_id),
    FOREIGN KEY (child_session_id, message_id)
        REFERENCES child_mailbox_messages(child_session_id, message_id) ON DELETE CASCADE
);
