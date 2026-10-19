-- Shell admissions have real tool evidence and no synthetic user message.
ALTER TABLE run_subject_sources ALTER COLUMN admitted_message_id DROP NOT NULL;
-- Copy provenance is metadata only. Parent coordinates are intentionally not
-- foreign keys: deleting an unprotected ordinary parent remains supported.
CREATE TABLE message_copy_sources (
 message_id TEXT PRIMARY KEY REFERENCES messages(id) ON DELETE CASCADE,
 parent_session_id TEXT NOT NULL,
 parent_message_id TEXT NOT NULL
);
