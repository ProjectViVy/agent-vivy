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
	*adk.BaseChatModelAgentMiddleware
}

func newWorkPlanGuidanceMiddleware() adk.ChatModelAgentMiddleware {
	return &workPlanGuidanceMiddleware{BaseChatModelAgentMiddleware: &adk.BaseChatModelAgentMiddleware{}}
}

func (m *workPlanGuidanceMiddleware) BeforeModelRewriteState(
	ctx context.Context,
	state *adk.ChatModelAgentState,
	modelContext *adk.ModelContext,
) (context.Context, *adk.ChatModelAgentState, error) {
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

func reconcilePlanGuidance(messages []*schema.Message, active bool) []*schema.Message {
	filtered := make([]*schema.Message, 0, len(messages)+1)
	hasGuidance := false
	for _, message := range messages {
		if message == nil {
			filtered = append(filtered, nil)
			continue
		}
		if message.Role == schema.System && message.Content == planGuidanceText {
			continue
		}
		if message.Role == schema.System && strings.Contains(message.Content, planGuidanceText) {
			hasGuidance = true
		}
		filtered = append(filtered, message)
	}
	if !active || hasGuidance {
		return filtered
	}
	insertAt := 0
	for insertAt < len(filtered) && filtered[insertAt] != nil && filtered[insertAt].Role == schema.System {
		insertAt++
	}
	filtered = append(filtered, nil)
	copy(filtered[insertAt+1:], filtered[insertAt:])
	filtered[insertAt] = schema.SystemMessage(planGuidanceText)
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
