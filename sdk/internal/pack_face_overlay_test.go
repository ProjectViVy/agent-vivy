package sdk

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	assemblyv1 "agent-vivy/sdk/internal/assembly"
	"agent-vivy/sdk/port"
	"gopkg.in/yaml.v3"
)

// A recipe that exclusively selects the restricted ACP face must compile an
// artifact whose Go overlay swaps cmd/vivy/tui.go for an unavailable stub:
// the selected generation's whole process surface is the stdio protocol, so
// `vivy tui`/`vivy run` never link (ACP-STDIO-FACE §288). Every other recipe
// must keep the real TUI file and the full dependency closure.

const acpTestSourceGoMod = `module projectvivy/acp

go 1.26.4

require agent-vivy v0.0.0

replace agent-vivy => ../..
`

const acpTestSourcePlugin = `package acp

import (
	"context"

	"agent-vivy/sdk/module"
	faceport "agent-vivy/sdk/port/face"
)

type acpModule struct{}

func New() module.Module { return acpModule{} }

func (acpModule) Descriptor() module.Descriptor { return module.Descriptor{} }

func (acpModule) Construct(context.Context, module.Host) (module.Instance, error) {
	return acpInstance{}, nil
}

type acpInstance struct{}

func (acpInstance) Start(context.Context) error { return nil }
func (acpInstance) Ready(context.Context) error { return nil }
func (acpInstance) Stop(context.Context) error  { return nil }
func (acpInstance) Close(context.Context) error { return nil }

type provider struct{}

func NewProvider() faceport.FaceProvider { return provider{} }

func (provider) Definition() faceport.Definition {
	return faceport.Definition{ID: "projectvivy.acp", Kind: "acp"}
}

func (provider) Construct(_ context.Context, _ faceport.Host) (faceport.Instance, error) {
	return boundFace{}, nil
}

type boundFace struct{}

func (boundFace) Run(context.Context, faceport.Options) (faceport.Result, error) {
	return faceport.Result{Status: "completed"}, nil
}
`

const acpTestModuleYAML = `apiVersion: vivy.module/v1
module:
  id: projectvivy/acp
  version: 0.0.0
source:
  ref: file:pack-acp
  sha256: @DIGEST@
provides:
  - port: std/face@v1
    id: projectvivy.acp
requires:
  - port: core/face-host@v1
    provider: vivy/face-host
requestedGrants:
  - rpc.client
lifecycle:
  scope: generation
`

const acpTestRecipeYAML = `apiVersion: vivy.generation/v1
profile: acp-test
modules: [vivy/loop, vivy/model, vivy/tool-host, vivy/storage, vivy/checkpoint, vivy/credential, vivy/sandbox, vivy/face-host, projectvivy/acp]
exclusive:
  std/face@v1: projectvivy/acp
grantApprovals:
  - module: projectvivy/acp
    name: rpc.client
sources:
  projectvivy/acp:
    ref: file:pack-acp
    sha256: @DIGEST@
`

func writePackACPFaceSource(t *testing.T, repoRoot string) (string, string) {
	t.Helper()
	root := temporaryRepositoryModule(t, repoRoot, "pack-acp-")
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte(acpTestSourceGoMod), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "plugin.go"), []byte(acpTestSourcePlugin), 0o600); err != nil {
		t.Fatal(err)
	}
	// The declared digest is zeroed inside the tree hash, so write the YAML
	// with the zero digest first, then bind the real content address.
	zero := strings.Repeat("0", 64)
	if err := os.WriteFile(filepath.Join(root, "vivy-module.yaml"), []byte(strings.Replace(acpTestModuleYAML, "@DIGEST@", zero, 1)), 0o600); err != nil {
		t.Fatal(err)
	}
	digest, err := assemblyv1.HashSourceTree(root, "")
	if err != nil {
		t.Fatal(err)
	}
	yaml := strings.Replace(acpTestModuleYAML, "@DIGEST@", digest, 1)
	if err := os.WriteFile(filepath.Join(root, "vivy-module.yaml"), []byte(yaml), 0o600); err != nil {
		t.Fatal(err)
	}
	return root, digest
}

// compileACPPlan reproduces the Pack compile leg for the ACP recipe so the
// test can inspect the real overlay and modfile before their cleanup.
func compileACPPlan(t *testing.T, repoRoot string, recipe assemblyv1.Recipe, buildSources []string) (assemblyv1.AssemblyPlan, []byte) {
	t.Helper()
	records, err := sourceRecords(repoRoot, buildSources, recipe.Sources)
	if err != nil {
		t.Fatal(err)
	}
	catalog, err := assemblyv1.NewSourceCatalog(records)
	if err != nil {
		t.Fatal(err)
	}
	compiler := assemblyv1.Compiler{
		Ports:              port.PublicCatalog(),
		Sources:            catalog,
		PortEvidence:       assemblyv1.SupportedPortEvidence(),
		ConformanceResults: assemblyv1.SupportedPortConformance(),
	}
	plan, err := compiler.Compile(context.Background(), recipe)
	if err != nil {
		t.Fatalf("compile ACP recipe: %v", err)
	}
	runtimeSource, err := assemblyv1.GenerateRuntimeAssembly(plan, "assembly")
	if err != nil {
		t.Fatal(err)
	}
	return plan, runtimeSource
}

func TestPackACPSelectedOverlay(t *testing.T) {
	repoRoot, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	source, digest := writePackACPFaceSource(t, repoRoot)
	snapshotRoot, buildSources, err := snapshotSourceDirs(repoRoot, []string{source})
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(snapshotRoot)

	var recipe assemblyv1.Recipe
	recipeRaw := strings.Replace(acpTestRecipeYAML, "@DIGEST@", digest, 1)
	if err := yaml.Unmarshal([]byte(recipeRaw), &recipe); err != nil {
		t.Fatal(err)
	}
	plan, runtimeSource := compileACPPlan(t, repoRoot, recipe, buildSources)
	if !strings.Contains(string(runtimeSource), `"projectvivy.acp"`) ||
		!strings.Contains(string(runtimeSource), "acp.NewProvider()") {
		t.Fatalf("generated assembly does not seal the ACP face:\n%s", runtimeSource)
	}

	// The selected face stages the pack-only TUI stub next to the generated
	// assembly and adds the tui.go replacement to the overlay.
	overlayDir := t.TempDir()
	overlayFile, err := stagePackOverlay(repoRoot, overlayDir, &recipe, plan, runtimeSource)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(overlayFile)
	if err != nil {
		t.Fatal(err)
	}
	var overlay struct {
		Replace map[string]string `json:"Replace"`
	}
	if err := json.Unmarshal(raw, &overlay); err != nil {
		t.Fatal(err)
	}
	if len(overlay.Replace) != 2 {
		t.Fatalf("selected overlay Replace = %v, want zz_default.go + tui.go", overlay.Replace)
	}
	tuiReplacement, ok := overlay.Replace[filepath.Join(repoRoot, "cmd", "vivy", "tui.go")]
	if !ok {
		t.Fatalf("overlay lacks the tui.go replacement: %v", overlay.Replace)
	}
	stub, err := os.ReadFile(tuiReplacement)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(stub), "func runTUI(args []string) int") ||
		!strings.Contains(string(stub), "unavailable") ||
		strings.Contains(string(stub), "agent-vivy/") {
		t.Fatalf("pack-only tui stub is wrong:\n%s", stub)
	}

	// go list -deps over the real overlay+modfile proves the selected build's
	// implementation closure before cleanup.
	modfile, err := prepareBuildModfile(repoRoot, overlayDir, buildSources)
	if err != nil {
		t.Fatal(err)
	}
	list := exec.Command("go", "list", "-mod=mod", "-modfile="+modfile, "-overlay", overlayFile, "-deps", "./cmd/vivy")
	list.Dir = repoRoot
	out, err := list.CombinedOutput()
	if err != nil {
		t.Fatalf("go list -deps failed: %v\n%s", err, out)
	}
	deps := string(out)
	// The TUI face organ, codeface, and the view/live render stack never
	// link. The sdk/tui leaf packages the control surface and headless
	// output reuse (command, i18n, surface, stream) are runtime-owned and
	// must remain — surface imports the bubbletea model types itself, so
	// the charm render stack stays reachable through it.
	for _, absent := range []string{
		"agent-vivy/internal/codeface",
		"agent-vivy/internal/tui",
		"agent-vivy/sdk/tui/face",
		"agent-vivy/sdk/tui/live",
		"agent-vivy/sdk/tui/view",
		"agent-vivy/sdk/tui/theme",
		"agent-vivy/sdk/tui/internal/textsafe",
		"github.com/charmbracelet/glamour",
	} {
		if strings.Contains(deps, absent) {
			t.Fatalf("selected build closure still links %s", absent)
		}
	}
	for _, present := range []string{
		"agent-vivy/internal/faceprocess",
		"projectvivy/acp",
		"agent-vivy/sdk/tui/command",
		"agent-vivy/sdk/tui/i18n",
		"agent-vivy/sdk/tui/surface",
		"agent-vivy/sdk/tui/stream",
	} {
		if !strings.Contains(deps, present) {
			t.Fatalf("selected build closure is missing %s", present)
		}
	}

	// Compile cmd/vivy through the same overlay+modfile flags Pack uses: the
	// stub must keep the selected build target linkable.
	bin := filepath.Join(t.TempDir(), "vivy-acp")
	build := exec.Command("go", "build", "-modfile="+modfile, "-mod=readonly", "-overlay", overlayFile, "-o", bin, "./cmd/vivy")
	build.Dir = repoRoot
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("selected overlay build failed: %v\n%s", err, out)
	}

	// A recipe that keeps the default/headless face must not stage the stub.
	var headlessRecipe assemblyv1.Recipe
	rawRecipe, err := os.ReadFile(filepath.Join(repoRoot, "recipes", "headless.vivy.yml"))
	if err != nil {
		t.Fatal(err)
	}
	if err := yaml.Unmarshal(rawRecipe, &headlessRecipe); err != nil {
		t.Fatal(err)
	}
	headlessPlan, headlessSource := compileACPPlan(t, repoRoot, headlessRecipe, nil)
	headlessDir := t.TempDir()
	headlessOverlay, err := stagePackOverlay(repoRoot, headlessDir, &headlessRecipe, headlessPlan, headlessSource)
	if err != nil {
		t.Fatal(err)
	}
	headlessRaw, err := os.ReadFile(headlessOverlay)
	if err != nil {
		t.Fatal(err)
	}
	var headlessMap struct {
		Replace map[string]string `json:"Replace"`
	}
	if err := json.Unmarshal(headlessRaw, &headlessMap); err != nil {
		t.Fatal(err)
	}
	if len(headlessMap.Replace) != 1 {
		t.Fatalf("non-selected overlay Replace = %v, want only zz_default.go", headlessMap.Replace)
	}
}
