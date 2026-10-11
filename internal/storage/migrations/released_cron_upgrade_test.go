package migrations

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"testing"
)

// These fixtures freeze the released bytes, rather than seeding an old
// database from the candidate's potentially edited migration history.
func releasedCronManifest(t *testing.T, head int) Manifest {
	t.Helper()
	full, err := Embedded()
	if err != nil {
		t.Fatal(err)
	}
	m := Manifest{byDialect: make(map[Dialect][]Migration)}
	for _, dialect := range []Dialect{SQLite, Postgres} {
		items := full.Migrations(dialect)[:head]
		if head >= 17 {
			data, err := os.ReadFile("testdata/released-017/" + string(dialect) + "/017_cron_repair.sql")
			if err != nil {
				t.Fatal(err)
			}
			items[16].SQL = string(data)
			items[16].Checksum = fmt.Sprintf("%x", sha256.Sum256(data))
		}
		m.byDialect[dialect] = items
	}
	return m
}

func TestReleasedCronRepairMigrationIsImmutable(t *testing.T) {
	full, err := Embedded()
	if err != nil {
		t.Fatal(err)
	}
	for dialect, checksum := range map[Dialect]string{
		SQLite:   "5deccacfe86210171818e5aca80dd51a3f02381f78f320325c4050b69619ec67",
		Postgres: "d8d06421d2906d09708123e80b8dd9262c9e6a57d4f91d5f681a0074bdfdb9bd",
	} {
		t.Run(string(dialect), func(t *testing.T) {
			released, err := os.ReadFile("testdata/released-017/" + string(dialect) + "/017_cron_repair.sql")
			if err != nil {
				t.Fatal(err)
			}
			// Git may check text fixtures out as CRLF on Windows. Verify the
			// frozen Git content independently, while retaining exact native
			// bytes and checksums for released databases on each platform.
			canonical := bytes.ReplaceAll(released, []byte("\r\n"), []byte("\n"))
			if got := fmt.Sprintf("%x", sha256.Sum256(canonical)); got != checksum {
				t.Fatalf("frozen released migration 017 Git checksum = %s, want %s", got, checksum)
			}
			migration := full.Migrations(dialect)[16]
			if migration.SQL != string(released) {
				t.Fatal("migration 017 bytes differ from the frozen release")
			}
			if got := fmt.Sprintf("%x", sha256.Sum256(released)); migration.Checksum != got {
				t.Fatalf("released migration 017 native checksum = %s, want %s", migration.Checksum, got)
			}
		})
	}
}

func TestSQLiteReleasedCronUpgradePreservesRows(t *testing.T) {
	for _, head := range []int{16, 17, 23, 35} {
		t.Run(fmt.Sprintf("head-%d", head), func(t *testing.T) {
			db := openMigrationTestDB(t)
			ctx := context.Background()
			if err := ApplyManifest(ctx, db, SQLite, releasedCronManifest(t, head)); err != nil {
				t.Fatalf("seed released schema: %v", err)
			}
			if _, err := db.ExecContext(ctx, `INSERT INTO cron_jobs
				(id, name, enabled, schedule_json, payload_json, created_at_ms, updated_at_ms)
				VALUES ('existing-cron', 'preserved', 1, '{}', '{}', 11, 12)`); err != nil {
				t.Fatal(err)
			}
			if err := Apply(ctx, db, SQLite); err != nil {
				t.Fatalf("upgrade released head %d: %v", head, err)
			}
			var name string
			var revision, created, updated int64
			if err := db.QueryRowContext(ctx, `SELECT name, revision, created_at_ms, updated_at_ms
				FROM cron_jobs WHERE id = 'existing-cron'`).Scan(&name, &revision, &created, &updated); err != nil {
				t.Fatal(err)
			}
			if name != "preserved" || revision != 1 || created != 11 || updated != 12 {
				t.Fatalf("upgraded cron = %q/%d/%d/%d", name, revision, created, updated)
			}
			if err := Apply(ctx, db, SQLite); err != nil {
				t.Fatalf("reapply upgraded schema: %v", err)
			}
		})
	}
}

func TestSQLiteReleasedCronRepairUpgrade(t *testing.T) {
	db := openMigrationTestDB(t)
	ctx := context.Background()
	if err := ApplyManifest(ctx, db, SQLite, releasedCronManifest(t, 16)); err != nil {
		t.Fatal(err)
	}
	// Historical migration 017 repairs deployments missing the cron table.
	if _, err := db.ExecContext(ctx, "DROP TABLE cron_jobs"); err != nil {
		t.Fatal(err)
	}
	if err := Apply(ctx, db, SQLite); err != nil {
		t.Fatalf("repair and additive revision upgrade: %v", err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO cron_jobs
		(id, name, enabled, schedule_json, payload_json, created_at_ms, updated_at_ms)
		VALUES ('repaired-cron', 'repaired', 1, '{}', '{}', 11, 12)`); err != nil {
		t.Fatal(err)
	}
	var revision int64
	if err := db.QueryRowContext(ctx, "SELECT revision FROM cron_jobs WHERE id = 'repaired-cron'").Scan(&revision); err != nil || revision != 1 {
		t.Fatalf("repaired cron revision = %d, error = %v", revision, err)
	}
}
