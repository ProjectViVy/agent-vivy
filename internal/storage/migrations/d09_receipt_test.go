package migrations

import (
	"context"
	"testing"
)

func TestD09ReceiptUpgradePreservesLegacyOutcomeWithoutInputAuthority(t *testing.T) {
	ctx := context.Background()
	db := openMigrationTestDB(t)
	manifest, err := Embedded()
	if err != nil {
		t.Fatal(err)
	}
	prior := Manifest{byDialect: map[Dialect][]Migration{SQLite: manifest.Migrations(SQLite)[:38], Postgres: manifest.Migrations(Postgres)[:38]}}
	if err := applyManifest(ctx, db, SQLite, prior); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO sessions(id,title,created_at) VALUES('s','retained',1);
 INSERT INTO runs(id,session_id,status,created_at) VALUES('r','s','completed',2);
 INSERT INTO run_events(run_id,seq,type,created_at,payload_version,payload) VALUES('r',1,'context.reference_attached',3,1,CAST('{"retained":"outcome"}' AS BLOB));
 INSERT INTO continuity_receipts(session_id,operation,request_id,input_hash,run_id,event_seq) VALUES('s','reference.attach','call','unrecoverable','r',1);`); err != nil {
		t.Fatal(err)
	}
	if err := Apply(ctx, db, SQLite); err != nil {
		t.Fatal(err)
	}
	if exists, err := sourceColumnExists(ctx, db, SQLite, "continuity_receipts", "input_hash"); err != nil || exists {
		t.Fatalf("retired input column: %v %v", exists, err)
	}
	var input []byte
	var run string
	var seq int
	if err := db.QueryRow(`SELECT intended_input,run_id,event_seq FROM continuity_receipts WHERE session_id='s'`).Scan(&input, &run, &seq); err != nil || input != nil || run != "r" || seq != 1 {
		t.Fatalf("legacy receipt reconstructed or lost: %v %s %d %v", input, run, seq, err)
	}
	var payload []byte
	if err := db.QueryRow(`SELECT payload FROM run_events WHERE run_id='r' AND seq=1`).Scan(&payload); err != nil || string(payload) != `{"retained":"outcome"}` {
		t.Fatalf("outcome changed: %q %v", payload, err)
	}
	if err := Apply(ctx, db, SQLite); err != nil {
		t.Fatal(err)
	}
}

func TestD09ReceiptMigrationRollsBackWithOutcomeIntact(t *testing.T) {
	ctx := context.Background()
	db := openMigrationTestDB(t)
	manifest, err := Embedded()
	if err != nil {
		t.Fatal(err)
	}
	prior := Manifest{byDialect: map[Dialect][]Migration{SQLite: manifest.Migrations(SQLite)[:38], Postgres: manifest.Migrations(Postgres)[:38]}}
	if err := applyManifest(ctx, db, SQLite, prior); err != nil {
		t.Fatal(err)
	}
	items := manifest.Migrations(SQLite)
	items[38].SQL += `SELECT * FROM d09_receipt_missing;`
	if err := ApplyManifest(ctx, db, SQLite, testManifest(items...)); err == nil {
		t.Fatal("injected migration succeeded")
	}
	if exists, err := sourceColumnExists(ctx, db, SQLite, "continuity_receipts", "input_hash"); err != nil || !exists {
		t.Fatalf("rollback lost old receipt column: %v %v", exists, err)
	}
	if exists, err := sourceColumnExists(ctx, db, SQLite, "continuity_receipts", "intended_input"); err != nil || exists {
		t.Fatalf("failed schema escaped rollback: %v %v", exists, err)
	}
	var head int
	if err := db.QueryRow(`SELECT MAX(version) FROM schema_migrations`).Scan(&head); err != nil || head != 38 {
		t.Fatalf("rollback metadata=%d: %v", head, err)
	}
	if err := Apply(ctx, db, SQLite); err != nil {
		t.Fatal(err)
	}
}
