package runtime

import (
	"errors"
	"sync"
	"testing"

	"agent-vivy/internal/domain"
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
