package studiocore

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLedgerFreshUsesFinalGenerationShape(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "studio.db")
	l, err := OpenLedger(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	rows, err := l.db.QueryContext(ctx, "PRAGMA table_info(generations)")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var cid, notnull, pk int
		var name, typ string
		var def any
		if err := rows.Scan(&cid, &name, &typ, &notnull, &def, &pk); err != nil {
			t.Fatal(err)
		}
		if name == "artifact_sha256" {
			t.Fatal("fresh ledger still requires artifact hash")
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
}
func TestLedgerRejectsOldAndUnknownBeforeWrites(t *testing.T) {
	for _, tc := range []struct{ name, ddl, want string }{
		{"old", `
CREATE TABLE IF NOT EXISTS worktrees (
	id TEXT PRIMARY KEY,
	path TEXT NOT NULL,
	kind TEXT NOT NULL,
	dirty INTEGER NOT NULL DEFAULT 0,
	created_at INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS generations (
	id TEXT PRIMARY KEY,
	parent_id TEXT NOT NULL DEFAULT '',
artifact_sha256 TEXT NOT NULL,
	source_ref TEXT NOT NULL,
	recipe_json TEXT NOT NULL DEFAULT '{}',
	phase TEXT NOT NULL,
	created_at INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS eval_runs (
	id TEXT PRIMARY KEY,
	candidate_id TEXT NOT NULL,
	baseline_id TEXT NOT NULL DEFAULT '',
	suite TEXT NOT NULL,
	verdict TEXT NOT NULL,
	journal_ref TEXT NOT NULL DEFAULT '',
	created_at INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS releases (
	id TEXT PRIMARY KEY,
	generation_id TEXT NOT NULL,
	eval_id TEXT NOT NULL DEFAULT '',
	actor TEXT NOT NULL,
	phase TEXT NOT NULL,
	created_at INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS installs (
	id TEXT PRIMARY KEY,
	release_id TEXT NOT NULL,
	target TEXT NOT NULL,
	phase TEXT NOT NULL,
	created_at INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS studio_events (
	seq INTEGER PRIMARY KEY AUTOINCREMENT,
	type TEXT NOT NULL,
	object_id TEXT NOT NULL,
	created_at INTEGER NOT NULL,
	payload TEXT NOT NULL DEFAULT '{}'
);
`, "upgrade_required"},
		{"partial", "CREATE TABLE mysterious(value TEXT)", "unsupported_source_format"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "studio.db")
			db, err := sql.Open("sqlite", path)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = db.Exec(tc.ddl); err != nil {
				t.Fatal(err)
			}
			db.Close()
			before, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			l, err := OpenLedger(context.Background(), path)
			if l != nil {
				l.Close()
			}
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("open = %v, want %s", err, tc.want)
			}
			after, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if string(before) != string(after) {
				t.Fatal("refusal changed source database")
			}
		})
	}
}
