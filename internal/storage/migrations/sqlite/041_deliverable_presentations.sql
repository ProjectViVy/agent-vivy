CREATE TABLE deliverable_presentations (
    session_id TEXT NOT NULL,
    item_id TEXT NOT NULL,
    run_id TEXT NOT NULL,
    content BLOB NOT NULL,
    PRIMARY KEY (session_id, item_id),
    FOREIGN KEY (session_id) REFERENCES sessions(id),
    FOREIGN KEY (run_id) REFERENCES runs(id)
);
CREATE INDEX deliverable_presentations_run_idx ON deliverable_presentations(session_id, run_id);
