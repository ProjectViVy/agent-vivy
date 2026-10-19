package acp

import (
	"context"
	"encoding/json"
	"io"
	"strings"
	"testing"
	"time"
)

// startConn wires an agent to a real SDK stdio connection over pipes and
// returns the client-side writer plus a decoded-frame reader.
func startConn(t *testing.T, a *agent) (io.WriteCloser, <-chan json.RawMessage, func() *agent) {
	t.Helper()
	clientToAgentR, clientToAgentW := io.Pipe()
	agentToClientR, agentToClientW := io.Pipe()

	conn, err := newConnection(a, clientToAgentR, agentToClientW)
	if err != nil {
		t.Fatalf("newConnection: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	if err := conn.Start(ctx); err != nil {
		cancel()
		t.Fatalf("conn.Start: %v", err)
	}

	frames := make(chan json.RawMessage, 64)
	go func() {
		defer close(frames)
		dec := json.NewDecoder(agentToClientR)
		for {
			var raw json.RawMessage
			if err := dec.Decode(&raw); err != nil {
				return
			}
			frames <- raw
		}
	}()

	t.Cleanup(func() {
		cancel()
		_ = conn.Close()
		_ = clientToAgentW.Close()
	})
	getConn := func() *agent { return a }
	return clientToAgentW, frames, getConn
}

func writeFrame(t *testing.T, w io.Writer, payload string) {
	t.Helper()
	if _, err := io.WriteString(w, payload+"\n"); err != nil {
		t.Fatalf("write frame: %v", err)
	}
}

func awaitFrame(t *testing.T, frames <-chan json.RawMessage) json.RawMessage {
	t.Helper()
	select {
	case f, ok := <-frames:
		if !ok {
			t.Fatal("frame channel closed before a response arrived")
		}
		return f
	case <-time.After(10 * time.Second):
		t.Fatal("no response frame within 10s")
		return nil
	}
}

func TestACPTransportBoundary(t *testing.T) {
	t.Run("oversized inbound frame terminates the connection", func(t *testing.T) {
		a := newAgent(&fakeHost{})
		clientToAgentR, clientToAgentW := io.Pipe()
		conn, err := newConnection(a, clientToAgentR, io.Discard)
		if err != nil {
			t.Fatalf("newConnection: %v", err)
		}
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		if err := conn.Start(ctx); err != nil {
			t.Fatalf("start: %v", err)
		}
		defer conn.Close()
		defer clientToAgentW.Close()

		// A line exceeding the 1 MiB inbound cap must tear the connection
		// down instead of growing memory.
		big := `{"jsonrpc":"2.0","id":1,"method":"session/new","params":{"cwd":"/","pad":"` +
			strings.Repeat("x", 2*1024*1024) + `"}}`
		go func() { _, _ = io.WriteString(clientToAgentW, big+"\n") }()

		select {
		case <-conn.Done():
			if conn.Err() == nil {
				t.Fatal("oversized frame closed the connection without an error")
			}
		case <-time.After(10 * time.Second):
			t.Fatal("oversized frame did not terminate the connection")
		}
	})

	t.Run("well-sized frame is answered", func(t *testing.T) {
		a := newAgent(&fakeHost{})
		w, frames, _ := startConn(t, a)
		writeFrame(t, w, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":1,"clientCapabilities":{},"clientInfo":{"name":"probe","version":"0"}}}`)
		resp := awaitFrame(t, frames)
		var parsed struct {
			ID     json.RawMessage `json:"id"`
			Result json.RawMessage `json:"result"`
			Error  *struct {
				Code int `json:"code"`
			} `json:"error"`
		}
		if err := json.Unmarshal(resp, &parsed); err != nil {
			t.Fatalf("response is not JSON-RPC: %v", err)
		}
		if parsed.ID == nil {
			t.Fatalf("response missing id: %s", string(resp))
		}
	})

	t.Run("client disconnect ends the connection", func(t *testing.T) {
		a := newAgent(&fakeHost{})
		clientToAgentR, clientToAgentW := io.Pipe()
		conn, err := newConnection(a, clientToAgentR, io.Discard)
		if err != nil {
			t.Fatalf("newConnection: %v", err)
		}
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		if err := conn.Start(ctx); err != nil {
			t.Fatalf("start: %v", err)
		}
		defer conn.Close()

		// The client going away (write side closed) must end the run.
		_ = clientToAgentW.Close()
		select {
		case <-conn.Done():
		case <-time.After(10 * time.Second):
			t.Fatal("client EOF did not terminate the connection")
		}
	})
}
