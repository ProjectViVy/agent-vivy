package runtime

// Boundary-safe steering (VCP-B1) on the agentic generation.
//
// The text generation injected a queued steer message by rewriting the
// resumed chat history through adk.WithHistoryModifier. That hook is typed to
// []*schema.Message and its state assertion does not match a typed agentic
// agent, so it cannot reach an AgenticMessage history. Steering therefore
// rides a Vivy-owned typed middleware instead: the resume leg binds the steer
// text on the run context, and the middleware appends it once — before the
// trailing assistant tool-call message, so the pinned ToolNode still runs the
// pending calls — on each model call of the resumed run. The run stays one
// run: turn.steered remains the single continuity marker.

import (
	"context"
	"crypto/sha256"
	"fmt"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"

	"agent-vivy/internal/domain"
)

const steerExtraKey = "__vivy_steer__"

type steerTextKey struct{}

// withSteerText binds the steer delivery for the resumed run leg.
func withSteerText(ctx context.Context, text string) context.Context {
	if text == "" {
		return ctx
	}
	return context.WithValue(ctx, steerTextKey{}, text)
}

func steerTextFrom(ctx context.Context) string {
	text, _ := ctx.Value(steerTextKey{}).(string)
	return text
}

// steerMessageText renders the queued steer turns as one delivery: multiple
// steer items under mode "all" join into a single message, because pi
// concatenates queued steering into the next turn.
func steerMessageText(items []domain.QueuedTurn) string {
	parts := make([]string, 0, len(items))
	for _, item := range items {
		parts = append(parts, item.Text)
	}
	return joinLines(parts)
}

type steerMiddleware struct {
	*adk.TypedBaseChatModelAgentMiddleware[*schema.AgenticMessage]
}

func newSteerMiddleware() adk.TypedChatModelAgentMiddleware[*schema.AgenticMessage] {
	return &steerMiddleware{TypedBaseChatModelAgentMiddleware: &adk.TypedBaseChatModelAgentMiddleware[*schema.AgenticMessage]{}}
}

func (m *steerMiddleware) BeforeModelRewriteState(ctx context.Context, state *adk.TypedChatModelAgentState[*schema.AgenticMessage], _ *adk.TypedModelContext[*schema.AgenticMessage]) (context.Context, *adk.TypedChatModelAgentState[*schema.AgenticMessage], error) {
	if state == nil {
		return ctx, state, nil
	}
	text := steerTextFrom(ctx)
	if text == "" {
		return ctx, state, nil
	}
	digest := fmt.Sprintf("%x", sha256.Sum256([]byte(text)))
	for _, msg := range state.Messages {
		if msg == nil || msg.Extra == nil {
			continue
		}
		if existing, ok := msg.Extra[steerExtraKey].(string); ok && existing == digest {
			return ctx, state, nil
		}
	}
	injected := agenticUserMessage(text)
	injected.Extra = map[string]any{steerExtraKey: digest}
	next := *state
	next.Messages = insertSteerBeforeTrailingToolCalls(state.Messages, injected)
	return ctx, &next, nil
}

// insertSteerBeforeTrailingToolCalls lands the steer message immediately before
// a trailing assistant tool-call message, so the pending calls still execute
// with the steer text in context. Without such a tail it appends.
func insertSteerBeforeTrailingToolCalls(messages []*schema.AgenticMessage, injected *schema.AgenticMessage) []*schema.AgenticMessage {
	if n := len(messages); n > 0 {
		last := messages[n-1]
		if last != nil && last.Role == schema.AgenticRoleTypeAssistant && len(agenticToolCalls(last)) > 0 {
			out := make([]*schema.AgenticMessage, 0, n+1)
			out = append(out, messages[:n-1]...)
			out = append(out, injected, last)
			return out
		}
	}
	return append(append(make([]*schema.AgenticMessage, 0, len(messages)+1), messages...), injected)
}
