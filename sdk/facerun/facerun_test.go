package facerun

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"agent-vivy/internal/rpc"
	faceport "agent-vivy/sdk/port/face"
)

type rpcTestHost struct {
	peer  *rpc.Peer
	mu    sync.Mutex
	event func(string, json.RawMessage)
}

func (*rpcTestHost) ModuleID() string { return "test/face" }
func (h *rpcTestHost) Call(ctx context.Context, method string, params any) (json.RawMessage, error) {
	return h.peer.Call(ctx, method, params)
}
func (h *rpcTestHost) OnEvent(fn func(string, json.RawMessage)) {
	h.mu.Lock()
	h.event = fn
	h.mu.Unlock()
}
func (h *rpcTestHost) Handle(_ context.Context, _ *rpc.Peer, request rpc.Request) (any, *rpc.Error) {
	if len(request.ID) != 0 {
		return true, nil
	}
	h.mu.Lock()
	fn := h.event
	h.mu.Unlock()
	if fn != nil {
		fn(request.Method, request.Params)
	}
	return nil, nil
}

type gatedTextSink struct {
	TextSink
	started chan struct{}
	release chan struct{}
	ctx     context.Context
	once    sync.Once
}

func (s *gatedTextSink) JournalEvent(typ string, _ int64, _ json.RawMessage) {
	if typ == "model.request" {
		s.once.Do(func() {
			close(s.started)
			select {
			case <-s.release:
			case <-s.ctx.Done():
			}
		})
	}
}

func TestRunVerifiesModelIntegrityThroughOrderedRPCNotifications(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var out, diagnostics bytes.Buffer
	sink := &gatedTextSink{
		TextSink: TextSink{Out: &out, Err: &diagnostics},
		started:  make(chan struct{}), release: make(chan struct{}), ctx: ctx,
	}
	wireDone := make(chan error, 1)
	const delta = "hello 世界\n"
	const count = 256
	text := strings.Repeat(delta, count)
	serverHandler := rpc.HandlerFunc(func(ctx context.Context, peer *rpc.Peer, request rpc.Request) (any, *rpc.Error) {
		switch request.Method {
		case "initialize":
			return map[string]any{"protocol_version": rpc.ProtocolVersion}, nil
		case "session/create":
			return map[string]any{"id": "session_test"}, nil
		case "turn/start":
			return map[string]any{"run_id": "run_test"}, nil
		case "run/subscribe":
			peer.AfterResponse(request.ID, func() {
				seq := 0
				send := func(typ string, version int, payload any) error {
					seq++
					return peer.NotifyContext(ctx, "run/event", map[string]any{
						"subscription_id": "sub_test",
						"event": map[string]any{"run_id": "run_test", "seq": seq,
							"type": typ, "payload_version": version, "payload": payload},
					})
				}
				// Two model calls prove reset boundaries as well as UTF-8 byte lengths.
				for call := 0; call < 2; call++ {
					if err := send("model.request", 1, map[string]any{}); err != nil {
						wireDone <- err
						return
					}
					for i := 0; i < count; i++ {
						if err := send("model.delta", 1, map[string]any{"delta": delta}); err != nil {
							wireDone <- err
							return
						}
					}
					if err := send("model.completed", 2, map[string]any{
						"content_sha256": fmt.Sprintf("%x", sha256.Sum256([]byte(text))), "byte_len": len(text),
					}); err != nil {
						wireDone <- err
						return
					}
				}
				if err := send("run.completed", 1, map[string]any{}); err != nil {
					wireDone <- err
					return
				}
				_, err := peer.Call(ctx, "barrier", nil)
				wireDone <- err
			})
			return map[string]any{"subscription_id": "sub_test"}, nil
		default:
			return nil, &rpc.Error{Code: rpc.MethodNotFound, Message: request.Method}
		}
	})
	left, right := net.Pipe()
	server := rpc.NewPeer(rpc.NewJSONLTransport(left, left, left.Close), serverHandler, rpc.Options{OutgoingBuffer: 1024})
	host := &rpcTestHost{}
	host.peer = rpc.NewPeer(rpc.NewJSONLTransport(right, right, right.Close), host, rpc.Options{})
	go func() { _ = server.Serve(ctx) }()
	go func() { _ = host.peer.Serve(ctx) }()
	defer func() {
		cancel()
		_ = server.Close()
		_ = host.peer.Close()
		<-server.ServeDone()
		<-host.peer.ServeDone()
	}()
	settled := make(chan faceport.Result, 1)
	failed := make(chan error, 1)
	go func() {
		result, err := Run(ctx, host, Options{Prompt: "test ordered stream", Face: "test"}, sink)
		if err != nil {
			failed <- err
			return
		}
		settled <- result
	}()
	select {
	case <-sink.started:
	case err := <-failed:
		t.Fatal(err)
	case <-ctx.Done():
		t.Fatal("model.request did not reach the face")
	}
	select {
	case err := <-wireDone:
		if err != nil {
			t.Fatal(err)
		}
	case <-ctx.Done():
		t.Fatal("reader could not pass the blocked face callback")
	}
	select {
	case result := <-settled:
		t.Fatalf("run settled before model.request returned: %s", result.Status)
	default:
	}
	close(sink.release)
	select {
	case result := <-settled:
		if result.Status != "completed" {
			t.Fatalf("status = %s; diagnostics = %s", result.Status, diagnostics.String())
		}
	case err := <-failed:
		t.Fatal(err)
	case <-ctx.Done():
		t.Fatal("face did not settle")
	}
	_ = host.peer.Close()
	<-host.peer.ServeDone()
	if got, want := out.String(), text+"\n"+text+"\n"; got != want {
		t.Fatalf("output byte length = %d, want %d", len(got), len(want))
	}
	if diagnostics.Len() != 0 {
		t.Fatalf("diagnostics = %s", diagnostics.String())
	}
}
