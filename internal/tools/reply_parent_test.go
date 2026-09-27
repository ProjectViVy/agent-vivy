package tools

import (
	"context"
	"encoding/json"
	"testing"

	"agent-vivy/internal/domain"
)

type fakeParentMessageOperations struct {
	runID     domain.RunID
	sessionID domain.SessionID
	operation string
	text      string
	result    ParentMessageResult
}

func (f *fakeParentMessageOperations) SendParentMessage(_ context.Context, runID domain.RunID, sessionID domain.SessionID, operation, text string) (ParentMessageResult, error) {
	f.runID, f.sessionID, f.operation, f.text = runID, sessionID, operation, text
	return f.result, nil
}

func TestReplyParentUsesRunSessionAndStableModelCallIdentity(t *testing.T) {
	operations := &fakeParentMessageOperations{result: ParentMessageResult{MessageID: "mail-1", Sequence: 3, Status: "pending"}}
	tool := NewReplyParent(operations)
	args := json.RawMessage(`{"text":"the result is ready"}`)
	ctx := WithToolCallID(WithSessionID(WithRunID(context.Background(), "child-run-1"), "child-session-1"), "call-reply-1")
	result, err := tool.InvokableRun(ctx, args)
	if err != nil {
		t.Fatalf("invoke reply_parent: %v", err)
	}
	if operations.runID != "child-run-1" || operations.sessionID != "child-session-1" || operations.operation == "" || operations.text != "the result is ready" {
		t.Fatalf("reply operation = %+v", operations)
	}
	var returned ParentMessageResult
	if err := json.Unmarshal([]byte(result), &returned); err != nil || returned.MessageID != "mail-1" || returned.Sequence != 3 {
		t.Fatalf("reply result = %q err=%v", result, err)
	}
	if !tool.Spec().Readonly {
		t.Fatal("direct bounded child reply should not open an effectful-tool approval")
	}
	if err := ValidateArgs(tool.Spec(), args); err != nil {
		t.Fatalf("reply schema: %v", err)
	}
}

func TestReplyParentOperationKeyIsScopedToRunAndStableWithinRun(t *testing.T) {
	operations := &fakeParentMessageOperations{}
	tool := NewReplyParent(operations)
	args := json.RawMessage(`{"text":"same"}`)
	invoke := func(runID domain.RunID) string {
		ctx := WithToolCallID(WithSessionID(WithRunID(context.Background(), runID), "child-session-1"), "provider-call-1")
		if _, err := tool.InvokableRun(ctx, args); err != nil {
			t.Fatalf("invoke %s: %v", runID, err)
		}
		return operations.operation
	}
	first := invoke("child-run-a")
	if retry := invoke("child-run-a"); retry != first {
		t.Fatalf("same-run retry key = %q, want %q", retry, first)
	}
	if otherRun := invoke("child-run-b"); otherRun == first {
		t.Fatalf("cross-run operation key collided: %q", first)
	}
}

func TestReplyParentRequiresScopedChildIdentity(t *testing.T) {
	tool := NewReplyParent(&fakeParentMessageOperations{})
	if _, err := tool.InvokableRun(context.Background(), json.RawMessage(`{"text":"hello"}`)); err == nil {
		t.Fatal("reply_parent accepted an unscoped invocation")
	}
}
