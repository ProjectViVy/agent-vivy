package domain

// RunMode controls the side-effect policy for one agent execution.
// Plan mode is a physical runtime restriction, not a prompt-only hint.
type RunMode string

const (
	RunModeNormal RunMode = "normal"
	RunModePlan   RunMode = "plan"
)

// Valid reports whether the mode is supported by the current harness.
func (m RunMode) Valid() bool {
	return m == RunModeNormal || m == RunModePlan
}

// CollaborationMode is a soft, host-owned collaboration hint. It never
// selects a policy profile or grants a tool permission.
type CollaborationMode string

const (
	CollaborationModeNone CollaborationMode = "none"
	CollaborationModePlan CollaborationMode = "plan"
	CollaborationVersion                    = 1
)

// Valid reports whether the collaboration hint is supported.
func (m CollaborationMode) Valid() bool {
	return m == CollaborationModeNone || m == CollaborationModePlan
}
