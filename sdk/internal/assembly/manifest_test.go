package assembly

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"agent-vivy/sdk/module"
	"agent-vivy/sdk/port"
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

func TestGenerationIDChangesWhenEmbeddedCatalogBodyChanges(t *testing.T) {
	catalog := CatalogManifest{
		Module:        "fixture/search",
		APIVersion:    "vivy.i18n/v1",
		SchemaVersion: "vivy.i18n/v1",
		Path:          "i18n/catalog.json",
		DefaultLocale: "en",
		Locales:       []string{"en", "zh"},
		Units: map[string]CatalogUnit{
			"plugin.fixture/search.title": {
				Description:  "Title",
				Placeholders: []string{},
				Messages:     map[string]string{"en": "Search", "zh": "搜索"},
			},
		},
	}
	catalog.Digest = catalogProjectionDigest(catalog)
	canonicalRecipe := []byte(`{"apiVersion":"vivy.generation/v1","modules":["fixture/search"]}`)
	seal := func(c CatalogManifest) GenerationManifest {
		manifest, _, err := SealManifest(manifestTestPlan(), SealInputs{
			SpecificationVersion: "vivy.assembly/v1",
			CompilerVersion:      "compiler-test",
			SDKVersion:           "sdk-test",
			CanonicalRecipe:      canonicalRecipe,
			UI: &UIAssemblyManifest{
				SDKPackage: UIAssemblySDKPackageName,
				SDKVersion: UIAssemblySDKVersion,
				Catalogs:   []CatalogManifest{c},
			},
			Catalogs: []CatalogManifest{c},
		})
		if err != nil {
			t.Fatal(err)
		}
		return manifest
	}
	first := seal(catalog)
	catalog.Units["plugin.fixture/search.title"].Messages["en"] = "Search results"
	catalog.Digest = catalogProjectionDigest(catalog)
	second := seal(catalog)
	if first.GenerationID == second.GenerationID {
		t.Fatalf("GenerationID did not change when embedded catalog body changed: %s", first.GenerationID)
	}
}

func TestSealManifestCanonicalizesUIProjectionCatalogOrder(t *testing.T) {
	catalogA := CatalogManifest{
		Module:        "fixture/catalog-a",
		APIVersion:    "vivy.i18n/v1",
		SchemaVersion: "vivy.i18n/v1",
		Path:          "i18n/catalog.json",
		DefaultLocale: "en",
		Locales:       []string{"zh", "en"},
		Evidence:      []string{"INCOMPLETE_LOCALE", "COMPLETE"},
		Units: map[string]CatalogUnit{
			"plugin.fixture/catalog-a.title": {
				Description:  "Title A",
				Placeholders: []string{},
				Messages:     map[string]string{"zh": "标题 A", "en": "Title A"},
			},
		},
	}
	catalogA.Digest = catalogProjectionDigest(catalogA)
	catalogB := CatalogManifest{
		Module:        "fixture/catalog-b",
		APIVersion:    "vivy.i18n/v1",
		SchemaVersion: "vivy.i18n/v1",
		Path:          "i18n/catalog.json",
		DefaultLocale: "en",
		Locales:       []string{"en"},
		Units: map[string]CatalogUnit{
			"plugin.fixture/catalog-b.title": {
				Description:  "Title B",
				Placeholders: []string{},
				Messages:     map[string]string{"en": "Title B"},
			},
		},
	}
	catalogB.Digest = catalogProjectionDigest(catalogB)
	canonicalRecipe := []byte(`{"apiVersion":"vivy.generation/v1","modules":["fixture/search"]}`)
	seal := func(catalogs []CatalogManifest) (GenerationManifest, []byte) {
		manifest, raw, err := SealManifest(manifestTestPlan(), SealInputs{
			SpecificationVersion: "vivy.assembly/v1",
			CompilerVersion:      "compiler-test",
			SDKVersion:           "sdk-test",
			CanonicalRecipe:      canonicalRecipe,
			UI: &UIAssemblyManifest{
				SDKPackage: UIAssemblySDKPackageName,
				SDKVersion: UIAssemblySDKVersion,
				Catalogs:   catalogs,
			},
			Catalogs: catalogs,
		})
		if err != nil {
			t.Fatal(err)
		}
		return manifest, raw
	}
	first, firstRaw := seal([]CatalogManifest{catalogB, catalogA})
	second, secondRaw := seal([]CatalogManifest{catalogA, catalogB})
	if first.GenerationID != second.GenerationID || !bytes.Equal(firstRaw, secondRaw) {
		t.Fatalf("UI catalog order changed sealed identity: %s != %s\nfirst: %s\nsecond: %s", first.GenerationID, second.GenerationID, firstRaw, secondRaw)
	}
	inspected, err := InspectManifest(firstRaw)
	if err != nil {
		t.Fatalf("InspectManifest() error = %v", err)
	}
	if got := inspected.UI.Catalogs[0].Module; got != "fixture/catalog-a" {
		t.Fatalf("InspectManifest() UI catalog order = %q, want fixture/catalog-a first", got)
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

func TestTask7P6PortsHaveCompleteSevenArtifactProof(t *testing.T) {
	evidence := P6UIPortEvidence()
	public := port.PublicCatalog()
	repositoryRoot, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	required := port.RequiredEvidenceKinds()
	for _, portName := range []string{UIExtensionPort, UIRootPort, "std/control-action@v1"} {
		t.Run(portName, func(t *testing.T) {
			definition, ok := public.Lookup(module.PortRef{Port: portName})
			if !ok {
				t.Fatalf("public Port catalog omitted %s", portName)
			}
			proof, ok := evidence[portName]
			if !ok {
				t.Fatalf("P6 evidence ledger omitted %s", portName)
			}
			if got := port.EvaluateSupport(definition, proof); got != port.SupportSupported {
				t.Fatalf("P6 evidence for %s evaluates to %s, want SUPPORTED", portName, got)
			}
			if len(proof.References) != len(required) {
				t.Fatalf("P6 evidence for %s has %d references, want exactly %d", portName, len(proof.References), len(required))
			}
			seen := make(map[port.EvidenceKind]string, len(proof.References))
			for _, reference := range proof.References {
				if reference.ID == "" {
					t.Fatalf("P6 evidence for %s has empty %s reference", portName, reference.Kind)
				}
				if previous, duplicate := seen[reference.Kind]; duplicate {
					t.Fatalf("P6 evidence for %s repeats %s (%q and %q)", portName, reference.Kind, previous, reference.ID)
				}
				seen[reference.Kind] = reference.ID
				pathPart, _, foundAnchor := strings.Cut(reference.ID, "#")
				if !foundAnchor || pathPart == "" {
					t.Fatalf("P6 evidence for %s has no source anchor: %q", portName, reference.ID)
				}
				info, statErr := os.Stat(filepath.Join(repositoryRoot, filepath.FromSlash(pathPart)))
				if statErr != nil || info.IsDir() {
					t.Fatalf("P6 evidence for %s points to unavailable artifact %q: %v", portName, pathPart, statErr)
				}
			}
			for _, kind := range required {
				if _, ok := seen[kind]; !ok {
					t.Fatalf("P6 evidence for %s is missing required %s artifact", portName, kind)
				}
			}
		})
	}
}

func TestTask7UIProvenanceBindsHashAndInspectIdentity(t *testing.T) {
	ui := UIAssemblyManifest{
		Root:                 "fixture/full-ui-root",
		Extensions:           []string{"fixture/full-ui-extension"},
		Replacements:         map[string][]string{"fixture/full-ui-extension": {"fixture/old-extension"}},
		SourceHashes:         map[string]string{"fixture/full-ui-root": strings.Repeat("a", 64), "fixture/full-ui-extension": strings.Repeat("b", 64)},
		DependencyLockHashes: map[string]string{"fixture/full-ui-root": strings.Repeat("c", 64), "fixture/full-ui-extension": strings.Repeat("d", 64)},
		SDKPackage:           UIAssemblySDKPackageName,
		SDKVersion:           UIAssemblySDKVersion,
		AssetHashes:          map[string]string{"fixture/full-ui-root": strings.Repeat("e", 64), "fixture/full-ui-extension": strings.Repeat("f", 64)},
	}
	seal := func(projection UIAssemblyManifest) (GenerationManifest, []byte) {
		t.Helper()
		manifest, raw, err := SealManifest(AssemblyPlan{}, SealInputs{
			SpecificationVersion: "vivy.assembly/v1",
			CompilerVersion:      "task7-provenance-compiler",
			SDKVersion:           UIAssemblySDKVersion,
			CanonicalRecipe:      []byte(`{"apiVersion":"vivy.generation/v1","profile":"task7","modules":["fixture/full-ui"]}`),
			DependencyLocks:      map[string]string{"fixture/full-ui": strings.Repeat("1", 64)},
			UIArtifacts:          map[string]string{"ui/dist": strings.Repeat("2", 64)},
			UI:                   &projection,
		})
		if err != nil {
			t.Fatal(err)
		}
		return manifest, raw
	}
	first, raw := seal(ui)
	inspected, err := InspectManifest(raw)
	if err != nil {
		t.Fatalf("InspectManifest() error = %v", err)
	}
	if inspected.GenerationID != first.GenerationID || inspected.UI == nil || inspected.UI.Root != ui.Root || !equalStrings(inspected.UI.Extensions, ui.Extensions) || !reflect.DeepEqual(inspected.UI.SourceHashes, ui.SourceHashes) || !reflect.DeepEqual(inspected.UI.DependencyLockHashes, ui.DependencyLockHashes) || !reflect.DeepEqual(inspected.UI.AssetHashes, ui.AssetHashes) || !reflect.DeepEqual(inspected.UI.Replacements, ui.Replacements) {
		t.Fatalf("InspectManifest() lost sealed UI provenance: %#v", inspected.UI)
	}

	mutated := ui
	mutated.SourceHashes = map[string]string{"fixture/full-ui-root": strings.Repeat("9", 64), "fixture/full-ui-extension": strings.Repeat("b", 64)}
	second, _ := seal(mutated)
	if first.GenerationID == second.GenerationID {
		t.Fatal("changing a selected UI source hash did not change GenerationID")
	}
}

func TestTask7MinimalManifestProvesLegacyUIPathsAreAbsent(t *testing.T) {
	minimal, err := GenerateUIAssembly(UIAssemblyInput{SDKVersion: UIAssemblySDKVersion})
	if err != nil {
		t.Fatal(err)
	}
	manifest, raw, err := SealManifest(AssemblyPlan{}, SealInputs{
		SpecificationVersion: "vivy.assembly/v1",
		CompilerVersion:      "task7-minimal-compiler",
		SDKVersion:           UIAssemblySDKVersion,
		CanonicalRecipe:      []byte(`{"apiVersion":"vivy.generation/v1","profile":"minimal","modules":[]}`),
		UIArtifacts:          map[string]string{"ui/dist": strings.Repeat("0", 64)},
		UI:                   &minimal.Manifest,
	})
	if err != nil {
		t.Fatal(err)
	}
	inspected, err := InspectManifest(raw)
	if err != nil {
		t.Fatalf("InspectManifest() error = %v", err)
	}
	if inspected.GenerationID != manifest.GenerationID || inspected.UI == nil {
		t.Fatalf("minimal manifest could not be inspected: %#v", inspected)
	}
	if inspected.UI.Root != "" || len(inspected.UI.Extensions) != 0 || len(inspected.UI.SourceHashes) != 0 || len(inspected.UI.DependencyLockHashes) != 0 || len(inspected.UI.AssetHashes) != 0 {
		t.Fatalf("minimal UI projection retained selected state: %#v", inspected.UI)
	}
	sealed := string(raw) + "\n" + string(minimal.Source)
	for _, legacyPath := range []string{"vivy.plugin/" + "v0", "vivy.generation/v0", "ui.full", "requestPermission", "permission-dialog", "legacy-plugin-route"} {
		if strings.Contains(sealed, legacyPath) {
			t.Fatalf("minimal UI Generation retained legacy path %q", legacyPath)
		}
	}
}
