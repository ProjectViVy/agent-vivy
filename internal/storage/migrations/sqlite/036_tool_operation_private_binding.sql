-- Converted baseline rows deliberately remain unbound; never infer original inputs.
ALTER TABLE tool_operations DROP COLUMN request_digest;
ALTER TABLE tool_operations DROP COLUMN arguments_digest;
ALTER TABLE tool_operations ADD COLUMN request_arguments BLOB;
ALTER TABLE tool_operations ADD COLUMN outcome_status TEXT;
ALTER TABLE tool_operations ADD COLUMN outcome_reason TEXT;
ALTER TABLE tool_operations ADD COLUMN outcome_effects TEXT;
