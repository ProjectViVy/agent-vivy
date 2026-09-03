package codeface

import (
	"context"
	"path/filepath"
	"testing"

	"agent-vivy/internal/app/settings"
	"agent-vivy/internal/config"
	"agent-vivy/internal/storage/sqlite"
)

func TestPrepareSharesSettingsAndIsolatesRuntime(t *testing.T) {
	shared := t.TempDir()
	project := t.TempDir()
	cfg := config.Default()
	cfg.Storage.DataDir = shared
	cfg.Storage.SQLite.Path = filepath.Join(shared, "web.db")
	cfg.Runtime.SkillsRoot = filepath.Join(shared, "skills")
	cfg.Providers.BundleDir = filepath.Join("..", "..", "fixtures", "provider")

	first, err := Prepare(cfg, project)
	if err != nil {
		t.Fatalf("prepare first: %v", err)
	}
	second, err := Prepare(cfg, project)
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
	projectAbs, _ := filepath.Abs(project)
	if first.Config.Runtime.World != "local" || first.Config.Runtime.WorkspaceRoot != projectAbs {
		t.Fatalf("local world = %+v", first.Config.Runtime)
	}
}

func TestPreparedInstancesCanHoldLeasesConcurrently(t *testing.T) {
	shared := t.TempDir()
	project := t.TempDir()
	cfg := config.Default()
	cfg.Storage.DataDir = shared
	cfg.Storage.SQLite.Path = filepath.Join(shared, "web.db")
	cfg.Runtime.SkillsRoot = filepath.Join(shared, "skills")

	first, err := Prepare(cfg, project)
	if err != nil {
		t.Fatalf("prepare first: %v", err)
	}
	second, err := Prepare(cfg, project)
	if err != nil {
		t.Fatalf("prepare second: %v", err)
	}
	ctx := context.Background()
	firstDB, err := sqlite.Open(ctx, first.Config.Storage.SQLite.Path)
	if err != nil {
		t.Fatalf("open first: %v", err)
	}
	defer firstDB.Close()
	if err := firstDB.TakeOrganismLease(ctx); err != nil {
		t.Fatalf("lease first: %v", err)
	}
	secondDB, err := sqlite.Open(ctx, second.Config.Storage.SQLite.Path)
	if err != nil {
		t.Fatalf("open second: %v", err)
	}
	defer secondDB.Close()
	if err := secondDB.TakeOrganismLease(ctx); err != nil {
		t.Fatalf("lease second: %v", err)
	}
}
