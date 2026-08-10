package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"agent-vivy/internal/domain"
)

type testToolHook struct {
	name       string
	pre        PreToolUseResult
	preErr     error
	postErr    error
	postCalled bool
}

func (h *testToolHook) Name() string { return h.name }

func (h *testToolHook) PreToolUse(context.Context, ToolHookCall) (PreToolUseResult, error) {
	return h.pre, h.preErr
}

func (h *testToolHook) PostToolUse(context.Context, ToolHookCall, string, error) error {
	h.postCalled = true
	return h.postErr
}

type timeoutToolHook struct{}

func (timeoutToolHook) Name() string { return "timeout" }

func (timeoutToolHook) PreToolUse(ctx context.Context, _ ToolHookCall) (PreToolUseResult, error) {
	<-ctx.Done()
	return PreToolUseResult{}, ctx.Err()
}

func (timeoutToolHook) PostToolUse(context.Context, ToolHookCall, string, error) error { return nil }

func TestToolHookChainRewriteAndPost(t *testing.T) {
	hook := &testToolHook{name: "rewrite", pre: PreToolUseResult{
		Decision: hookRewrite, UpdatedArgs: json.RawMessage(`{"path":"safe"}`), Reason: "normalized",
	}}
	chain := NewToolHookChain(time.Second, hook)
	args, err := chain.PreToolUse(context.Background(), ToolHookCall{
		ToolName: "write_note", Arguments: json.RawMessage(`{"path":"unsafe"}`), Profile: domain.PolicyProfileDefault,
	})
	if err != nil {
		t.Fatal(err)
	}
	if string(args) != `{"path":"safe"}` {
		t.Fatalf("rewritten args = %s", args)
	}
	chain.PostToolUse(context.Background(), ToolHookCall{ToolName: "write_note"}, "ok", nil)
	if !hook.postCalled {
		t.Fatal("post hook was not called")
	}
}

func TestToolHookChainDenyFailsClosed(t *testing.T) {
	chain := NewToolHookChain(time.Second, &testToolHook{
		name: "deny", pre: PreToolUseResult{Decision: hookDeny, Reason: "blocked by test"},
	})
	_, err := chain.PreToolUse(context.Background(), ToolHookCall{ToolName: "write_note"})
	if !errors.Is(err, ErrHookBlocked) {
		t.Fatalf("error = %v, want ErrHookBlocked", err)
	}
}

func TestToolHookChainTimeoutFailsClosed(t *testing.T) {
	chain := NewToolHookChain(5*time.Millisecond, timeoutToolHook{})
	_, err := chain.PreToolUse(context.Background(), ToolHookCall{ToolName: "write_note"})
	if !errors.Is(err, ErrHookBlocked) {
		t.Fatalf("error = %v, want ErrHookBlocked", err)
	}
}

func TestToolHookChainPostFailureIsFailOpen(t *testing.T) {
	hook := &testToolHook{name: "post-failure", postErr: errors.New("audit unavailable")}
	chain := NewToolHookChain(time.Second, hook)
	chain.PostToolUse(context.Background(), ToolHookCall{ToolName: "write_note"}, "result", nil)
	if !hook.postCalled {
		t.Fatal("post hook was not called")
	}
}
