package port

import (
	"strings"
	"testing"

	"agent-vivy/sdk/module"
)

func TestDescriptorCannotClaimSupport(t *testing.T) {
	descriptor := module.Descriptor{
		APIVersion: module.APIVersionV1,
		Provides:   []module.PortRef{{Port: "std/tool@v1", ID: "example.search"}},
	}

	err := PublicCatalog().RequireSelectable(descriptor.Provides[0], SupportEvidence{})
	if err == nil || !strings.Contains(err.Error(), "std/tool@v1 is SPECIFIED") {
		t.Fatalf("RequireSelectable() error = %v, want build-evidence rejection", err)
	}
}

func TestSupportStateRequiresBuildOwnedEvidence(t *testing.T) {
	definition, ok := PublicCatalog().Lookup(module.PortRef{Port: "std/tool@v1"})
	if !ok {
		t.Fatal("std/tool@v1 missing from catalog")
	}

	references := make([]EvidenceReference, 0, len(RequiredEvidenceKinds()))
	for _, kind := range RequiredEvidenceKinds() {
		references = append(references, EvidenceReference{Kind: kind, ID: "evidence/" + string(kind)})
	}

	tests := []struct {
		name     string
		evidence SupportEvidence
		want     SupportState
	}{
		{name: "normative definition only", want: SupportSpecified},
		{name: "seven artifacts under validation", evidence: SupportEvidence{References: references}, want: SupportCandidate},
		{name: "all gates pass", evidence: SupportEvidence{References: references, GatesPassed: true}, want: SupportSupported},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := EvaluateSupport(definition, test.evidence); got != test.want {
				t.Fatalf("EvaluateSupport() = %q, want %q", got, test.want)
			}
		})
	}
}
