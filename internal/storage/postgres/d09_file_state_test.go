package postgres

import (
	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage/migrations"
	"testing"
)

func TestD09FileSchemaRollbackAndReopenPostgres(t *testing.T) {
	ctx, db, manifest := generationFixturePool(t)
	buildGenerationPrefix(t, ctx, db, manifest, 38)
	if _, err := db.ExecContext(ctx, `
 INSERT INTO sessions(id,title,created_at) VALUES('s','retained',1);
 INSERT INTO runs(id,session_id,status,created_at) VALUES('r','s','completed',2);
 INSERT INTO approvals(id,run_id,tool_call_id,decision,expires_at,precondition_hash,proposal_data) VALUES('a','r','call','approved',9,'retired',convert_to('{"opaque":"private"}','UTF8'));
 INSERT INTO run_events(run_id,seq,type,created_at,payload_version,payload) VALUES('r',1,'tool.approval_required',3,1,convert_to('{"precondition_hash":"historical"}','UTF8'));
 INSERT INTO file_reads(session_id,path,read_at) VALUES('s','old.txt',99);
 INSERT INTO file_versions(session_id,run_id,path,version,content_hash,content,created_at) VALUES('s','r','old.txt',1,'retired',convert_to('preserved','UTF8'),7);
 CREATE VIEW file_version_drop_guard AS SELECT content_hash FROM file_versions;`); err != nil {
		t.Fatal(err)
	}
	// The fixture executor reaches the second DROP, proving the first DROP and
	// marker roll back together. The production preflight independently refuses views.
	if err := applyGenerationCopyFixture(ctx, db, manifest); err == nil {
		t.Fatal("injected dependent-view failure succeeded")
	}
	var old string
	if err := db.QueryRowContext(ctx, `SELECT precondition_hash FROM approvals WHERE id='a'`).Scan(&old); err != nil || old != "retired" {
		t.Fatalf("rollback lost prior approval: %q %v", old, err)
	}
	var head int
	if err := db.QueryRowContext(ctx, `SELECT MAX(version) FROM schema_migrations`).Scan(&head); err != nil || head != 38 {
		t.Fatalf("rollback marker: %d %v", head, err)
	}
	if _, err := db.ExecContext(ctx, `DROP VIEW file_version_drop_guard`); err != nil {
		t.Fatal(err)
	}
	if err := migrations.Apply(ctx, db, migrations.Postgres); err != nil {
		t.Fatal(err)
	}
	var private, history, version []byte
	if err := db.QueryRowContext(ctx, `SELECT proposal_data FROM approvals WHERE id='a'`).Scan(&private); err != nil || string(private) != `{"opaque":"private"}` {
		t.Fatalf("private state changed: %q %v", private, err)
	}
	if err := db.QueryRowContext(ctx, `SELECT payload FROM run_events WHERE run_id='r' AND seq=1`).Scan(&history); err != nil || string(history) != `{"precondition_hash":"historical"}` {
		t.Fatalf("Journal changed: %q %v", history, err)
	}
	if err := db.QueryRowContext(ctx, `SELECT content FROM file_versions WHERE path='old.txt'`).Scan(&version); err != nil || string(version) != "preserved" {
		t.Fatalf("file history changed: %q %v", version, err)
	}
	backend := &Backend{db: &DB{SQL: db}}
	state, tracked, err := backend.LastFileAccess(ctx, "s", "old.txt")
	if err != nil || !tracked || state != nil {
		t.Fatalf("historical read reconstructed: %+v %v %v", state, tracked, err)
	}
	for _, expected := range []domain.FileReadState{{Exists: true, Content: []byte{}}, {Exists: false, Content: []byte{}}} {
		if err := backend.TrackFileAccess(ctx, "s", "old.txt", expected); err != nil {
			t.Fatal(err)
		}
		got, ok, err := backend.LastFileAccess(ctx, "s", "old.txt")
		if err != nil || !ok || got == nil || got.Exists != expected.Exists || len(got.Content) != 0 {
			t.Fatalf("read state collapsed: %+v %v %v", got, ok, err)
		}
	}
	if _, err := db.ExecContext(ctx, `UPDATE file_reads SET read_exists=1,read_content=NULL WHERE session_id='s' AND path='old.txt'`); err != nil {
		t.Fatal(err)
	}
	state, tracked, err = backend.LastFileAccess(ctx, "s", "old.txt")
	if err != nil || !tracked || state != nil {
		t.Fatalf("NULL content authorized empty bytes: %+v %v %v", state, tracked, err)
	}
	if err := migrations.Apply(ctx, db, migrations.Postgres); err != nil {
		t.Fatal(err)
	}
}
