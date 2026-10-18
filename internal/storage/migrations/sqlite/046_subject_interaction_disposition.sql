-- A terminal run whose tool effects can never be resolved (cancelled or failed
-- mid-tool, unretainable result, invalid argument bytes) is sealed as an
-- explicit 'excluded' interaction: no originals enter Recap, the sequence
-- advances, and the tool operation keeps its unresolved state as evidence.
ALTER TABLE subject_interactions ADD COLUMN disposition TEXT NOT NULL DEFAULT 'included' CHECK (disposition IN ('included','excluded'));
-- Pending-run and terminal-evidence cuts join on session_id.
CREATE INDEX run_subject_sources_session ON run_subject_sources(session_id);
