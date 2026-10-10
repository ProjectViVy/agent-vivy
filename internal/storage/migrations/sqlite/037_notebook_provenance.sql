-- N2 notebook provenance: automatic-ingest exclusion carried on tool
-- operations and persisted messages. No retroactive taint.
ALTER TABLE tool_operations ADD COLUMN content_origin TEXT NOT NULL DEFAULT '';
ALTER TABLE tool_operations ADD COLUMN exclude_automatic_ingest INTEGER NOT NULL DEFAULT 0;
ALTER TABLE messages ADD COLUMN content_origin TEXT NOT NULL DEFAULT '';
ALTER TABLE messages ADD COLUMN exclude_automatic_ingest INTEGER NOT NULL DEFAULT 0;
CREATE INDEX IF NOT EXISTS tool_operations_run_excluded_idx ON tool_operations(run_id, exclude_automatic_ingest);
