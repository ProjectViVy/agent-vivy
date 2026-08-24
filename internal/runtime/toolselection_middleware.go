package runtime

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/adk"
	einotool "github.com/cloudwego/eino/components/tool"
)

// toolSelectionMiddleware applies the request-scoped manifest at Eino's
// agent boundary. The adapter remains a second enforcement point, but the
// model and tool node also receive the filtered list instead of the whole
// config-enabled registry.
type toolSelectionMiddleware struct {
	*adk.BaseChatModelAgentMiddleware
}

func newToolSelectionMiddleware() adk.ChatModelAgentMiddleware {
	return &toolSelectionMiddleware{BaseChatModelAgentMiddleware: &adk.BaseChatModelAgentMiddleware{}}
}

func (m *toolSelectionMiddleware) BeforeAgent(ctx context.Context, runCtx *adk.ChatModelAgentContext) (context.Context, *adk.ChatModelAgentContext, error) {
	allowed, scoped := selectedToolSet(ctx)
	if !scoped || runCtx == nil {
		return ctx, runCtx, nil
	}
	filtered := *runCtx
	filtered.Tools = make([]einotool.BaseTool, 0, len(runCtx.Tools))
	for _, candidate := range runCtx.Tools {
		info, err := candidate.Info(ctx)
		if err != nil {
			return ctx, nil, fmt.Errorf("runtime: inspect tool for selection: %w", err)
		}
		if _, ok := allowed[info.Name]; ok {
			filtered.Tools = append(filtered.Tools, candidate)
		}
	}
	return ctx, &filtered, nil
}
