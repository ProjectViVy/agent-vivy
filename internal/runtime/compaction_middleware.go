package runtime

import (
	"context"
	"fmt"
	"time"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/adk/middlewares/reduction"
	"github.com/cloudwego/eino/adk/middlewares/summarization"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	"agent-vivy/internal/domain"
)

// compaction middleware wiring (research AGENT-LOOP-PORT-COMPARISON §4.3-E2):
// official Eino `reduction` (deterministic clear of old tool turns) runs
// first, then official `summarization` (LLM summary) when the feed is still
// over the trigger. A tiny let-us-own ContextMonitor records the token count
// before any rewrite so the official hooks can report before/after sizes in
// a Vivy context.compacted event through the run's governance sink.

// compactionBeforeKey carries the pre-rewrite token count computed by
// ContextMonitor into the reduction/summarization hooks of the same turn.
type compactionBeforeKey struct{}

type compactionBefore struct {
	tokens  int
	message int
}

func withCompactionBefore(ctx context.Context, before compactionBefore) context.Context {
	return context.WithValue(ctx, compactionBeforeKey{}, before)
}

func compactionBeforeFrom(ctx context.Context) (compactionBefore, bool) {
	before, ok := ctx.Value(compactionBeforeKey{}).(compactionBefore)
	return before, ok
}

// contextMonitor records the token size of the message list before any
// reduction/summarization rewrite runs, so post-process hooks can report
// before/after sizes. It must be registered before the official middlewares.
type contextMonitor struct {
	*adk.BaseChatModelAgentMiddleware
}

func newContextMonitor() adk.ChatModelAgentMiddleware {
	return &contextMonitor{BaseChatModelAgentMiddleware: &adk.BaseChatModelAgentMiddleware{}}
}

func (m *contextMonitor) BeforeModelRewriteState(ctx context.Context, state *adk.TypedChatModelAgentState[*schema.Message], mc *adk.TypedModelContext[*schema.Message]) (context.Context, *adk.TypedChatModelAgentState[*schema.Message], error) {
	tokens, err := countMessageTokens(state.Messages, state.ToolInfos)
	if err != nil {
		return nil, nil, err
	}
	return withCompactionBefore(ctx, compactionBefore{tokens: tokens, message: len(state.Messages)}), state, nil
}

// countMessageTokens is the shared Vivy token estimate (1 token ≈ 4 bytes),
// applied to the in-run message list exactly like compaction.Measure.
func countMessageTokens(msgs []*schema.Message, tools []*schema.ToolInfo) (int, error) {
	total := 0
	for _, msg := range msgs {
		if msg == nil {
			continue
		}
		switch msg.Role {
		case schema.Assistant:
			total += len(msg.Content) + len(msg.ReasoningContent) + 16
			for _, tc := range msg.ToolCalls {
				total += len(tc.ID) + len(tc.Function.Name) + len(tc.Function.Arguments)
			}
		default:
			total += len(msg.Content) + 16
		}
	}
	for _, tl := range tools {
		if tl == nil {
			continue
		}
		info := *tl
		info.Extra = nil
		total += len(info.Name) + len(info.Desc)
	}
	return total / 4, nil
}

// buildCompactionHandlers assembles the official Eino middlewares with Vivy
// bridges. When the engine has no run context (e.g. Checkpoints nil in unit
// tests), the event hooks are inert: they only fire when the run ctx carries
// a governance sink, which the service always installs.
func buildCompactionHandlers(ctx context.Context, chatModel model.BaseModel[*schema.Message], summaryModel model.BaseModel[*schema.Message], policy CompactionPolicy, feedBudgetBytes int) ([]adk.ChatModelAgentMiddleware, error) {
	triggerTokens := policy.TriggerTokens(bytesToTokens(feedBudgetBytes))
	if triggerTokens <= 0 {
		triggerTokens = policy.TriggerTokens(0)
	}
	keepRecent := policy.KeepRecent
	if keepRecent <= 0 {
		keepRecent = 12
	}

	reducer, err := reduction.New(ctx, &reduction.Config{
		// Vivy's tooladapter already bounds every single result
		// (MaxToolResultBytes); the Eino truncation/offload phase would
		// double-truncate, so keep only the deterministic clear phase.
		SkipTruncation:            true,
		SkipClear:                 false,
		Backend:                   nil, // clear-only: in-memory placeholders, no offload (v1)
		MaxTokensForClear:         int64(triggerTokens),
		ClearRetentionSuffixLimit: keepRecent,
		TokenCounter: func(_ context.Context, msgs []*schema.Message, tools []*schema.ToolInfo) (int64, error) {
			n, err := countMessageTokens(msgs, tools)
			return int64(n), err
		},
		ClearPostProcess: func(ctx context.Context, state *adk.TypedChatModelAgentState[*schema.Message]) context.Context {
			before, ok := compactionBeforeFrom(ctx)
			after, _ := countMessageTokens(state.Messages, state.ToolInfos)
			if !ok {
				before = compactionBefore{tokens: after}
			}
			dropped := 0
			if before.message > 0 {
				dropped = before.message - len(state.Messages)
				if dropped < 0 {
					dropped = 0
				}
			}
			emitCompactionEvent(ctx, "reduction", before.tokens, after, dropped, keepRecent)
			return ctx
		},
	})
	if err != nil {
		return nil, fmt.Errorf("runtime: reduction middleware: %w", err)
	}

	// CMP-2: an optional cheaper summary model generates the summary; the
	// main chat model stays the one-shot failover. With no override, the
	// main model summarises directly and no failover is wired (a second
	// identical call would be a pointless retry).
	summModel := chatModel
	var failover *summarization.FailoverConfig
	if summaryModel != nil {
		summModel = summaryModel
		failover = &summarization.FailoverConfig{
			MaxRetries: intPtr(1),
			BackoffFunc: func(context.Context, int, *schema.Message, error) time.Duration {
				return 0
			},
			GetFailoverModel: func(_ context.Context, fc *summarization.FailoverContext) (model.BaseModel[*schema.Message], []*schema.Message, error) {
				// Mirror the middleware's default input: the original
				// leading system messages are dropped (the summary
				// instruction is re-supplied) and the rest is sandwiched
				// between the middleware's system/user instructions.
				input := make([]*schema.Message, 0, len(fc.OriginalMessages)+2)
				input = append(input, fc.SystemInstruction)
				rest := fc.OriginalMessages
				for len(rest) > 0 && rest[0] != nil && rest[0].Role == schema.System {
					rest = rest[1:]
				}
				input = append(input, rest...)
				input = append(input, fc.UserInstruction)
				return chatModel, input, nil
			},
		}
	}

	summ, err := summarization.New(ctx, &summarization.Config{
		Model: summModel,
		Trigger: &summarization.TriggerCondition{
			ContextTokens: triggerTokens,
		},
		TokenCounter: func(_ context.Context, input *summarization.TokenCounterInput) (int, error) {
			return countMessageTokens(input.Messages, input.Tools)
		},
		// The summary generation is a real provider call; surface it as a
		// CustomizedAction event so the mapper can account model.usage
		// (research P3 bridge ii: MaxModelCalls is not bypassed).
		EmitInternalEvents: true,
		Retry: &summarization.RetryConfig{
			MaxRetries: intPtr(0),
		},
		Failover: failover,
		Callback: func(ctx context.Context, before, after adk.TypedChatModelAgentState[*schema.Message]) error {
			beforeTokens, _ := countMessageTokens(before.Messages, before.ToolInfos)
			afterTokens, _ := countMessageTokens(after.Messages, after.ToolInfos)
			dropped := len(before.Messages) - len(after.Messages)
			if dropped < 0 {
				dropped = 0
			}
			emitCompactionEvent(ctx, "summarization", beforeTokens, afterTokens, dropped, keepRecent)
			return nil
		},
	})
	if err != nil {
		return nil, fmt.Errorf("runtime: summarization middleware: %w", err)
	}
	return []adk.ChatModelAgentMiddleware{newContextMonitor(), reducer, summ}, nil
}

// emitCompactionEvent publishes a durable context.compacted event through
// the run's governance sink (installed by Service.drive). Outside a run the
// sink is nil and the event is a no-op.
func emitCompactionEvent(ctx context.Context, mode string, beforeTokens, afterTokens, dropped, retention int) {
	emitGovernanceEvent(ctx, GovernanceEvent{
		Type:            domain.EventContextCompacted,
		Mode:            mode,
		BeforeTokens:    beforeTokens,
		AfterTokens:     afterTokens,
		DroppedMessages: dropped,
		RetentionSuffix: retention,
	})
}

func intPtr(v int) *int { return &v }
