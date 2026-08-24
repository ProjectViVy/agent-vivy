package worker

import "context"

// ChatMessage is the wire-safe, bounded message shape shared by a worker and
// its parent model broker. It intentionally contains only the current child
// task context; it is not a persisted conversation or memory record.
type ChatMessage struct {
	Role       string          `json:"role"`
	Content    string          `json:"content,omitempty"`
	ToolCallID string          `json:"tool_call_id,omitempty"`
	ToolCalls  []ModelToolCall `json:"tool_calls,omitempty"`
}

type ModelToolCall struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Arguments any    `json:"arguments,omitempty"`
}

type ModelParam struct {
	Description string `json:"description,omitempty"`
	Required    bool   `json:"required,omitempty"`
}

type ModelTool struct {
	Name        string                `json:"name"`
	Description string                `json:"description,omitempty"`
	Readonly    bool                  `json:"readonly"`
	Params      map[string]ModelParam `json:"params,omitempty"`
}

type ModelRequest struct {
	RequestID     string        `json:"request_id"`
	RunID         string        `json:"run_id"`
	ParentRunID   string        `json:"parent_run_id"`
	PolicyProfile string        `json:"policy_profile"`
	PolicyHash    string        `json:"policy_hash"`
	WorkspaceID   string        `json:"workspace_id"`
	Messages      []ChatMessage `json:"messages"`
	Tools         []ModelTool   `json:"tools,omitempty"`
}

type ModelResponse struct {
	Status     string      `json:"status"`
	Message    ChatMessage `json:"message"`
	StopReason string      `json:"stop_reason,omitempty"`
	Usage      *ModelUsage `json:"usage,omitempty"`
}

type ModelUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
	ReasoningTokens  int `json:"reasoning_tokens,omitempty"`
}

type ModelBroker interface {
	Complete(context.Context, ModelRequest) (ModelResponse, error)
}

type ApprovalWaitRequest struct {
	ApprovalID string `json:"approval_id"`
	RunID      string `json:"run_id"`
	ToolCallID string `json:"tool_call_id"`
}

type ApprovalWaitResult struct {
	Decision string `json:"decision"`
}

type ApprovalWaiter interface {
	Wait(context.Context, ApprovalWaitRequest) (ApprovalWaitResult, error)
}
