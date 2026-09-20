// Package rpccontract defines implementation-free protocol and contribution
// types shared by the control-plane dispatcher and build-owned Modules.
package rpccontract

import (
	"context"
	"encoding/json"
	"fmt"

	"agent-vivy/internal/actionhost"
)

// Error is the JSON-RPC error shape used by both core and contributed methods.
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

// Request is the request envelope presented to a method handler.
type Request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// Peer is the typed, least-authority view of an authenticated control-plane
// connection available to contributed methods. Authentication and identity
// remain server-attested in context; Modules cannot construct a transport.
type Peer interface {
	AuthenticatedCaller() (actionhost.Caller, bool)
	AfterResponse(json.RawMessage, func())
	Call(context.Context, string, any) (json.RawMessage, error)
	Notify(string, any) error
	NotifyContext(context.Context, string, any) error
}

// authenticatedCallerKey is private to the shared protocol contract. A
// transport adapter admits this opaque caller; request JSON cannot set it.
type authenticatedCallerKey struct{}

// WithAuthenticatedCaller binds an already-authenticated transport caller to
// a request context.
func WithAuthenticatedCaller(ctx context.Context, caller actionhost.Caller) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if caller.Opaque() == "" {
		return ctx
	}
	return context.WithValue(ctx, authenticatedCallerKey{}, caller)
}

// AuthenticatedCallerFromContext returns the server-attested transport caller.
func AuthenticatedCallerFromContext(ctx context.Context) (actionhost.Caller, bool) {
	if ctx == nil {
		return actionhost.Caller{}, false
	}
	caller, ok := ctx.Value(authenticatedCallerKey{}).(actionhost.Caller)
	return caller, ok && caller.Opaque() != ""
}

func WithCaller(ctx context.Context, caller actionhost.Caller) context.Context {
	return WithAuthenticatedCaller(ctx, caller)
}

func CallerFromContext(ctx context.Context) (actionhost.Caller, bool) {
	return AuthenticatedCallerFromContext(ctx)
}

// HandlerFunc is a contributed method handler. It shares Request and Error
// with the core dispatcher while avoiding a dependency on its implementation.
type HandlerFunc func(context.Context, Peer, Request) (any, *Error)
