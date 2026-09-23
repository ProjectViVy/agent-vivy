package runtime

// ND-3 (docs/plans/nudge/ND-3.md): the transient reminder at the model
// boundary (NUDGE-DESIGN §6/§7). An immutable WrapModel middleware waits
// for the run-local nudgeState to seal the batch whose tool results sit
// at the input tail, persists at most one tool.nudge scheduling event
// through the Service-supplied emitter, then appends a tagged
// runtime_nudge user message to a COPY of the input. No Journal, model
// or storage handle lives inside the middleware — everything arrives
// through context per leg.

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/model"
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
func newNudgeMiddleware(maxContextBytes int) adk.ChatModelAgentMiddleware {
	return &nudgeMiddleware{
		BaseChatModelAgentMiddleware: &adk.BaseChatModelAgentMiddleware{},
		maxContextBytes:              maxContextBytes,
	}
}

type nudgeMiddleware struct {
	*adk.BaseChatModelAgentMiddleware
	maxContextBytes int
}

func (m *nudgeMiddleware) WrapModel(_ context.Context, inner model.BaseModel[*schema.Message], _ *adk.ModelContext) (model.BaseModel[*schema.Message], error) {
	return &nudgeWrappedModel{inner: inner, maxContextBytes: m.maxContextBytes}, nil
}

// nudgeWrappedModel implements the §6 request barrier on both model
// entry points: the wait happens BEFORE inner.Generate/Stream, never on
// their outputs. It is safe for provider retries — Take yields the same
// notice again without a second scheduling signal.
type nudgeWrappedModel struct {
	inner           model.BaseModel[*schema.Message]
	maxContextBytes int
}

func (w *nudgeWrappedModel) Generate(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	prepared, err := w.prepare(ctx, input)
	if err != nil {
		return nil, err
	}
	return w.inner.Generate(ctx, prepared, opts...)
}

func (w *nudgeWrappedModel) Stream(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	prepared, err := w.prepare(ctx, input)
	if err != nil {
		return nil, err
	}
	return w.inner.Stream(ctx, prepared, opts...)
}

func (w *nudgeWrappedModel) prepare(ctx context.Context, input []*schema.Message) ([]*schema.Message, error) {
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
				return nil, err
			}
		}
	}
	out := make([]*schema.Message, len(input), len(input)+1)
	copy(out, input)
	reminder := schema.UserMessage(rendered)
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
// sealed before the inner model may run. Keying on the input (not on the
// latest registered batch) matters: the engine can reach the next model
// call before the consumer finishes journaling the request events.
func trailingToolCallIDs(input []*schema.Message) []string {
	var ids []string
	for i := len(input) - 1; i >= 0; i-- {
		msg := input[i]
		if msg == nil || msg.Role != schema.Tool || msg.ToolCallID == "" {
			break
		}
		ids = append(ids, msg.ToolCallID)
	}
	for i, j := 0, len(ids)-1; i < j; i, j = i+1, j-1 {
		ids[i], ids[j] = ids[j], ids[i]
	}
	return ids
}
