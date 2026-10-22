package postgres

import (
	"agent-vivy/internal/storage/migrations"
	"context"
	"database/sql"
	"testing"
	"time"
)

// LIKE INCLUDING ALL allocates a new index name. Restore the released name in
// the synthetic copied schema so later immutable migrations see the same
// catalog as a real pg_dump/restore copy, rather than a fabricated lineage.
func restoreCopiedWorkflowIndexName(t *testing.T, ctx context.Context, db *sql.DB) {
	t.Helper()
	var name string
	if err := db.QueryRowContext(ctx, `SELECT indexname FROM pg_indexes WHERE schemaname=current_schema() AND tablename='workflow_definition_revisions' AND indexdef LIKE '%(workflow_id, artifact_digest, used_catalog_digest)'`).Scan(&name); err != nil {
		t.Fatal(err)
	}
	if !schemaNamePattern.MatchString(name) {
		t.Fatal("unsafe fixture index name")
	}
	if name != "workflow_definition_revisions_dedup_idx" {
		if _, err := db.ExecContext(ctx, `ALTER INDEX `+name+` RENAME TO workflow_definition_revisions_dedup_idx`); err != nil {
			t.Fatal(err)
		}
	}
}

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
