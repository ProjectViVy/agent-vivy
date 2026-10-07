package faceprocess

import (
	"os"
	"path/filepath"
	"testing"

	"agent-vivy/internal/app/settings"
	"agent-vivy/internal/config"
)

func TestPreparePrivateIsolation(t *testing.T) {
	shared := t.TempDir()
	cfg := config.Default()
	cfg.Storage.DataDir = shared
	cfg.Storage.SQLite.Path = filepath.Join(shared, "web.db")
	cfg.Runtime.SkillsRoot = filepath.Join(shared, "skills")

	first, err := PreparePrivate(cfg, NamespaceCodeInstances)
	if err != nil {
		t.Fatalf("prepare first: %v", err)
	}
	second, err := PreparePrivate(cfg, NamespaceCodeInstances)
	if err != nil {
		t.Fatalf("prepare second: %v", err)
	}
	if first.SharedSettingsPath != settings.Path(shared) || second.SharedSettingsPath != first.SharedSettingsPath {
		t.Fatalf("settings paths = %q / %q, want shared %q", first.SharedSettingsPath, second.SharedSettingsPath, settings.Path(shared))
	}
	if first.InstanceRoot == second.InstanceRoot || first.Config.Storage.SQLite.Path == second.Config.Storage.SQLite.Path {
		t.Fatalf("instances must be isolated: %+v / %+v", first, second)
	}
	if first.Config.Storage.Backend != "sqlite" || first.Config.DataDirectory() != first.InstanceRoot {
		t.Fatalf("private storage = %+v", first.Config.Storage)
	}
	if got := filepath.Dir(first.Config.Logging.Dir); got != first.InstanceRoot {
		t.Fatalf("log dir parent = %q, want instance root %q", got, first.InstanceRoot)
	}

	// The resident Journal must be untouched: the only writes under the
	// shared root are the instances directory itself.
	entries, err := os.ReadDir(shared)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.Name() != NamespaceCodeInstances && entry.Name() != "web.db" {
			t.Fatalf("unexpected write under shared root: %q", entry.Name())
		}
	}

	// The face namespace is a sibling root, never nested under code instances.
	face, err := PreparePrivate(cfg, NamespaceFaceInstances)
	if err != nil {
		t.Fatalf("prepare face namespace: %v", err)
	}
	if want := filepath.Join(shared, NamespaceFaceInstances); filepath.Dir(face.InstanceRoot) != want {
		t.Fatalf("face instance root parent = %q, want %q", filepath.Dir(face.InstanceRoot), want)
	}

	if _, err := PreparePrivate(cfg, "custom-namespace"); err == nil {
		t.Fatal("PreparePrivate must reject unknown namespaces")
	}
}
