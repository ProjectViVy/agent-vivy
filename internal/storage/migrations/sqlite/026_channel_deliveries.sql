-- Migration 026 adds the durable outbound channel-delivery intents
-- (CH-C3-N1). Rows are Host-internal operational state keyed by the run
-- they must answer; success deletes the row, so the table only ever holds
-- undelivered intent and needs no retention of its own. No foreign keys —
-- chanin-adjacent operational state must not couple to the runs table.
CREATE TABLE IF NOT EXISTS channel_deliveries (
	run_id TEXT PRIMARY KEY,
	session_id TEXT NOT NULL,
	channel TEXT NOT NULL,
	chat_id TEXT NOT NULL,
	topic_id TEXT NOT NULL DEFAULT '',
	state TEXT NOT NULL,
	attempts INTEGER NOT NULL DEFAULT 0,
	created_at_ms INTEGER NOT NULL,
	updated_at_ms INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS channel_deliveries_state_idx ON channel_deliveries(state, created_at_ms);
