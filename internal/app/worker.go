package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"agent-vivy/internal/domain"
	controlrpc "agent-vivy/internal/rpc"
	"agent-vivy/internal/runtime"
	"agent-vivy/internal/tools"
	"agent-vivy/internal/worker"
)

type workerController struct {
	service       *runtime.Service
	policy        *runtime.PolicyEngine
	hooks         *runtime.ToolHookChain
	tools         map[string]tools.Tool
	maxResultSize int
}

func newWorkerController(service *runtime.Service, policy *runtime.PolicyEngine, hooks *runtime.ToolHookChain, registered []tools.Tool, maxResultSize int) *workerController {
	byName := make(map[string]tools.Tool, len(registered))
	for _, tool := range registered {
		byName[tool.Spec().Name] = tool
	}
	return &workerController{service: service, policy: policy, hooks: hooks, tools: byName, maxResultSize: maxResultSize}
}

func (c *workerController) Run(ctx context.Context, request controlrpc.WorkerRequest) (controlrpc.WorkerResult, error) {
	if c == nil || c.service == nil {
		return controlrpc.WorkerResult{}, errors.New("worker controller is not wired")
	}
	parentID := domain.RunID(request.ParentRunID)
	snapshot, budget, workspaceID, err := c.service.WorkerParentAuthority(ctx, parentID)
	if err != nil {
		return controlrpc.WorkerResult{}, err
	}
	authority := worker.Authority{ParentRunID: parentID, Snapshot: snapshot, WorkspaceID: workspaceID, Budget: budget}
	broker := &workerToolBroker{tools: c.tools, policy: c.policy, hooks: c.hooks, budget: budget, maxResultSize: c.maxResultSize}
	supervisor, err := worker.Start(ctx, authority, broker, nil)
	if err != nil {
		return controlrpc.WorkerResult{}, err
	}
	defer func() { _ = supervisor.Close() }()

	var args any
	if len(request.ToolArgs) > 0 {
		if err := json.Unmarshal(request.ToolArgs, &args); err != nil {
			return controlrpc.WorkerResult{}, fmt.Errorf("worker tool args must be JSON: %w", err)
		}
	}
	spec := worker.Spec{
		RunID: domain.RunID(request.RunID), ParentRunID: parentID,
		PolicyProfile: domain.PolicyProfile(request.PolicyProfile), PolicyHash: request.PolicyHash,
		WorkspaceID: request.WorkspaceID, Text: request.Text, ToolName: request.ToolName, ToolArgs: args,
	}
	result, err := supervisor.Run(ctx, spec)
	if err != nil {
		return controlrpc.WorkerResult{}, err
	}
	return controlrpc.WorkerResult{RunID: request.RunID, Status: "completed", Result: result}, nil
}

type workerToolBroker struct {
	tools         map[string]tools.Tool
	policy        *runtime.PolicyEngine
	hooks         *runtime.ToolHookChain
	budget        *runtime.BudgetLedger
	maxResultSize int
}

func (b *workerToolBroker) Execute(ctx context.Context, call worker.ToolCall) (worker.ToolResult, error) {
	tool, ok := b.tools[call.ToolName]
	if !ok {
		return worker.ToolResult{}, fmt.Errorf("worker tool %q is not configured", call.ToolName)
	}
	if b.budget != nil {
		if err := b.budget.ReserveToolCall(); err != nil {
			return worker.ToolResult{}, err
		}
	}
	result, err := runtime.ExecuteBrokerTool(ctx, tool, b.policy, b.hooks, call.RunID, call.PolicyProfile, call.Args, b.maxResultSize)
	if err != nil {
		return worker.ToolResult{}, err
	}
	return worker.ToolResult{Result: result}, nil
}
