CREATE TABLE bundle_installations (
    install_id TEXT PRIMARY KEY,
    bundle_id TEXT NOT NULL,
    state TEXT NOT NULL,
    source_json TEXT NOT NULL,
    release_digest TEXT NOT NULL,
    revision BIGINT NOT NULL,
    tombstone BOOLEAN NOT NULL DEFAULT FALSE,
    updated_at BIGINT NOT NULL
);
CREATE UNIQUE INDEX bundle_live_bundle_id ON bundle_installations(bundle_id) WHERE tombstone = FALSE;
CREATE TABLE bundle_releases (
    install_id TEXT NOT NULL,
    release_sequence BIGINT NOT NULL,
    bundle_id TEXT NOT NULL,
    release_digest TEXT NOT NULL,
    payload_json TEXT NOT NULL,
    created_at BIGINT NOT NULL,
    PRIMARY KEY (install_id, release_sequence),
    UNIQUE (install_id, release_digest)
);
CREATE TABLE bundle_catalog_epochs (
    epoch BIGINT PRIMARY KEY,
    effective_digest TEXT NOT NULL,
    generation_id TEXT NOT NULL,
    payload_json TEXT NOT NULL,
    created_at BIGINT NOT NULL
);
CREATE TABLE bundle_catalog_state (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    epoch BIGINT NOT NULL,
    config_revision BIGINT NOT NULL,
    effective_digest TEXT NOT NULL,
    generation_id TEXT NOT NULL,
    revocation_revision BIGINT NOT NULL
);
INSERT INTO bundle_catalog_state VALUES (1, 0, 0, '', '', 0);
CREATE TABLE bundle_operations (
    operation_id TEXT PRIMARY KEY,
    input_digest TEXT NOT NULL,
    kind TEXT NOT NULL,
    state TEXT NOT NULL,
    revision BIGINT NOT NULL,
    expected_epoch BIGINT NOT NULL,
    committed_epoch BIGINT NOT NULL DEFAULT 0,
    error_code TEXT NOT NULL DEFAULT '',
    retryable BOOLEAN NOT NULL DEFAULT FALSE,
    payload_json TEXT NOT NULL,
    created_at BIGINT NOT NULL,
    updated_at BIGINT NOT NULL
);
CREATE TABLE bundle_consents (
    install_id TEXT NOT NULL,
    kind TEXT NOT NULL,
    local_id TEXT NOT NULL,
    declaration_digest TEXT NOT NULL,
    allowed BOOLEAN NOT NULL,
    revision BIGINT NOT NULL,
    subject TEXT NOT NULL,
    updated_at BIGINT NOT NULL,
    PRIMARY KEY (install_id, kind, local_id)
);
CREATE TABLE bundle_revocations (
    install_id TEXT NOT NULL,
    kind TEXT NOT NULL,
    local_id TEXT NOT NULL,
    declaration_digest TEXT NOT NULL,
    revocation_revision BIGINT NOT NULL,
    subject TEXT NOT NULL,
    created_at BIGINT NOT NULL,
    PRIMARY KEY (install_id, kind, local_id, declaration_digest)
);
CREATE TABLE run_bundle_snapshots (
    run_id TEXT PRIMARY KEY,
    schema_version INTEGER NOT NULL,
    payload BYTEA NOT NULL,
    payload_sha256 TEXT NOT NULL,
    created_at BIGINT NOT NULL
);
CREATE TABLE bundle_artifact_refs (
    owner_kind TEXT NOT NULL,
    owner_id TEXT NOT NULL,
    digest TEXT NOT NULL,
    PRIMARY KEY (owner_kind, owner_id, digest)
);
CREATE INDEX bundle_artifact_refs_digest ON bundle_artifact_refs(digest);
