package runtime

import (
	"context"
	"encoding/json"
	"fmt"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/tools"
)

// ExecuteBrokerTool is the parent-owned tool boundary for child workers. A
// worker can request a named tool, but only the parent process can resolve,
// policy-check, hook, invoke, redact, and bound the result.
func ExecuteBrokerTool(ctx context.Context, tool tools.Tool, policy *PolicyEngine, hooks *ToolHookChain, runID domain.RunID, profile domain.PolicyProfile, args any, maxResultBytes int) (string, error) {
	return executeBrokerTool(ctx, tool, policy, hooks, runID, profile, args, maxResultBytes, false)
}

// ExecuteApprovedBrokerTool executes the exact argument set after a durable
// parent-side approval. The approval bypasses the initial prompt decision,
// but hook rewrites are revalidated and cannot silently widen authority.
func ExecuteApprovedBrokerTool(ctx context.Context, tool tools.Tool, policy *PolicyEngine, hooks *ToolHookChain, runID domain.RunID, profile domain.PolicyProfile, args any, maxResultBytes int) (string, error) {
	return executeBrokerTool(ctx, tool, policy, hooks, runID, profile, args, maxResultBytes, true)
}

func executeBrokerTool(ctx context.Context, tool tools.Tool, policy *PolicyEngine, hooks *ToolHookChain, runID domain.RunID, profile domain.PolicyProfile, args any, maxResultBytes int, approved bool) (string, error) {
	if tool == nil {
		return "", fmt.Errorf("runtime: worker tool is not configured")
	}
	if policy == nil {
		var err error
		policy, err = NewPolicyEngine(nil)
		if err != nil {
			return "", err
		}
	}
	rawArgs := json.RawMessage(`{}`)
	if args != nil {
		encoded, err := json.Marshal(args)
		if err != nil {
			return "", fmt.Errorf("runtime: marshal worker tool args: %w", err)
		}
		rawArgs = encoded
	}
	spec := tool.Spec()
	if err := tools.ValidateArgs(spec, rawArgs); err != nil {
		return "", err
	}
	if err := tools.ValidateArgsSafety(spec, rawArgs); err != nil {
		return "", err
	}
	evaluation, err := policy.Evaluate(profile, spec, rawArgs)
	if err != nil {
		return "", err
	}
	emitGovernanceEvent(ctx, GovernanceEvent{
		Type: domain.EventPolicyEvaluated, ToolName: spec.Name, Decision: string(evaluation.Decision),
		Profile: profile, PolicyHash: evaluation.Snapshot.Hash, Reason: evaluation.Reason,
	})
	if !approved && evaluation.Decision != domain.PolicyAllow {
		return "", fmt.Errorf("%w: child worker request for %s is not allowed", ErrPolicyDenied, spec.Name)
	}
	if hooks != nil {
		originalArgs := string(rawArgs)
		rawArgs, err = hooks.PreToolUse(ctx, ToolHookCall{RunID: runID, ToolName: spec.Name, Arguments: rawArgs, Profile: profile})
		if err != nil {
			return "", err
		}
		if err := tools.ValidateArgs(spec, rawArgs); err != nil {
			return "", err
		}
		if err := tools.ValidateArgsSafety(spec, rawArgs); err != nil {
			return "", err
		}
		if string(rawArgs) != originalArgs {
			if next, err := policy.Evaluate(profile, spec, rawArgs); err != nil {
				return "", err
			} else if next.Decision != domain.PolicyAllow {
				return "", fmt.Errorf("%w: rewritten child worker request for %s", ErrPolicyDenied, spec.Name)
			}
		}
	}
	result, toolErr := tool.InvokableRun(ctx, rawArgs)
	redacted := tools.RedactSensitive(result)
	if hooks != nil {
		hooks.PostToolUse(ctx, ToolHookCall{RunID: runID, ToolName: spec.Name, Arguments: rawArgs, Profile: profile}, redacted, toolErr)
	}
	if toolErr != nil {
		return "", toolErr
	}
	return compactToolResult(untrustedToolResultHeader+redacted, maxResultBytes), nil
}
