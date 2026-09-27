package rpc

import (
	"context"
	"encoding/json"
	"testing"

	"agent-vivy/internal/actionhost"
	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
)

func TestPlanEnterUsesAuthenticatedPeerSessionBinding(t *testing.T) {
	ctx := context.Background()
	env := newControlTestEnv(t, func(deps *ControlDeps) { deps.Work = deps.Sessions.(storage.WorkStore) })
	peer := NewPeer(nil, env.handler, Options{Identity: actionhost.Identity{ID: "face/connection", Face: "web"}})
	created, rpcErr := env.handler.Handle(ctx, peer, Request{JSONRPC: "2.0", Method: "session/create", Params: json.RawMessage(`{"title":"bound Plan"}`)})
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	encoded, err := json.Marshal(created)
	if err != nil {
		t.Fatal(err)
	}
	var bound sessionResult
	if err := json.Unmarshal(encoded, &bound); err != nil || bound.ID == "" {
		t.Fatalf("bound session = %s / %v", encoded, err)
	}
	identity, ok := actionhost.IdentityFromContext(peer.authenticatedContext(ctx))
	if !ok || identity.SessionID != string(bound.ID) {
		t.Fatalf("peer identity = %+v / %v", identity, ok)
	}
	other := domain.SessionID("sess-unbound-plan")
	if err := env.backend.CreateSession(ctx, domain.Session{ID: other, CreatedAt: 1}); err != nil {
		t.Fatal(err)
	}
	call := func(sessionID domain.SessionID, requestID string) (any, *Error) {
		params, err := json.Marshal(map[string]any{"session_id": sessionID, "expected_version": 0, "request_id": requestID})
		if err != nil {
			t.Fatal(err)
		}
		return env.handler.Handle(peer.authenticatedContext(ctx), peer, Request{JSONRPC: "2.0", Method: "plan/enter", Params: params})
	}
	if _, rpcErr := call(other, "wrong-session"); rpcErr == nil || rpcErr.Code != CodeConflict {
		t.Fatalf("mismatched Plan session = %v, want conflict", rpcErr)
	}
	otherState, err := env.backend.ReadWork(ctx, other)
	if err != nil || otherState.Version != 0 || otherState.Plan.Active {
		t.Fatalf("mismatched session Work = %+v / %v", otherState, err)
	}
	if _, rpcErr := call(bound.ID, "bound-plan"); rpcErr != nil {
		t.Fatalf("bound plan/enter: %v", rpcErr)
	}
	state, err := env.backend.ReadWork(ctx, bound.ID)
	if err != nil || state.Version != 1 || !state.Plan.Active {
		t.Fatalf("bound Plan Work = %+v / %v", state, err)
	}
	events, _, err := env.backend.ReplayWork(ctx, bound.ID, domain.WorkState{SessionID: bound.ID}, 10)
	if err != nil || len(events) != 1 || events[0].Kind != domain.WorkEventPlanEntered {
		t.Fatalf("bound Plan Journal = %+v / %v", events, err)
	}
}
