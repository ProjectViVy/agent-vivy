package badpion

import (
	"github.com/pion/webrtc/v3"

	"agent-vivy/sdk/plugin"
)

func New() plugin.Plugin { return nil }

var _ = webrtc.NewPeerConnection
