package app

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"agent-vivy/internal/actionhost"
	controlrpc "agent-vivy/internal/rpc"

	"agent-vivy/internal/config"
	plugin "agent-vivy/sdk/port/face"
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
	defer func() { _ = a.Close() }()

	env := &faceEnv{ctx: ctx}
	peer, err := a.DialControl(ctx, env)
	if err != nil {
		return plugin.FaceResult{}, err
	}
	defer peer.Close()
	env.peer = peer
	return ctor(opts).Run(ctx, env)
}

// RunFaceProviderWithAppOptions runs a generated std/face@v1 Provider.
func RunFaceProviderWithAppOptions(ctx context.Context, cfg config.Config, provider plugin.FaceProvider, opts plugin.Options, appOpts ...AppOption) (plugin.Result, error) {
	if provider == nil {
		return plugin.Result{}, errors.New("app: no face provider compiled into this generation")
	}
	if opts.Out == nil || opts.Err == nil {
		return plugin.Result{}, errors.New("app: face requires output and error writers")
	}
	appOpts = append(appOpts, WithoutEars(), WithoutGateway())
	a, err := New(ctx, cfg, appOpts...)
	if err != nil {
		return plugin.Result{}, err
	}
	defer func() { _ = a.Close() }()
	env := &faceEnv{ctx: ctx}
	peer, err := a.DialControl(ctx, env)
	if err != nil {
		return plugin.Result{}, err
	}
	defer peer.Close()
	env.peer = peer
	return RunFaceProvider(ctx, provider, env, opts)
}

// selectedFaceTeardownBudget is the fallback bound for the process
// teardown after the selected Face's provider returns. On the cancel path
// the launcher installs one shared monotonic deadline instead, so stream
// close and the audited app drain share a single budget
// (ACP-STDIO-FACE §490).
const selectedFaceTeardownBudget = 10 * time.Second

// RunSelectedFaceWithAppOptions runs the single Face Provider sealed into
// the compiled RuntimeAssembly — the selected-generation launch path
// (ACP-STDIO-FACE §294). The provider comes from the app's own validated
// assembly (a.assembly.Face), never a caller-supplied organ. App teardown
// is audited through CloseContext under a distinct teardown context, so a
// cancelled invocation cannot starve the drain, and a stage that outlives
// the budget reports which component is still unwinding instead of
// blocking silently. deadline is the launcher's shared shutdown slot: when
// it holds a deadline the close honors it (remaining-budget semantics),
// otherwise the close gets a fresh selectedFaceTeardownBudget.
func RunSelectedFaceWithAppOptions(ctx context.Context, cfg config.Config, opts plugin.Options, deadline *atomic.Pointer[time.Time], appOpts ...AppOption) (result plugin.Result, retErr error) {
	if opts.In == nil || opts.Out == nil || opts.Err == nil {
		return plugin.Result{}, errors.New("app: face requires input, output, and error streams")
	}
	appOpts = append(appOpts, WithoutEars(), WithoutGateway())
	a, err := New(ctx, cfg, appOpts...)
	if err != nil {
		return plugin.Result{}, err
	}
	defer func() {
		teardown, cancel := context.WithDeadline(context.Background(), selectedCloseDeadline(deadline))
		defer cancel()
		retErr = errors.Join(retErr, a.CloseContext(teardown))
	}()
	return runAssemblyFace(ctx, a, opts)
}

// selectedCloseDeadline resolves the app-close bound: the launcher's shared
// shutdown deadline when the cancel path installed one, else a fresh
// fallback budget so a normal provider exit is still bounded.
func selectedCloseDeadline(slot *atomic.Pointer[time.Time]) time.Time {
	if slot != nil {
		if shared := slot.Load(); shared != nil {
			return *shared
		}
	}
	return time.Now().Add(selectedFaceTeardownBudget)
}

// runAssemblyFace drives the Face Provider sealed into an already-composed
// App. Keeping the a.assembly.Face read separate from app construction keeps
// the manifest-validated datum testable against a synthetic assembly while
// production always takes the sealed BuildDefault provider.
func runAssemblyFace(ctx context.Context, a *App, opts plugin.Options) (plugin.Result, error) {
	provider := a.assembly.Face
	if provider == nil {
		return plugin.Result{}, errors.New("app: no face provider compiled into this generation")
	}
	env := &faceEnv{ctx: ctx}
	peer, err := a.DialControl(ctx, env)
	if err != nil {
		return plugin.Result{}, err
	}
	defer peer.Close()
	env.peer = peer
	return RunFaceProvider(ctx, provider, env, opts)
}

// runFaceProvider is the focused std/face@v1 Host consumer. Keeping the
// Provider/Instance handoff separate from app construction makes the Port
// contract directly conformance-testable while production still supplies the
// authenticated in-process control-plane Host above.
func RunFaceProvider(ctx context.Context, provider plugin.FaceProvider, host plugin.Host, opts plugin.Options) (plugin.Result, error) {
	instance, err := provider.Construct(ctx, host)
	if err != nil {
		return plugin.Result{}, err
	}
	return instance.Run(ctx, opts)
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

func (*faceEnv) ModuleID() string { return "vivy/face-host" }

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
	// The in-process pipe admits no handshake, so the serving peer carries
	// the app's own session token and an embedded-face identity; ActionHost
	// still validates caller.Opaque against rpcToken on every invoke.
	host := controlrpc.NewPeer(
		controlrpc.NewJSONLTransport(hostConn, hostConn, hostConn.Close),
		a.control, controlrpc.Options{
			OutgoingBuffer: 64,
			Caller:         actionhost.NewCaller(a.rpcToken),
			Identity:       actionhost.Identity{ID: "face/embedded", Face: "embedded"},
		},
	)
	go func() { _ = host.Serve(ctx) }()
	client := controlrpc.NewPeer(
		controlrpc.NewJSONLTransport(clientConn, clientConn, clientConn.Close),
		notifications, controlrpc.Options{OutgoingBuffer: 64},
	)
	go func() { _ = client.Serve(ctx) }()
	return client, nil
}
