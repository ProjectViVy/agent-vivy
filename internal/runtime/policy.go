package runtime

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"agent-vivy/internal/domain"
)

var (
	ErrInvalidPolicyProfile = errors.New("runtime: invalid policy profile")
	ErrPolicyDenied         = errors.New("runtime: policy denied tool")
)

// PolicyRule is the runtime form of one configuration rule. A rule without a
// field matches the tool itself; field rules match string command/path args.
type PolicyRule struct {
	Tool     string
	Field    string
	Equals   string
	Prefix   string
	Decision domain.PolicyDecision
	Reason   string
}

type PolicyDefinition struct {
	Default domain.PolicyDecision
	Rules   []PolicyRule
}

// PolicyEngine is immutable after construction and safe to share across runs.
type PolicyEngine struct {
	profiles map[domain.PolicyProfile]PolicyDefinition
	hashes   map[domain.PolicyProfile]string
}

type PolicyEvaluation struct {
	Snapshot domain.PolicySnapshot
	Decision domain.PolicyDecision
	Reason   string
}

func NewPolicyEngine(definitions map[domain.PolicyProfile]PolicyDefinition) (*PolicyEngine, error) {
	profiles := map[domain.PolicyProfile]PolicyDefinition{
		domain.PolicyProfileDefault:  {},
		domain.PolicyProfilePlan:     {Default: domain.PolicyDeny},
		domain.PolicyProfileReadOnly: {Default: domain.PolicyDeny},
		domain.PolicyProfileFullAuto: {Default: domain.PolicyAllow},
	}
	for profile, definition := range definitions {
		if !profile.Valid() {
			return nil, fmt.Errorf("%w: %q", ErrInvalidPolicyProfile, profile)
		}
		if definition.Default != "" && !definition.Default.Valid() {
			return nil, fmt.Errorf("runtime: policy %q has invalid default %q", profile, definition.Default)
		}
		for i, rule := range definition.Rules {
			if strings.TrimSpace(rule.Tool) == "" || !rule.Decision.Valid() {
				return nil, fmt.Errorf("runtime: policy %q rule %d is invalid", profile, i)
			}
		}
		profiles[profile] = PolicyDefinition{
			Default: definition.Default,
			Rules:   append([]PolicyRule(nil), definition.Rules...),
		}
	}
	hashes := make(map[domain.PolicyProfile]string, len(profiles))
	for profile, definition := range profiles {
		encoded, err := json.Marshal(struct {
			Profile domain.PolicyProfile
			Config  PolicyDefinition
		}{profile, definition})
		if err != nil {
			return nil, fmt.Errorf("runtime: hash policy %q: %w", profile, err)
		}
		digest := sha256.Sum256(encoded)
		hashes[profile] = hex.EncodeToString(digest[:])
	}
	return &PolicyEngine{profiles: profiles, hashes: hashes}, nil
}

func (e *PolicyEngine) Snapshot(profile domain.PolicyProfile) (domain.PolicySnapshot, error) {
	if e == nil {
		return domain.PolicySnapshot{Profile: profile}, nil
	}
	if _, ok := e.profiles[profile]; !ok {
		return domain.PolicySnapshot{}, fmt.Errorf("%w: %q", ErrInvalidPolicyProfile, profile)
	}
	return domain.PolicySnapshot{Profile: profile, Hash: e.hashes[profile]}, nil
}

func (e *PolicyEngine) Evaluate(profile domain.PolicyProfile, spec domain.ToolSpec, args []byte) (PolicyEvaluation, error) {
	snapshot, err := e.Snapshot(profile)
	if err != nil {
		return PolicyEvaluation{}, err
	}
	definition := e.profiles[profile]
	fields := map[string]string{}
	if len(args) > 0 {
		var raw map[string]json.RawMessage
		if err := json.Unmarshal(args, &raw); err != nil {
			return PolicyEvaluation{}, fmt.Errorf("runtime: policy args are not JSON: %w", err)
		}
		for name, value := range raw {
			var text string
			if err := json.Unmarshal(value, &text); err == nil {
				fields[strings.ToLower(name)] = text
			}
		}
	}

	bestRank := -1
	decision := domain.PolicyDecision("")
	reason := ""
	for _, rule := range definition.Rules {
		if !ruleMatches(rule, spec.Name, fields) {
			continue
		}
		rank := policyRank(rule.Decision)
		if rank > bestRank {
			bestRank = rank
			decision = rule.Decision
			reason = rule.Reason
		}
	}
	if decision == "" {
		decision = defaultDecision(profile, definition.Default, spec)
		reason = defaultReason(decision, spec)
	}
	return PolicyEvaluation{Snapshot: snapshot, Decision: decision, Reason: reason}, nil
}

func defaultDecision(profile domain.PolicyProfile, configured domain.PolicyDecision, spec domain.ToolSpec) domain.PolicyDecision {
	if spec.Interaction == domain.ToolInteractionQuestion || spec.Readonly {
		return domain.PolicyAllow
	}
	if configured.Valid() {
		return configured
	}
	switch profile {
	case domain.PolicyProfileFullAuto:
		return domain.PolicyAllow
	case domain.PolicyProfilePlan, domain.PolicyProfileReadOnly:
		return domain.PolicyDeny
	default:
		return domain.PolicyPrompt
	}
}

func defaultReason(decision domain.PolicyDecision, spec domain.ToolSpec) string {
	if spec.Interaction == domain.ToolInteractionQuestion {
		return "user question is a control-flow interaction"
	}
	if spec.Readonly && decision == domain.PolicyAllow {
		return "readonly tool is allowed by default"
	}
	switch decision {
	case domain.PolicyAllow:
		return "effectful tool is allowed by the selected profile"
	case domain.PolicyPrompt:
		return "effectful tool requires user approval"
	default:
		return "effectful tool is denied by the selected profile"
	}
}

func ruleMatches(rule PolicyRule, tool string, fields map[string]string) bool {
	if rule.Tool != "*" && rule.Tool != tool {
		return false
	}
	if rule.Field == "" {
		return true
	}
	value, ok := fields[strings.ToLower(rule.Field)]
	if !ok {
		return false
	}
	if rule.Equals != "" {
		return value == rule.Equals
	}
	if rule.Prefix != "" {
		return strings.HasPrefix(value, rule.Prefix)
	}
	return true
}

func policyRank(decision domain.PolicyDecision) int {
	switch decision {
	case domain.PolicyDeny:
		return 3
	case domain.PolicyPrompt:
		return 2
	case domain.PolicyAllow:
		return 1
	default:
		return -1
	}
}
