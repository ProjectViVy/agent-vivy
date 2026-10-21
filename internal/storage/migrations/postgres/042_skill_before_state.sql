-- Preserve reviewed actual file inputs; directory and ambiguous historical records
-- remain unbound and require a fresh review. Never infer bytes from old hashes.
ALTER TABLE skill_revisions ADD COLUMN before_exists BOOLEAN;
UPDATE skill_revisions SET before_exists = FALSE WHERE action = 'create';
UPDATE skill_revisions SET before_exists = TRUE
 WHERE action IN ('edit', 'patch', 'write_file', 'remove_file') AND length(before_payload) > 0;
ALTER TABLE skill_revisions DROP COLUMN base_hash;
ALTER TABLE skill_revisions DROP COLUMN content_hash;
