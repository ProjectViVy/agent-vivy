package migrations

import (
	"context"
	"strings"
	"testing"
)

func TestD12FreshMetadataHasOnlyFinalColumns(t *testing.T) {
	db := openMigrationTestDB(t)
	if err := Apply(context.Background(), db, SQLite); err != nil {
		t.Fatal(err)
	}
	cols, err := metadataColumns(context.Background(), db, SQLite)
	if err != nil {
		t.Fatal(err)
	}
	if len(cols) != 3 || !cols["version"] || !cols["name"] || !cols["applied_at"] {
		t.Fatalf("final metadata columns = %v", cols)
	}
}

func TestD12PristineInitializationFailureRollsBackEverything(t *testing.T) {
	db := openMigrationTestDB(t)
	manifest := testManifest(
		Migration{Version: 1, Name: "initial", SQL: "CREATE TABLE runner_base (id INTEGER);"},
		Migration{Version: 2, Name: "failure", SQL: "CREATE TABLE runner_partial (id INTEGER); SELECT * FROM missing_table;"},
	)
	if err := ApplyManifest(context.Background(), db, SQLite, manifest); err == nil {
		t.Fatal("expected injected initialization failure")
	}
	for _, table := range []string{"schema_migrations", "runner_base", "runner_partial"} {
		if tableExists(t, db, table) {
			t.Fatalf("atomic initialization retained %s", table)
		}
	}
}

func TestD12RefusesFinalMetadataWithWrongConstraints(t *testing.T) {
	db := openMigrationTestDB(t)
	ctx := context.Background()
	if err := Apply(ctx, db, SQLite); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`ALTER TABLE schema_migrations RENAME TO invalid_metadata;
 CREATE TABLE schema_migrations(version INTEGER, name TEXT, applied_at INTEGER);
 INSERT INTO schema_migrations SELECT * FROM invalid_metadata; DROP TABLE invalid_metadata;`); err != nil {
		t.Fatal(err)
	}
	if err := Apply(ctx, db, SQLite); err == nil {
		t.Fatal("malformed final metadata reopened")
	}
}

func TestD12RefusesViewReplacingTargetTable(t *testing.T) {
	db := openMigrationTestDB(t)
	ctx := context.Background()
	if err := Apply(ctx, db, SQLite); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`DROP TABLE notes; CREATE VIEW notes AS SELECT 'incomplete' AS id;`); err != nil {
		t.Fatal(err)
	}
	if err := Apply(ctx, db, SQLite); err == nil {
		t.Fatal("view substituted for owned target table")
	}
}

func TestD12RefusesTargetMissingRequiredColumn(t *testing.T) {
	db := openMigrationTestDB(t)
	ctx := context.Background()
	if err := Apply(ctx, db, SQLite); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`ALTER TABLE sessions DROP COLUMN title`); err != nil {
		t.Fatal(err)
	}
	if err := Apply(ctx, db, SQLite); err == nil {
		t.Fatal("incomplete target session shape reopened")
	}
}

func TestD12OldLineageHeadCannotImpersonateD03(t *testing.T) {
	db := openMigrationTestDB(t)
	ctx := context.Background()
	if err := Apply(ctx, db, SQLite); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`ALTER TABLE schema_migrations ADD COLUMN checksum TEXT NOT NULL DEFAULT 'old'; UPDATE schema_migrations SET name='notebook_content' WHERE version=36;`); err != nil {
		t.Fatal(err)
	}
	if err := Apply(ctx, db, SQLite); err == nil || !strings.Contains(err.Error(), "unsupported_source_format") {
		t.Fatalf("old lineage admission=%v", err)
	}
}

func TestD12RefusesTargetWithoutOperationKey(t *testing.T) {
	db := openMigrationTestDB(t)
	ctx := context.Background()
	if err := Apply(ctx, db, SQLite); err != nil {
		t.Fatal(err)
	}
	var ddl string
	if err := db.QueryRow(`SELECT sql FROM sqlite_master WHERE name='tool_operations'`).Scan(&ddl); err != nil {
		t.Fatal(err)
	}
	ddl = strings.Replace(ddl, "PRIMARY KEY (run_id, operation_id),", "", 1)
	if _, err := db.Exec(`DROP TABLE tool_operations;` + ddl); err != nil {
		t.Fatal(err)
	}
	if err := Apply(ctx, db, SQLite); err == nil {
		t.Fatal("operation owner without primary key reopened")
	}
}

func TestD12TargetShapeIncludesPostgresSerialIdentity(t *testing.T) {
	items := []Migration{{SQL: `CREATE TABLE objects (
 id BIGSERIAL PRIMARY KEY,
 body BYTEA NOT NULL
);`}}
	columns := targetColumns(items)
	keys := targetKeys(items)
	if columns["objects"]["id"] != "BIGINT" || len(keys["objects"]) != 1 || keys["objects"][0] != "id" {
		t.Fatalf("serial identity missing: %v/%v", columns, keys)
	}
}
