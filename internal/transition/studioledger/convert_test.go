package studioledger

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agent-vivy/internal/studiocore"
)

const legacySchema = `
CREATE TABLE IF NOT EXISTS worktrees (
	id TEXT PRIMARY KEY,
	path TEXT NOT NULL,
	kind TEXT NOT NULL,
	dirty INTEGER NOT NULL DEFAULT 0,
	created_at INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS generations (
	id TEXT PRIMARY KEY,
	parent_id TEXT NOT NULL DEFAULT '',
artifact_sha256 TEXT NOT NULL,
	source_ref TEXT NOT NULL,
	recipe_json TEXT NOT NULL DEFAULT '{}',
	phase TEXT NOT NULL,
	created_at INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS eval_runs (
	id TEXT PRIMARY KEY,
	candidate_id TEXT NOT NULL,
	baseline_id TEXT NOT NULL DEFAULT '',
	suite TEXT NOT NULL,
	verdict TEXT NOT NULL,
	journal_ref TEXT NOT NULL DEFAULT '',
	created_at INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS releases (
	id TEXT PRIMARY KEY,
	generation_id TEXT NOT NULL,
	eval_id TEXT NOT NULL DEFAULT '',
	actor TEXT NOT NULL,
	phase TEXT NOT NULL,
	created_at INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS installs (
	id TEXT PRIMARY KEY,
	release_id TEXT NOT NULL,
	target TEXT NOT NULL,
	phase TEXT NOT NULL,
	created_at INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS studio_events (
	seq INTEGER PRIMARY KEY AUTOINCREMENT,
	type TEXT NOT NULL,
	object_id TEXT NOT NULL,
	created_at INTEGER NOT NULL,
	payload TEXT NOT NULL DEFAULT '{}'
);
`

func legacyFixture(t *testing.T) (string, string) {
	t.Helper()
	home := t.TempDir()
	source := filepath.Join(home, "studio.db")
	artifact := filepath.Join(t.TempDir(), "custom-output", "vivy.exe")
	if err := os.MkdirAll(filepath.Dir(artifact), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(artifact, []byte("candidate bytes"), 0700); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", source)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(legacySchema); err != nil {
		t.Fatal(err)
	}
	for _, q := range []string{
		`INSERT INTO worktrees VALUES('wt','source','kernel',0,1)`,
		`INSERT INTO generations VALUES('gen-ref','','old','git:owned-ref','{}','built',1)`,
		`INSERT INTO eval_runs VALUES('evl','gen-file','gen-ref','airgap.probe','better','evidence',3)`,
		`INSERT INTO eval_runs VALUES('evl-old','gen-ref','','airgap.probe','mixed','evidence-old',1)`,
		`INSERT INTO releases VALUES('rel-old','gen-ref','evl-old','human','accepted',1)`,
		`INSERT INTO releases VALUES('rel','gen-file','evl','human','accepted',4)`,
		`INSERT INTO studio_events(seq,type,object_id,created_at,payload) VALUES(9,'generation.created','gen-file',2,'{}')`,
		`UPDATE sqlite_sequence SET seq=12 WHERE name='studio_events'`,
	} {
		if _, err := db.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.Exec(`INSERT INTO installs VALUES('ins','rel',?,'current',5)`, t.TempDir()); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO generations VALUES('gen-file','gen-ref','old',?,'{"face":"headless"}','released',2)`, "file:"+filepath.ToSlash(artifact)); err != nil {
		t.Fatal(err)
	}
	snapshot := filepath.Join(home, "rollback", "ins")
	if err := os.MkdirAll(snapshot, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(snapshot, "vivy.exe"), []byte("previous bytes"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(snapshot, "install.json"), []byte(`{"release_id":"rel-old","generation_id":"gen-ref","artifact_sha256":"old","installed_at":1}`), 0600); err != nil {
		t.Fatal(err)
	}
	return source, artifact
}

func TestConvertCopiesActualArtifactAndPreservesHistory(t *testing.T) {
	ctx := context.Background()
	source, artifact := legacyFixture(t)
	before, _ := os.ReadFile(source)
	target := filepath.Join(t.TempDir(), "converted")
	result, err := Convert(ctx, Options{SourceLedger: source, TargetHome: target})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Artifacts) != 1 || result.Artifacts[artifact] == "" {
		t.Fatalf("actual artifact mapping = %#v", result.Artifacts)
	}
	ledger, err := studiocore.OpenLedger(ctx, filepath.Join(target, "studio.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer ledger.Close()
	gen, err := ledger.GetGeneration(ctx, "gen-file")
	if err != nil {
		t.Fatal(err)
	}
	if gen.ParentID != "gen-ref" || gen.SourceRef != "file:"+filepath.ToSlash(result.Artifacts[artifact]) || gen.Recipe.Face != "headless" {
		t.Fatalf("converted generation = %#v", gen)
	}
	ref, err := ledger.GetGeneration(ctx, "gen-ref")
	if err != nil || ref.SourceRef != "git:owned-ref" {
		t.Fatalf("selection ref = %#v, %v", ref, err)
	}
	for _, file := range []struct{ path, want string }{{result.Artifacts[artifact], "candidate bytes"}, {filepath.Join(target, "rollback", "ins", "vivy.exe"), "previous bytes"}} {
		raw, err := os.ReadFile(file.path)
		if err != nil || string(raw) != file.want {
			t.Fatalf("copied %s = %q, %v", file.path, raw, err)
		}
	}
	snapshot, err := os.ReadFile(filepath.Join(target, "rollback", "ins", "install.json"))
	if err != nil || strings.Contains(string(snapshot), "artifact_sha256") || !strings.Contains(string(snapshot), "rel-old") {
		t.Fatalf("snapshot identity = %s, %v", snapshot, err)
	}
	db, err := sql.Open("sqlite", filepath.Join(target, "studio.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for table, want := range map[string]int{"worktrees": 1, "generations": 2, "eval_runs": 2, "releases": 2, "installs": 1, "studio_events": 1} {
		var count int
		if err := db.QueryRow("SELECT count(*) FROM " + table).Scan(&count); err != nil || count != want {
			t.Fatalf("%s count %d, %v", table, count, err)
		}
	}
	var seq int
	if err := db.QueryRow("SELECT seq FROM sqlite_sequence WHERE name='studio_events'").Scan(&seq); err != nil || seq != 12 {
		t.Fatalf("event allocator = %d, %v", seq, err)
	}
	installed, err := ledger.GetInstall(ctx, "ins")
	if err != nil {
		t.Fatal(err)
	}
	service, err := studiocore.NewService(ctx, studiocore.Options{Worktree: t.TempDir(), LedgerPath: filepath.Join(target, "studio.db"), InstallTarget: installed.Target})
	if err != nil {
		t.Fatal(err)
	}
	restored, err := service.Rollback(ctx, installed.Target)
	if err != nil {
		t.Fatal(err)
	}
	if restored.ReleaseID != "rel-old" {
		t.Fatalf("restored release = %#v", restored)
	}
	body, err := os.ReadFile(filepath.Join(installed.Target, "vivy.exe"))
	if err != nil || string(body) != "previous bytes" {
		t.Fatalf("restored body %q: %v", body, err)
	}
	if err := service.Close(); err != nil {
		t.Fatal(err)
	}
	after, _ := os.ReadFile(source)
	if string(before) != string(after) {
		t.Fatal("source ledger changed")
	}
	if _, err := Convert(ctx, Options{SourceLedger: source, TargetHome: target}); err == nil {
		t.Fatal("overwrote completed target")
	}
}

func TestConvertMissingArtifactAndInjectedFailuresLeaveSource(t *testing.T) {
	for _, phase := range []string{"missing", "copy", "sql", "verify", "publish"} {
		t.Run(phase, func(t *testing.T) {
			source, artifact := legacyFixture(t)
			before, _ := os.ReadFile(source)
			if phase == "missing" {
				if err := os.Remove(artifact); err != nil {
					t.Fatal(err)
				}
			}
			parent := t.TempDir()
			target := filepath.Join(parent, "converted")
			sentinel := filepath.Join(parent, "keep")
			os.WriteFile(sentinel, []byte("keep"), 0600)
			_, err := convert(context.Background(), Options{SourceLedger: source, TargetHome: target}, func(at string) error {
				if at == phase {
					return errors.New("injected " + phase)
				}
				return nil
			})
			if err == nil {
				t.Fatal("unexpected conversion success")
			}
			if _, err := os.Stat(target); !os.IsNotExist(err) {
				t.Fatal("partial target published")
			}
			entries, _ := os.ReadDir(parent)
			if len(entries) != 1 || entries[0].Name() != "keep" {
				t.Fatalf("cleanup escaped owned staging: %#v", entries)
			}
			after, _ := os.ReadFile(source)
			if string(before) != string(after) {
				t.Fatal("source ledger changed")
			}
		})
	}
}

func TestConvertRefusesUnknownShapeBeforeTargetAndAllowsRetry(t *testing.T) {
	source, artifact := legacyFixture(t)
	parent := t.TempDir()
	target := filepath.Join(parent, "converted")
	db, err := sql.Open("sqlite", source)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("PRAGMA user_version=9"); err != nil {
		t.Fatal(err)
	}
	db.Close()
	if _, err := Convert(context.Background(), Options{SourceLedger: source, TargetHome: target}); !errors.Is(err, studiocore.ErrUnsupportedSourceFormat) {
		t.Fatalf("unknown format = %v", err)
	}
	entries, _ := os.ReadDir(parent)
	if len(entries) != 0 {
		t.Fatal("unknown source created target files")
	}
	db, _ = sql.Open("sqlite", source)
	db.Exec("PRAGMA user_version=0")
	db.Close()
	before, _ := os.ReadFile(source)
	if _, err := convert(context.Background(), Options{SourceLedger: source, TargetHome: target}, func(at string) error {
		if at == "verify" {
			return errors.New("verification injection")
		}
		return nil
	}); err == nil {
		t.Fatal("injection succeeded")
	}
	result, err := Convert(context.Background(), Options{SourceLedger: source, TargetHome: target})
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(result.Artifacts[artifact]) != "artifact-000001.exe" {
		t.Fatalf("retry allocation = %#v", result)
	}
	after, _ := os.ReadFile(source)
	if string(before) != string(after) {
		t.Fatal("retry changed source")
	}
}

func TestConvertRefusesAliasedSourceHomeTarget(t *testing.T) {
	source, _ := legacyFixture(t)
	alias := filepath.Join(t.TempDir(), "alias")
	if err := os.Symlink(filepath.Dir(source), alias); err != nil {
		t.Skipf("symlink creation unavailable: %v", err)
	}
	if _, err := Convert(context.Background(), Options{SourceLedger: source, TargetHome: filepath.Join(alias, "converted")}); err == nil {
		t.Fatal("aliased source-home target accepted")
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(source), "converted")); !os.IsNotExist(err) {
		t.Fatal("source home changed")
	}
}
