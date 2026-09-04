package runtime

import "encoding/json"

// Event payload structs. Field names and shapes mirror
// schemas/events/payloads/*.json (A3) field for field. Most payloads remain
// v1; model.completed v2 commits its preceding bounded delta sequence.

type payloadRunStarted struct {
	Provider       string `json:"provider"`
	Model          string `json:"model"`
	Mode           string `json:"mode"`
	Face           string `json:"face"`
	PolicyProfile  string `json:"policy_profile,omitempty"`
	PolicyHash     string `json:"policy_hash,omitempty"`
	SandboxMode    string `json:"sandbox_mode,omitempty"`
	ApprovalPolicy string `json:"approval_policy,omitempty"`
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
	// CachedTokens is the prompt-token prefix served from the provider's
	// cache (OpenAI prompt_tokens_details.cached_tokens); omitted when the
	// provider does not report it.
	CachedTokens int `json:"cached_tokens,omitempty"`
}

// payloadContextCompacted records one context compression event. It carries
// numbers only — never transcript or summary content (D-010). Mode is
// "reduction", "summarization", or "session" (manual durable compaction).
type payloadContextCompacted struct {
	Mode            string `json:"mode"`
	BeforeTokens    int    `json:"before_tokens"`
	AfterTokens     int    `json:"after_tokens"`
	DroppedMessages int    `json:"dropped_messages,omitempty"`
	RetentionSuffix int    `json:"retention_suffix,omitempty"`
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

// payloadModelCompletedV2 commits the preceding bounded model.delta sequence.
// Keeping it distinct prevents legacy v1 encoders from leaking v2 fields.
type payloadModelCompletedV2 struct {
	ContentSHA256 string `json:"content_sha256"`
	ByteLen       int    `json:"byte_len"`
}

type payloadModelRequest struct {
	SelectedTools  []string                     `json:"selected_tools"`
	PreambleSHA256 string                       `json:"preamble_sha256"`
	PreambleBytes  int                          `json:"preamble_bytes"`
	Messages       []payloadModelRequestMessage `json:"messages"`
}

type payloadModelRequestMessage struct {
	Role          string `json:"role"`
	ToolName      string `json:"tool_name,omitempty"`
	ToolCallID    string `json:"tool_call_id,omitempty"`
	ContentSHA256 string `json:"content_sha256"`
	ByteLen       int    `json:"byte_len"`
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
	ToolCallID string            `json:"tool_call_id"`
	ToolName   string            `json:"tool_name"`
	Result     string            `json:"result"`
	Parts      []json.RawMessage `json:"parts,omitempty"`
	Error      string            `json:"error,omitempty"`
}

// payloadToolMounted journals the tools a mounting tool (today: skill_view)
// newly activated mid-run. ToolName is the mounting tool; Tools lists only
// the names that became mounted by this call (TT-3 audit trail).
type payloadToolMounted struct {
	ToolName string   `json:"tool_name"`
	Tools    []string `json:"tools"`
}

// payloadToolApprovalRequired is committed in the single journal commit
// that follows a durable checkpoint (D-029 write order).
type payloadToolApprovalRequired struct {
	ApprovalID       string         `json:"approval_id"`
	ToolCallID       string         `json:"tool_call_id"`
	ToolName         string         `json:"tool_name"`
	Args             map[string]any `json:"args"`
	ExpiresAt        int64          `json:"expires_at"`
	SelectedTools    []string       `json:"selected_tools,omitempty"`
	Mode             string         `json:"mode,omitempty"`
	Face             string         `json:"face"`
	PolicyProfile    string         `json:"policy_profile,omitempty"`
	PolicyHash       string         `json:"policy_hash,omitempty"`
	SandboxMode      string         `json:"sandbox_mode,omitempty"`
	ApprovalPolicy   string         `json:"approval_policy,omitempty"`
	Action           string         `json:"action,omitempty"`
	Target           string         `json:"target,omitempty"`
	PreconditionHash string         `json:"precondition_hash,omitempty"`
	Preview          string         `json:"preview,omitempty"`
	RiskFindings     []string       `json:"risk_findings,omitempty"`
}

type payloadUserQuestionRequired struct {
	QuestionID     string   `json:"question_id"`
	ToolCallID     string   `json:"tool_call_id"`
	Prompt         string   `json:"prompt"`
	ExpiresAt      int64    `json:"expires_at"`
	ResumeTarget   string   `json:"resume_target"`
	SelectedTools  []string `json:"selected_tools,omitempty"`
	Mode           string   `json:"mode,omitempty"`
	Face           string   `json:"face"`
	PolicyProfile  string   `json:"policy_profile,omitempty"`
	PolicyHash     string   `json:"policy_hash,omitempty"`
	SandboxMode    string   `json:"sandbox_mode,omitempty"`
	ApprovalPolicy string   `json:"approval_policy,omitempty"`
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

type payloadApprovalDecided struct {
	ApprovalID string `json:"approval_id"`
	Decision   string `json:"decision"`
	Actor      string `json:"actor,omitempty"`
	Reason     string `json:"reason,omitempty"`
	DecidedAt  int64  `json:"decided_at"`
}

type payloadApprovalCancelled struct {
	ApprovalID string `json:"approval_id"`
	Actor      string `json:"actor,omitempty"`
	Reason     string `json:"reason,omitempty"`
}

type payloadInteractionExpired struct {
	ReviewID  string `json:"review_id"`
	Kind      string `json:"kind"`
	ExpiresAt int64  `json:"expires_at"`
	Reason    string `json:"reason"`
}

type payloadProposalStale struct {
	ApprovalID string `json:"approval_id"`
	Target     string `json:"target,omitempty"`
	Reason     string `json:"reason"`
}

type payloadQuestionCancelled struct {
	QuestionID string `json:"question_id"`
	Actor      string `json:"actor,omitempty"`
	Reason     string `json:"reason,omitempty"`
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
	causeHumanTimeout  = "human_timeout"
	causeLoopDetected  = "loop_detected"
)

type payloadRunFailed struct {
	CauseCategory string `json:"cause_category"`
	Message       string `json:"message"`
}

// Provider failures intentionally collapse to one stable, actionable user
// message. The detailed cause remains in the structured server log only.
const providerUnavailableMessage = "无法连接！请检查供应商配置！"

// Cancel reasons for run.cancelled.
const (
	reasonUserRequested = "user_requested"
	reasonRecovery      = "recovery"
)

type payloadRunCancelled struct {
	Reason string `json:"reason"`
}

type payloadChildRequested struct {
	ParentRunID string `json:"parent_run_id"`
	Depth       int    `json:"depth"`
	Text        string `json:"text"`
}

type payloadChildStarted struct {
	ParentRunID string `json:"parent_run_id"`
	WorkspaceID string `json:"workspace_id,omitempty"`
}

type payloadChildSuspended struct {
	ApprovalID string `json:"approval_id"`
	ToolCallID string `json:"tool_call_id"`
}

type payloadChildResumed struct {
	ApprovalID string `json:"approval_id"`
	Decision   string `json:"decision"`
}

type payloadChildCompleted struct {
	Summary string `json:"summary,omitempty"`
}

type payloadChildFailed struct {
	CauseCategory string `json:"cause_category"`
	Message       string `json:"message"`
}

type payloadChildCancelled struct {
	Reason string `json:"reason"`
}
