package acp

import (
	"io"
	"time"

	acp "github.com/eino-contrib/acp"
	acpconn "github.com/eino-contrib/acp/conn"
	acpstdio "github.com/eino-contrib/acp/transport/stdio"
)

// Connection bounds from the spec (§10): a single 1 MiB inbound frame cap,
// 64 SDK pending dispatch frames, and a bounded shutdown so teardown never
// hangs the launcher. Generic handler failures reach the wire sanitized.
const (
	maxInboundFrameBytes = 1 << 20
	maxPendingDispatch   = 64
	shutdownTimeout      = 10 * time.Second
	connectionLabel      = "projectvivy.acp"
)

// newConnection builds the single owned ACP stdio connection. SDK logging is
// disabled before any SDK worker starts so nothing besides wire frames can
// reach the streams the launcher pinned.
func newConnection(a *agent, in io.Reader, out io.Writer) (*acpconn.AgentConnection, error) {
	acp.SetLogger(nil, acp.LevelDisabled)
	transport := acpstdio.NewTransport(in, out,
		acpstdio.WithMaxMessageSize(maxInboundFrameBytes),
	)
	return acpconn.NewAgentConnectionFromTransport(a, transport,
		acpconn.WithAgentMaxPendingDispatch(maxPendingDispatch),
		acpconn.WithAgentShutdownTimeout(shutdownTimeout),
		acpconn.WithAgentSanitizedErrors(),
		acpconn.WithAgentConnectionLabel(connectionLabel),
	), nil
}
