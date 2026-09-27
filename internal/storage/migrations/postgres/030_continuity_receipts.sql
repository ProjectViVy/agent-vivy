CREATE TABLE continuity_receipts (
  session_id TEXT NOT NULL,
  operation TEXT NOT NULL,
  request_id TEXT NOT NULL,
  input_hash TEXT NOT NULL,
  run_id TEXT NOT NULL,
  event_seq BIGINT NOT NULL,
  PRIMARY KEY (session_id, operation, request_id)
);

CREATE INDEX continuity_receipts_run_idx ON continuity_receipts(run_id);
