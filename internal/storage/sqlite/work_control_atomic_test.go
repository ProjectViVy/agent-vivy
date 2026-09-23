package sqlite

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
)

func TestWorkRequestIDRejectsChangedPayloadWithSameCallerHash(t *testing.T) {
	b := openBackend(t)
	ctx := context.Background()
	const sessionID domain.SessionID = "work-payload-conflict"
	if err := b.CreateSession(ctx, domain.Session{ID: sessionID, CreatedAt: 1}); err != nil {
		t.Fatal(err)
	}
	create := domain.WorkMutation{
		SessionID: sessionID, RequestID: "create", RequestHash: "caller-hash",
		Kind: domain.WorkEventGoalCreated, Goal: domain.GoalRef{ID: "goal", Revision: 1},
		Objective: "original objective", MaxRounds: 2,
	}
	if _, err := b.CommitWork(ctx, create); err != nil {
		t.Fatal(err)
	}
	changed := create
	changed.Objective = "different objective"
	if _, err := b.CommitWork(ctx, changed); !errors.Is(err, storage.ErrWorkRequestConflict) {
		t.Fatalf("same request/hash with changed objective = %v, want request conflict", err)
	}
	state, err := b.ReadWork(ctx, sessionID)
	if err != nil || state.Version != 1 || state.Goal == nil || state.Goal.Objective != create.Objective {
		t.Fatalf("committed state = %+v, %v; want original objective only", state, err)
	}
}

func TestGoalRunRetryRejectsChangedMessageWithSameRequest(t *testing.T) {
	b, sessionID, admission := goalAdmissionFixture(t)
	ctx := context.Background()
	first, err := b.CommitGoalRun(ctx, admission)
	if err != nil {
		t.Fatal(err)
	}
	changed := admission
	changed.Message.Content = "different instruction"
	if _, err := b.CommitGoalRun(ctx, changed); !errors.Is(err, storage.ErrWorkRequestConflict) {
		t.Fatalf("changed message on admitted request = %v, want request conflict", err)
	}
	state, err := b.ReadWork(ctx, sessionID)
	if err != nil || state.Version != 2 || state.Goal == nil || state.Goal.RoundsStarted != 1 {
		t.Fatalf("work state after rejected retry = %+v, %v", state, err)
	}
	messages, err := b.ListMessages(ctx, sessionID)
	if err != nil || len(messages) != 1 || messages[0].Content != admission.Message.Content {
		t.Fatalf("messages after rejected retry = %+v, %v", messages, err)
	}
	if first.Run.ID != admission.Run.ID {
		t.Fatalf("first run = %q, want %q", first.Run.ID, admission.Run.ID)
	}
}

func goalAdmissionFixture(t *testing.T) (*Backend, domain.SessionID, storage.GoalRunCommit) {
	t.Helper()
	b := openBackend(t)
	ctx := context.Background()
	const sessionID domain.SessionID = "work-goal-atomic"
	const runID domain.RunID = "run-goal-atomic"
	ref := domain.GoalRef{ID: "goal", Revision: 1}
	if err := b.CreateSession(ctx, domain.Session{ID: sessionID, CreatedAt: 1}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.CommitWork(ctx, domain.WorkMutation{
		SessionID: sessionID, RequestID: "create", RequestHash: "hash-create",
		Kind: domain.WorkEventGoalCreated, Goal: ref, Objective: "ship", MaxRounds: 2,
	}); err != nil {
		t.Fatal(err)
	}
	return b, sessionID, storage.GoalRunCommit{
		Mutation: domain.WorkMutation{
			SessionID: sessionID, ExpectedVersion: 1, RequestID: "round-one", RequestHash: "hash-round-one",
			Kind:      domain.WorkEventGoalRoundAdmitted,
			Admission: domain.GoalRunAdmission{SessionID: sessionID, Goal: ref, Round: 1, RunID: runID},
		},
		Message: domain.Message{ID: "message-goal", SessionID: sessionID, RunID: runID, Role: domain.RoleUser, CreatedAt: 2, Content: "continue"},
		Run:     domain.Run{ID: runID, SessionID: sessionID, Kind: domain.RunKindPrimary, Status: domain.RunActive, CreatedAt: 2},
		Started: domain.RunEvent{RunID: runID, Type: domain.EventRunStarted, CreatedAt: 2, PayloadVersion: 1, Payload: []byte(`{}`)},
	}
}

func TestGoalRunTransactionRollsBackAtEachStartupBoundary(t *testing.T) {
	for _, tc := range []struct {
		name    string
		trigger string
	}{
		{"after message", `CREATE TRIGGER fail_goal_run BEFORE INSERT ON runs BEGIN SELECT RAISE(FAIL, 'injected run insert failure'); END`},
		{"after run.started", `CREATE TRIGGER fail_goal_work BEFORE INSERT ON session_work_events WHEN NEW.kind = 'goal.round_admitted' BEGIN SELECT RAISE(FAIL, 'injected work insert failure'); END`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b, sessionID, admission := goalAdmissionFixture(t)
			ctx := context.Background()
			if _, err := b.db.ExecContext(ctx, tc.trigger); err != nil {
				t.Fatal(err)
			}
			if _, err := b.CommitGoalRun(ctx, admission); err == nil || !strings.Contains(err.Error(), "injected") {
				t.Fatalf("injected admission failure = %v", err)
			}
			state, err := b.ReadWork(ctx, sessionID)
			if err != nil || state.Version != 1 || state.Goal == nil || state.Goal.RoundsStarted != 0 {
				t.Fatalf("work state after rollback = %+v, %v", state, err)
			}
			messages, err := b.ListMessages(ctx, sessionID)
			if err != nil || len(messages) != 0 {
				t.Fatalf("messages after rollback = %+v, %v", messages, err)
			}
			runs, err := b.ListRunsBySession(ctx, sessionID)
			if err != nil || len(runs) != 0 {
				t.Fatalf("runs after rollback = %+v, %v", runs, err)
			}
			var count int
			if err := b.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM run_events WHERE run_id = ?", admission.Run.ID).Scan(&count); err != nil || count != 0 {
				t.Fatalf("run.started rows after rollback = %d, %v", count, err)
			}
		})
	}
}

func TestConcurrentGoalAdmissionsAtOneVersionHaveOneWinner(t *testing.T) {
	b, sessionID, first := goalAdmissionFixture(t)
	second := first
	second.Mutation.RequestID = "round-other"
	second.Mutation.RequestHash = "hash-round-other"
	second.Mutation.Admission.RunID = "run-goal-other"
	second.Message.ID = "message-other"
	second.Message.RunID = second.Mutation.Admission.RunID
	second.Run.ID = second.Mutation.Admission.RunID
	second.Started.RunID = second.Mutation.Admission.RunID
	admissions := []storage.GoalRunCommit{first, second}
	results := make(chan error, 2)
	var wg sync.WaitGroup
	for _, admission := range admissions {
		wg.Add(1)
		go func(in storage.GoalRunCommit) {
			defer wg.Done()
			_, err := b.CommitGoalRun(context.Background(), in)
			results <- err
		}(admission)
	}
	wg.Wait()
	close(results)
	successes := 0
	for err := range results {
		if err == nil {
			successes++
		} else if !errors.Is(err, storage.ErrWorkVersionConflict) {
			t.Fatalf("losing admission = %v, want stale version", err)
		}
	}
	if successes != 1 {
		t.Fatalf("successful admissions = %d, want one", successes)
	}
	state, err := b.ReadWork(context.Background(), sessionID)
	if err != nil || state.Version != 2 || state.Goal == nil || state.Goal.RoundsStarted != 1 {
		t.Fatalf("work state = %+v, %v", state, err)
	}
	runs, err := b.ListRunsBySession(context.Background(), sessionID)
	if err != nil || len(runs) != 1 {
		t.Fatalf("runs = %+v, %v", runs, err)
	}
}

func TestCancelledOriginRunCannotApprovePlanIntoGoal(t *testing.T) {
	b := openBackend(t)
	ctx := context.Background()
	const sessionID domain.SessionID = "work-cancelled-review"
	const runID domain.RunID = "run-plan-origin"
	if err := b.CreateSession(ctx, domain.Session{ID: sessionID, CreatedAt: 1}); err != nil {
		t.Fatal(err)
	}
	if err := b.CreateRun(ctx, domain.Run{ID: runID, SessionID: sessionID, Status: domain.RunActive, Kind: domain.RunKindPrimary, CreatedAt: 1}); err != nil {
		t.Fatal(err)
	}
	commit := func(version domain.WorkVersion, id string, kind domain.WorkEventKind, fill func(*domain.WorkMutation)) {
		t.Helper()
		mutation := domain.WorkMutation{SessionID: sessionID, ExpectedVersion: version, RequestID: id, RequestHash: id, Kind: kind}
		if fill != nil {
			fill(&mutation)
		}
		if _, err := b.CommitWork(ctx, mutation); err != nil {
			t.Fatalf("%s: %v", kind, err)
		}
	}
	commit(0, "enter", domain.WorkEventPlanEntered, nil)
	commit(1, "submit", domain.WorkEventPlanSubmitted, func(m *domain.WorkMutation) {
		m.PlanSubmissionID, m.PlanMarkdown = "submission", "# plan"
		m.PlanOriginRunID, m.PlanOriginToolCallID = runID, "tool-call"
	})
	commit(2, "suspend", domain.WorkEventPlanReviewSuspended, func(m *domain.WorkMutation) {
		m.PlanSubmissionID, m.PlanOriginRunID = "submission", runID
		m.PlanOriginToolCallID, m.PlanResumeTarget = "tool-call", "resume-target"
	})
	if err := b.SetRunStatus(ctx, runID, domain.RunCancelled); err != nil {
		t.Fatal(err)
	}
	_, err := b.CommitWork(ctx, domain.WorkMutation{
		SessionID: sessionID, ExpectedVersion: 3, RequestID: "approve", RequestHash: "approve",
		Kind: domain.WorkEventPlanDecided, PlanSubmissionID: "submission", PlanAction: domain.PlanDecisionStartGoal,
		Goal: domain.GoalRef{ID: "new-goal", Revision: 1}, Objective: "execute", MaxRounds: 1,
	})
	if !errors.Is(err, storage.ErrWorkRunConflict) {
		t.Fatalf("decision after origin run cancellation = %v, want run conflict", err)
	}
	state, err := b.ReadWork(ctx, sessionID)
	if err != nil || state.Version != 3 || state.Goal != nil || state.Plan.ReviewStatus != domain.PlanReviewPending {
		t.Fatalf("state after rejected decision = %+v, %v", state, err)
	}
}
