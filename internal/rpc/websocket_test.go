package rpc

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"agent-vivy/internal/actionhost"
	"agent-vivy/internal/domain"
)

func TestWebSocketServerRequiresTokenAndServesRPC(t *testing.T) {
	handler := HandlerFunc(func(_ context.Context, _ *Peer, request Request) (any, *Error) {
		if request.Method == "initialize" {
			return map[string]string{"protocol_version": ProtocolVersion}, nil
		}
		return nil, &Error{Code: MethodNotFound, Message: "missing"}
	})
	token := "test-token"
	server := httptest.NewServer(WebSocketServer{Handler: handler, Token: token})
	defer server.Close()
	parsed, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	wsURL := "ws://" + parsed.Host + "/rpc?token=" + token
	dialer := websocket.DefaultDialer
	conn, _, err := dialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if err := conn.WriteJSON(Request{JSONRPC: "2.0", ID: json.RawMessage(`"1"`), Method: "initialize"}); err != nil {
		t.Fatal(err)
	}
	_ = conn.SetReadDeadline(time.Now().Add(time.Second))
	var response responseFrame
	if err := conn.ReadJSON(&response); err != nil {
		t.Fatal(err)
	}
	if response.Error != nil {
		t.Fatalf("response error = %v", response.Error)
	}
	var result map[string]string
	if err := json.Unmarshal(response.Result, &result); err != nil {
		t.Fatal(err)
	}
	if result["protocol_version"] != ProtocolVersion {
		t.Fatalf("result = %v", result)
	}
}

func TestWebSocketServerAllowsConfiguredOriginAndRejectsOthers(t *testing.T) {
	handler := HandlerFunc(func(_ context.Context, _ *Peer, request Request) (any, *Error) {
		if request.Method == "initialize" {
			return map[string]string{"protocol_version": ProtocolVersion}, nil
		}
		return nil, &Error{Code: MethodNotFound, Message: "missing"}
	})
	origins, err := NewOriginPolicy([]string{"http://127.0.0.1:3015"})
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(WebSocketServer{Handler: handler, Token: "test-token", Origins: origins})
	defer server.Close()
	parsed, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	wsURL := "ws://" + parsed.Host + "/rpc?token=test-token"
	allowedHeaders := http.Header{"Origin": []string{"http://127.0.0.1:3015"}}
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, allowedHeaders)
	if err != nil {
		t.Fatal(err)
	}
	_ = conn.Close()

	deniedHeaders := http.Header{"Origin": []string{"http://127.0.0.1:3016"}}
	if _, _, err := websocket.DefaultDialer.Dial(wsURL, deniedHeaders); err == nil {
		t.Fatal("expected disallowed origin to fail")
	}
}

func TestPeerBindsServerValidatedSessionAndRun(t *testing.T) {
	peer := NewPeer(nil, nil, Options{
		Caller:   actionhost.NewCaller("transport-token"),
		Identity: actionhost.Identity{ID: "face/connection", Face: "web"},
	})
	peer.bindSession("sess_server")
	ctx := peer.authenticatedContext(context.Background())
	identity, ok := actionhost.IdentityFromContext(ctx)
	if !ok || identity.SessionID != "sess_server" || identity.RunID != "" {
		t.Fatalf("session binding = %#v (ok=%v)", identity, ok)
	}
	peer.bindRun("sess_server", "run_server")
	identity, ok = actionhost.IdentityFromContext(peer.authenticatedContext(context.Background()))
	if !ok || identity.SessionID != "sess_server" || identity.RunID != "run_server" {
		t.Fatalf("run binding = %#v (ok=%v)", identity, ok)
	}
}

func TestPeerWithoutTransportIdentityCannotAcquireBinding(t *testing.T) {
	peer := NewPeer(nil, nil, Options{Caller: actionhost.NewCaller("transport-token")})
	peer.bindSession("browser_claim")
	if _, ok := actionhost.IdentityFromContext(peer.authenticatedContext(context.Background())); ok {
		t.Fatal("peer acquired an action identity without a transport-attested base identity")
	}
}

func TestControlBindsPeerOnlyFromValidatedSessionAndRun(t *testing.T) {
	env := newControlTestEnv(t)
	peer := NewPeer(nil, env.handler, Options{Identity: actionhost.Identity{ID: "face/connection", Face: "web"}})
	created, rpcErr := env.handler.Handle(context.Background(), peer, Request{JSONRPC: "2.0", Method: "session/create", Params: json.RawMessage(`{"title":"bound"}`)})
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	var session sessionResult
	createdJSON, _ := json.Marshal(created)
	if err := json.Unmarshal(createdJSON, &session); err != nil || session.ID == "" {
		t.Fatalf("created session = %s/%v", createdJSON, err)
	}
	identity, ok := actionhost.IdentityFromContext(peer.authenticatedContext(context.Background()))
	if !ok || identity.SessionID != string(session.ID) {
		t.Fatalf("session identity = %#v (ok=%v)", identity, ok)
	}
	runID := domain.RunID("run_bound")
	if err := env.backend.CreateRun(context.Background(), domain.Run{ID: runID, SessionID: session.ID, Status: domain.RunCompleted, CreatedAt: 1}); err != nil {
		t.Fatal(err)
	}
	_, rpcErr = env.handler.Handle(context.Background(), peer, Request{JSONRPC: "2.0", Method: "run/get", Params: json.RawMessage(`{"run_id":"run_bound"}`)})
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	identity, ok = actionhost.IdentityFromContext(peer.authenticatedContext(context.Background()))
	if !ok || identity.SessionID != string(session.ID) || identity.RunID != string(runID) {
		t.Fatalf("run identity = %#v (ok=%v)", identity, ok)
	}
}

func TestControlRunBindingIgnoresBrowserSessionClaim(t *testing.T) {
	env := newControlTestEnv(t)
	handler := env.handler.(*controlHandler)
	peer := NewPeer(nil, env.handler, Options{Identity: actionhost.Identity{ID: "face/connection", Face: "web"}})
	created, rpcErr := env.handler.Handle(context.Background(), peer, Request{JSONRPC: "2.0", Method: "session/create", Params: json.RawMessage(`{"title":"bound"}`)})
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	var session sessionResult
	createdJSON, _ := json.Marshal(created)
	if err := json.Unmarshal(createdJSON, &session); err != nil || session.ID == "" {
		t.Fatalf("created session = %s/%v", createdJSON, err)
	}
	runID := domain.RunID("run_bound_claim")
	if err := env.backend.CreateRun(context.Background(), domain.Run{ID: runID, SessionID: session.ID, Status: domain.RunCompleted, CreatedAt: 1}); err != nil {
		t.Fatal(err)
	}
	// The response carries a caller-controlled session claim; only the durable
	// RunStore's session/run pair may enter the Peer identity.
	handler.bindPeerRunResult(context.Background(), peer, map[string]any{"run_id": runID, "session_id": "browser-claimed"})
	identity, ok := actionhost.IdentityFromContext(peer.authenticatedContext(context.Background()))
	if !ok || identity.SessionID != string(session.ID) || identity.RunID != string(runID) {
		t.Fatalf("run binding = %#v (ok=%v), want durable session %q", identity, ok, session.ID)
	}
	handler.bindPeerRunResult(context.Background(), peer, map[string]any{"run_id": "missing-run", "session_id": "browser-only"})
	identity, ok = actionhost.IdentityFromContext(peer.authenticatedContext(context.Background()))
	if !ok || identity.SessionID != string(session.ID) || identity.RunID != string(runID) {
		t.Fatalf("unknown run changed identity = %#v (ok=%v)", identity, ok)
	}
}
