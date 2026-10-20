package postgres

import (
	"agent-vivy/internal/storage/migrations"
	"context"
	"database/sql"
	"testing"
	"time"
)

// finishMigrationFixture prepares a copied synthetic business-schema fixture;
// it is excluded from runtime builds and supplies no live upgrade entry point.
func finishMigrationFixture(t *testing.T, ctx context.Context, db *sql.DB, manifest migrations.Manifest, after int64) {
	t.Helper()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	for _, item := range manifest.Migrations(migrations.Postgres) {
		if item.Version <= after {
			continue
		}
		if _, err := tx.ExecContext(ctx, item.SQL); err != nil {
			t.Fatal(err)
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO schema_migrations(version,name,applied_at) VALUES($1,$2,$3)`, item.Version, item.Name, time.Now().UnixMilli()); err != nil {
			t.Fatal(err)
		}
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
}
