package headless

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"

	"agent-vivy/sdk/plugin"
)

// The organ is tested against a fake FaceEnv: the RPC vocabulary is the
// contract, so the test replays the exact control-plane shapes the
// kernel controlHandler produces (session/create → turn/start →
// run/subscribe → run/event notifications) without composing the engine.

type fakeEnv struct {
	mu      sync.Mutex
	calls   []string
	handler func(method string, params json.RawMessage)
}

func (e *fakeEnv) Call(ctx context.Context, method string, params any) (json.RawMessage, error) {
	e.mu.Lock()
	e.calls = append(e.calls, method)
	e.mu.Unlock()
	switch method {
	case "initialize":
		return json.RawMessage(`{"protocol_version":"vivy.rpc.v1"}`), nil
	case "session/create":
		return json.RawMessage(`{"id":"sess_new"}`), nil
	case "session/list":
		return json.RawMessage(`{"sessions":[{"id":"sess_old"}]}`), nil
	case "turn/start":
		return json.RawMessage(`{"run_id":"run_1"}`), nil
	case "run/subscribe":
		return json.RawMessage(`{"subscription_id":"sub_1","run_id":"run_1"}`), nil
	case "run/cancel":
		go e.deliver("run.cancelled", `{}`)
		return json.RawMessage(`{"cancelled":true}`), nil
	}
	return nil, nil
}

func (e *fakeEnv) OnEvent(handler func(method string, params json.RawMessage)) {
	e.mu.Lock()
	e.handler = handler
	e.mu.Unlock()
}

// deliver replays one server notification exactly as the kernel control
// plane would wrap it: run/cancel answered with a durable run.cancelled
// terminal, approvals re-raised as tool.approval_required, and so on.
func (e *fakeEnv) deliver(typ string, payload string) {
	e.mu.Lock()
	handler := e.handler
	e.mu.Unlock()
	if handler == nil {
		return
	}
	raw, err := json.Marshal(map[string]any{
		"subscription_id": "sub_1",
		"event": map[string]any{
			"run_id":  "run_1",
			"seq":     1,
			"type":    typ,
			"payload": json.RawMessage(payload),
		},
	})
	if err != nil {
		return
	}
	handler("run/event", raw)
}

func newFace(t *testing.T, prompt string, continueNewest bool) (plugin.Face, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	out := &bytes.Buffer{}
	errw := &bytes.Buffer{}
	return New(plugin.FaceOptions{Prompt: prompt, ContinueNewest: continueNewest, Out: out, Err: errw}), out, errw
}

func runFace(t *testing.T, f plugin.Face, env plugin.FaceEnv) plugin.FaceResult {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	result, err := f.Run(ctx, env)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	return result
}

func TestCompletedRunStreamsAndReturnsStatus(t *testing.T) {
	f, out, _ := newFace(t, "say hi", false)
	env := &fakeEnv{}

	done := make(chan plugin.FaceResult, 1)
	go func() {
		done <- runFace(t, f, env)
	}()

	// The events arrive after the organ subscribes; deliver them from the
	// test goroutine once the handler is registered.
	deadline := time.Now().Add(2 * time.Second)
	for {
		env.mu.Lock()
		registered := env.handler != nil
		env.mu.Unlock()
		if registered {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("organ never registered its event handler")
		}
		time.Sleep(10 * time.Millisecond)
	}
	env.deliver("model.delta", `{"delta":"hel"}`)
	env.deliver("model.delta", `{"delta":"lo"}`)
	env.deliver("model.completed", `{"content":"hello"}`)
	env.deliver("run.completed", `{}`)

	select {
	case result := <-done:
		if result.Status != "completed" {
			t.Fatalf("status = %q, want completed", result.Status)
		}
	case <-time.After(3 * time.Second):
		t.Fatalf("Run did not return after the terminal event")
	}
	if got := out.String(); got != "hello\n" {
		t.Fatalf("out = %q, want %q", got, "hello\n")
	}
	env.mu.Lock()
	calls := append([]string(nil), env.calls...)
	env.mu.Unlock()
	joined := strings.Join(calls, ",")
	for _, want := range []string{"initialize", "session/create", "turn/start", "run/subscribe"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("call chain missing %s: %s", want, joined)
		}
	}
}

func TestUnstreamedCompletedPrintsContent(t *testing.T) {
	f, out, _ := newFace(t, "say hi", false)
	env := &fakeEnv{}
	done := make(chan plugin.FaceResult, 1)
	go func() { done <- runFace(t, f, env) }()
	waitHandler(t, env)
	env.deliver("model.completed", `{"content":"plain"}`)
	env.deliver("run.completed", `{}`)
	<-done
	if got := out.String(); got != "plain\n" {
		t.Fatalf("out = %q, want %q", got, "plain\n")
	}
}

func TestApprovalBlockCancelsAndReturnsCancelled(t *testing.T) {
	f, _, errw := newFace(t, "needs approval", false)
	env := &fakeEnv{}
	done := make(chan plugin.FaceResult, 1)
	go func() { done <- runFace(t, f, env) }()
	waitHandler(t, env)

	env.deliver("tool.started", `{"tool_name":"write_note"}`)
	env.deliver("tool.approval_required", `{"tool_name":"write_note","action":"write","target":"note.md"}`)

	select {
	case result := <-done:
		if result.Status != "cancelled" {
			t.Fatalf("status = %q, want cancelled", result.Status)
		}
	case <-time.After(3 * time.Second):
		t.Fatalf("Run did not return after the approval block")
	}
	if !strings.Contains(errw.String(), "requires human approval") {
		t.Fatalf("err = %q, want the blocking notice", errw.String())
	}
	env.mu.Lock()
	calls := append([]string(nil), env.calls...)
	env.mu.Unlock()
	if !strings.Contains(strings.Join(calls, ","), "run/cancel") {
		t.Fatalf("organ did not cancel the blocked run: %v", calls)
	}
}

func TestQuestionBlockCancelsLoudly(t *testing.T) {
	f, _, errw := newFace(t, "ask me", false)
	env := &fakeEnv{}
	done := make(chan plugin.FaceResult, 1)
	go func() { done <- runFace(t, f, env) }()
	waitHandler(t, env)
	env.deliver("user.question_required", `{"prompt":"which flavor?"}`)
	result := <-done
	if result.Status != "cancelled" {
		t.Fatalf("status = %q, want cancelled", result.Status)
	}
	if !strings.Contains(errw.String(), "the model asked") {
		t.Fatalf("err = %q, want the question notice", errw.String())
	}
}

func TestContinueNewestPicksExistingSession(t *testing.T) {
	f, _, _ := newFace(t, "again", true)
	env := &fakeEnv{}
	done := make(chan plugin.FaceResult, 1)
	go func() { done <- runFace(t, f, env) }()
	waitHandler(t, env)
	env.deliver("run.completed", `{}`)
	<-done
	env.mu.Lock()
	calls := append([]string(nil), env.calls...)
	env.mu.Unlock()
	joined := strings.Join(calls, ",")
	if strings.Contains(joined, "session/create") {
		t.Fatalf("--continue created a new session: %s", joined)
	}
	if !strings.Contains(joined, "session/list") {
		t.Fatalf("--continue never listed sessions: %s", joined)
	}
}

func TestFailedRunReportsFailed(t *testing.T) {
	f, _, errw := newFace(t, "break", false)
	env := &fakeEnv{}
	done := make(chan plugin.FaceResult, 1)
	go func() { done <- runFace(t, f, env) }()
	waitHandler(t, env)
	env.deliver("run.failed", `{"cause_category":"provider","message":"boom"}`)
	result := <-done
	if result.Status != "failed" {
		t.Fatalf("status = %q, want failed", result.Status)
	}
	if !strings.Contains(errw.String(), "vivy: run failed (provider): boom") {
		t.Fatalf("err = %q, want the failure line", errw.String())
	}
}

func TestEmptyPromptFailsBeforeDialing(t *testing.T) {
	f, _, _ := newFace(t, "   ", false)
	env := &fakeEnv{}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if _, err := f.Run(ctx, env); err == nil {
		t.Fatalf("empty prompt accepted")
	}
	env.mu.Lock()
	calls := append([]string(nil), env.calls...)
	env.mu.Unlock()
	if len(calls) != 0 {
		t.Fatalf("empty prompt produced calls: %v", calls)
	}
}

func waitHandler(t *testing.T, env *fakeEnv) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for {
		env.mu.Lock()
		registered := env.handler != nil
		env.mu.Unlock()
		if registered {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("organ never registered its event handler")
		}
		time.Sleep(10 * time.Millisecond)
	}
}
