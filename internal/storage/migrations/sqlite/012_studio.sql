
CREATE TABLE generations (
	id TEXT PRIMARY KEY,
	parent_id TEXT NOT NULL DEFAULT '',
	artifact_sha256 TEXT NOT NULL,
	source_ref TEXT NOT NULL DEFAULT '',
	recipe_json BLOB NOT NULL,
	phase TEXT NOT NULL,
	created_at INTEGER NOT NULL
);
CREATE TABLE eval_runs (
	id TEXT PRIMARY KEY,
	candidate_id TEXT NOT NULL,
	baseline_id TEXT NOT NULL DEFAULT '',
	suite TEXT NOT NULL,
	verdict TEXT NOT NULL,
	journal_ref TEXT NOT NULL DEFAULT '',
	created_at INTEGER NOT NULL,
	FOREIGN KEY(candidate_id) REFERENCES generations(id)
);
CREATE INDEX eval_runs_candidate_idx ON eval_runs(candidate_id, created_at, id);
CREATE TABLE promotions (
	id TEXT PRIMARY KEY,
	from_id TEXT NOT NULL,
	to_id TEXT NOT NULL,
	eval_id TEXT NOT NULL,
	actor TEXT NOT NULL,
	phase TEXT NOT NULL,
	applies_at TEXT NOT NULL,
	created_at INTEGER NOT NULL,
	FOREIGN KEY(from_id) REFERENCES generations(id),
	FOREIGN KEY(to_id) REFERENCES generations(id)
);
CREATE UNIQUE INDEX promotions_from_accepted_idx ON promotions(from_id) WHERE phase = 'accepted';
CREATE TABLE studio_events (
	seq INTEGER PRIMARY KEY AUTOINCREMENT,
	type TEXT NOT NULL,
	object_id TEXT NOT NULL,
	created_at INTEGER NOT NULL,
	payload BLOB NOT NULL
);
