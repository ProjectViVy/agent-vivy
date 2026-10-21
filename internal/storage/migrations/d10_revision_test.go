package migrations

import (
	"context"
	"database/sql"
	"testing"
)

func d10RevisionSource(t *testing.T) (*sql.DB, Manifest) {
	t.Helper()
	db := openMigrationTestDB(t)
	manifest, err := Embedded()
	if err != nil {
		t.Fatal(err)
	}
	prior := Manifest{byDialect: map[Dialect][]Migration{SQLite: manifest.Migrations(SQLite)[:41], Postgres: manifest.Migrations(Postgres)[:41]}}
	if err := applyManifest(context.Background(), db, SQLite, prior); err != nil {
		t.Fatal(err)
	}
	for _, item := range []struct{ id, action, before string }{{"create", "create", ""}, {"edit", "edit", "exact before"}, {"empty", "write_file", ""}, {"delete", "delete", "only old SKILL.md"}} {
		if _, err := db.Exec(`INSERT INTO skill_revisions(id,run_id,skill_name,action,target_path,payload,before_payload,base_hash,content_hash,preview,warnings_json,status,created_at) VALUES(?,'r','skill',?,'skill/SKILL.md',?,?,'retired','retired',?,'[]','pending',1)`, item.id, item.action, []byte(`{"Content":"actual after"}`), []byte(item.before), []byte("review")); err != nil {
			t.Fatal(err)
		}
	}
	for i := 1; i <= 2; i++ {
		if _, err := db.Exec(`INSERT INTO mask_definitions(id,name,description,body,revision,digest,created_at,updated_at,create_operation_id,create_request_digest) VALUES(?,?,?,?,?,'retired',1,2,?,'retired')`, "custom/"+string(rune('a'+i)), "actual name", "actual description", []byte("actual body"), i, "operation-"+string(rune('a'+i))); err != nil {
			t.Fatal(err)
		}
	}
	return db, manifest
}

func TestD10RevisionUpgradePreservesActualInputsAndFencesUnknownHistory(t *testing.T) {
	db, _ := d10RevisionSource(t)
	ctx := context.Background()
	if err := Apply(ctx, db, SQLite); err != nil {
		t.Fatal(err)
	}
	for _, item := range []struct {
		id            string
		valid, exists bool
		before        string
	}{{"create", true, false, ""}, {"edit", true, true, "exact before"}, {"empty", false, false, ""}, {"delete", false, false, "only old SKILL.md"}} {
		var exists sql.NullBool
		var before, payload []byte
		var status string
		if err := db.QueryRow(`SELECT before_exists,before_payload,payload,status FROM skill_revisions WHERE id=?`, item.id).Scan(&exists, &before, &payload, &status); err != nil {
			t.Fatal(err)
		}
		if exists.Valid != item.valid || exists.Bool != item.exists || string(before) != item.before || string(payload) != `{"Content":"actual after"}` || status != "pending" {
			t.Fatalf("conversion changed or armed history %s: %v %q %q %s", item.id, exists, before, payload, status)
		}
	}
	var name, description, body sql.NullString
	if err := db.QueryRow(`SELECT create_name,create_description,create_body FROM mask_definitions WHERE revision=1`).Scan(&name, &description, &body); err != nil || name.String != "actual name" || description.String != "actual description" || body.String != "actual body" {
		t.Fatalf("original create input lost: %v", err)
	}
	if err := db.QueryRow(`SELECT create_name,create_description,create_body FROM mask_definitions WHERE revision=2`).Scan(&name, &description, &body); err != nil || name.Valid || description.Valid || body.Valid {
		t.Fatalf("edited history fabricated original create: %v", err)
	}
	for table, columns := range map[string][]string{"skill_revisions": {"base_hash", "content_hash"}, "mask_definitions": {"digest", "create_request_digest"}} {
		for _, column := range columns {
			if exists, err := sourceColumnExists(ctx, db, SQLite, table, column); err != nil || exists {
				t.Fatalf("retired column %s.%s retained: %v", table, column, err)
			}
		}
	}
	if err := Apply(ctx, db, SQLite); err != nil {
		t.Fatalf("reopen: %v", err)
	}
}

func TestD10RevisionMigrationsRollbackAtEachOwnerBoundary(t *testing.T) {
	for _, version := range []int{42, 43} {
		t.Run(string(rune('A'+version-42)), func(t *testing.T) {
			db, manifest := d10RevisionSource(t)
			ctx := context.Background()
			items := manifest.Migrations(SQLite)
			if version == 43 {
				prior := Manifest{byDialect: map[Dialect][]Migration{SQLite: items[:42], Postgres: manifest.Migrations(Postgres)[:42]}}
				if err := applyManifest(ctx, db, SQLite, prior); err != nil {
					t.Fatal(err)
				}
			}
			items[version-1].SQL += `SELECT * FROM d10_injected_failure;`
			if err := ApplyManifest(ctx, db, SQLite, testManifest(items...)); err == nil {
				t.Fatal("injected failure committed")
			}
			var head int
			if err := db.QueryRow(`SELECT MAX(version) FROM schema_migrations`).Scan(&head); err != nil || head != version-1 {
				t.Fatalf("partial migration committed: %d %v", head, err)
			}
			if err := Apply(ctx, db, SQLite); err != nil {
				t.Fatal(err)
			}
		})
	}
}
