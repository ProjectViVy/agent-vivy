package sdk

import (
	"path/filepath"
	"strings"
	"testing"

	assemblyv1 "agent-vivy/sdk/internal/assembly"
	generationconformance "agent-vivy/sdk/internal/conformance"
	"agent-vivy/sdk/port"
)

func TestSupportStateRequiresEveryArtifactForEveryPublicPort(t *testing.T) {
	catalog := port.PublicCatalog()
	evidence := assemblyv1.SupportedPortEvidence()
	results := assemblyv1.SupportedPortConformance()
	records, err := generationconformance.EvaluatePortSupport(catalog, evidence, results)
	if err != nil {
		t.Fatal(err)
	}
	for _, definition := range catalog.Definitions() {
		if got := releaseSupportState(records, definition.Ref.Port); got != port.SupportSupported {
			t.Fatalf("release state for %s = %s, want %s", definition.Ref.Port, got, port.SupportSupported)
		}
		for _, missing := range port.RequiredEvidenceKinds() {
			proof := evidence[definition.Ref.Port]
			filtered := proof.References[:0:0]
			for _, reference := range proof.References {
				if reference.Kind != missing {
					filtered = append(filtered, reference)
				}
			}
			proof.References = filtered
			without := cloneSupportEvidence(evidence)
			without[definition.Ref.Port] = proof
			recomputed, evaluateErr := generationconformance.EvaluatePortSupport(catalog, without, results)
			if evaluateErr != nil {
				t.Fatal(evaluateErr)
			}
			if got := releaseSupportState(recomputed, definition.Ref.Port); got == port.SupportSupported {
				t.Fatalf("%s remained SUPPORTED without %s", definition.Ref.Port, missing)
			}
		}
	}
}

func TestSupportStateRejectsUnsafeEvidenceIdentifiers(t *testing.T) {
	evidence := assemblyv1.SupportedPortEvidence()
	proof := evidence["std/tool@v1"]
	proof.References[0].ID = filepath.Join(string(filepath.Separator), "home", "developer", "secret-token")
	evidence["std/tool@v1"] = proof

	_, err := generationconformance.EvaluatePortSupport(port.PublicCatalog(), evidence, assemblyv1.SupportedPortConformance())
	if err == nil || !strings.Contains(err.Error(), "safe repository-relative or opaque identifier") {
		t.Fatalf("EvaluatePortSupport() error = %v, want unsafe evidence rejection", err)
	}
}

func TestSupportStateAndConformanceAreSealedForInspect(t *testing.T) {
	evidence := assemblyv1.SupportedPortEvidence()
	results := assemblyv1.SupportedPortConformance()
	records, err := generationconformance.EvaluatePortSupport(port.PublicCatalog(), evidence, results)
	if err != nil {
		t.Fatal(err)
	}
	canonicalRecipe, err := assemblyv1.CanonicalRecipe(assemblyv1.Recipe{APIVersion: assemblyv1.RecipeAPIVersionV1})
	if err != nil {
		t.Fatal(err)
	}
	sealed, raw, err := assemblyv1.SealManifest(assemblyv1.AssemblyPlan{}, assemblyv1.SealInputs{
		SpecificationVersion: "vivy.module/v1", CompilerVersion: "plg-p9", SDKVersion: "v1",
		CanonicalRecipe: canonicalRecipe, ConformanceResults: results, PortSupport: records,
	})
	if err != nil {
		t.Fatal(err)
	}
	inspected, err := assemblyv1.InspectManifest(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(inspected.PortSupport) != len(port.PublicCatalog().Definitions()) || len(inspected.ConformanceResults) != len(results) {
		t.Fatalf("Inspect evidence counts = support %d, results %d", len(inspected.PortSupport), len(inspected.ConformanceResults))
	}
	if inspected.GenerationID != sealed.GenerationID {
		t.Fatal("Inspect changed sealed support evidence identity")
	}
}

func releaseSupportState(records []generationconformance.PortSupportRecord, portID string) port.SupportState {
	for _, record := range records {
		if record.Port.Port == portID {
			return record.State
		}
	}
	return ""
}

func cloneSupportEvidence(source map[string]port.SupportEvidence) map[string]port.SupportEvidence {
	cloned := make(map[string]port.SupportEvidence, len(source))
	for portID, proof := range source {
		proof.References = append([]port.EvidenceReference(nil), proof.References...)
		cloned[portID] = proof
	}
	return cloned
}
