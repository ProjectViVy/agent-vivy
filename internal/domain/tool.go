package domain

import "encoding/json"

// ToolCall is one requested tool invocation inside a run. Args is JSON
// whose shape is fixed by the tool's Spec.
type ToolCall struct {
	ID    string
	RunID RunID
	Spec  ToolSpec
	Args  []byte
}

// ToolProposal is the reviewable mutation plan prepared before an effectful
// tool is allowed to continue. The payload is deliberately generic so file,
// Skill, HTTP, MCP, and process tools can share the same HITL lifecycle while
// keeping their domain validation in the owning adapter.
type ToolProposal struct {
	Action           string
	Target           string
	PreconditionHash string
	Preview          string
	RiskFindings     []string
	Data             json.RawMessage
}

// ToolSpec describes a registered tool. Readonly tools auto-execute;
// effectful tools are approval-gated (D-012). Params declares the
// argument schema so the runtime can publish it to the model; without
// it, real gateways hallucinate argument names.
type ToolSpec struct {
	Name        string
	Description string
	Readonly    bool
	Params      map[string]ToolParam
	// Keywords are deterministic request-routing hints owned by Vivy. They
	// are not sent to the provider as a second schema; the runtime uses them
	// to select the smallest tool set for one run.
	Keywords []string
	// Interaction identifies a control-flow tool that suspends the run.
	Interaction ToolInteraction
}

// ToolInteraction distinguishes ordinary calls from user-input suspension.
type ToolInteraction string

const (
	ToolInteractionNone     ToolInteraction = ""
	ToolInteractionQuestion ToolInteraction = "question"
)

// ToolParam describes one tool argument. V0 tools take only strings;
// the type stays implicit until a richer tool set arrives.
type ToolParam struct {
	Desc     string
	Required bool
	// Type is a JSON Schema primitive/collection name. Empty preserves the
	// V0 string contract for legacy tools.
	Type string
	Enum []string
}

// Approval decisions. ApprovalPending marks a row that has not been
// decided yet; Decisions are exactly approved or denied (D-009).
const (
	ApprovalPending   = "pending"
	ApprovalApproved  = "approved"
	ApprovalDenied    = "denied"
	ApprovalExpired   = "expired"
	ApprovalStale     = "stale"
	ApprovalCancelled = "cancelled"
)

// ApprovalKind identifies the execution owner that will receive a decision.
// The existing run kind remains the default for backwards-compatible rows.
const (
	ApprovalKindRun   = "run"
	ApprovalKindChild = "child"
)

// Approval records a server-side decision for an effectful tool call
// (D-009). Authority is server-side only; ExpiresAt bounds validity.
type Approval struct {
	ID             string
	RunID          RunID
	ToolCallID     string
	Decision       string // ApprovalPending | ApprovalApproved | ApprovalDenied
	ExpiresAt      int64  // unix milli
	CreatedAt      int64  // unix milli
	DecidedAt      int64  // unix milli
	Actor          string
	DecisionReason string
	// ResumeTarget is the eino interrupt key the decision feeds back to
	// (ResumeWithParams target); persisted so a decision can resume even
	// after bookkeeping restarts (C6).
	ResumeTarget string
	Kind         string
	ToolName     string
	// Proposal fields make the human review auditable and allow the resumed
	// tool to fail closed when the approved target has changed.
	Action           string
	Target           string
	PreconditionHash string
	Preview          string
	RiskFindings     []string
	ProposalData     json.RawMessage
	// Sandbox fields control permission boundaries (D-021).
	SandboxMode    string // read_only | workspace_write | danger_full_access
	ApprovalPolicy string // ask | never | auto
	TimeoutAt      int64  // unix milli when this approval expires
}
