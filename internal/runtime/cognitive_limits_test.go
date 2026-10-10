package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/ProjectViVy/inofy"
	laputaevolution "github.com/ProjectViVy/laputa/laputa/evolution"
	laputainofy "github.com/ProjectViVy/laputa/laputa/evolution/inofy"
)

type stageBudgetModel struct{}

func (stageBudgetModel) Infer(_ context.Context, req laputaevolution.ModelRequest) (laputaevolution.ModelReply, error) {
	body := `{"base_revision":0,"changes":[]}`
	if req.Stage == laputaevolution.StageReflect {
		body = `{"candidates":[{"kind":"reflection_note","reflection_note":{"body":"observed","sources":[]}}]}`
	}
	return laputaevolution.ModelReply{OutputJSON: json.RawMessage(body)}, nil
}

// Each packet is bounded, but the six-stage strategy legitimately carries
// the same evidence through several outputs. A single-packet total ceiling
// must not reject the graph after its effect has already committed.
func TestCognitiveStageOutputBudget(t *testing.T) {
	ctx := context.Background()
	a, err := trustedStrategyAdmission(ctx, TrustedStrategyDIVA)
	if err != nil {
		t.Fatal(err)
	}
	// Three pre-reflection packets still exceed the older cumulative
	// ceiling after the strategy drops its now-unused evidence batch.
	d := &fakeCognitiveDomain{batch: laputaevolution.EvidenceBatch{Entries: []laputaevolution.Entry{{ID: "e", Body: strings.Repeat("x", 2200)}}}}
	result, err := a.Program.Run(ctx, inofy.RunRequest{Ref: inofy.ExecutionRef{RunID: "budget-proof", Epoch: 1, ProgramDigest: a.Meta.ProgramDigest, HostBindingID: "test-bound"}, Input: cognitiveInput(t), Limits: cognitiveWorkflowLimits()}, inofy.Bindings{Nodes: laputainofy.NewExecutor(d, stageBudgetModel{}), Runs: inofy.NewMemoryRunStore()})
	if err != nil || result.Status != inofy.RunSucceeded {
		t.Fatalf("bounded strategy failed after %d effects: status=%s error=%v", len(d.applied), result.Status, err)
	}
	if len(d.applied) != 1 {
		t.Fatalf("effect count=%d", len(d.applied))
	}
	// An older admitted ceiling remains authoritative; widening the
	// compiled ceiling must not silently upgrade its persisted run policy.
	old := &fakeCognitiveDomain{batch: d.batch}
	_, err = a.Program.Run(ctx, inofy.RunRequest{Ref: inofy.ExecutionRef{RunID: "old-budget-proof", Epoch: 1, ProgramDigest: a.Meta.ProgramDigest, HostBindingID: "old-bound"}, Input: cognitiveInput(t), Limits: inofyWorkflowLimits()}, inofy.Bindings{Nodes: laputainofy.NewExecutor(old, stageBudgetModel{}), Runs: inofy.NewMemoryRunStore()})
	var budgetErr *inofy.Error
	if !errors.As(err, &budgetErr) || budgetErr.Code != inofy.ErrBudgetExceeded {
		t.Fatalf("old persisted ceiling bypassed: %v", err)
	}
}
