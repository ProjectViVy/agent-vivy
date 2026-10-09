-- R0: trusted root report workflow admission. Paired with the SQLite
-- table rebuild: purpose columns plus the admission_namespace collision
-- domain replacing the parent_run_id unique pair.

ALTER TABLE sessions ADD COLUMN purpose TEXT NOT NULL DEFAULT '';
ALTER TABLE runs ADD COLUMN purpose TEXT NOT NULL DEFAULT '';

ALTER TABLE workflow_revisions ALTER COLUMN parent_run_id SET DEFAULT '';
ALTER TABLE workflow_revisions ADD COLUMN root_purpose TEXT NOT NULL DEFAULT '';
ALTER TABLE workflow_revisions ADD COLUMN admission_namespace TEXT NOT NULL DEFAULT '';
ALTER TABLE workflow_revisions ADD COLUMN request_digest TEXT NOT NULL DEFAULT '';
ALTER TABLE workflow_revisions ADD COLUMN target_key TEXT NOT NULL DEFAULT '';

UPDATE workflow_revisions
SET admission_namespace = parent_run_id,
    request_digest = descriptor_digest;

ALTER TABLE workflow_revisions
    DROP CONSTRAINT workflow_revisions_parent_run_id_fkey,
    DROP CONSTRAINT workflow_revisions_parent_run_id_operation_key_key,
    ADD CONSTRAINT workflow_revisions_namespace_operation_key_key
        UNIQUE (admission_namespace, operation_key);

CREATE INDEX workflow_revisions_namespace_idx
    ON workflow_revisions(admission_namespace, created_at, workflow_run_id);
CREATE INDEX workflow_revisions_target_idx
    ON workflow_revisions(admission_namespace, target_key);
