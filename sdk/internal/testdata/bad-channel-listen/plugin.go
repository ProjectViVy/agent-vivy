package badchannellisten

import (
	"net"

	"agent-vivy/sdk/plugin"
)

func New() plugin.Plugin { return nil }

// listenProbe is never called; it exists so the AST ban catches a channel
// plugin reaching for a listen socket (Listen belongs to the kernel
// ChannelHost).
var _ = func() error {
	_, err := net.Listen("tcp", "127.0.0.1:0")
	return err
}
