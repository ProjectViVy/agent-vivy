package tui

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"
	"time"

	controlrpc "agent-vivy/internal/rpc"
)

func TestREPLHelpAndQuit(t *testing.T) {
	handler := controlrpc.HandlerFunc(func(_ context.Context, _ *controlrpc.Peer, request controlrpc.Request) (any, *controlrpc.Error) {
		switch request.Method {
		case "session/create":
			return map[string]string{"id": "sess_1", "title": "TUI", "permission_preset": "smart"}, nil
		default:
			return nil, &controlrpc.Error{Code: controlrpc.MethodNotFound, Message: request.Method}
		}
	})
	client, stop := attachTestClient(t, handler)
	defer stop()

	var out bytes.Buffer
	err := RunREPL(context.Background(), client, Options{
		Input:  strings.NewReader("/help\n/quit\n"),
		Output: &out,
		Title:  "test",
	})
	if err != nil {
		t.Fatal(err)
	}
	got := out.String()
	if !strings.Contains(got, "session sess_1") {
		t.Fatalf("missing session banner:\n%s", got)
	}
	if !strings.Contains(got, "/cancel") {
		t.Fatalf("missing help:\n%s", got)
	}
}

func TestREPLTurnStreamsDeltas(t *testing.T) {
	handler := controlrpc.HandlerFunc(func(ctx context.Context, peer *controlrpc.Peer, request controlrpc.Request) (any, *controlrpc.Error) {
		switch request.Method {
		case "session/create":
			return map[string]string{"id": "sess_1", "title": "TUI", "permission_preset": "smart"}, nil
		case "turn/start":
			return map[string]string{"run_id": "run_1", "status": "accepted"}, nil
		case "run/subscribe":
			if peer == nil {
				return nil, &controlrpc.Error{Code: controlrpc.InternalError, Message: "no peer"}
			}
			peer.AfterResponse(request.ID, func() {
				_ = peer.Notify("run/event", map[string]any{
					"subscription_id": "sub_1",
					"event": map[string]any{
						"run_id": "run_1", "seq": 1, "type": "model.delta",
						"payload": map[string]string{"delta": "hi"},
					},
				})
				_ = peer.Notify("run/event", map[string]any{
					"subscription_id": "sub_1",
					"event": map[string]any{
						"run_id": "run_1", "seq": 2, "type": "run.completed",
						"payload": map[string]any{},
					},
				})
			})
			return map[string]string{"subscription_id": "sub_1"}, nil
		default:
			return nil, &controlrpc.Error{Code: controlrpc.MethodNotFound, Message: request.Method}
		}
	})
	client, stop := attachTestClient(t, handler)
	defer stop()

	var out bytes.Buffer
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	err := RunREPL(ctx, client, Options{
		Input:  strings.NewReader("hello\n/quit\n"),
		Output: &out,
		Title:  "test",
	})
	if err != nil {
		t.Fatal(err)
	}
	got := out.String()
	if !strings.Contains(got, "vivy: hi") {
		t.Fatalf("missing streamed reply:\n%s", got)
	}
}

func attachTestClient(t *testing.T, serverHandler controlrpc.Handler) (*Client, func()) {
	t.Helper()
	left, right := io.Pipe()
	up, down := io.Pipe()
	client := &Client{}
	serverPeer := controlrpc.NewPeer(controlrpc.NewJSONLTransport(left, down, left.Close), serverHandler, controlrpc.Options{OutgoingBuffer: 16})
	clientPeer := controlrpc.NewPeer(controlrpc.NewJSONLTransport(up, right, up.Close), client, controlrpc.Options{OutgoingBuffer: 16})
	client.peer = clientPeer
	ctx, cancel := context.WithCancel(context.Background())
	go func() { _ = serverPeer.Serve(ctx) }()
	go func() { _ = clientPeer.Serve(ctx) }()
	return client, func() {
		cancel()
		_ = serverPeer.Close()
		_ = clientPeer.Close()
	}
}

func TestSplitAddr(t *testing.T) {
	httpBase, wsBase, err := splitAddr("127.0.0.1:8787")
	if err != nil {
		t.Fatal(err)
	}
	if httpBase != "http://127.0.0.1:8787" || wsBase != "ws://127.0.0.1:8787" {
		t.Fatalf("%s %s", httpBase, wsBase)
	}
}

func TestCallJSONRoundTrip(t *testing.T) {
	handler := controlrpc.HandlerFunc(func(_ context.Context, _ *controlrpc.Peer, request controlrpc.Request) (any, *controlrpc.Error) {
		if request.Method == "session/create" {
			return map[string]string{"id": "sess_x", "title": "n"}, nil
		}
		return nil, &controlrpc.Error{Code: controlrpc.MethodNotFound, Message: request.Method}
	})
	client, stop := attachTestClient(t, handler)
	defer stop()
	session, err := client.createSession(context.Background(), "n")
	if err != nil {
		t.Fatal(err)
	}
	if session.ID != "sess_x" {
		t.Fatalf("session = %+v", session)
	}
}
