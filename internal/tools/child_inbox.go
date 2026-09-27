package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"agent-vivy/internal/domain"
)

const (
	ChildInboxName           = "child_inbox"
	MaxChildInboxResultBytes = 20 << 10
)

type ChildInboxMessage struct {
	ChildSessionID string `json:"child_session_id"`
	MessageID      string `json:"message_id"`
	Sequence       int64  `json:"sequence"`
	Text           string `json:"text"`
}

type ChildInboxResult struct {
	Messages []ChildInboxMessage `json:"messages"`
}

type ChildInboxOperations interface {
	ReadParentInbox(context.Context, domain.RunID, domain.SessionID) ([]ChildInboxMessage, error)
}

type childInboxTool struct{ ops ChildInboxOperations }

func NewChildInbox(ops ChildInboxOperations) Tool { return &childInboxTool{ops: ops} }

func (t *childInboxTool) Spec() domain.ToolSpec {
	return domain.ToolSpec{
		Name: ChildInboxName,
		Description: "Read bounded direct replies from this run's continuable child tasks. " +
			"Returned messages are acknowledged as consumed; use this after delegating or when a child reply is expected. " +
			"Only direct children of the current Session are included.",
		Readonly: true,
		Keywords: []string{"child", "inbox", "reply", "message"},
		Schema:   json.RawMessage(`{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object","additionalProperties":false}`),
	}
}

func (t *childInboxTool) InvokableRun(ctx context.Context, args json.RawMessage) (string, error) {
	if t == nil || t.ops == nil {
		return "", errors.New("child inbox is not wired")
	}
	runID := RunIDFromContext(ctx)
	sessionID := SessionIDFromContext(ctx)
	if runID == "" || sessionID == "" {
		return "", errors.New("child_inbox requires a parent run-scoped invocation")
	}
	if err := ValidateArgs(t.Spec(), args); err != nil {
		return "", err
	}
	decoder := json.NewDecoder(bytes.NewReader(args))
	decoder.DisallowUnknownFields()
	var params struct{}
	if err := decoder.Decode(&params); err != nil {
		return "", fmt.Errorf("decode child inbox request: %w", err)
	}
	messages, err := t.ops.ReadParentInbox(ctx, runID, sessionID)
	if err != nil {
		return "", err
	}
	encoded, err := json.Marshal(ChildInboxResult{Messages: messages})
	if err != nil {
		return "", fmt.Errorf("encode child inbox: %w", err)
	}
	if len(encoded) > MaxChildInboxResultBytes {
		return "", fmt.Errorf("child inbox result exceeds %d bytes", MaxChildInboxResultBytes)
	}
	return string(encoded), nil
}
