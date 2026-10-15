package migrations

import (
	"context"
	"strings"
	"testing"
)

func TestIssue58MigrationUpgradeAndRollback(t *testing.T) {
	ctx := context.Background()
	manifest, err := Embedded()
	if err != nil {
		t.Fatal(err)
	}
	prior := Manifest{byDialect: map[Dialect][]Migration{SQLite: manifest.Migrations(SQLite)[:43], Postgres: manifest.Migrations(Postgres)[:43]}}
	db := openMigrationTestDB(t)
	if err := ApplyManifest(ctx, db, SQLite, prior); err != nil {
		t.Fatal(err)
	}
	// Inject failure after the real 044 DDL. No 044 table or marker may survive.
	failed := Manifest{byDialect: map[Dialect][]Migration{SQLite: manifest.Migrations(SQLite), Postgres: manifest.Migrations(Postgres)}}
	failed.byDialect[SQLite][43].SQL += "\nSELECT * FROM issue58_missing_table;"
	if err := ApplyManifest(ctx, db, SQLite, failed); err == nil || !strings.Contains(err.Error(), "migration 44") {
		t.Fatalf("expected 044 failure: %v", err)
	}
	for _, table := range []string{"subject_sources", "subject_main_conversations", "run_subject_sources", "subject_history_heads", "subject_interactions"} {
		if tableExists(t, db, table) {
			t.Fatalf("failed migration left %s", table)
		}
	}
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM schema_migrations`).Scan(&count); err != nil || count != 43 {
		t.Fatalf("failed migration marker %d %v", count, err)
	}
	if err := Apply(ctx, db, SQLite); err != nil {
		t.Fatal(err)
	}
	if err := Apply(ctx, db, SQLite); err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"subject_sources", "subject_main_conversations", "run_subject_sources", "subject_history_heads", "subject_interactions"} {
		if !tableExists(t, db, table) {
			t.Fatalf("upgrade omitted %s", table)
		}
	}
	sqlite, postgres := manifest.Migrations(SQLite)[43], manifest.Migrations(Postgres)[43]
	if sqlite.Version != 44 || postgres.Version != 44 || sqlite.Name != "subject_history" || postgres.Name != sqlite.Name {
		t.Fatal("044 pair mismatch")
	}
	if strings.ReplaceAll(sqlite.SQL, " BLOB", " BYTEA") != postgres.SQL {
		t.Fatal("subject history logical schema drift")
	}
}
