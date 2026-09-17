package domain

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

func TestWorkflowPlanCarriesOnlyDomainData(t *testing.T) {
	plan := WorkflowPlan{
		DefinitionID: "demo",
		Rev:          2,
		Hash:         "hash",
		Nodes:        []PlanNode{{ID: "input", Kind: WorkflowNodeIO, Spec: json.RawMessage(`{"template":"hello"}`), TimeoutMS: 1000}},
		Edges:        []Edge{},
		Outputs:      []OutputBinding{{Name: "answer", Template: "${{ nodes.input.output }}"}},
		Budget:       BudgetEnvelope{MaxNodes: 32, Parallelism: 4, MaxNodeOutputBytes: 65536, DefinitionMaxBytes: 262144},
	}
	if plan.Nodes[0].Kind != WorkflowNodeIO || plan.Budget.Parallelism != 4 {
		t.Fatalf("unexpected plan: %+v", plan)
	}
}

func TestUnavailableWorkflowExecutorPreservesContract(t *testing.T) {
	var executor PlanExecutor = unavailableExecutorForTest{}
	_, err := executor.ExecutePlan(context.Background(), "run-1", WorkflowPlan{}, nil)
	if err == nil {
		t.Fatal("unavailable executor returned nil error")
	}
}

// The real unavailable executor lives in internal/workflowhost. This local
// implementation keeps the domain package test independent of that adapter.
type unavailableExecutorForTest struct{}

func (unavailableExecutorForTest) ExecutePlan(context.Context, RunID, WorkflowPlan, PlanEventSink) (PlanResult, error) {
	return PlanResult{}, errors.New("workflow execution unavailable")
}
