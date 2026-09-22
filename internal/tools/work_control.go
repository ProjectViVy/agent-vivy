package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"agent-vivy/internal/domain"
)

const (
	EnterPlanModeName = "enter_plan_mode"
	SubmitPlanName    = "submit_plan"
	GetGoalName       = "get_goal"
	CreateGoalName    = "create_goal"
	ReportGoalName    = "report_goal"

	modelPlanMaxBytes     = 256 << 10
	modelGoalObjectiveMax = 8 << 10
	modelGoalMaxRounds    = 1000
)

// WorkControlOperations is the run-scoped host capability exposed to the
// model-facing work tools. The runtime binds it to the current primary run;
// callers cannot provide session or run identities as tool arguments.
type WorkControlOperations interface {
	EnterPlanMode(context.Context) (domain.WorkState, error)
	SubmitPlan(context.Context, string) (domain.WorkState, error)
	GetGoal(context.Context) (*domain.GoalState, error)
	CreateGoal(context.Context, string, int) (domain.WorkState, error)
	ReportGoal(context.Context, string, int64, string, string) (domain.WorkState, error)
}

type workControlContextKey struct{}

// WithWorkControl binds the runtime-owned work capability to one model run.
func WithWorkControl(ctx context.Context, operations WorkControlOperations) context.Context {
	return context.WithValue(ctx, workControlContextKey{}, operations)
}

// WorkControlFromContext returns the runtime-owned work capability, if any.
func WorkControlFromContext(ctx context.Context) WorkControlOperations {
	operations, _ := ctx.Value(workControlContextKey{}).(WorkControlOperations)
	return operations
}

func requireWorkControl(ctx context.Context) (WorkControlOperations, error) {
	if operations := WorkControlFromContext(ctx); operations != nil {
		return operations, nil
	}
	return nil, fmt.Errorf("tools: work control capability is not available")
}

type workGoalOutput struct {
	ID            string `json:"id"`
	Revision      int64  `json:"revision"`
	Objective     string `json:"objective"`
	Phase         string `json:"phase"`
	MaxRounds     int    `json:"max_rounds"`
	RoundsStarted int    `json:"rounds_started"`
	Reason        string `json:"reason,omitempty"`
	EvidenceRunID string `json:"evidence_run_id,omitempty"`
}

type workStateOutput struct {
	SessionID string          `json:"session_id"`
	Version   int64           `json:"version"`
	Goal      *workGoalOutput `json:"goal,omitempty"`
}

func workStateOutputOf(state domain.WorkState) workStateOutput {
	result := workStateOutput{SessionID: string(state.SessionID), Version: int64(state.Version)}
	if state.Goal != nil {
		result.Goal = &workGoalOutput{
			ID:            state.Goal.Ref.ID,
			Revision:      state.Goal.Ref.Revision,
			Objective:     state.Goal.Objective,
			Phase:         string(state.Goal.Phase),
			MaxRounds:     state.Goal.MaxRounds,
			RoundsStarted: state.Goal.RoundsStarted,
			Reason:        state.Goal.Reason,
			EvidenceRunID: string(state.Goal.EvidenceRunID),
		}
	}
	return result
}

func encodeWorkState(state domain.WorkState) (string, error) {
	encoded, err := json.Marshal(workStateOutputOf(state))
	if err != nil {
		return "", fmt.Errorf("tools: encode work state: %w", err)
	}
	return string(encoded), nil
}

func encodeGoal(goal *domain.GoalState) (string, error) {
	var output *workGoalOutput
	if goal != nil {
		output = &workGoalOutput{
			ID:            goal.Ref.ID,
			Revision:      goal.Ref.Revision,
			Objective:     goal.Objective,
			Phase:         string(goal.Phase),
			MaxRounds:     goal.MaxRounds,
			RoundsStarted: goal.RoundsStarted,
			Reason:        goal.Reason,
			EvidenceRunID: string(goal.EvidenceRunID),
		}
	}
	encoded, err := json.Marshal(map[string]any{"goal": output})
	if err != nil {
		return "", fmt.Errorf("tools: encode Goal: %w", err)
	}
	return string(encoded), nil
}

func decodeToolArgs(args json.RawMessage, target any) error {
	trimmed := bytes.TrimSpace(args)
	if len(trimmed) == 0 {
		trimmed = []byte("{}")
	}
	decoder := json.NewDecoder(bytes.NewReader(trimmed))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return &ArgError{Field: "args", Reason: fmt.Sprintf("must be one JSON object: %v", err)}
	}
	var extra any
	if err := decoder.Decode(&extra); err == nil {
		return &ArgError{Field: "args", Reason: "must contain one JSON object"}
	}
	return nil
}

func emptyToolArgs(args json.RawMessage) error {
	var target struct{}
	return decodeToolArgs(args, &target)
}

type enterPlanModeTool struct{}

func NewEnterPlanMode() Tool { return enterPlanModeTool{} }

func (enterPlanModeTool) Spec() domain.ToolSpec {
	return domain.ToolSpec{
		Name: EnterPlanModeName, Description: "Enter collaboration Plan mode for this session.", Readonly: true,
		Keywords: []string{"plan", "planning", "design"},
	}
}

func (enterPlanModeTool) InvokableRun(ctx context.Context, args json.RawMessage) (string, error) {
	if err := emptyToolArgs(args); err != nil {
		return "", err
	}
	operations, err := requireWorkControl(ctx)
	if err != nil {
		return "", err
	}
	state, err := operations.EnterPlanMode(ctx)
	if err != nil {
		return "", err
	}
	return encodeWorkState(state)
}

type submitPlanArgs struct {
	Markdown string `json:"markdown"`
}

type submitPlanTool struct{}

func NewSubmitPlan() Tool { return submitPlanTool{} }

func (submitPlanTool) Spec() domain.ToolSpec {
	return domain.ToolSpec{
		Name: SubmitPlanName, Description: "Submit a bounded Markdown Plan for human review.", Readonly: true,
		Keywords: []string{"plan", "submit", "review"},
		Params:   map[string]domain.ToolParam{"markdown": {Desc: "The Markdown plan to submit for review.", Required: true}},
	}
}

func (submitPlanTool) InvokableRun(ctx context.Context, args json.RawMessage) (string, error) {
	var params submitPlanArgs
	if err := decodeToolArgs(args, &params); err != nil {
		return "", err
	}
	params.Markdown = strings.TrimSpace(params.Markdown)
	if params.Markdown == "" {
		return "", &ArgError{Field: "markdown", Reason: "must not be empty"}
	}
	if len([]byte(params.Markdown)) > modelPlanMaxBytes {
		return "", &ArgError{Field: "markdown", Reason: "exceeds the 256 KiB limit"}
	}
	operations, err := requireWorkControl(ctx)
	if err != nil {
		return "", err
	}
	state, err := operations.SubmitPlan(ctx, params.Markdown)
	if err != nil {
		return "", err
	}
	return encodeWorkState(state)
}

type getGoalTool struct{}

func NewGetGoal() Tool { return getGoalTool{} }

func (getGoalTool) Spec() domain.ToolSpec {
	return domain.ToolSpec{
		Name: GetGoalName, Description: "Inspect the current session Goal, including phase, progress, reason, and evidence.", Readonly: true,
		Keywords: []string{"goal", "progress", "status"},
	}
}

func (getGoalTool) InvokableRun(ctx context.Context, args json.RawMessage) (string, error) {
	if err := emptyToolArgs(args); err != nil {
		return "", err
	}
	operations, err := requireWorkControl(ctx)
	if err != nil {
		return "", err
	}
	goal, err := operations.GetGoal(ctx)
	if err != nil {
		return "", err
	}
	return encodeGoal(goal)
}

type createGoalArgs struct {
	Objective string `json:"objective"`
	MaxRounds int    `json:"max_rounds"`
}

type createGoalTool struct{}

func NewCreateGoal() Tool { return createGoalTool{} }

func (createGoalTool) Spec() domain.ToolSpec {
	return domain.ToolSpec{
		Name: CreateGoalName, Description: "Create a bounded Goal for this session.", Readonly: false,
		Keywords: []string{"goal", "create", "objective"},
		Params: map[string]domain.ToolParam{
			"objective":  {Desc: "The bounded objective for the Goal.", Required: true},
			"max_rounds": {Desc: "Maximum number of continuation rounds.", Required: true, Type: "integer"},
		},
	}
}

func (createGoalTool) InvokableRun(ctx context.Context, args json.RawMessage) (string, error) {
	var params createGoalArgs
	if err := decodeToolArgs(args, &params); err != nil {
		return "", err
	}
	params.Objective = strings.TrimSpace(params.Objective)
	if params.Objective == "" {
		return "", &ArgError{Field: "objective", Reason: "must not be empty"}
	}
	if len([]byte(params.Objective)) > modelGoalObjectiveMax {
		return "", &ArgError{Field: "objective", Reason: "exceeds the 8 KiB limit"}
	}
	if params.MaxRounds <= 0 || params.MaxRounds > modelGoalMaxRounds {
		return "", &ArgError{Field: "max_rounds", Reason: "must be between 1 and 1000"}
	}
	operations, err := requireWorkControl(ctx)
	if err != nil {
		return "", err
	}
	state, err := operations.CreateGoal(ctx, params.Objective, params.MaxRounds)
	if err != nil {
		return "", err
	}
	return encodeWorkState(state)
}

type reportGoalArgs struct {
	GoalID   string `json:"goal_id"`
	Revision int64  `json:"revision"`
	Status   string `json:"status"`
	Reason   string `json:"reason"`
}

type reportGoalTool struct{}

func NewReportGoal() Tool { return reportGoalTool{} }

func (reportGoalTool) Spec() domain.ToolSpec {
	return domain.ToolSpec{
		Name: ReportGoalName, Description: "Report that the current Goal completed or is blocked, with a reason.", Readonly: true,
		Keywords: []string{"goal", "complete", "blocked", "report"},
		Params: map[string]domain.ToolParam{
			"goal_id":  {Desc: "The current Goal id.", Required: true},
			"revision": {Desc: "The current Goal revision.", Required: true, Type: "integer"},
			"status":   {Desc: "Either completed or blocked.", Required: true},
			"reason":   {Desc: "Why the Goal reached this terminal state."},
		},
	}
}

func (reportGoalTool) InvokableRun(ctx context.Context, args json.RawMessage) (string, error) {
	var params reportGoalArgs
	if err := decodeToolArgs(args, &params); err != nil {
		return "", err
	}
	params.GoalID, params.Status, params.Reason = strings.TrimSpace(params.GoalID), strings.TrimSpace(params.Status), strings.TrimSpace(params.Reason)
	if params.GoalID == "" {
		return "", &ArgError{Field: "goal_id", Reason: "must not be empty"}
	}
	if params.Revision <= 0 {
		return "", &ArgError{Field: "revision", Reason: "must be positive"}
	}
	if params.Status != "completed" && params.Status != "blocked" {
		return "", &ArgError{Field: "status", Reason: "must be completed or blocked"}
	}
	if params.Reason == "" {
		return "", &ArgError{Field: "reason", Reason: "must not be empty"}
	}
	if len([]byte(params.Reason)) > 4<<10 {
		return "", &ArgError{Field: "reason", Reason: "exceeds the 4 KiB limit"}
	}
	operations, err := requireWorkControl(ctx)
	if err != nil {
		return "", err
	}
	state, err := operations.ReportGoal(ctx, params.GoalID, params.Revision, params.Status, params.Reason)
	if err != nil {
		return "", err
	}
	return encodeWorkState(state)
}
