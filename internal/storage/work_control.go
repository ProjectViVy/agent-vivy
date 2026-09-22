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
