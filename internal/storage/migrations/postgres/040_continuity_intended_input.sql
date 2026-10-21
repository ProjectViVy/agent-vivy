-- Retain receipt/outcome identity; historical exact inputs cannot be recovered.
ALTER TABLE continuity_receipts DROP COLUMN input_hash;
ALTER TABLE continuity_receipts ADD COLUMN intended_input BYTEA;
