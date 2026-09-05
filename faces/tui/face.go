// Package tui is the packable first-party terminal organ. The implementation
// lives in sdk/tui so every Vivy Code entry runs the same state machine.
package tui

import (
	"agent-vivy/sdk/plugin"
	tuiface "agent-vivy/sdk/tui/face"
)

const FaceKind = tuiface.Kind

// New is the seam-face constructor required by the packer.
func New(opts plugin.FaceOptions) plugin.Face { return tuiface.New(opts) }
