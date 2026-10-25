package runtime

import (
	"context"
	"errors"
	"strings"
	"testing"

	"agent-vivy/internal/domain"
)

// A child activation feed is bounded by the run's own context budget, the same
// number the primary run uses. A child-specific ceiling would silently give
// delegated work less room than the agent that started it.
func TestChildActivationUsesTheEngineContextBudget(t *testing.T) {
	svc := &Service{}
	task := strings.Repeat("x", 300<<10) // above the retired 128 KiB child cap

	roomy := &Engine{cfg: EngineConfig{MaxContextBytes: 1 << 20}}
	if _, _, _, _, err := svc.runMessagesForChild(context.Background(), task, nil, roomy); err != nil {
		t.Fatalf("child activation refused under a 1 MiB budget: %v", err)
	}

	tight := &Engine{cfg: EngineConfig{MaxContextBytes: 256 << 10}}
	_, _, _, _, err := svc.runMessagesForChild(context.Background(), task, nil, tight)
	if !errors.Is(err, ErrContextBudgetExceeded) {
		t.Fatalf("child activation error = %v, want context budget exceeded", err)
	}
}

// Pending mailbox messages ride the same budget: included while they fit,
// dropped from the activation (not fatal) once they do not.
func TestChildActivationMailboxRidesTheSameBudget(t *testing.T) {
	svc := &Service{}
	eng := &Engine{cfg: EngineConfig{MaxContextBytes: 512 << 10}}
	pending := []domain.ChildMailboxMessage{
		{ID: "msg-1", Body: []byte(strings.Repeat("a", 100<<10))},
		{ID: "msg-2", Body: []byte(strings.Repeat("b", 700<<10))},
	}
	_, _, _, included, err := svc.runMessagesForChild(context.Background(), "task", pending, eng)
	if err != nil {
		t.Fatalf("child activation: %v", err)
	}
	if len(included) != 1 || included[0].ID != "msg-1" {
		t.Fatalf("included messages = %+v, want only msg-1", included)
	}
}
