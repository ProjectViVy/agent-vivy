package assembly

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	providerconformance "agent-vivy/sdk/conformance"
	"agent-vivy/sdk/module"
)

func TestSupportedPortEvidenceReferencesRepositoryFiles(t *testing.T) {
	for portID, evidence := range SupportedPortEvidence() {
		for _, reference := range evidence.References {
			path, _, ok := strings.Cut(reference.ID, "#")
			if !ok || path == "" {
				t.Fatalf("%s %s evidence has no file anchor: %q", portID, reference.Kind, reference.ID)
			}
			body, err := os.ReadFile(filepath.Join("..", "..", "..", path))
			if err != nil {
				t.Fatalf("%s %s evidence path %q: %v", portID, reference.Kind, path, err)
			}
			_, anchor, _ := strings.Cut(reference.ID, "#")
			if !bytes.Contains(body, []byte(anchor)) {
				t.Fatalf("%s %s evidence anchor is absent from %q: %q", portID, reference.Kind, path, reference.ID)
			}
		}
	}
}

func TestProviderConformanceBundleReferencesRepositoryAnchors(t *testing.T) {
	for _, result := range SupportedPortConformance() {
		path, anchor, ok := strings.Cut(result.EvidenceID, "#")
		if !ok || path == "" || anchor == "" {
			t.Fatalf("%s %s has no repository evidence anchor: %q", result.Port.Port, result.ProviderID, result.EvidenceID)
		}
		body, err := os.ReadFile(filepath.Join("..", "..", "..", path))
		if err != nil {
			t.Fatalf("%s %s evidence path %q: %v", result.Port.Port, result.ProviderID, path, err)
		}
		if !bytes.Contains(body, []byte(anchor)) {
			t.Fatalf("%s %s evidence anchor is absent from %q: %q", result.Port.Port, result.ProviderID, path, result.EvidenceID)
		}
	}
}

func TestConformanceResultsForPlanNeverRelabelsProviderEvidence(t *testing.T) {
	plan := AssemblyPlan{Modules: []ResolvedModule{{Descriptor: module.Descriptor{
		Module:   module.Identity{ID: "fixture/provider", Version: "1.0.0"},
		Source:   module.Source{Ref: "test:fixture/provider", SHA256: strings.Repeat("a", 64)},
		Provides: []module.PortRef{{Port: "std/tool@v1", ID: "fixture.tool"}},
	}}}}
	if results, err := ConformanceResultsForPlan(plan); err != nil {
		t.Fatal(err)
	} else if len(results) != 0 {
		t.Fatalf("unattested Provider acquired relabelled release evidence: %#v", results)
	}
	minimal, err := ConformanceResultsForPlan(AssemblyPlan{})
	if err != nil {
		t.Fatal(err)
	}
	if len(minimal) != 0 {
		t.Fatalf("minimal plan retained conformance Providers: %#v", minimal)
	}
}

func TestConformanceResultsForPlanIncludesImplicitUIAndActionProviders(t *testing.T) {
	plan := AssemblyPlan{Modules: []ResolvedModule{{Descriptor: module.Descriptor{
		Module: module.Identity{ID: "fixture/full-ui", Version: "1.0.0"},
		Source: module.Source{Ref: "file:full-ui-module", SHA256: "c106d108fd1510eba19351b3eed7d87338fe777188d75ed942db61443d72b8cc"},
		Provides: []module.PortRef{
			{Port: UIRootPort, ID: "fixture.full-ui.root"},
			{Port: UIExtensionPort, ID: "fixture.full-ui.extension"},
			{Port: "std/control-action@v1", ID: "fixture.full-ui.echo"},
		},
	}}}}
	results, err := ConformanceResultsForPlan(plan)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 3*len(providerconformance.RequiredProviderChecks()) {
		t.Fatalf("selected full-UI conformance results = %#v", results)
	}
	for _, result := range results {
		if result.ProviderID != "fixture/full-ui" || !result.Passed {
			t.Fatalf("selected full-UI conformance result = %#v", result)
		}
	}
}

func TestConformanceResultsForPlanRejectsStaleSourceEvidence(t *testing.T) {
	plan := AssemblyPlan{Modules: []ResolvedModule{{Descriptor: module.Descriptor{
		Module:   module.Identity{ID: "fixture/full-ui", Version: "1.0.0"},
		Source:   module.Source{Ref: "file:full-ui-module", SHA256: strings.Repeat("f", 64)},
		Provides: []module.PortRef{{Port: UIRootPort, ID: "fixture.full-ui.root"}},
	}}}}
	if results, err := ConformanceResultsForPlan(plan); err != nil {
		t.Fatal(err)
	} else if len(results) != 0 {
		t.Fatalf("stale source acquired conformance evidence: %#v", results)
	}
}
