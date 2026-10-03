package app

// DN-0 fixture capture harness (agent-diva ledger aid; not part of the
// shipped suite). Composes the gateway-less app, drives the core control
// surface over DialControl, and writes a redacted request/response/event
// transcript for agent-diva/docs/plans/diva-next/fixtures/core-rpc.json.

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	controlrpc "agent-vivy/internal/rpc"
	"agent-vivy/internal/runtime"
)

type dn0Record struct {
	Seq       int             `json:"seq"`
	Direction string          `json:"direction"` // request | response | notification | error
	Method    string          `json:"method,omitempty"`
	Params    json.RawMessage `json:"params,omitempty"`
	Result    json.RawMessage `json:"result,omitempty"`
	RPCError  *dn0Err         `json:"error,omitempty"`
}

type dn0Err struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

type dn0Capture struct {
	t      *testing.T
	client *controlrpc.Peer

	mu            sync.Mutex
	seq           int
	records       []dn0Record
	notifications []dn0Record
}

func (c *dn0Capture) call(method string, params any) json.RawMessage {
	c.t.Helper()
	c.mu.Lock()
	c.seq++
	rec := dn0Record{Seq: c.seq, Direction: "request", Method: method}
	if params != nil {
		rec.Params, _ = json.Marshal(params)
	}
	c.records = append(c.records, rec)
	c.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	result, err := c.client.Call(ctx, method, params)

	c.mu.Lock()
	defer c.mu.Unlock()
	c.seq++
	out := dn0Record{Seq: c.seq, Method: method}
	if err != nil {
		out.Direction = "error"
		if rpcErr, ok := err.(*controlrpc.Error); ok {
			out.RPCError = &dn0Err{Code: rpcErr.Code, Message: rpcErr.Message, Data: rpcErr.Data}
		} else {
			out.RPCError = &dn0Err{Code: -1, Message: err.Error()}
		}
	} else {
		out.Direction = "response"
		out.Result = result
	}
	c.records = append(c.records, out)
	return result
}

// Handle receives server-initiated notifications ("run/event",
// "run/stream_error", "session/work/*") on the client peer.
func (c *dn0Capture) Handle(_ context.Context, _ *controlrpc.Peer, req controlrpc.Request) (any, *controlrpc.Error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.seq++
	c.notifications = append(c.notifications, dn0Record{
		Seq: c.seq, Direction: "notification", Method: req.Method, Params: req.Params,
	})
	return nil, nil
}

func (c *dn0Capture) waitForNotification(timeout time.Duration, pred func(dn0Record) bool) dn0Record {
	c.t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		c.mu.Lock()
		for _, n := range c.notifications {
			if pred(n) {
				c.mu.Unlock()
				return n
			}
		}
		c.mu.Unlock()
		time.Sleep(25 * time.Millisecond)
	}
	c.t.Fatalf("notification predicate not met within %s; got %v", timeout, c.notifications)
	return dn0Record{}
}

func decodeOK(t *testing.T, raw json.RawMessage, method string) map[string]any {
	t.Helper()
	if raw == nil {
		t.Fatalf("%s returned no result (error captured)", method)
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("%s: decode %s: %v", method, raw, err)
	}
	return out
}

// TestDN0CaptureCoreTranscript exercises the pinned core surface end to end
// and dumps the transcript to $DN0_CAPTURE_OUT (default /tmp/dn0-core-rpc-capture.json).
func TestDN0CaptureCoreTranscript(t *testing.T) {
	// Compose manually (composeGatewayless passes nil notification handler;
	// capture needs one).
	runtime.SetEngineVersionOverride(pinnedEinoVersion)
	t.Cleanup(func() { runtime.SetEngineVersionOverride("") })
	t.Setenv("DEEPSEEK_API_KEY", "facehost-test-key")
	t.Setenv("VIVY_PROVIDER", "deepseek")
	srv := scriptedDeepSeekServer(t, "loopback note", "loopback done")
	t.Setenv("VIVY_API_BASE", srv.URL)

	a, err := New(context.Background(), newDeepSeekTestConfig(t), WithoutEars(), WithoutGateway())
	if err != nil {
		t.Fatalf("compose gateway-less: %v", err)
	}
	t.Cleanup(func() { _ = a.backend.Close() })
	dialCtx, cancelDial := context.WithCancel(context.Background())
	t.Cleanup(cancelDial)

	cap := &dn0Capture{t: t}
	client, err := a.DialControl(dialCtx, cap)
	if err != nil {
		t.Fatalf("dial control: %v", err)
	}
	cap.client = client

	cap.call("initialize", nil)
	session := decodeOK(t, cap.call("session/create", map[string]any{"title": "dn0-capture"}), "session/create")
	sessionID, _ := session["id"].(string)
	cap.call("session/list", nil)
	cap.call("session/get", map[string]any{"session_id": sessionID})

	// Subscribe before the turn so run.started is captured.
	turn := decodeOK(t, cap.call("turn/start", map[string]any{"session_id": sessionID, "text": "echo something"}), "turn/start")
	runID, _ := turn["run_id"].(string)
	cap.call("run/subscribe", map[string]any{"run_id": runID})
	cap.call("run/get", map[string]any{"run_id": runID})

	// Wait for the approval to surface, capture its tool.approval_required event.
	var approvalID string
	deadline := time.Now().Add(15 * time.Second)
	for approvalID == "" && time.Now().Before(deadline) {
		list := cap.call("approval/list", nil)
		items, _ := decodeOK(t, list, "approval/list")["approvals"].([]any)
		for _, item := range items {
			if m, ok := item.(map[string]any); ok {
				approvalID, _ = m["id"].(string)
			}
		}
		if approvalID == "" {
			time.Sleep(50 * time.Millisecond)
		}
	}
	if approvalID == "" {
		t.Fatalf("no approval surfaced")
	}
	cap.call("approval/respond", map[string]any{"approval_id": approvalID, "decision": "approved", "reason": "dn0 capture"})

	// Wait for terminal.
	done := time.Now().Add(30 * time.Second)
	for {
		run := decodeOK(t, cap.call("run/get", map[string]any{"run_id": runID}), "run/get")
		if status, _ := run["status"].(string); status == "completed" {
			break
		}
		if time.Now().After(done) {
			t.Fatalf("run did not complete")
		}
		time.Sleep(50 * time.Millisecond)
	}
	cap.call("run/log", map[string]any{"run_id": runID})
	cap.call("session/messages", map[string]any{"session_id": sessionID})
	cap.call("session/rename", map[string]any{"session_id": sessionID, "title": "dn0-renamed"})
	cap.call("question/list", nil)

	// Error-envelope captures (deliberate, to pin error shape).
	cap.call("question/respond", map[string]any{"question_id": "que_nonexistent", "answer": "x"})
	cap.call("run/cancel", map[string]any{"run_id": runID}) // completed run → CodeNotFound path
	cap.call("session/delete", map[string]any{"session_id": sessionID})

	out := map[string]any{
		"captured_at":        time.Now().UTC().Format(time.RFC3339),
		"vivy_pin":           "5347032d8f18a047b67e85761c0dcc48728bee7c",
		"protocol_version":   "vivy.rpc.v1",
		"transport":          "DialControl net.Pipe JSONLTransport",
		"provider_fixture":   "scriptedDeepSeekServer (OpenAI chat.completions wire; write_note gated by prompt policy)",
		"requests_responses": cap.records,
		"notifications":      cap.notifications,
	}
	path := os.Getenv("DN0_CAPTURE_OUT")
	if path == "" {
		path = "/tmp/dn0-core-rpc-capture.json"
	}
	raw, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		t.Fatalf("marshal transcript: %v", err)
	}
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatalf("write transcript: %v", err)
	}
	t.Logf("captured %d request/response records, %d notifications -> %s",
		len(cap.records), len(cap.notifications), path)
	fmt.Println("capture written:", path)
}
