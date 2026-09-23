package storage

import (
	"context"
	"errors"
	"fmt"

	"agent-vivy/internal/domain"
)

// Continuity operation names recorded in continuity_receipts.
const (
	// ContinuityOperationAdmission is the task admission receipt identity.
	// Turn admission uses a caller-stable request_id within the session.
	ContinuityOperationAdmission = "run.admission"
)

var (
	// ErrSourceChanged rejects an admission whose source expectations no
	// longer hold inside the commit transaction — the source was deleted or
	// its view-truncation revision moved between preview and commit.
	ErrSourceChanged = errors.New("storage: source expectations changed")
	// ErrCommitUncertain marks a transaction whose final commit outcome is
	// unknown to the caller (commit error, driver failure or context loss at
	// the commit boundary). Callers must preserve provisional resources on
	// this error until a receipt lookup settles the outcome.
	ErrCommitUncertain = errors.New("storage: commit outcome uncertain")
)

// SourceExpectation is a server-validated projection fact the admission
// transaction must still observe. Preview-time callers pin the source
// session's view-truncation revision (the count of rewind/edit markers); a
// negative TruncationMarkers means existence-only, for admissions built
// without a preview.
type SourceExpectation struct {
	SourceSessionID   domain.SessionID
	TruncationMarkers int64
	Digest            string
}

func (e SourceExpectation) validate() error {
	if e.SourceSessionID == "" {
		return fmt.Errorf("storage: continuity expectation has an empty source session")
	}
	return nil
}

// ContinuityAdmission carries the complete atomic task admission: the user
// row, the accepted run row, the ordered startup events, the accepted
// scope, exact source expectations and the idempotency receipt. Everything
// commits in one transaction or nothing does.
type ContinuityAdmission struct {
	SessionID    domain.SessionID
	Message      domain.Message
	Run          domain.Run
	Events       []domain.RunEvent
	Scope        domain.AcceptedHistoryScope
	Expectations []SourceExpectation
	Receipt      domain.ContinuityReceipt
	// TestFaultHook injects a failure immediately after the named write
	// step inside the transaction ("message", "run", "events", "receipt").
	// Backends invoke it only when non-nil; production callers leave it nil.
	TestFaultHook func(step string) error
}

func (a ContinuityAdmission) Validate() error {
	if a.SessionID == "" || a.Message.SessionID != a.SessionID || a.Run.SessionID != a.SessionID {
		return fmt.Errorf("storage: continuity admission session is inconsistent")
	}
	if a.Message.RunID != a.Run.ID || len(a.Events) == 0 {
		return fmt.Errorf("storage: continuity admission has no startup events")
	}
	if a.Events[0].Type != domain.EventRunStarted {
		return fmt.Errorf("storage: continuity admission first event must be run.started")
	}
	for _, e := range a.Events {
		if e.RunID != a.Run.ID {
			return fmt.Errorf("storage: continuity admission event %s belongs to another run", e.Type)
		}
	}
	if a.Receipt.Operation != ContinuityOperationAdmission || a.Receipt.SessionID != a.SessionID || a.Receipt.RequestID == "" || a.Receipt.InputHash == "" {
		return fmt.Errorf("storage: continuity admission receipt is invalid")
	}
	if a.Scope.DestinationSessionID != a.SessionID {
		return fmt.Errorf("storage: continuity admission scope destination mismatch")
	}
	for _, e := range a.Expectations {
		if err := e.validate(); err != nil {
			return err
		}
	}
	return nil
}

// ContinuityMutation carries one typed domain event append guarded by an
// idempotency receipt — the atomic form used by model-call operations. The
// receipt's request_id folds the stable tool_call_id under the owner run.
type ContinuityMutation struct {
	SessionID domain.SessionID
	RunID     domain.RunID
	Event     domain.RunEvent
	InputHash string
	Receipt   domain.ContinuityReceipt
	// TestFaultHook mirrors ContinuityAdmission.TestFaultHook for the
	// "events" and "receipt" write steps.
	TestFaultHook func(step string) error
}

func (m ContinuityMutation) Validate() error {
	if m.SessionID == "" || m.RunID == "" || m.Event.RunID != m.RunID {
		return fmt.Errorf("storage: continuity mutation owner is inconsistent")
	}
	if m.Event.Type == "" || m.Event.Type == domain.EventRunStarted || m.Event.Type.Terminal() {
		return fmt.Errorf("storage: continuity mutation event type is invalid")
	}
	if m.Receipt.Operation == "" || m.Receipt.SessionID != m.SessionID || m.Receipt.RequestID == "" || m.Receipt.InputHash == "" {
		return fmt.Errorf("storage: continuity mutation receipt is invalid")
	}
	if m.InputHash == "" || m.Receipt.InputHash != m.InputHash {
		return fmt.Errorf("storage: continuity mutation input hash is invalid")
	}
	return nil
}

// ContinuityResult is the committed outcome. On identical replay
// NewlyCommitted is false and Events carries the originally committed
// events in seq order, so a lost response returns the original result
// without launching a second engine or duplicating an attachment.
type ContinuityResult struct {
	RunID          domain.RunID
	Events         []domain.RunEvent
	Receipt        domain.ContinuityReceipt
	NewlyCommitted bool
}

// ContinuityStore commits task admissions and guarded operations
// atomically. The receipt is a rebuildable index into the authoritative
// events; the event payload remains the source of truth for the result.
type ContinuityStore interface {
	// CommitContinuityRun inserts message, run, the ordered startup events
	// and the admission receipt in one transaction. Source and destination
	// sessions lock in sorted-ID order before the expectations are
	// rechecked. An identical replay returns the original RunID and events
	// with NewlyCommitted=false; a different input hash is ErrConflict.
	CommitContinuityRun(ctx context.Context, a ContinuityAdmission) (ContinuityResult, error)
	// CommitContinuityOperation appends one typed event plus its receipt to
	// an existing run. The receipt lookup precedes the terminal check so an
	// identical replay still returns the original result after the run
	// finished; a genuinely new mutation after a terminal event fails
	// ErrRunClosed.
	CommitContinuityOperation(ctx context.Context, m ContinuityMutation) (ContinuityResult, error)
	// FindContinuityReceipt resolves one receipt without mutating state.
	FindContinuityReceipt(ctx context.Context, sessionID domain.SessionID, operation, requestID string) (domain.ContinuityReceipt, bool, error)
}
