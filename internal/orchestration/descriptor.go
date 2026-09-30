// Package orchestration carries the host-owned resource ceilings shared by the
// INOFY admission boundary, the tool result budget, and inspection output
// caps. The retired first-party descriptor grammar was removed in the INOFY
// cutover; these constants remain the single home for the shared limits.
package orchestration

const (
	MaxNodes          = 12
	MaxEdges          = 24
	MaxWidth          = 4
	MaxOutputs        = 4
	MaxTaskBytes      = 4 << 10
	MaxTotalTaskBytes = 16 << 10
	MaxOutputBytes    = 8 << 10
)
