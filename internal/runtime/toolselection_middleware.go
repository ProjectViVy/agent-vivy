package runtime

import (
	"context"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"

	"agent-vivy/internal/tools"
)

// toolSurfaceMiddleware owns the model's VIEW of the tool surface. The
// executable universe (active + hidden tools) is registered on the agent so
// mid-run mounts can execute; each model generation this middleware narrows
// state.ToolInfos to the active set plus the tools skill_view mounted during
// the run. Tools injected by other middleware (e.g. the Eino skill tool) are
// foreign names outside the Vivy universe and are never hidden.
//
// Execution remains gated by the adapter: the selected set (active surface)
// plus the run's MountedTools.
type toolSurfaceMiddleware struct {
	*adk.BaseChatModelAgentMiddleware
	active   map[string]struct{}
	universe map[string]struct{}
}

func newToolSurfaceMiddleware(active, universe []string) adk.ChatModelAgentMiddleware {
	m := &toolSurfaceMiddleware{
		active:   make(map[string]struct{}, len(active)),
		universe: make(map[string]struct{}, len(universe)),
	}
	for _, name := range active {
		m.active[name] = struct{}{}
	}
	for _, name := range universe {
		m.universe[name] = struct{}{}
	}
	return m
}

func (m *toolSurfaceMiddleware) BeforeModelRewriteState(ctx context.Context, state *adk.ChatModelAgentState, _ *adk.ModelContext) (context.Context, *adk.ChatModelAgentState, error) {
	if state == nil || len(state.ToolInfos) == 0 {
		return ctx, state, nil
	}
	allowed := make(map[string]struct{}, len(m.active)+1)
	for name := range m.active {
		allowed[name] = struct{}{}
	}
	for _, name := range tools.MountedToolsFromContext(ctx).Mounted() {
		allowed[name] = struct{}{}
	}
	kept := make([]*schema.ToolInfo, 0, len(state.ToolInfos))
	for _, info := range state.ToolInfos {
		if info == nil {
			continue
		}
		if _, ok := allowed[info.Name]; ok {
			kept = append(kept, info)
			continue
		}
		if _, known := m.universe[info.Name]; !known {
			kept = append(kept, info)
		}
	}
	state.ToolInfos = kept
	return ctx, state, nil
}
