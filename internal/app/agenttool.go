package app

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"time"

	"agent-vivy/internal/domain"
	controlrpc "agent-vivy/internal/rpc"
	"agent-vivy/internal/runtime"
	"agent-vivy/internal/storage"
	"agent-vivy/internal/tools"
)

// agentToolRef defers the worker-manager binding: the builtin registry is
// built before the manager (which needs the registered tool set), so the
// agent tool holds a stable pointer that is armed once the manager exists.
type agentToolRef struct {
	mu      sync.Mutex
	manager *workerManager
}

func (r *agentToolRef) arm(manager *workerManager) {
	r.mu.Lock()
	r.manager = manager
	r.mu.Unlock()
}

// StartAgentTask runs one synchronous sub-agent over the durable child-run
// machinery: clean context, read-only tool surface, parent-owned budget,
// approvals surfacing in the parent session. The parent run id comes from
// the run-scoped tool context.
func (r *agentToolRef) StartAgentTask(ctx context.Context, task string) (string, error) {
	r.mu.Lock()
	m := r.manager
	r.mu.Unlock()
	if m == nil {
		return "", errors.New("sub-agent machinery is not wired")
	}
	parentRunID := tools.RunIDFromContext(ctx)
	if parentRunID == "" {
		return "", errors.New("agent tool requires a run-scoped parent")
	}
	started, err := m.StartChild(ctx, controlrpc.ChildRequest{
		ParentRunID: string(parentRunID),
		Text:        task,
	})
	if err != nil {
		// A saturated child budget is the model's planning problem, not a
		// broken run: surface it as a tool result the parent can act on.
		if errors.Is(err, storage.ErrChildConcurrencyLimit) {
			return "", tools.NewChildTaskError("too many sub-agents are already running for this request; wait for one to finish or do the work directly")
		}
		return "", err
	}
	final, err := m.WaitChild(ctx, started.ID)
	if err != nil {
		// The parent context is gone (cancel or shutdown); do not leave the
		// child running detached behind a failed tool call.
		cancelCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
		defer cancel()
		_, _ = m.CancelChild(cancelCtx, started.ID)
		return "", err
	}
	switch final.Status {
	case "completed":
		if strings.TrimSpace(final.Result) == "" {
			// A completion without text still needs a model-visible answer;
			// an empty tool result tells the parent nothing.
			return "the sub-agent finished without a written answer", nil
		}
		return final.Result, nil
	case "cancelled":
		return "", tools.NewChildTaskError("the sub-agent was cancelled before it finished")
	default:
		if final.Error != "" {
			return "", tools.NewChildTaskError(final.Error)
		}
		return "", tools.NewChildTaskError("the sub-agent ended without a result")
	}
}

// workflowToolRef delays binding the workflow operations until the native
// Service is composed, matching the agent tool's app-owned lifecycle.
type workflowToolRef struct {
	mu      sync.Mutex
	manager *workerManager
}

func (r *workflowToolRef) arm(manager *workerManager) {
	r.mu.Lock()
	r.manager = manager
	r.mu.Unlock()
}

func (r *workflowToolRef) DefinitionSchema() json.RawMessage {
	return runtime.WorkflowDefinitionSchema()
}

func (r *workflowToolRef) RunWorkflow(ctx context.Context, parentRunID domain.RunID, operationKey string, definition json.RawMessage) (tools.WorkflowTaskResult, error) {
	r.mu.Lock()
	manager := r.manager
	r.mu.Unlock()
	if manager == nil {
		return tools.WorkflowTaskResult{}, errors.New("workflow machinery is not wired")
	}
	if scopedParent := tools.RunIDFromContext(ctx); scopedParent == "" || scopedParent != parentRunID {
		return tools.WorkflowTaskResult{}, errors.New("workflow tool requires a matching run-scoped parent")
	}
	if tools.ToolCallIDFromContext(ctx) == "" {
		return tools.WorkflowTaskResult{}, errors.New("workflow tool requires a stable model call identity")
	}
	return manager.RunWorkflow(ctx, parentRunID, operationKey, definition)
}
