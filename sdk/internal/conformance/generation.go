// Package conformance aggregates build-owned Port evidence and executable
// conformance results into the support records sealed by a Generation.
package conformance

import (
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	providerconformance "agent-vivy/sdk/conformance"
	"agent-vivy/sdk/module"
	"agent-vivy/sdk/port"
)

var opaqueEvidenceIDPattern = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)

// PortSupportRecord is the Inspect projection for one public Port. EvidenceIDs
// are repository-relative or opaque identifiers; failure output and local
// paths are never part of this record.
type PortSupportRecord struct {
	Port        module.PortRef    `json:"port"`
	State       port.SupportState `json:"state"`
	EvidenceIDs []string          `json:"evidenceIds,omitempty"`
}

// EvaluatePortSupport applies the P1 seven-artifact rule and then requires a
// passing machine result bound to the declared conformance-suite evidence.
// Descriptor or documentation claims are deliberately not inputs.
func EvaluatePortSupport(catalog port.Catalog, evidence map[string]port.SupportEvidence, results []providerconformance.ConformanceResult) ([]PortSupportRecord, error) {
	canonicalResults, err := providerconformance.CanonicalResults(results)
	if err != nil {
		return nil, err
	}
	knownPorts := make(map[string]struct{})
	for _, definition := range catalog.Definitions() {
		knownPorts[definition.Ref.Port] = struct{}{}
	}
	for _, result := range canonicalResults {
		if _, ok := knownPorts[result.Port.Port]; !ok {
			return nil, fmt.Errorf("conformance: result references unknown Port %s", result.Port.Port)
		}
		if !safeEvidenceID(result.EvidenceID) {
			return nil, fmt.Errorf("conformance: %s is not a safe repository-relative or opaque identifier", result.EvidenceID)
		}
	}

	records := make([]PortSupportRecord, 0, len(knownPorts))
	for _, definition := range catalog.Definitions() {
		proof := evidence[definition.Ref.Port]
		conformanceEvidenceID := ""
		evidenceIDs := make([]string, 0, len(proof.References))
		for _, reference := range proof.References {
			if reference.ID != "" {
				if !safeEvidenceID(reference.ID) {
					return nil, fmt.Errorf("conformance: %s is not a safe repository-relative or opaque identifier", reference.ID)
				}
				evidenceIDs = append(evidenceIDs, reference.ID)
			}
			if reference.Kind == port.EvidenceConformanceSuite {
				conformanceEvidenceID = reference.ID
			}
		}
		sort.Strings(evidenceIDs)
		proof.GatesPassed = conformanceEvidenceID != "" && hasCompletePassingProviderSuite(canonicalResults, definition.Ref.Port)
		state := port.EvaluateSupport(definition, proof)
		records = append(records, PortSupportRecord{Port: definition.Ref, State: state, EvidenceIDs: evidenceIDs})
	}
	return records, nil
}

// CanonicalPortSupport validates and sorts the release-wide Inspect records.
func CanonicalPortSupport(records []PortSupportRecord) ([]PortSupportRecord, error) {
	canonical := append([]PortSupportRecord(nil), records...)
	seen := make(map[string]struct{}, len(canonical))
	validStates := map[port.SupportState]struct{}{
		port.SupportReserved: {}, port.SupportSpecified: {}, port.SupportCandidate: {},
		port.SupportSupported: {}, port.SupportDeferredIndefinite: {},
	}
	for index := range canonical {
		record := &canonical[index]
		if strings.TrimSpace(record.Port.Port) == "" {
			return nil, fmt.Errorf("conformance: Port support record requires a Port")
		}
		if _, ok := validStates[record.State]; !ok {
			return nil, fmt.Errorf("conformance: unknown support state %q for %s", record.State, record.Port.Port)
		}
		if _, ok := seen[record.Port.Port]; ok {
			return nil, fmt.Errorf("conformance: duplicate Port support record for %s", record.Port.Port)
		}
		seen[record.Port.Port] = struct{}{}
		for _, evidenceID := range record.EvidenceIDs {
			if !safeEvidenceID(evidenceID) {
				return nil, fmt.Errorf("conformance: %s is not a safe repository-relative or opaque identifier", evidenceID)
			}
		}
		record.EvidenceIDs = canonicalStrings(record.EvidenceIDs)
	}
	sort.Slice(canonical, func(i, j int) bool { return canonical[i].Port.Port < canonical[j].Port.Port })
	return canonical, nil
}

func safeEvidenceID(id string) bool {
	if opaqueEvidenceIDPattern.MatchString(id) {
		return true
	}
	if id == "" || filepath.IsAbs(id) || !filepath.IsLocal(id) || strings.Contains(id, `\`) {
		return false
	}
	for _, character := range id {
		if character < 0x20 || character == 0x7f {
			return false
		}
	}
	return true
}

func canonicalStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		seen[value] = struct{}{}
	}
	out := make([]string, 0, len(seen))
	for value := range seen {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func hasCompletePassingProviderSuite(results []providerconformance.ConformanceResult, portID string) bool {
	type providerKey struct {
		providerID, sourceSHA256 string
	}
	groups := make(map[providerKey]map[string]bool)
	for _, result := range results {
		if result.Port.Port != portID {
			continue
		}
		key := providerKey{providerID: result.ProviderID, sourceSHA256: result.SourceSHA256}
		if groups[key] == nil {
			groups[key] = make(map[string]bool)
		}
		groups[key][result.Suite] = result.Passed
	}
	for _, checks := range groups {
		complete := true
		for _, required := range providerconformance.RequiredProviderChecks() {
			if !checks[required] {
				complete = false
				break
			}
		}
		if complete {
			return true
		}
	}
	return false
}
