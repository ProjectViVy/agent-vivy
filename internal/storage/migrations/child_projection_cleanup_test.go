package migrations

import (
	"context"
	"testing"
)

func TestOneShotChildProjectionCleanupUpgrade(t *testing.T) {
	ctx := context.Background()
	db := openMigrationTestDB(t)
	manifest, err := Embedded()
	if err != nil {
		t.Fatal(err)
	}
	prior := Manifest{byDialect: map[Dialect][]Migration{
		SQLite:   manifest.Migrations(SQLite)[:47],
		Postgres: manifest.Migrations(Postgres)[:47],
	}}
	if err := applyManifest(ctx, db, SQLite, prior); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO sessions(id,title,created_at) VALUES
 ('sess_parent','parent',1),('sess_child','continuable child',2);
 INSERT INTO runs(id,session_id,status,created_at,kind,child_mode) VALUES
 ('run_parent','sess_parent','completed',3,'primary',''),
 ('run_child_oneshot','sess_parent','completed',4,'child','one-shot'),
 ('run_child_legacy','sess_parent','completed',5,'child',''),
 ('run_child_continuable','sess_child','completed',6,'child','continuable');
 INSERT INTO messages(id,session_id,run_id,role,created_at,content,position) VALUES
 ('msg_parent','sess_parent','run_parent','assistant',10,CAST('parent answer' AS BLOB),1),
 ('msg_child_oneshot','sess_parent','run_child_oneshot','assistant',11,CAST('one-shot child answer' AS BLOB),2),
 ('msg_child_legacy','sess_parent','run_child_legacy','assistant',12,CAST('legacy child answer' AS BLOB),3),
 ('msg_child_continuable','sess_child','run_child_continuable','assistant',13,CAST('continuable child answer' AS BLOB),1);`); err != nil {
		t.Fatal(err)
	}

	if err := Apply(ctx, db, SQLite); err != nil {
		t.Fatal(err)
	}

	for id, wantKept := range map[string]bool{
		"msg_parent":            true,
		"msg_child_continuable": true,
		"msg_child_oneshot":     false,
		"msg_child_legacy":      false,
	} {
		var count int
		if err := db.QueryRow(`SELECT COUNT(*) FROM messages WHERE id = ?`, id).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if kept := count == 1; kept != wantKept {
			t.Fatalf("message %s kept = %v, want %v", id, kept, wantKept)
		}
	}
}
