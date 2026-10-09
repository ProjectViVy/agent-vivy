package rpc

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net"
	"testing"
	"time"

	"agent-vivy/internal/attachment"
)

func TestDefaultFrameLimitCarriesMaximumInlineAttachmentEnvelope(t *testing.T) {
	wantMinimum := base64.StdEncoding.EncodedLen(attachment.MaxCount*attachment.MaxBytes) + (64 << 10)
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
	err := peer.NotifyContext(ctx, "two", nil)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("NotifyContext error = %v", err)
	}
	if len(peer.out) != 1 {
		t.Fatal("cancelled notification changed the full queue")
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
	for i := 0; i < count; i++ {
		select {
		case value := <-received:
			if value != i {
				t.Fatalf("notification %d = %d, want wire order", i, value)
			}
		case <-ctx.Done():
			t.Fatalf("received %d/%d notifications: %v", i, count, ctx.Err())
		}
	}
}

func TestPeerNotificationsDoNotOvertakeBlockedHandler(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	defer close(release)
	received := make(chan string, 3)
	handler := HandlerFunc(func(ctx context.Context, _ *Peer, request Request) (any, *Error) {
		if request.Method == "barrier" {
			return true, nil
		}
		if request.Method == "model.request" {
			close(started)
			select {
			case <-release:
			case <-ctx.Done():
				return nil, nil
			}
		}
		received <- request.Method
		return nil, nil
	})
	server, _, _ := startPeerPair(t, nil, handler)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := server.NotifyContext(ctx, "model.request", nil); err != nil {
		t.Fatal(err)
	}
	select {
	case <-started:
	case <-ctx.Done():
		t.Fatal("first notification did not start")
	}
	for _, method := range []string{"model.delta", "model.completed"} {
		if err := server.NotifyContext(ctx, method, nil); err != nil {
			t.Fatal(err)
		}
	}
	// The later request proves the reader is past both queued notifications.
	if _, err := server.Call(ctx, "barrier", nil); err != nil {
		t.Fatalf("request blocked behind notification: %v", err)
	}
	select {
	case method := <-received:
		t.Fatalf("%s overtook blocked model.request", method)
	case <-time.After(25 * time.Millisecond):
	}
	release <- struct{}{}
	for _, want := range []string{"model.request", "model.delta", "model.completed"} {
		select {
		case got := <-received:
			if got != want {
				t.Fatalf("notification = %s, want %s", got, want)
			}
		case <-ctx.Done():
			t.Fatal("notification worker did not drain")
		}
	}
}

func TestPeerNotificationHandlerCanCallRPCThroughBurst(t *testing.T) {
	const count = 256
	result := make(chan error, 1)
	received := make(chan int, count)
	clientHandler := HandlerFunc(func(ctx context.Context, peer *Peer, request Request) (any, *Error) {
		switch request.Method {
		case "start":
			_, err := peer.Call(ctx, "server/burst", nil)
			result <- err
		case "client/confirm":
			return true, nil
		case "chunk":
			var value int
			if err := json.Unmarshal(request.Params, &value); err != nil {
				t.Errorf("decode chunk: %v", err)
			}
			received <- value
		}
		return nil, nil
	})
	serverHandler := HandlerFunc(func(ctx context.Context, peer *Peer, _ Request) (any, *Error) {
		if _, err := peer.Call(ctx, "client/confirm", nil); err != nil {
			return nil, &Error{Code: InternalError, Message: err.Error()}
		}
		for i := 0; i < count; i++ {
			if err := peer.NotifyContext(ctx, "chunk", i); err != nil {
				return nil, &Error{Code: InternalError, Message: err.Error()}
			}
		}
		return true, nil
	})
	server, _, _ := startPeerPair(t, serverHandler, clientHandler)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.NotifyContext(ctx, "start", nil); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-result:
		if err != nil {
			t.Fatalf("RPC inside notification: %v", err)
		}
	case <-ctx.Done():
		t.Fatal("RPC response deadlocked behind notifications")
	}
	for i := 0; i < count; i++ {
		select {
		case got := <-received:
			if got != i {
				t.Fatalf("chunk %d = %d", i, got)
			}
		case <-ctx.Done():
			t.Fatalf("received %d/%d chunks", i, count)
		}
	}
}

func TestPeerCloseCancelsNotificationAndDiscardsQueue(t *testing.T) {
	started := make(chan struct{})
	cancelled := make(chan struct{})
	queued := make(chan struct{}, 1)
	handler := HandlerFunc(func(ctx context.Context, _ *Peer, request Request) (any, *Error) {
		switch request.Method {
		case "block":
			close(started)
			<-ctx.Done()
			close(cancelled)
		case "queued":
			queued <- struct{}{}
		}
		return true, nil
	})
	server, client, _ := startPeerPair(t, nil, handler)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := server.NotifyContext(ctx, "block", nil); err != nil {
		t.Fatal(err)
	}
	select {
	case <-started:
	case <-ctx.Done():
		t.Fatal("callback did not start")
	}
	if err := server.NotifyContext(ctx, "queued", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := server.Call(ctx, "barrier", nil); err != nil {
		t.Fatal(err)
	}
	_ = client.Close()
	select {
	case <-client.ServeDone():
	case <-ctx.Done():
		t.Fatal("Serve did not join cancelled callback")
	}
	select {
	case <-cancelled:
	default:
		t.Fatal("ServeDone closed before callback returned")
	}
	select {
	case <-queued:
		t.Fatal("queued callback ran after close")
	default:
	}
	if len(client.notifications) != 0 {
		t.Fatal("closed peer retained notification payloads")
	}
}

func TestPeerNotificationOverflowClosesWithoutBlockingReader(t *testing.T) {
	started := make(chan struct{})
	queued := make(chan struct{}, 1)
	handler := HandlerFunc(func(ctx context.Context, _ *Peer, request Request) (any, *Error) {
		if request.Method == "block" {
			close(started)
			<-ctx.Done()
		} else {
			queued <- struct{}{}
		}
		return nil, nil
	})
	left, right := net.Pipe()
	defer right.Close()
	peer := NewPeer(NewJSONLTransport(left, left, left.Close), handler, Options{NotificationBuffer: 1})
	defer peer.Close()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	served := make(chan error, 1)
	go func() { served <- peer.Serve(ctx) }()
	transport := NewJSONLTransport(right, right, nil)
	if err := transport.WriteFrame([]byte(`{"jsonrpc":"2.0","method":"block"}`)); err != nil {
		t.Fatal(err)
	}
	select {
	case <-started:
	case <-ctx.Done():
		t.Fatal("callback did not start")
	}
	for i := 0; i < 2; i++ {
		if err := transport.WriteFrame([]byte(`{"jsonrpc":"2.0","method":"queued"}`)); err != nil {
			t.Fatal(err)
		}
	}
	select {
	case err := <-served:
		if !errors.Is(err, ErrNotificationOverloaded) {
			t.Fatalf("Serve error = %v, want notification overflow", err)
		}
	case <-ctx.Done():
		t.Fatal("overflow blocked the reader")
	}
	select {
	case <-peer.done:
	default:
		t.Fatal("overflow did not close peer")
	}
	select {
	case <-queued:
		t.Fatal("overflow dispatched queued callback")
	default:
	}
	if len(peer.notifications) != 0 {
		t.Fatal("overflow retained queued payloads")
	}
}

func TestNotificationBufferNormalization(t *testing.T) {
	for _, value := range []int{-1, 0, 1, 7} {
		want := value
		if value <= 0 {
			want = 1024
		}
		if got := (Options{NotificationBuffer: value}).normalized().NotificationBuffer; got != want {
			t.Fatalf("NotificationBuffer %d normalized to %d, want %d", value, got, want)
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
