package domain

// ToolCall is one requested tool invocation inside a run. Args is JSON
// whose shape is fixed by the tool's Spec.
type ToolCall struct {
	ID    string
	RunID RunID
	Spec  ToolSpec
	Args  []byte
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
}

// Approval decisions. ApprovalPending marks a row that has not been
// decided yet; Decisions are exactly approved or denied (D-009).
const (
	ApprovalPending  = "pending"
	ApprovalApproved = "approved"
	ApprovalDenied   = "denied"
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
	ID         string
	RunID      RunID
	ToolCallID string
	Decision   string // ApprovalPending | ApprovalApproved | ApprovalDenied
	ExpiresAt  int64  // unix milli
	// ResumeTarget is the eino interrupt key the decision feeds back to
	// (ResumeWithParams target); persisted so a decision can resume even
	// after bookkeeping restarts (C6).
	ResumeTarget string
	Kind         string
}
