// Package conformance defines deterministic, machine-readable results for
// public Port Provider/Consumer contract suites. Port-specific semantics stay
// with the Host that owns them; this package only standardizes the common
// release checks and their sealed evidence records.
package conformance

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"agent-vivy/sdk/module"
	"agent-vivy/sdk/port"
)

const (
	CheckRegistration      = "registration"
	CheckMissingProvider   = "missing-provider"
	CheckDuplicateProvider = "duplicate-provider"
	CheckVersion           = "version"
	CheckCycle             = "cycle"
	CheckGrant             = "grant"
	CheckTimeout           = "timeout"
	CheckCancellation      = "cancellation"
	CheckStartup           = "startup"
	CheckUnavailable       = "unavailable"
	CheckCleanup           = "cleanup"
	CheckRedaction         = "redaction"
	CheckProvenance        = "provenance"
	CheckDefault           = "default"
	CheckRealFailure       = "real-failure"
)

var requiredProviderChecks = []string{
	CheckRegistration,
	CheckMissingProvider,
	CheckDuplicateProvider,
	CheckVersion,
	CheckCycle,
	CheckGrant,
	CheckTimeout,
	CheckCancellation,
	CheckStartup,
	CheckUnavailable,
	CheckCleanup,
	CheckRedaction,
	CheckProvenance,
	CheckDefault,
	CheckRealFailure,
}

// ConformanceResult is the immutable evidence unit embedded in a Generation
// Manifest. Failure details are intentionally excluded so Secrets exercised by
// negative-path checks cannot leak into Inspect output.
type ConformanceResult struct {
	Port       module.PortRef `json:"port"`
	ProviderID string         `json:"providerId"`
	Suite      string         `json:"suite"`
	Passed     bool           `json:"passed"`
	EvidenceID string         `json:"evidenceId"`
}

// Check executes one assertion owned by a focused Host conformance suite. A
// nil error means the assertion passed, including checks that prove a real
// provider failure is handled correctly.
type Check func(context.Context) error

func RequiredProviderChecks() []string {
	return append([]string(nil), requiredProviderChecks...)
}

// RunProviderSuite executes the common checks in a closed, deterministic
// order. Callers supply executable Provider, Host, compiler, and lifecycle
// behavior through Check closures; this keeps the reusable harness independent
// of Port-specific SDK interfaces.
func RunProviderSuite(ctx context.Context, definition port.Definition, providerID, evidenceID string, checks map[string]Check) ([]ConformanceResult, error) {
	if strings.TrimSpace(definition.Ref.Port) == "" {
		return nil, fmt.Errorf("conformance: Port Definition is required")
	}
	if strings.TrimSpace(providerID) == "" {
		return nil, fmt.Errorf("conformance: provider ID is required for %s", definition.Ref.Port)
	}
	path, anchor, ok := strings.Cut(evidenceID, "#")
	if !ok || strings.TrimSpace(path) == "" || strings.TrimSpace(anchor) == "" || filepath.IsAbs(path) || strings.HasPrefix(path, "/") || strings.ContainsAny(path, "\\:") || strings.Contains(path, "..") {
		return nil, fmt.Errorf("conformance: Provider %s requires a repository-relative evidence path and anchor", providerID)
	}
	required := make(map[string]struct{}, len(requiredProviderChecks))
	for _, name := range requiredProviderChecks {
		required[name] = struct{}{}
		if checks[name] == nil {
			return nil, fmt.Errorf("conformance: %s Provider %s is missing common check %s", definition.Ref.Port, providerID, name)
		}
	}
	for name := range checks {
		if _, ok := required[name]; !ok {
			return nil, fmt.Errorf("conformance: %s Provider %s has unknown common check %s", definition.Ref.Port, providerID, name)
		}
	}

	results := make([]ConformanceResult, 0, len(requiredProviderChecks))
	for _, name := range requiredProviderChecks {
		var checkErr error
		if err := ctx.Err(); err != nil {
			checkErr = err
		} else {
			checkErr = checks[name](ctx)
		}
		results = append(results, ConformanceResult{
			Port:       definition.Ref,
			ProviderID: providerID,
			Suite:      name,
			Passed:     checkErr == nil,
			EvidenceID: evidenceID,
		})
	}
	return results, nil
}

// CanonicalResults validates and sorts result records for stable Manifest
// identity. A Port/provider/suite tuple can appear at most once.
func CanonicalResults(results []ConformanceResult) ([]ConformanceResult, error) {
	canonical := append([]ConformanceResult(nil), results...)
	seen := make(map[string]struct{}, len(canonical))
	for _, result := range canonical {
		if strings.TrimSpace(result.Port.Port) == "" || strings.TrimSpace(result.ProviderID) == "" || strings.TrimSpace(result.Suite) == "" || strings.TrimSpace(result.EvidenceID) == "" {
			return nil, fmt.Errorf("conformance: result requires Port, provider, suite, and evidence ID")
		}
		key := result.Port.Port + "\x00" + result.Port.ID + "\x00" + result.ProviderID + "\x00" + result.Suite
		if _, ok := seen[key]; ok {
			return nil, fmt.Errorf("conformance: duplicate result for %s Provider %s suite %s", result.Port.Port, result.ProviderID, result.Suite)
		}
		seen[key] = struct{}{}
	}
	sort.Slice(canonical, func(i, j int) bool {
		left, right := canonical[i], canonical[j]
		if left.Port.Port != right.Port.Port {
			return left.Port.Port < right.Port.Port
		}
		if left.Port.ID != right.Port.ID {
			return left.Port.ID < right.Port.ID
		}
		if left.ProviderID != right.ProviderID {
			return left.ProviderID < right.ProviderID
		}
		return left.Suite < right.Suite
	})
	return canonical, nil
}
