package acp

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	acp "github.com/eino-contrib/acp"
)

// runHost stubs the Control lifecycle calls used by Prompt. turn/start and
// run/subscribe may be gated by barriers; run/event notifications are
// emitted through emitEvent.
type runHost struct {
	*fakeHost

	mu         sync.Mutex
	nextRun    int
	nextSub    int
	turnGate   chan struct{}
	subGate    chan struct{}
	runs       []string
	subs       []string
	cancels    []string
	unsubs     []string
	sessionIDs []string
}

func newRunHost() *runHost {
	h := &runHost{turnGate: make(chan struct{}), subGate: make(chan struct{})}
	close(h.turnGate)
	close(h.subGate)
	h.fakeHost = &fakeHost{callFn: h.call}
	return h
}

func (h *runHost) call(ctx context.Context, method string, params any) (json.RawMessage, error) {
	switch method {
	case "initialize":
		return json.RawMessage(`{"protocol_version":1,"capabilities":["session","turn","run","run.subscribe","approval","question","review"],"code_mode_available":true}`), nil
	case "session/create":
		h.mu.Lock()
		id := "sess_" + itoa(len(h.sessionIDs)+1)
		h.sessionIDs = append(h.sessionIDs, id)
		h.mu.Unlock()
		var p struct {
			WorkspacePath string `json:"workspace_path"`
		}
		if b, _ := json.Marshal(params); b != nil {
			_ = json.Unmarshal(b, &p)
		}
		out, _ := json.Marshal(map[string]any{"id": id, "workspace_path": p.WorkspacePath})
		return json.RawMessage(out), nil
	case "turn/start":
		select {
		case <-h.turnGate:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
		h.mu.Lock()
		h.nextRun++
		runID := "run_" + itoa(h.nextRun)
		h.runs = append(h.runs, runID)
		h.mu.Unlock()
		out, _ := json.Marshal(map[string]any{"run_id": runID, "status": "accepted"})
		return json.RawMessage(out), nil
	case "run/subscribe":
		select {
		case <-h.subGate:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
		h.mu.Lock()
		h.nextSub++
		subID := "sub_" + itoa(h.nextSub)
		h.subs = append(h.subs, subID)
		h.mu.Unlock()
		out, _ := json.Marshal(map[string]any{"subscription_id": subID, "run_id": runIDFromParams(t0{}, params), "after_seq": 0})
		return json.RawMessage(out), nil
	case "run/cancel":
		h.mu.Lock()
		h.cancels = append(h.cancels, stringParam(params, "run_id"))
		h.mu.Unlock()
		out, _ := json.Marshal(map[string]any{"status": "cancelling"})
		return json.RawMessage(out), nil
	case "run/unsubscribe":
		h.mu.Lock()
		h.unsubs = append(h.unsubs, stringParam(params, "subscription_id"))
		h.mu.Unlock()
		return json.RawMessage(`{"unsubscribed":true}`), nil
	}
	return nil, errors.New("unexpected call: " + method)
}

type t0 struct{}

func runIDFromParams(_ t0, params any) string { return stringParam(params, "run_id") }

func stringParam(params any, key string) string {
	b, _ := json.Marshal(params)
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		return ""
	}
	s, _ := m[key].(string)
	return s
}

func (h *runHost) emitRunEvent(subID, runID string, seq int64, typ string, payload string) {
	params, _ := json.Marshal(map[string]any{
		"subscription_id": subID,
		"event": map[string]any{
			"run_id":          runID,
			"seq":             seq,
			"type":            typ,
			"created_at":      time.Now().UnixMilli(),
			"payload_version": 1,
			"payload":         json.RawMessage(payload),
		},
	})
	h.fakeHost.emit("run/event", params)
}

func (h *runHost) sessionID(i int) string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.sessionIDs[i]
}

func (h *runHost) runID(i int) string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.runs[i]
}

func (h *runHost) subID(i int) string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.subs[i]
}

func (h *runHost) cancelCount() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.cancels)
}

func (h *runHost) unsubsCount() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.unsubs)
}

// newTestAgent builds an initialized agent owning n sessions on h.
func newTestAgent(t *testing.T, h *runHost, sessions int) *agent {
	t.Helper()
	a := newAgent(h.fakeHost)
	h.fakeHost.OnEvent(a.onEvent)
	mustInitialize(t, a)
	for i := 0; i < sessions; i++ {
		if _, err := a.NewSession(context.Background(), acp.NewSessionRequest{Cwd: "/tmp"}); err != nil {
			t.Fatalf("session %d: %v", i, err)
		}
	}
	return a
}

func promptText(sessionID, text string) acp.PromptRequest {
	return acp.PromptRequest{
		SessionID: acp.SessionID(sessionID),
		Prompt:    []acp.ContentBlock{textBlock(text)},
	}
}

// waitForSubs reports whether n subscriptions were bound within 10 s.
func (h *runHost) waitForSubs(n int) bool {
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		h.mu.Lock()
		if len(h.subs) >= n {
			h.mu.Unlock()
			return true
		}
		h.mu.Unlock()
		time.Sleep(5 * time.Millisecond)
	}
	return false
}

// emitTerminal emits a terminal event once the first subscription is bound,
// using the real run/subscription IDs the adapter learned.
func (h *runHost) emitTerminal(_ string, _ string, typ string, payload string) {
	if !h.waitForSubs(1) {
		return
	}
	h.emitRunEvent(h.subID(0), h.runID(0), 1, typ, payload)
}

func TestPromptAdmissionAndCancelBeforeRunID(t *testing.T) {
	t.Run("cancel during turn/start applies once run ID arrives", func(t *testing.T) {
		h := newRunHost()
		h.turnGate = make(chan struct{})
		a := newTestAgent(t, h, 1)
		sess := h.sessionID(0)

		done := make(chan struct{})
		var resp acp.PromptResponse
		var perr error
		go func() {
			defer close(done)
			resp, perr = a.Prompt(context.Background(), promptText(sess, "hi"))
		}()

		// Wait until turn/start is in flight, then cancel before run ID.
		h.mu.Lock()
		h.mu.Unlock()
		time.Sleep(50 * time.Millisecond)
		if err := a.SessionCancel(context.Background(), acp.CancelNotification{SessionID: acp.SessionID(sess)}); err != nil {
			t.Fatalf("cancel: %v", err)
		}
		close(h.turnGate)
		go h.emitTerminal("", "", "run.cancelled", `{"reason":"user_requested"}`)

		select {
		case <-done:
		case <-time.After(10 * time.Second):
			t.Fatal("prompt did not return")
		}
		if perr != nil {
			t.Fatalf("prompt error = %v", perr)
		}
		if resp.StopReason != acp.StopReasonCancelled {
			t.Fatalf("stopReason = %q", resp.StopReason)
		}
		// run/cancel was issued for the just-learned run.
		if h.cancelCount() == 0 {
			t.Fatal("run/cancel was not issued for the latched cancel")
		}
	})

	t.Run("prompt is validated before capacity is reserved", func(t *testing.T) {
		h := newRunHost()
		a := newTestAgent(t, h, 1)
		sess := h.sessionID(0)
		_, err := a.Prompt(context.Background(), acp.PromptRequest{
			SessionID: acp.SessionID(sess),
			Prompt:    []acp.ContentBlock{{Image: &acp.ContentBlockImage{}}},
		})
		var re *acp.RPCError
		if !errors.As(err, &re) || re.Code != -32602 {
			t.Fatalf("error = %v", err)
		}
		if len(h.runs) != 0 {
			t.Fatal("invalid prompt created a run")
		}
		// Slot is free for the next prompt.
		go h.emitTerminal("", "", "run.completed", `{"summary":"ok"}`)
		resp, err := a.Prompt(context.Background(), promptText(sess, "again"))
		if err != nil {
			t.Fatalf("second prompt: %v", err)
		}
		if resp.StopReason != acp.StopReasonEndTurn {
			t.Fatalf("stopReason = %q", resp.StopReason)
		}
	})

	t.Run("session busy while a prompt is active", func(t *testing.T) {
		h := newRunHost()
		h.subGate = make(chan struct{})
		a := newTestAgent(t, h, 1)
		sess := h.sessionID(0)

		first := make(chan struct{})
		go func() {
			defer close(first)
			_, _ = a.Prompt(context.Background(), promptText(sess, "one"))
		}()
		time.Sleep(50 * time.Millisecond)
		_, err := a.Prompt(context.Background(), promptText(sess, "two"))
		var re *acp.RPCError
		if !errors.As(err, &re) || re.Code != -32001 {
			t.Fatalf("busy error = %v", err)
		}
		var data map[string]string
		_ = json.Unmarshal(re.Data, &data)
		if data["reason"] != "SESSION_BUSY" {
			t.Fatalf("reason = %q", data["reason"])
		}
		close(h.subGate)
		go h.emitTerminal("", "", "run.completed", `{"summary":"ok"}`)
		<-first
	})

	t.Run("unknown session is -32002 before Control", func(t *testing.T) {
		h := newRunHost()
		a := newTestAgent(t, h, 0)
		_, err := a.Prompt(context.Background(), promptText("sess_zzz", "hi"))
		var re *acp.RPCError
		if !errors.As(err, &re) || re.Code != -32002 {
			t.Fatalf("error = %v", err)
		}
		if len(h.runs) != 0 {
			t.Fatal("unknown session reached turn/start")
		}
	})
}

func TestSessionCancellationIsolation(t *testing.T) {
	h := newRunHost()
	h.subGate = make(chan struct{})
	a := newTestAgent(t, h, 2)
	sessA, sessB := h.sessionID(0), h.sessionID(1)

	done := make(chan struct{})
	var resp acp.PromptResponse
	go func() {
		defer close(done)
		resp, _ = a.Prompt(context.Background(), promptText(sessA, "work"))
	}()
	time.Sleep(50 * time.Millisecond)

	// Cancel the other owned session and an unknown session: neither must
	// touch session A's run.
	if err := a.SessionCancel(context.Background(), acp.CancelNotification{SessionID: acp.SessionID(sessB)}); err != nil {
		t.Fatalf("cancel B: %v", err)
	}
	if err := a.SessionCancel(context.Background(), acp.CancelNotification{SessionID: "sess_foreign"}); err != nil {
		t.Fatalf("cancel foreign: %v", err)
	}
	if n := h.cancelCount(); n != 0 {
		t.Fatalf("foreign/other-session cancel issued %d run/cancel calls", n)
	}

	close(h.subGate)
	go h.emitTerminal("", "", "run.completed", `{"summary":"done"}`)
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("prompt A did not return")
	}
	if resp.StopReason != acp.StopReasonEndTurn {
		t.Fatalf("stopReason = %q", resp.StopReason)
	}
}

func TestIdleCancelDoesNotAffectNextPrompt(t *testing.T) {
	t.Run("stale latch is discarded", func(t *testing.T) {
		h := newRunHost()
		a := newTestAgent(t, h, 1)
		sess := h.sessionID(0)

		old := cancelLatchTTL
		cancelLatchTTL = 50 * time.Millisecond
		defer func() { cancelLatchTTL = old }()

		if err := a.SessionCancel(context.Background(), acp.CancelNotification{SessionID: acp.SessionID(sess)}); err != nil {
			t.Fatalf("cancel: %v", err)
		}
		time.Sleep(150 * time.Millisecond) // latch now stale

		go h.emitTerminal("", "", "run.completed", `{"summary":"ok"}`)
		resp, err := a.Prompt(context.Background(), promptText(sess, "fresh"))
		if err != nil {
			t.Fatalf("prompt: %v", err)
		}
		if resp.StopReason != acp.StopReasonEndTurn {
			t.Fatalf("stale latch affected prompt: %q", resp.StopReason)
		}
	})

	t.Run("unknown session cancel creates no state", func(t *testing.T) {
		h := newRunHost()
		a := newTestAgent(t, h, 1)
		if err := a.SessionCancel(context.Background(), acp.CancelNotification{SessionID: "sess_nope"}); err != nil {
			t.Fatalf("cancel: %v", err)
		}
		a.mu.Lock()
		n := len(a.sessions)
		a.mu.Unlock()
		if n != 1 {
			t.Fatalf("unknown cancel created session state: %d sessions", n)
		}
	})
}

func TestTerminalCancelLinearization(t *testing.T) {
	t.Run("durable run.completed -> end_turn", func(t *testing.T) {
		h := newRunHost()
		a := newTestAgent(t, h, 1)
		sess := h.sessionID(0)
		go h.emitTerminal("", "", "run.completed", `{"summary":"ok"}`)
		resp, err := a.Prompt(context.Background(), promptText(sess, "hi"))
		if err != nil {
			t.Fatalf("prompt: %v", err)
		}
		if resp.StopReason != acp.StopReasonEndTurn {
			t.Fatalf("stopReason = %q", resp.StopReason)
		}
	})

	t.Run("client cancel wins over terminal outcome", func(t *testing.T) {
		h := newRunHost()
		h.subGate = make(chan struct{})
		a := newTestAgent(t, h, 1)
		sess := h.sessionID(0)

		done := make(chan struct{})
		var resp acp.PromptResponse
		var perr error
		go func() {
			defer close(done)
			resp, perr = a.Prompt(context.Background(), promptText(sess, "hi"))
		}()
		time.Sleep(50 * time.Millisecond)
		close(h.subGate)
		// run ID is known now (turn/start returned); cancel wins over the
		// run.cancelled terminal that follows.
		if err := a.SessionCancel(context.Background(), acp.CancelNotification{SessionID: acp.SessionID(sess)}); err != nil {
			t.Fatalf("cancel: %v", err)
		}
		if !h.waitForSubs(1) {
			t.Fatal("subscription not bound")
		}
		h.emitRunEvent(h.subID(0), h.runID(0), 1, "run.cancelled", `{"reason":"user_requested"}`)
		select {
		case <-done:
		case <-time.After(10 * time.Second):
			t.Fatal("no response")
		}
		if perr != nil {
			t.Fatalf("error = %v", perr)
		}
		if resp.StopReason != acp.StopReasonCancelled {
			t.Fatalf("stopReason = %q", resp.StopReason)
		}
	})

	t.Run("run.cancelled without client cancel -> -32800", func(t *testing.T) {
		h := newRunHost()
		a := newTestAgent(t, h, 1)
		sess := h.sessionID(0)
		go h.emitTerminal("", "", "run.cancelled", `{"reason":"recovery"}`)
		_, err := a.Prompt(context.Background(), promptText(sess, "hi"))
		var re *acp.RPCError
		if !errors.As(err, &re) || re.Code != -32800 {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("run.failed -> safe error, never end_turn", func(t *testing.T) {
		h := newRunHost()
		a := newTestAgent(t, h, 1)
		sess := h.sessionID(0)
		go h.emitTerminal("", "", "run.failed", `{"cause_category":"internal_error","message":"secret detail"}`)
		_, err := a.Prompt(context.Background(), promptText(sess, "hi"))
		var re *acp.RPCError
		if !errors.As(err, &re) || re.Code != -32603 {
			t.Fatalf("error = %v", err)
		}
		wire, _ := json.Marshal(re)
		if contains(wire, "secret detail") {
			t.Fatalf("failure leaked cause: %s", string(wire))
		}
	})

	t.Run("cancel after final response is a no-op", func(t *testing.T) {
		h := newRunHost()
		a := newTestAgent(t, h, 1)
		sess := h.sessionID(0)
		go h.emitTerminal("", "", "run.completed", `{"summary":"ok"}`)
		resp, err := a.Prompt(context.Background(), promptText(sess, "hi"))
		if err != nil || resp.StopReason != acp.StopReasonEndTurn {
			t.Fatalf("prompt: %v", err)
		}
		before := h.cancelCount()
		if err := a.SessionCancel(context.Background(), acp.CancelNotification{SessionID: acp.SessionID(sess)}); err != nil {
			t.Fatalf("late cancel: %v", err)
		}
		time.Sleep(30 * time.Millisecond)
		if h.cancelCount() != before {
			t.Fatal("late cancel issued a run/cancel")
		}
		// The latch is still live: a prompt admitted within the TTL
		// consumes it per the ordered-cancel contract.
		go func() {
			if h.waitForSubs(2) {
				h.emitRunEvent(h.subID(1), h.runID(1), 1, "run.cancelled", `{"reason":"user_requested"}`)
			}
		}()
		resp2, err := a.Prompt(context.Background(), promptText(sess, "again"))
		if err != nil {
			t.Fatalf("second prompt: %v", err)
		}
		if resp2.StopReason != acp.StopReasonCancelled {
			t.Fatalf("fresh latch was not consumed: %q", resp2.StopReason)
		}
	})

	t.Run("stale latch after a completed prompt does not affect the next", func(t *testing.T) {
		h := newRunHost()
		a := newTestAgent(t, h, 1)
		sess := h.sessionID(0)
		go h.emitTerminal("", "", "run.completed", `{"summary":"ok"}`)
		if _, err := a.Prompt(context.Background(), promptText(sess, "hi")); err != nil {
			t.Fatalf("first prompt: %v", err)
		}
		old := cancelLatchTTL
		cancelLatchTTL = 50 * time.Millisecond
		defer func() { cancelLatchTTL = old }()
		if err := a.SessionCancel(context.Background(), acp.CancelNotification{SessionID: acp.SessionID(sess)}); err != nil {
			t.Fatalf("idle cancel: %v", err)
		}
		time.Sleep(150 * time.Millisecond)
		go func() {
			if h.waitForSubs(2) {
				h.emitRunEvent(h.subID(1), h.runID(1), 1, "run.completed", `{"summary":"ok"}`)
			}
		}()
		resp2, err := a.Prompt(context.Background(), promptText(sess, "again"))
		if err != nil || resp2.StopReason != acp.StopReasonEndTurn {
			t.Fatalf("second prompt: %v %q", err, resp2.StopReason)
		}
	})

	t.Run("interaction event cancels run instead of auto-approving", func(t *testing.T) {
		h := newRunHost()
		a := newTestAgent(t, h, 1)
		sess := h.sessionID(0)
		go func() {
			if !h.waitForSubs(1) {
				return
			}
			h.emitRunEvent(h.subID(0), h.runID(0), 1, "tool.approval_required", `{"tool_call_id":"t1","tool_name":"write","args":{}}`)
			h.emitRunEvent(h.subID(0), h.runID(0), 2, "run.cancelled", `{"reason":"user_requested"}`)
		}()
		_, err := a.Prompt(context.Background(), promptText(sess, "hi"))
		var re *acp.RPCError
		if !errors.As(err, &re) || re.Code != -32603 {
			t.Fatalf("error = %v", err)
		}
		if h.cancelCount() == 0 {
			t.Fatal("interaction did not cancel the run")
		}
	})
}

func TestCancelOnActivePromptDoesNotArmLatch(t *testing.T) {
	// A cancel accepted for an in-flight prompt applies to it alone; the
	// latch stays unarmed so the next prompt admitted within the TTL is
	// unaffected (G0 ruling: idle cancel latches, active cancel does not).
	h := newRunHost()
	a := newTestAgent(t, h, 1)
	sess := h.sessionID(0)

	done := make(chan struct{})
	var resp acp.PromptResponse
	var perr error
	go func() {
		defer close(done)
		resp, perr = a.Prompt(context.Background(), promptText(sess, "hi"))
	}()
	if !h.waitForSubs(1) {
		t.Fatal("no subscription")
	}
	sub, run := h.subID(0), h.runID(0)
	if err := a.SessionCancel(context.Background(), acp.CancelNotification{SessionID: acp.SessionID(sess)}); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	h.emitRunEvent(sub, run, 1, "run.cancelled", `{"reason":"user_requested"}`)
	<-done
	if perr != nil || resp.StopReason != acp.StopReasonCancelled {
		t.Fatalf("first prompt: %v %q", perr, resp.StopReason)
	}

	go func() {
		if h.waitForSubs(2) {
			h.emitRunEvent(h.subID(1), h.runID(1), 1, "run.completed", `{"summary":"ok"}`)
		}
	}()
	resp2, err := a.Prompt(context.Background(), promptText(sess, "again"))
	if err != nil {
		t.Fatalf("second prompt: %v", err)
	}
	if resp2.StopReason != acp.StopReasonEndTurn {
		t.Fatalf("active-prompt cancel leaked into the latch: %q", resp2.StopReason)
	}
}
