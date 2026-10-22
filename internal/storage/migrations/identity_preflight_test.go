package migrations

import (
	"context"
	"errors"
	"fmt"
	"testing"
)

func TestIdentityPreflightRefusesUnconvertedStateBeforeWrites(t *testing.T) {
	ctx := context.Background()
	manifest, err := Embedded()
	if err != nil {
		t.Fatal(err)
	}
	for _, head := range []int{46, 47} {
		for _, owner := range []string{"observer", "channel"} {
			t.Run(fmt.Sprintf("%d-%s", head, owner), func(t *testing.T) {
				db := openMigrationTestDB(t)
				prefix := Manifest{byDialect: map[Dialect][]Migration{SQLite: manifest.Migrations(SQLite)[:head], Postgres: manifest.Migrations(Postgres)[:head]}}
				if err := applyManifest(ctx, db, SQLite, prefix); err != nil {
					t.Fatal(err)
				}
				if owner == "observer" {
					_, err = db.Exec(`INSERT INTO snapshots(key,value,version) VALUES('observer/run/old-uninterpreted-key',X'31',7)`)
				} else {
					_, err = db.Exec(`INSERT INTO sessions(id,title,created_at) VALUES('sess_ch_oldopaque','preserved history',1)`)
				}
				if err != nil {
					t.Fatal(err)
				}
				want := ErrUpgradeRequired
				if head == 47 {
					want = ErrUnsupportedSourceFormat
				}
				if err := Apply(ctx, db, SQLite); !errors.Is(err, want) {
					t.Fatalf("identity state admitted: %v", err)
				}
				var count int
				if err := db.QueryRow(`SELECT COUNT(*) FROM schema_migrations`).Scan(&count); err != nil || count != head {
					t.Fatalf("metadata changed: %d, %v", count, err)
				}
				if owner == "observer" {
					var value []byte
					var version int
					if err := db.QueryRow(`SELECT value,version FROM snapshots`).Scan(&value, &version); err != nil || string(value) != "1" || version != 7 {
						t.Fatalf("progress changed: %q %d %v", value, version, err)
					}
				} else {
					var title string
					if err := db.QueryRow(`SELECT title FROM sessions`).Scan(&title); err != nil || title != "preserved history" {
						t.Fatalf("history changed: %q %v", title, err)
					}
				}
			})
		}
	}
}

func TestIdentityPreflightReopensConvertedOwnerState(t *testing.T) {
	db := openMigrationTestDB(t)
	ctx := context.Background()
	if err := Apply(ctx, db, SQLite); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO sessions(id,title,created_at,channel_name,channel_chat_id,channel_topic_id)
		VALUES('sess_ch_preservedopaque','unchanged history',1,'fake','chat/raw','');
		INSERT INTO snapshots(key,value,version) VALUES('observer/run/v2/Zml4dHVyZQ/cnVu',X'31',7)`); err != nil {
		t.Fatal(err)
	}
	if err := Apply(ctx, db, SQLite); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO sessions(id,title,created_at,channel_name,channel_chat_id,channel_topic_id)
		VALUES('different-id','duplicate tuple',1,'fake','chat/raw','')`); err == nil {
		t.Fatal("schema accepted duplicate platform tuple")
	}
	if _, err := db.Exec(`INSERT INTO sessions(id,title,created_at,channel_name) VALUES('partial','partial tuple',1,'fake')`); err == nil {
		t.Fatal("schema accepted partial platform tuple")
	}
}

func TestChannelIdentityMigrationFailureRollsBack(t *testing.T) {
	ctx := context.Background()
	db := openMigrationTestDB(t)
	manifest, err := Embedded()
	if err != nil {
		t.Fatal(err)
	}
	prior := Manifest{byDialect: map[Dialect][]Migration{SQLite: manifest.Migrations(SQLite)[:46], Postgres: manifest.Migrations(Postgres)[:46]}}
	if err := applyManifest(ctx, db, SQLite, prior); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO sessions(id,title,created_at) VALUES('ordinary-session','unchanged',1)`); err != nil {
		t.Fatal(err)
	}
	broken := Manifest{byDialect: map[Dialect][]Migration{SQLite: append([]Migration(nil), manifest.Migrations(SQLite)...), Postgres: append([]Migration(nil), manifest.Migrations(Postgres)...)}}
	broken.byDialect[SQLite][46].SQL += "\nSELECT * FROM injected_missing_identity_table;"
	if err := ApplyManifest(ctx, db, SQLite, broken); err == nil {
		t.Fatal("injected migration failure succeeded")
	}
	var head int
	if err := db.QueryRow(`SELECT MAX(version) FROM schema_migrations`).Scan(&head); err != nil || head != 46 {
		t.Fatalf("migration marker changed: %d %v", head, err)
	}
	if exists, err := sourceColumnExists(ctx, db, SQLite, "sessions", "channel_name"); err != nil || exists {
		t.Fatalf("partial identity schema retained: %v %v", exists, err)
	}
	var title string
	if err := db.QueryRow(`SELECT title FROM sessions WHERE id='ordinary-session'`).Scan(&title); err != nil || title != "unchanged" {
		t.Fatalf("history changed: %q %v", title, err)
	}
}
