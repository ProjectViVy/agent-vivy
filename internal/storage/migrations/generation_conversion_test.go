package migrations

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
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
	if err := applyManifest(ctx, source, SQLite, base); err != nil {
		t.Fatal(err)
	}
	if _, err := source.Exec(generationHistoryFixture); err != nil {
		t.Fatal(err)
	}
	if err := Apply(ctx, source, SQLite); !errors.Is(err, ErrUnsupportedSourceFormat) {
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
	if err := applyManifest(ctx, target, SQLite, manifest); err != nil {
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
	if err := applyManifest(ctx, db, SQLite, base); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(generationHistoryFixture); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("CREATE INDEX generation_failure_guard ON generations(artifact_sha256)"); err != nil {
		t.Fatal(err)
	}
	if err := applyManifest(ctx, db, SQLite, manifest); err == nil {
		t.Fatal("target column-drop fault unexpectedly succeeded")
	}
	var count int
	if err := db.QueryRow("SELECT count(*) FROM schema_migrations WHERE version=38").Scan(&count); err != nil || count != 0 {
		t.Fatalf("failed target recorded038: %d %v", count, err)
	}
	present, err := sourceColumnExists(ctx, db, SQLite, "generations", "artifact_sha256")
	if err != nil || !present {
		t.Fatalf("drop-column failure did not roll back: %v %v", present, err)
	}
	// A known D12 three-column target may advance through the same runner;
	// the dependent index still prevents 038 and its marker from committing.
	if err := Apply(ctx, db, SQLite); err == nil {
		t.Fatal("guarded intermediate target unexpectedly advanced")
	}
	if _, err := db.Exec("DROP INDEX generation_failure_guard"); err != nil {
		t.Fatal(err)
	}
	if err := applyManifest(ctx, db, SQLite, manifest); err != nil {
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
	if err := applyManifest(ctx, db, SQLite, base); err != nil {
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

func TestGenerationLegacyMetadataRefusesDamageBeforeWrites(t *testing.T) {
	ctx := context.Background()
	manifest, err := Embedded()
	if err != nil {
		t.Fatal(err)
	}
	for _, head := range []int{35, 36} {
		for _, missing := range []string{"", "artifact_sha256", "source_ref"} {
			t.Run(fmt.Sprintf("head%d-missing-%s", head, missing), func(t *testing.T) {
				db := openMigrationTestDB(t)
				base := Manifest{byDialect: map[Dialect][]Migration{SQLite: manifest.Migrations(SQLite)[:head], Postgres: manifest.Migrations(Postgres)[:head]}}
				if err := applyManifest(ctx, db, SQLite, base); err != nil {
					t.Fatal(err)
				}
				if _, err := db.Exec(`ALTER TABLE schema_migrations ADD COLUMN checksum TEXT NOT NULL DEFAULT 'historical'`); err != nil {
					t.Fatal(err)
				}
				if missing != "" {
					if _, err := db.Exec("ALTER TABLE generations DROP COLUMN " + missing); err != nil {
						t.Fatal(err)
					}
				}
				want := ErrUpgradeRequired
				if missing != "" {
					want = ErrUnsupportedSourceFormat
				}
				for _, apply := range []func() error{func() error { return Apply(ctx, db, SQLite) }, func() error { return ApplyManifest(ctx, db, SQLite, manifest) }} {
					if err := apply(); !errors.Is(err, want) {
						t.Fatalf("legacy classification: got %v want %v", err, want)
					}
				}
				var actual int
				if err := db.QueryRow(`SELECT MAX(version) FROM schema_migrations`).Scan(&actual); err != nil || actual != head {
					t.Fatalf("refusal changed metadata: %d %v", actual, err)
				}
			})
		}
	}
}
func TestGenerationD12TargetPublicAdvancePreservesHistory(t *testing.T) {
	ctx := context.Background()
	manifest, err := Embedded()
	if err != nil {
		t.Fatal(err)
	}
	db := openMigrationTestDB(t)
	prefix := Manifest{byDialect: map[Dialect][]Migration{SQLite: manifest.Migrations(SQLite)[:36], Postgres: manifest.Migrations(Postgres)[:36]}}
	if err := applyManifest(ctx, db, SQLite, prefix); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(generationHistoryFixture); err != nil {
		t.Fatal(err)
	}
	if err := Apply(ctx, db, SQLite); err != nil {
		t.Fatal(err)
	}
	var head int
	if err := db.QueryRow(`SELECT MAX(version) FROM schema_migrations`).Scan(&head); err != nil || head != len(manifest.Migrations(SQLite)) {
		t.Fatalf("public advance: %d %v", head, err)
	}
	var ref string
	if err := db.QueryRow(`SELECT source_ref FROM generations WHERE id='legacy-candidate'`).Scan(&ref); err != nil || ref != "file:/custom/output/body.exe" {
		t.Fatalf("history changed: %q %v", ref, err)
	}
	for table, want := range map[string]int{"generations": 2, "eval_runs": 1, "promotions": 1, "studio_events": 1} {
		var got int
		if err := db.QueryRow("SELECT count(*) FROM " + table).Scan(&got); err != nil || got != want {
			t.Fatalf("%s lost history: %d %v", table, got, err)
		}
	}
	if err := Apply(ctx, db, SQLite); err != nil {
		t.Fatal(err)
	}
}
