package assembly

import (
	"agent-vivy/sdk/module"
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestD06GenerationIdentityIsExplicitAndInspectIsStructural(t *testing.T) {
	var inputs SealInputs
	if err := json.Unmarshal([]byte(`{"GenerationID":"authoring-test","SpecificationVersion":"vivy.module/v1","CompilerVersion":"test","SDKVersion":"v1","CanonicalRecipe": "e30="}`), &inputs); err != nil {
		t.Fatal(err)
	}
	manifest, raw, err := SealManifest(manifestTestPlan(), inputs)
	if err != nil {
		t.Fatal(err)
	}
	if manifest.GenerationID != "authoring-test" {
		t.Fatalf("identity = %q, want explicit authoring-test", manifest.GenerationID)
	}
	_, again, err := SealManifest(manifestTestPlan(), inputs)
	if err != nil || !bytes.Equal(raw, again) {
		t.Fatalf("same inputs not byte stable: %v", err)
	}
	var document map[string]any
	if err := json.Unmarshal(raw, &document); err != nil {
		t.Fatal(err)
	}
	document["compilerVersion"] = "changed"
	altered, _ := json.Marshal(document)
	inspected, err := InspectManifest(altered)
	if err != nil || inspected.GenerationID != manifest.GenerationID {
		t.Fatalf("structurally valid bytes rejected as a content identity: %v", err)
	}
	document["generationId"] = ""
	malformed, _ := json.Marshal(document)
	if _, err := InspectManifest(malformed); err == nil {
		t.Fatal("missing identity accepted")
	}
}

func TestD06CatalogBindsSelectionWithoutContentHash(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "module.go"), []byte("package fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	record := SourceRecord{Descriptor: module.Descriptor{Module: module.Identity{ID: "fixture/source"}, Source: module.Source{Ref: "file:fixture"}}, Trust: TrustT2, Root: root, Ref: "file:fixture"}
	if _, err := NewSourceCatalog([]SourceRecord{record}); err != nil {
		t.Fatalf("explicit unhashed source refused: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "module.go"), []byte("package changed"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := NewSourceCatalog([]SourceRecord{record}); err != nil {
		t.Fatalf("changed bytes acquired absent hash claim: %v", err)
	}
	if _, err := NewSourceCatalog([]SourceRecord{record, record}); err == nil {
		t.Fatal("duplicate owner accepted")
	}
	record.Ref = "file:another"
	if _, err := NewSourceCatalog([]SourceRecord{record}); err == nil {
		t.Fatal("source ref mismatch accepted")
	}
}

func TestD06UISelectionNeedsNoHashClaims(t *testing.T) {
	result, err := GenerateUIAssembly(UIAssemblyInput{GenerationID: "authoring-test", SDKVersion: UIAssemblySDKVersion, Roots: []UIModule{{ID: "fixture/root", ModuleID: "fixture/root", Entry: "./index", Port: UIRootPort}}})
	if err != nil {
		t.Fatalf("explicit UI source rejected: %v", err)
	}
	for _, retired := range []string{"sourceHashes", "assetHashes", "dependencyLockHashes"} {
		if bytes.Contains(result.Manifest.Canonical(), []byte(retired)) {
			t.Fatalf("retired field emitted: %s", retired)
		}
	}
}

func TestGenerationIDSharedAcrossProjections(t *testing.T) {
	const id = "authoring-projections"
	plan := manifestTestPlan()
	plan.Modules[0].Binding = GoBinding{ImportPath: "example.com/fixture/search", Package: "search", Constructor: "New", ProviderConstructor: "NewProvider"}
	goSource, err := GenerateRuntimeAssembly(plan, "assembly", WithFormIdentity(id))
	if err != nil {
		t.Fatal(err)
	}
	ui, err := GenerateUIAssembly(UIAssemblyInput{GenerationID: id, SDKVersion: UIAssemblySDKVersion})
	if err != nil {
		t.Fatal(err)
	}
	manifest, raw, err := SealManifest(plan, SealInputs{GenerationID: id, SpecificationVersion: "vivy.module/v1", CompilerVersion: "test", SDKVersion: "v1", CanonicalRecipe: []byte(`{}`), UI: &ui.Manifest})
	if err != nil {
		t.Fatal(err)
	}
	inspected, err := InspectManifest(raw)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(goSource, []byte(id)) || !bytes.Contains(ui.Source, []byte(id)) || manifest.GenerationID != id || inspected.GenerationID != id || inspected.UI == nil || inspected.UI.GenerationID != id {
		t.Fatal("Generation identity differs across Go, UI, manifest and Inspect")
	}
}
