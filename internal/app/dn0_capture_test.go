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
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	controlrpc "agent-vivy/internal/rpc"
	"agent-vivy/internal/runtime"
	"agent-vivy/internal/tools"
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
	result, _ := c.callBoth(method, params)
	return result
}

// callBoth captures one request/response or request/error pair and also
// returns the client-side error so a closure case can assert error codes.
func (c *dn0Capture) callBoth(method string, params any) (json.RawMessage, error) {
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
	return result, err
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
	deadline := time.Now().Add(60 * time.Second)
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
		path = filepath.Join(t.TempDir(), "dn0-core-rpc-capture.json")
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

// --- DN-0C closure capture -------------------------------------------------

// dn0PNG is a valid 1x1 PNG used to prove image bytes survive the
// send/store/read path byte-for-byte.
const dn0PNGBase64 = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg=="

func dn0UserMessageID(t *testing.T, messages []map[string]any, contains string) string {
	t.Helper()
	for _, m := range messages {
		if role, _ := m["role"].(string); role != "user" {
			continue
		}
		if strings.Contains(fmt.Sprint(m["content"]), contains) {
			id, _ := m["id"].(string)
			return id
		}
	}
	return ""
}

func dn0SessionMessages(t *testing.T, cap *dn0Capture, sessionID string) []map[string]any {
	t.Helper()
	raw := decodeOK(t, cap.call("session/messages", map[string]any{
		"session_id": sessionID, "include_attachment_data": true,
	}), "session/messages")
	items, _ := raw["messages"].([]any)
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		if m, ok := item.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out
}

func dn0WaitRun(t *testing.T, cap *dn0Capture, runID string, want string) map[string]any {
	t.Helper()
	deadline := time.Now().Add(45 * time.Second)
	for time.Now().Before(deadline) {
		run := decodeOK(t, cap.call("run/get", map[string]any{"run_id": runID}), "run/get")
		if status, _ := run["status"].(string); status == want {
			return run
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("run %s did not reach %s", runID, want)
	return nil
}

func dn0WaitApproval(t *testing.T, cap *dn0Capture, seen map[string]bool) string {
	t.Helper()
	deadline := time.Now().Add(45 * time.Second)
	for time.Now().Before(deadline) {
		list := decodeOK(t, cap.call("approval/list", nil), "approval/list")
		items, _ := list["approvals"].([]any)
		for _, item := range items {
			m, _ := item.(map[string]any)
			if id, _ := m["id"].(string); id != "" && !seen[id] {
				return id
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("no new approval surfaced")
	return ""
}

func dn0WaitWork(t *testing.T, cap *dn0Capture, sessionID string, pred func(map[string]any) bool, what string) map[string]any {
	t.Helper()
	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		work := decodeOK(t, cap.call("session/work/get", map[string]any{"session_id": sessionID}), "session/work/get")
		if pred(work) {
			return work
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("work projection predicate %q not met", what)
	return nil
}

func dn0RPCErrorCode(err error) int {
	if rpcErr, ok := err.(*controlrpc.Error); ok {
		return rpcErr.Code
	}
	return 0
}

// TestDN0CaptureClosureTranscript is the DN-0C source-host capture: it freezes
// the closure-relevant chat, work and OBS producer contracts at the pinned
// VIVY source revision into a redacted fixture written to $DN0_CAPTURE_OUT.
// It deliberately uses a separate output path from the historical
// TestDN0CaptureCoreTranscript fixture (fixtures/core-rpc.json stays frozen).
func TestDN0CaptureClosureTranscript(t *testing.T) {
	runtime.SetEngineVersionOverride(pinnedEinoVersion)
	t.Cleanup(func() { runtime.SetEngineVersionOverride("") })
	t.Setenv("DEEPSEEK_API_KEY", "facehost-test-key")
	t.Setenv("VIVY_PROVIDER", "deepseek")
	srv := newPlanGoalScriptServer(t, []planGoalReply{
		{tool: tools.WriteNoteName, args: `{"content":"image turn"}`},
		{tool: tools.WriteNoteName, args: `{"content":"second turn"}`},
		{text: "loopback done"},
		{text: "edited answer"},
		{tool: tools.SubmitPlanName, args: `{"markdown":"1. capture plan step"}`},
		{text: "Plan accepted; the Goal is starting."},
		{tool: tools.ReportGoalName, args: `{"goal_id":"goal-closure-capture","revision":1,"status":"completed","reason":"captured"}`},
		{text: "Goal complete."},
	})
	t.Setenv("VIVY_API_BASE", srv.URL)

	cfg := newDeepSeekTestConfig(t)
	cfg.Tools.Enabled = append(cfg.Tools.Enabled, tools.SubmitPlanName, tools.ReportGoalName)
	a, err := New(context.Background(), cfg, WithoutEars(), WithoutGateway())
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

	checks := map[string]any{}
	mark := func(name string, ok bool, detail string) {
		checks[name] = map[string]any{"ok": ok, "detail": detail}
		if !ok {
			t.Errorf("closure assertion %s failed: %s", name, detail)
		}
	}

	cap.call("initialize", nil)
	session := decodeOK(t, cap.call("session/create", map[string]any{"title": "dn0-closure"}), "session/create")
	sessionID, _ := session["id"].(string)

	// Permission presets: admitted values plus the invalid-preset error.
	cap.call("session/set_permission", map[string]any{"session_id": sessionID, "preset": "smart"})
	_, permErr := cap.callBoth("session/set_permission", map[string]any{"session_id": sessionID, "preset": "bogus"})
	mark("permission/preset-enum", dn0RPCErrorCode(permErr) == controlrpc.InvalidParams, fmt.Sprint(permErr))
	cap.call("session/set_permission", map[string]any{"session_id": sessionID, "preset": "cautious"})

	// Attachment decode and MIME failures stay InvalidParams.
	_, b64Err := cap.callBoth("turn/start", map[string]any{"session_id": sessionID, "text": "x",
		"attachments": []map[string]any{{"mime_type": "image/png", "data": "@@@"}}})
	mark("attachment/bad-base64", dn0RPCErrorCode(b64Err) == controlrpc.InvalidParams, fmt.Sprint(b64Err))
	_, mimeErr := cap.callBoth("turn/start", map[string]any{"session_id": sessionID, "text": "x",
		"attachments": []map[string]any{{"mime_type": "application/pdf", "data": dn0PNGBase64}}})
	mark("attachment/unsupported-mime", dn0RPCErrorCode(mimeErr) == controlrpc.InvalidParams, fmt.Sprint(mimeErr))
	_, paramErr := cap.callBoth("turn/start", map[string]any{"session_id": "", "text": "x"})
	mark("turn/malformed-params", dn0RPCErrorCode(paramErr) == controlrpc.InvalidParams, fmt.Sprint(paramErr))

	// Image turn: approval-gated run stays active, so the session is busy.
	turn1 := decodeOK(t, cap.call("turn/start", map[string]any{
		"session_id": sessionID, "text": "image echo",
		"attachments": []map[string]any{{"name": "dot.png", "mime_type": "image/png", "data": dn0PNGBase64}},
	}), "turn/start")
	run1, _ := turn1["run_id"].(string)
	seenApprovals := map[string]bool{}
	approval1 := dn0WaitApproval(t, cap, seenApprovals)
	seenApprovals[approval1] = true

	messages := dn0SessionMessages(t, cap, sessionID)
	msgImage := dn0UserMessageID(t, messages, "image echo")
	if msgImage == "" {
		t.Fatalf("image user message not found in %v", messages)
	}
	_, busyErr := cap.callBoth("session/rewind", map[string]any{"session_id": sessionID, "message_id": msgImage})
	mark("rewind/busy-conflict", dn0RPCErrorCode(busyErr) == controlrpc.CodeConflict, fmt.Sprint(busyErr))

	cap.call("run/cancel", map[string]any{"run_id": run1})
	dn0WaitRun(t, cap, run1, "cancelled")
	_, cancelErr := cap.callBoth("run/cancel", map[string]any{"run_id": run1})
	mark("cancel/terminal-not-found", dn0RPCErrorCode(cancelErr) == controlrpc.CodeNotFound, fmt.Sprint(cancelErr))

	// Reopen semantics: a fresh turn is admitted after the cancelled run.
	turn2 := decodeOK(t, cap.call("turn/start", map[string]any{"session_id": sessionID, "text": "second turn"}), "turn/start")
	run2, _ := turn2["run_id"].(string)
	approval2 := dn0WaitApproval(t, cap, seenApprovals)
	seenApprovals[approval2] = true
	cap.call("approval/respond", map[string]any{"approval_id": approval2, "decision": "approved", "reason": "closure capture"})
	dn0WaitRun(t, cap, run2, "completed")

	// Image bytes survive the store/read path byte-for-byte.
	messages = dn0SessionMessages(t, cap, sessionID)
	imageBytesOK := false
	for _, m := range messages {
		if m["id"] != msgImage {
			continue
		}
		atts, _ := m["attachments"].([]any)
		for _, att := range atts {
			a, _ := att.(map[string]any)
			dataURL, _ := a["data_url"].(string)
			if strings.HasPrefix(dataURL, "data:image/png;base64,") &&
				strings.TrimPrefix(dataURL, "data:image/png;base64,") == dn0PNGBase64 {
				imageBytesOK = true
			}
		}
	}
	mark("attachment/bytes-roundtrip", imageBytesOK, "stored data_url must equal the sent bytes")

	// stats/tokens projection v2 plus coverage; invalid period is an error.
	stats := decodeOK(t, cap.call("stats/tokens", nil), "stats/tokens")
	mark("stats/projection-v2", stats["projection_version"] == float64(2), fmt.Sprint(stats["projection_version"]))
	_, coverageOK := stats["coverage"].(map[string]any)
	mark("stats/coverage-shape", coverageOK, fmt.Sprint(stats["coverage"]))
	_, periodErr := cap.callBoth("stats/tokens", map[string]any{"period": "bogus"})
	mark("stats/invalid-period", dn0RPCErrorCode(periodErr) == controlrpc.InvalidParams, fmt.Sprint(periodErr))

	// trajectory/session projection v2 with safe integer sequences.
	traj := decodeOK(t, cap.call("trajectory/session", map[string]any{"session_id": sessionID}), "trajectory/session")
	mark("trajectory/projection-v2", traj["projection_version"] == float64(2), fmt.Sprint(traj["projection_version"]))
	seqSafe := true
	if wm, ok := traj["watermarks"].(map[string]any); ok {
		for _, v := range wm {
			if n, ok := v.(float64); !ok || n < 0 || n >= 1<<53 {
				seqSafe = false
			}
		}
	} else {
		seqSafe = false
	}
	mark("trajectory/safe-watermarks", seqSafe, fmt.Sprint(traj["watermarks"]))
	// Unknown sessions return an empty projection, not an error.
	missingTraj, trajErr := cap.callBoth("trajectory/session", map[string]any{"session_id": "sess_missing"})
	missingRecords := 1
	if trajErr == nil {
		if mt := decodeOK(t, missingTraj, "trajectory/session missing"); mt != nil {
			missingRecords = len(mt["records"].([]any))
		}
	}
	mark("trajectory/unknown-session-empty", trajErr == nil && missingRecords == 0, fmt.Sprintf("err=%v records=%d", trajErr, missingRecords))

	// Diagnostics: gui append then bounded read; bad source and oversized
	// batches stay errors; stale cursor behavior is captured as observed.
	cap.call("diagnostics/gui/append", map[string]any{"records": []map[string]any{
		{"level": "info", "component": "dn0", "message": "closure capture probe"},
	}})
	guiPage := decodeOK(t, cap.call("diagnostics/logs", map[string]any{"source": "gui"}), "diagnostics/logs")
	guiRecords, _ := guiPage["records"].([]any)
	mark("diagnostics/gui-roundtrip", len(guiRecords) > 0, fmt.Sprint(guiPage["records"]))
	_, srcErr := cap.callBoth("diagnostics/logs", map[string]any{"source": "bogus"})
	mark("diagnostics/invalid-source", dn0RPCErrorCode(srcErr) == controlrpc.InvalidParams, fmt.Sprint(srcErr))
	cap.call("diagnostics/logs", map[string]any{"source": "gui", "after": "bogus-cursor"})
	oversized := make([]map[string]any, 501)
	for i := range oversized {
		oversized[i] = map[string]any{"level": "info", "component": "dn0", "message": fmt.Sprintf("overflow-%d", i)}
	}
	_, appendErr := cap.callBoth("diagnostics/gui/append", map[string]any{"records": oversized})
	mark("diagnostics/oversized-batch", dn0RPCErrorCode(appendErr) == controlrpc.InvalidParams, fmt.Sprint(appendErr))
	cap.call("diagnostics/logs", map[string]any{"source": "runtime", "limit": 1})

	// Child read DTOs: empty list and not-found detail, no control work.
	childList := decodeOK(t, cap.call("child/list", map[string]any{"parent_run_id": run2}), "child/list")
	_, childrenOK := childList["children"]
	mark("children/list-shape", childrenOK, fmt.Sprint(childList))
	_, childErr := cap.callBoth("child/get", map[string]any{"run_id": "run_missing"})
	mark("children/not-found", dn0RPCErrorCode(childErr) == controlrpc.CodeNotFound, fmt.Sprint(childErr))

	// Atomic text edit: one new run; the edited suffix is replaced.
	messages = dn0SessionMessages(t, cap, sessionID)
	msgSecond := dn0UserMessageID(t, messages, "second turn")
	if msgSecond == "" {
		t.Fatalf("second user message not found in %v", messages)
	}
	edit := decodeOK(t, cap.call("session/edit", map[string]any{
		"session_id": sessionID, "message_id": msgSecond, "text": "edited echo",
	}), "session/edit")
	run3, _ := edit["run_id"].(string)
	mark("edit/run-admitted", run3 != "" && run3 != run2, fmt.Sprint(edit))
	dn0WaitRun(t, cap, run3, "completed")
	messages = dn0SessionMessages(t, cap, sessionID)
	mark("edit/suffix-replaced", dn0UserMessageID(t, messages, "second turn") == "" &&
		dn0UserMessageID(t, messages, "edited echo") != "", fmt.Sprint(messages))
	// A missing cutoff message surfaces as an internal error from EditSession.
	_, editErr := cap.callBoth("session/edit", map[string]any{
		"session_id": sessionID, "message_id": "msg_missing", "text": "x",
	})
	mark("edit/missing-message", dn0RPCErrorCode(editErr) == controlrpc.InternalError, fmt.Sprint(editErr))

	// Fork at the image message is inclusive; the rewind at the same message
	// leaves zero messages, proving the inclusive cutoff.
	fork := decodeOK(t, cap.call("session/fork", map[string]any{
		"session_id": sessionID, "message_id": msgImage, "title": "closure-fork",
	}), "session/fork")
	childID, _ := fork["session_id"].(string)
	mark("fork/inclusive-copy", childID != "" && fork["copied_count"] == float64(1), fmt.Sprint(fork))
	if childID != "" {
		cap.call("session/messages", map[string]any{"session_id": childID})
	}

	rew := decodeOK(t, cap.call("session/rewind", map[string]any{
		"session_id": sessionID, "message_id": msgImage,
	}), "session/rewind")
	mark("rewind/inclusive-cutoff", rew["remaining_count"] == float64(0) && rew["cutoff_message_id"] == msgImage, fmt.Sprint(rew))
	messages = dn0SessionMessages(t, cap, sessionID)
	mark("rewind/view-empty", len(messages) == 0, fmt.Sprint(messages))
	_, rewErr := cap.callBoth("session/rewind", map[string]any{"session_id": sessionID, "message_id": "bogus"})
	mark("rewind/invalid-cutoff", dn0RPCErrorCode(rewErr) == controlrpc.CodeNotFound, fmt.Sprint(rewErr))

	// plan/decide with start_goal drives a real Goal round to completion.
	cap.call("plan/enter", map[string]any{"session_id": sessionID, "request_id": "enter-closure", "expected_version": 0})
	turn4 := decodeOK(t, cap.call("turn/start", map[string]any{"session_id": sessionID, "text": "prepare a plan"}), "turn/start")
	_, _ = turn4["run_id"].(string)
	work := dn0WaitWork(t, cap, sessionID, func(w map[string]any) bool {
		plan, _ := w["plan"].(map[string]any)
		return plan["review_status"] == "pending"
	}, "pending plan review")
	plan, _ := work["plan"].(map[string]any)
	cap.call("plan/decide", map[string]any{
		"session_id": sessionID, "request_id": "decide-closure", "expected_version": work["version"],
		"submission_id": plan["submission_id"], "action": "start_goal", "goal_id": "goal-closure-capture",
		"objective": "capture the goal round", "max_rounds": 2,
	})
	dn0WaitWork(t, cap, sessionID, func(w map[string]any) bool {
		goal, _ := w["goal"].(map[string]any)
		return goal["phase"] == "completed"
	}, "completed goal")
	cap.call("session/work/get", map[string]any{"session_id": sessionID})
	cap.call("plan/leave", map[string]any{"session_id": sessionID, "request_id": "leave-closure"})

	out := map[string]any{
		"captured_at":        time.Now().UTC().Format(time.RFC3339),
		"capture_kind":       "source_host",
		"vivy_pin":           "1db8b55ce905ee4a212326e7801e0958e7f4376a",
		"protocol_version":   "vivy.rpc.v1",
		"transport":          "DialControl net.Pipe JSONLTransport",
		"provider_fixture":   "newPlanGoalScriptServer (scripted DeepSeek/OpenAI SSE wire; write_note gated by prompt policy)",
		"assertions":         checks,
		"requests_responses": cap.records,
		"notifications":      cap.notifications,
	}
	path := os.Getenv("DN0_CAPTURE_OUT")
	if path == "" {
		path = filepath.Join(t.TempDir(), "dn0-closure-chat-obs.json")
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
