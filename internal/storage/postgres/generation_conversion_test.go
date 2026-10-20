package postgres

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage/migrations"
)

func TestGenerationSchemaFreshPostgres(t *testing.T) {
	dsn := os.Getenv("VIVY_POSTGRES_TEST_DSN")
	if dsn == "" {
		t.Skip("VIVY_POSTGRES_TEST_DSN not set")
	}
	ctx := context.Background()
	schema := fmt.Sprintf("generation_fresh_%d", time.Now().UnixNano())
	admin, err := openPool(dsn, "")
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	defer admin.ExecContext(ctx, "DROP SCHEMA IF EXISTS "+schema+" CASCADE")
	backend, err := OpenSchema(ctx, dsn, schema)
	if err != nil {
		t.Fatal(err)
	}
	gen := domain.Generation{ID: "opaque-generation", SourceRef: "git:owned-ref", Phase: domain.GenerationBuilt, Recipe: domain.AssemblyRecipe{Face: "headless"}, CreatedAt: 5}
	if err := backend.CreateGeneration(ctx, gen); err != nil {
		t.Fatal(err)
	}
	if err := backend.Close(); err != nil {
		t.Fatal(err)
	}
	backend, err = OpenSchema(ctx, dsn, schema)
	if err != nil {
		t.Fatal(err)
	}
	got, err := backend.GetGeneration(ctx, gen.ID)
	if err != nil || got.ID != gen.ID || got.SourceRef != gen.SourceRef || got.Recipe.Face != "headless" {
		t.Fatalf("reopened generation: %#v: %v", got, err)
	}
	if _, err := backend.db.ExecContext(ctx, "ALTER TABLE generations ADD COLUMN artifact_sha256 TEXT"); err != nil {
		t.Fatal(err)
	}
	if err := migrations.Apply(ctx, backend.db.SQL, migrations.Postgres); !errors.Is(err, migrations.ErrUnsupportedSourceFormat) {
		t.Fatalf("retired column accepted: %v", err)
	}
	backend.Close()
}

func TestGenerationCopiedBaselineConversionPostgres(t *testing.T) {
	dsn := os.Getenv("VIVY_POSTGRES_TEST_DSN")
	if dsn == "" {
		t.Skip("VIVY_POSTGRES_TEST_DSN not set")
	}
	ctx := context.Background()
	sourceSchema := fmt.Sprintf("generation_source_%d", time.Now().UnixNano())
	targetSchema := sourceSchema + "_copy"
	admin, err := openPool(dsn, "")
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	defer admin.ExecContext(ctx, "DROP SCHEMA IF EXISTS "+targetSchema+" CASCADE")
	defer admin.ExecContext(ctx, "DROP SCHEMA IF EXISTS "+sourceSchema+" CASCADE")
	if _, err := admin.ExecContext(ctx, "CREATE SCHEMA "+sourceSchema+"; CREATE SCHEMA "+targetSchema); err != nil {
		t.Fatal(err)
	}
	source, err := openPool(dsn, sourceSchema)
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()
	manifest, err := migrations.Embedded()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := source.ExecContext(ctx, `CREATE TABLE schema_migrations(version BIGINT PRIMARY KEY,name TEXT NOT NULL,checksum TEXT NOT NULL,applied_at BIGINT NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	for _, migration := range manifest.Migrations(migrations.Postgres)[:35] {
		if _, err := source.ExecContext(ctx, migration.SQL); err != nil {
			t.Fatal(err)
		}
		if _, err := source.ExecContext(ctx, "INSERT INTO schema_migrations VALUES($1,$2,$3,1)", migration.Version, migration.Name, migration.Checksum); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := source.ExecContext(ctx, `
INSERT INTO generations(id,parent_id,artifact_sha256,source_ref,recipe_json,phase,created_at) VALUES
 ('legacy-parent','','old-body','git:parent','{}','built',1),
 ('legacy-candidate','legacy-parent','old-candidate','file:/custom/output/body.exe','{"face":"headless"}','evaluated',2);
INSERT INTO eval_runs(id,candidate_id,baseline_id,suite,verdict,journal_ref,created_at) VALUES('eval','legacy-candidate','legacy-parent','airgap.probe','better','eval/history',3);
INSERT INTO promotions(id,from_id,to_id,eval_id,actor,phase,applies_at,created_at) VALUES('promotion','legacy-parent','legacy-candidate','eval','human','accepted','next_launch',4);
INSERT INTO studio_events(type,object_id,created_at,payload) VALUES('generation.created','legacy-candidate',2,'{"id":"legacy-candidate"}');
`); err != nil {
		t.Fatal(err)
	}
	if err := migrations.Apply(ctx, source, migrations.Postgres); !errors.Is(err, migrations.ErrUpgradeRequired) {
		t.Fatalf("baseline runtime open = %v", err)
	}
	rows, err := source.QueryContext(ctx, "SELECT table_name FROM information_schema.tables WHERE table_schema=current_schema() AND table_type='BASE TABLE'")
	if err != nil {
		t.Fatal(err)
	}
	var tables []string
	for rows.Next() {
		var table string
		if err := rows.Scan(&table); err != nil {
			t.Fatal(err)
		}
		tables = append(tables, table)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	rows.Close()
	for _, table := range tables {
		if !schemaNamePattern.MatchString(table) {
			t.Fatal("unsafe fixture table")
		}
		if _, err := admin.ExecContext(ctx, "CREATE TABLE "+targetSchema+"."+table+" (LIKE "+sourceSchema+"."+table+" INCLUDING ALL); INSERT INTO "+targetSchema+"."+table+" SELECT * FROM "+sourceSchema+"."+table); err != nil {
			t.Fatal(err)
		}
	}
	target, err := openPool(dsn, targetSchema)
	if err != nil {
		t.Fatal(err)
	}
	defer target.Close()
	if _, err := target.ExecContext(ctx, "CREATE VIEW generation_drop_guard AS SELECT artifact_sha256 FROM generations"); err != nil {
		t.Fatal(err)
	}
	if err := migrations.ApplyManifest(ctx, target, migrations.Postgres, manifest); err == nil {
		t.Fatal("dependent-view failure unexpectedly succeeded")
	}
	var count int
	if err := target.QueryRowContext(ctx, "SELECT count(*) FROM schema_migrations WHERE version=37").Scan(&count); err != nil || count != 0 {
		t.Fatalf("failed037 metadata=%d: %v", count, err)
	}
	var oldArtifact string
	if err := target.QueryRowContext(ctx, "SELECT artifact_sha256 FROM generations WHERE id='legacy-candidate'").Scan(&oldArtifact); err != nil || oldArtifact != "old-candidate" {
		t.Fatalf("drop failure did not roll back: %q: %v", oldArtifact, err)
	}
	if err := migrations.Apply(ctx, target, migrations.Postgres); !errors.Is(err, migrations.ErrUnsupportedSourceFormat) {
		t.Fatalf("intermediate036 accepted: %v", err)
	}
	if _, err := target.ExecContext(ctx, "DROP VIEW generation_drop_guard"); err != nil {
		t.Fatal(err)
	}
	if err := migrations.ApplyManifest(ctx, target, migrations.Postgres, manifest); err != nil {
		t.Fatal(err)
	}
	if err := migrations.Apply(ctx, target, migrations.Postgres); err != nil {
		t.Fatalf("converted reopen: %v", err)
	}
	backend := &Backend{db: &DB{SQL: target}}
	gen, err := backend.GetGeneration(ctx, "legacy-candidate")
	if err != nil || gen.ParentID != "legacy-parent" || gen.SourceRef != "file:/custom/output/body.exe" || gen.Recipe.Face != "headless" || gen.Phase != domain.GenerationEvaluated || gen.CreatedAt != 2 {
		t.Fatalf("converted generation changed: %#v: %v", gen, err)
	}
	for table, want := range map[string]int{"generations": 2, "eval_runs": 1, "promotions": 1, "studio_events": 1} {
		if err := target.QueryRowContext(ctx, "SELECT count(*) FROM "+table).Scan(&count); err != nil || count != want {
			t.Fatalf("%s count=%d: %v", table, count, err)
		}
	}
	var candidate, baseline, verdict, actor, evalID, applies string
	if err := target.QueryRowContext(ctx, "SELECT e.candidate_id,e.baseline_id,e.verdict,p.actor,p.eval_id,p.applies_at FROM eval_runs e JOIN promotions p ON p.eval_id=e.id WHERE e.id='eval'").Scan(&candidate, &baseline, &verdict, &actor, &evalID, &applies); err != nil || candidate != "legacy-candidate" || baseline != "legacy-parent" || verdict != "better" || actor != "human" || evalID != "eval" || applies != "next_launch" {
		t.Fatalf("lifecycle history changed: %s/%s/%s/%s/%s/%s: %v", candidate, baseline, verdict, actor, evalID, applies, err)
	}
	if err := source.QueryRowContext(ctx, "SELECT count(*) FROM schema_migrations").Scan(&count); err != nil || count != 35 {
		t.Fatalf("source metadata=%d: %v", count, err)
	}
	if err := source.QueryRowContext(ctx, "SELECT artifact_sha256 FROM generations WHERE id='legacy-candidate'").Scan(&oldArtifact); err != nil || oldArtifact != "old-candidate" {
		t.Fatalf("source generation changed: %q: %v", oldArtifact, err)
	}
}
