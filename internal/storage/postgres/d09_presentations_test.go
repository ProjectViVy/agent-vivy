package postgres

import (
	"agent-vivy/internal/storage/conformance"
	"agent-vivy/internal/storage/migrations"
	"fmt"
	"testing"
	"testing/fstest"
)

func TestD09DeliverablePresentations(t *testing.T) {
	conformance.AssertDeliverablePresentations(t, continuitySlot(t))
}
func TestD09PresentationRollbackAndValidation(t *testing.T) {
	conformance.AssertPresentationRollbackAndValidation(t, continuitySlot(t))
}
func TestD09PresentationVisibility(t *testing.T) {
	conformance.AssertPresentationVisibility(t, continuitySlot(t))
}
func TestD09PresentationHistoryTransactions(t *testing.T) {
	conformance.AssertPresentationHistoryTransactions(t, continuitySlot(t))
}

func TestD09PresentationMigrationUpgradeAndRollbackPostgres(t *testing.T) {
	ctx, db, manifest := generationFixturePool(t)
	buildGenerationPrefix(t, ctx, db, manifest, 40)
	files := fstest.MapFS{}
	for _, dialect := range []migrations.Dialect{migrations.SQLite, migrations.Postgres} {
		for _, item := range manifest.Migrations(dialect) {
			source := item.SQL
			if item.Version == 41 {
				source += `SELECT * FROM missing_d09_presentation;`
			}
			files[fmt.Sprintf("%s/%03d_%s.sql", dialect, item.Version, item.Name)] = &fstest.MapFile{Data: []byte(source)}
		}
	}
	failing, err := migrations.Load(files)
	if err != nil {
		t.Fatal(err)
	}
	if err := migrations.ApplyManifest(ctx, db, migrations.Postgres, failing); err == nil {
		t.Fatal("injected 040 succeeded")
	}
	var count, head int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM information_schema.tables WHERE table_schema=current_schema() AND table_name='deliverable_presentations'`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("failed private table remained: %d %v", count, err)
	}
	if err := db.QueryRowContext(ctx, `SELECT MAX(version) FROM schema_migrations`).Scan(&head); err != nil || head != 40 {
		t.Fatalf("failed marker remained: %d %v", head, err)
	}
	if err := migrations.Apply(ctx, db, migrations.Postgres); err != nil {
		t.Fatal(err)
	}
	if err := migrations.Apply(ctx, db, migrations.Postgres); err != nil {
		t.Fatal(err)
	}
}
