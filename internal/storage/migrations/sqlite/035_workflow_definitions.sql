-- S11-F: reusable INOFY workflow definitions (G9).
-- One mutable draft per workflow under ETag CAS; immutable monotone
-- published revisions deduplicated by (workflow, artifact+catalog digest).
-- Drafts are bound to their author session; published revisions are
-- organism-visible. Paired with the PostgreSQL migration of the same
-- number.
CREATE TABLE workflow_definition_drafts (
    workflow_id TEXT PRIMARY KEY,
    etag TEXT NOT NULL,
    artifact BLOB NOT NULL,
    definition_digest TEXT NOT NULL,
    artifact_digest TEXT NOT NULL,
    archived INTEGER NOT NULL DEFAULT 0,
    author_session_id TEXT NOT NULL,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    FOREIGN KEY (author_session_id) REFERENCES sessions(id) ON DELETE CASCADE
);

CREATE TABLE workflow_definition_revisions (
    workflow_id TEXT NOT NULL,
    revision INTEGER NOT NULL,
    artifact BLOB NOT NULL,
    definition_digest TEXT NOT NULL,
    artifact_digest TEXT NOT NULL,
    used_catalog_digest TEXT NOT NULL,
    used_implementations BLOB NOT NULL,
    author_session_id TEXT NOT NULL,
    published_at INTEGER NOT NULL,
    PRIMARY KEY (workflow_id, revision)
);
CREATE INDEX workflow_definition_revisions_dedup_idx
    ON workflow_definition_revisions(workflow_id, artifact_digest, used_catalog_digest);

-- Published/draft lineage bound onto the admitted workflow Run so a
-- product Run carries its immutable definition identity (revision 0 =
-- started from a draft snapshot).
ALTER TABLE workflow_revisions ADD COLUMN definition_id TEXT;
ALTER TABLE workflow_revisions ADD COLUMN definition_revision INTEGER;
-- Canonical run input (the JSON object admitted alongside the definition). The
-- restart-recovery path re-executes the committed program and must reproduce
-- the exact input that input_digest covers; it cannot come from the caller.
ALTER TABLE workflow_revisions ADD COLUMN input_json BLOB;
CREATE INDEX workflow_revisions_definition_idx ON workflow_revisions(definition_id);
