-- R0: trusted root report workflow admission.
-- Purpose columns distinguish the hidden report control session and report
-- workflow runs from the interactive lane. workflow_revisions is rebuilt so
-- the operation-key collision domain moves from parent_run_id to
-- admission_namespace: child workflows keep their parent run id, report roots
-- carry the control session id and may leave parent_run_id empty.

ALTER TABLE sessions ADD COLUMN purpose TEXT NOT NULL DEFAULT '';
ALTER TABLE runs ADD COLUMN purpose TEXT NOT NULL DEFAULT '';

CREATE TABLE workflow_revisions_next (
    workflow_run_id TEXT PRIMARY KEY,
    parent_run_id TEXT NOT NULL DEFAULT '',
    parent_session_id TEXT NOT NULL,
    root_run_id TEXT NOT NULL,
    operation_key TEXT NOT NULL,
    root_purpose TEXT NOT NULL DEFAULT '',
    admission_namespace TEXT NOT NULL,
    request_digest TEXT NOT NULL DEFAULT '',
    target_key TEXT NOT NULL DEFAULT '',
    descriptor_digest TEXT NOT NULL CHECK (length(descriptor_digest) = 64),
    authority_digest TEXT NOT NULL CHECK (length(authority_digest) = 64),
    descriptor_json BLOB NOT NULL,
    authority_json BLOB NOT NULL,
    schema_version INTEGER NOT NULL CHECK (schema_version > 0),
    created_at INTEGER NOT NULL,
    program_digest TEXT,
    catalog_digest TEXT,
    compiler_version TEXT,
    eino_build TEXT,
    input_digest TEXT,
    effective_limits BLOB,
    host_binding_id TEXT,
    definition_id TEXT,
    definition_revision INTEGER,
    input_json BLOB,
    UNIQUE (admission_namespace, operation_key),
    FOREIGN KEY (workflow_run_id) REFERENCES runs(id) ON DELETE CASCADE,
    FOREIGN KEY (parent_session_id) REFERENCES sessions(id) ON DELETE CASCADE
);

INSERT INTO workflow_revisions_next (
    workflow_run_id, parent_run_id, parent_session_id, root_run_id,
    operation_key, root_purpose, admission_namespace, request_digest,
    target_key, descriptor_digest, authority_digest, descriptor_json,
    authority_json, schema_version, created_at, program_digest,
    catalog_digest, compiler_version, eino_build, input_digest,
    effective_limits, host_binding_id, definition_id,
    definition_revision, input_json
)
SELECT
    workflow_run_id, parent_run_id, parent_session_id, root_run_id,
    operation_key, '', parent_run_id, descriptor_digest,
    '', descriptor_digest, authority_digest, descriptor_json,
    authority_json, schema_version, created_at, program_digest,
    catalog_digest, compiler_version, eino_build, input_digest,
    effective_limits, host_binding_id, definition_id,
    definition_revision, input_json
FROM workflow_revisions;

DROP TABLE workflow_revisions;
ALTER TABLE workflow_revisions_next RENAME TO workflow_revisions;

CREATE INDEX workflow_revisions_parent_idx ON workflow_revisions(parent_run_id, created_at, workflow_run_id);
CREATE INDEX workflow_revisions_namespace_idx ON workflow_revisions(admission_namespace, created_at, workflow_run_id);
CREATE INDEX workflow_revisions_target_idx ON workflow_revisions(admission_namespace, target_key);
