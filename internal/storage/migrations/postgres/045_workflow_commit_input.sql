-- D11: actual canonical commit input stays with the existing receipt.
-- Old NULL inputs are unbound history and cannot authorize replay.
ALTER TABLE workflow_commits DROP COLUMN digest;
ALTER TABLE workflow_commits ADD COLUMN input_json BYTEA;
