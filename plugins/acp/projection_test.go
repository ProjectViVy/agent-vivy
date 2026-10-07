package acp

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	acp "github.com/eino-contrib/acp"
)

// updatesOf returns the recorded session updates for a finished prompt.
func promptUpdates(p *promptState) []acp.SessionUpdate {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]acp.SessionUpdate(nil), p.updates...)
}

func activePromptOf(t *testing.T, a *agent, sess string) *promptState {
	t.Helper()
	a.mu.Lock()
	s := a.sessions[sess]
	a.mu.Unlock()
	if s == nil {
		t.Fatal("session missing")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.active == nil {
		t.Fatal("no active prompt")
	}
	return s.active
}

func TestOrderedCommittedEvents(t *testing.T) {
	t.Run("out-of-order events release in sequence", func(t *testing.T) {
		h := newRunHost()
		a := newTestAgent(t, h, 1)
		sess := h.sessionID(0)

		done := make(chan struct{})
		var resp acp.PromptResponse
		go func() {
			defer close(done)
			resp, _ = a.Prompt(context.Background(), promptText(sess, "hi"))
		}()
		if !h.waitForSubs(1) {
			t.Fatal("no subscription")
		}
		sub, run := h.subID(0), h.runID(0)

		// Emit 3,1,2 — tool events at 1,2 project; terminal at 3.
		h.emitRunEvent(sub, run, 3, "run.completed", `{"summary":"ok"}`)
		h.emitRunEvent(sub, run, 1, "tool.requested", `{"tool_call_id":"t1","tool_name":"read_file","args":{"p":"x"}}`)
		h.emitRunEvent(sub, run, 2, "tool.started", `{"tool_call_id":"t1","tool_name":"read_file"}`)

		select {
		case <-done:
		case <-time.After(10 * time.Second):
			t.Fatal("prompt did not finish")
		}
		if resp.StopReason != acp.StopReasonEndTurn {
			t.Fatalf("stopReason = %q", resp.StopReason)
		}
	})

	t.Run("duplicate sequence numbers are dropped", func(t *testing.T) {
		h := newRunHost()
		a := newTestAgent(t, h, 1)
		sess := h.sessionID(0)

		done := make(chan struct{})
		var resp acp.PromptResponse
		go func() {
			defer close(done)
			resp, _ = a.Prompt(context.Background(), promptText(sess, "hi"))
		}()
		if !h.waitForSubs(1) {
			t.Fatal("no subscription")
		}
		sub, run := h.subID(0), h.runID(0)
		h.emitRunEvent(sub, run, 1, "tool.requested", `{"tool_call_id":"t1","tool_name":"read"}`)
		h.emitRunEvent(sub, run, 1, "tool.requested", `{"tool_call_id":"t1","tool_name":"read"}`)
		h.emitRunEvent(sub, run, 2, "run.completed", `{"summary":"ok"}`)
		select {
		case <-done:
		case <-time.After(10 * time.Second):
			t.Fatal("hang")
		}
		if resp.StopReason != acp.StopReasonEndTurn {
			t.Fatalf("stopReason = %q", resp.StopReason)
		}
	})

	t.Run("persistent sequence gap fails the prompt", func(t *testing.T) {
		old := missingSeqTimeout
		missingSeqTimeout = 100 * time.Millisecond
		defer func() { missingSeqTimeout = old }()

		h := newRunHost()
		a := newTestAgent(t, h, 1)
		sess := h.sessionID(0)

		done := make(chan struct{})
		var perr error
		go func() {
			defer close(done)
			_, perr = a.Prompt(context.Background(), promptText(sess, "hi"))
		}()
		if !h.waitForSubs(1) {
			t.Fatal("no subscription")
		}
		sub, run := h.subID(0), h.runID(0)
		// seq 2 arrives without seq 1: gap persists until expiry.
		h.emitRunEvent(sub, run, 2, "run.completed", `{"summary":"ok"}`)
		select {
		case <-done:
		case <-time.After(10 * time.Second):
			t.Fatal("gap did not fail")
		}
		var re *acp.RPCError
		if !errors.As(perr, &re) || re.Code != -32603 {
			t.Fatalf("error = %v", perr)
		}
		var data map[string]string
		_ = json.Unmarshal(re.Data, &data)
		if data["reason"] != "STREAM_INTEGRITY_FAILED" {
			t.Fatalf("reason = %q", data["reason"])
		}
	})

	t.Run("ingress overflow fails with integrity error and cancels run", func(t *testing.T) {
		h := newRunHost()
		a := newTestAgent(t, h, 1)
		sess := h.sessionID(0)

		done := make(chan struct{})
		var perr error
		go func() {
			defer close(done)
			_, perr = a.Prompt(context.Background(), promptText(sess, "hi"))
		}()
		if !h.waitForSubs(1) {
			t.Fatal("no subscription")
		}
		sub, run := h.subID(0), h.runID(0)
		// Flood beyond the 256-event ingress bound with a persistent
		// gap (seq starts at 2) so events accumulate in reorder storage.
		for i := int64(2); i <= 400; i++ {
			h.emitRunEvent(sub, run, i, "model.usage", `{"input_tokens":1,"output_tokens":1,"total_tokens":2}`)
		}
		select {
		case <-done:
		case <-time.After(10 * time.Second):
			t.Fatal("overflow did not fail")
		}
		var re *acp.RPCError
		if !errors.As(perr, &re) || re.Code != -32603 {
			t.Fatalf("error = %v", perr)
		}
		if h.cancelCount() == 0 {
			t.Fatal("overflowed run was not cancelled")
		}
	})

	t.Run("event for a foreign run id is dropped", func(t *testing.T) {
		h := newRunHost()
		a := newTestAgent(t, h, 1)
		sess := h.sessionID(0)
		done := make(chan struct{})
		var resp acp.PromptResponse
		go func() {
			defer close(done)
			resp, _ = a.Prompt(context.Background(), promptText(sess, "hi"))
		}()
		if !h.waitForSubs(1) {
			t.Fatal("no subscription")
		}
		sub, run := h.subID(0), h.runID(0)
		h.emitRunEvent(sub, "run_other", 1, "run.cancelled", `{"reason":"user_requested"}`)
		h.emitRunEvent(sub, run, 1, "run.completed", `{"summary":"ok"}`)
		h.emitRunEvent(sub, run, 2, "run.completed", `{"summary":"dup"}`)
		select {
		case <-done:
		case <-time.After(10 * time.Second):
			t.Fatal("hang")
		}
		if resp.StopReason != acp.StopReasonEndTurn {
			t.Fatalf("foreign run event affected the prompt: %q", resp.StopReason)
		}
	})
}

func TestOpaqueToolID(t *testing.T) {
	scopeA := promptScope{SessionID: "s1", RunID: "r1", Generation: 1}
	scopeB := promptScope{SessionID: "s1", RunID: "r2", Generation: 2}

	id1 := opaqueToolID("nonce-x", scopeA, "t1")
	if id1 != opaqueToolID("nonce-x", scopeA, "t1") {
		t.Fatal("not deterministic")
	}
	if id1 == opaqueToolID("nonce-x", scopeB, "t1") {
		t.Fatal("same tool id across runs")
	}
	if id1 == opaqueToolID("nonce-y", scopeA, "t1") {
		t.Fatal("same tool id across connections")
	}
	if id1 == opaqueToolID("nonce-x", scopeA, "t2") {
		t.Fatal("same id for different tool calls")
	}
	// Length-prefix safety: ("ab","c") must differ from ("a","bc").
	s1 := promptScope{SessionID: "ab", RunID: "r1"}
	s2 := promptScope{SessionID: "a", RunID: "br1"}
	if opaqueToolID("n", s1, "tc") == opaqueToolID("n", s2, "tc") {
		t.Fatal("length-prefix collision")
	}
	if !strings.Contains(id1, "t1") && len(id1) != 64 {
		t.Fatalf("unexpected id shape: %q", id1)
	}
}

func TestToolCallOrderingAndSafety(t *testing.T) {
	t.Run("tool_call precedes its updates", func(t *testing.T) {
		h := newRunHost()
		a := newTestAgent(t, h, 1)
		sess := h.sessionID(0)
		pCh := make(chan *promptState, 1)

		done := make(chan struct{})
		go func() {
			defer close(done)
			resp, err := a.Prompt(context.Background(), promptText(sess, "hi"))
			_ = resp
			_ = err
			pCh <- nil
		}()
		if !h.waitForSubs(1) {
			t.Fatal("no subscription")
		}
		sub, run := h.subID(0), h.runID(0)
		p := activePromptOf(t, a, sess)

		h.emitRunEvent(sub, run, 1, "tool.requested", `{"tool_call_id":"t1","tool_name":"exec","args":{"cmd":"rm -rf /"}}`)
		h.emitRunEvent(sub, run, 2, "tool.started", `{"tool_call_id":"t1","tool_name":"exec"}`)
		h.emitRunEvent(sub, run, 3, "tool.finished", `{"tool_call_id":"t1","tool_name":"exec","result":"stdout leaked"}`)
		h.emitRunEvent(sub, run, 4, "run.completed", `{"summary":"ok"}`)
		<-done
		<-pCh

		ups := promptUpdates(p)
		if len(ups) != 3 {
			t.Fatalf("updates = %d, want 3", len(ups))
		}
		if ups[0].ToolCall == nil || ups[0].ToolCall.Title != "exec" {
			t.Fatalf("first update = %+v", ups[0])
		}
		if *ups[0].ToolCall.Status != acp.ToolCallStatusPending {
			t.Fatalf("status = %v", *ups[0].ToolCall.Status)
		}
		if ups[1].ToolCallUpdate == nil || *ups[1].ToolCallUpdate.Status != acp.ToolCallStatusInProgress {
			t.Fatalf("second update = %+v", ups[1])
		}
		if ups[2].ToolCallUpdate == nil || *ups[2].ToolCallUpdate.Status != acp.ToolCallStatusCompleted {
			t.Fatalf("third update = %+v", ups[2])
		}
		// Safety: raw args and raw result text never reach the wire.
		raw, _ := json.Marshal(ups)
		if strings.Contains(string(raw), "rm -rf") || strings.Contains(string(raw), "stdout leaked") {
			t.Fatalf("unsafe content in updates: %s", raw)
		}
		if string(ups[0].ToolCall.ToolCallID) == "t1" {
			t.Fatal("raw tool_call_id leaked to the wire")
		}
	})

	t.Run("finished without requested synthesizes the tool_call first", func(t *testing.T) {
		h := newRunHost()
		a := newTestAgent(t, h, 1)
		sess := h.sessionID(0)
		done := make(chan struct{})
		go func() {
			defer close(done)
			_, _ = a.Prompt(context.Background(), promptText(sess, "hi"))
		}()
		if !h.waitForSubs(1) {
			t.Fatal("no subscription")
		}
		p := activePromptOf(t, a, sess)
		sub, run := h.subID(0), h.runID(0)
		h.emitRunEvent(sub, run, 1, "tool.finished", `{"tool_call_id":"t9","tool_name":"read","error":"denied"}`)
		h.emitRunEvent(sub, run, 2, "run.completed", `{"summary":"ok"}`)
		<-done
		ups := promptUpdates(p)
		if len(ups) != 2 || ups[0].ToolCall == nil || ups[1].ToolCallUpdate == nil {
			t.Fatalf("updates = %+v", ups)
		}
		if *ups[1].ToolCallUpdate.Status != acp.ToolCallStatusFailed {
			t.Fatalf("status = %v", *ups[1].ToolCallUpdate.Status)
		}
		raw, _ := json.Marshal(ups)
		if strings.Contains(string(raw), "denied") {
			t.Fatal("raw error text leaked")
		}
	})

	t.Run("malformed tool event fails with integrity error", func(t *testing.T) {
		h := newRunHost()
		a := newTestAgent(t, h, 1)
		sess := h.sessionID(0)
		done := make(chan struct{})
		var perr error
		go func() {
			defer close(done)
			_, perr = a.Prompt(context.Background(), promptText(sess, "hi"))
		}()
		if !h.waitForSubs(1) {
			t.Fatal("no subscription")
		}
		sub, run := h.subID(0), h.runID(0)
		h.emitRunEvent(sub, run, 1, "tool.finished", `{"tool_name":"x"}`) // missing tool_call_id
		select {
		case <-done:
		case <-time.After(10 * time.Second):
			t.Fatal("hang")
		}
		var re *acp.RPCError
		if !errors.As(perr, &re) || re.Code != -32603 {
			t.Fatalf("error = %v", perr)
		}
	})
}
