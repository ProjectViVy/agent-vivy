package domain

// EventType is the RunEvent vocabulary. It is the single contract shared
// by the Eino-side mapping and the UI-side stream (FR-5, task A3 ships
// the matching JSON Schema under schemas/events/).
type EventType string

const (
	EventRunStarted            EventType = "run.started"
	EventProviderRetry         EventType = "provider.retry"
	EventProviderStall         EventType = "provider.stall"
	EventModelReasoningDelta   EventType = "model.reasoning_delta"
	EventModelDelta            EventType = "model.delta"
	EventModelUsage            EventType = "model.usage"
	EventModelCompleted        EventType = "model.completed"
	EventModelRequest          EventType = "model.request"
	EventToolRequested         EventType = "tool.requested"
	EventToolApprovalRequired  EventType = "tool.approval_required"
	EventToolApprovalDecided   EventType = "tool.approval_decided"
	EventToolApprovalExpired   EventType = "tool.approval_expired"
	EventToolApprovalCancelled EventType = "tool.approval_cancelled"
	EventToolProposalStale     EventType = "tool.proposal_stale"
	EventToolStarted           EventType = "tool.started"
	EventToolFinished          EventType = "tool.finished"
	EventPolicyEvaluated       EventType = "policy.evaluated"
	EventHookStarted           EventType = "hook.started"
	EventHookCompleted         EventType = "hook.completed"
	EventHookBlocked           EventType = "hook.blocked"
	EventUserQuestionRequired  EventType = "user.question_required"
	EventUserQuestionAnswered  EventType = "user.question_answered"
	EventUserQuestionCancelled EventType = "user.question_cancelled"
	EventUserQuestionExpired   EventType = "user.question_expired"
	EventChildRequested        EventType = "child.requested"
	EventChildStarted          EventType = "child.started"
	EventChildSuspended        EventType = "child.suspended"
	EventChildResumed          EventType = "child.resumed"
	EventChildCompleted        EventType = "child.completed"
	EventChildFailed           EventType = "child.failed"
	EventChildCancelled        EventType = "child.cancelled"
	EventRunCompleted          EventType = "run.completed"
	EventRunFailed             EventType = "run.failed"
	EventRunCancelled          EventType = "run.cancelled"
)

// EventTypes lists the full vocabulary in canonical order.
var EventTypes = []EventType{
	EventRunStarted,
	EventProviderRetry,
	EventProviderStall,
	EventModelReasoningDelta,
	EventModelDelta,
	EventModelUsage,
	EventModelCompleted,
	EventModelRequest,
	EventToolRequested,
	EventToolApprovalRequired,
	EventToolApprovalDecided,
	EventToolApprovalExpired,
	EventToolApprovalCancelled,
	EventToolProposalStale,
	EventToolStarted,
	EventToolFinished,
	EventPolicyEvaluated,
	EventHookStarted,
	EventHookCompleted,
	EventHookBlocked,
	EventUserQuestionRequired,
	EventUserQuestionAnswered,
	EventUserQuestionCancelled,
	EventUserQuestionExpired,
	EventChildRequested,
	EventChildStarted,
	EventChildSuspended,
	EventChildResumed,
	EventChildCompleted,
	EventChildFailed,
	EventChildCancelled,
	EventRunCompleted,
	EventRunFailed,
	EventRunCancelled,
}

// Valid reports whether the type is part of the vocabulary.
func (t EventType) Valid() bool {
	for _, known := range EventTypes {
		if known == t {
			return true
		}
	}
	return false
}

// Terminal reports whether the event closes a run. Exactly one of these
// must exist per run (D-008).
func (t EventType) Terminal() bool {
	switch t {
	case EventRunCompleted, EventRunFailed, EventRunCancelled,
		EventChildCompleted, EventChildFailed, EventChildCancelled:
		return true
	}
	return false
}

// RunStatus maps a terminal event to the run status it produces.
// Non-terminal events return ok=false.
func (t EventType) RunStatus() (RunStatus, bool) {
	switch t {
	case EventRunCompleted, EventChildCompleted:
		return RunCompleted, true
	case EventRunFailed, EventChildFailed:
		return RunFailed, true
	case EventRunCancelled, EventChildCancelled:
		return RunCancelled, true
	}
	return "", false
}

// RunEvent is one durable journal entry of a run. Payload is JSON whose
// shape is fixed by schemas/events/payloads/ (A3) and versioned by
// PayloadVersion.
type RunEvent struct {
	RunID          RunID
	Seq            EventSeq // monotonic per run
	Type           EventType
	CreatedAt      int64 // unix milli
	PayloadVersion int
	Payload        []byte
}
