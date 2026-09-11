// Package rpc implements Vivy's local bidirectional JSON-RPC control plane.
// The transport is deliberately separated from the dispatcher so worker
// stdio and browser WebSocket clients share one protocol and one backpressure
// implementation.
package rpc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"agent-vivy/internal/actionhost"
)

const ProtocolVersion = "vivy.rpc.v1"

const (
	ParseError     = -32700
	InvalidRequest = -32600
	MethodNotFound = -32601
	InvalidParams  = -32602
	InternalError  = -32603
	ServerOverload = -32001
)

var (
	ErrPeerClosed = errors.New("rpc: peer closed")
	ErrOverloaded = errors.New("rpc: outgoing queue is full")
)

type Error struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	return fmt.Sprintf("rpc error %d: %s", e.Code, e.Message)
}

type Request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type Handler interface {
	Handle(context.Context, *Peer, Request) (any, *Error)
}

type HandlerFunc func(context.Context, *Peer, Request) (any, *Error)

func (f HandlerFunc) Handle(ctx context.Context, peer *Peer, request Request) (any, *Error) {
	return f(ctx, peer, request)
}

type Transport interface {
	ReadFrame() ([]byte, error)
	WriteFrame([]byte) error
	Close() error
}

type Options struct {
	MaxFrameBytes  int
	OutgoingBuffer int
	// Caller is the opaque credential bound to an already-authenticated
	// connection. It is copied into each request context by Peer and is never
	// read from browser JSON. WebSocketServer supplies it after validating its
	// handshake token; other transports may supply their own authenticated
	// connection handle.
	Caller actionhost.Caller
	// AuthenticatedCaller is a source-compatible descriptive alias for Caller.
	AuthenticatedCaller actionhost.Caller
	// Identity is an optional server-attested Face identity associated with the
	// connection. It is intentionally not a request parameter.
	Identity actionhost.Identity
}

func (o Options) normalized() Options {
	if o.MaxFrameBytes <= 0 {
		// Four 5 MiB inline images expand to roughly 26.7 MiB as base64,
		// plus the JSON envelope. Keep the transport contract large enough
		// for the handler's documented attachment limit while remaining bounded.
		o.MaxFrameBytes = 32 << 20
	}
	if o.OutgoingBuffer <= 0 {
		o.OutgoingBuffer = 64
	}
	return o
}

type responseFrame struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *Error          `json:"error,omitempty"`
}

type Peer struct {
	transport Transport
	handler   Handler
	options   Options
	caller    actionhost.Caller
	identity  actionhost.Identity

	out  chan []byte
	done chan struct{}
	stop sync.Once

	sequence atomic.Uint64
	mu       sync.Mutex
	pending  map[string]chan responseFrame
	after    map[string][]func()
}

func NewPeer(transport Transport, handler Handler, options Options) *Peer {
	options = options.normalized()
	caller := options.Caller
	if caller.Opaque() == "" {
		caller = options.AuthenticatedCaller
	}
	return &Peer{
		transport: transport,
		handler:   handler,
		options:   options,
		caller:    caller,
		identity:  options.Identity,
		out:       make(chan []byte, options.OutgoingBuffer),
		done:      make(chan struct{}),
		pending:   make(map[string]chan responseFrame),
		after:     make(map[string][]func()),
	}
}

// Serve owns the transport read loop. Requests are handled concurrently;
// writes are serialized by one bounded writer goroutine.
func (p *Peer) Serve(ctx context.Context) error {
	if p.transport == nil {
		return errors.New("rpc: nil transport")
	}
	writerDone := make(chan struct{})
	go p.writeLoop(writerDone)
	watchDone := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			p.stopPeer()
		case <-p.done:
		case <-watchDone:
		}
	}()
	defer func() {
		close(watchDone)
		p.stopPeer()
		<-writerDone
	}()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-p.done:
			return ErrPeerClosed
		default:
		}
		frame, err := p.transport.ReadFrame()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return ErrPeerClosed
			}
			return err
		}
		if len(frame) > p.options.MaxFrameBytes {
			_ = p.sendError(nil, InvalidRequest, "message exceeds maximum frame size", nil)
			return fmt.Errorf("rpc: frame exceeds %d bytes", p.options.MaxFrameBytes)
		}
		p.dispatch(ctx, frame)
	}
}

func (p *Peer) writeLoop(done chan<- struct{}) {
	defer close(done)
	for {
		select {
		case <-p.done:
			return
		case frame := <-p.out:
			if err := p.transport.WriteFrame(frame); err != nil {
				p.stopPeer()
				return
			}
		}
	}
}

func (p *Peer) dispatch(ctx context.Context, frame []byte) {
	var envelope struct {
		JSONRPC string          `json:"jsonrpc"`
		ID      json.RawMessage `json:"id"`
		Method  string          `json:"method"`
		Params  json.RawMessage `json:"params"`
		Result  json.RawMessage `json:"result"`
		Error   *Error          `json:"error"`
	}
	if err := json.Unmarshal(frame, &envelope); err != nil {
		_ = p.sendError(nil, ParseError, "invalid JSON", nil)
		return
	}
	if envelope.Method == "" && (envelope.Result != nil || envelope.Error != nil) {
		p.resolve(responseFrame{JSONRPC: envelope.JSONRPC, ID: envelope.ID, Result: envelope.Result, Error: envelope.Error})
		return
	}
	request := Request{JSONRPC: envelope.JSONRPC, ID: envelope.ID, Method: envelope.Method, Params: envelope.Params}
	if request.JSONRPC != "2.0" || request.Method == "" {
		if len(request.ID) > 0 {
			_ = p.sendError(request.ID, InvalidRequest, "invalid JSON-RPC request", nil)
		}
		return
	}
	go p.handleRequest(ctx, request)
}

func (p *Peer) handleRequest(ctx context.Context, request Request) {
	var result any
	var rpcErr *Error
	ctx = p.authenticatedContext(ctx)
	if p.handler == nil {
		rpcErr = &Error{Code: MethodNotFound, Message: "method not found"}
	} else {
		result, rpcErr = p.handler.Handle(ctx, p, request)
	}
	if len(request.ID) == 0 {
		return
	}
	if rpcErr != nil {
		p.discardAfterResponse(request.ID)
		_ = p.sendErrorContext(ctx, request.ID, rpcErr.Code, rpcErr.Message, rpcErr.Data)
		return
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		p.discardAfterResponse(request.ID)
		_ = p.sendErrorContext(ctx, request.ID, InternalError, "result is not JSON serializable", nil)
		return
	}
	if err := p.enqueueContext(ctx, responseFrame{JSONRPC: "2.0", ID: request.ID, Result: encoded}); err != nil {
		p.discardAfterResponse(request.ID)
		return
	}
	p.runAfterResponse(request.ID)
}

// authenticatedCallerKey is intentionally private to the RPC transport. A
// caller is admitted by the connection adapter (for example, the WebSocket
// handshake), then carried in this context into the control handler. Browser
// request fields can never populate this value.
type authenticatedCallerKey struct{}

// WithAuthenticatedCaller binds an already-authenticated transport caller to
// a request context. It does not authenticate the value; ActionHost performs
// the server-owned token/session check before executing an action.
func WithAuthenticatedCaller(ctx context.Context, caller actionhost.Caller) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if caller.Opaque() == "" {
		return ctx
	}
	return context.WithValue(ctx, authenticatedCallerKey{}, caller)
}

// AuthenticatedCallerFromContext returns the transport-bound caller carried
// by an RPC request context. The boolean is false for an unauthenticated or
// absent caller.
func AuthenticatedCallerFromContext(ctx context.Context) (actionhost.Caller, bool) {
	if ctx == nil {
		return actionhost.Caller{}, false
	}
	caller, ok := ctx.Value(authenticatedCallerKey{}).(actionhost.Caller)
	return caller, ok && caller.Opaque() != ""
}

// WithCaller and CallerFromContext are concise aliases used by transport
// adapters that already use caller terminology.
func WithCaller(ctx context.Context, caller actionhost.Caller) context.Context {
	return WithAuthenticatedCaller(ctx, caller)
}

func CallerFromContext(ctx context.Context) (actionhost.Caller, bool) {
	return AuthenticatedCallerFromContext(ctx)
}

// AuthenticatedCaller returns the connection-bound caller for this Peer. It
// is a transport handle, not an identity or permission result.
func (p *Peer) AuthenticatedCaller() (actionhost.Caller, bool) {
	if p == nil || p.caller.Opaque() == "" {
		return actionhost.Caller{}, false
	}
	return p.caller, true
}

func (p *Peer) authenticatedContext(ctx context.Context) context.Context {
	if p == nil {
		return ctx
	}
	p.mu.Lock()
	identity := p.identity
	p.mu.Unlock()
	if p.caller.Opaque() != "" {
		ctx = WithAuthenticatedCaller(ctx, p.caller)
	}
	if identity.ID != "" {
		ctx = actionhost.WithIdentity(ctx, identity)
	}
	return ctx
}

// bindSession records a session selected by a server-validated control
// operation. Browser JSON never calls this method directly; control handlers
// invoke it only after the session was created or loaded through the trusted
// SessionStore. A connection without a transport-attested base identity stays
// fail-closed and cannot acquire action authority.
func (p *Peer) bindSession(sessionID string) {
	if p == nil {
		return
	}
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return
	}
	p.mu.Lock()
	if p.identity.ID != "" {
		p.identity.SessionID = sessionID
		p.identity.RunID = ""
	}
	p.mu.Unlock()
}

// bindRun records a run returned by a server-owned runtime operation. The
// session/run pair is taken from the operation's durable result, not from an
// action request, so ActionHost receives a connection-bound identity for
// bridge calls without trusting browser-supplied identity fields.
func (p *Peer) bindRun(sessionID, runID string) {
	if p == nil {
		return
	}
	sessionID = strings.TrimSpace(sessionID)
	runID = strings.TrimSpace(runID)
	if sessionID == "" || runID == "" {
		return
	}
	p.mu.Lock()
	if p.identity.ID != "" {
		p.identity.SessionID = sessionID
		p.identity.RunID = runID
	}
	p.mu.Unlock()
}

// AfterResponse schedules work after a request response enters the bounded
// writer queue, preserving response-before-event ordering for subscriptions.
func (p *Peer) AfterResponse(id json.RawMessage, fn func()) {
	if len(id) == 0 || fn == nil {
		return
	}
	p.mu.Lock()
	p.after[string(id)] = append(p.after[string(id)], fn)
	p.mu.Unlock()
}

func (p *Peer) runAfterResponse(id json.RawMessage) {
	p.mu.Lock()
	callbacks := append([]func(){}, p.after[string(id)]...)
	delete(p.after, string(id))
	p.mu.Unlock()
	for _, callback := range callbacks {
		go callback()
	}
}

func (p *Peer) discardAfterResponse(id json.RawMessage) {
	p.mu.Lock()
	delete(p.after, string(id))
	p.mu.Unlock()
}

func (p *Peer) sendError(id json.RawMessage, code int, message string, data json.RawMessage) error {
	return p.enqueue(responseFrame{JSONRPC: "2.0", ID: id, Error: &Error{Code: code, Message: message, Data: data}})
}

func (p *Peer) sendErrorContext(ctx context.Context, id json.RawMessage, code int, message string, data json.RawMessage) error {
	return p.enqueueContext(ctx, responseFrame{JSONRPC: "2.0", ID: id, Error: &Error{Code: code, Message: message, Data: data}})
}

func (p *Peer) enqueue(value responseFrame) error {
	if value.Result == nil && value.Error == nil {
		value.Result = json.RawMessage("null")
	}
	frame, err := json.Marshal(value)
	if err != nil {
		return err
	}
	select {
	case <-p.done:
		return ErrPeerClosed
	case p.out <- frame:
		return nil
	default:
		return ErrOverloaded
	}
}

func (p *Peer) enqueueContext(ctx context.Context, value responseFrame) error {
	if value.Result == nil && value.Error == nil {
		value.Result = json.RawMessage("null")
	}
	frame, err := json.Marshal(value)
	if err != nil {
		return err
	}
	if ctx == nil {
		ctx = context.Background()
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-p.done:
		return ErrPeerClosed
	case p.out <- frame:
		return nil
	}
}

func (p *Peer) resolve(response responseFrame) {
	key := string(response.ID)
	p.mu.Lock()
	waiter := p.pending[key]
	delete(p.pending, key)
	p.mu.Unlock()
	if waiter != nil {
		select {
		case waiter <- response:
		default:
		}
	}
}

// Call sends a request and waits for its response. It is safe to call from a
// handler, allowing the server to issue approval/question requests to a
// client while it is processing another request.
func (p *Peer) Call(ctx context.Context, method string, params any) (json.RawMessage, error) {
	if method == "" {
		return nil, errors.New("rpc: empty method")
	}
	paramsJSON, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("rpc: marshal params: %w", err)
	}
	seq := p.sequence.Add(1)
	id := json.RawMessage(strconv.Quote(strconv.FormatUint(seq, 10)))
	waiter := make(chan responseFrame, 1)
	p.mu.Lock()
	p.pending[string(id)] = waiter
	p.mu.Unlock()
	frame, err := json.Marshal(Request{JSONRPC: "2.0", ID: id, Method: method, Params: paramsJSON})
	if err != nil {
		p.removePending(id)
		return nil, err
	}
	if err := p.enqueueRaw(frame); err != nil {
		p.removePending(id)
		return nil, err
	}
	select {
	case response := <-waiter:
		if response.Error != nil {
			return nil, response.Error
		}
		return response.Result, nil
	case <-ctx.Done():
		p.removePending(id)
		return nil, ctx.Err()
	case <-p.done:
		p.removePending(id)
		return nil, ErrPeerClosed
	}
}

func (p *Peer) Notify(method string, params any) error {
	return p.notify(context.Background(), method, params, false)
}

// NotifyContext enqueues a notification with cancellation-aware backpressure.
// Durable run streams use this path so a temporarily full writer queue cannot
// silently tear down a subscription and strand the client before terminal.
// Ordinary notifications retain Notify's fail-fast ErrOverloaded contract.
func (p *Peer) NotifyContext(ctx context.Context, method string, params any) error {
	return p.notify(ctx, method, params, true)
}

func (p *Peer) notify(ctx context.Context, method string, params any, wait bool) error {
	paramsJSON, err := json.Marshal(params)
	if err != nil {
		return err
	}
	frame, err := json.Marshal(Request{JSONRPC: "2.0", Method: method, Params: paramsJSON})
	if err != nil {
		return err
	}
	if !wait {
		return p.enqueueRaw(frame)
	}
	if ctx == nil {
		ctx = context.Background()
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-p.done:
		return ErrPeerClosed
	case p.out <- frame:
		return nil
	}
}

func (p *Peer) enqueueRaw(frame []byte) error {
	select {
	case <-p.done:
		return ErrPeerClosed
	case p.out <- frame:
		return nil
	default:
		return ErrOverloaded
	}
}

func (p *Peer) removePending(id json.RawMessage) {
	p.mu.Lock()
	delete(p.pending, string(id))
	p.mu.Unlock()
}

// Close stops the peer and the underlying transport.
func (p *Peer) Close() error {
	if p == nil {
		return nil
	}
	p.stopPeer()
	return nil
}

func (p *Peer) stopPeer() {
	p.stop.Do(func() {
		close(p.done)
		_ = p.transport.Close()
		p.mu.Lock()
		defer p.mu.Unlock()
		for key, waiter := range p.pending {
			delete(p.pending, key)
			select {
			case waiter <- responseFrame{Error: &Error{Code: InternalError, Message: ErrPeerClosed.Error()}}:
			default:
			}
		}
	})
}
