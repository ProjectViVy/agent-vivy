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

// PolicySnapshot contains the exact admitted policy. Definition is canonical
// JSON from the policy owner, held only in private execution state. Public
// projections must use Profile rather than serialize this value.
type PolicySnapshot struct {
	Profile    PolicyProfile
	Definition string
}
