CREATE TABLE bundle_bindings (
    install_id TEXT NOT NULL,
    kind TEXT NOT NULL,
    local_id TEXT NOT NULL,
    name TEXT NOT NULL,
    reference TEXT NOT NULL,
    PRIMARY KEY (install_id, kind, local_id, name)
);
CREATE INDEX bundle_bindings_component ON bundle_bindings(install_id, kind, local_id);
