package rpccontract

import (
	"context"
	"encoding/json"
	"testing"

	"agent-vivy/internal/actionhost"
)

type fakePeer struct{}

func (fakePeer) AuthenticatedCaller() (actionhost.Caller, bool)             { return actionhost.Caller{}, false }
func (fakePeer) AfterResponse(json.RawMessage, func())                      {}
func (fakePeer) Call(context.Context, string, any) (json.RawMessage, error) { return nil, nil }
func (fakePeer) Notify(string, any) error                                   { return nil }
func (fakePeer) NotifyContext(context.Context, string, any) error           { return nil }

func TestContributionKeepsTypedPeerAndProtocolTypes(t *testing.T) {
	binding := MethodBinding{
		Method:     "channel/inspect",
		Capability: "channel.inspect",
		Handler: func(_ context.Context, peer Peer, request Request) (any, *Error) {
			if peer == nil || request.Method != "channel/inspect" {
				t.Fatalf("handler inputs = (%#v, %#v)", peer, request)
			}
			return "ok", nil
		},
	}
	got, rpcErr := binding.Handler(context.Background(), fakePeer{}, Request{Method: "channel/inspect"})
	if rpcErr != nil || got != "ok" {
		t.Fatalf("handler result = (%#v, %#v), want (ok, nil)", got, rpcErr)
	}
}

func TestAuthenticatedCallerContextRoundTrip(t *testing.T) {
	want := actionhost.NewCaller("server-attested")
	ctx := WithAuthenticatedCaller(context.Background(), want)
	got, ok := AuthenticatedCallerFromContext(ctx)
	if !ok || got.Opaque() != want.Opaque() {
		t.Fatalf("caller = (%q, %v), want (%q, true)", got.Opaque(), ok, want.Opaque())
	}
}
