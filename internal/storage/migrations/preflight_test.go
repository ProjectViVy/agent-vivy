package migrations

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestApplyPreflightRefusesOldAndUnknownWithoutWrites(t *testing.T) {
	for _, fixture := range []struct{ name, sql, want string }{
		{"unexplained", `CREATE TABLE unknown_data (value TEXT); INSERT INTO unknown_data VALUES ('preserve')`, "unsupported_source_format"},
		{"unexplained-view", `CREATE VIEW unknown_data AS SELECT 'preserve'`, "unsupported_source_format"},
		{"partial-metadata", `CREATE TABLE schema_migrations (version INTEGER PRIMARY KEY, name TEXT); INSERT INTO schema_migrations VALUES (35,'workflow_definitions')`, "unsupported_source_format"},
		{"number-is-not-target", `CREATE TABLE schema_migrations (version INTEGER PRIMARY KEY, name TEXT, checksum TEXT, applied_at INTEGER); INSERT INTO schema_migrations VALUES (36,'tool_operation_private_binding','wrong',1)`, "unsupported_source_format"},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			db := openMigrationTestDB(t)
			ctx := context.Background()
			if _, err := db.ExecContext(ctx, fixture.sql); err != nil {
				t.Fatal(err)
			}
			var before string
			if err := db.QueryRow(`SELECT group_concat(sql,';') FROM sqlite_master ORDER BY name`).Scan(&before); err != nil {
				t.Fatal(err)
			}
			err := Apply(ctx, db, SQLite)
			if err == nil || !strings.Contains(err.Error(), fixture.want) {
				t.Fatalf("Apply=%v, want %s", err, fixture.want)
			}
			var after string
			if err := db.QueryRow(`SELECT group_concat(sql,';') FROM sqlite_master ORDER BY name`).Scan(&after); err != nil {
				t.Fatal(err)
			}
			if after != before {
				t.Fatal("refused source schema changed")
			}
		})
	}
	manifest, err := Embedded()
	if err != nil {
		t.Fatal(err)
	}
	for _, head := range []int{1, 16, 17, 23, 35} {
		t.Run(fmt.Sprintf("old-prefix-%d", head), func(t *testing.T) {
			db := openMigrationTestDB(t)
			ctx := context.Background()
			old := Manifest{byDialect: map[Dialect][]Migration{SQLite: manifest.Migrations(SQLite)[:head], Postgres: manifest.Migrations(Postgres)[:head]}}
			if err := ApplyManifest(ctx, db, SQLite, old); err != nil {
				t.Fatal(err)
			}
			err := Apply(ctx, db, SQLite)
			if err == nil || !strings.Contains(err.Error(), "upgrade_required") {
				t.Fatalf("Apply=%v", err)
			}
			var count int
			if err := db.QueryRow(`SELECT COUNT(*) FROM schema_migrations`).Scan(&count); err != nil || count != head {
				t.Fatalf("old metadata changed: %d, %v", count, err)
			}
		})
	}
}

func TestApplyPreflightRejectsUnexplainedTablesOnKnownMetadata(t *testing.T) {
	manifest, err := Embedded()
	if err != nil {
		t.Fatal(err)
	}
	for _, head := range []int{35, 36} {
		t.Run(fmt.Sprintf("head-%d", head), func(t *testing.T) {
			db := openMigrationTestDB(t)
			ctx := context.Background()
			fixture := Manifest{byDialect: map[Dialect][]Migration{SQLite: manifest.Migrations(SQLite)[:head], Postgres: manifest.Migrations(Postgres)[:head]}}
			if err := ApplyManifest(ctx, db, SQLite, fixture); err != nil {
				t.Fatal(err)
			}
			if _, err := db.Exec(`CREATE TABLE unlisted_source_data (value TEXT); INSERT INTO unlisted_source_data VALUES ('preserve')`); err != nil {
				t.Fatal(err)
			}
			if err := Apply(ctx, db, SQLite); !errors.Is(err, ErrUnsupportedSourceFormat) {
				t.Fatalf("unexplained schema opened: %v", err)
			}
			var count int
			if err := db.QueryRow(`SELECT COUNT(*) FROM schema_migrations`).Scan(&count); err != nil || count != head {
				t.Fatalf("refusal changed metadata: %d %v", count, err)
			}
		})
	}
}

func TestApplyPreflightRejectsTargetMissingBindingColumns(t *testing.T) {
	db := openMigrationTestDB(t)
	ctx := context.Background()
	if err := Apply(ctx, db, SQLite); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`ALTER TABLE tool_operations DROP COLUMN request_arguments`); err != nil {
		t.Fatal(err)
	}
	if err := Apply(ctx, db, SQLite); err == nil || !strings.Contains(err.Error(), "unsupported_source_format") {
		t.Fatalf("incomplete target reopened: %v", err)
	}
}

func TestApplyPreflightRejectsConversionWithoutTargetMarker(t *testing.T) {
	db := openMigrationTestDB(t)
	ctx := context.Background()
	if err := Apply(ctx, db, SQLite); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`DELETE FROM schema_migrations WHERE version=36`); err != nil {
		t.Fatal(err)
	}
	if err := Apply(ctx, db, SQLite); !errors.Is(err, ErrUnsupportedSourceFormat) {
		t.Fatalf("incomplete target metadata accepted: %v", err)
	}
}

func TestApplyPreflightDoesNotHideSQLiteLikeNames(t *testing.T) {
	for _, initialized := range []bool{false, true} {
		for _, object := range []struct{ name, ddl string }{
			{"table", "CREATE TABLE sqliteXforeign (value TEXT); INSERT INTO sqliteXforeign VALUES ('preserve')"},
			{"view", "CREATE VIEW sqlitebackup AS SELECT 'preserve' AS value"},
		} {
			t.Run(fmt.Sprintf("initialized=%v/%s", initialized, object.name), func(t *testing.T) {
				db := openMigrationTestDB(t)
				ctx := context.Background()
				if initialized {
					if err := Apply(ctx, db, SQLite); err != nil {
						t.Fatal(err)
					}
				}
				if _, err := db.ExecContext(ctx, object.ddl); err != nil {
					t.Fatal(err)
				}
				snapshot := func() string {
					t.Helper()
					var schema string
					if err := db.QueryRow("SELECT group_concat(sql,';') FROM (SELECT sql FROM sqlite_master ORDER BY name)").Scan(&schema); err != nil {
						t.Fatal(err)
					}
					return schema
				}
				before := snapshot()
				if err := Apply(ctx, db, SQLite); !errors.Is(err, ErrUnsupportedSourceFormat) {
					t.Fatalf("unknown sqlite-like %s opened: %v", object.name, err)
				}
				if after := snapshot(); after != before {
					t.Fatal("refusal changed source schema")
				}
				var value string
				name := "sqliteXforeign"
				if object.name == "view" {
					name = "sqlitebackup"
				}
				if err := db.QueryRow("SELECT value FROM " + name).Scan(&value); err != nil || value != "preserve" {
					t.Fatalf("source changed: %q, %v", value, err)
				}
			})
		}
	}
}
