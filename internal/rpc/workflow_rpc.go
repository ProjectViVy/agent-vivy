package rpc

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
	"agent-vivy/internal/tools"
	workflow "agent-vivy/internal/workflow"
	"agent-vivy/internal/workflowhost"
)

type workflowDefinitionParams struct {
	Definition json.RawMessage `json:"definition"`
}

type workflowGetParams struct {
	ID  string `json:"id"`
	Rev int64  `json:"rev,omitempty"`
}

type workflowRunParams struct {
	ID        string         `json:"id"`
	Rev       int64          `json:"rev,omitempty"`
	Inputs    map[string]any `json:"inputs"`
	SessionID string         `json:"session_id,omitempty"`
}

type workflowRunsParams struct {
	ID    string `json:"id"`
	Limit int    `json:"limit,omitempty"`
}

func (h *controlHandler) workflowHost() (tools.WorkflowOperations, *Error) {
	if h.deps.Workflow == nil {
		return nil, &Error{Code: MethodNotFound, Message: "workflow host is not configured"}
	}
	return h.deps.Workflow, nil
}

func (h *controlHandler) listWorkflows(ctx context.Context) (any, *Error) {
	workflowHost, rpcErr := h.workflowHost()
	if rpcErr != nil {
		return nil, rpcErr
	}
	workflows, err := workflowHost.List(ctx)
	if err != nil {
		return nil, workflowError(err)
	}
	if workflows == nil {
		workflows = []workflowhost.DefinitionSummary{}
	}
	return map[string]any{"workflows": workflows}, nil
}

func (h *controlHandler) getWorkflow(ctx context.Context, request Request) (any, *Error) {
	workflowHost, rpcErr := h.workflowHost()
	if rpcErr != nil {
		return nil, rpcErr
	}
	var params workflowGetParams
	if err := decodeParams(request, &params); err != nil {
		return nil, err
	}
	if strings.TrimSpace(params.ID) == "" || params.Rev < 0 {
		return nil, &Error{Code: InvalidParams, Message: "id is required and rev must not be negative"}
	}
	workflowView, err := workflowHost.Get(ctx, params.ID, params.Rev)
	if err != nil {
		return nil, workflowError(err)
	}
	return map[string]any{"workflow": workflowView}, nil
}

func (h *controlHandler) validateWorkflow(ctx context.Context, request Request) (any, *Error) {
	workflowHost, rpcErr := h.workflowHost()
	if rpcErr != nil {
		return nil, rpcErr
	}
	definition, rpcErr := parseWorkflowDefinition(request)
	if rpcErr != nil {
		return nil, rpcErr
	}
	result, err := workflowHost.Validate(ctx, definition)
	if err != nil {
		var validationErr *workflow.ValidationError
		if errors.As(err, &validationErr) {
			return map[string]any{"valid": false, "diagnostics": validationErr.Diagnostics}, nil
		}
		return nil, workflowError(err)
	}
	diagnostics := result.Diagnostics
	if diagnostics == nil {
		diagnostics = []workflow.Diagnostic{}
	}
	return map[string]any{"valid": true, "diagnostics": diagnostics}, nil
}

func (h *controlHandler) defineWorkflow(ctx context.Context, request Request) (any, *Error) {
	workflowHost, rpcErr := h.workflowHost()
	if rpcErr != nil {
		return nil, rpcErr
	}
	definition, rpcErr := parseWorkflowDefinition(request)
	if rpcErr != nil {
		return nil, rpcErr
	}
	result, err := workflowHost.Define(ctx, definition)
	if err != nil {
		return nil, workflowError(err)
	}
	return result, nil
}

func (h *controlHandler) runWorkflow(ctx context.Context, peer *Peer, request Request) (any, *Error) {
	workflowHost, rpcErr := h.workflowHost()
	if rpcErr != nil {
		return nil, rpcErr
	}
	params, rpcErr := parseWorkflowRunParams(request)
	if rpcErr != nil {
		return nil, rpcErr
	}
	result, err := workflowHost.Run(ctx, workflowhost.RunRequest{ID: params.ID, Rev: params.Rev, Inputs: params.Inputs, SessionID: domainSessionID(params.SessionID)})
	if err != nil {
		return nil, workflowError(err)
	}
	h.bindPeerRunResult(ctx, peer, result)
	return result, nil
}

func (h *controlHandler) listWorkflowRuns(ctx context.Context, request Request) (any, *Error) {
	workflowHost, rpcErr := h.workflowHost()
	if rpcErr != nil {
		return nil, rpcErr
	}
	var params workflowRunsParams
	if err := decodeParams(request, &params); err != nil {
		return nil, err
	}
	if strings.TrimSpace(params.ID) == "" || params.Limit < 0 {
		return nil, &Error{Code: InvalidParams, Message: "id is required and limit must not be negative"}
	}
	if params.Limit == 0 {
		params.Limit = 100
	}
	runs, err := workflowHost.Runs(ctx, params.ID, params.Limit)
	if err != nil {
		return nil, workflowError(err)
	}
	if runs == nil {
		runs = []workflowhost.RunSummary{}
	}
	return map[string]any{"runs": runs}, nil
}

func parseWorkflowDefinition(request Request) ([]byte, *Error) {
	var params workflowDefinitionParams
	if err := decodeParams(request, &params); err != nil {
		return nil, err
	}
	if len(params.Definition) == 0 || string(params.Definition) == "null" {
		return nil, &Error{Code: InvalidParams, Message: "definition is required"}
	}
	var object map[string]any
	if err := json.Unmarshal(params.Definition, &object); err != nil || object == nil {
		return nil, &Error{Code: InvalidParams, Message: "definition must be a JSON object"}
	}
	return append([]byte(nil), params.Definition...), nil
}

func parseWorkflowRunParams(request Request) (workflowRunParams, *Error) {
	var params workflowRunParams
	if err := decodeParams(request, &params); err != nil {
		return params, err
	}
	if strings.TrimSpace(params.ID) == "" || params.Rev < 0 || params.Inputs == nil {
		return params, &Error{Code: InvalidParams, Message: "id and inputs are required and rev must not be negative"}
	}
	return params, nil
}

func domainSessionID(value string) domain.SessionID {
	return domain.SessionID(strings.TrimSpace(value))
}

func workflowError(err error) *Error {
	if err == nil {
		return nil
	}
	var validationErr *workflow.ValidationError
	switch {
	case errors.As(err, &validationErr), errors.Is(err, workflowhost.ErrInvalidInputs):
		data := json.RawMessage(nil)
		if validationErr != nil {
			data, _ = json.Marshal(map[string]any{"diagnostics": validationErr.Diagnostics})
		}
		return &Error{Code: InvalidParams, Message: err.Error(), Data: data}
	case errors.Is(err, workflow.ErrExecutionUnavailable):
		return &Error{Code: CodeUnavailable, Message: "workflow execution is not configured in this generation"}
	case errors.Is(err, workflow.ErrCapabilityUnavailable):
		return &Error{Code: CodeConflict, Message: err.Error()}
	case errors.Is(err, workflow.ErrAuthorityDenied):
		return &Error{Code: CodeConflict, Message: err.Error()}
	case errors.Is(err, storage.ErrNotFound):
		return &Error{Code: CodeNotFound, Message: "workflow not found"}
	case errors.Is(err, storage.ErrVersionConflict):
		return &Error{Code: CodeConflict, Message: "workflow definition revision conflict"}
	case errors.Is(err, workflowhost.ErrHostNotConfigured):
		return &Error{Code: MethodNotFound, Message: "workflow host is not configured"}
	default:
		return internalError(err)
	}
}
