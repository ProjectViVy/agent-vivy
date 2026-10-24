package runtime

// ND-3 (docs/archive/plans/nudge/ND-3.md): the transient reminder at the model
// boundary (NUDGE-DESIGN §6/§7). An immutable WrapModel middleware reads
// committed producer outcomes for the native batch whose tool results sit
// at the input tail, persists at most one tool.nudge scheduling event
// through the Service-supplied emitter, then appends a tagged
// runtime_nudge user message to a COPY of the input. No Journal, model
// or storage handle lives inside the middleware — everything arrives
// through context per leg.

import (
	"context"
	"fmt"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/tools"
	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/model"
	einotool "github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

// nudgeReminderMaxBytes is the §7 ceiling on the rendered reminder.
const nudgeReminderMaxBytes = 1024

// nudgeEmitter persists the scheduled tool.nudge event for one notice.
// The Service installs it per leg; a false/error return aborts the
// model call — a reminder never degrades to an unrecorded injection.
type nudgeEmitter func(context.Context, nudgeNotice) error

type nudgeEmitterContextKey struct{}

func withNudgeEmitter(ctx context.Context, emit nudgeEmitter) context.Context {
	return context.WithValue(ctx, nudgeEmitterContextKey{}, emit)
}

func nudgeEmitterFromContext(ctx context.Context) nudgeEmitter {
	emit, _ := ctx.Value(nudgeEmitterContextKey{}).(nudgeEmitter)
	return emit
}

// newNudgeMiddleware builds the immutable boundary middleware.
// maxContextBytes is the run's context byte budget; the reminder must
// fit inside what the current input leaves.
func newNudgeMiddleware(maxContextBytes int) adk.TypedChatModelAgentMiddleware[*schema.AgenticMessage] {
	return &nudgeMiddleware{
		TypedBaseChatModelAgentMiddleware: &adk.TypedBaseChatModelAgentMiddleware[*schema.AgenticMessage]{},
		maxContextBytes:                   maxContextBytes,
	}
}

type nudgeMiddleware struct {
	*adk.TypedBaseChatModelAgentMiddleware[*schema.AgenticMessage]
	maxContextBytes int
}

func (m *nudgeMiddleware) WrapModel(_ context.Context, inner model.BaseModel[*schema.AgenticMessage], _ *adk.TypedModelContext[*schema.AgenticMessage]) (model.BaseModel[*schema.AgenticMessage], error) {
	return &nudgeWrappedModel{inner: inner, maxContextBytes: m.maxContextBytes}, nil
}

// nudgeWrappedModel implements the §6 request barrier on both model
// entry points: evaluation happens BEFORE inner.Generate/Stream, never on
// their outputs. It is safe for provider retries — Take yields the same
// notice again without a second scheduling signal.
type nudgeWrappedModel struct {
	inner           model.BaseModel[*schema.AgenticMessage]
	maxContextBytes int
}

func (w *nudgeWrappedModel) Generate(ctx context.Context, input []*schema.AgenticMessage, opts ...model.Option) (*schema.AgenticMessage, error) {
	prepared, err := w.prepare(ctx, input)
	if err != nil {
		return nil, err
	}
	return w.inner.Generate(ctx, prepared, opts...)
}

func (w *nudgeWrappedModel) Stream(ctx context.Context, input []*schema.AgenticMessage, opts ...model.Option) (*schema.StreamReader[*schema.AgenticMessage], error) {
	prepared, err := w.prepare(ctx, input)
	if err != nil {
		return nil, err
	}
	return w.inner.Stream(ctx, prepared, opts...)
}

func (w *nudgeWrappedModel) prepare(ctx context.Context, input []*schema.AgenticMessage) ([]*schema.AgenticMessage, error) {
	// Nothing settled this turn → no barrier. The reminder only ever
	// attaches after the current tool results (§7).
	ids := trailingToolCallIDs(input)
	if len(ids) == 0 {
		return input, nil
	}
	state := nudgeStateFromContext(ctx)
	if state == nil {
		return input, nil
	}
	notice, first, err := state.Take(ctx, ids)
	if err != nil {
		return nil, err
	}
	if notice == nil {
		return input, nil
	}
	rendered := renderNudge(*notice)
	if len(rendered) > nudgeReminderMaxBytes {
		return nil, fmt.Errorf("%w: nudge reminder requires %d bytes", ErrContextBudgetExceeded, len(rendered))
	}
	if w.maxContextBytes > 0 && projectedContextBytes(input)+len(rendered) > w.maxContextBytes {
		return nil, fmt.Errorf("%w: nudge reminder does not fit the remaining context budget", ErrContextBudgetExceeded)
	}
	// Persist scheduling before the inner model ever sees the reminder;
	// an append failure stops the call with the original cause.
	if first {
		if emit := nudgeEmitterFromContext(ctx); emit != nil {
			if err := emit(ctx, *notice); err != nil {
				state.Abort(err)
				return nil, err
			}
		}
	}
	out := make([]*schema.AgenticMessage, len(input), len(input)+1)
	copy(out, input)
	reminder := agenticUserMessage(rendered)
	reminder.Extra = map[string]any{
		"kind":             "runtime_nudge",
		"tool_call_id":     notice.CallID,
		"tool_name":        notice.ToolName,
		"template_version": notice.TemplateVersion,
	}
	return append(out, reminder), nil
}

// trailingToolCallIDs returns the call ids of the contiguous tool-result
// messages at the input tail — the batch that just settled and must be
// covered before the inner model may run. Native request order is independent
// of when the derived consumer journals request and result projections.
func trailingToolCallIDs(input []*schema.AgenticMessage) []string {
	var ids []string
	for i := len(input) - 1; i >= 0; i-- {
		msg := input[i]
		if msg == nil {
			break
		}
		results := agenticToolResults(msg)
		if len(results) == 0 || results[0].CallID == "" {
			break
		}
		ids = append(ids, results[0].CallID)
	}
	for i, j := 0, len(ids)-1; i < j; i, j = i+1, j-1 {
		ids[i], ids[j] = ids[j], ids[i]
	}
	return ids
}

// Eino owns these readonly metadata tools, so they do not enter toolAdapter.
// Reuse its typed invocation wrapper at the producer, never a consumer hook.
func (m *nudgeMiddleware) WrapInvokableToolCall(_ context.Context, endpoint adk.InvokableToolCallEndpoint, tool *adk.ToolContext) (adk.InvokableToolCallEndpoint, error) {
	if tool.Name != officialToolSearchName && tool.Name != "skill" {
		return endpoint, nil
	}
	return func(ctx context.Context, args string, opts ...einotool.Option) (string, error) {
		beginNudgeInvocation(ctx)
		result, err := endpoint(ctx, args, opts...)
		if err != nil {
			return result, err
		}
		outcome := domain.ToolOperationOutcome{Result: truncateUTF8(tools.RedactSensitive(result), 64<<10), Status: domain.ToolOutcomeSuccess, Effects: domain.ToolEffectsNone}
		return result, recordNudgeProjection(ctx, tool.Name, args, outcome)
	}, nil
}
