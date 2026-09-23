package runtime

import (
	"context"
	"path/filepath"
	"testing"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage/sqlite"
)

func TestRecoveredGoalRefUsesFoldedWorkReplay(t *testing.T) {
	ctx := context.Background()
	backend, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "goal-recovery.db"))
	if err != nil {
		t.Fatalf("open SQLite: %v", err)
	}
	t.Cleanup(func() { _ = backend.Close() })
	const sessionID domain.SessionID = "sess-goal-recovery"
	if err := backend.CreateSession(ctx, domain.Session{ID: sessionID, CreatedAt: 1}); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	ref := domain.GoalRef{ID: "goal-1", Revision: 1}
	if _, err := backend.CommitWork(ctx, domain.WorkMutation{
		SessionID: sessionID, RequestID: "create", RequestHash: "hash-create",
		Kind: domain.WorkEventGoalCreated, Goal: ref, Objective: "ship it", MaxRounds: 2,
	}); err != nil {
		t.Fatalf("CommitWork create: %v", err)
	}
	if _, err := backend.CommitWork(ctx, domain.WorkMutation{
		SessionID: sessionID, ExpectedVersion: 1, RequestID: "admit", RequestHash: "hash-admit",
		Kind:      domain.WorkEventGoalRoundAdmitted,
		Admission: domain.GoalRunAdmission{SessionID: sessionID, Goal: ref, Round: 1, RunID: "run-1"},
	}); err != nil {
		t.Fatalf("CommitWork admit: %v", err)
	}
	svc := &Service{deps: ServiceDeps{Work: backend}}
	got, ok := svc.recoveredGoalRef(ctx, domain.Run{ID: "run-1", SessionID: sessionID})
	if !ok || got != ref {
		t.Fatalf("recoveredGoalRef = %+v, %t; want %+v", got, ok, ref)
	}
}
