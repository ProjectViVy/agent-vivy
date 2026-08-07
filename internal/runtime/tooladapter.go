package runtime

import (
	"context"
	"encoding/json"

	einotool "github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/tools"
)

// toolAdapter exposes a Vivy tools.Tool to Eino's ToolsNode and enforces
// the approval gate (D-012): readonly tools execute directly; effectful
// tools interrupt on first execution and only run for real once a resume
// delivers an approved decision. InvokableRun of the wrapped tool is only
// ever reached for calls that already passed policy.
type toolAdapter struct {
	t tools.Tool
}

var _ einotool.InvokableTool = (*toolAdapter)(nil)

func newToolAdapter(t tools.Tool) *toolAdapter {
	return &toolAdapter{t: t}
}

func (a *toolAdapter) Info(_ context.Context) (*schema.ToolInfo, error) {
	spec := a.t.Spec()
	return &schema.ToolInfo{Name: spec.Name, Desc: spec.Description}, nil
}

func (a *toolAdapter) InvokableRun(ctx context.Context, argumentsInJSON string, _ ...einotool.Option) (string, error) {
	spec := a.t.Spec()
	if spec.Readonly {
		return a.t.InvokableRun(ctx, json.RawMessage(argumentsInJSON))
	}
	wasInterrupted, _, _ := einotool.GetInterruptState[any](ctx)
	if !wasInterrupted {
		// First execution: pause the run so the service can surface
		// tool.approval_required over a durable checkpoint (D-029).
		return "", einotool.Interrupt(ctx, "approval required for "+spec.Name)
	}
	isTarget, hasData, decision := einotool.GetResumeContext[string](ctx)
	if !isTarget {
		// A sibling interrupt resumed first; keep waiting.
		return "", einotool.Interrupt(ctx, "still waiting for approval of "+spec.Name)
	}
	if hasData && decision == domain.ApprovalDenied {
		// A plain tool result lets the model continue and close the run
		// without executing the effectful call.
		return spec.Name + " was denied by the user and did not run; continue without it.", nil
	}
	return a.t.InvokableRun(ctx, json.RawMessage(argumentsInJSON))
}
