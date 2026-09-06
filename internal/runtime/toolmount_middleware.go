package runtime

import (
	"context"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"

	"agent-vivy/internal/tools"
)

// fixedVisibleToolNames is the intentionally small, always-disclosed core.
// The names are product surface, not a search catalog: only tools that are
// actually enabled are included by NewEngine. Everything else active is
// supplied to Eino's dynamic tool-search middleware.
var fixedVisibleToolNames = map[string]struct{}{
	tools.AskUserName:     {},
	tools.ListDirName:     {},
	tools.ReadFileName:    {},
	tools.SearchFilesName: {},
	tools.SkillsListName:  {},
	tools.SkillViewName:   {},
	tools.WriteFileName:   {},
	tools.PatchName:       {},
	tools.MultiEditName:   {},
	tools.ExecuteName:     {},
	tools.BashName:        {},
}

func isFixedVisibleTool(name string) bool {
	_, ok := fixedVisibleToolNames[name]
	return ok
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
