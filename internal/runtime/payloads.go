package runtime

// Event payload structs. Field names and shapes mirror
// schemas/events/payloads/*.json (A3) field for field; PayloadVersion is
// always 1 in V0.

type payloadRunStarted struct {
	Provider      string `json:"provider"`
	Model         string `json:"model"`
	Mode          string `json:"mode"`
	PolicyProfile string `json:"policy_profile,omitempty"`
	PolicyHash    string `json:"policy_hash,omitempty"`
}

type payloadModelDelta struct {
	Delta string `json:"delta"`
}

type payloadModelReasoningDelta struct {
	Delta string `json:"delta"`
}

type payloadModelUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
	ReasoningTokens  int `json:"reasoning_tokens,omitempty"`
}

type payloadProviderRetry struct {
	Attempt int `json:"attempt"`
}

type payloadProviderStall struct {
	ElapsedMs int64 `json:"elapsed_ms"`
}

type payloadModelCompleted struct {
	Content string `json:"content"`
}

type payloadToolRequested struct {
	ToolCallID string         `json:"tool_call_id"`
	ToolName   string         `json:"tool_name"`
	Args       map[string]any `json:"args"`
}

type payloadToolStarted struct {
	ToolCallID string `json:"tool_call_id"`
	ToolName   string `json:"tool_name"`
}

// payloadToolFinished carries a non-empty Error only when the call failed.
type payloadToolFinished struct {
	ToolCallID string `json:"tool_call_id"`
	ToolName   string `json:"tool_name"`
	Result     string `json:"result"`
	Error      string `json:"error,omitempty"`
}

// payloadToolApprovalRequired is committed in the single journal commit
// that follows a durable checkpoint (D-029 write order).
type payloadToolApprovalRequired struct {
	ApprovalID    string         `json:"approval_id"`
	ToolCallID    string         `json:"tool_call_id"`
	ToolName      string         `json:"tool_name"`
	Args          map[string]any `json:"args"`
	ExpiresAt     int64          `json:"expires_at"`
	SelectedTools []string       `json:"selected_tools,omitempty"`
	Mode          string         `json:"mode,omitempty"`
	PolicyProfile string         `json:"policy_profile,omitempty"`
	PolicyHash    string         `json:"policy_hash,omitempty"`
}

type payloadUserQuestionRequired struct {
	QuestionID    string   `json:"question_id"`
	ToolCallID    string   `json:"tool_call_id"`
	Prompt        string   `json:"prompt"`
	ExpiresAt     int64    `json:"expires_at"`
	ResumeTarget  string   `json:"resume_target"`
	SelectedTools []string `json:"selected_tools,omitempty"`
	Mode          string   `json:"mode,omitempty"`
	PolicyProfile string   `json:"policy_profile,omitempty"`
	PolicyHash    string   `json:"policy_hash,omitempty"`
}

type payloadPolicyEvaluated struct {
	ToolName string `json:"tool_name"`
	Decision string `json:"decision"`
	Profile  string `json:"profile"`
	Hash     string `json:"policy_hash,omitempty"`
	Reason   string `json:"reason,omitempty"`
}

type payloadHookLifecycle struct {
	ToolName   string `json:"tool_name"`
	HookName   string `json:"hook_name"`
	Phase      string `json:"phase"`
	Decision   string `json:"decision,omitempty"`
	Profile    string `json:"profile,omitempty"`
	Reason     string `json:"reason,omitempty"`
	DurationMs int64  `json:"duration_ms,omitempty"`
}

type payloadUserQuestionAnswered struct {
	QuestionID string `json:"question_id"`
	Answer     string `json:"answer"`
}

type payloadRunCompleted struct {
	Summary string `json:"summary,omitempty"`
}

// Cause categories for run.failed (FR-11; structured, never leaky).
const (
	causeProviderError = "provider_error"
	causeToolError     = "tool_error"
	causeInternalError = "internal_error"
	causeCancelled     = "cancelled"
)

type payloadRunFailed struct {
	CauseCategory string `json:"cause_category"`
	Message       string `json:"message"`
}

// Cancel reasons for run.cancelled.
const (
	reasonUserRequested = "user_requested"
	reasonRecovery      = "recovery"
)

type payloadRunCancelled struct {
	Reason string `json:"reason"`
}
