package domain

import (
	"errors"
	"fmt"
	"reflect"
)

// WorkSeq is the session-scoped sequence assigned to a work event.
type WorkSeq int64

// WorkVersion is the session-scoped version of folded work state.
type WorkVersion int64

const WorkPayloadVersion = 1

// WorkControlLimits are the single source of truth shared by model tools,
// runtime operations, and RPC validation.
const MaxPlanMarkdownBytes = 256 << 10
const MaxGoalObjectiveBytes = 8 << 10
const MaxGoalRounds = 1000

// WorkEventKind is the bounded vocabulary of session work events.
type WorkEventKind string

const WorkEventGoalCreated WorkEventKind = "goal.created"
const WorkEventGoalEdited WorkEventKind = "goal.edited"
const WorkEventGoalPaused WorkEventKind = "goal.paused"
const WorkEventGoalResumed WorkEventKind = "goal.resumed"
const WorkEventGoalCompleted WorkEventKind = "goal.completed"
const WorkEventGoalBlocked WorkEventKind = "goal.blocked"
const WorkEventGoalCleared WorkEventKind = "goal.cleared"
const WorkEventGoalRoundAdmitted WorkEventKind = "goal.round_admitted"
const WorkEventPlanEntered WorkEventKind = "plan.entered"
const WorkEventPlanLeft WorkEventKind = "plan.left"
const WorkEventPlanSubmitted WorkEventKind = "plan.submitted"
const WorkEventPlanReviewSuspended WorkEventKind = "plan.review_suspended"
const WorkEventPlanReviewCancelled WorkEventKind = "plan.review_cancelled"
const WorkEventPlanDecided WorkEventKind = "plan.decided"

// Valid reports whether k is a supported work event kind.
func (k WorkEventKind) Valid() bool {
	switch k {
	case WorkEventGoalCreated,
		WorkEventGoalEdited,
		WorkEventGoalPaused,
		WorkEventGoalResumed,
		WorkEventGoalCompleted,
		WorkEventGoalBlocked,
		WorkEventGoalCleared,
		WorkEventGoalRoundAdmitted,
		WorkEventPlanEntered,
		WorkEventPlanLeft,
		WorkEventPlanSubmitted,
		WorkEventPlanReviewSuspended,
		WorkEventPlanReviewCancelled,
		WorkEventPlanDecided:
		return true
	}
	return false
}

// WorkPhase is the durable lifecycle phase of a Goal.
type WorkPhase string

const WorkPhaseActive WorkPhase = "active"
const WorkPhasePaused WorkPhase = "paused"
const WorkPhaseBlocked WorkPhase = "blocked"
const WorkPhaseCompleted WorkPhase = "completed"

// GoalRef identifies one revision of a session Goal.
type GoalRef struct {
	ID       string
	Revision int64
}

// GoalState is the folded durable state of the current Goal.
type GoalState struct {
	Ref           GoalRef
	Objective     string
	Phase         WorkPhase
	MaxRounds     int
	RoundsStarted int
	Reason        string
	EvidenceRunID RunID
}

// PlanReviewStatus is the durable state of the current plan submission.
type PlanReviewStatus string

const (
	PlanReviewNone      PlanReviewStatus = "none"
	PlanReviewPending   PlanReviewStatus = "pending"
	PlanReviewAccepted  PlanReviewStatus = "accepted"
	PlanReviewRejected  PlanReviewStatus = "rejected"
	PlanReviewCancelled PlanReviewStatus = "cancelled"
	PlanReviewExpired   PlanReviewStatus = "expired"
)

// PlanDecisionAction is the bounded human review vocabulary.
type PlanDecisionAction string

const (
	PlanDecisionRevise      PlanDecisionAction = "revise"
	PlanDecisionExecuteOnce PlanDecisionAction = "execute_once"
	PlanDecisionStartGoal   PlanDecisionAction = "start_goal"
)

// PlanState is the current session collaboration state. The immutable
// submission body is kept here for the current view and remains replayable
// through the work event stream.
type PlanState struct {
	Active           bool
	SubmissionID     string
	Markdown         string
	ReviewStatus     PlanReviewStatus
	Feedback         string
	OriginRunID      RunID
	OriginToolCallID string
	ResumeTarget     string
	BlockedToolCalls []string
	DecisionAction   PlanDecisionAction
}

// WorkState is the pure reducer state for one session.
type WorkState struct {
	SessionID SessionID
	Version   WorkVersion
	Goal      *GoalState
	Plan      PlanState
}

// WorkMutation is the versioned Goal lifecycle payload of a WorkEvent.
type WorkMutation struct {
	SessionID            SessionID
	ExpectedVersion      WorkVersion
	RequestID            string
	RequestHash          string
	Kind                 WorkEventKind
	Goal                 GoalRef
	Objective            string
	MaxRounds            int
	Reason               string
	EvidenceRunID        RunID
	Admission            GoalRunAdmission
	PlanSubmissionID     string
	PlanMarkdown         string
	PlanAction           PlanDecisionAction
	PlanFeedback         string
	PlanOriginRunID      RunID
	PlanOriginToolCallID string
	PlanResumeTarget     string
	PlanBlockedToolCalls []string
}

// GoalRunAdmission records one atomically admitted Goal round.
type GoalRunAdmission struct {
	SessionID SessionID
	Goal      GoalRef
	Round     int
	RunID     RunID
}

// WorkEvent is one session-scoped, versioned work mutation.
type WorkEvent struct {
	SessionID      SessionID
	Seq            WorkSeq
	Kind           WorkEventKind
	PayloadVersion int
	RequestID      string
	RequestHash    string
	CreatedAt      int64
	Mutation       WorkMutation
	Admission      GoalRunAdmission
}

var ErrNonContiguousWorkSeq = errors.New("non-contiguous work sequence")
var ErrUnsupportedWorkPayloadVersion = errors.New("unsupported work payload version")
var ErrUnsupportedWorkEventKind = errors.New("unsupported work event kind")
var ErrStaleGoalReference = errors.New("stale goal reference")
var ErrGoalArmed = errors.New("goal_armed")
var ErrWorkRoundLimit = errors.New("work round limit exceeded")
var ErrInvalidWorkCursor = errors.New("invalid work replay cursor")

// FoldWork strictly reduces events into session work state.
func FoldWork(events []WorkEvent) (WorkState, error) {
	return FoldWorkFrom(WorkState{}, events)
}

// FoldWorkFrom strictly reduces a contiguous page from a caller-carried state.
// The input state is not mutated, including when the page is invalid.
func FoldWorkFrom(cursor WorkState, events []WorkEvent) (WorkState, error) {
	if cursor.Version < 0 || (cursor.Version != 0 && cursor.SessionID == "") ||
		(cursor.Version == 0 && (cursor.Goal != nil || !reflect.DeepEqual(cursor.Plan, PlanState{}))) {
		return WorkState{}, ErrInvalidWorkCursor
	}
	state := cursor
	if cursor.Goal != nil {
		goal := *cursor.Goal
		state.Goal = &goal
	}
	state.Plan.BlockedToolCalls = append([]string(nil), cursor.Plan.BlockedToolCalls...)
	for _, event := range events {
		if event.SessionID == "" {
			return WorkState{}, fmt.Errorf("%w: empty event session", ErrStaleGoalReference)
		}
		if event.Seq != WorkSeq(state.Version)+1 {
			return WorkState{}, fmt.Errorf("%w: got %d after %d", ErrNonContiguousWorkSeq, event.Seq, state.Version)
		}
		if event.PayloadVersion != WorkPayloadVersion {
			return WorkState{}, fmt.Errorf("%w: got %d", ErrUnsupportedWorkPayloadVersion, event.PayloadVersion)
		}
		if !event.Kind.Valid() {
			return WorkState{}, fmt.Errorf("%w: %q", ErrUnsupportedWorkEventKind, event.Kind)
		}
		if state.SessionID == "" {
			state.SessionID = event.SessionID
		}
		if event.SessionID != state.SessionID {
			return WorkState{}, fmt.Errorf("%w: event session %q does not match %q", ErrStaleGoalReference, event.SessionID, state.SessionID)
		}

		next := state
		if err := applyWorkEvent(&next, event); err != nil {
			return WorkState{}, err
		}
		next.Version = WorkVersion(event.Seq)
		state = next
	}
	return state, nil
}

func applyWorkEvent(state *WorkState, event WorkEvent) error {
	switch event.Kind {
	case WorkEventGoalCreated:
		return createGoal(state, event.Mutation)
	case WorkEventGoalEdited:
		return editGoal(state, event.Mutation)
	case WorkEventGoalPaused:
		return transitionGoal(state, event.Mutation, WorkPhasePaused)
	case WorkEventGoalResumed:
		return transitionGoal(state, event.Mutation, WorkPhaseActive)
	case WorkEventGoalCompleted:
		return transitionGoal(state, event.Mutation, WorkPhaseCompleted)
	case WorkEventGoalBlocked:
		return transitionGoal(state, event.Mutation, WorkPhaseBlocked)
	case WorkEventGoalCleared:
		if event.Mutation.SessionID != state.SessionID {
			return fmt.Errorf("%w: mutation session %q", ErrStaleGoalReference, event.Mutation.SessionID)
		}
		if err := requireGoal(state, event.Mutation.Goal); err != nil {
			return err
		}
		state.Goal = nil
		return nil
	case WorkEventGoalRoundAdmitted:
		return admitGoalRound(state, event.Admission)
	case WorkEventPlanEntered:
		return enterPlan(state, event.Mutation)
	case WorkEventPlanLeft:
		return leavePlan(state, event.Mutation)
	case WorkEventPlanSubmitted:
		return submitPlan(state, event.Mutation)
	case WorkEventPlanReviewSuspended:
		return suspendPlanReview(state, event.Mutation)
	case WorkEventPlanReviewCancelled:
		return cancelPlanReview(state, event.Mutation)
	case WorkEventPlanDecided:
		return decidePlan(state, event.Mutation)
	}
	return fmt.Errorf("%w: %q", ErrUnsupportedWorkEventKind, event.Kind)
}

func enterPlan(state *WorkState, mutation WorkMutation) error {
	if mutation.SessionID != state.SessionID {
		return fmt.Errorf("%w: plan session %q", ErrStaleGoalReference, mutation.SessionID)
	}
	if state.Plan.Active {
		return fmt.Errorf("%w: plan already active", ErrStaleGoalReference)
	}
	if state.Goal != nil && state.Goal.Phase == WorkPhaseActive {
		return fmt.Errorf("%w: %w: active Goal must be paused before Plan", ErrGoalArmed, ErrStaleGoalReference)
	}
	state.Plan = PlanState{Active: true, ReviewStatus: PlanReviewNone}
	return nil
}

func leavePlan(state *WorkState, mutation WorkMutation) error {
	if mutation.SessionID != state.SessionID || !state.Plan.Active {
		return fmt.Errorf("%w: Plan is not active", ErrStaleGoalReference)
	}
	state.Plan.Active = false
	if state.Plan.ReviewStatus == PlanReviewPending {
		state.Plan.ReviewStatus = PlanReviewCancelled
	}
	return nil
}

func submitPlan(state *WorkState, mutation WorkMutation) error {
	if mutation.SessionID != state.SessionID ||
		!state.Plan.Active ||
		mutation.PlanSubmissionID == "" ||
		mutation.PlanMarkdown == "" ||
		mutation.PlanOriginRunID == "" ||
		mutation.PlanOriginToolCallID == "" {
		return fmt.Errorf("%w: invalid plan submission", ErrStaleGoalReference)
	}
	if state.Plan.ReviewStatus == PlanReviewPending {
		return fmt.Errorf("%w: plan review already pending", ErrStaleGoalReference)
	}
	state.Plan.SubmissionID = mutation.PlanSubmissionID
	state.Plan.Markdown = mutation.PlanMarkdown
	state.Plan.ReviewStatus = PlanReviewPending
	state.Plan.Feedback = ""
	state.Plan.OriginRunID = mutation.PlanOriginRunID
	state.Plan.OriginToolCallID = mutation.PlanOriginToolCallID
	state.Plan.ResumeTarget = ""
	state.Plan.BlockedToolCalls = nil
	state.Plan.DecisionAction = ""
	return nil
}

func suspendPlanReview(state *WorkState, mutation WorkMutation) error {
	if mutation.SessionID != state.SessionID ||
		state.Plan.ReviewStatus != PlanReviewPending ||
		mutation.PlanSubmissionID == "" || mutation.PlanSubmissionID != state.Plan.SubmissionID ||
		mutation.PlanOriginRunID == "" || mutation.PlanOriginRunID != state.Plan.OriginRunID ||
		mutation.PlanOriginToolCallID == "" || mutation.PlanOriginToolCallID != state.Plan.OriginToolCallID ||
		mutation.PlanResumeTarget == "" {
		return fmt.Errorf("%w: invalid Plan review suspension", ErrStaleGoalReference)
	}
	state.Plan.ResumeTarget = mutation.PlanResumeTarget
	state.Plan.BlockedToolCalls = append([]string(nil), mutation.PlanBlockedToolCalls...)
	return nil
}

func cancelPlanReview(state *WorkState, mutation WorkMutation) error {
	if mutation.SessionID != state.SessionID || state.Plan.ReviewStatus != PlanReviewPending ||
		mutation.PlanSubmissionID == "" || mutation.PlanSubmissionID != state.Plan.SubmissionID ||
		mutation.PlanOriginRunID == "" || mutation.PlanOriginRunID != state.Plan.OriginRunID {
		return fmt.Errorf("%w: invalid Plan review cancellation", ErrStaleGoalReference)
	}
	state.Plan.ReviewStatus = PlanReviewCancelled
	return nil
}

func decidePlan(state *WorkState, mutation WorkMutation) error {
	if mutation.SessionID != state.SessionID ||
		!state.Plan.Active ||
		state.Plan.ReviewStatus != PlanReviewPending ||
		mutation.PlanSubmissionID == "" ||
		mutation.PlanSubmissionID != state.Plan.SubmissionID {
		return fmt.Errorf("%w: invalid plan decision", ErrStaleGoalReference)
	}
	if state.Plan.OriginRunID == "" || state.Plan.OriginToolCallID == "" || state.Plan.ResumeTarget == "" {
		return fmt.Errorf("%w: Plan review has not been suspended", ErrStaleGoalReference)
	}
	switch mutation.PlanAction {
	case PlanDecisionRevise:
		if mutation.Goal != (GoalRef{}) || mutation.Objective != "" || mutation.MaxRounds != 0 {
			return fmt.Errorf("%w: revision cannot carry Goal fields", ErrStaleGoalReference)
		}
		state.Plan.ReviewStatus = PlanReviewRejected
		state.Plan.DecisionAction = mutation.PlanAction
		state.Plan.Feedback = mutation.PlanFeedback
	case PlanDecisionExecuteOnce:
		if mutation.Goal != (GoalRef{}) || mutation.Objective != "" || mutation.MaxRounds != 0 {
			return fmt.Errorf("%w: execute_once cannot carry Goal fields", ErrStaleGoalReference)
		}
		state.Plan.Active = false
		state.Plan.ReviewStatus = PlanReviewAccepted
		state.Plan.DecisionAction = mutation.PlanAction
		state.Plan.Feedback = mutation.PlanFeedback
	case PlanDecisionStartGoal:
		if state.Goal != nil || mutation.Goal.ID == "" || mutation.Goal.Revision != 1 || mutation.Objective == "" || mutation.MaxRounds <= 0 {
			return fmt.Errorf("%w: start_goal requires a new Goal", ErrStaleGoalReference)
		}
		state.Plan.Active = false
		state.Plan.ReviewStatus = PlanReviewAccepted
		state.Plan.DecisionAction = mutation.PlanAction
		state.Plan.Feedback = mutation.PlanFeedback
		state.Goal = &GoalState{Ref: mutation.Goal, Objective: mutation.Objective, Phase: WorkPhaseActive, MaxRounds: mutation.MaxRounds}
	default:
		return fmt.Errorf("%w: unsupported plan decision %q", ErrStaleGoalReference, mutation.PlanAction)
	}
	return nil
}

func createGoal(state *WorkState, mutation WorkMutation) error {
	if state.Goal != nil {
		return fmt.Errorf("%w: goal already exists", ErrStaleGoalReference)
	}
	if state.Plan.Active {
		return fmt.Errorf("%w: Plan is active", ErrStaleGoalReference)
	}
	if mutation.SessionID != state.SessionID ||
		mutation.Goal.ID == "" ||
		mutation.Goal.Revision != 1 ||
		mutation.Objective == "" ||
		mutation.MaxRounds <= 0 {
		return fmt.Errorf("%w: invalid goal creation", ErrStaleGoalReference)
	}
	state.Goal = &GoalState{
		Ref:           mutation.Goal,
		Objective:     mutation.Objective,
		Phase:         WorkPhaseActive,
		MaxRounds:     mutation.MaxRounds,
		EvidenceRunID: mutation.EvidenceRunID,
	}
	return nil
}

func editGoal(state *WorkState, mutation WorkMutation) error {
	if err := requireGoal(state, mutation.Goal); err != nil {
		return err
	}
	if mutation.SessionID != state.SessionID ||
		mutation.Objective == "" ||
		mutation.MaxRounds < state.Goal.RoundsStarted ||
		mutation.MaxRounds <= 0 {
		return fmt.Errorf("%w: invalid goal edit", ErrStaleGoalReference)
	}
	state.Goal.Ref.Revision++
	state.Goal.Objective = mutation.Objective
	state.Goal.MaxRounds = mutation.MaxRounds
	return nil
}

func transitionGoal(state *WorkState, mutation WorkMutation, phase WorkPhase) error {
	if err := requireGoal(state, mutation.Goal); err != nil {
		return err
	}
	if mutation.SessionID != state.SessionID {
		return fmt.Errorf("%w: mutation session %q", ErrStaleGoalReference, mutation.SessionID)
	}
	if state.Goal.Phase != WorkPhaseActive && phase != WorkPhaseActive {
		return fmt.Errorf("%w: goal phase %q", ErrStaleGoalReference, state.Goal.Phase)
	}
	if phase == WorkPhaseActive && state.Goal.Phase != WorkPhasePaused {
		return fmt.Errorf("%w: goal phase %q", ErrStaleGoalReference, state.Goal.Phase)
	}
	if phase == WorkPhaseActive && state.Plan.Active {
		return fmt.Errorf("%w: Plan is active", ErrStaleGoalReference)
	}
	state.Goal.Phase = phase
	state.Goal.Reason = mutation.Reason
	state.Goal.EvidenceRunID = mutation.EvidenceRunID
	return nil
}

func requireGoal(state *WorkState, ref GoalRef) error {
	if state.Goal == nil || state.Goal.Ref != ref {
		return fmt.Errorf("%w: got %#v", ErrStaleGoalReference, ref)
	}
	return nil
}

func admitGoalRound(state *WorkState, admission GoalRunAdmission) error {
	if err := requireGoal(state, admission.Goal); err != nil {
		return err
	}
	if state.Goal.Phase != WorkPhaseActive {
		return fmt.Errorf("%w: goal phase %q", ErrStaleGoalReference, state.Goal.Phase)
	}
	if admission.SessionID != state.SessionID ||
		admission.RunID == "" ||
		admission.Round != state.Goal.RoundsStarted+1 {
		return fmt.Errorf("%w: invalid round admission", ErrStaleGoalReference)
	}
	if state.Goal.RoundsStarted >= state.Goal.MaxRounds {
		return fmt.Errorf("%w: max rounds %d", ErrWorkRoundLimit, state.Goal.MaxRounds)
	}
	state.Goal.RoundsStarted++
	state.Goal.Reason = ""
	state.Goal.EvidenceRunID = admission.RunID
	return nil
}
