package assembly

import (
	"fmt"
	"sort"
	"strings"

	"agent-vivy/sdk/module"
)

type GrantApproval struct {
	Module      string              `json:"module" yaml:"module"`
	Name        module.Grant        `json:"name" yaml:"name"`
	Constraints map[string][]string `json:"constraints,omitempty" yaml:"constraints,omitempty"`
	Evidence    []string            `json:"evidence,omitempty" yaml:"evidence,omitempty"`
}

type EffectiveGrant struct {
	Name        module.Grant        `json:"name"`
	Constraints map[string][]string `json:"constraints"`
}

func calculateEffectiveGrants(
	moduleID string,
	requested []module.Grant,
	allowed []module.Grant,
	trust Trust,
	approvals []GrantApproval,
) ([]EffectiveGrant, error) {
	allowedSet := make(map[module.Grant]struct{}, len(allowed))
	for _, grant := range allowed {
		allowedSet[grant] = struct{}{}
	}
	approvalByGrant := make(map[module.Grant]GrantApproval)
	for _, approval := range approvals {
		if approval.Module != moduleID {
			continue
		}
		if _, exists := approvalByGrant[approval.Name]; exists {
			return nil, fmt.Errorf("duplicate grant approval %s for %s", approval.Name, moduleID)
		}
		approvalByGrant[approval.Name] = approval
	}

	effective := make([]EffectiveGrant, 0, len(requested))
	seen := make(map[module.Grant]struct{}, len(requested))
	for _, grant := range requested {
		if !grant.Valid() {
			return nil, fmt.Errorf("unknown requested grant %q for %s", grant, moduleID)
		}
		if _, duplicate := seen[grant]; duplicate {
			return nil, fmt.Errorf("duplicate requested grant %q for %s", grant, moduleID)
		}
		seen[grant] = struct{}{}
		if _, ok := allowedSet[grant]; !ok {
			return nil, fmt.Errorf("grant %s is not allowed by selected Ports for %s", grant, moduleID)
		}
		approval, ok := approvalByGrant[grant]
		if !ok {
			return nil, fmt.Errorf("grant %s is not approved for %s", grant, moduleID)
		}
		if trust == TrustT2 && (grant == module.GrantProcSpawn || grant == module.GrantTTY) && !hasBuildEvidence(approval.Evidence) {
			return nil, fmt.Errorf("grant %s exceeds T2 Trust ceiling for %s without conformance evidence", grant, moduleID)
		}
		constraints, err := canonicalConstraints(approval.Constraints)
		if err != nil {
			return nil, fmt.Errorf("grant %s for %s: %w", grant, moduleID, err)
		}
		if err := validateGrantConstraints(grant, constraints); err != nil {
			return nil, fmt.Errorf("grant %s for %s: %w", grant, moduleID, err)
		}
		effective = append(effective, EffectiveGrant{Name: grant, Constraints: constraints})
	}
	sort.Slice(effective, func(i, j int) bool { return effective[i].Name < effective[j].Name })
	return effective, nil
}

func canonicalConstraints(constraints map[string][]string) (map[string][]string, error) {
	canonical := make(map[string][]string, len(constraints))
	for key, values := range constraints {
		if forbiddenConstraintKey(key) {
			return nil, fmt.Errorf("forbidden Secret or environment material in constraint %q", key)
		}
		unique := make(map[string]struct{}, len(values))
		for _, value := range values {
			trimmed := strings.TrimSpace(value)
			lower := strings.ToLower(trimmed)
			if trimmed == "" {
				return nil, fmt.Errorf("empty constraint value for %q", key)
			}
			if strings.Contains(trimmed, "${") || strings.HasPrefix(trimmed, "$") || strings.HasPrefix(lower, "env:") {
				return nil, fmt.Errorf("forbidden Secret or environment material in constraint %q", key)
			}
			unique[trimmed] = struct{}{}
		}
		canonical[key] = sortedSet(unique)
	}
	return canonical, nil
}

func forbiddenConstraintKey(key string) bool {
	lower := strings.ToLower(strings.TrimSpace(key))
	for _, forbidden := range []string{"secret", "token", "password", "credential", "value", "environment", "env"} {
		if lower == forbidden || strings.HasSuffix(lower, "_"+forbidden) {
			return true
		}
	}
	return false
}

func hasBuildEvidence(evidence []string) bool {
	for _, reference := range evidence {
		if strings.HasPrefix(strings.TrimSpace(reference), "build:") {
			return true
		}
	}
	return false
}

func validateGrantConstraints(grant module.Grant, constraints map[string][]string) error {
	allowed := map[module.Grant]map[string]bool{
		module.GrantFSRead:     {"roots": true},
		module.GrantFSWrite:    {"roots": true},
		module.GrantSecretRead: {"names": true},
		module.GrantProcSpawn:  {"commands": true},
		module.GrantNetClient:  {"hosts": true, "ports": true, "schemes": true},
	}
	if keys := allowed[grant]; keys != nil {
		for key := range constraints {
			if !keys[key] {
				return fmt.Errorf("unknown constraint key %q", key)
			}
		}
	} else if len(constraints) != 0 {
		return fmt.Errorf("grant does not accept constraints")
	}
	if grant == module.GrantNetClient {
		if len(constraints["hosts"]) == 0 || len(constraints["schemes"]) == 0 {
			return fmt.Errorf("net.client requires non-empty hosts and schemes constraints")
		}
	}
	return nil
}
