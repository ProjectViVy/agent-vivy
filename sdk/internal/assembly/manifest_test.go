package assembly

import (
	"bytes"
	"encoding/json"
	"reflect"
	"testing"

	"agent-vivy/sdk/module"
)

func TestCanonicalRecipeAndManifestAreSemanticByteStable(t *testing.T) {
	firstRecipe := Recipe{
		APIVersion: RecipeAPIVersionV1,
		Profile:    "default",
		Modules:    []string{"fixture/search", "fixture/host"},
		Order:      map[string][]string{"std/ui-extension@v1": {"fixture/search"}},
		GrantApprovals: []GrantApproval{{
			Module: "fixture/search",
			Name:   module.GrantNetClient,
			Constraints: map[string][]string{
				"hosts": {"b.example.com", "a.example.com"},
			},
		}},
	}
	secondRecipe := Recipe{
		APIVersion: RecipeAPIVersionV1,
		Profile:    "default",
		Modules:    []string{"fixture/host", "fixture/search"},
		GrantApprovals: []GrantApproval{{
			Name:   module.GrantNetClient,
			Module: "fixture/search",
			Constraints: map[string][]string{
				"hosts": {"a.example.com", "b.example.com"},
			},
		}},
		Order: map[string][]string{"std/ui-extension@v1": {"fixture/search"}},
	}

	firstCanonical, err := CanonicalRecipe(firstRecipe)
	if err != nil {
		t.Fatal(err)
	}
	secondCanonical, err := CanonicalRecipe(secondRecipe)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(firstCanonical, secondCanonical) {
		t.Fatalf("canonical recipes differ\nfirst: %s\nsecond: %s", firstCanonical, secondCanonical)
	}

	plan := manifestTestPlan()
	inputs := SealInputs{
		SpecificationVersion: "vivy.assembly/v1",
		CompilerVersion:      "compiler-test",
		SDKVersion:           "sdk-test",
		CanonicalRecipe:      firstCanonical,
		DependencyLocks:      map[string]string{"go": "go-lock", "ui": "ui-lock"},
		UIArtifacts:          map[string]string{"fixture/search": "ui-hash"},
	}
	firstManifest, firstBytes, err := SealManifest(plan, inputs)
	if err != nil {
		t.Fatal(err)
	}
	inputs.CanonicalRecipe = secondCanonical
	secondManifest, secondBytes, err := SealManifest(plan, inputs)
	if err != nil {
		t.Fatal(err)
	}
	if firstManifest.GenerationID != secondManifest.GenerationID || !bytes.Equal(firstBytes, secondBytes) {
		t.Fatalf("semantic equivalents changed manifest identity: %s != %s", firstManifest.GenerationID, secondManifest.GenerationID)
	}
}

func TestGenerationIDChangesForEveryIdentityInput(t *testing.T) {
	basePlan := manifestTestPlan()
	baseInputs := SealInputs{
		SpecificationVersion: "vivy.assembly/v1",
		CompilerVersion:      "compiler-a",
		SDKVersion:           "sdk-a",
		CanonicalRecipe:      []byte(`{"apiVersion":"vivy.generation/v1","modules":["fixture/search"]}`),
		DependencyLocks:      map[string]string{"go": "go-lock"},
		UIArtifacts:          map[string]string{"fixture/search": "ui-a"},
		Catalogs: []CatalogManifest{{
			Module: "fixture/search", SchemaVersion: "vivy.i18n/v1", Path: "i18n/catalog.json",
			Digest: "catalog-a", DefaultLocale: "en", Locales: []string{"en", "zh"},
		}},
	}
	base, _, err := SealManifest(basePlan, baseInputs)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name   string
		mutate func(*AssemblyPlan, *SealInputs)
	}{
		{name: "source hash", mutate: func(plan *AssemblyPlan, _ *SealInputs) {
			plan.Modules[0].Descriptor.Source.SHA256 = "ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"
		}},
		{name: "ui hash", mutate: func(_ *AssemblyPlan, inputs *SealInputs) {
			inputs.UIArtifacts["fixture/search"] = "ui-b"
		}},
		{name: "port version", mutate: func(plan *AssemblyPlan, _ *SealInputs) {
			plan.PortEdges[0].Port.Port = "std/tool@v2"
		}},
		{name: "compiler version", mutate: func(_ *AssemblyPlan, inputs *SealInputs) {
			inputs.CompilerVersion = "compiler-b"
		}},
		{name: "resolved dependency build list", mutate: func(_ *AssemblyPlan, inputs *SealInputs) {
			inputs.DependencyLocks["resolved-build-list"] = "go-lock-b"
		}},
		{name: "catalog digest", mutate: func(_ *AssemblyPlan, inputs *SealInputs) {
			inputs.Catalogs[0].Digest = "catalog-b"
		}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			plan := cloneAssemblyPlan(basePlan)
			inputs := cloneSealInputs(baseInputs)
			test.mutate(&plan, &inputs)
			manifest, _, err := SealManifest(plan, inputs)
			if err != nil {
				t.Fatal(err)
			}
			if manifest.GenerationID == base.GenerationID {
				t.Fatalf("GenerationID did not change after %s mutation", test.name)
			}
		})
	}
}

func manifestTestPlan() AssemblyPlan {
	descriptor := testDescriptor("fixture/search")
	descriptor.Provides = []module.PortRef{{Port: "std/tool@v1", ID: "fixture.search"}}
	return AssemblyPlan{
		Modules: []ResolvedModule{{Descriptor: descriptor, Trust: TrustT2}},
		PortEdges: []PortEdge{{
			Port: module.PortRef{Port: "std/tool@v1"}, Provider: "fixture/search", Consumer: "fixture/host",
		}},
		LifecycleOrder: []string{"fixture/search"},
	}
}

func cloneAssemblyPlan(plan AssemblyPlan) AssemblyPlan {
	raw, _ := json.Marshal(plan)
	var cloned AssemblyPlan
	_ = json.Unmarshal(raw, &cloned)
	return cloned
}

func cloneSealInputs(inputs SealInputs) SealInputs {
	raw, _ := json.Marshal(inputs)
	var cloned SealInputs
	_ = json.Unmarshal(raw, &cloned)
	return cloned
}

func TestCapabilityStatesAreClosed(t *testing.T) {
	want := []CapabilityState{
		CapabilityNotCompiled,
		CapabilityUnconfigured,
		CapabilityInactive,
		CapabilityReady,
		CapabilityUnavailable,
		CapabilitySpecified,
		CapabilityDeferred,
	}
	if got := CapabilityStates(); !reflect.DeepEqual(got, want) {
		t.Fatalf("CapabilityStates() = %v, want %v", got, want)
	}
}

func TestInspectManifestVerifiesIdentityAndCapabilityStates(t *testing.T) {
	canonicalRecipe, err := CanonicalRecipe(Recipe{APIVersion: RecipeAPIVersionV1, Modules: []string{"fixture/search"}})
	if err != nil {
		t.Fatal(err)
	}
	inputs := SealInputs{
		SpecificationVersion: "vivy.assembly/v1",
		CompilerVersion:      "compiler-test",
		SDKVersion:           "sdk-test",
		CanonicalRecipe:      canonicalRecipe,
		CapabilityStates: map[string]CapabilityState{
			"std/tool@v1":    CapabilityReady,
			"std/channel@v1": CapabilityUnconfigured,
		},
	}
	sealed, raw, err := SealManifest(manifestTestPlan(), inputs)
	if err != nil {
		t.Fatal(err)
	}
	inspected, err := InspectManifest(raw)
	if err != nil {
		t.Fatalf("InspectManifest() error = %v", err)
	}
	if !reflect.DeepEqual(inspected, sealed) {
		t.Fatalf("inspected manifest differs\n got: %#v\nwant: %#v", inspected, sealed)
	}

	var tampered map[string]any
	if err := json.Unmarshal(raw, &tampered); err != nil {
		t.Fatal(err)
	}
	tampered["compilerVersion"] = "tampered"
	tamperedRaw, err := json.Marshal(tampered)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := InspectManifest(tamperedRaw); err == nil {
		t.Fatal("InspectManifest() accepted a manifest whose sealed inputs changed")
	}
}

func TestSealManifestRejectsUnknownCapabilityState(t *testing.T) {
	inputs := SealInputs{
		SpecificationVersion: "vivy.assembly/v1",
		CompilerVersion:      "compiler-test",
		SDKVersion:           "sdk-test",
		CanonicalRecipe:      []byte(`{"apiVersion":"vivy.generation/v1","modules":[]}`),
		CapabilityStates:     map[string]CapabilityState{"std/tool@v1": "MAGIC"},
	}
	if _, _, err := SealManifest(AssemblyPlan{}, inputs); err == nil {
		t.Fatal("SealManifest() accepted an unknown capability state")
	}
}
