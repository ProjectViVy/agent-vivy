
CREATE TABLE todos (
	 id TEXT NOT NULL,
	 session_id TEXT NOT NULL,
	 subject TEXT NOT NULL,
	 description TEXT NOT NULL,
	 status TEXT NOT NULL,
	 blocks_json BLOB NOT NULL,
	 blocked_by_json BLOB NOT NULL,
	 active_form TEXT NOT NULL,
	 owner TEXT NOT NULL,
	 metadata_json BLOB NOT NULL,
	 position INTEGER NOT NULL,
	 created_at INTEGER NOT NULL,
	 updated_at INTEGER NOT NULL,
	 PRIMARY KEY(session_id, id),
	 FOREIGN KEY(session_id) REFERENCES sessions(id)
);
CREATE INDEX todos_session_position_idx ON todos(session_id, position, id);
