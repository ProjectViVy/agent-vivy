package sdk

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	assemblyv1 "agent-vivy/sdk/internal/assembly"
	"agent-vivy/sdk/module"
)

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
		"vivy/dingtalk", "channel.poll", "example.com/vivy/plugins/dingtalk",
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
	if info, err := os.Stat(artifact.Binary); err != nil || info.Mode()&0o111 == 0 {
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

func TestPackAndInspectEveryShippedRecipe(t *testing.T) {
	root := t.TempDir()
	tests := []struct {
		name, channels, mcp string
	}{
		{"default", string(assemblyv1.CapabilityUnconfigured), string(assemblyv1.CapabilityUnconfigured)},
		{"minimal", string(assemblyv1.CapabilityNotCompiled), string(assemblyv1.CapabilityNotCompiled)},
		{"headless", string(assemblyv1.CapabilityNotCompiled), string(assemblyv1.CapabilityUnconfigured)},
		{"vivy-code", string(assemblyv1.CapabilityNotCompiled), string(assemblyv1.CapabilityUnconfigured)},
	}
	for _, test := range tests {
		name := test.name
		t.Run(name, func(t *testing.T) {
			output := filepath.Join(root, name)
			artifact, err := Pack(context.Background(), packOptions{Recipe: filepath.Join("..", "..", "recipes", name+".vivy.yml"), Output: output})
			if err != nil {
				t.Fatal(err)
			}
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

func TestVerifyRejectsModifiedSourceTree(t *testing.T) {
	dir := t.TempDir()
	descriptor, err := os.ReadFile("../../plugins/hello-fs/vivy-module.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "vivy-module.yaml"), descriptor, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "plugin.go"), []byte("package modified\n\n// content changed without updating the declared hash\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Verify(dir); err == nil || !strings.Contains(err.Error(), "source hash mismatch") {
		t.Fatalf("Verify() error = %v, want source hash rejection", err)
	}
}

func TestPackSourceCatalogAppliesCapabilityFirewall(t *testing.T) {
	dir := t.TempDir()
	const placeholder = "0000000000000000000000000000000000000000000000000000000000000000"
	descriptor := "apiVersion: vivy.module/v1\nmodule: {id: example/bad, version: 0.1.0}\nsource: {ref: file:test, sha256: " + placeholder + "}\nprovides: [{port: std/tool-world@v1, id: example.bad}]\nlifecycle: {scope: generation}\n"
	if err := os.WriteFile(filepath.Join(dir, "vivy-module.yaml"), []byte(descriptor), 0o600); err != nil {
		t.Fatal(err)
	}
	plugin := "package bad\n\nimport \"os\"\n\nfunc New() {}\nfunc NewProvider() {}\nfunc forbidden() { _, _ = os.Open(\"secret\") }\n"
	if err := os.WriteFile(filepath.Join(dir, "plugin.go"), []byte(plugin), 0o600); err != nil {
		t.Fatal(err)
	}
	digest, err := assemblyv1.HashSourceTree(dir, placeholder)
	if err != nil {
		t.Fatal(err)
	}
	descriptor = strings.Replace(descriptor, placeholder, digest, 1)
	if err := os.WriteFile(filepath.Join(dir, "vivy-module.yaml"), []byte(descriptor), 0o600); err != nil {
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
			digest, err := assemblyv1.HashSourceTree(dir, "")
			if err != nil {
				t.Fatal(err)
			}
			descriptor := module.Descriptor{Source: module.Source{SHA256: digest}}
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
		{"hello-fs", "apiVersion: vivy.generation/v1\nmodules: [vivy/kernel, vivy/tool-host, vivy/hello-fs]\ngrantApprovals:\n  - {module: vivy/hello-fs, name: fs.read}\n"},
		{"lsp", "apiVersion: vivy.generation/v1\nmodules: [vivy/kernel, vivy/tool-host, vivy/lsp]\ngrantApprovals:\n  - {module: vivy/lsp, name: fs.read}\n  - {module: vivy/lsp, name: fs.write}\n  - module: vivy/lsp\n    name: proc.spawn\n    constraints: {commands: [gopls, typescript-language-server, pyright-langserver, rust-analyzer]}\n    evidence: [build:plugins/lsp/plugin_test.go]\n"},
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
	source := t.TempDir()
	goMod := fmt.Sprintf("module example.com/acme/vivy-tool\n\ngo 1.26.4\n\nrequire agent-vivy v0.0.0\nreplace agent-vivy => %s\n", filepath.ToSlash(repoRoot))
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
	const placeholder = "0000000000000000000000000000000000000000000000000000000000000000"
	descriptor := "apiVersion: vivy.module/v1\nmodule: {id: acme/greet, version: 0.1.0}\nsource: {ref: file:test, sha256: " + placeholder + "}\nprovides: [{port: std/tool@v1, id: acme.greet}]\nrequires: [{port: core/tool-host@v1, provider: vivy/tool-host}]\nlifecycle: {scope: generation}\n"
	if err := os.WriteFile(filepath.Join(source, "vivy-module.yaml"), []byte(descriptor), 0o600); err != nil {
		t.Fatal(err)
	}
	digest, err := assemblyv1.HashSourceTree(source, placeholder)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "vivy-module.yaml"), []byte(strings.Replace(descriptor, placeholder, digest, 1)), 0o600); err != nil {
		t.Fatal(err)
	}
	recipe := filepath.Join(t.TempDir(), "recipe.yml")
	recipeBody := fmt.Sprintf("apiVersion: vivy.generation/v1\nmodules: [vivy/kernel, vivy/tool-host, acme/greet]\nsources:\n  acme/greet: {ref: file:test, sha256: %s}\n", digest)
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

func TestCanonicalResolvedModfileSealsSemanticsWithoutLocalPaths(t *testing.T) {
	writeMod := func(name, localPath, version string) string {
		t.Helper()
		path := filepath.Join(t.TempDir(), name+".mod")
		body := fmt.Sprintf("module example.com/test\n\ngo 1.26.4\n\nrequire example.com/dependency %s\nreplace example.com/dependency => %s\n", version, filepath.ToSlash(localPath))
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		return path
	}
	first, err := canonicalResolvedModfile(".", writeMod("first", filepath.Join(t.TempDir(), "checkout-a"), "v1.2.3"))
	if err != nil {
		t.Fatal(err)
	}
	second, err := canonicalResolvedModfile(".", writeMod("second", filepath.Join(t.TempDir(), "checkout-b"), "v1.2.3"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, second) {
		t.Fatalf("machine-local replace paths changed canonical modfile:\n%s\n%s", first, second)
	}
	changed, err := canonicalResolvedModfile(".", writeMod("changed", filepath.Join(t.TempDir(), "checkout-c"), "v1.2.4"))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(first, changed) {
		t.Fatal("resolved dependency version did not change canonical modfile")
	}
}
