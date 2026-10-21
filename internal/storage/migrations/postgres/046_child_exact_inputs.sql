-- Existing hashes cannot establish exact original input. Preserve history and
-- mailbox/review state; NULL input is unavailable for execution or retry reuse.
ALTER TABLE child_sessions DROP COLUMN request_digest;
ALTER TABLE child_sessions DROP COLUMN authority_ceiling_digest;
ALTER TABLE child_sessions DROP COLUMN activation_request_digest;
ALTER TABLE child_sessions ADD COLUMN request_input TEXT;
ALTER TABLE child_sessions ADD COLUMN activation_request_input TEXT;
ALTER TABLE child_session_activations DROP COLUMN request_digest;
ALTER TABLE child_session_activations ADD COLUMN request_input TEXT;
ALTER TABLE session_work_events DROP COLUMN request_hash;
ALTER TABLE session_work_events ADD COLUMN request_input TEXT;
