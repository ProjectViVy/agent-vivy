package sdk

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"agent-vivy/sdk/generation"
	assemblyv1 "agent-vivy/sdk/internal/assembly"
	"agent-vivy/sdk/module"
	"agent-vivy/sdk/port"
	"gopkg.in/yaml.v3"
)

// Opt-in integration preparation. Compile the real DIVA recipe using the
// same compiler as Pack, and bind the sealed manifest from a real SDK pack.
// Nothing under internal/generated is hand-edited or replaced on disk.
func TestMemoryLoopGenerateIntegrationOverlay(t *testing.T) {
	output := os.Getenv("VIVY_MEMORY_LOOP_OVERLAY_DIR")
	artifact := os.Getenv("VIVY_MEMORY_LOOP_ARTIFACT")
	if output == "" || artifact == "" {
		t.Skip("integration preparation requires explicit output and packed artifact")
	}
	if !filepath.IsAbs(output) || !filepath.IsAbs(artifact) {
		t.Fatal("integration paths must be absolute")
	}
	root, err := findRepoRoot(".")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(root, "recipes", "diva.vivy.yml"))
	if err != nil {
		t.Fatal(err)
	}
	var recipe assemblyv1.Recipe
	if err := yaml.Unmarshal(raw, &recipe); err != nil {
		t.Fatal(err)
	}
	records, err := sourceRecords(root, nil, recipe.Sources)
	if err != nil {
		t.Fatal(err)
	}
	catalog, err := assemblyv1.NewSourceCatalog(records)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := (assemblyv1.Compiler{Ports: port.PublicCatalog(), Sources: catalog,
		PortEvidence: assemblyv1.SupportedPortEvidence(), ConformanceResults: assemblyv1.SupportedPortConformance()}).Compile(context.Background(), recipe)
	if err != nil {
		t.Fatal(err)
	}
	source, err := assemblyv1.GenerateRuntimeAssembly(plan, "assembly")
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := os.ReadFile(filepath.Join(artifact, "generation.json"))
	if err != nil {
		t.Fatal(err)
	}
	// Inspection verifies the packed binary and source/manifest checksums.
	inspected, err := InspectArtifact(artifact)
	if err != nil {
		t.Fatalf("packed artifact inspection: %v", err)
	}
	uiInput := assemblyv1.UIAssemblyInput{SDKVersion: assemblyv1.UIAssemblySDKVersion, Catalogs: inspected.Manifest.Catalogs}
	if recipe.UI != nil {
		uiInput = *recipe.UI
		uiInput.Catalogs = inspected.Manifest.Catalogs
	}
	if err := applyRecipeUIOrder(&uiInput, recipe.Order); err != nil {
		t.Fatal(err)
	}
	if err := bindUIContentHashes(&uiInput, plan, catalog); err != nil {
		t.Fatal(err)
	}
	if err := bindUIBuildDependencies(&uiInput, root, plan, catalog); err != nil {
		t.Fatal(err)
	}
	setUIAssetHashes(&uiInput, inspected.Manifest.UIArtifacts["ui/dist"])
	recipe.UI = &uiInput
	canonical, err := assemblyv1.CanonicalRecipe(recipe)
	if err != nil {
		t.Fatal(err)
	}
	recipeHash := fmt.Sprintf("%x", sha256.Sum256(canonical))
	modfile, err := prepareBuildModfile(root, t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	dependencies, err := resolveDependencyLocks(root, modfile)
	if err != nil {
		t.Fatal(err)
	}
	if err := memoryLoopVerifyPackedPlan(plan, recipeHash, dependencies, inspected.Manifest); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(output, 0700); err != nil {
		t.Fatal(err)
	}
	replacement := filepath.Join(output, "zz_default.go")
	if err := os.WriteFile(replacement, source, 0600); err != nil {
		t.Fatal(err)
	}
	overlay := filepath.Join(output, "overlay.json")
	encoded, err := json.Marshal(map[string]any{"Replace": map[string]string{
		filepath.Join(root, "internal/generated/assembly/zz_default.go"): replacement,
	}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(overlay, encoded, 0600); err != nil {
		t.Fatal(err)
	}
	if err := writeEmbeddedManifestOverlay(overlay, filepath.Join(root, "sdk/generation/manifest.go"),
		filepath.Join(output, "manifest.go"), generation.FrameEmbeddedManifest(manifest)); err != nil {
		t.Fatal(err)
	}
	t.Logf("generated DIVA integration overlay: %s", overlay)
}

func memoryLoopVerifyPackedPlan(plan assemblyv1.AssemblyPlan, recipeHash string, dependencies map[string]string, manifest assemblyv1.GenerationManifest) error {

	if manifest.RecipeDigest != recipeHash || !reflect.DeepEqual(manifest.DependencyLocks, dependencies) {
		return fmt.Errorf("packed recipe/dependency identity does not match current inputs")
	}
	// Reuse the packer's own canonicalization: descriptors and edges are
	// sorted for sealing, and empty slices may become nil.
	expected, _, err := assemblyv1.SealManifest(plan, assemblyv1.SealInputs{
		SpecificationVersion: "vivy.module/v1", CompilerVersion: "plg-p9",
		SDKVersion: "v1", CanonicalRecipe: []byte("{}"),
	})
	if err != nil {
		return fmt.Errorf("canonicalize current plan: %w", err)
	}
	if !reflect.DeepEqual(manifest.Modules, expected.Modules) {
		return fmt.Errorf("packed module inputs differ from current plan")
	}
	if !reflect.DeepEqual(manifest.PortEdges, expected.PortEdges) || !reflect.DeepEqual(manifest.LifecycleOrder, expected.LifecycleOrder) || !reflect.DeepEqual(manifest.OrderedContributions, expected.OrderedContributions) {
		return fmt.Errorf("packed assembly ordering/bindings differ from current plan")
	}
	return nil
}

func TestMemoryLoopOverlayRejectsMismatchedInputs(t *testing.T) {
	descriptor := module.Descriptor{Module: module.Identity{ID: "test/module", Version: "1.0.0"}, Source: module.Source{Ref: "file:test", SHA256: "current-source"}}
	plan := assemblyv1.AssemblyPlan{Modules: []assemblyv1.ResolvedModule{{Descriptor: descriptor, Trust: assemblyv1.TrustT1}}}
	deps := map[string]string{"example/dependency": "v1.0.0"}
	manifest := assemblyv1.GenerationManifest{RecipeDigest: "current-recipe", DependencyLocks: deps, Modules: []assemblyv1.ManifestModule{{ID: "test/module", Version: "1.0.0", Source: descriptor.Source, Trust: assemblyv1.TrustT1}}}
	if err := memoryLoopVerifyPackedPlan(plan, "current-recipe", deps, manifest); err != nil {
		t.Fatal(err)
	}
	changed := manifest
	changed.RecipeDigest = "stale-recipe"
	if err := memoryLoopVerifyPackedPlan(plan, "current-recipe", deps, changed); err == nil {
		t.Fatal("stale recipe accepted")
	}
	changed = manifest
	changed.Modules = append([]assemblyv1.ManifestModule(nil), manifest.Modules...)
	changed.Modules[0].Source.SHA256 = "stale-source"
	if err := memoryLoopVerifyPackedPlan(plan, "current-recipe", deps, changed); err == nil {
		t.Fatal("stale module source accepted")
	}
	changed = manifest
	changed.DependencyLocks = map[string]string{"example/dependency": "v0.9.0"}
	if err := memoryLoopVerifyPackedPlan(plan, "current-recipe", deps, changed); err == nil {
		t.Fatal("stale dependency accepted")
	}
}
