package worker

import (
	"context"
	"encoding/json"
	"net"
	"testing"
	"time"

	"agent-vivy/internal/rpc"
)

func TestRunBrokersToolCallsToParent(t *testing.T) {
	serverConn, clientConn := net.Pipe()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	serverDone := make(chan error, 1)
	go func() { serverDone <- Run(ctx, serverConn, serverConn) }()

	events := make(chan Event, 2)
	parent := rpc.NewPeer(rpc.NewJSONLTransport(clientConn, clientConn, clientConn.Close), rpc.HandlerFunc(func(_ context.Context, _ *rpc.Peer, request rpc.Request) (any, *rpc.Error) {
		switch request.Method {
		case "tool/execute":
			return ToolResult{Result: "broker-result"}, nil
		case "worker/event":
			var event Event
			if err := json.Unmarshal(request.Params, &event); err != nil {
				return nil, &rpc.Error{Code: rpc.InvalidParams, Message: err.Error()}
			}
			events <- event
			return nil, nil
		default:
			return nil, &rpc.Error{Code: rpc.MethodNotFound, Message: "unexpected method"}
		}
	}), rpc.Options{})
	parentDone := make(chan error, 1)
	go func() { parentDone <- parent.Serve(ctx) }()

	result, err := parent.Call(ctx, "initialize", map[string]string{"protocol_version": rpc.ProtocolVersion})
	if err != nil {
		t.Fatalf("initialize: %v", err)
	}
	var initialized map[string]any
	if err := json.Unmarshal(result, &initialized); err != nil {
		t.Fatalf("decode initialize: %v", err)
	}
	if initialized["protocol_version"] != rpc.ProtocolVersion {
		t.Fatalf("unexpected protocol version: %v", initialized["protocol_version"])
	}

	result, err = parent.Call(ctx, "worker/run", RunRequest{
		RunID: "child-1", ParentRunID: "parent-1", PolicyProfile: "default",
		PolicyHash: "snapshot-1", WorkspaceID: "workspace-1", Text: "hello",
		ToolName: "echo", ToolArgs: map[string]any{"text": "hello"},
	})
	if err != nil {
		t.Fatalf("worker/run: %v", err)
	}
	var runResult RunResult
	if err := json.Unmarshal(result, &runResult); err != nil {
		t.Fatalf("decode worker result: %v", err)
	}
	if runResult.Status != "completed" || runResult.Result != "broker-result" {
		t.Fatalf("unexpected worker result: %+v", runResult)
	}

	for i := 0; i < 2; i++ {
		select {
		case event := <-events:
			if event.RunID != "child-1" {
				t.Fatalf("unexpected event: %+v", event)
			}
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for worker event")
		}
	}
	cancel()
	_ = serverConn.Close()
	_ = clientConn.Close()
	select {
	case <-serverDone:
	case <-time.After(time.Second):
		t.Fatal("worker did not stop")
	}
	select {
	case <-parentDone:
	case <-time.After(time.Second):
		t.Fatal("parent peer did not stop")
	}
}
