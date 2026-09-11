package runtime

import (
	"context"
	"encoding/json"
	"fmt"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/toolhost"
	"agent-vivy/internal/tools"
)

type governedPreTool interface {
	ApplyPreTool(context.Context, json.RawMessage, toolhost.RevalidateFunc) (toolhost.MiddlewareResult, error)
}

func (a *toolAdapter) applyGovernedMiddleware(
	ctx context.Context,
	spec domain.ToolSpec,
	arguments json.RawMessage,
	profile domain.PolicyProfile,
	evaluation PolicyEvaluation,
) (json.RawMessage, PolicyEvaluation, []string, error) {
	governed, ok := a.t.(governedPreTool)
	if !ok {
		return arguments, evaluation, nil, nil
	}

	result, err := governed.ApplyPreTool(ctx, arguments, func(_ context.Context, request toolhost.Request) error {
		if request.ID != spec.Name {
			return fmt.Errorf("runtime: middleware changed tool identity from %q to %q", spec.Name, request.ID)
		}
		if err := tools.ValidateArgs(spec, request.Args); err != nil {
			return err
		}
		if err := tools.ValidateArgsSafety(spec, request.Args); err != nil {
			return err
		}
		next, err := a.policy.Evaluate(profile, spec, request.Args)
		if err != nil {
			return err
		}
		emitGovernanceEvent(ctx, GovernanceEvent{
			Type:       domain.EventPolicyEvaluated,
			ToolName:   spec.Name,
			Decision:   string(next.Decision),
			Profile:    profile,
			PolicyHash: next.Snapshot.Hash,
			Reason:     "post-middleware argument rewrite: " + next.Reason,
		})
		if next.Decision == domain.PolicyDeny {
			return fmt.Errorf("%w: rewritten arguments for %s", ErrPolicyDenied, spec.Name)
		}
		evaluation = next
		return nil
	})
	if err != nil {
		return nil, PolicyEvaluation{}, nil, err
	}
	return append(json.RawMessage(nil), result.Arguments...), evaluation, append([]string(nil), result.ApprovalClasses...), nil
}
