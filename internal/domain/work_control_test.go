package domain

import (
	"errors"
	"testing"
)

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

func TestWorkEventKindIsBounded(t *testing.T) {
	event := goalEvent(1, WorkEventKind("goal.deleted"), GoalRef{ID: "goal-1", Revision: 1}, "ship it", 1)

	_, err := FoldWork([]WorkEvent{event})
	if !errors.Is(err, ErrUnsupportedWorkEventKind) {
		t.Fatalf("FoldWork() error = %v, want ErrUnsupportedWorkEventKind", err)
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
