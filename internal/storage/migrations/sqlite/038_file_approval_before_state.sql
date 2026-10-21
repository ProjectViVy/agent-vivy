-- Exact file before-state is private v1 provider data within D03 approval v2.
-- Historical events remain immutable; unavailable old before-state cannot run.
ALTER TABLE approvals DROP COLUMN precondition_hash;

-- Historical read timestamps cannot reconstruct reviewed bytes. NULL requires a new read.
ALTER TABLE file_versions DROP COLUMN content_hash;
ALTER TABLE file_reads ADD COLUMN read_exists INTEGER;
ALTER TABLE file_reads ADD COLUMN read_content BLOB;
