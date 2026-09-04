package rpc

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net"
	"testing"
	"time"
)

func TestDefaultFrameLimitCarriesMaximumInlineAttachmentEnvelope(t *testing.T) {
	wantMinimum := base64.StdEncoding.EncodedLen(maxAttachmentCount*maxAttachmentBytes) + (64 << 10)
	if got := (Options{}).normalized().MaxFrameBytes; got < wantMinimum {
		t.Fatalf("default frame limit = %d, need at least %d for attachment contract", got, wantMinimum)
	}
}

func startPeerPair(t *testing.T, serverHandler Handler, clientHandler Handler) (*Peer, *Peer, context.CancelFunc) {
	t.Helper()
	left, right := net.Pipe()
	server := NewPeer(NewJSONLTransport(left, left, left.Close), serverHandler, Options{OutgoingBuffer: 8})
	client := NewPeer(NewJSONLTransport(right, right, right.Close), clientHandler, Options{OutgoingBuffer: 8})
	ctx, cancel := context.WithCancel(context.Background())
	go func() { _ = server.Serve(ctx) }()
	go func() { _ = client.Serve(ctx) }()
	t.Cleanup(func() {
		cancel()
		_ = left.Close()
		_ = right.Close()
	})
	return server, client, cancel
}

func TestPeerCallAndServerInitiatedCall(t *testing.T) {
	clientHandler := HandlerFunc(func(_ context.Context, _ *Peer, request Request) (any, *Error) {
		if request.Method != "client/confirm" {
			return nil, &Error{Code: MethodNotFound, Message: "unknown"}
		}
		return map[string]any{"accepted": true}, nil
	})
	serverHandler := HandlerFunc(func(ctx context.Context, peer *Peer, request Request) (any, *Error) {
		if request.Method != "server/ask-client" {
			return nil, &Error{Code: MethodNotFound, Message: "unknown"}
		}
		result, err := peer.Call(ctx, "client/confirm", map[string]any{"run_id": "run_1"})
		if err != nil {
			return nil, &Error{Code: InternalError, Message: err.Error()}
		}
		return result, nil
	})
	_, client, _ := startPeerPair(t, serverHandler, clientHandler)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	result, err := client.Call(ctx, "server/ask-client", nil)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]bool
	if err := json.Unmarshal(result, &got); err != nil {
		t.Fatal(err)
	}
	if !got["accepted"] {
		t.Fatalf("server-initiated result = %v", got)
	}
}

func TestPeerReturnsRemoteError(t *testing.T) {
	serverHandler := HandlerFunc(func(context.Context, *Peer, Request) (any, *Error) {
		return nil, &Error{Code: InvalidParams, Message: "bad params"}
	})
	_, client, _ := startPeerPair(t, serverHandler, nil)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_, err := client.Call(ctx, "bad", nil)
	var remote *Error
	if !errors.As(err, &remote) || remote.Code != InvalidParams {
		t.Fatalf("error = %v, want invalid params", err)
	}
}

func TestPeerRejectsOverloadedOutgoingQueue(t *testing.T) {
	left, right := net.Pipe()
	defer left.Close()
	defer right.Close()
	peer := NewPeer(NewJSONLTransport(left, left, left.Close), nil, Options{OutgoingBuffer: 1})
	if err := peer.Notify("one", nil); err != nil {
		t.Fatal(err)
	}
	if !errors.Is(peer.Notify("two", nil), ErrOverloaded) {
		t.Fatal("second notify should fail with ErrOverloaded")
	}
}

func TestPeerNotifyContextWaitsForCapacityAndCancels(t *testing.T) {
	left, right := net.Pipe()
	defer left.Close()
	defer right.Close()
	peer := NewPeer(NewJSONLTransport(left, left, left.Close), nil, Options{OutgoingBuffer: 1})
	if err := peer.Notify("one", nil); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
	defer cancel()
	started := time.Now()
	err := peer.NotifyContext(ctx, "two", nil)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("NotifyContext error = %v", err)
	}
	if time.Since(started) < 20*time.Millisecond {
		t.Fatal("NotifyContext did not apply backpressure")
	}
}

func TestPeerResponseEnqueueWaitsForCapacity(t *testing.T) {
	left, right := net.Pipe()
	defer left.Close()
	defer right.Close()
	peer := NewPeer(NewJSONLTransport(left, left, left.Close), nil, Options{OutgoingBuffer: 1})
	if err := peer.Notify("occupy", nil); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		done <- peer.enqueueContext(context.Background(), responseFrame{JSONRPC: "2.0", ID: json.RawMessage(`"response"`), Result: json.RawMessage(`true`)})
	}()
	select {
	case err := <-done:
		t.Fatalf("response enqueue bypassed backpressure: %v", err)
	case <-time.After(20 * time.Millisecond):
	}
	<-peer.out
	if err := <-done; err != nil {
		t.Fatalf("response enqueue after capacity: %v", err)
	}
}

func TestPeerNotifyContextDeliversBurstThroughBoundedQueue(t *testing.T) {
	const count = 256
	received := make(chan int, count)
	clientHandler := HandlerFunc(func(_ context.Context, _ *Peer, request Request) (any, *Error) {
		if request.Method == "chunk" {
			var value int
			if err := json.Unmarshal(request.Params, &value); err != nil {
				t.Errorf("decode chunk: %v", err)
			} else {
				received <- value
			}
		}
		return nil, nil
	})
	serverHandler := HandlerFunc(func(ctx context.Context, peer *Peer, request Request) (any, *Error) {
		if request.Method != "burst" {
			return nil, &Error{Code: MethodNotFound, Message: "unknown"}
		}
		peer.AfterResponse(request.ID, func() {
			for i := 0; i < count; i++ {
				if err := peer.NotifyContext(ctx, "chunk", i); err != nil {
					t.Errorf("notify %d: %v", i, err)
					return
				}
			}
		})
		return map[string]bool{"started": true}, nil
	})
	_, client, _ := startPeerPair(t, serverHandler, clientHandler)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := client.Call(ctx, "burst", nil); err != nil {
		t.Fatal(err)
	}
	seen := make([]bool, count)
	for i := 0; i < count; i++ {
		select {
		case value := <-received:
			if value < 0 || value >= count || seen[value] {
				t.Fatalf("invalid or duplicate chunk %d", value)
			}
			seen[value] = true
		case <-ctx.Done():
			t.Fatalf("received %d/%d notifications: %v", i, count, ctx.Err())
		}
	}
}

func TestPeerRejectsInvalidJSON(t *testing.T) {
	left, right := net.Pipe()
	defer left.Close()
	defer right.Close()
	server := NewPeer(NewJSONLTransport(left, left, left.Close), nil, Options{})
	go func() { _ = server.Serve(context.Background()) }()
	if _, err := right.Write([]byte("not-json\n")); err != nil {
		t.Fatal(err)
	}
	transport := NewJSONLTransport(right, right, nil)
	frame, err := transport.ReadFrame()
	if err != nil {
		t.Fatal(err)
	}
	var response responseFrame
	if err := json.Unmarshal(frame, &response); err != nil {
		t.Fatal(err)
	}
	if response.Error == nil || response.Error.Code != ParseError {
		t.Fatalf("response = %+v, want parse error", response)
	}
	_ = server.transport.Close()
}
