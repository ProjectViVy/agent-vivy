package badpicoclaw

import (
	"github.com/sipeed/picoclaw/pkg/channels"

	"agent-vivy/sdk/plugin"
)

func New() plugin.Plugin { return nil }

// picoclawProbe never runs; it exists so the verifier catches reference
// material (the picoclaw clone, contract §9.3) imported as a dependency
// instead of being rewritten. This file need not compile.
var _ = channels.Unknown
