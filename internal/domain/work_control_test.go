package domain

import (
	"errors"
	"testing"
)

func TestFoldWorkGoalCreationCarriesCreatorEvidence(t *testing.T) {
	ref := GoalRef{ID: "goal-1", Revision: 1}
	event := goalEvent(1, WorkEventGoalCreated, ref, "ship it", 2)
	event.Mutation.EvidenceRunID = "run-creator"

	state, err := FoldWork([]WorkEvent{event})
	if err != nil {
		t.Fatalf("FoldWork() error = %v", err)
	}
	if state.Goal == nil || state.Goal.EvidenceRunID != "run-creator" {
		t.Fatalf("goal evidence = %#v, want creator run", state.Goal)
	}
}

func TestFoldWorkAcceptsGoalAndRoundAdmission(t *testing.T) {
	ref := GoalRef{ID: "goal-1", Revision: 1}
	state, err := FoldWork([]WorkEvent{
		goalEvent(1, WorkEventGoalCreated, ref, "ship it", 2),
		{
			SessionID:      "session-1",
			Seq:            2,
			Kind:           WorkEventGoalRoundAdmitted,
			PayloadVersion: WorkPayloadVersion,
			Admission: GoalRunAdmission{
				SessionID: "session-1",
				Goal:      ref,
				Round:     1,
				RunID:     "run-1",
			},
		},
	})
	if err != nil {
		t.Fatalf("FoldWork() error = %v", err)
	}
	if state.Version != WorkVersion(2) {
		t.Fatalf("state.Version = %d, want 2", state.Version)
	}
	if state.Goal == nil || state.Goal.RoundsStarted != 1 {
		t.Fatalf("state.Goal.RoundsStarted = %#v, want 1", state.Goal)
	}
}

func TestFoldWorkRejectsNonContiguousSequence(t *testing.T) {
	_, err := FoldWork([]WorkEvent{
		goalEvent(2, WorkEventGoalCreated, GoalRef{ID: "goal-1", Revision: 1}, "ship it", 1),
	})
	if !errors.Is(err, ErrNonContiguousWorkSeq) {
		t.Fatalf("FoldWork() error = %v, want ErrNonContiguousWorkSeq", err)
	}
}

func TestFoldWorkRejectsUnsupportedPayloadVersion(t *testing.T) {
	event := goalEvent(1, WorkEventGoalCreated, GoalRef{ID: "goal-1", Revision: 1}, "ship it", 1)
	event.PayloadVersion = WorkPayloadVersion + 1

	_, err := FoldWork([]WorkEvent{event})
	if !errors.Is(err, ErrUnsupportedWorkPayloadVersion) {
		t.Fatalf("FoldWork() error = %v, want ErrUnsupportedWorkPayloadVersion", err)
	}
}

func TestFoldWorkRejectsStaleGoalReference(t *testing.T) {
	ref := GoalRef{ID: "goal-1", Revision: 1}
	_, err := FoldWork([]WorkEvent{
		goalEvent(1, WorkEventGoalCreated, ref, "ship it", 1),
		goalEvent(2, WorkEventGoalPaused, GoalRef{ID: ref.ID, Revision: 2}, "", 0),
	})
	if !errors.Is(err, ErrStaleGoalReference) {
		t.Fatalf("FoldWork() error = %v, want ErrStaleGoalReference", err)
	}
}

func TestFoldWorkEditsExactCurrentGoalRevision(t *testing.T) {
	ref := GoalRef{ID: "goal-1", Revision: 1}
	state, err := FoldWork([]WorkEvent{
		goalEvent(1, WorkEventGoalCreated, ref, "first objective", 2),
		admissionEvent(2, ref, 1),
		goalEvent(3, WorkEventGoalEdited, ref, "revised objective", 3),
	})
	if err != nil {
		t.Fatalf("FoldWork valid edit: %v", err)
	}
	if state.Version != 3 || state.Goal == nil ||
		state.Goal.Ref != (GoalRef{ID: "goal-1", Revision: 2}) ||
		state.Goal.Objective != "revised objective" || state.Goal.RoundsStarted != 1 || state.Goal.MaxRounds != 3 {
		t.Fatalf("FoldWork edited state = %+v, want revision 2 and spent round retained", state)
	}
}

func TestFoldWorkFromCarriesStateWithoutMutatingCursor(t *testing.T) {
	ref := GoalRef{ID: "goal-1", Revision: 1}
	cursor, err := FoldWork([]WorkEvent{goalEvent(1, WorkEventGoalCreated, ref, "ship it", 2)})
	if err != nil {
		t.Fatalf("FoldWork create: %v", err)
	}
	next, err := FoldWorkFrom(cursor, []WorkEvent{goalEvent(2, WorkEventGoalPaused, ref, "", 0)})
	if err != nil {
		t.Fatalf("FoldWorkFrom pause: %v", err)
	}
	if cursor.Version != 1 || cursor.Goal == nil || cursor.Goal.Phase != WorkPhaseActive {
		t.Fatalf("input cursor mutated: %+v", cursor)
	}
	if next.Version != 2 || next.Goal == nil || next.Goal.Phase != WorkPhasePaused {
		t.Fatalf("next state = %+v, want paused at version 2", next)
	}
}

func TestFoldWorkRejectsRoundCapOverflow(t *testing.T) {
	ref := GoalRef{ID: "goal-1", Revision: 1}
	_, err := FoldWork([]WorkEvent{
		goalEvent(1, WorkEventGoalCreated, ref, "ship it", 1),
		admissionEvent(2, ref, 1),
		admissionEvent(3, ref, 2),
	})
	if !errors.Is(err, ErrWorkRoundLimit) {
		t.Fatalf("FoldWork() error = %v, want ErrWorkRoundLimit", err)
	}
}

func TestFoldWorkRejectsRoundAdmissionUnlessGoalActive(t *testing.T) {
	ref := GoalRef{ID: "goal-1", Revision: 1}
	for _, phase := range []struct {
		name string
		kind WorkEventKind
	}{
		{"paused", WorkEventGoalPaused},
		{"blocked", WorkEventGoalBlocked},
		{"completed", WorkEventGoalCompleted},
	} {
		t.Run(phase.name, func(t *testing.T) {
			_, err := FoldWork([]WorkEvent{
				goalEvent(1, WorkEventGoalCreated, ref, "ship it", 2),
				goalEvent(2, phase.kind, ref, "", 0),
				admissionEvent(3, ref, 1),
			})
			if !errors.Is(err, ErrStaleGoalReference) {
				t.Fatalf("FoldWork() error = %v, want inactive Goal rejected", err)
			}
		})
	}
}

func TestFoldWorkRejectsCrossSessionRoundAdmission(t *testing.T) {
	ref := GoalRef{ID: "goal-1", Revision: 1}
	admission := admissionEvent(2, ref, 1)
	admission.Admission.SessionID = "other-session"
	_, err := FoldWork([]WorkEvent{
		goalEvent(1, WorkEventGoalCreated, ref, "ship it", 2),
		admission,
	})
	if !errors.Is(err, ErrStaleGoalReference) {
		t.Fatalf("FoldWork() error = %v, want cross-session admission rejected", err)
	}
}

func TestWorkEventKindIsBounded(t *testing.T) {
	event := goalEvent(1, WorkEventKind("goal.deleted"), GoalRef{ID: "goal-1", Revision: 1}, "ship it", 1)

	_, err := FoldWork([]WorkEvent{event})
	if !errors.Is(err, ErrUnsupportedWorkEventKind) {
		t.Fatalf("FoldWork() error = %v, want ErrUnsupportedWorkEventKind", err)
	}
}

func TestFoldWorkPlanReviewLifecycle(t *testing.T) {
	state, err := FoldWork([]WorkEvent{
		planEvent(1, WorkEventPlanEntered, WorkMutation{SessionID: "session-1"}),
		planEvent(2, WorkEventPlanSubmitted, WorkMutation{
			SessionID: "session-1", PlanSubmissionID: "submission-1", PlanMarkdown: "# plan",
			PlanOriginRunID: "run-1", PlanOriginToolCallID: "tool-1",
		}),
		planEvent(3, WorkEventPlanReviewSuspended, WorkMutation{
			SessionID: "session-1", PlanSubmissionID: "submission-1",
			PlanOriginRunID: "run-1", PlanOriginToolCallID: "tool-1", PlanResumeTarget: "resume-1",
		}),
		planEvent(4, WorkEventPlanDecided, WorkMutation{
			SessionID: "session-1", PlanSubmissionID: "submission-1",
			PlanAction: PlanDecisionRevise, PlanFeedback: "clarify rollback",
		}),
		planEvent(5, WorkEventPlanSubmitted, WorkMutation{
			SessionID: "session-1", PlanSubmissionID: "submission-2", PlanMarkdown: "# revised",
			PlanOriginRunID: "run-2", PlanOriginToolCallID: "tool-2",
		}),
		planEvent(6, WorkEventPlanReviewSuspended, WorkMutation{
			SessionID: "session-1", PlanSubmissionID: "submission-2",
			PlanOriginRunID: "run-2", PlanOriginToolCallID: "tool-2", PlanResumeTarget: "resume-2",
		}),
		planEvent(7, WorkEventPlanDecided, WorkMutation{
			SessionID: "session-1", PlanSubmissionID: "submission-2",
			PlanAction: PlanDecisionExecuteOnce,
		}),
	})
	if err != nil {
		t.Fatalf("FoldWork() error = %v", err)
	}
	if state.Plan.Active || state.Plan.ReviewStatus != PlanReviewAccepted {
		t.Fatalf("plan state = %+v, want inactive accepted plan", state.Plan)
	}
	if state.Plan.SubmissionID != "submission-2" || state.Plan.Markdown != "# revised" {
		t.Fatalf("plan submission = %+v, want latest immutable submission", state.Plan)
	}
}

func TestFoldWorkRejectsOriginlessPlanSubmission(t *testing.T) {
	_, err := FoldWork([]WorkEvent{
		planEvent(1, WorkEventPlanEntered, WorkMutation{SessionID: "session-1"}),
		planEvent(2, WorkEventPlanSubmitted, WorkMutation{
			SessionID: "session-1", PlanSubmissionID: "submission-1", PlanMarkdown: "# no Eino origin",
		}),
	})
	if !errors.Is(err, ErrStaleGoalReference) {
		t.Fatalf("FoldWork originless Plan submission = %v, want stale Plan rejection", err)
	}
}

func TestFoldWorkPlanReviewCancellationLeavesPlanActive(t *testing.T) {
	state, err := FoldWork([]WorkEvent{
		planEvent(1, WorkEventPlanEntered, WorkMutation{SessionID: "session-1"}),
		planEvent(2, WorkEventPlanSubmitted, WorkMutation{
			SessionID: "session-1", PlanSubmissionID: "submission-1", PlanMarkdown: "# plan",
			PlanOriginRunID: "run-1", PlanOriginToolCallID: "tool-1",
		}),
		planEvent(3, WorkEventPlanReviewSuspended, WorkMutation{
			SessionID: "session-1", PlanSubmissionID: "submission-1",
			PlanOriginRunID: "run-1", PlanOriginToolCallID: "tool-1", PlanResumeTarget: "resume-1",
		}),
		planEvent(4, WorkEventPlanReviewCancelled, WorkMutation{
			SessionID: "session-1", PlanSubmissionID: "submission-1", PlanOriginRunID: "run-1",
		}),
	})
	if err != nil {
		t.Fatalf("FoldWork() error = %v", err)
	}
	if !state.Plan.Active || state.Plan.ReviewStatus != PlanReviewCancelled {
		t.Fatalf("plan state = %+v, want active Plan with cancelled review", state.Plan)
	}
}

func TestFoldWorkRejectsPlanWhileGoalActive(t *testing.T) {
	ref := GoalRef{ID: "goal-1", Revision: 1}
	_, err := FoldWork([]WorkEvent{
		goalEvent(1, WorkEventGoalCreated, ref, "ship it", 1),
		planEvent(2, WorkEventPlanEntered, WorkMutation{SessionID: "session-1"}),
	})
	if !errors.Is(err, ErrStaleGoalReference) {
		t.Fatalf("FoldWork() error = %v, want ErrStaleGoalReference", err)
	}
}

func planEvent(seq WorkSeq, kind WorkEventKind, mutation WorkMutation) WorkEvent {
	return WorkEvent{
		SessionID: "session-1", Seq: seq, Kind: kind, PayloadVersion: WorkPayloadVersion,
		Mutation: mutation,
	}
}

func goalEvent(seq WorkSeq, kind WorkEventKind, ref GoalRef, objective string, maxRounds int) WorkEvent {
	return WorkEvent{
		SessionID:      "session-1",
		Seq:            seq,
		Kind:           kind,
		PayloadVersion: WorkPayloadVersion,
		Mutation: WorkMutation{
			SessionID: "session-1",
			Goal:      ref,
			Objective: objective,
			MaxRounds: maxRounds,
		},
	}
}

func admissionEvent(seq WorkSeq, ref GoalRef, round int) WorkEvent {
	return WorkEvent{
		SessionID:      "session-1",
		Seq:            seq,
		Kind:           WorkEventGoalRoundAdmitted,
		PayloadVersion: WorkPayloadVersion,
		Admission: GoalRunAdmission{
			SessionID: "session-1",
			Goal:      ref,
			Round:     round,
			RunID:     RunID("run-1"),
		},
	}
}
