package tools

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"agent-vivy/internal/domain"
)

const (
	ReplyParentName   = "reply_parent"
	maxParentReplyLen = 32 << 10
)

type ParentMessageResult struct {
	MessageID string `json:"message_id"`
	Sequence  int64  `json:"sequence"`
	Status    string `json:"status"`
}

type ParentMessageOperations interface {
	SendParentMessage(context.Context, domain.RunID, domain.SessionID, string, string) (ParentMessageResult, error)
}

type replyParentTool struct{ ops ParentMessageOperations }

func NewReplyParent(ops ParentMessageOperations) Tool { return &replyParentTool{ops: ops} }

func (t *replyParentTool) Spec() domain.ToolSpec {
	return domain.ToolSpec{
		Name: ReplyParentName,
		Description: "Send one bounded direct message from this continuable child task to its parent. " +
			"The host derives and verifies the child identity; this does not contact peers or siblings. " +
			"Use it only when the parent needs a result or clarification. Acknowledgement means the message was admitted, not read.",
		Readonly: true,
		Keywords: []string{"reply", "parent", "child", "message"},
		Schema:   json.RawMessage(`{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object","additionalProperties":false,"required":["text"],"properties":{"text":{"type":"string","minLength":1,"maxLength":32768}}}`),
	}
}

func (t *replyParentTool) InvokableRun(ctx context.Context, args json.RawMessage) (string, error) {
	if t == nil || t.ops == nil {
		return "", errors.New("child messaging is not wired")
	}
	runID := RunIDFromContext(ctx)
	sessionID := SessionIDFromContext(ctx)
	callID := ToolCallIDFromContext(ctx)
	if runID == "" || sessionID == "" || callID == "" {
		return "", errors.New("reply_parent requires a run-scoped child invocation")
	}
	if err := ValidateArgs(t.Spec(), args); err != nil {
		return "", err
	}
	var params struct {
		Text string `json:"text"`
	}
	decoder := json.NewDecoder(bytes.NewReader(args))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&params); err != nil {
		return "", fmt.Errorf("decode parent reply: %w", err)
	}
	text := strings.TrimSpace(params.Text)
	if text == "" || len([]byte(text)) > maxParentReplyLen {
		return "", fmt.Errorf("parent reply must contain 1 to %d bytes", maxParentReplyLen)
	}
	operationKey := "child-reply-" + base64.RawURLEncoding.EncodeToString([]byte(runID)) + "." + base64.RawURLEncoding.EncodeToString([]byte(callID))
	if len(operationKey) > 256 {
		return "", errors.New("reply_parent model call identity exceeds the operation key byte bound")
	}
	result, err := t.ops.SendParentMessage(ctx, runID, sessionID, operationKey, text)
	if err != nil {
		return "", err
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("encode parent reply result: %w", err)
	}
	return string(encoded), nil
}
