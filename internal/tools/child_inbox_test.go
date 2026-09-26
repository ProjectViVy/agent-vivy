package tools

import (
	"context"
	"encoding/json"
	"testing"

	"agent-vivy/internal/domain"
)

type fakeChildInboxOperations struct {
	runID     domain.RunID
	sessionID domain.SessionID
	result    []ChildInboxMessage
}

func (f *fakeChildInboxOperations) ReadParentInbox(_ context.Context, runID domain.RunID, sessionID domain.SessionID) ([]ChildInboxMessage, error) {
	f.runID, f.sessionID = runID, sessionID
	return append([]ChildInboxMessage(nil), f.result...), nil
}

func TestChildInboxUsesParentRunScopeAndReturnsBoundedMessages(t *testing.T) {
	ops := &fakeChildInboxOperations{result: []ChildInboxMessage{{
		ChildSessionID: "child-session-1", MessageID: "mail-1", Sequence: 3, Text: "reply ready",
	}}}
	tool := NewChildInbox(ops)
	ctx := WithSessionID(WithRunID(context.Background(), "parent-run-1"), "parent-session-1")
	result, err := tool.InvokableRun(ctx, json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("invoke child_inbox: %v", err)
	}
	if ops.runID != "parent-run-1" || ops.sessionID != "parent-session-1" {
		t.Fatalf("inbox scope = run %q session %q", ops.runID, ops.sessionID)
	}
	var decoded ChildInboxResult
	if err := json.Unmarshal([]byte(result), &decoded); err != nil || len(decoded.Messages) != 1 || decoded.Messages[0].MessageID != "mail-1" {
		t.Fatalf("inbox result = %q err=%v", result, err)
	}
	if !tool.Spec().Readonly {
		t.Fatal("child inbox should be a read-only tool")
	}
}

func TestChildInboxRequiresRunAndSessionScope(t *testing.T) {
	tool := NewChildInbox(&fakeChildInboxOperations{})
	if _, err := tool.InvokableRun(context.Background(), json.RawMessage(`{}`)); err == nil {
		t.Fatal("child_inbox accepted an unscoped invocation")
	}
}
