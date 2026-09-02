// Face register of the main line: the committed body carries no face
// organ — `vivy run` keeps the built-in kernel headless loop. vivy-sdk
// pack --face replaces this exact file at build time to assemble a
// generation whose run command is a face organ (VIVY-FACE-PACK.md §6).
// Hand-maintained default; keep the overlay's constructor call in sync.

package face

import (
	"agent-vivy/sdk/plugin"
)

// Register returns the face constructor compiled into this generation, or
// nil when the launcher should keep the built-in headless loop.
func Register() plugin.FaceConstructor {
	return nil
}
