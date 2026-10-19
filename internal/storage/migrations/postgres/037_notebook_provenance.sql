-- N2 notebook provenance: automatic-ingest exclusion carried on tool
-- operations and persisted messages. No retroactive taint.
ALTER TABLE tool_operations ADD COLUMN IF NOT EXISTS content_origin TEXT NOT NULL DEFAULT '';
ALTER TABLE tool_operations ADD COLUMN IF NOT EXISTS exclude_automatic_ingest BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE messages ADD COLUMN IF NOT EXISTS content_origin TEXT NOT NULL DEFAULT '';
ALTER TABLE messages ADD COLUMN IF NOT EXISTS exclude_automatic_ingest BOOLEAN NOT NULL DEFAULT FALSE;
CREATE INDEX IF NOT EXISTS tool_operations_run_excluded_idx ON tool_operations(run_id) WHERE exclude_automatic_ingest;
