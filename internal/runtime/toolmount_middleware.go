package runtime

import (
	"context"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"

	"agent-vivy/internal/tools"
)

// suppressedToolVisibilityMiddleware strips ToolInfos for tools that must
// stay executable inside the agent but never be disclosed to the model —
// exposure-hidden tools (the deferred/hidden contract). They are absent
// from the dynamic search catalog too, so a model-side call can only be a
// hallucinated name; governedTool then rejects it with tool_not_active.
type suppressedToolVisibilityMiddleware struct {
	*adk.BaseChatModelAgentMiddleware
	names map[string]struct{}
}

func newSuppressedToolVisibilityMiddleware(names map[string]struct{}) adk.ChatModelAgentMiddleware {
	return &suppressedToolVisibilityMiddleware{
		BaseChatModelAgentMiddleware: &adk.BaseChatModelAgentMiddleware{},
		names:                        names,
	}
}

func (m *suppressedToolVisibilityMiddleware) BeforeModelRewriteState(ctx context.Context, state *adk.ChatModelAgentState, _ *adk.ModelContext) (context.Context, *adk.ChatModelAgentState, error) {
	if state == nil {
		return ctx, state, nil
	}
	kept := state.ToolInfos[:0]
	for _, info := range state.ToolInfos {
		if info == nil {
			continue
		}
		if _, hidden := m.names[info.Name]; hidden {
			continue
		}
		kept = append(kept, info)
	}
	state.ToolInfos = kept
	return ctx, state, nil
}

// activatedToolVisibilityMiddleware projects the session's tool-activation
// set (tools/activate and earlier tool_search results, journal-folded) onto
// the model's ToolInfos. Activated deferred tools are rehydrated on every
// model generation — mirroring the mounted-visibility rehydration pattern,
// which is required because Eino persists the previous rewrite.
type activatedToolVisibilityMiddleware struct {
	*adk.BaseChatModelAgentMiddleware
	deferred      map[string]*schema.ToolInfo
	deferredOrder []string
}

func newActivatedToolVisibilityMiddleware(deferred map[string]*schema.ToolInfo, order []string) adk.ChatModelAgentMiddleware {
	return &activatedToolVisibilityMiddleware{
		BaseChatModelAgentMiddleware: &adk.BaseChatModelAgentMiddleware{},
		deferred:                     deferred,
		deferredOrder:                append([]string(nil), order...),
	}
}

func (m *activatedToolVisibilityMiddleware) BeforeModelRewriteState(ctx context.Context, state *adk.ChatModelAgentState, _ *adk.ModelContext) (context.Context, *adk.ChatModelAgentState, error) {
	if state == nil {
		return ctx, state, nil
	}
	activation := tools.ToolActivationFromContext(ctx)
	if activation == nil {
		return ctx, state, nil
	}
	existing := make(map[string]struct{}, len(state.ToolInfos))
	for _, info := range state.ToolInfos {
		if info != nil {
			existing[info.Name] = struct{}{}
		}
	}
	for _, name := range m.deferredOrder {
		if !activation.Has(name) {
			continue
		}
		if _, duplicate := existing[name]; duplicate {
			continue
		}
		if info := m.deferred[name]; info != nil {
			existing[name] = struct{}{}
			state.ToolInfos = append(state.ToolInfos, info)
		}
	}
	return ctx, state, nil
}

// mountedToolVisibilityMiddleware projects Vivy's Skill mount state onto the
// model's persisted ToolInfos. Hidden tools remain executable in ToolsNode,
// but their schemas are absent until skill_view mounts them. Mounted schemas
// are rehydrated from the static inventory on every model generation; this is
// important because Eino persists the previous rewrite and cannot otherwise
// discover a ToolInfo added after the first model call.
type mountedToolVisibilityMiddleware struct {
	*adk.BaseChatModelAgentMiddleware
	hidden      map[string]*schema.ToolInfo
	hiddenOrder []string
}

func newMountedToolVisibilityMiddleware(hidden map[string]*schema.ToolInfo, order []string) adk.ChatModelAgentMiddleware {
	return &mountedToolVisibilityMiddleware{
		BaseChatModelAgentMiddleware: &adk.BaseChatModelAgentMiddleware{},
		hidden:                       hidden,
		hiddenOrder:                  append([]string(nil), order...),
	}
}

func (m *mountedToolVisibilityMiddleware) BeforeModelRewriteState(ctx context.Context, state *adk.ChatModelAgentState, _ *adk.ModelContext) (context.Context, *adk.ChatModelAgentState, error) {
	if state == nil {
		return ctx, state, nil
	}
	mounts := tools.MountedToolsFromContext(ctx)
	kept := make([]*schema.ToolInfo, 0, len(state.ToolInfos)+len(m.hiddenOrder))
	seen := make(map[string]struct{}, len(state.ToolInfos)+len(m.hiddenOrder))
	for _, info := range state.ToolInfos {
		if info == nil {
			continue
		}
		if _, hidden := m.hidden[info.Name]; hidden && (mounts == nil || !mounts.Has(info.Name)) {
			continue
		}
		if _, duplicate := seen[info.Name]; duplicate {
			continue
		}
		seen[info.Name] = struct{}{}
		kept = append(kept, info)
	}
	for _, name := range m.hiddenOrder {
		if mounts == nil || !mounts.Has(name) {
			continue
		}
		if _, duplicate := seen[name]; duplicate {
			continue
		}
		if info := m.hidden[name]; info != nil {
			seen[name] = struct{}{}
			kept = append(kept, info)
		}
	}
	state.ToolInfos = kept
	return ctx, state, nil
}
