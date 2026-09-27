package runtime

import (
	"context"
	"errors"
	"sync"
	"testing"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
	"agent-vivy/internal/testsupport"
)

func TestBudgetLedgerChildCannotBypassParent(t *testing.T) {
	parent, err := NewBudgetLedger(BudgetPolicy{MaxToolCalls: 2})
	if err != nil {
		t.Fatalf("new parent: %v", err)
	}
	child, err := parent.Child(BudgetPolicy{MaxToolCalls: 99})
	if err != nil {
		t.Fatalf("new child: %v", err)
	}
	if err := child.ReserveToolCall(); err != nil {
		t.Fatalf("child first reservation: %v", err)
	}
	if err := parent.ReserveToolCall(); err != nil {
		t.Fatalf("parent reservation: %v", err)
	}
	if err := child.ReserveToolCall(); !errors.Is(err, ErrBudgetExceeded) {
		t.Fatalf("third reservation error = %v, want budget exceeded", err)
	}
	if got := parent.Snapshot().Usage.ToolCalls; got != 2 {
		t.Fatalf("shared tool usage = %d, want 2", got)
	}
}

func TestBudgetLedgerChildKeepsLocalLimit(t *testing.T) {
	parent, err := NewBudgetLedger(BudgetPolicy{MaxRetries: 8})
	if err != nil {
		t.Fatalf("new parent: %v", err)
	}
	child, err := parent.Child(BudgetPolicy{MaxRetries: 1})
	if err != nil {
		t.Fatalf("new child: %v", err)
	}
	if err := child.ReserveRetry(); err != nil {
		t.Fatalf("first retry reservation: %v", err)
	}
	if err := child.ReserveRetry(); !errors.Is(err, ErrBudgetExceeded) {
		t.Fatalf("second retry error = %v, want budget exceeded", err)
	}
	grandchild, err := child.Child(BudgetPolicy{MaxRetries: 99})
	if err != nil {
		t.Fatalf("new grandchild: %v", err)
	}
	if err := grandchild.ReserveRetry(); !errors.Is(err, ErrBudgetExceeded) {
		t.Fatalf("grandchild bypass error = %v, want budget exceeded", err)
	}
	if got := parent.Snapshot().Usage.Retries; got != 1 {
		t.Fatalf("shared retry usage = %d, want 1", got)
	}
}

func TestBudgetLedgerConcurrentReservationsAreAtomic(t *testing.T) {
	ledger, err := NewBudgetLedger(BudgetPolicy{MaxToolCalls: 7})
	if err != nil {
		t.Fatalf("new ledger: %v", err)
	}
	var wg sync.WaitGroup
	var mu sync.Mutex
	succeeded := 0
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := ledger.ReserveToolCall(); err == nil {
				mu.Lock()
				succeeded++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if succeeded != 7 || ledger.Snapshot().Usage.ToolCalls != 7 {
		t.Fatalf("successful reservations = %d, usage = %d; want 7/7", succeeded, ledger.Snapshot().Usage.ToolCalls)
	}
}

func TestBudgetLedgerReplayPreservesCircuitState(t *testing.T) {
	ledger, err := NewBudgetLedger(BudgetPolicy{MaxEvents: 2})
	if err != nil {
		t.Fatalf("new ledger: %v", err)
	}
	if err := ledger.ReplayEvent(domain.RunEvent{Type: domain.EventRunStarted}); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 600; i++ {
		if err := ledger.ReplayEvent(domain.RunEvent{Type: domain.EventModelDelta}); err != nil {
			t.Fatalf("delta %d consumed replay event budget: %v", i, err)
		}
	}
	if err := ledger.ReplayEvent(domain.RunEvent{Type: domain.EventToolRequested}); err != nil {
		t.Fatalf("second semantic event: %v", err)
	}
	if err := ledger.ReplayEvent(domain.RunEvent{Type: domain.EventModelUsage}); !errors.Is(err, ErrBudgetExceeded) {
		t.Fatalf("third semantic event error = %v, want budget exceeded", err)
	}
}

func TestRecoveredSiblingRunsShareOneBudgetAccount(t *testing.T) {
	ctx := context.Background()
	svc, backend, _ := newTestService(t, testsupport.NewEchoModel())
	svc.deps.Budget = BudgetPolicy{MaxModelCalls: 2}
	const sessionID = domain.SessionID("session-recovered-budget-siblings")
	const rootID = domain.RunID("run-recovered-budget-root")
	if err := backend.CreateSession(ctx, domain.Session{ID: sessionID, Title: "budget", CreatedAt: 1}); err != nil {
		t.Fatal(err)
	}
	if err := backend.CreateRun(ctx, domain.Run{ID: rootID, SessionID: sessionID, Status: domain.RunActive, CreatedAt: 2, RootID: rootID}); err != nil {
		t.Fatal(err)
	}
	for i, id := range []domain.RunID{"run-recovered-budget-child-a", "run-recovered-budget-child-b"} {
		if err := backend.CreateRun(ctx, domain.Run{
			ID: id, SessionID: sessionID, Status: domain.RunActive, CreatedAt: int64(3 + i),
			Kind: domain.RunKindChild, ChildMode: domain.ChildModeOneShot,
			ParentID: rootID, RootID: rootID, Depth: 1,
		}); err != nil {
			t.Fatal(err)
		}
	}
	first := svc.recoverBudgetLedger(ctx, "run-recovered-budget-child-a")
	second := svc.recoverBudgetLedger(ctx, "run-recovered-budget-child-b")
	if first == nil || second == nil {
		t.Fatal("recovered sibling budget ledger is nil")
	}
	if err := first.ReserveModelCall(); err != nil {
		t.Fatalf("first sibling reservation: %v", err)
	}
	if err := second.ReserveModelCall(); err != nil {
		t.Fatalf("second sibling reservation: %v", err)
	}
	if err := first.ReserveModelCall(); !errors.Is(err, ErrBudgetExceeded) {
		t.Fatalf("third combined reservation = %v, want shared run-tree budget exhaustion", err)
	}
}

// R13: a restarted run resumes against the usage its journal already charged.
// Recovery replays durable events exactly once, a repeated recovery reuses the
// same ledger without recharging, and post-restart reservations consume only
// the remaining shared headroom.
func TestRecoveredLedgerReplaysJournalOnceAndCapsResumedRun(t *testing.T) {
	ctx := context.Background()
	svc, backend, _ := newTestService(t, testsupport.NewEchoModel())
	svc.deps.Budget = BudgetPolicy{MaxModelCalls: 3}
	const sessionID = domain.SessionID("session-recovered-budget-replay")
	const rootID = domain.RunID("run-recovered-replay-root")
	const childID = domain.RunID("run-recovered-replay-child")
	if err := backend.CreateSession(ctx, domain.Session{ID: sessionID, Title: "budget", CreatedAt: 1}); err != nil {
		t.Fatal(err)
	}
	if err := backend.CreateRun(ctx, domain.Run{ID: rootID, SessionID: sessionID, Status: domain.RunActive, CreatedAt: 2, RootID: rootID}); err != nil {
		t.Fatal(err)
	}
	if err := backend.CreateRun(ctx, domain.Run{
		ID: childID, SessionID: sessionID, Status: domain.RunActive, CreatedAt: 3,
		Kind: domain.RunKindChild, ChildMode: domain.ChildModeOneShot,
		ParentID: rootID, RootID: rootID, Depth: 1,
	}); err != nil {
		t.Fatal(err)
	}
	// Durable history before the restart: one tool.requested (model + tool
	// call) and one model.completed (model call) on the child.
	if _, err := backend.Append(ctx, storage.Commit{RunID: childID, Events: []domain.RunEvent{
		{RunID: childID, Type: domain.EventToolRequested, CreatedAt: 4, Payload: []byte(`{}`)},
		{RunID: childID, Type: domain.EventModelCompleted, CreatedAt: 5, Payload: []byte(`{}`)},
	}}); err != nil {
		t.Fatal(err)
	}

	ledger := svc.recoverBudgetLedger(ctx, childID)
	if ledger == nil {
		t.Fatal("recovered ledger is nil")
	}
	snap := ledger.Snapshot()
	if snap.Usage.ModelCalls != 2 || snap.Usage.ToolCalls != 1 {
		t.Fatalf("replayed usage = %+v, want 2 model calls and 1 tool call", snap.Usage)
	}
	// Reauthorization path reuses the recovered ledger; usage must not double.
	if again := svc.recoverBudgetLedger(ctx, childID); again != ledger {
		t.Fatal("second recovery rebuilt the ledger instead of reusing it")
	}
	if snap = ledger.Snapshot(); snap.Usage.ModelCalls != 2 {
		t.Fatalf("usage after second recovery = %+v, replay double counted", snap.Usage)
	}
	// One model call of shared headroom remains; the next is refused.
	if err := ledger.ReserveModelCall(); err != nil {
		t.Fatalf("reservation within remaining headroom: %v", err)
	}
	if err := ledger.ReserveModelCall(); !errors.Is(err, ErrBudgetExceeded) {
		t.Fatalf("reservation past replayed headroom = %v, want budget exceeded", err)
	}
}
