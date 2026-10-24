package domain

// ToolOperationState records the durable recovery boundary around one
// logical tool call. A claimed operation is never automatically reclaimed:
// after its in-process owner disappears, an external effect may be unknown.
type ToolOperationState string

const (
	ToolOperationAdmitted  ToolOperationState = "admitted"
	ToolOperationClaimed   ToolOperationState = "claimed"
	ToolOperationCompleted ToolOperationState = "completed"
)

func (s ToolOperationState) Valid() bool {
	switch s {
	case ToolOperationAdmitted, ToolOperationClaimed, ToolOperationCompleted:
		return true
	default:
		return false
	}
}

// ToolOperation is the private durable invocation record for one logical
// tool call. Its exact inputs and typed outcome remain private; the Journal
// mirrors the existing lifecycle and bounded result/failure presentation.
type ToolOperation struct {
	RunID                    RunID
	OperationID              string
	ToolName                 string
	RequestArguments         []byte
	MiddlewareInputArguments []byte
	EffectiveArguments       []byte
	State                    ToolOperationState
	ClaimOwner               string
	Result                   string
	Failure                  string
	OutcomeStatus            string
	OutcomeReason            string
	OutcomeEffects           string
	CreatedAt                int64
	UpdatedAt                int64
}

// ToolOperationOutcome is committed with completion and its Journal event.
type ToolOperationOutcome struct {
	Result  string
	Failure string
	Status  string
	Reason  string
	Effects string
}

func (op ToolOperation) Outcome() ToolOperationOutcome {
	return ToolOperationOutcome{Result: op.Result, Failure: op.Failure, Status: op.OutcomeStatus, Reason: op.OutcomeReason, Effects: op.OutcomeEffects}
}

const (
	ToolOutcomeSuccess         = "success"
	ToolOutcomeRecoverable     = "recoverable"
	ToolOutcomeRefused         = "refused"
	ToolOutcomeFatal           = "fatal"
	ToolReasonInvalidArguments = "invalid_arguments"
	ToolReasonNotFound         = "not_found"
	ToolReasonPolicyDenied     = "policy_denied"
	ToolReasonUserDenied       = "user_denied"
	ToolReasonCommandFailed    = "command_failed"
	ToolReasonRemoteTool       = "remote_tool_error"
	ToolReasonChildFailed      = "child_failed"
	ToolReasonNetworkFailed    = "network_failed"
	ToolReasonToolError        = "tool_error"
	ToolEffectsNotExecuted     = "not_executed"
	ToolEffectsNone            = "none"
	ToolEffectsUnknown         = "unknown"
)
