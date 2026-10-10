package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/tools"
)

// ExecuteBrokerToolOperation runs the worker broker path under the same
// ExecuteBrokerToolOperation runs the worker broker path under the same
// durable operation boundary as native Eino tools. operationID is the stable
// runner call id; it must not change across a retry or recovery attempt.
func (s *Service) ExecuteBrokerToolOperation(ctx context.Context, operationID string, tool tools.Tool, policy *PolicyEngine, hooks *ToolHookChain, runID domain.RunID, profile domain.PolicyProfile, args any, maxResultBytes int, approved bool) (string, error) {
	if s == nil || s.deps.ToolOperations == nil {
		return "", ErrToolOperationUnavailable
	}
	if tool == nil {
		return "", fmt.Errorf("runtime: worker tool is not configured")
	}
	var err error
	policy, err = brokerPolicy(policy)
	if err != nil {
		return "", err
	}
	if s.deps.Runs == nil {
		return "", errors.New("runtime: tool operation run store is unavailable")
	}
	rawArgs, err := marshalBrokerArgs(args)
	if err != nil {
		return "", err
	}
	run, err := s.deps.Runs.GetRun(ctx, runID)
	if err != nil {
		return "", err
	}
	coordinator := serviceToolOperationCoordinator{service: s, runID: runID, sessionID: run.SessionID}
	operation, found, err := coordinator.Lookup(ctx, operationID, tool.Spec().Name, rawArgs)
	if err != nil {
		return "", err
	}
	var effective []byte
	if found {
		effective = append([]byte(nil), operation.EffectiveArguments...)
		if err := validatePreparedBrokerTool(ctx, tool, policy, profile, effective, approved); err != nil {
			return "", err
		}
	} else {
		prepared, err := prepareBrokerTool(ctx, tool, policy, hooks, runID, profile, rawArgs, approved)
		if err != nil {
			return "", err
		}
		effective = prepared
		operation, err = coordinator.Admit(ctx, operationID, tool.Spec().Name, rawArgs, effective, effective)
		if err != nil {
			return "", err
		}
	}
	return coordinator.Execute(ctx, operation, func(invokeCtx context.Context) (string, error) {
		return invokePreparedBrokerTool(invokeCtx, tool, hooks, runID, profile, effective, maxResultBytes)
	})
}

func brokerPolicy(policy *PolicyEngine) (*PolicyEngine, error) {
	if policy != nil {
		return policy, nil
	}
	return NewPolicyEngine(nil)
}

func marshalBrokerArgs(args any) (json.RawMessage, error) {
	if args == nil {
		return json.RawMessage(`{}`), nil
	}
	encoded, err := json.Marshal(args)
	if err != nil {
		return nil, fmt.Errorf("runtime: marshal worker tool args: %w", err)
	}
	return encoded, nil
}

func prepareBrokerTool(ctx context.Context, tool tools.Tool, policy *PolicyEngine, hooks *ToolHookChain, runID domain.RunID, profile domain.PolicyProfile, rawArgs json.RawMessage, approved bool) (json.RawMessage, error) {
	spec := tool.Spec()
	if err := tools.ValidateArgs(spec, rawArgs); err != nil {
		return nil, err
	}
	evaluation, err := policy.Evaluate(profile, spec, rawArgs)
	if err != nil {
		return nil, err
	}
	emitGovernanceEvent(ctx, GovernanceEvent{
		Type: domain.EventPolicyEvaluated, ToolName: spec.Name, Decision: string(evaluation.Decision),
		Profile: profile, PolicyHash: evaluation.Snapshot.Hash, Reason: evaluation.Reason,
	})
	if evaluation.Decision == domain.PolicyDeny || (!approved && evaluation.Decision != domain.PolicyAllow) {
		return nil, fmt.Errorf("%w: child worker request for %s is not allowed", ErrPolicyDenied, spec.Name)
	}
	if hooks == nil {
		return append(json.RawMessage(nil), rawArgs...), nil
	}
	originalArgs := string(rawArgs)
	effective, err := hooks.PreToolUse(ctx, ToolHookCall{RunID: runID, ToolName: spec.Name, Arguments: rawArgs, Profile: profile})
	if err != nil {
		return nil, err
	}
	if err := tools.ValidateArgs(spec, effective); err != nil {
		return nil, err
	}
	if string(effective) != originalArgs {
		if next, err := policy.Evaluate(profile, spec, effective); err != nil {
			return nil, err
		} else if next.Decision != domain.PolicyAllow {
			return nil, fmt.Errorf("%w: rewritten child worker request for %s", ErrPolicyDenied, spec.Name)
		}
	}
	return effective, nil
}

func validatePreparedBrokerTool(ctx context.Context, tool tools.Tool, policy *PolicyEngine, profile domain.PolicyProfile, effective json.RawMessage, approved bool) error {
	spec := tool.Spec()
	if err := tools.ValidateArgs(spec, effective); err != nil {
		return err
	}
	evaluation, err := policy.Evaluate(profile, spec, effective)
	if err != nil {
		return err
	}
	emitGovernanceEvent(ctx, GovernanceEvent{
		Type: domain.EventPolicyEvaluated, ToolName: spec.Name, Decision: string(evaluation.Decision),
		Profile: profile, PolicyHash: evaluation.Snapshot.Hash, Reason: "revalidated persisted tool operation",
	})
	if evaluation.Decision == domain.PolicyDeny || (!approved && evaluation.Decision != domain.PolicyAllow) {
		return fmt.Errorf("%w: persisted child worker request for %s is not allowed", ErrPolicyDenied, spec.Name)
	}
	return nil
}

func invokePreparedBrokerTool(ctx context.Context, tool tools.Tool, hooks *ToolHookChain, runID domain.RunID, profile domain.PolicyProfile, args json.RawMessage, maxResultBytes int) (string, error) {
	result, toolErr := tool.InvokableRun(tools.WithRunID(ctx, runID), args)
	if hooks != nil {
		hooks.PostToolUse(ctx, ToolHookCall{RunID: runID, ToolName: tool.Spec().Name, Arguments: args, Profile: profile}, result, toolErr)
	}
	if toolErr != nil {
		return "", toolErr
	}
	return compactToolResult(untrustedToolResultHeader+result, maxResultBytes), nil
}
