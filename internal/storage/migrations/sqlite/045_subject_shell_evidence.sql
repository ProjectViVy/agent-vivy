-- Shell admissions have real tool evidence and no synthetic user message.
-- Rebuild both FK-linked metadata tables without changing original identities.
CREATE TABLE run_subject_sources_next (
 run_id TEXT PRIMARY KEY REFERENCES runs(id),
 admitted_message_id TEXT REFERENCES messages(id),
 session_id TEXT NOT NULL REFERENCES subject_sources(session_id),
 binding_json BLOB NOT NULL
);
INSERT INTO run_subject_sources_next SELECT * FROM run_subject_sources;
CREATE TABLE subject_interactions_next (
 subject_id TEXT NOT NULL,
 domain_id TEXT NOT NULL,
 sequence BIGINT NOT NULL CHECK (sequence > 0),
 interaction_id TEXT NOT NULL UNIQUE,
 session_id TEXT NOT NULL REFERENCES subject_sources(session_id),
 run_id TEXT NOT NULL UNIQUE REFERENCES run_subject_sources_next(run_id),
 terminal_seq BIGINT NOT NULL CHECK (terminal_seq > 0),
 source_digest TEXT NOT NULL,
 terminal_type TEXT NOT NULL,
 PRIMARY KEY(subject_id,domain_id,sequence)
);
INSERT INTO subject_interactions_next SELECT * FROM subject_interactions;
DROP TABLE subject_interactions;
DROP TABLE run_subject_sources;
ALTER TABLE run_subject_sources_next RENAME TO run_subject_sources;
ALTER TABLE subject_interactions_next RENAME TO subject_interactions;
-- Copy provenance is metadata only. Parent coordinates are intentionally not
-- foreign keys: deleting an unprotected ordinary parent remains supported.
CREATE TABLE message_copy_sources (
 message_id TEXT PRIMARY KEY REFERENCES messages(id) ON DELETE CASCADE,
 parent_session_id TEXT NOT NULL,
 parent_message_id TEXT NOT NULL
);
