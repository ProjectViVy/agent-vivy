package conformance

import (
	"strings"
	"testing"

	providerconformance "agent-vivy/sdk/conformance"
	"agent-vivy/sdk/module"
	"agent-vivy/sdk/port"
)

func TestUnsupportedPortCannotClaimSupported(t *testing.T) {
	evidence := completeEvidence("std/tool@v1")
	records, err := EvaluatePortSupport(port.PublicCatalog(), map[string]port.SupportEvidence{
		"std/tool@v1": evidence,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := supportState(records, "std/tool@v1"); got == port.SupportSupported {
		t.Fatalf("descriptor/build claims promoted Port without conformance: %s", got)
	}
}

func TestPortSupportRequiresPassingMatchingConformance(t *testing.T) {
	evidence := completeEvidence("std/tool@v1")
	results := completeProviderResults("std/tool@v1", "vivy/protected-tools", evidence.References[5].ID)
	partial := results[:1]
	records, err := EvaluatePortSupport(port.PublicCatalog(), map[string]port.SupportEvidence{
		"std/tool@v1": evidence,
	}, partial)
	if err != nil {
		t.Fatal(err)
	}
	if got := supportState(records, "std/tool@v1"); got != port.SupportCandidate {
		t.Fatalf("partial conformance state = %s, want %s", got, port.SupportCandidate)
	}

	records, err = EvaluatePortSupport(port.PublicCatalog(), map[string]port.SupportEvidence{
		"std/tool@v1": evidence,
	}, results)
	if err != nil {
		t.Fatal(err)
	}
	if got := supportState(records, "std/tool@v1"); got != port.SupportSupported {
		t.Fatalf("complete conformance state = %s, want %s", got, port.SupportSupported)
	}

	results[len(results)-1].Passed = false
	records, err = EvaluatePortSupport(port.PublicCatalog(), map[string]port.SupportEvidence{
		"std/tool@v1": evidence,
	}, results)
	if err != nil {
		t.Fatal(err)
	}
	if got := supportState(records, "std/tool@v1"); got != port.SupportCandidate {
		t.Fatalf("failed conformance state = %s, want %s", got, port.SupportCandidate)
	}
}

func completeProviderResults(portID, providerID, evidenceID string) []providerconformance.ConformanceResult {
	results := make([]providerconformance.ConformanceResult, 0, len(providerconformance.RequiredProviderChecks()))
	for _, check := range providerconformance.RequiredProviderChecks() {
		results = append(results, providerconformance.ConformanceResult{
			Port: module.PortRef{Port: portID}, ProviderID: providerID,
			SourceSHA256: strings.Repeat("a", 64), Suite: check, Passed: true, EvidenceID: evidenceID,
		})
	}
	return results
}

func completeEvidence(portID string) port.SupportEvidence {
	references := make([]port.EvidenceReference, 0, len(port.RequiredEvidenceKinds()))
	for _, kind := range port.RequiredEvidenceKinds() {
		references = append(references, port.EvidenceReference{Kind: kind, ID: portID + "/" + string(kind)})
	}
	return port.SupportEvidence{References: references, GatesPassed: true}
}

func supportState(records []PortSupportRecord, portID string) port.SupportState {
	for _, record := range records {
		if record.Port.Port == portID {
			return record.State
		}
	}
	return ""
}
