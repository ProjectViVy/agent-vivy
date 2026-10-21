package migrations

import (
	"bytes"
	"context"
	"database/sql"
	"testing"
)

func d09ApprovalSource(t *testing.T) (*sql.DB, Manifest) {
	t.Helper()
	db := openMigrationTestDB(t)
	manifest, err := Embedded()
	if err != nil {
		t.Fatal(err)
	}
	prior := Manifest{byDialect: map[Dialect][]Migration{SQLite: manifest.Migrations(SQLite)[:38], Postgres: manifest.Migrations(Postgres)[:38]}}
	if err := applyManifest(context.Background(), db, SQLite, prior); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO sessions(id,title,created_at) VALUES('s','retained',1);
 INSERT INTO runs(id,session_id,status,created_at) VALUES('r','s','completed',2);
 INSERT INTO approvals(id,run_id,tool_call_id,decision,expires_at,precondition_hash,proposal_data) VALUES('a','r','call','approved',9,'historical',CAST('{"opaque":"private-before-state"}' AS BLOB));
 INSERT INTO file_reads(session_id,path,read_at) VALUES('s','old.txt',99);
 INSERT INTO file_versions(session_id,run_id,path,version,content_hash,content,created_at) VALUES('s','r','old.txt',1,'retired',CAST('preserved' AS BLOB),7);
 INSERT INTO run_events(run_id,seq,type,created_at,payload_version,payload) VALUES('r',1,'tool.approval_required',3,1,CAST('{"precondition_hash":"historical"}' AS BLOB));`); err != nil {
		t.Fatal(err)
	}
	return db, manifest
}

func TestD09ApprovalSchemaUpgradePreservesPrivateInputAndHistory(t *testing.T) {
	db, _ := d09ApprovalSource(t)
	ctx := context.Background()
	if err := Apply(ctx, db, SQLite); err != nil {
		t.Fatal(err)
	}
	if exists, err := sourceColumnExists(ctx, db, SQLite, "approvals", "precondition_hash"); err != nil || exists {
		t.Fatalf("retired approval column: %v %v", exists, err)
	}
	if exists, err := sourceColumnExists(ctx, db, SQLite, "file_versions", "content_hash"); err != nil || exists {
		t.Fatalf("retired version column: %v %v", exists, err)
	}
	var readExists sql.NullInt64
	var readContent []byte
	if err := db.QueryRow(`SELECT read_exists,read_content FROM file_reads WHERE session_id='s' AND path='old.txt'`).Scan(&readExists, &readContent); err != nil || readExists.Valid || readContent != nil {
		t.Fatalf("historical read was reconstructed: %v %v", readExists, err)
	}
	var version []byte
	if err := db.QueryRow(`SELECT content FROM file_versions WHERE path='old.txt'`).Scan(&version); err != nil || string(version) != "preserved" {
		t.Fatalf("history changed: %q %v", version, err)
	}
	var private, history []byte
	if err := db.QueryRow(`SELECT proposal_data FROM approvals WHERE id='a'`).Scan(&private); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(private, []byte(`{"opaque":"private-before-state"}`)) {
		t.Fatal("private input changed")
	}
	if err := db.QueryRow(`SELECT payload FROM run_events WHERE run_id='r' AND seq=1`).Scan(&history); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(history, []byte(`{"precondition_hash":"historical"}`)) {
		t.Fatal("historical Journal changed")
	}
	if err := Apply(ctx, db, SQLite); err != nil {
		t.Fatal(err)
	}
}

func TestD09ApprovalDropRollsBackOnFailure(t *testing.T) {
	db, manifest := d09ApprovalSource(t)
	ctx := context.Background()
	items := manifest.Migrations(SQLite)
	items[38].SQL += `SELECT * FROM d09_missing_table;`
	if err := ApplyManifest(ctx, db, SQLite, testManifest(items...)); err == nil {
		t.Fatal("injected D09 migration succeeded")
	}
	if exists, err := sourceColumnExists(ctx, db, SQLite, "approvals", "precondition_hash"); err != nil || !exists {
		t.Fatalf("rollback lost old column: %v %v", exists, err)
	}
	var head int
	if err := db.QueryRow(`SELECT MAX(version) FROM schema_migrations`).Scan(&head); err != nil || head != 38 {
		t.Fatalf("rollback metadata: %d %v", head, err)
	}
	if err := Apply(ctx, db, SQLite); err != nil {
		t.Fatal(err)
	}
}
