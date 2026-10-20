package sdk

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	providerconformance "agent-vivy/sdk/conformance"
	generationv1 "agent-vivy/sdk/generation"
	assemblyv1 "agent-vivy/sdk/internal/assembly"
	"agent-vivy/sdk/module"
)

func TestRepositoryModulePathsUseCanonicalNamespace(t *testing.T) {
	const canonicalPrefix = "agent-vivy/"
	placeholder := "example.com" + "/vivy"

	for _, entry := range repoSourceDirs {
		if strings.Contains(entry.importPath, placeholder) {
			t.Errorf("%s uses placeholder import path %q", entry.dir, entry.importPath)
		}
		if !strings.HasPrefix(entry.importPath, canonicalPrefix) {
			t.Errorf("%s import path = %q, want prefix %q", entry.dir, entry.importPath, canonicalPrefix)
		}
	}

	for _, path := range []string{
		"../../faces/headless/go.mod",
		"../../faces/tui/go.mod",
		"../../plugins/dingtalk/go.mod",
		"../../plugins/discord/go.mod",
		"../../plugins/feishu/go.mod",
		"../../plugins/governance/go.mod",
		"../../plugins/coding/lsp/go.mod",
		"../../plugins/qq/go.mod",
		"../../plugins/scx-reference/go.mod",
		"../../plugins/telegram/go.mod",
		"../../plugins/vivy-persona/go.mod",
		"../../plugins/vivy-evolution/go.mod",
		"../../plugins/vivy-memory/go.mod",
		"../../plugins/vivy-masks-ui/go.mod",
		"../../plugins/vivy-notebook/go.mod",
	} {
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		if strings.Contains(string(body), placeholder) {
			t.Errorf("%s contains placeholder module path %q", path, placeholder)
		}
	}
}

func TestWriteEmbeddedManifestOverlay(t *testing.T) {
	root := t.TempDir()
	overlayFile := filepath.Join(root, "overlay.json")
	if err := os.WriteFile(overlayFile, []byte(`{"Replace":{"existing.go":"existing-replacement.go"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	sourcePath := filepath.Join(root, "manifest.go")
	original := []byte("package generation\n\nvar EmbeddedManifestBase64 string\n\nfunc EmbeddedManifest() {}\n")
	if err := os.WriteFile(sourcePath, original, 0o600); err != nil {
		t.Fatal(err)
	}
	replacementPath := filepath.Join(root, "generated-manifest.go")
	embedded := "VIVY_GENERATION_V1_BEGIN[eyJnZW5lcmF0aW9uSWQiOiJ0ZXN0In0=]VIVY_GENERATION_V1_END"
	if err := writeEmbeddedManifestOverlay(overlayFile, sourcePath, replacementPath, embedded); err != nil {
		t.Fatal(err)
	}
	replacement, err := os.ReadFile(replacementPath)
	if err != nil {
		t.Fatal(err)
	}
	wantValue := `var EmbeddedManifestBase64 = "` + embedded + `"`
	if !strings.Contains(string(replacement), wantValue) {
		t.Fatalf("generated manifest source = %q, want %q", replacement, wantValue)
	}
	if strings.Contains(string(replacement), "var EmbeddedManifestBase64 string") || !strings.Contains(string(replacement), "func EmbeddedManifest()") {
		t.Fatalf("generated manifest source did not preserve the original file: %q", replacement)
	}
	raw, err := os.ReadFile(overlayFile)
	if err != nil {
		t.Fatal(err)
	}
	var overlay struct {
		Replace map[string]string
	}
	if err := json.Unmarshal(raw, &overlay); err != nil {
		t.Fatal(err)
	}
	if got := overlay.Replace[sourcePath]; got != replacementPath {
		t.Fatalf("manifest overlay replacement = %q, want %q", got, replacementPath)
	}
	if got := overlay.Replace["existing.go"]; got != "existing-replacement.go" {
		t.Fatalf("existing overlay replacement = %q, want it preserved", got)
	}
}

func TestV1PackAndInspectProveRecipeRemoval(t *testing.T) {
	root := t.TempDir()
	defaultArtifact, err := Pack(context.Background(), packOptions{Recipe: "../../recipes/default.vivy.yml", Output: filepath.Join(root, "default")})
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(root, "minimal")
	artifact, err := Pack(context.Background(), packOptions{Recipe: "../../recipes/minimal.vivy.yml", Output: out})
	if err != nil {
		t.Fatal(err)
	}
	if artifact.Manifest.GenerationID == "" {
		t.Fatal("missing generation identity")
	}
	inspected, err := InspectArtifact(out)
	if err != nil {
		t.Fatal(err)
	}
	if inspected.Manifest.GenerationID != artifact.Manifest.GenerationID {
		t.Fatal("inspect identity drift")
	}
	if err := os.Chmod(artifact.Binary, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := InspectArtifact(out); err != nil {
		t.Fatalf("InspectArtifact executed the non-executable target instead of parsing it: %v", err)
	}
	if err := os.Chmod(artifact.Binary, 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := InspectArtifact(defaultArtifact.Directory); err != nil {
		t.Fatalf("default artifact inspection failed: %v", err)
	}
	binder, err := os.ReadFile(filepath.Join(out, "zz_assembly.go"))
	if err != nil {
		t.Fatal(err)
	}
	for _, omitted := range []string{
		"vivy/dingtalk", "channel.poll", "agent-vivy/plugins/dingtalk",
		"vivy/context-host", "vivy/context-source", "vivy/skill-host", "vivy/skill-source",
	} {
		if strings.Contains(string(binder), omitted) {
			t.Errorf("minimal binder contains omitted %q", omitted)
		}
	}
	// This gate is intentionally scoped to the generated Assembly source and
	// sealed Manifest. The common app/runtime packages are compiled by every
	// packed command, so their Go package symbols are outside P4's generator
	// boundary and are not falsely presented as physically absent here.
	omittedModules := map[string]bool{
		"vivy/context-host":   true,
		"vivy/context-source": true,
		"vivy/skill-host":     true,
		"vivy/skill-source":   true,
	}
	for _, module := range inspected.Manifest.Modules {
		if omittedModules[module.ID] {
			t.Errorf("minimal Manifest contains omitted module %q", module.ID)
		}
	}
	for _, edge := range inspected.Manifest.PortEdges {
		if omittedModules[edge.Provider] || omittedModules[edge.Consumer] {
			t.Errorf("minimal Manifest contains edge to omitted module: %#v", edge)
		}
	}
	if info, err := os.Stat(artifact.Binary); err != nil || (runtime.GOOS != "windows" && info.Mode()&0o111 == 0) {
		t.Fatalf("generation binary is not executable: %v", err)
	}
}

func TestCapabilityStatesRequireTypedMCPHostBinding(t *testing.T) {
	descriptor := module.Descriptor{
		Module:   module.Identity{ID: "vivy/mcp-host", Version: "1.0.0"},
		Provides: []module.PortRef{{Port: "std/tool-world@v1", ID: "mcp"}},
	}
	plan := assemblyv1.AssemblyPlan{Modules: []assemblyv1.ResolvedModule{{Descriptor: descriptor}}}
	states := capabilityStatesForPlan(plan)
	if got := states["mcp"]; got != assemblyv1.CapabilityNotCompiled {
		t.Fatalf("untyped MCP world capability = %q, want NOT_COMPILED", got)
	}
	plan.Modules[0].Binding.MCPHostProvider = true
	states = capabilityStatesForPlan(plan)
	if got := states["mcp"]; got != assemblyv1.CapabilityUnconfigured {
		t.Fatalf("typed MCP host capability = %q, want UNCONFIGURED", got)
	}
}

func TestPackAndInspectSealUIAssemblyIdentity(t *testing.T) {
	repoRoot, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	source := writePackUIFixture(t, repoRoot)
	recipePath := filepath.Join(t.TempDir(), "recipe.yml")
	if err := os.WriteFile(recipePath, []byte(packUIRecipe()), 0o600); err != nil {
		t.Fatal(err)
	}
	first, err := Pack(context.Background(), packOptions{Recipe: recipePath, Output: filepath.Join(t.TempDir(), "first"), Sources: []string{source}})
	if err != nil {
		t.Fatal(err)
	}
	second, err := Pack(context.Background(), packOptions{Recipe: recipePath, Output: filepath.Join(t.TempDir(), "second"), Sources: []string{source}})
	if err != nil {
		t.Fatal(err)
	}
	if first.Manifest.GenerationID == "" || first.Manifest.GenerationID == second.Manifest.GenerationID {
		t.Fatal("each pack must allocate a distinct Generation ID")
	}
	inspected, err := InspectArtifact(first.Directory)
	if err != nil {
		t.Fatal(err)
	}
	if inspected.Manifest.GenerationID != first.Manifest.GenerationID || inspected.Manifest.UI == nil || inspected.Manifest.UI.GenerationID != first.Manifest.GenerationID || inspected.Manifest.UI.Root != "example.pack-ui-root" {
		t.Fatalf("UI/Inspect identity differs from pack: %#v", inspected.Manifest)
	}
}

func packUIRecipe() string {
	return fmt.Sprintf(`apiVersion: vivy.generation/v1
profile: minimal
modules: [vivy/loop, vivy/model, vivy/tool-host, vivy/storage, vivy/checkpoint, vivy/credential, vivy/sandbox, vivy/presentation-host, example/pack-ui]
sources:
  example/pack-ui: {ref: file:pack-ui}
exclusive:
  std/ui-root@v1: example/pack-ui
order:
  std/ui-extension@v1: [example/pack-ui]
ui:
  sdkVersion: %s
  root: {id: example.pack-ui-root, moduleId: example/pack-ui, port: std/ui-root@v1, entry: ./root.tsx, export: root}
  extensions:
    - {id: example.pack-ui-extension, moduleId: example/pack-ui, port: std/ui-extension@v1, entry: ./extension.tsx, export: extension}
`, assemblyv1.UIAssemblySDKVersion)
}

func TestPackBuildsSelectedUIIntoFinalArtifact(t *testing.T) {
	repoRoot, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	source := writePackUIFixture(t, repoRoot)
	recipe := filepath.Join(t.TempDir(), "recipe.yml")
	recipeBody := packUIRecipe()
	if err := os.WriteFile(recipe, []byte(recipeBody), 0o600); err != nil {
		t.Fatal(err)
	}
	artifact, err := Pack(context.Background(), packOptions{
		Recipe:  recipe,
		Output:  filepath.Join(t.TempDir(), "artifact"),
		Sources: []string{source},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(artifact.Directory, "ui", "dist")); err != nil {
		t.Fatalf("Pack() did not publish final UI artifact: %v", err)
	}
	var emitted []string
	err = filepath.WalkDir(filepath.Join(artifact.Directory, "ui", "dist"), func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		body, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		emitted = append(emitted, string(body))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	bundle := strings.Join(emitted, "\n")
	for _, marker := range []string{"PACK_SELECTED_ROOT_MARKER", "PACK_SELECTED_EXTENSION_MARKER", "plugin.example/pack-ui.marker"} {
		if !strings.Contains(bundle, marker) {
			t.Fatalf("final UI bundle omitted selected %s; Pack must compile generated Assembly, not copy the repository dist", marker)
		}
	}
	binary, err := os.ReadFile(artifact.Binary)
	if err != nil {
		t.Fatal(err)
	}
	for _, marker := range []string{"PACK_SELECTED_ROOT_MARKER", "PACK_SELECTED_EXTENSION_MARKER", "plugin.example/pack-ui.marker"} {
		if !bytes.Contains(binary, []byte(marker)) {
			t.Fatalf("packed executable omitted selected UI marker %s; embedded Handler would serve the repository UI", marker)
		}
	}
	if inspected, inspectErr := InspectArtifact(artifact.Directory); inspectErr != nil {
		t.Fatal(inspectErr)
	} else if inspected.Manifest.UI == nil || inspected.Manifest.UI.GenerationID != artifact.Manifest.GenerationID {
		t.Fatal("UI artifact identity differs from packed Generation")
	} else if !strings.Contains(bundle, artifact.Manifest.GenerationID) {
		t.Fatalf("selected UI bundle omitted Generation identity %s", artifact.Manifest.GenerationID)
	}
}

func TestPackAndInspectCheckedInFullUIGateRecipe(t *testing.T) {
	artifact, err := Pack(context.Background(), packOptions{
		Recipe:  "testdata/full-ui.vivy.yml",
		Output:  filepath.Join(t.TempDir(), "artifact"),
		Sources: []string{"testdata/full-ui-module"},
	})
	if err != nil {
		t.Fatal(err)
	}
	inspected, err := InspectArtifact(artifact.Directory)
	if err != nil {
		t.Fatal(err)
	}
	if inspected.Manifest.UI == nil || inspected.Manifest.UI.Root != "fixture.full-ui.root" {
		t.Fatalf("full-UI gate root = %#v", inspected.Manifest.UI)
	}
	if len(inspected.Manifest.ConformanceResults) != 3*len(providerconformance.RequiredProviderChecks()) {
		t.Fatalf("full-UI gate conformance results = %#v", inspected.Manifest.ConformanceResults)
	}
	t.Logf("full-UI generation=%s", inspected.Manifest.GenerationID)
}

func writePackUIFixture(t *testing.T, repoRoot string) string {
	t.Helper()
	root := temporaryRepositoryModule(t, repoRoot, "pack-ui-")
	if err := os.MkdirAll(filepath.Join(root, "i18n"), 0o700); err != nil {
		t.Fatal(err)
	}
	goMod := "module example.com/pack-ui\n\ngo 1.26.4\n\nrequire agent-vivy v0.0.0\nreplace agent-vivy => ../..\n"
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte(goMod), 0o600); err != nil {
		t.Fatal(err)
	}
	plugin := `package packui

import (
	"context"
	"agent-vivy/sdk/module"
)

type owner struct{}
type instance struct{}

func New() module.Module { return owner{} }
func NewProvider() struct{} { return struct{}{} }
func (owner) Descriptor() module.Descriptor { return module.Descriptor{} }
func (owner) Construct(context.Context, module.Host) (module.Instance, error) { return instance{}, nil }
func (instance) Start(context.Context) error { return nil }
func (instance) Ready(context.Context) error { return nil }
func (instance) Stop(context.Context) error { return nil }
func (instance) Close(context.Context) error { return nil }
`
	if err := os.WriteFile(filepath.Join(root, "plugin.go"), []byte(plugin), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "root.tsx"), []byte(`import { defineUIRoot } from '@vivy/ui-sdk';

export const root = defineUIRoot({
  id: 'example.pack-ui-root',
  render: () => <div>PACK_SELECTED_ROOT_MARKER</div>,
});
`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "extension.tsx"), []byte(`import { defineUIExtension } from '@vivy/ui-sdk';

export const extension = defineUIExtension({
  id: 'example.pack-ui-extension',
  install: (host) => {
    host.composition.navigation.register('pack-ui-marker', {
      label: 'PACK_SELECTED_EXTENSION_MARKER',
      to: '/pack-ui',
    });
  },
});
`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "pnpm-lock.yaml"), []byte("lockfileVersion: '9.0'\n\nimporters: {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	catalog := `{"apiVersion":"vivy.i18n/v1","units":{"plugin.example/pack-ui.marker":{"description":"Pack marker","placeholders":[],"messages":{"en":"PACK_CATALOG_MARKER"}}}}`
	if err := os.WriteFile(filepath.Join(root, "i18n", "catalog.json"), []byte(catalog), 0o600); err != nil {
		t.Fatal(err)
	}
	descriptor := `apiVersion: vivy.module/v1
module: {id: example/pack-ui, version: 1.0.0}
source: {ref: file:pack-ui}
provides:
  - {port: std/ui-root@v1, id: example.pack-ui-root}
  - {port: std/ui-extension@v1, id: example.pack-ui-extension}
i18n: {catalog: i18n/catalog.json, default_locale: en, locales: [en]}
lifecycle: {scope: generation}
`
	if err := os.WriteFile(filepath.Join(root, "vivy-module.yaml"), []byte(descriptor), 0o600); err != nil {
		t.Fatal(err)
	}
	return root
}

func temporaryRepositoryModule(t *testing.T, repoRoot, prefix string) string {
	t.Helper()
	workspace := filepath.Join(repoRoot, ".workspace")
	if err := os.MkdirAll(workspace, 0o700); err != nil {
		t.Fatal(err)
	}
	root, err := os.MkdirTemp(workspace, prefix)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(root) })
	return root
}

func TestInspectArtifactDoesNotInferUIHashClaims(t *testing.T) {
	root := t.TempDir()
	artifactDir := filepath.Join(root, "artifact")
	if err := os.MkdirAll(filepath.Join(artifactDir, "ui", "dist"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(artifactDir, "ui", "dist", "index.html"), []byte("final UI asset"), 0o600); err != nil {
		t.Fatal(err)
	}
	sealed, raw, err := assemblyv1.SealManifest(assemblyv1.AssemblyPlan{}, assemblyv1.SealInputs{GenerationID: "authoring-test",
		SpecificationVersion: "vivy.assembly/v1",
		CompilerVersion:      "fixture-compiler",
		SDKVersion:           "fixture-sdk",
		CanonicalRecipe:      []byte(`{"apiVersion":"vivy.generation/v1","modules":[]}`),
	})
	if err != nil || sealed.GenerationID == "" {
		t.Fatalf("seal fixture: %v", err)
	}
	if err := os.WriteFile(filepath.Join(artifactDir, "generation.json"), raw, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(artifactDir, artifactBinaryName(runtime.GOOS)), []byte(generationv1.FrameEmbeddedManifest(raw)), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := InspectArtifact(artifactDir); err != nil {
		t.Fatalf("InspectArtifact() rejected structurally valid artifact with explicit identity: %v", err)
	}
}

func TestArtifactBinaryNameUsesNativeExecutableSuffix(t *testing.T) {
	if got := artifactBinaryName("windows"); got != "vivy.exe" {
		t.Fatalf("Windows artifact name = %q, want vivy.exe", got)
	}
	if got := artifactBinaryName("linux"); got != "vivy" {
		t.Fatalf("Linux artifact name = %q, want vivy", got)
	}
}

func TestPackRejectsUIAssemblySDKPinDrift(t *testing.T) {
	root := t.TempDir()
	recipePath := filepath.Join(root, "recipe.yml")
	body := `apiVersion: vivy.generation/v1
profile: minimal
modules: [vivy/loop, vivy/model, vivy/tool-host, vivy/storage, vivy/checkpoint, vivy/credential, vivy/sandbox]
ui:
  sdkVersion: 1.0.1
`
	if err := os.WriteFile(recipePath, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := Pack(context.Background(), packOptions{
		Recipe: recipePath,
		Output: filepath.Join(root, "drift"),
	})
	if err == nil || !strings.Contains(err.Error(), "pinned UI SDK") {
		t.Fatalf("Pack() error = %v, want pinned UI SDK rejection", err)
	}
}

func TestPackBridgesCanonicalUIExtensionOrder(t *testing.T) {
	input := assemblyv1.UIAssemblyInput{GenerationID: "authoring-test", ExtensionOrder: []string{"fixture/extension-a"}}
	if err := applyRecipeUIOrder(&input, map[string][]string{
		assemblyv1.UIExtensionPort: {"fixture/extension-a"},
	}); err != nil {
		t.Fatal(err)
	}
	if !equalStringSlices(input.ExtensionOrder, []string{"fixture/extension-a"}) {
		t.Fatalf("UI ExtensionOrder = %v, want canonical Recipe order", input.ExtensionOrder)
	}
}

func TestPackRejectsConflictingCanonicalUIExtensionOrder(t *testing.T) {
	input := assemblyv1.UIAssemblyInput{GenerationID: "authoring-test", ExtensionOrder: []string{"fixture/extension-a"}}
	err := applyRecipeUIOrder(&input, map[string][]string{
		assemblyv1.UIExtensionPort: {"fixture/extension-b"},
	})
	if err == nil || !strings.Contains(err.Error(), "disagrees") {
		t.Fatalf("applyRecipeUIOrder() error = %v, want disagreement", err)
	}
}

func TestFailedCompilationEmitsNoGenerationDirectory(t *testing.T) {
	dir := t.TempDir()
	recipe := filepath.Join(dir, "bad.yml")
	out := filepath.Join(dir, "generation")
	if err := os.WriteFile(recipe, []byte("apiVersion: vivy.generation/v1\nmodules: [missing/module]\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Pack(context.Background(), packOptions{Recipe: recipe, Output: out}); err == nil {
		t.Fatal("Pack succeeded")
	}
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		t.Fatalf("failed pack emitted output: %v", err)
	}
}

func TestGenerationFailureMatrixEmitsNoFormalArtifact(t *testing.T) {
	const minimalModules = "vivy/loop, vivy/model, vivy/tool-host, vivy/storage, vivy/checkpoint, vivy/credential, vivy/sandbox"
	tests := []struct {
		name      string
		recipe    string
		wantError string
	}{
		{
			name:      "legacy v0 recipe",
			recipe:    "apiVersion: vivy.generation/v0\nmodules: [" + minimalModules + "]\n",
			wantError: "unsupported recipe apiVersion vivy.generation/v0",
		},
		{
			name:      "missing module",
			recipe:    "apiVersion: vivy.generation/v1\nmodules: [" + minimalModules + ", missing/module]\n",
			wantError: "missing source for module missing/module",
		},
		{
			name:      "duplicate module",
			recipe:    "apiVersion: vivy.generation/v1\nmodules: [" + minimalModules + ", vivy/loop]\n",
			wantError: "duplicate module vivy/loop",
		},
		{
			name:      "selected Provider without Host",
			recipe:    "apiVersion: vivy.generation/v1\nmodules: [" + minimalModules + ", vivy/context-source]\n",
			wantError: "requires conditional Host core/context-host@v1",
		},
		{
			name: "duplicate exclusive face Provider",
			recipe: "apiVersion: vivy.generation/v1\nmodules: [" + minimalModules + ", vivy/face-host, vivy/headless, vivy/tui]\nexclusive:\n" +
				"  std/face@v1: vivy/headless\n",
			wantError: "duplicate provider for std/face@v1",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			recipe := filepath.Join(root, "recipe.yml")
			output := filepath.Join(root, "generation")
			if err := os.WriteFile(recipe, []byte(test.recipe), 0o600); err != nil {
				t.Fatal(err)
			}
			_, err := Pack(context.Background(), packOptions{Recipe: recipe, Output: output})
			if err == nil || !strings.Contains(err.Error(), test.wantError) {
				t.Fatalf("Pack() error = %v, want %q", err, test.wantError)
			}
			if _, statErr := os.Stat(output); !os.IsNotExist(statErr) {
				t.Fatalf("invalid Generation emitted formal artifact: %v", statErr)
			}
		})
	}
}

func TestPackAndInspectEveryShippedRecipe(t *testing.T) {
	root := t.TempDir()
	tests := []struct {
		name, channels, mcp string
	}{
		{"default", string(assemblyv1.CapabilityUnconfigured), string(assemblyv1.CapabilityUnconfigured)},
		{"lite", string(assemblyv1.CapabilityNotCompiled), string(assemblyv1.CapabilityNotCompiled)},
		{"masks-selected", string(assemblyv1.CapabilityUnconfigured), string(assemblyv1.CapabilityUnconfigured)},
		{"masks-omitted", string(assemblyv1.CapabilityUnconfigured), string(assemblyv1.CapabilityUnconfigured)},
		{"masks-backend-only", string(assemblyv1.CapabilityUnconfigured), string(assemblyv1.CapabilityUnconfigured)},
		{"minimal", string(assemblyv1.CapabilityNotCompiled), string(assemblyv1.CapabilityNotCompiled)},
		{"headless", string(assemblyv1.CapabilityNotCompiled), string(assemblyv1.CapabilityUnconfigured)},
		{"vivy-code", string(assemblyv1.CapabilityNotCompiled), string(assemblyv1.CapabilityUnconfigured)},
		{"scx", string(assemblyv1.CapabilityNotCompiled), string(assemblyv1.CapabilityUnconfigured)},
	}
	for _, test := range tests {
		name := test.name
		t.Run(name, func(t *testing.T) {
			output := filepath.Join(root, name)
			artifact, err := Pack(context.Background(), packOptions{Recipe: filepath.Join("..", "..", "recipes", name+".vivy.yml"), Output: output})
			if err != nil {
				t.Fatal(err)
			}
			t.Logf("recipe=recipes/%s.vivy.yml target=executable generation=%s", name, artifact.Manifest.GenerationID)
			inspected, err := InspectArtifact(output)
			if err != nil {
				t.Fatal(err)
			}
			if inspected.Manifest.GenerationID != artifact.Manifest.GenerationID {
				t.Fatal("embedded and returned Generation identity differ")
			}
			if got := string(inspected.Manifest.CapabilityStates["channels"]); got != test.channels {
				t.Fatalf("channels capability = %s, want %s", got, test.channels)
			}
			if got := string(inspected.Manifest.CapabilityStates["mcp"]); got != test.mcp {
				t.Fatalf("mcp capability = %s, want %s", got, test.mcp)
			}
		})
	}
}

func TestPackAndInspectSCXCandidateIsDeterministicAndRemovable(t *testing.T) {
	root := t.TempDir()
	pack := func(name, recipe string) Artifact {
		t.Helper()
		artifact, err := Pack(context.Background(), packOptions{
			Recipe: filepath.Join("..", "..", "recipes", recipe+".vivy.yml"),
			Output: filepath.Join(root, name),
		})
		if err != nil {
			t.Fatal(err)
		}
		inspected, err := InspectArtifact(artifact.Directory)
		if err != nil {
			t.Fatal(err)
		}
		return inspected
	}

	first := pack("scx-first", "scx")
	second := pack("scx-second", "scx")
	if first.Manifest.GenerationID == second.Manifest.GenerationID {
		t.Fatalf("SCX packs reused identity: %s", first.Manifest.GenerationID)
	}

	const fixtureID = "scx/reference-fixtures"
	var fixture *assemblyv1.ManifestModule
	for index := range first.Manifest.Modules {
		if first.Manifest.Modules[index].ID == fixtureID {
			fixture = &first.Manifest.Modules[index]
			break
		}
	}
	if fixture == nil {
		t.Fatal("SCX candidate Manifest omits reference fixture module")
	}
	if fixture.Version != "0.1.0" || fixture.Source.Ref != "repo:plugins/scx-reference" {
		t.Fatalf("SCX source identity is not sealed: %#v", fixture)
	}
	if len(first.Manifest.ContextSourcePolicies) != 1 || first.Manifest.ContextSourcePolicies[0].ProviderID != "scx.reference" || !first.Manifest.ContextSourcePolicies[0].Required {
		t.Fatalf("SCX required Source policy is not sealed: %#v", first.Manifest.ContextSourcePolicies)
	}
	if len(first.Manifest.RunObserverPolicies) != 1 || first.Manifest.RunObserverPolicies[0].ProviderID != "scx.reference" {
		t.Fatalf("SCX Observer policy is not sealed: %#v", first.Manifest.RunObserverPolicies)
	}
	edges := map[string]bool{}
	for _, edge := range first.Manifest.PortEdges {
		if edge.Provider == fixtureID || edge.Consumer == fixtureID {
			edges[edge.Port.Port+":"+edge.Provider+":"+edge.Consumer] = true
		}
	}
	for _, required := range []string{
		"core/context-host@v1:vivy/context-host:" + fixtureID,
		"core/observer-host@v1:vivy/observer-host:" + fixtureID,
		"std/context-source@v1:" + fixtureID + ":vivy/context-host",
		"std/observer/run@v1:" + fixtureID + ":vivy/observer-host",
	} {
		if !edges[required] {
			t.Errorf("SCX candidate Manifest omits edge %q", required)
		}
	}
	for _, recipe := range []string{"default", "minimal"} {
		artifact := pack(recipe, recipe)
		for _, selected := range artifact.Manifest.Modules {
			if selected.ID == fixtureID {
				t.Fatalf("%s Generation retained removed SCX module", recipe)
			}
		}
	}
}

func TestInspectRejectsManifestFromAnotherBinary(t *testing.T) {
	root := t.TempDir()
	first, err := Pack(context.Background(), packOptions{Recipe: "../../recipes/default.vivy.yml", Output: filepath.Join(root, "default")})
	if err != nil {
		t.Fatal(err)
	}
	second, err := Pack(context.Background(), packOptions{Recipe: "../../recipes/minimal.vivy.yml", Output: filepath.Join(root, "minimal")})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(first.Directory, "generation.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(second.Directory, "generation.json"), raw, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := InspectArtifact(second.Directory); err == nil || !strings.Contains(err.Error(), "not bound to executable") {
		t.Fatalf("InspectArtifact() error = %v, want binary binding rejection", err)
	}
}

func TestVerifyAcceptsChangedSourceWithoutHashClaim(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/modified\n\ngo 1.26.4\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	descriptor, err := os.ReadFile("../../plugins/hello-fs/vivy-module.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "vivy-module.yaml"), descriptor, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "plugin.go"), []byte("package modified\n\nfunc New() {}\nfunc NewProvider() {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Verify(dir); err != nil {
		t.Fatalf("Verify() rejected valid changed source: %v", err)
	}
}

func TestVerifyEverySelectedPublicModule(t *testing.T) {
	for _, source := range []string{
		"../../plugins/dingtalk",
		"../../plugins/discord",
		"../../plugins/feishu",
		"../../plugins/governance",
		"../../plugins/hello-fs",
		"../../plugins/coding/lsp",
		"../../plugins/qq",
		"../../plugins/scx-reference",
		"../../plugins/telegram",
		"../../faces/headless",
		"../../faces/tui",
		"testdata/full-ui-module",
	} {
		source := source
		t.Run(filepath.Base(source), func(t *testing.T) {
			report, err := Verify(source)
			if err != nil {
				t.Fatalf("Verify(%q) error = %v", source, err)
			}
			if !report.OK || report.Module == "" {
				t.Fatalf("Verify(%q) report = %#v", source, report)
			}
		})
	}
}

func TestPackSourceCatalogAppliesCapabilityFirewall(t *testing.T) {
	dir := t.TempDir()
	descriptor := "apiVersion: vivy.module/v1\nmodule: {id: example/bad, version: 0.1.0}\nsource: {ref: file:test}\nprovides: [{port: std/tool-world@v1, id: example.bad}]\nlifecycle: {scope: generation}\n"
	if err := os.WriteFile(filepath.Join(dir, "vivy-module.yaml"), []byte(descriptor), 0o600); err != nil {
		t.Fatal(err)
	}
	plugin := "package bad\n\nimport \"os\"\n\nfunc New() {}\nfunc NewProvider() {}\nfunc forbidden() { _, _ = os.Open(\"secret\") }\n"
	if err := os.WriteFile(filepath.Join(dir, "plugin.go"), []byte(plugin), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := sourceRecords("../..", []string{dir}, nil); err == nil || !strings.Contains(err.Error(), "forbidden direct capability call") {
		t.Fatalf("sourceRecords() error = %v, want capability firewall rejection", err)
	}
}

func TestCapabilityFirewallRejectsFunctionValuesAndDotImports(t *testing.T) {
	for name, source := range map[string]string{
		"dot-import":     "package bad\n\nimport . \"os\"\nfunc New() {}\nfunc NewProvider() {}\n",
		"function-value": "package bad\n\nimport \"os\"\nvar read = os.ReadFile\nfunc New() {}\nfunc NewProvider() {}\n",
	} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, "plugin.go"), []byte(source), 0o600); err != nil {
				t.Fatal(err)
			}
			descriptor := module.Descriptor{Source: module.Source{}}
			if err := verifySource(dir, descriptor); err == nil {
				t.Fatal("capability firewall accepted bypass")
			}
		})
	}
}

func TestPackSelectedToolWorlds(t *testing.T) {
	tests := []struct {
		name, recipe string
	}{
		{"hello-fs", "apiVersion: vivy.generation/v1\nmodules: [vivy/loop, vivy/model, vivy/tool-host, vivy/storage, vivy/checkpoint, vivy/credential, vivy/sandbox, vivy/hello-fs]\ngrantApprovals:\n  - {module: vivy/hello-fs, name: fs.read}\n"},
		{"lsp", "apiVersion: vivy.generation/v1\nmodules: [vivy/loop, vivy/model, vivy/tool-host, vivy/storage, vivy/checkpoint, vivy/credential, vivy/sandbox, vivy/lsp]\ngrantApprovals:\n  - {module: vivy/lsp, name: fs.read}\n  - {module: vivy/lsp, name: fs.write}\n  - module: vivy/lsp\n    name: proc.spawn\n    constraints: {commands: [gopls, typescript-language-server, pyright-langserver, rust-analyzer]}\n    evidence: [build:plugins/coding/lsp/plugin_test.go]\n"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			repoRecipe := filepath.Join(root, "recipe.yml")
			if err := os.WriteFile(repoRecipe, []byte(test.recipe), 0o600); err != nil {
				t.Fatal(err)
			}
			artifact, err := Pack(context.Background(), packOptions{Recipe: repoRecipe, Output: filepath.Join(root, "artifact")})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := InspectArtifact(artifact.Directory); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestPackLinksExternalStandaloneModule(t *testing.T) {
	repoRoot, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	source := temporaryRepositoryModule(t, repoRoot, "acme-tool-")
	goMod := "module example.com/acme/vivy-tool\n\ngo 1.26.4\n\nrequire agent-vivy v0.0.0\nreplace agent-vivy => ../..\n"
	if err := os.WriteFile(filepath.Join(source, "go.mod"), []byte(goMod), 0o600); err != nil {
		t.Fatal(err)
	}
	plugin := `package acmetool

import (
	"context"
	"encoding/json"
	"agent-vivy/sdk/module"
	toolport "agent-vivy/sdk/port/tool"
)

type owner struct{}
type instance struct{}
type provider struct{}
func New() module.Module { return owner{} }
func (owner) Descriptor() module.Descriptor { return module.Descriptor{} }
func (owner) Construct(context.Context, module.Host) (module.Instance, error) { return instance{}, nil }
func (instance) Start(context.Context) error { return nil }
func (instance) Ready(context.Context) error { return nil }
func (instance) Stop(context.Context) error { return nil }
func (instance) Close(context.Context) error { return nil }
func NewProvider() toolport.ToolProvider { return provider{} }
func (provider) Definition() toolport.Definition { return toolport.Definition{ID: "acme.greet", Description: "greet", Effect: toolport.EffectRead, Schema: json.RawMessage(` + "`" + `{"type":"object"}` + "`" + `)} }
func (provider) Invoke(context.Context, toolport.Host, json.RawMessage) (toolport.Result, error) { return toolport.Result{Text: "hello"}, nil }
`
	if err := os.WriteFile(filepath.Join(source, "provider.go"), []byte(plugin), 0o600); err != nil {
		t.Fatal(err)
	}
	descriptor := "apiVersion: vivy.module/v1\nmodule: {id: acme/greet, version: 0.1.0}\nsource: {ref: file:test}\nprovides: [{port: std/tool@v1, id: acme.greet}]\nrequires: [{port: core/tool-host@v1, provider: vivy/tool-host}]\nlifecycle: {scope: generation}\n"
	if err := os.WriteFile(filepath.Join(source, "vivy-module.yaml"), []byte(descriptor), 0o600); err != nil {
		t.Fatal(err)
	}
	recipe := filepath.Join(t.TempDir(), "recipe.yml")
	recipeBody := "apiVersion: vivy.generation/v1\nmodules: [vivy/loop, vivy/model, vivy/tool-host, vivy/storage, vivy/checkpoint, vivy/credential, vivy/sandbox, acme/greet]\nsources:\n  acme/greet: {ref: file:test}\n"
	if err := os.WriteFile(recipe, []byte(recipeBody), 0o600); err != nil {
		t.Fatal(err)
	}
	artifact, err := Pack(context.Background(), packOptions{Recipe: recipe, Output: filepath.Join(t.TempDir(), "artifact"), Sources: []string{source}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := InspectArtifact(artifact.Directory); err != nil {
		t.Fatal(err)
	}
	binder, err := os.ReadFile(filepath.Join(artifact.Directory, "zz_assembly.go"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(binder), `"example.com/acme/vivy-tool"`) {
		t.Fatal("external module is absent from generated binder")
	}
}

// DN-L: the shared library target must produce a sealed DLL/.so whose
// embedded Manifest, ABI headers, and artifact identity all verify through
// the same inspect path as the executable, and whose failures publish
// nothing.
func TestPackAndInspectSharedTarget(t *testing.T) {
	root := t.TempDir()
	out := filepath.Join(root, "diva-shared")
	artifact, err := Pack(context.Background(), packOptions{Recipe: "../../recipes/diva.vivy.yml", Output: out, Target: packTargetShared})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("recipe=recipes/diva.vivy.yml target=shared generation=%s", artifact.Manifest.GenerationID)
	sharedName := artifactSharedName(runtime.GOOS)
	if filepath.Base(artifact.Binary) != sharedName {
		t.Fatalf("shared binary = %q, want %q", artifact.Binary, sharedName)
	}
	if _, err := os.Stat(filepath.Join(out, "vivy_abi.h")); err != nil {
		t.Fatalf("ABI constants header missing: %v", err)
	}
	headerName := strings.TrimSuffix(sharedName, filepath.Ext(sharedName)) + ".h"
	if _, err := os.Stat(filepath.Join(out, headerName)); err != nil {
		t.Fatalf("generated ABI header missing: %v", err)
	}
	inspected, err := InspectArtifact(out)
	if err != nil {
		t.Fatalf("shared inspect failed: %v", err)
	}
	if inspected.Manifest.GenerationID != artifact.Manifest.GenerationID {
		t.Fatal("shared inspect identity drift")
	}
	// A shared artifact directory must not satisfy the executable check:
	// no vivy binary may exist beside the library.
	if _, err := os.Stat(filepath.Join(out, artifactBinaryName(runtime.GOOS))); !os.IsNotExist(err) {
		t.Fatalf("shared artifact unexpectedly contains an executable: %v", err)
	}

	// Tampered ABI header is an inspection failure.
	abiPath := filepath.Join(out, "vivy_abi.h")
	original, err := os.ReadFile(abiPath)
	if err != nil {
		t.Fatal(err)
	}
	tampered := bytes.Replace(original, []byte("VIVY_ABI_VERSION 1"), []byte("VIVY_ABI_VERSION 999"), 1)
	if err := os.WriteFile(abiPath, tampered, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := InspectArtifact(out); err == nil {
		t.Fatal("inspect accepted a tampered ABI header")
	}
	if err := os.WriteFile(abiPath, original, 0o644); err != nil {
		t.Fatal(err)
	}

	// Tampered library bytes inside the embedded Manifest break the binding.
	// Some platforms (PE) store the framed constant more than once, so every
	// occurrence must be corrupted: inspection skips invalid frames and only
	// rejects when no intact copy survives.
	bin, err := os.ReadFile(artifact.Binary)
	if err != nil {
		t.Fatal(err)
	}
	marker := []byte("VIVY_GENERATION_V1_BEGIN[")
	frames := 0
	for scan := 0; ; {
		offset := bytes.Index(bin[scan:], marker)
		if offset < 0 {
			break
		}
		offset += scan
		bin[offset+len(marker)+8] ^= 0xFF
		frames++
		scan = offset + len(marker)
	}
	if frames == 0 {
		t.Fatal("shared library does not carry the sealed manifest frame")
	}
	if err := os.WriteFile(artifact.Binary, bin, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := InspectArtifact(out); err == nil {
		t.Fatal("inspect accepted a tampered shared library")
	}
}

// DN-L: a recipe naming a missing module must fail before any Generation is
// published to the output directory.
func TestPackSharedMissingModulePublishesNothing(t *testing.T) {
	root := t.TempDir()
	recipe := filepath.Join(root, "bad.vivy.yml")
	if err := os.WriteFile(recipe, []byte("apiVersion: vivy.generation/v1\nprofile: minimal\nmodules: [vivy/loop, vivy/does-not-exist]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(root, "out")
	if _, err := Pack(context.Background(), packOptions{Recipe: recipe, Output: out, Target: packTargetShared}); err == nil {
		t.Fatal("pack accepted a missing module")
	}
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		t.Fatalf("failed pack published output: %v", err)
	}
}
