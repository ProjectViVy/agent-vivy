package runtime

import (
	"context"
	"errors"
	"strings"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/tools"
)

type workPlanGuidanceMiddleware struct {
	*adk.TypedBaseChatModelAgentMiddleware[*schema.AgenticMessage]
}

func newWorkPlanGuidanceMiddleware() adk.TypedChatModelAgentMiddleware[*schema.AgenticMessage] {
	return &workPlanGuidanceMiddleware{TypedBaseChatModelAgentMiddleware: &adk.TypedBaseChatModelAgentMiddleware[*schema.AgenticMessage]{}}
}

func (m *workPlanGuidanceMiddleware) BeforeModelRewriteState(
	ctx context.Context,
	state *adk.TypedChatModelAgentState[*schema.AgenticMessage],
	modelContext *adk.TypedModelContext[*schema.AgenticMessage],
) (context.Context, *adk.TypedChatModelAgentState[*schema.AgenticMessage], error) {
	_ = modelContext
	if state == nil {
		return ctx, state, nil
	}
	operations := tools.WorkControlFromContext(ctx)
	reader, ok := operations.(interface {
		modelWorkState(context.Context) (domain.WorkState, error)
	})
	if !ok {
		return ctx, state, nil
	}
	work, err := reader.modelWorkState(ctx)
	if err != nil {
		if errors.Is(err, ErrWorkUnavailable) {
			return ctx, state, nil
		}
		return ctx, nil, err
	}
	state.Messages = reconcilePlanGuidance(state.Messages, work.Plan.Active)
	return ctx, state, nil
}

// reconcilePlanGuidance keeps exactly one plan-guidance block in the system
// preamble and removes it when no plan is active. The guidance is plain text
// inside a system message, so the rewrite works on the message's projected
// text and rebuilds a single text block that keeps the message role.
func reconcilePlanGuidance(messages []*schema.AgenticMessage, active bool) []*schema.AgenticMessage {
	filtered := make([]*schema.AgenticMessage, 0, len(messages)+1)
	hasGuidance := false
	for _, message := range messages {
		if message == nil {
			filtered = append(filtered, nil)
			continue
		}
		if message.Role == schema.AgenticRoleTypeSystem {
			content := agenticMessageText(message)
			if strings.Contains(content, planGuidanceText) {
				if !active || hasGuidance {
					content = strings.ReplaceAll(content, planGuidanceText, "")
				} else {
					first := strings.Index(content, planGuidanceText)
					afterFirst := first + len(planGuidanceText)
					content = content[:afterFirst] + strings.ReplaceAll(content[afterFirst:], planGuidanceText, "")
					hasGuidance = true
				}
				if content != agenticMessageText(message) {
					content = strings.TrimSpace(content)
					if content == "" {
						continue
					}
					message = agenticSystemMessage(content)
				}
			}
		}
		filtered = append(filtered, message)
	}
	if !active || hasGuidance {
		return filtered
	}
	insertAt := 0
	for insertAt < len(filtered) && filtered[insertAt] != nil && filtered[insertAt].Role == schema.AgenticRoleTypeSystem {
		insertAt++
	}
	filtered = append(filtered, nil)
	copy(filtered[insertAt+1:], filtered[insertAt:])
	filtered[insertAt] = agenticSystemMessage(planGuidanceText)
	return filtered
}

func (s *Service) modelWorkState(ctx context.Context) (domain.WorkState, error) {
	if s == nil || s.deps.Work == nil {
		return domain.WorkState{}, ErrWorkUnavailable
	}
	sessionID := tools.SessionIDFromContext(ctx)
	if strings.TrimSpace(string(sessionID)) == "" {
		return domain.WorkState{}, ErrWorkSessionRequired
	}
	return s.deps.Work.ReadWork(ctx, sessionID)
}
