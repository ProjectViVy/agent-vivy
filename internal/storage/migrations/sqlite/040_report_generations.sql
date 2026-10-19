CREATE TABLE IF NOT EXISTS report_generations (
    scope            TEXT NOT NULL,
    run_id           TEXT NOT NULL,
    period           TEXT NOT NULL,
    series_id        TEXT NOT NULL,
    window_id        TEXT NOT NULL,
    entry_id         TEXT NOT NULL DEFAULT '',
    revision_id      TEXT NOT NULL DEFAULT '',
    config_revision  INTEGER NOT NULL DEFAULT 0,
    timezone         TEXT NOT NULL DEFAULT '',
    window_start_ms  INTEGER NOT NULL DEFAULT 0,
    window_end_ms    INTEGER NOT NULL DEFAULT 0,
    as_of_ms         INTEGER NOT NULL DEFAULT 0,
    input_digest     TEXT NOT NULL DEFAULT '',
    facts_digest     TEXT NOT NULL DEFAULT '',
    provider         TEXT NOT NULL DEFAULT '',
    model_id         TEXT NOT NULL DEFAULT '',
    outcome_mode     TEXT NOT NULL DEFAULT '',
    outcome_reason   TEXT NOT NULL DEFAULT '',
    facts_json       TEXT NOT NULL DEFAULT '',
    feedback_json    TEXT NOT NULL DEFAULT '',
    payload_json     TEXT NOT NULL DEFAULT '',
    created_at       INTEGER NOT NULL,
    PRIMARY KEY (scope, run_id),
    UNIQUE (scope, run_id)
);
CREATE INDEX IF NOT EXISTS report_generations_series_idx
    ON report_generations (scope, series_id, window_id);

CREATE UNIQUE INDEX IF NOT EXISTS notebook_entries_series_uniq
    ON notebook_entries(scope, report_series_id, report_window_id)
    WHERE report_series_id <> '';
