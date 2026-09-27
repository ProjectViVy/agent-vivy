ALTER TABLE session_truncations ADD COLUMN run_id TEXT NOT NULL DEFAULT '';
CREATE INDEX IF NOT EXISTS session_truncations_run_idx ON session_truncations(run_id);
