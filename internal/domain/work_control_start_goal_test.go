package domain

import "testing"

func TestFoldWorkPlanStartGoalAtomicallyCreatesGoal(t *testing.T) {
	state, err := FoldWork([]WorkEvent{
		planEvent(1, WorkEventPlanEntered, WorkMutation{SessionID: "session-1"}),
		planEvent(2, WorkEventPlanSubmitted, WorkMutation{
			SessionID: "session-1", PlanSubmissionID: "submission-1", PlanMarkdown: "# bounded work",
		}),
		{
			SessionID: "session-1", Seq: 3, Kind: WorkEventPlanDecided,
			PayloadVersion: WorkPayloadVersion,
			Mutation: WorkMutation{
				SessionID: "session-1", PlanSubmissionID: "submission-1",
				PlanAction: PlanDecisionStartGoal, Goal: GoalRef{ID: "goal-1", Revision: 1},
				Objective: "finish the bounded task", MaxRounds: 3,
			},
		},
	})
	if err != nil {
		t.Fatalf("FoldWork() error = %v", err)
	}
	if state.Plan.Active || state.Plan.ReviewStatus != PlanReviewAccepted {
		t.Fatalf("plan state = %+v, want inactive accepted Plan", state.Plan)
	}
	if state.Goal == nil || state.Goal.Ref.ID != "goal-1" || state.Goal.Objective != "finish the bounded task" ||
		state.Goal.Phase != WorkPhaseActive || state.Goal.MaxRounds != 3 {
		t.Fatalf("goal state = %+v, want atomically created active Goal", state.Goal)
	}
}

func TestFoldWorkGoalTerminalOutcomeCarriesReasonAndEvidence(t *testing.T) {
	ref := GoalRef{ID: "goal-1", Revision: 1}
	state, err := FoldWork([]WorkEvent{
		goalEvent(1, WorkEventGoalCreated, ref, "ship it", 1),
		{
			SessionID: "session-1", Seq: 2, Kind: WorkEventGoalBlocked,
			PayloadVersion: WorkPayloadVersion,
			Mutation: WorkMutation{
				SessionID: "session-1", Goal: ref,
				Reason: "budget exhausted", EvidenceRunID: "run-1",
			},
		},
	})
	if err != nil {
		t.Fatalf("FoldWork() error = %v", err)
	}
	if state.Goal == nil || state.Goal.Phase != WorkPhaseBlocked ||
		state.Goal.Reason != "budget exhausted" || state.Goal.EvidenceRunID != "run-1" {
		t.Fatalf("goal outcome = %+v, want reason and evidence", state.Goal)
	}
}
