package sdk

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/eval"
)

func TestMinimalArtifactPhysicallyOmitsOptionalModules(t *testing.T) {
	root := t.TempDir()
	pack := func(name string) Artifact {
		t.Helper()
		artifact, err := Pack(context.Background(), packOptions{
			Recipe: filepath.Join("..", "..", "recipes", name+".vivy.yml"),
			Output: filepath.Join(root, name),
		})
		if err != nil {
			t.Fatal(err)
		}
		return artifact
	}
	defaultArtifact := pack("default")
	artifact := pack("minimal")
	for name, packed := range map[string]Artifact{"default": defaultArtifact, "minimal": artifact} {
		probe, err := eval.Launch(context.Background(), eval.LaunchRequest{
			Executable: packed.Binary,
			EvalRoot:   filepath.Join(root, name+"-eval"),
			Timeout:    20 * time.Second,
		})
		if err != nil {
			t.Fatal(err)
		}
		if probe.Verdict != domain.EvalMixed {
			stderr, _ := os.ReadFile(filepath.Join(probe.Layout.Root, "stderr.log"))
			t.Fatalf("%s packed Generation did not boot: verdict=%s stderr=%q", name, probe.Verdict, stderr)
		}
	}
	inspected, err := InspectArtifact(artifact.Directory)
	if err != nil {
		t.Fatal(err)
	}
	if len(inspected.Manifest.ConformanceResults) != 0 {
		t.Fatalf("minimal Inspect retained omitted Provider records: %#v", inspected.Manifest.ConformanceResults)
	}
	if len(inspected.Manifest.Catalogs) != 0 {
		t.Fatalf("minimal Inspect retained catalog projections: %#v", inspected.Manifest.Catalogs)
	}
	if inspected.Manifest.UI == nil || inspected.Manifest.UI.Root != "" || len(inspected.Manifest.UI.Extensions) != 0 || len(inspected.Manifest.UI.SourceHashes) != 0 || len(inspected.Manifest.UI.DependencyLockHashes) != 0 || len(inspected.Manifest.UI.AssetHashes) != 0 || len(inspected.Manifest.UI.Catalogs) != 0 {
		t.Fatalf("minimal Inspect retained optional UI/localization state: %#v", inspected.Manifest.UI)
	}

	binder, err := os.ReadFile(filepath.Join(artifact.Directory, "zz_assembly.go"))
	if err != nil {
		t.Fatal(err)
	}
	uiAssembly, err := os.ReadFile(filepath.Join(artifact.Directory, "ui-assembly.ts"))
	if err != nil {
		t.Fatal(err)
	}
	generated := string(binder) + "\n" + string(uiAssembly)
	omitted := map[string][]string{
		"vivy/action-host":       {"vivy/action-host", "NewActionHost"},
		"vivy/channel-host":      {"vivy/channel-host", "NewChannelHost"},
		"vivy/context-host":      {"vivy/context-host", "NewContextHost"},
		"vivy/context-source":    {"vivy/context-source", "NewContextSource"},
		"vivy/face-host":         {"vivy/face-host", "NewFaceHost"},
		"vivy/mcp-host":          {"vivy/mcp-host", "NewMCPHost", "NewMCPProvider"},
		"vivy/observer-host":     {"vivy/observer-host", "NewObserverHost"},
		"vivy/presentation-host": {"vivy/presentation-host", "NewPresentationHost"},
		"vivy/protected-tools":   {"vivy/protected-tools", "NewProtectedTools", "ProtectedToolProviders"},
		"vivy/provider-profiles": {"vivy/provider-profiles", "NewProviderProfiles", "ProviderProfiles"},
		"vivy/skill-host":        {"vivy/skill-host", "NewSkillHost"},
		"vivy/skill-source":      {"vivy/skill-source", "NewSkillSource"},
		"vivy/status-host":       {"vivy/status-host", "NewStatusHost"},
		"vivy/dingtalk":          {"vivy/dingtalk", "plugins/dingtalk"},
		"vivy/discord":           {"vivy/discord", "plugins/discord"},
		"vivy/evolution":         {"vivy/evolution", "vivy.evolution.sidebar", "plugins/vivy-evolution"},
		"vivy/feishu":            {"vivy/feishu", "plugins/feishu"},
		"vivy/memory":            {"vivy/memory", "vivy.memory.sidebar", "plugins/vivy-memory"},
		"vivy/notebook":          {"vivy/notebook", "vivy.notebook.sidebar", "plugins/vivy-notebook"},
		"vivy/persona":           {"vivy/persona", "vivy.persona.sidebar", "plugins/vivy-persona"},
		"vivy/qq":                {"vivy/qq", "plugins/qq"},
		"vivy/telegram":          {"vivy/telegram", "plugins/telegram"},
	}
	for moduleID, needles := range omitted {
		for _, needle := range needles {
			if strings.Contains(generated, needle) {
				t.Errorf("minimal generated Assembly retained %s surface %q", moduleID, needle)
			}
		}
		for _, selected := range inspected.Manifest.Modules {
			if selected.ID == moduleID {
				t.Errorf("minimal Manifest retained Module %s", moduleID)
			}
		}
		for _, edge := range inspected.Manifest.PortEdges {
			if edge.Provider == moduleID || edge.Consumer == moduleID {
				t.Errorf("minimal Manifest retained %s Port edge: %#v", moduleID, edge)
			}
		}
	}
}

func TestLegacyPluginSurfaceIsPhysicallyRemoved(t *testing.T) {
	root := filepath.Clean(filepath.Join("..", ".."))
	for _, path := range []string{"sdk/" + "plugin", "internal/" + "pluginhost", "internal/generated/" + "plugins", "internal/generated/" + "face"} {
		entries, err := os.ReadDir(filepath.Join(root, path))
		if err != nil && !os.IsNotExist(err) {
			t.Fatal(err)
		}
		if len(entries) > 0 {
			t.Errorf("legacy path contains files: %s", path)
		}
	}
	banned := []string{"agent-vivy/sdk/" + "plugin", "vivy.plugin/" + "v0", "type " + "Seam", "type " + "Plugin interface", "plugin." + "Plugin"}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		if entry.IsDir() {
			if rel == "docs" || rel == ".git" || rel == ".agents" || strings.Contains(rel, "node_modules") || strings.HasPrefix(rel, ".worktrees") {
				return filepath.SkipDir
			}
			return nil
		}
		ext := filepath.Ext(path)
		if ext != ".go" && ext != ".json" && ext != ".yaml" && ext != ".yml" && ext != ".mjs" {
			return nil
		}
		if filepath.Base(path) == "AGENTS.md" || strings.HasSuffix(path, "removal_conformance_test.go") {
			return nil
		}
		raw, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		for _, needle := range banned {
			if strings.Contains(string(raw), needle) {
				t.Errorf("%s contains removed surface %q", rel, needle)
			}
		}
		if filepath.Base(path) == "vivy-plugin.json" {
			t.Errorf("legacy descriptor remains: %s", rel)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
