package sdk

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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
