package app

import (
	"context"
	"errors"
	"net"

	controlrpc "agent-vivy/internal/rpc"
)

// DialControl returns an in-process JSON-RPC client for the control plane
// (VIVY-FACE-PACK §7): faces are in-process clients speaking the same
// methods the web face reaches over WebSocket, with the transport reduced
// to a memory pipe. The host side serves the composed control handler; the
// returned peer is a client whose optional handler receives server-initiated
// notifications ("run/event" after run/subscribe). The face owns the peer
// lifecycle: cancelling ctx (or closing the peer) ends its half of the
// pipe, and the face must stop before the app's storage closes.
func (a *App) DialControl(ctx context.Context, notifications controlrpc.Handler) (*controlrpc.Peer, error) {
	if a.control == nil {
		return nil, errors.New("app: control plane is not wired")
	}
	hostConn, clientConn := net.Pipe()
	host := controlrpc.NewPeer(
		controlrpc.NewJSONLTransport(hostConn, hostConn, hostConn.Close),
		a.control, controlrpc.Options{OutgoingBuffer: 64},
	)
	go func() { _ = host.Serve(ctx) }()
	client := controlrpc.NewPeer(
		controlrpc.NewJSONLTransport(clientConn, clientConn, clientConn.Close),
		notifications, controlrpc.Options{OutgoingBuffer: 64},
	)
	go func() { _ = client.Serve(ctx) }()
	return client, nil
}
