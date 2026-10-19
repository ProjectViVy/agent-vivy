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
// tool call. The Journal mirrors its lifecycle and digests, while the exact
// validated, hook-adjusted arguments stay out of public event payloads.
type ToolOperation struct {
	RunID                    RunID
	OperationID              string
	ToolName                 string
	RequestDigest            string
	MiddlewareInputArguments []byte
	ArgumentsDigest          string
	EffectiveArguments       []byte
	State                    ToolOperationState
	ClaimOwner               string
	Result                   string
	Failure                  string
	CreatedAt                int64
	UpdatedAt                int64
	// ContentOrigin marks which trusted subsystem produced the result.
	// "notebook" is stamped only by the runtime for notebook-owned tools.
	ContentOrigin string
	// ExcludeAutomaticIngest bars the result and its derivatives from
	// automatic BML/cognitive ingestion and compaction summaries.
	ExcludeAutomaticIngest bool
}

// ContentOriginNotebook is the only provenance value the runtime stamps for
// notebook-owned tool operations.
const ContentOriginNotebook = "notebook"
