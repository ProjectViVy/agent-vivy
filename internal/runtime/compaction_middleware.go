package runtime

import (
	"context"
	"fmt"
	"path"
	"strings"
	"time"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/adk/middlewares/reduction"
	"github.com/cloudwego/eino/adk/middlewares/summarization"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/google/uuid"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/tools"
)

// compaction middleware wiring (research AGENT-LOOP-PORT-COMPARISON §4.3-E2):
// official Eino `reduction` (deterministic clear of old tool turns) runs
// first, then official `summarization` (LLM summary) when the feed is still
// over the trigger. Both are the typed (agentic-generation) variants, so the
// whole chain stays on schema.AgenticMessage. A tiny let-us-own ContextMonitor
// records the token count before any rewrite so the official hooks can report
// before/after sizes in a Vivy context.compacted event through the run's
// governance sink.

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
	*adk.TypedBaseChatModelAgentMiddleware[*schema.AgenticMessage]
}

func newContextMonitor() adk.TypedChatModelAgentMiddleware[*schema.AgenticMessage] {
	return &contextMonitor{TypedBaseChatModelAgentMiddleware: &adk.TypedBaseChatModelAgentMiddleware[*schema.AgenticMessage]{}}
}

func (m *contextMonitor) BeforeModelRewriteState(ctx context.Context, state *adk.TypedChatModelAgentState[*schema.AgenticMessage], mc *adk.TypedModelContext[*schema.AgenticMessage]) (context.Context, *adk.TypedChatModelAgentState[*schema.AgenticMessage], error) {
	tokens, err := countMessageTokens(state.Messages, state.ToolInfos)
	if err != nil {
		return nil, nil, err
	}
	return withCompactionBefore(ctx, compactionBefore{tokens: tokens, message: len(state.Messages)}), state, nil
}

// countMessageTokens is the shared Vivy token estimate (1 token ≈ 4 bytes),
// applied to the in-run agentic message list exactly like compaction.Measure.
func countMessageTokens(msgs []*schema.AgenticMessage, tools []*schema.ToolInfo) (int, error) {
	total := 0
	for _, msg := range msgs {
		if msg == nil {
			continue
		}
		total += len(agenticMessageText(msg)) + len(agenticReasoningText(msg)) + 16
		for _, call := range agenticToolCalls(msg) {
			total += len(call.CallID) + len(call.Name) + len(call.Arguments)
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

// clearOffloadDir anchors the reduction clear offloads inside the run
// workspace, one directory read_file can always reach.
const clearOffloadDir = "compaction"

// genClearOffloadPath mirrors Eino's default clear-offload path
// (RootDir/clear/<call-id>) but anchors it at a workspace-relative,
// forward-slash location so the persisted-output placeholder names a path
// read_file can open on every platform. Provider-issued call ids become the
// file name, so anything outside a bounded safe token set (traversal,
// separators, empty) falls back to a generated id.
func genClearOffloadPath(_ context.Context, detail *reduction.ToolDetail) (string, error) {
	if detail != nil && detail.ToolContext != nil {
		if id := safeOffloadCallID(detail.ToolContext.CallID); id != "" {
			return path.Join(clearOffloadDir, "clear", id), nil
		}
	}
	return path.Join(clearOffloadDir, "clear", uuid.NewString()), nil
}

func safeOffloadCallID(callID string) string {
	if callID == "" || len(callID) > 128 {
		return ""
	}
	for _, r := range callID {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
		default:
			return ""
		}
	}
	return callID
}

// buildCompactionHandlers assembles the official Eino typed middlewares with
// Vivy bridges. When the engine has no run context (e.g. Checkpoints nil in
// unit tests), the event hooks are inert: they only fire when the run ctx
// carries a governance sink, which the service always installs.
func buildCompactionHandlers(ctx context.Context, chatModel model.BaseModel[*schema.AgenticMessage], summaryModel model.BaseModel[*schema.AgenticMessage], policy CompactionPolicy, feedBudgetBytes int, offload *EinoFilesystemBackend) ([]adk.TypedChatModelAgentMiddleware[*schema.AgenticMessage], error) {
	triggerTokens := policy.TriggerTokens(bytesToTokens(feedBudgetBytes))
	if triggerTokens <= 0 {
		triggerTokens = policy.TriggerTokens(0)
	}
	keepRecent := policy.KeepRecent
	if keepRecent <= 0 {
		keepRecent = 12
	}
	// A typed-nil *EinoFilesystemBackend must not reach the middleware as a
	// non-nil interface: eino would treat clear as offloading and every
	// write would fail on the nil receiver.
	var offloadBackend reduction.Backend
	if offload != nil {
		offloadBackend = offload
	}

	reducer, err := reduction.NewTyped(ctx, &reduction.TypedConfig[*schema.AgenticMessage]{
		// Vivy's tooladapter already bounds every single result
		// (MaxToolResultBytes); the Eino truncation/offload phase would
		// double-truncate, so keep only the deterministic clear phase.
		SkipTruncation:            true,
		SkipClear:                 false,
		Backend:                   offloadBackend,
		MaxTokensForClear:         int64(triggerTokens),
		ClearRetentionSuffixLimit: keepRecent,
		// CMP-1: with a backend wired, cleared tool results are written into
		// the run workspace and the placeholder names the path so read_file
		// can recover the content later in the same run.
		GenClearOffloadFilePath: genClearOffloadPath,
		ReadFileToolName:        tools.ReadFileName,
		TokenCounter: func(_ context.Context, msgs []*schema.AgenticMessage, tools []*schema.ToolInfo) (int64, error) {
			n, err := countMessageTokens(msgs, tools)
			return int64(n), err
		},
		ClearPostProcess: func(ctx context.Context, state *adk.TypedChatModelAgentState[*schema.AgenticMessage]) context.Context {
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
	// Summary calls run through the call-observation seam too (OBS-02):
	// both the primary (or the main model standing in for it) and the
	// failover wrapper journal their own v3 request / v2 usage / finish
	// records with source "summary".
	summModel := observeSummaryModel(chatModel, false)
	var failover *summarization.TypedFailoverConfig[*schema.AgenticMessage]
	if summaryModel != nil {
		summModel = observeSummaryModel(summaryModel, false)
		failover = &summarization.TypedFailoverConfig[*schema.AgenticMessage]{
			MaxRetries: intPtr(1),
			BackoffFunc: func(context.Context, int, *schema.AgenticMessage, error) time.Duration {
				return 0
			},
			GetFailoverModel: func(_ context.Context, fc *summarization.TypedFailoverContext[*schema.AgenticMessage]) (model.BaseModel[*schema.AgenticMessage], []*schema.AgenticMessage, error) {
				// Mirror the middleware's default input: the original
				// leading system messages are dropped (the summary
				// instruction is re-supplied) and the rest is sandwiched
				// between the middleware's system/user instructions.
				input := make([]*schema.AgenticMessage, 0, len(fc.OriginalMessages)+2)
				input = append(input, fc.SystemInstruction)
				rest := fc.OriginalMessages
				for len(rest) > 0 && rest[0] != nil && rest[0].Role == schema.AgenticRoleTypeSystem {
					rest = rest[1:]
				}
				input = append(input, rest...)
				input = append(input, fc.UserInstruction)
				return observeSummaryModel(chatModel, true), input, nil
			},
		}
	}

	summ, err := summarization.NewTyped(ctx, &summarization.TypedConfig[*schema.AgenticMessage]{
		Model: summModel,
		Trigger: &summarization.TriggerCondition{
			ContextTokens: triggerTokens,
		},
		TokenCounter: func(_ context.Context, input *summarization.TypedTokenCounterInput[*schema.AgenticMessage]) (int, error) {
			return countMessageTokens(input.Messages, input.Tools)
		},
		// The summary generation is a real provider call; surface it as a
		// CustomizedAction event so the mapper can account model.usage
		// (research P3 bridge ii: MaxModelCalls is not bypassed).
		EmitInternalEvents: true,
		Retry: &summarization.TypedRetryConfig[*schema.AgenticMessage]{
			MaxRetries: intPtr(0),
		},
		Failover: failover,
		Finalize: preserveProjectFileContextsFinalize,
		Callback: func(ctx context.Context, before, after adk.TypedChatModelAgentState[*schema.AgenticMessage]) error {
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
	return []adk.TypedChatModelAgentMiddleware[*schema.AgenticMessage]{newContextMonitor(), reducer, summ}, nil
}

// preserveProjectFileContextsFinalize applies Eino's canonical summary
// formatting, then appends every exact project-file message from the
// original state. Summaries are intentionally lossy; captured source text is
// not. Keeping those bounded messages makes the snapshot available to later
// model calls in the same agent run.
func preserveProjectFileContextsFinalize(ctx context.Context, original []*schema.AgenticMessage, summary *schema.AgenticMessage) ([]*schema.AgenticMessage, error) {
	finalized, err := summarization.DefaultFinalize(ctx, original, summary)
	if err != nil {
		return nil, err
	}
	for _, msg := range original {
		if agenticMessageHasProjectFileContext(msg) {
			finalized = append(finalized, msg)
		}
	}
	return finalized, nil
}

func agenticMessageHasProjectFileContext(msg *schema.AgenticMessage) bool {
	if msg == nil || msg.Role != schema.AgenticRoleTypeUser {
		return false
	}
	for _, block := range msg.ContentBlocks {
		if block == nil || block.UserInputText == nil {
			continue
		}
		if strings.HasPrefix(block.UserInputText.Text, "\n\n[project file: ") {
			return true
		}
	}
	return false
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
