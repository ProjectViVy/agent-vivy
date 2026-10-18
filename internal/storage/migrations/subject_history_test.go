package migrations

import (
	"context"
	"database/sql"
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

func TestIssue58ShellMigrationPreservesPopulated044(t *testing.T) {
	ctx := context.Background()
	manifest, err := Embedded()
	if err != nil {
		t.Fatal(err)
	}
	db := openMigrationTestDB(t)
	if _, err := db.Exec(`PRAGMA foreign_keys=ON`); err != nil {
		t.Fatal(err)
	}
	prior := Manifest{byDialect: map[Dialect][]Migration{SQLite: manifest.Migrations(SQLite)[:44], Postgres: manifest.Migrations(Postgres)[:44]}}
	if err := ApplyManifest(ctx, db, SQLite, prior); err != nil {
		t.Fatal(err)
	}
	for _, q := range []string{
		`INSERT INTO sessions(id,title,created_at) VALUES('s','original',1)`,
		`INSERT INTO messages(id,session_id,run_id,role,created_at,content,position) VALUES('m','s','r','user',1,'exact original',1)`,
		`INSERT INTO runs(id,session_id,status,created_at) VALUES('r','s','completed',1)`,
		`INSERT INTO subject_sources(session_id,subject_id,domain_id,conversation_id,binding_json) VALUES('s','subject','domain','','{}')`,
		`INSERT INTO run_subject_sources VALUES('r','m','s','{}')`,
		`INSERT INTO subject_interactions VALUES('subject','domain',1,'r:2','s','r',2,'digest','run.completed')`,
	} {
		if _, err := db.ExecContext(ctx, q); err != nil {
			t.Fatal(err)
		}
	}
	if len(manifest.Migrations(SQLite)) < 45 {
		t.Fatal("shell source schema still requires synthetic user evidence")
	}
	failed := Manifest{byDialect: map[Dialect][]Migration{SQLite: manifest.Migrations(SQLite), Postgres: manifest.Migrations(Postgres)}}
	failed.byDialect[SQLite][44].SQL += "\nSELECT * FROM issue58_missing_table;"
	if err := ApplyManifest(ctx, db, SQLite, failed); err == nil {
		t.Fatal("injected migration failure accepted")
	}
	if tableExists(t, db, "message_copy_sources") {
		t.Fatal("failed 045 migration left copy metadata")
	}
	var original string
	if err := db.QueryRow(`SELECT admitted_message_id FROM run_subject_sources WHERE run_id='r'`).Scan(&original); err != nil || original != "m" {
		t.Fatalf("rollback changed original pin %q %v", original, err)
	}
	if err := Apply(ctx, db, SQLite); err != nil {
		t.Fatal(err)
	}
	if err := Apply(ctx, db, SQLite); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM subject_interactions WHERE interaction_id='r:2' AND source_digest='digest'`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("upgrade lost sealed interaction: %d %v", count, err)
	}
	rows, err := db.Query(`PRAGMA foreign_key_check`)
	if err != nil {
		t.Fatal(err)
	}
	if rows.Next() {
		t.Fatal("migration broke original foreign keys")
	}
	rows.Close()
	if _, err := db.Exec(`DELETE FROM run_subject_sources WHERE run_id='r'`); err == nil {
		t.Fatal("migration removed restrictive source foreign key")
	}
	if _, err := db.Exec(`INSERT INTO runs(id,session_id,status,created_at) VALUES('shell','s','active',2)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO run_subject_sources VALUES('shell',NULL,'s','{}')`); err != nil {
		t.Fatalf("shell still requires fake user: %v", err)
	}
	var databaseID int
	var databaseName, databasePath string
	if err := db.QueryRow(`PRAGMA database_list`).Scan(&databaseID, &databaseName, &databasePath); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := sql.Open("sqlite", "file:"+databasePath)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	reopened.SetMaxOpenConns(1)
	if _, err := reopened.Exec(`PRAGMA foreign_keys=ON`); err != nil {
		t.Fatal(err)
	}
	if err := Apply(ctx, reopened, SQLite); err != nil {
		t.Fatalf("reopen migration failed: %v", err)
	}
	if err := reopened.QueryRow(`SELECT admitted_message_id FROM run_subject_sources WHERE run_id='r'`).Scan(&original); err != nil || original != "m" {
		t.Fatalf("reopen changed original admission: %s %v", original, err)
	}

}

// 046 adds the interaction disposition to a populated 045 database without
// touching existing rows, and its paired dialects stay logically identical.
func TestIssue58DispositionMigrationPreservesPopulated045(t *testing.T) {
	ctx := context.Background()
	manifest, err := Embedded()
	if err != nil {
		t.Fatal(err)
	}
	prior := Manifest{byDialect: map[Dialect][]Migration{SQLite: manifest.Migrations(SQLite)[:45], Postgres: manifest.Migrations(Postgres)[:45]}}
	db := openMigrationTestDB(t)
	if err := ApplyManifest(ctx, db, SQLite, prior); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`PRAGMA foreign_keys=OFF`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO subject_interactions(subject_id,domain_id,sequence,interaction_id,session_id,run_id,terminal_seq,source_digest,terminal_type) VALUES('s','d',1,'r:2','sess','r',2,'digest','run.completed')`); err != nil {
		t.Fatal(err)
	}
	if err := Apply(ctx, db, SQLite); err != nil {
		t.Fatal(err)
	}
	var disposition string
	if err := db.QueryRow(`SELECT disposition FROM subject_interactions WHERE run_id='r'`).Scan(&disposition); err != nil || disposition != "included" {
		t.Fatalf("existing interaction disposition %q %v", disposition, err)
	}
	if _, err := db.Exec(`UPDATE subject_interactions SET disposition='bogus'`); err == nil {
		t.Fatal("disposition accepted an unknown value")
	}
	sqlite, postgres := manifest.Migrations(SQLite)[45], manifest.Migrations(Postgres)[45]
	if sqlite.Version != 46 || postgres.Version != 46 || sqlite.Name != "subject_interaction_disposition" || postgres.Name != sqlite.Name || sqlite.SQL != postgres.SQL {
		t.Fatal("046 pair mismatch")
	}
}
