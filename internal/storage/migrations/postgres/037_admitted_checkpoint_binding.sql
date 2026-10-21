-- Snapshot identity is owned by the Run row; native checkpoint bytes are opaque.
ALTER TABLE run_prompt_snapshots DROP COLUMN payload_sha256;
ALTER TABLE checkpoints DROP COLUMN checksum;
