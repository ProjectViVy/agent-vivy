// Package workflowhost owns the kernel boundary for workflow definitions and
// invocations. It deliberately does not import Eino; WF-2 supplies the
// executor behind the domain.PlanExecutor seam.
package workflowhost

import (
	"context"

	"agent-vivy/internal/domain"
	workflow "agent-vivy/internal/workflow"
)

// UnavailableExecutor is the WF-1 default. Keeping the unavailable state in
// the executor seam makes the RPC/tool contract explicit without creating a
// second scheduler or a fake successful execution path.
type UnavailableExecutor struct{}

func (UnavailableExecutor) ExecutePlan(context.Context, domain.RunID, domain.WorkflowPlan, domain.PlanEventSink) (domain.PlanResult, error) {
	return domain.PlanResult{}, workflow.ErrExecutionUnavailable
}

// Available lets WorkflowHost reject the unavailable executor before it
// creates a session, run row, or Journal entry.
func (UnavailableExecutor) Available() bool { return false }

var _ domain.PlanExecutor = UnavailableExecutor{}
