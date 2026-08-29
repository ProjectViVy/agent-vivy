package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"agent-vivy/internal/domain"
)

var ErrHookBlocked = errors.New("runtime: tool hook blocked execution")

type hookDecision string

const (
	hookAllow   hookDecision = "allow"
	hookDeny    hookDecision = "deny"
	hookRewrite hookDecision = "rewrite"
)

type ToolHookCall struct {
	RunID     domain.RunID
	ToolName  string
	Arguments json.RawMessage
	Profile   domain.PolicyProfile
}

type PreToolUseResult struct {
	Decision    hookDecision
	UpdatedArgs json.RawMessage
	Reason      string
}

// ToolHook is intentionally in-process for the first governance slice. An
// external adapter can implement this interface later without changing the
// tool execution order or failure semantics.
type ToolHook interface {
	Name() string
	PreToolUse(context.Context, ToolHookCall) (PreToolUseResult, error)
	PostToolUse(context.Context, ToolHookCall, string, error) error
}

type ToolHookChain struct {
	hooks   []ToolHook
	timeout time.Duration
}

func NewToolHookChain(timeout time.Duration, hooks ...ToolHook) *ToolHookChain {
	if timeout <= 0 {
		timeout = time.Second
	}
	return &ToolHookChain{hooks: append([]ToolHook(nil), hooks...), timeout: timeout}
}

func (c *ToolHookChain) PreToolUse(ctx context.Context, call ToolHookCall) (json.RawMessage, error) {
	args := append(json.RawMessage(nil), call.Arguments...)
	if c == nil {
		return args, nil
	}
	for _, hook := range c.hooks {
		started := time.Now()
		emitGovernanceEvent(ctx, GovernanceEvent{Type: domain.EventHookStarted, ToolName: call.ToolName, HookName: hook.Name(), Phase: "pre"})
		hookCtx, cancel := context.WithTimeout(ctx, c.timeout)
		result, err := hook.PreToolUse(hookCtx, ToolHookCall{
			RunID: call.RunID, ToolName: call.ToolName, Arguments: append(json.RawMessage(nil), args...), Profile: call.Profile,
		})
		cancel()
		elapsed := time.Since(started)
		if err != nil {
			reason := "pre-tool hook failed"
			if errors.Is(hookCtx.Err(), context.DeadlineExceeded) {
				reason = "pre-tool hook timed out"
			}
			emitGovernanceEvent(ctx, GovernanceEvent{Type: domain.EventHookBlocked, ToolName: call.ToolName, HookName: hook.Name(), Phase: "pre", Reason: reason, DurationMs: elapsed.Milliseconds()})
			return nil, fmt.Errorf("%w: %s: %v", ErrHookBlocked, hook.Name(), err)
		}
		switch result.Decision {
		case hookDeny:
			reason := result.Reason
			if reason == "" {
				reason = "pre-tool hook denied execution"
			}
			emitGovernanceEvent(ctx, GovernanceEvent{Type: domain.EventHookBlocked, ToolName: call.ToolName, HookName: hook.Name(), Phase: "pre", Reason: reason, DurationMs: elapsed.Milliseconds()})
			return nil, fmt.Errorf("%w: %s", ErrHookBlocked, reason)
		case hookRewrite:
			if len(result.UpdatedArgs) == 0 {
				return nil, fmt.Errorf("%w: %s returned an empty rewrite", ErrHookBlocked, hook.Name())
			}
			var value any
			if err := json.Unmarshal(result.UpdatedArgs, &value); err != nil {
				return nil, fmt.Errorf("%w: %s returned invalid JSON: %v", ErrHookBlocked, hook.Name(), err)
			}
			args = append(json.RawMessage(nil), result.UpdatedArgs...)
			emitGovernanceEvent(ctx, GovernanceEvent{Type: domain.EventHookCompleted, ToolName: call.ToolName, HookName: hook.Name(), Phase: "pre", Decision: string(hookRewrite), Reason: result.Reason, DurationMs: elapsed.Milliseconds()})
		default:
			emitGovernanceEvent(ctx, GovernanceEvent{Type: domain.EventHookCompleted, ToolName: call.ToolName, HookName: hook.Name(), Phase: "pre", Decision: string(hookAllow), DurationMs: elapsed.Milliseconds()})
		}
	}
	return args, nil
}

// PostToolUse is fail-open by design: tool side effects cannot be undone by a
// post hook. Failures are observable and never replace the tool result.
func (c *ToolHookChain) PostToolUse(ctx context.Context, call ToolHookCall, result string, toolErr error) {
	if c == nil {
		return
	}
	for _, hook := range c.hooks {
		started := time.Now()
		emitGovernanceEvent(ctx, GovernanceEvent{Type: domain.EventHookStarted, ToolName: call.ToolName, HookName: hook.Name(), Phase: "post"})
		hookCtx, cancel := context.WithTimeout(ctx, c.timeout)
		err := hook.PostToolUse(hookCtx, call, result, toolErr)
		cancel()
		elapsed := time.Since(started)
		if err != nil {
			reason := "post-tool hook failed"
			if errors.Is(hookCtx.Err(), context.DeadlineExceeded) {
				reason = "post-tool hook timed out"
			}
			slog.Warn(reason, "run", string(call.RunID), "hook", hook.Name(), "tool", call.ToolName, "err", err)
			emitGovernanceEvent(ctx, GovernanceEvent{Type: domain.EventHookBlocked, ToolName: call.ToolName, HookName: hook.Name(), Phase: "post", Reason: reason, DurationMs: elapsed.Milliseconds()})
			continue
		}
		emitGovernanceEvent(ctx, GovernanceEvent{Type: domain.EventHookCompleted, ToolName: call.ToolName, HookName: hook.Name(), Phase: "post", Decision: string(hookAllow), DurationMs: elapsed.Milliseconds()})
	}
}

type GovernanceEvent struct {
	Type       domain.EventType
	ToolName   string
	HookName   string
	Phase      string
	Decision   string
	Profile    domain.PolicyProfile
	PolicyHash string
	Reason     string
	DurationMs int64
	// Compaction fields for context.compacted events (numbers only, D-010).
	Mode            string
	BeforeTokens    int
	AfterTokens     int
	DroppedMessages int
	RetentionSuffix int
}

type governanceEventSinkKey struct{}

type GovernanceEventSink func(context.Context, GovernanceEvent) error

func withGovernanceEventSink(ctx context.Context, sink GovernanceEventSink) context.Context {
	return context.WithValue(ctx, governanceEventSinkKey{}, sink)
}

func emitGovernanceEvent(ctx context.Context, event GovernanceEvent) {
	sink, _ := ctx.Value(governanceEventSinkKey{}).(GovernanceEventSink)
	if sink == nil {
		return
	}
	if err := sink(ctx, event); err != nil {
		slog.Warn("governance event was not persisted", "type", event.Type, "tool", event.ToolName, "err", err)
	}
}
