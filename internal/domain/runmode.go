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
