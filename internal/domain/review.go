package domain

import "encoding/json"

// ReviewKind identifies the interaction surface that is waiting for a human.
// Both kinds share the ReviewItem projection, while their response actions
// remain intentionally different: approve/deny versus answer/cancel.
type ReviewKind string

const (
	ReviewKindApproval ReviewKind = "approval"
	ReviewKindQuestion ReviewKind = "question"
)

// ReviewStatus is the durable, queryable lifecycle shown by Review Center.
type ReviewStatus string

const (
	ReviewPending   ReviewStatus = "pending"
	ReviewApproved  ReviewStatus = "approved"
	ReviewDenied    ReviewStatus = "denied"
	ReviewAnswered  ReviewStatus = "answered"
	ReviewCancelled ReviewStatus = "cancelled"
	ReviewExpired   ReviewStatus = "expired"
	ReviewStale     ReviewStatus = "stale"
)

// ReviewItem is the read model for the global Review Center and the inline
// run inspector. Sensitive request data is represented only by the redacted
// Arguments projection; ProposalData remains runtime-owned and is never
// returned by the review API.
type ReviewItem struct {
	ID               string
	Kind             ReviewKind
	Status           ReviewStatus
	SessionID        SessionID
	SessionTitle     string
	RunID            RunID
	ToolCallID       string
	ToolName         string
	Source           string
	Actor            string
	CreatedAt        int64
	ExpiresAt        int64
	DecidedAt        int64
	Action           string
	Target           string
	PreconditionHash string
	Preview          string
	RiskFindings     []string
	Arguments        json.RawMessage
	Prompt           string
	DecisionReason   string
	StaleReason      string
	Error            string
	Effect           string
	Reversibility    string
	Scope            string
	Trust            string
}
