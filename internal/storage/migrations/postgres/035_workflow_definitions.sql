-- S11-F: reusable INOFY workflow definitions (G9). Paired with the SQLite
-- migration of the same number.
CREATE TABLE workflow_definition_drafts (
    workflow_id TEXT PRIMARY KEY,
    etag TEXT NOT NULL,
    artifact BYTEA NOT NULL,
    definition_digest TEXT NOT NULL,
    artifact_digest TEXT NOT NULL,
    archived INTEGER NOT NULL DEFAULT 0,
    author_session_id TEXT NOT NULL,
    created_at BIGINT NOT NULL,
    updated_at BIGINT NOT NULL,
    FOREIGN KEY (author_session_id) REFERENCES sessions(id) ON DELETE CASCADE
);

CREATE TABLE workflow_definition_revisions (
    workflow_id TEXT NOT NULL,
    revision BIGINT NOT NULL,
    artifact BYTEA NOT NULL,
    definition_digest TEXT NOT NULL,
    artifact_digest TEXT NOT NULL,
    used_catalog_digest TEXT NOT NULL,
    used_implementations BYTEA NOT NULL,
    author_session_id TEXT NOT NULL,
    published_at BIGINT NOT NULL,
    PRIMARY KEY (workflow_id, revision)
);
CREATE INDEX workflow_definition_revisions_dedup_idx
    ON workflow_definition_revisions(workflow_id, artifact_digest, used_catalog_digest);

ALTER TABLE workflow_revisions ADD COLUMN definition_id TEXT;
ALTER TABLE workflow_revisions ADD COLUMN definition_revision BIGINT;
-- Canonical run input (the JSON object admitted alongside the definition). The
-- restart-recovery path re-executes the committed program and must reproduce
-- the exact input that input_digest covers; it cannot come from the caller.
ALTER TABLE workflow_revisions ADD COLUMN input_json BYTEA;
CREATE INDEX workflow_revisions_definition_idx ON workflow_revisions(definition_id);
