package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
)

func openReleasedCronPostgresDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("VIVY_POSTGRES_TEST_DSN")
	if dsn == "" {
		t.Skip("VIVY_POSTGRES_TEST_DSN not set")
	}
	cfg, err := pgx.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	admin := stdlib.OpenDB(*cfg)
	schema := fmt.Sprintf("released_cron_%d", time.Now().UnixNano())
	if _, err := admin.ExecContext(context.Background(), "CREATE SCHEMA "+schema); err != nil {
		_ = admin.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = admin.ExecContext(context.Background(), "DROP SCHEMA "+schema+" CASCADE")
		_ = admin.Close()
	})
	cfg.RuntimeParams["search_path"] = schema
	db := stdlib.OpenDB(*cfg)
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func TestPostgresReleasedCronUpgradePreservesRows(t *testing.T) {
	for _, head := range []int{16, 17, 23, 35} {
		t.Run(fmt.Sprintf("head-%d", head), func(t *testing.T) {
			db := openReleasedCronPostgresDB(t)
			ctx := context.Background()
			if err := ApplyManifest(ctx, db, Postgres, releasedCronManifest(t, head)); err != nil {
				t.Fatalf("seed released schema: %v", err)
			}
			if _, err := db.ExecContext(ctx, `INSERT INTO cron_jobs
				(id, name, enabled, schedule_json, payload_json, created_at_ms, updated_at_ms)
				VALUES ('existing-cron', 'preserved', TRUE, '{}', '{}', 11, 12)`); err != nil {
				t.Fatal(err)
			}
			if err := Apply(ctx, db, Postgres); err != nil {
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
			if err := Apply(ctx, db, Postgres); err != nil {
				t.Fatalf("reapply upgraded schema: %v", err)
			}
		})
	}
}

func TestPostgresReleasedCronRepairUpgrade(t *testing.T) {
	db := openReleasedCronPostgresDB(t)
	ctx := context.Background()
	if err := ApplyManifest(ctx, db, Postgres, releasedCronManifest(t, 16)); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, "DROP TABLE cron_jobs"); err != nil {
		t.Fatal(err)
	}
	if err := Apply(ctx, db, Postgres); err != nil {
		t.Fatalf("repair and additive revision upgrade: %v", err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO cron_jobs
		(id, name, enabled, schedule_json, payload_json, created_at_ms, updated_at_ms)
		VALUES ('repaired-cron', 'repaired', TRUE, '{}', '{}', 11, 12)`); err != nil {
		t.Fatal(err)
	}
	var revision int64
	if err := db.QueryRowContext(ctx, "SELECT revision FROM cron_jobs WHERE id = 'repaired-cron'").Scan(&revision); err != nil || revision != 1 {
		t.Fatalf("repaired cron revision = %d, error = %v", revision, err)
	}
}
