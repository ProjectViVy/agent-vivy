package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"agent-vivy/internal/tools"
	"github.com/cloudwego/eino/adk"
	native "github.com/cloudwego/eino/adk/middlewares/dynamictool/toolsearch"
	einotool "github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
)

// Eino 0.9.20's official search has no request authorization hook. This adapter
// selects an authorized catalog and delegates all search/selection semantics to
// the official middleware; there is no second search implementation or registry.
type authorizedToolSearch struct {
	*adk.BaseChatModelAgentMiddleware
	dynamic []einotool.BaseTool
	checks  map[string]tools.ContextAuthorizer
}
type authorizedSearchKey struct{ owner *authorizedToolSearch }

func newAuthorizedToolSearch(dynamic []einotool.BaseTool, checks map[string]tools.ContextAuthorizer) adk.ChatModelAgentMiddleware {
	return &authorizedToolSearch{BaseChatModelAgentMiddleware: &adk.BaseChatModelAgentMiddleware{}, dynamic: dynamic, checks: checks}
}
func (m *authorizedToolSearch) permitted(ctx context.Context) ([]einotool.BaseTool, error) {
	allowed := make([]einotool.BaseTool, 0, len(m.dynamic))
	for _, t := range m.dynamic {
		info, err := t.Info(ctx)
		if err != nil {
			return nil, err
		}
		if check := m.checks[info.Name]; check != nil && check.AuthorizeTool(toolAuthorizationContext(ctx)) != nil {
			continue
		}
		allowed = append(allowed, t)
	}
	return allowed, nil
}
func (m *authorizedToolSearch) BeforeAgent(ctx context.Context, run *adk.ChatModelAgentContext) (context.Context, *adk.ChatModelAgentContext, error) {
	allowed, err := m.permitted(ctx)
	if err != nil {
		return ctx, run, err
	}
	if len(allowed) == 0 {
		return ctx, run, nil
	}
	delegate, err := native.New(ctx, &native.Config{DynamicTools: allowed, UseModelToolSearch: false})
	if err != nil {
		return ctx, run, err
	}
	ctx = context.WithValue(ctx, authorizedSearchKey{m}, delegate)
	ctx, next, err := delegate.BeforeAgent(ctx, run)
	if err != nil || next == nil {
		return ctx, next, err
	}
	for i, t := range next.Tools {
		info, infoErr := t.Info(ctx)
		if infoErr != nil {
			return ctx, nil, infoErr
		}
		if info.Name == officialToolSearchName {
			next.Tools[i] = authorizedSearchInvocation{original: t, owner: m}
		}
	}
	return ctx, next, nil
}
func (m *authorizedToolSearch) BeforeModelRewriteState(ctx context.Context, state *adk.ChatModelAgentState, model *adk.ModelContext) (context.Context, *adk.ChatModelAgentState, error) {
	if delegate, ok := ctx.Value(authorizedSearchKey{m}).(adk.ChatModelAgentMiddleware); ok {
		return delegate.BeforeModelRewriteState(ctx, state, model)
	}
	return ctx, state, nil
}

type authorizedSearchInvocation struct {
	original einotool.BaseTool
	owner    *authorizedToolSearch
}

func (t authorizedSearchInvocation) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return t.original.Info(ctx)
}
func (t authorizedSearchInvocation) InvokableRun(ctx context.Context, raw string, opts ...einotool.Option) (string, error) {
	if coordinator, ok := toolOperationCoordinatorFromContext(ctx).(serviceToolOperationCoordinator); ok {
		retained, err := coordinator.retained(ctx)
		if err != nil {
			return "", err
		}
		if retained {
			op, err := coordinator.Admit(ctx, compose.GetToolCallID(ctx), officialToolSearchName, []byte(raw), []byte(raw), []byte(raw))
			if err != nil {
				return "", err
			}
			result, executeErr := coordinator.Execute(ctx, op, func(callCtx context.Context) (string, error) {
				result, err := t.search(callCtx, raw, opts...)
				if original, ok := callCtx.Value(originalToolOutcomeKey{}).(*originalToolOutcome); ok {
					original.result = result
					original.failure = err
					original.captured = true
				}
				return result, err
			})
			if executeErr != nil {
				return "", executeErr
			}
			// Native forward-selection and activation consume a machine JSON result.
			// Execute has verified the complete retained bytes; remove only its known
			// data framing, never a shortened projection or guessed/default metadata.
			result = strings.TrimPrefix(result, untrustedToolResultHeader)
			if !json.Valid([]byte(result)) {
				return "", ErrRecapUnavailable
			}
			return result, nil
		}
	}
	return t.search(ctx, raw, opts...)
}
func (t authorizedSearchInvocation) search(ctx context.Context, raw string, opts ...einotool.Option) (string, error) {
	allowed, err := t.owner.permitted(ctx)
	if err != nil {
		return "", err
	}
	if len(allowed) == 0 {
		return `{"matches":[]}`, nil
	}
	delegate, err := native.New(ctx, &native.Config{DynamicTools: allowed, UseModelToolSearch: false})
	if err != nil {
		return "", err
	}
	_, run, err := delegate.BeforeAgent(ctx, &adk.ChatModelAgentContext{})
	if err != nil {
		return "", err
	}
	for _, candidate := range run.Tools {
		info, err := candidate.Info(ctx)
		if err != nil {
			return "", err
		}
		if info.Name == officialToolSearchName {
			call, ok := candidate.(einotool.InvokableTool)
			if !ok {
				return "", fmt.Errorf("runtime: official search is not invokable")
			}
			return call.InvokableRun(ctx, raw, opts...)
		}
	}
	return "", fmt.Errorf("runtime: official search is unavailable")
}

// Apply after activation so a retained activation never grants authority.
type authorizedToolVisibility struct {
	*adk.BaseChatModelAgentMiddleware
	checks map[string]tools.ContextAuthorizer
}

func (m *authorizedToolVisibility) BeforeModelRewriteState(ctx context.Context, state *adk.ChatModelAgentState, _ *adk.ModelContext) (context.Context, *adk.ChatModelAgentState, error) {
	if state == nil {
		return ctx, state, nil
	}
	kept := state.ToolInfos[:0]
	for _, info := range state.ToolInfos {
		if info == nil {
			continue
		}
		if check := m.checks[info.Name]; check != nil && check.AuthorizeTool(toolAuthorizationContext(ctx)) != nil {
			continue
		}
		kept = append(kept, info)
	}
	state.ToolInfos = kept
	return ctx, state, nil
}

// Reuse the established Eino-to-Vivy identity conversion for discovery too.
func toolAuthorizationContext(ctx context.Context) context.Context {
	return tools.WithSessionID(tools.WithRunID(ctx, contextRunID(ctx)), contextSessionID(ctx))
}
