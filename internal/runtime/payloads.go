package runtime

// Event payload structs. Field names and shapes mirror
// schemas/events/payloads/*.json (A3) field for field; PayloadVersion is
// always 1 in V0.

type payloadRunStarted struct {
	Provider string `json:"provider"`
	Model    string `json:"model"`
	Mode     string `json:"mode"`
}

type payloadModelDelta struct {
	Delta string `json:"delta"`
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
}

type payloadUserQuestionRequired struct {
	QuestionID    string   `json:"question_id"`
	ToolCallID    string   `json:"tool_call_id"`
	Prompt        string   `json:"prompt"`
	ExpiresAt     int64    `json:"expires_at"`
	ResumeTarget  string   `json:"resume_target"`
	SelectedTools []string `json:"selected_tools,omitempty"`
	Mode          string   `json:"mode,omitempty"`
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
