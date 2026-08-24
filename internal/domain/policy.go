package domain

// PolicyProfile selects the immutable execution policy for one run.
type PolicyProfile string

const (
	PolicyProfileDefault  PolicyProfile = "default"
	PolicyProfilePlan     PolicyProfile = "plan"
	PolicyProfileReadOnly PolicyProfile = "read_only"
	PolicyProfileFullAuto PolicyProfile = "full_auto"
)

func (p PolicyProfile) Valid() bool {
	switch p {
	case PolicyProfileDefault, PolicyProfilePlan, PolicyProfileReadOnly, PolicyProfileFullAuto:
		return true
	default:
		return false
	}
}

// PolicyDecision is the result of evaluating a tool call before execution.
type PolicyDecision string

const (
	PolicyAllow  PolicyDecision = "allow"
	PolicyPrompt PolicyDecision = "prompt"
	PolicyDeny   PolicyDecision = "deny"
)

func (d PolicyDecision) Valid() bool {
	switch d {
	case PolicyAllow, PolicyPrompt, PolicyDeny:
		return true
	default:
		return false
	}
}

// PolicySnapshot identifies the exact policy selected for a run. The hash is
// metadata only; the policy definition remains process configuration.
type PolicySnapshot struct {
	Profile PolicyProfile
	Hash    string
}
