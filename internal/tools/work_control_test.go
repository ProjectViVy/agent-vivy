package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"agent-vivy/internal/domain"
)

type fakeWorkControlOperations struct {
	state domain.WorkState
	goal  *domain.GoalState
}

func (f *fakeWorkControlOperations) EnterPlanMode(context.Context) (domain.WorkState, error) {
	return f.state, nil
}
func (f *fakeWorkControlOperations) SubmitPlan(context.Context, string) (domain.WorkState, error) {
	return f.state, nil
}
func (f *fakeWorkControlOperations) GetGoal(context.Context) (*domain.GoalState, error) {
	return f.goal, nil
}
func (f *fakeWorkControlOperations) CreateGoal(context.Context, string, int) (domain.WorkState, error) {
	return f.state, nil
}
func (f *fakeWorkControlOperations) ReportGoal(context.Context, string, int64, string, string) (domain.WorkState, error) {
	return f.state, nil
}

func TestWorkControlToolsExposeExactReadonlyContract(t *testing.T) {
	cases := []struct {
		name     string
		tool     Tool
		readonly bool
	}{
		{name: EnterPlanModeName, tool: NewEnterPlanMode(), readonly: false},
		{name: SubmitPlanName, tool: NewSubmitPlan(), readonly: false},
		{name: GetGoalName, tool: NewGetGoal(), readonly: true},
		{name: CreateGoalName, tool: NewCreateGoal(), readonly: false},
		{name: ReportGoalName, tool: NewReportGoal(), readonly: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.tool.Spec().Readonly; got != tc.readonly {
				t.Fatalf("%s readonly = %v, want %v", tc.name, got, tc.readonly)
			}
		})
	}
}

func TestWorkControlToolsRequireRuntimeCapability(t *testing.T) {
	if _, err := NewGetGoal().InvokableRun(context.Background(), json.RawMessage(`{}`)); err == nil {
		t.Fatal("get_goal without runtime capability must fail closed")
	}
}

func TestWorkControlToolsExposeOnlyBoundedModelArguments(t *testing.T) {
	registry := NewRegistry(NewEnterPlanMode(), NewSubmitPlan(), NewGetGoal(), NewCreateGoal(), NewReportGoal())
	for _, spec := range registry.Specs() {
		if _, ok := spec.Params["session_id"]; ok {
			t.Fatalf("%s must not accept session_id from the model", spec.Name)
		}
		if _, ok := spec.Params["run_id"]; ok {
			t.Fatalf("%s must not accept run_id from the model", spec.Name)
		}
	}
	if NewCreateGoal().Spec().Readonly {
		t.Fatal("create_goal must stay behind the existing host approval gate")
	}
}

func TestWorkControlToolsUseHostBoundCapability(t *testing.T) {
	goal := &domain.GoalState{Ref: domain.GoalRef{ID: "goal-1", Revision: 1}, Objective: "bounded", Phase: domain.WorkPhaseActive, MaxRounds: 2}
	ops := &fakeWorkControlOperations{
		state: domain.WorkState{SessionID: "session-1", Version: 4, Goal: goal},
		goal:  goal,
	}
	ctx := WithWorkControl(context.Background(), ops)

	out, err := NewGetGoal().InvokableRun(ctx, json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("get_goal: %v", err)
	}
	if !strings.Contains(out, "goal-1") || !strings.Contains(out, "bounded") {
		t.Fatalf("get_goal output = %q", out)
	}
	out, err = NewCreateGoal().InvokableRun(ctx, json.RawMessage(`{"objective":"next","max_rounds":2}`))
	if err != nil {
		t.Fatalf("create_goal: %v", err)
	}
	if !strings.Contains(out, "session-1") {
		t.Fatalf("create_goal output = %q", out)
	}
}
