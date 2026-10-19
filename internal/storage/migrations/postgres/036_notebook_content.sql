-- N1: scoped, revisioned notebook content (NB-02..NB-07). Paired with the
-- SQLite migration of the same number; see that file for the design notes.

CREATE TABLE notebook_sections (
    scope TEXT NOT NULL,
    id TEXT NOT NULL,
    title TEXT NOT NULL,
    system_role TEXT NOT NULL DEFAULT '',
    version BIGINT NOT NULL,
    created_at BIGINT NOT NULL,
    updated_at BIGINT NOT NULL,
    deleted_at BIGINT NOT NULL DEFAULT 0,
    PRIMARY KEY (scope, id)
);
CREATE UNIQUE INDEX notebook_sections_role_uq
    ON notebook_sections(scope, system_role)
    WHERE system_role <> '' AND deleted_at = 0;

CREATE TABLE notebook_entries (
    scope TEXT NOT NULL,
    id TEXT NOT NULL,
    section_id TEXT NOT NULL,
    kind TEXT NOT NULL,
    title TEXT NOT NULL,
    head_revision_id TEXT NOT NULL DEFAULT '',
    version BIGINT NOT NULL,
    report_series_id TEXT NOT NULL DEFAULT '',
    report_window_id TEXT NOT NULL DEFAULT '',
    created_at BIGINT NOT NULL,
    updated_at BIGINT NOT NULL,
    deleted_at BIGINT NOT NULL DEFAULT 0,
    PRIMARY KEY (scope, id),
    FOREIGN KEY (scope, section_id) REFERENCES notebook_sections(scope, id)
);
CREATE INDEX notebook_entries_section_idx
    ON notebook_entries(scope, section_id, updated_at, id);
CREATE INDEX notebook_entries_series_idx
    ON notebook_entries(scope, report_series_id, report_window_id)
    WHERE report_series_id <> '';

CREATE TABLE notebook_revisions (
    scope TEXT NOT NULL,
    entry_id TEXT NOT NULL,
    revision_id TEXT NOT NULL,
    sequence BIGINT NOT NULL,
    parent_revision_id TEXT NOT NULL DEFAULT '',
    title TEXT NOT NULL,
    markdown BYTEA NOT NULL,
    origin TEXT NOT NULL,
    base_revision_id TEXT NOT NULL DEFAULT '',
    actor TEXT NOT NULL,
    created_at BIGINT NOT NULL,
    PRIMARY KEY (scope, revision_id),
    UNIQUE (scope, entry_id, sequence),
    FOREIGN KEY (scope, entry_id) REFERENCES notebook_entries(scope, id)
);
CREATE INDEX notebook_revisions_entry_idx
    ON notebook_revisions(scope, entry_id, sequence);

CREATE TABLE notebook_comments (
    scope TEXT NOT NULL,
    id TEXT NOT NULL,
    entry_id TEXT NOT NULL,
    anchor_revision_id TEXT,
    body BYTEA NOT NULL,
    version BIGINT NOT NULL,
    author TEXT NOT NULL,
    status TEXT NOT NULL,
    created_at BIGINT NOT NULL,
    updated_at BIGINT NOT NULL,
    PRIMARY KEY (scope, id),
    FOREIGN KEY (scope, entry_id) REFERENCES notebook_entries(scope, id),
    FOREIGN KEY (scope, anchor_revision_id) REFERENCES notebook_revisions(scope, revision_id)
);
CREATE INDEX notebook_comments_entry_idx
    ON notebook_comments(scope, entry_id, created_at, id);

CREATE TABLE notebook_comment_versions (
    scope TEXT NOT NULL,
    comment_id TEXT NOT NULL,
    version BIGINT NOT NULL,
    body BYTEA NOT NULL,
    status TEXT NOT NULL,
    updated_at BIGINT NOT NULL,
    PRIMARY KEY (scope, comment_id, version),
    FOREIGN KEY (scope, comment_id) REFERENCES notebook_comments(scope, id)
);

CREATE TABLE notebook_mutations (
    scope TEXT NOT NULL,
    operation_key TEXT NOT NULL,
    request_digest TEXT NOT NULL,
    resource_kind TEXT NOT NULL,
    resource_id TEXT NOT NULL,
    version BIGINT NOT NULL,
    revision_id TEXT NOT NULL DEFAULT '',
    created_at BIGINT NOT NULL,
    PRIMARY KEY (scope, operation_key)
);

INSERT INTO notebook_sections (scope, id, title, system_role, version, created_at, updated_at, deleted_at)
VALUES
    ('home', 'section-notes', 'Notes', 'notes', 1, (extract(epoch from now()) * 1000)::bigint, (extract(epoch from now()) * 1000)::bigint, 0),
    ('home', 'section-daily', 'Daily', 'daily', 1, (extract(epoch from now()) * 1000)::bigint, (extract(epoch from now()) * 1000)::bigint, 0),
    ('home', 'section-weekly', 'Weekly', 'weekly', 1, (extract(epoch from now()) * 1000)::bigint, (extract(epoch from now()) * 1000)::bigint, 0),
    ('home', 'section-monthly', 'Monthly', 'monthly', 1, (extract(epoch from now()) * 1000)::bigint, (extract(epoch from now()) * 1000)::bigint, 0);

INSERT INTO notebook_entries (scope, id, section_id, kind, title, head_revision_id, version, report_series_id, report_window_id, created_at, updated_at, deleted_at)
SELECT 'home', n.id, 'section-notes', 'note', 'Imported note', 'legacy:' || n.id, 1, '', '', n.created_at, n.created_at, 0
FROM notes n;

INSERT INTO notebook_revisions (scope, entry_id, revision_id, sequence, parent_revision_id, title, markdown, origin, base_revision_id, actor, created_at)
SELECT 'home', n.id, 'legacy:' || n.id, 1, '', 'Imported note', n.content, 'legacy', '', 'legacy-import', n.created_at
FROM notes n;
