-- Metadata only. Original bodies remain core messages/Journal/artifacts.
CREATE TABLE subject_sources (
 session_id TEXT PRIMARY KEY REFERENCES sessions(id),
 subject_id TEXT NOT NULL,
 domain_id TEXT NOT NULL,
 conversation_id TEXT NOT NULL,
 binding_json BLOB NOT NULL,
 archived INTEGER NOT NULL DEFAULT 0 CHECK (archived IN (0,1))
);
CREATE INDEX subject_sources_domain ON subject_sources(subject_id,domain_id);
CREATE TABLE subject_main_conversations (
 subject_id TEXT PRIMARY KEY,
 domain_id TEXT NOT NULL,
 conversation_id TEXT NOT NULL UNIQUE,
 policy_binding_json BLOB NOT NULL
);
CREATE TABLE run_subject_sources (
 run_id TEXT PRIMARY KEY REFERENCES runs(id),
 admitted_message_id TEXT NOT NULL REFERENCES messages(id),
 session_id TEXT NOT NULL REFERENCES subject_sources(session_id),
 binding_json BLOB NOT NULL
);
CREATE TABLE subject_history_heads (
 subject_id TEXT NOT NULL,
 domain_id TEXT NOT NULL,
 next_sequence BIGINT NOT NULL CHECK (next_sequence > 0),
 PRIMARY KEY(subject_id,domain_id)
);
CREATE TABLE subject_interactions (
 subject_id TEXT NOT NULL,
 domain_id TEXT NOT NULL,
 sequence BIGINT NOT NULL CHECK (sequence > 0),
 interaction_id TEXT NOT NULL UNIQUE,
 session_id TEXT NOT NULL REFERENCES subject_sources(session_id),
 run_id TEXT NOT NULL UNIQUE REFERENCES run_subject_sources(run_id),
 terminal_seq BIGINT NOT NULL CHECK (terminal_seq > 0),
 source_digest TEXT NOT NULL,
 terminal_type TEXT NOT NULL,
 PRIMARY KEY(subject_id,domain_id,sequence)
);
