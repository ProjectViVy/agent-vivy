package storage

import (
	"context"
	"errors"

	"agent-vivy/internal/domain"
)

var (
	ErrWorkVersionConflict = errors.New("storage: work version conflict")
	ErrWorkRequestConflict = errors.New("storage: work request conflict")
	ErrWorkInvalidMutation = errors.New("storage: invalid work mutation")
	ErrWorkEventCorrupt    = errors.New("storage: corrupt work event")
)

// WorkCommitResult is the durable result of one work mutation. Replayed results
// return the original event and the state immediately after that event.
type WorkCommitResult struct {
	State    domain.WorkState
	Event    domain.WorkEvent
	Replayed bool
}

// WorkStore is the optional session work-control extension. It is deliberately
// separate from Engine until runtime admission consumes the complete contract.
type WorkStore interface {
	ReadWork(ctx context.Context, sessionID domain.SessionID) (domain.WorkState, error)
	ReplayWork(ctx context.Context, sessionID domain.SessionID, after domain.WorkVersion, limit int) ([]domain.WorkEvent, error)
	CommitWork(ctx context.Context, mutation domain.WorkMutation) (WorkCommitResult, error)
}

// GoalRunCommit is the complete set of ordinary run records admitted by one
// Goal round. First-party backends persist all four records atomically.
type GoalRunCommit struct {
	Mutation domain.WorkMutation
	Message  domain.Message
	Run      domain.Run
	Started  domain.RunEvent
}

// GoalRunCommitResult returns the durable work event and the run-start record.
// Replayed requests return the original admitted run without inserting rows.
type GoalRunCommitResult struct {
	Work    WorkCommitResult
	Run     domain.Run
	Started domain.RunEvent
}

// GoalRunStore is the atomic Goal-round admission extension.
type GoalRunStore interface {
	CommitGoalRun(ctx context.Context, admission GoalRunCommit) (GoalRunCommitResult, error)
}

// ValidateGoalRunCommit checks the cross-record identity invariants for an
// atomic Goal round admission.
func ValidateGoalRunCommit(admission GoalRunCommit) error {
	if err := ValidateWorkMutation(admission.Mutation); err != nil {
		return err
	}
	if admission.Mutation.Kind != domain.WorkEventGoalRoundAdmitted ||
		admission.Mutation.Admission.SessionID != admission.Mutation.SessionID ||
		admission.Mutation.Admission.RunID == "" {
		return ErrWorkInvalidMutation
	}
	if admission.Message.ID == "" ||
		admission.Message.SessionID != admission.Mutation.SessionID ||
		admission.Message.RunID != admission.Mutation.Admission.RunID {
		return ErrWorkInvalidMutation
	}
	if admission.Run.ID != admission.Mutation.Admission.RunID ||
		admission.Run.SessionID != admission.Mutation.SessionID ||
		admission.Run.Status != domain.RunActive ||
		(admission.Run.Kind != "" && admission.Run.Kind != domain.RunKindPrimary) {
		return ErrWorkInvalidMutation
	}
	if admission.Started.RunID != admission.Run.ID ||
		admission.Started.Type != domain.EventRunStarted ||
		admission.Started.PayloadVersion <= 0 {
		return ErrWorkInvalidMutation
	}
	return nil
}

// ValidateWorkMutation checks the host-authenticated request envelope before
// opening a backend transaction. Lifecycle-specific validation remains in the
// domain reducer.
func ValidateWorkMutation(mutation domain.WorkMutation) error {
	if mutation.SessionID == "" ||
		mutation.ExpectedVersion < 0 ||
		mutation.RequestID == "" ||
		mutation.RequestHash == "" ||
		!mutation.Kind.Valid() {
		return ErrWorkInvalidMutation
	}
	if mutation.Kind == domain.WorkEventGoalRoundAdmitted {
		if mutation.Admission.SessionID != mutation.SessionID ||
			mutation.Admission.Goal.ID == "" ||
			mutation.Admission.Round <= 0 ||
			mutation.Admission.RunID == "" {
			return ErrWorkInvalidMutation
		}
		return nil
	}
	if mutation.Admission != (domain.GoalRunAdmission{}) || mutation.Goal.ID == "" {
		return ErrWorkInvalidMutation
	}
	return nil
}
