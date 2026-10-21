package migrations

import (
	"context"
	"errors"
	"testing"
)

func TestD08PreflightRejectsPartialLegacySnapshotShape(t *testing.T) {
	for _, fixture := range []struct{ table, column string }{{"checkpoints", "checksum"}, {"run_prompt_snapshots", "payload_sha256"}} {
		t.Run(fixture.table, func(t *testing.T) {
			db := openMigrationTestDB(t)
			ctx := context.Background()
			manifest, err := Embedded()
			if err != nil {
				t.Fatal(err)
			}
			prefix := Manifest{byDialect: map[Dialect][]Migration{SQLite: manifest.Migrations(SQLite)[:36], Postgres: manifest.Migrations(Postgres)[:36]}}
			if err := applyManifest(ctx, db, SQLite, prefix); err != nil {
				t.Fatal(err)
			}
			if _, err := db.Exec(`ALTER TABLE schema_migrations ADD COLUMN checksum TEXT NOT NULL DEFAULT 'historical'; ALTER TABLE ` + fixture.table + ` DROP COLUMN ` + fixture.column); err != nil {
				t.Fatal(err)
			}
			if err := Apply(ctx, db, SQLite); !errors.Is(err, ErrUnsupportedSourceFormat) {
				t.Fatalf("partial old snapshot shape recognized: %v", err)
			}
			var head int
			if err := db.QueryRow(`SELECT MAX(version) FROM schema_migrations`).Scan(&head); err != nil || head != 36 {
				t.Fatalf("refusal mutated source: %d %v", head, err)
			}
		})
	}
}

func TestD08KnownMetadata36AdvancesThroughOwnerAndPreservesBytes(t *testing.T) {
	db := openMigrationTestDB(t)
	ctx := context.Background()
	manifest, err := Embedded()
	if err != nil {
		t.Fatal(err)
	}
	prefix := Manifest{byDialect: map[Dialect][]Migration{SQLite: manifest.Migrations(SQLite)[:36], Postgres: manifest.Migrations(Postgres)[:36]}}
	if err := applyManifest(ctx, db, SQLite, prefix); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO sessions(id,title,created_at) VALUES('s','owner',1);
 INSERT INTO runs(id,session_id,status,created_at) VALUES('r','s','completed',1);
 INSERT INTO run_prompt_snapshots(run_id,schema_version,composer_version,generation_id,payload,payload_sha256) VALUES('r',1,'v1','g',CAST('{"kept":"prompt"}' AS BLOB),'historical');
 INSERT INTO checkpoint_generations(id,generation,blob) VALUES('private-owner',1,CAST('kept opaque bytes' AS BLOB));
 INSERT INTO checkpoints(id,generation,checksum,created_at) VALUES('private-owner',1,'historical',1);`); err != nil {
		t.Fatal(err)
	}
	if err := Apply(ctx, db, SQLite); err != nil {
		t.Fatal(err)
	}
	for _, fixture := range []struct{ table, column string }{{"checkpoints", "checksum"}, {"run_prompt_snapshots", "payload_sha256"}} {
		if exists, err := sourceColumnExists(ctx, db, SQLite, fixture.table, fixture.column); err != nil || exists {
			t.Fatalf("retired column remains: %s %v", fixture.table, err)
		}
	}
	var payload, blob []byte
	if err := db.QueryRow(`SELECT payload FROM run_prompt_snapshots WHERE run_id='r'`).Scan(&payload); err != nil || string(payload) != `{"kept":"prompt"}` {
		t.Fatalf("prompt changed: %q %v", payload, err)
	}
	if err := db.QueryRow(`SELECT blob FROM checkpoint_generations WHERE id='private-owner'`).Scan(&blob); err != nil || string(blob) != "kept opaque bytes" {
		t.Fatalf("opaque bytes changed: %q %v", blob, err)
	}
	columns, err := metadataColumns(ctx, db, SQLite)
	if err != nil || !finalMetadataColumns(columns) {
		t.Fatalf("metadata owner changed: %v", err)
	}
	if err := Apply(ctx, db, SQLite); err != nil {
		t.Fatal(err)
	}
}
