package sdk

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
	"time"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/eval"
)

// This gate must run in a fresh headless checkout without installing the UI.
// It exercises the production packer and executable, not a CI-only assembler.
func TestMinimalKernelArtifactBootAndInspect(t *testing.T) {
	root := t.TempDir()
	artifact, err := Pack(context.Background(), packOptions{
		Recipe: filepath.Join("..", "..", "recipes", "minimal.vivy.yml"),
		Output: filepath.Join(root, "minimal"),
	})
	if err != nil {
		t.Fatal(err)
	}
	inspected, err := InspectArtifact(artifact.Directory)
	if err != nil {
		t.Fatal(err)
	}
	if inspected.Manifest.GenerationID != artifact.Manifest.GenerationID {
		t.Fatal("Inspect changed the sealed Generation identity")
	}
	var modules []string
	for _, module := range inspected.Manifest.Modules {
		modules = append(modules, module.ID)
	}
	sort.Strings(modules)
	want := []string{"vivy/checkpoint", "vivy/credential", "vivy/loop", "vivy/model", "vivy/protected-tools", "vivy/provider-profiles", "vivy/sandbox", "vivy/storage", "vivy/tool-host"}
	if !reflect.DeepEqual(modules, want) {
		t.Fatalf("minimal modules = %v, want %v", modules, want)
	}
	ui := inspected.Manifest.UI
	if ui == nil || ui.Root != "" || len(ui.Extensions) != 0 {
		t.Fatal("minimal retained optional UI")
	}
	assets, err := os.ReadDir(filepath.Join(artifact.Directory, "ui", "dist"))
	if err != nil || len(assets) != 0 {
		t.Fatalf("minimal embeds frontend assets: %v, %v", assets, err)
	}
	probe, err := eval.Launch(context.Background(), eval.LaunchRequest{
		Executable: artifact.Binary, EvalRoot: filepath.Join(root, "eval"), Timeout: 20 * time.Second,
	})
	if err != nil || probe.Verdict != domain.EvalMixed {
		t.Fatalf("minimal did not answer the isolated health probe: %v, %v", probe.Verdict, err)
	}
}

// An omitted replacement root still uses the base Web shell in lite.
func TestLiteArtifactRetainsBaseWebShell(t *testing.T) {
	artifact, err := Pack(context.Background(), packOptions{
		Recipe: filepath.Join("..", "..", "recipes", "lite.vivy.yml"),
		Output: filepath.Join(t.TempDir(), "lite"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := InspectArtifact(artifact.Directory); err != nil {
		t.Fatal(err)
	}
	index, err := os.ReadFile(filepath.Join(artifact.Directory, "ui", "dist", "index.html"))
	if err != nil || len(index) == 0 {
		t.Fatalf("lite lost the base Web shell: %v", err)
	}
	binary, err := os.ReadFile(artifact.Binary)
	if err != nil || !bytes.Contains(binary, index) {
		t.Fatalf("lite executable does not embed its published Web shell: %v", err)
	}
}
