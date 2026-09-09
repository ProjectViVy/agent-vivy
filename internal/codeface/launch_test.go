package codeface

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"

	"agent-vivy/internal/app"
	"agent-vivy/internal/app/settings"
	"agent-vivy/internal/config"
	"agent-vivy/internal/storage/sqlite"
	"agent-vivy/sdk/plugin"
	"agent-vivy/sdk/tui/live"
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
	projectAbs, err := filepath.EvalSymlinks(project)
	if err != nil {
		t.Fatal(err)
	}
	projectAbs, _ = filepath.Abs(projectAbs)
	if first.Config.Runtime.World != "local" || first.Config.Runtime.WorkspaceRoot != projectAbs {
		t.Fatalf("local world = %+v", first.Config.Runtime)
	}
}

// The code launcher must hydrate from the shared settings, not the private
// runtime's conflicting locale. The face test covers the remaining view hop.
func TestCodeLaunchSettingsLocaleUsesSharedPath(t *testing.T) {
	shared := t.TempDir()
	cfg := config.Default()
	cfg.Storage.DataDir = shared
	cfg.Storage.SQLite.Path = filepath.Join(shared, "web.db")
	cfg.Runtime.SkillsRoot = filepath.Join(shared, "skills")
	cfg.Providers.BundleDir = filepath.Join("..", "..", "fixtures", "provider")
	prepared, err := Prepare(cfg, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := settings.Save(prepared.SharedSettingsPath, settings.Settings{Locale: "zh"}); err != nil {
		t.Fatal(err)
	}
	if _, err := settings.Save(settings.Path(prepared.InstanceRoot), settings.Settings{Locale: "en"}); err != nil {
		t.Fatal(err)
	}
	result, err := app.RunFaceWithAppOptions(context.Background(), prepared.Config,
		func(plugin.FaceOptions) plugin.Face { return &localeCheckingFace{t: t} },
		plugin.FaceOptions{Out: io.Discard, Err: io.Discard}, codeAppOptions(prepared)...)
	if err != nil || result.Status != "completed" {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}

type localeCheckingFace struct{ t *testing.T }

func (*localeCheckingFace) Kind() string { return "tui" }

func (f *localeCheckingFace) Run(ctx context.Context, env plugin.FaceEnv) (plugin.FaceResult, error) {
	controller, err := live.New(ctx, localeFaceTransport{env}, live.Options{})
	if err != nil {
		return plugin.FaceResult{}, err
	}
	defer controller.Close()
	if controller.Locale() != "zh" || controller.Meta().Error != "" {
		f.t.Errorf("locale=%q warning=%q", controller.Locale(), controller.Meta().Error)
	}
	return plugin.FaceResult{Status: "completed"}, nil
}

type localeFaceTransport struct{ plugin.FaceEnv }

func (t localeFaceTransport) OnNotify(fn func(string, json.RawMessage)) { t.OnEvent(fn) }

func TestPrepareCanonicalizesProjectReachedThroughLinkedParent(t *testing.T) {
	realParent := t.TempDir()
	realProject := filepath.Join(realParent, "project")
	if err := os.Mkdir(realProject, 0o700); err != nil {
		t.Fatal(err)
	}
	linkBase := t.TempDir()
	linkedParent := filepath.Join(linkBase, "linked-parent")
	if err := os.Symlink(realParent, linkedParent); err != nil {
		t.Skipf("directory symlink creation unavailable: %v", err)
	}

	prepared, err := Prepare(config.Default(), filepath.Join(linkedParent, "project"))
	if err != nil {
		t.Fatalf("prepare linked parent: %v", err)
	}
	want, err := filepath.EvalSymlinks(realProject)
	if err != nil {
		t.Fatal(err)
	}
	want, _ = filepath.Abs(want)
	if prepared.Config.Runtime.WorkspaceRoot != want {
		t.Fatalf("project root = %q, want canonical %q", prepared.Config.Runtime.WorkspaceRoot, want)
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
