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
// effectful tools are approval-gated (D-012).
type ToolSpec struct {
	Name        string
	Description string
	Readonly    bool
}

// Approval decisions.
const (
	ApprovalApproved = "approved"
	ApprovalDenied   = "denied"
)

// Approval records a server-side decision for an effectful tool call
// (D-009). Authority is server-side only; ExpiresAt bounds validity.
type Approval struct {
	ID         string
	RunID      RunID
	ToolCallID string
	Decision   string // ApprovalApproved | ApprovalDenied
	ExpiresAt  int64  // unix milli
}
