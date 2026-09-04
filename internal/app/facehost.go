package app

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"sync"

	controlrpc "agent-vivy/internal/rpc"

	"agent-vivy/internal/config"
	"agent-vivy/sdk/plugin"
)

// RunFace composes a gateway-less app, dials the in-process control plane,
// and hands the control-plane client to a face organ (VIVY-FACE-PACK §7).
// It is the launcher half of the seam-face contract: the organ never sees
// the app — only a plugin.FaceEnv speaking the same JSON-RPC methods the
// web face reaches over WebSocket, with the transport reduced to a memory
// pipe. The organ owns the invocation; returning its FaceResult is the
// launcher's only signal for exit codes.
func RunFace(ctx context.Context, cfg config.Config, ctor plugin.FaceConstructor, opts plugin.FaceOptions) (plugin.FaceResult, error) {
	return RunFaceWithAppOptions(ctx, cfg, ctor, opts)
}

// RunFaceWithAppOptions is RunFace with explicit composition overrides.
// Face launchers use it to select a shared settings document independently
// from their private Journal path; ears and the HTTP gateway remain disabled.
func RunFaceWithAppOptions(ctx context.Context, cfg config.Config, ctor plugin.FaceConstructor, opts plugin.FaceOptions, appOpts ...AppOption) (plugin.FaceResult, error) {
	if ctor == nil {
		return plugin.FaceResult{}, errors.New("app: no face organ compiled into this generation")
	}
	if opts.Out == nil || opts.Err == nil {
		return plugin.FaceResult{}, errors.New("app: face requires output and error writers")
	}
	appOpts = append(appOpts, WithoutEars(), WithoutGateway())
	a, err := New(ctx, cfg, appOpts...)
	if err != nil {
		return plugin.FaceResult{}, err
	}
	defer func() { _ = a.backend.Close() }()

	env := &faceEnv{ctx: ctx}
	peer, err := a.DialControl(ctx, env)
	if err != nil {
		return plugin.FaceResult{}, err
	}
	defer peer.Close()
	env.peer = peer
	return ctor(opts).Run(ctx, env)
}

// faceEnv adapts the control-plane peer to plugin.FaceEnv. OnEvent
// installs the notification callback; notifications arriving before
// registration are dropped — the web face has the same property (events
// before run/subscribe are not delivered). A server-initiated request
// that expects a response is answered with a null result; this cut's
// organs drive approval/question through Call, never through callbacks.
type faceEnv struct {
	ctx  context.Context
	peer *controlrpc.Peer

	mu      sync.Mutex
	handler func(method string, params json.RawMessage)
}

func (e *faceEnv) Call(ctx context.Context, method string, params any) (json.RawMessage, error) {
	if e.peer == nil {
		return nil, errors.New("app: control plane is not wired")
	}
	return e.peer.Call(ctx, method, params)
}

func (e *faceEnv) OnEvent(handler func(method string, params json.RawMessage)) {
	e.mu.Lock()
	e.handler = handler
	e.mu.Unlock()
}

func (e *faceEnv) Handle(_ context.Context, _ *controlrpc.Peer, request controlrpc.Request) (any, *controlrpc.Error) {
	e.mu.Lock()
	handler := e.handler
	e.mu.Unlock()
	if handler != nil {
		handler(request.Method, request.Params)
	}
	return nil, nil
}

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
