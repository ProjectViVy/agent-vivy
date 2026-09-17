package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"agent-vivy/internal/domain"
	workflowmodule "agent-vivy/internal/modules/workflow"
	workflow "agent-vivy/internal/workflow"
	"agent-vivy/internal/workflowhost"
	toolport "agent-vivy/sdk/port/tool"
)

const (
	WorkflowListName     = "workflow_list"
	WorkflowGetName      = "workflow_get"
	WorkflowValidateName = "workflow_validate"
	WorkflowDefineName   = "workflow_define"
	WorkflowRunName      = "workflow_run"
	WorkflowRunsName     = "workflow_runs"
)

func IsWorkflowTool(name string) bool {
	switch name {
	case WorkflowListName, WorkflowGetName, WorkflowValidateName, WorkflowDefineName, WorkflowRunName, WorkflowRunsName:
		return true
	default:
		return false
	}
}

// WorkflowOperations is the tool-facing projection of WorkflowHost. The
// host stays responsible for storage, capability snapshots, validation, and
// execution preflight; tools only marshal bounded results and proposals.
type WorkflowOperations interface {
	List(context.Context) ([]workflowhost.DefinitionSummary, error)
	Get(context.Context, string, int64) (workflowhost.DefinitionView, error)
	Validate(context.Context, []byte) (workflow.ValidationResult, error)
	PreviewDefine(context.Context, []byte) (workflowhost.DefinePreview, error)
	Define(context.Context, []byte) (workflowhost.DefineResult, error)
	PreviewRun(context.Context, workflowhost.RunRequest) (workflowhost.RunPreview, error)
	Run(context.Context, workflowhost.RunRequest) (workflowhost.RunResult, error)
	Runs(context.Context, string, int) ([]workflowhost.RunSummary, error)
}

type workflowTool struct {
	name string
	ops  WorkflowOperations
}

func NewWorkflowTools(ops WorkflowOperations) []Tool {
	return []Tool{
		&workflowTool{name: WorkflowListName, ops: ops},
		&workflowTool{name: WorkflowGetName, ops: ops},
		&workflowTool{name: WorkflowValidateName, ops: ops},
		&workflowTool{name: WorkflowDefineName, ops: ops},
		&workflowTool{name: WorkflowRunName, ops: ops},
		&workflowTool{name: WorkflowRunsName, ops: ops},
	}
}

func (t *workflowTool) Spec() domain.ToolSpec {
	definition := workflowmodule.ToolDefinition(t.name)
	return domain.ToolSpec{
		Name: t.name, Description: definition.Description,
		Readonly: definition.Effect != toolport.EffectWrite,
		Schema:   append(json.RawMessage(nil), definition.Schema...),
		Keywords: []string{"workflow", t.name},
	}
}

func (t *workflowTool) InvokableRun(ctx context.Context, args json.RawMessage) (string, error) {
	if t.ops == nil {
		return "", fmt.Errorf("tools: workflow host is not wired")
	}
	switch t.name {
	case WorkflowListName:
		workflows, err := t.ops.List(ctx)
		if err != nil {
			return "", err
		}
		return marshalToolResult(map[string]any{"workflows": workflows})
	case WorkflowGetName:
		input, err := decodeWorkflowGet(args)
		if err != nil {
			return "", err
		}
		workflowView, err := t.ops.Get(ctx, input.ID, input.Rev)
		if err != nil {
			return "", err
		}
		return marshalToolResult(map[string]any{"workflow": workflowView})
	case WorkflowValidateName:
		raw, err := decodeWorkflowDefinition(args)
		if err != nil {
			return "", err
		}
		result, validateErr := t.ops.Validate(ctx, raw)
		if validateErr != nil {
			var validationErr *workflow.ValidationError
			if !errors.As(validateErr, &validationErr) {
				return "", validateErr
			}
			return marshalToolResult(map[string]any{"valid": false, "diagnostics": validationErr.Diagnostics})
		}
		return marshalToolResult(map[string]any{"valid": true, "diagnostics": result.Diagnostics})
	case WorkflowDefineName:
		raw, err := decodeWorkflowDefinition(args)
		if err != nil {
			return "", err
		}
		defined, err := t.ops.Define(ctx, raw)
		if err != nil {
			return "", err
		}
		return marshalToolResult(defined)
	case WorkflowRunName:
		request, err := decodeWorkflowRun(args)
		if err != nil {
			return "", err
		}
		run, err := t.ops.Run(ctx, request)
		if err != nil {
			return "", err
		}
		return marshalToolResult(run)
	case WorkflowRunsName:
		input, err := decodeWorkflowRuns(args)
		if err != nil {
			return "", err
		}
		runs, err := t.ops.Runs(ctx, input.ID, input.Limit)
		if err != nil {
			return "", err
		}
		return marshalToolResult(map[string]any{"runs": runs})
	default:
		return "", fmt.Errorf("tools: unknown workflow tool %q", t.name)
	}
}

func (t *workflowTool) PrepareProposal(ctx context.Context, args json.RawMessage) (domain.ToolProposal, error) {
	if t.ops == nil {
		return domain.ToolProposal{}, fmt.Errorf("tools: workflow host is not wired")
	}
	switch t.name {
	case WorkflowDefineName:
		raw, err := decodeWorkflowDefinition(args)
		if err != nil {
			return domain.ToolProposal{}, err
		}
		preview, err := t.ops.PreviewDefine(ctx, raw)
		if err != nil {
			return domain.ToolProposal{}, err
		}
		return domain.ToolProposal{
			Action: t.name, Target: preview.ID,
			Preview:      fmt.Sprintf("define workflow %s@%d (%d nodes, sha256 %s)", preview.ID, preview.NextRev, preview.NodeCount, preview.Hash),
			RiskFindings: []string{"persist a new immutable workflow revision"}, Data: append(json.RawMessage(nil), raw...),
		}, nil
	case WorkflowRunName:
		request, err := decodeWorkflowRun(args)
		if err != nil {
			return domain.ToolProposal{}, err
		}
		preview, err := t.ops.PreviewRun(ctx, request)
		if err != nil {
			return domain.ToolProposal{}, err
		}
		return domain.ToolProposal{
			Action: t.name, Target: preview.ID,
			Preview:      fmt.Sprintf("run workflow %s@%d (sha256 %s, inputs %s)", preview.ID, preview.Rev, preview.Hash, preview.InputsSHA256),
			RiskFindings: []string{"start a workflow run"}, Data: mustMarshal(preview),
		}, nil
	default:
		return domain.ToolProposal{}, fmt.Errorf("tools: workflow tool %q is not effectful", t.name)
	}
}

type workflowGetInput struct {
	ID  string `json:"id"`
	Rev int64  `json:"rev,omitempty"`
}

func decodeWorkflowGet(args json.RawMessage) (workflowGetInput, error) {
	var input workflowGetInput
	if err := json.Unmarshal(args, &input); err != nil {
		return input, fmt.Errorf("workflow_get: invalid arguments: %w", err)
	}
	if strings.TrimSpace(input.ID) == "" || input.Rev < 0 {
		return input, &ArgError{Field: "id/rev", Reason: "id is required and rev must not be negative"}
	}
	return input, nil
}

func decodeWorkflowDefinition(args json.RawMessage) ([]byte, error) {
	var input struct {
		Definition json.RawMessage `json:"definition"`
	}
	if err := json.Unmarshal(args, &input); err != nil {
		return nil, fmt.Errorf("workflow: invalid arguments: %w", err)
	}
	if len(input.Definition) == 0 || string(input.Definition) == "null" || string(input.Definition) == "{}" {
		return nil, &ArgError{Field: "definition", Reason: "is required"}
	}
	var object map[string]any
	if err := json.Unmarshal(input.Definition, &object); err != nil || object == nil {
		return nil, &ArgError{Field: "definition", Reason: "must be a JSON object"}
	}
	return append([]byte(nil), input.Definition...), nil
}

func decodeWorkflowRun(args json.RawMessage) (workflowhost.RunRequest, error) {
	var input struct {
		ID        string         `json:"id"`
		Rev       int64          `json:"rev,omitempty"`
		Inputs    map[string]any `json:"inputs"`
		SessionID string         `json:"session_id,omitempty"`
	}
	if err := json.Unmarshal(args, &input); err != nil {
		return workflowhost.RunRequest{}, fmt.Errorf("workflow_run: invalid arguments: %w", err)
	}
	if strings.TrimSpace(input.ID) == "" || input.Rev < 0 || input.Inputs == nil {
		return workflowhost.RunRequest{}, &ArgError{Field: "id/rev/inputs", Reason: "id and inputs are required and rev must not be negative"}
	}
	return workflowhost.RunRequest{ID: input.ID, Rev: input.Rev, Inputs: input.Inputs, SessionID: domain.SessionID(strings.TrimSpace(input.SessionID))}, nil
}

func decodeWorkflowRuns(args json.RawMessage) (struct {
	ID    string `json:"id"`
	Limit int    `json:"limit,omitempty"`
}, error) {
	var input struct {
		ID    string `json:"id"`
		Limit int    `json:"limit,omitempty"`
	}
	if err := json.Unmarshal(args, &input); err != nil {
		return input, fmt.Errorf("workflow_runs: invalid arguments: %w", err)
	}
	if strings.TrimSpace(input.ID) == "" || input.Limit < 0 {
		return input, &ArgError{Field: "id/limit", Reason: "id is required and limit must not be negative"}
	}
	if input.Limit == 0 {
		input.Limit = 100
	}
	return input, nil
}

func mustMarshal(value any) json.RawMessage {
	data, _ := json.Marshal(value)
	return data
}
