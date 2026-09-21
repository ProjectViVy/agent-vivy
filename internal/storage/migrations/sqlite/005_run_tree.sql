
ALTER TABLE runs ADD COLUMN kind TEXT NOT NULL DEFAULT 'primary';
ALTER TABLE runs ADD COLUMN parent_run_id TEXT NOT NULL DEFAULT '';
ALTER TABLE runs ADD COLUMN root_run_id TEXT NOT NULL DEFAULT '';
ALTER TABLE runs ADD COLUMN depth INTEGER NOT NULL DEFAULT 0;
UPDATE runs SET root_run_id = id WHERE root_run_id = '';
CREATE INDEX runs_parent_idx ON runs(parent_run_id, created_at, id);
CREATE INDEX runs_root_idx ON runs(root_run_id, created_at, id);
ALTER TABLE approvals ADD COLUMN kind TEXT NOT NULL DEFAULT 'run';
