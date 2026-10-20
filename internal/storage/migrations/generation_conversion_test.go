package migrations

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

const generationHistoryFixture = `
INSERT INTO generations(id,parent_id,artifact_sha256,source_ref,recipe_json,phase,created_at) VALUES
 ('legacy-parent','','old-body','git:parent','{}','built',1),
 ('legacy-candidate','legacy-parent','old-candidate','file:/custom/output/body.exe','{"face":"headless"}','evaluated',2);
INSERT INTO eval_runs(id,candidate_id,baseline_id,suite,verdict,journal_ref,created_at) VALUES('eval','legacy-candidate','legacy-parent','airgap.probe','better','eval/history',3);
INSERT INTO promotions(id,from_id,to_id,eval_id,actor,phase,applies_at,created_at) VALUES('promotion','legacy-parent','legacy-candidate','eval','human','accepted','next_launch',4);
INSERT INTO studio_events(type,object_id,created_at,payload) VALUES('generation.created','legacy-candidate',2,'{"id":"legacy-candidate"}');
`

func TestGenerationSchemaFreshReopenAndRetiredColumnRefusal(t *testing.T) {
	ctx := context.Background()
	db := openMigrationTestDB(t)
	if err := Apply(ctx, db, SQLite); err != nil {
		t.Fatal(err)
	}
	present, err := sourceColumnExists(ctx, db, SQLite, "generations", "artifact_sha256")
	if err != nil || present {
		t.Fatalf("retired column present=%v: %v", present, err)
	}
	if err := Apply(ctx, db, SQLite); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("ALTER TABLE generations ADD COLUMN artifact_sha256 TEXT"); err != nil {
		t.Fatal(err)
	}
	if err := Apply(ctx, db, SQLite); !errors.Is(err, ErrUnsupportedSourceFormat) {
		t.Fatalf("retired column reopened: %v", err)
	}
}

func TestGenerationCopiedBaselineConversionPreservesHistory(t *testing.T) {
	ctx := context.Background()
	manifest, err := Embedded()
	if err != nil {
		t.Fatal(err)
	}
	base := Manifest{byDialect: map[Dialect][]Migration{SQLite: manifest.Migrations(SQLite)[:35], Postgres: manifest.Migrations(Postgres)[:35]}}
	sourcePath := filepath.Join(t.TempDir(), "source.db")
	source, err := sql.Open("sqlite", sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	if err := ApplyManifest(ctx, source, SQLite, base); err != nil {
		t.Fatal(err)
	}
	if _, err := source.Exec(generationHistoryFixture); err != nil {
		t.Fatal(err)
	}
	if err := Apply(ctx, source, SQLite); !errors.Is(err, ErrUpgradeRequired) {
		t.Fatalf("baseline runtime open = %v", err)
	}
	source.Close()
	before, err := os.ReadFile(sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	targetPath := filepath.Join(t.TempDir(), "target.db")
	if err := os.WriteFile(targetPath, before, 0600); err != nil {
		t.Fatal(err)
	}
	target, err := sql.Open("sqlite", targetPath)
	if err != nil {
		t.Fatal(err)
	}
	defer target.Close()
	if err := ApplyManifest(ctx, target, SQLite, manifest); err != nil {
		t.Fatal(err)
	}
	present, err := sourceColumnExists(ctx, target, SQLite, "generations", "artifact_sha256")
	if err != nil || present {
		t.Fatalf("converted retired column = %v: %v", present, err)
	}
	var parent, sourceRef, phase string
	var recipe []byte
	var created int
	if err := target.QueryRow("SELECT parent_id,source_ref,recipe_json,phase,created_at FROM generations WHERE id='legacy-candidate'").Scan(&parent, &sourceRef, &recipe, &phase, &created); err != nil || parent != "legacy-parent" || sourceRef != "file:/custom/output/body.exe" || string(recipe) != `{"face":"headless"}` || phase != "evaluated" || created != 2 {
		t.Fatalf("generation changed: %q/%q/%s/%q/%d: %v", parent, sourceRef, recipe, phase, created, err)
	}
	for table, want := range map[string]int{"generations": 2, "eval_runs": 1, "promotions": 1, "studio_events": 1} {
		var count int
		if err := target.QueryRow("SELECT count(*) FROM " + table).Scan(&count); err != nil || count != want {
			t.Fatalf("%s count=%d: %v", table, count, err)
		}
	}
	var candidate, baseline, verdict, actor, evalID, applies string
	if err := target.QueryRow("SELECT e.candidate_id,e.baseline_id,e.verdict,p.actor,p.eval_id,p.applies_at FROM eval_runs e JOIN promotions p ON p.eval_id=e.id WHERE e.id='eval'").Scan(&candidate, &baseline, &verdict, &actor, &evalID, &applies); err != nil || candidate != "legacy-candidate" || baseline != "legacy-parent" || verdict != "better" || actor != "human" || evalID != "eval" || applies != "next_launch" {
		t.Fatalf("lifecycle history changed: %s/%s/%s/%s/%s/%s: %v", candidate, baseline, verdict, actor, evalID, applies, err)
	}
	if err := Apply(ctx, target, SQLite); err != nil {
		t.Fatalf("converted reopen: %v", err)
	}
	after, err := os.ReadFile(sourcePath)
	if err != nil || string(before) != string(after) {
		t.Fatalf("source changed: %v", err)
	}
}

func TestGenerationCopyMigrationRollbackAndIntermediateRefusal(t *testing.T) {
	ctx := context.Background()
	manifest, err := Embedded()
	if err != nil {
		t.Fatal(err)
	}
	db := openMigrationTestDB(t)
	base := Manifest{byDialect: map[Dialect][]Migration{SQLite: manifest.Migrations(SQLite)[:35], Postgres: manifest.Migrations(Postgres)[:35]}}
	if err := ApplyManifest(ctx, db, SQLite, base); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(generationHistoryFixture); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("CREATE INDEX generation_failure_guard ON generations(artifact_sha256)"); err != nil {
		t.Fatal(err)
	}
	if err := ApplyManifest(ctx, db, SQLite, manifest); err == nil {
		t.Fatal("target column-drop fault unexpectedly succeeded")
	}
	var count int
	if err := db.QueryRow("SELECT count(*) FROM schema_migrations WHERE version=37").Scan(&count); err != nil || count != 0 {
		t.Fatalf("failed target recorded037: %d %v", count, err)
	}
	present, err := sourceColumnExists(ctx, db, SQLite, "generations", "artifact_sha256")
	if err != nil || !present {
		t.Fatalf("drop-column failure did not roll back: %v %v", present, err)
	}
	if err := Apply(ctx, db, SQLite); !errors.Is(err, ErrUnsupportedSourceFormat) {
		t.Fatalf("intermediate036 runtime accepted: %v", err)
	}
	if _, err := db.Exec("DROP INDEX generation_failure_guard"); err != nil {
		t.Fatal(err)
	}
	if err := ApplyManifest(ctx, db, SQLite, manifest); err != nil {
		t.Fatal(err)
	}
	if err := Apply(ctx, db, SQLite); err != nil {
		t.Fatal(err)
	}
}

func TestGenerationBaselineShapeDriftRefusesBeforeWrites(t *testing.T) {
	ctx := context.Background()
	manifest, err := Embedded()
	if err != nil {
		t.Fatal(err)
	}
	db := openMigrationTestDB(t)
	base := Manifest{byDialect: map[Dialect][]Migration{SQLite: manifest.Migrations(SQLite)[:35], Postgres: manifest.Migrations(Postgres)[:35]}}
	if err := ApplyManifest(ctx, db, SQLite, base); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("ALTER TABLE generations DROP COLUMN artifact_sha256"); err != nil {
		t.Fatal(err)
	}
	if err := Apply(ctx, db, SQLite); !errors.Is(err, ErrUnsupportedSourceFormat) {
		t.Fatalf("drifted baseline admitted: %v", err)
	}
	var count int
	if err := db.QueryRow("SELECT count(*) FROM schema_migrations").Scan(&count); err != nil || count != 35 {
		t.Fatalf("drift refusal wrote metadata: %d: %v", count, err)
	}
}
