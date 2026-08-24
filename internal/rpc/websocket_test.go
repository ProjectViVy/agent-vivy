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
