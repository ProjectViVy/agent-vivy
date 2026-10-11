CREATE TABLE mask_bundle_metadata (
    mask_id TEXT PRIMARY KEY,
    install_id TEXT NOT NULL,
    bundle_id TEXT NOT NULL,
    local_id TEXT NOT NULL,
    release_digest TEXT NOT NULL,
    release_sequence INTEGER NOT NULL CHECK (release_sequence > 0),
    recommended_skills_json BLOB NOT NULL,
    FOREIGN KEY (mask_id) REFERENCES mask_definitions(id) ON DELETE CASCADE
);
