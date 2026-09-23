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
	ErrWorkRunConflict     = errors.New("storage: active run conflict")
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
// ReplayWork starts from WorkState{SessionID: sessionID}; callers carry each
// returned state to the next bounded page.
type WorkStore interface {
	ReadWork(ctx context.Context, sessionID domain.SessionID) (domain.WorkState, error)
	ReplayWork(ctx context.Context, sessionID domain.SessionID, cursor domain.WorkState, limit int) ([]domain.WorkEvent, domain.WorkState, error)
	CommitWork(ctx context.Context, mutation domain.WorkMutation) (WorkCommitResult, error)
}

// GoalRunCommit is the complete set of ordinary run records admitted by one
// Goal round. First-party backends persist all four records atomically.
type GoalRunCommit struct {
	Mutation     domain.WorkMutation
	Message      domain.Message
	Run          domain.Run
	Started      domain.RunEvent
	Prompt       *RunPromptSnapshot
	ExpectedMask *MaskCaptureCheck
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

// PrimaryRunCommit is the transaction boundary for an ordinary primary
// run. The message, active run row, and run.started event are committed
// together so a rejected concurrent admission cannot leave an orphan turn.
type PrimaryRunCommit struct {
	Message      domain.Message
	Run          domain.Run
	Started      domain.RunEvent
	Prompt       *RunPromptSnapshot
	ExpectedMask *MaskCaptureCheck
}

// PrimaryRunStore atomically admits one ordinary primary run.
type PrimaryRunStore interface {
	CommitPrimaryRun(ctx context.Context, admission PrimaryRunCommit) (domain.RunEvent, error)
}

// ValidatePrimaryRunCommit checks the identity and lifecycle invariants for
// the ordinary primary-run admission transaction.
func ValidatePrimaryRunCommit(admission PrimaryRunCommit) error {
	if admission.Message.ID == "" ||
		admission.Message.SessionID == "" ||
		admission.Message.SessionID != admission.Run.SessionID ||
		admission.Message.RunID != admission.Run.ID ||
		admission.Message.Role != domain.RoleUser {
		return ErrWorkInvalidMutation
	}
	if admission.Run.ID == "" ||
		admission.Run.Status != domain.RunActive ||
		(admission.Run.Kind != "" && admission.Run.Kind != domain.RunKindPrimary) {
		return ErrWorkInvalidMutation
	}
	if admission.Started.RunID != admission.Run.ID ||
		admission.Started.Type != domain.EventRunStarted ||
		admission.Started.PayloadVersion <= 0 {
		return ErrWorkInvalidMutation
	}
	return ValidateRunAdmissionInput(RunAdmission{
		Message: admission.Message, Run: admission.Run, Started: admission.Started,
		Prompt: admission.Prompt, ExpectedMask: admission.ExpectedMask,
	})
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
	return ValidateRunAdmissionInput(RunAdmission{
		Message: admission.Message, Run: admission.Run, Started: admission.Started,
		Prompt: admission.Prompt, ExpectedMask: admission.ExpectedMask,
	})
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
	switch mutation.Kind {
	case domain.WorkEventPlanEntered,
		domain.WorkEventPlanLeft,
		domain.WorkEventPlanSubmitted:
		if mutation.Admission != (domain.GoalRunAdmission{}) ||
			mutation.Goal != (domain.GoalRef{}) {
			return ErrWorkInvalidMutation
		}
	case domain.WorkEventPlanReviewSuspended:
		if mutation.Admission != (domain.GoalRunAdmission{}) ||
			mutation.Goal != (domain.GoalRef{}) ||
			mutation.PlanSubmissionID == "" || mutation.PlanOriginRunID == "" ||
			mutation.PlanOriginToolCallID == "" || mutation.PlanResumeTarget == "" {
			return ErrWorkInvalidMutation
		}
	case domain.WorkEventPlanReviewCancelled:
		if mutation.Admission != (domain.GoalRunAdmission{}) ||
			mutation.Goal != (domain.GoalRef{}) || mutation.PlanSubmissionID == "" ||
			mutation.PlanOriginRunID == "" {
			return ErrWorkInvalidMutation
		}
	case domain.WorkEventPlanDecided:
		switch mutation.PlanAction {
		case domain.PlanDecisionRevise, domain.PlanDecisionExecuteOnce:
			if mutation.Admission != (domain.GoalRunAdmission{}) || mutation.Goal != (domain.GoalRef{}) || mutation.Objective != "" || mutation.MaxRounds != 0 {
				return ErrWorkInvalidMutation
			}
		case domain.PlanDecisionStartGoal:
			if mutation.Admission != (domain.GoalRunAdmission{}) || mutation.Goal.ID == "" || mutation.Goal.Revision != 1 || mutation.Objective == "" || mutation.MaxRounds <= 0 {
				return ErrWorkInvalidMutation
			}
		default:
			return ErrWorkInvalidMutation
		}
	default:
		if mutation.Admission != (domain.GoalRunAdmission{}) || mutation.Goal.ID == "" {
			return ErrWorkInvalidMutation
		}
	}
	return nil
}
