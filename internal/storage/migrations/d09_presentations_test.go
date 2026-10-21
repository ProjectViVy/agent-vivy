package migrations

import (
	"context"
	"testing"
)

func TestD09PresentationSchemaFreshAndReopen(t *testing.T) {
	db := openMigrationTestDB(t)
	ctx := context.Background()
	if err := Apply(ctx, db, SQLite); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO sessions(id,title,created_at) VALUES('s','owned',1); INSERT INTO runs(id,session_id,status,created_at) VALUES('r','s','active',1); INSERT INTO deliverable_presentations(session_id,item_id,run_id,content) VALUES('s','empty','r',X'')`); err != nil {
		t.Fatal(err)
	}
	if err := Apply(ctx, db, SQLite); err != nil {
		t.Fatal(err)
	}
	var size int
	if err := db.QueryRow(`SELECT length(content) FROM deliverable_presentations WHERE item_id='empty'`).Scan(&size); err != nil || size != 0 {
		t.Fatalf("known empty presentation lost: %d %v", size, err)
	}
}

func TestD09PresentationUpgradePreservesHistoricalEventWithoutGuessingBytes(t *testing.T) {
	db := openMigrationTestDB(t)
	ctx := context.Background()
	manifest, err := Embedded()
	if err != nil {
		t.Fatal(err)
	}
	prior := Manifest{byDialect: map[Dialect][]Migration{SQLite: manifest.Migrations(SQLite)[:40], Postgres: manifest.Migrations(Postgres)[:40]}}
	if err := applyManifest(ctx, db, SQLite, prior); err != nil {
		t.Fatal(err)
	}
	historical := `{"delivery_set":{"id":"legacy","items":[{"id":"legacy-item","sha256":"frozen-historical-value"}]}}`
	if _, err := db.Exec(`INSERT INTO run_events(run_id,seq,type,created_at,payload_version,payload) VALUES('old-run',1,'deliverables.presented',1,1,?)`, []byte(historical)); err != nil {
		t.Fatal(err)
	}
	if err := Apply(ctx, db, SQLite); err != nil {
		t.Fatal(err)
	}
	var count int
	var payload []byte
	if err := db.QueryRow(`SELECT COUNT(*) FROM deliverable_presentations`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("guessed historical bytes: %d %v", count, err)
	}
	if err := db.QueryRow(`SELECT payload FROM run_events WHERE run_id='old-run'`).Scan(&payload); err != nil || string(payload) != historical {
		t.Fatalf("historical event changed: %v", err)
	}
	if err := Apply(ctx, db, SQLite); err != nil {
		t.Fatal(err)
	}
}

func TestD09PresentationMigrationRollback(t *testing.T) {
	db := openMigrationTestDB(t)
	ctx := context.Background()
	manifest, err := Embedded()
	if err != nil {
		t.Fatal(err)
	}
	prior := Manifest{byDialect: map[Dialect][]Migration{SQLite: manifest.Migrations(SQLite)[:40], Postgres: manifest.Migrations(Postgres)[:40]}}
	if err := applyManifest(ctx, db, SQLite, prior); err != nil {
		t.Fatal(err)
	}
	items := manifest.Migrations(SQLite)
	items[40].SQL += `SELECT * FROM missing_d09_presentation;`
	if err := ApplyManifest(ctx, db, SQLite, testManifest(items...)); err == nil {
		t.Fatal("injected migration succeeded")
	}
	if tableExists(t, db, "deliverable_presentations") {
		t.Fatal("failed migration retained private table")
	}
	var head int
	if err := db.QueryRow(`SELECT MAX(version) FROM schema_migrations`).Scan(&head); err != nil || head != 40 {
		t.Fatalf("failed migration marker: %d %v", head, err)
	}
	if err := Apply(ctx, db, SQLite); err != nil {
		t.Fatal(err)
	}
}
