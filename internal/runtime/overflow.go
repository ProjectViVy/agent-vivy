package runtime

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strings"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
)

// Context-overflow recovery (VCP-D2, spec §5.5): the pinned Eino ADK offers
// exactly this seam — ChatModelAgentConfig.ModelRetryConfig / ShouldRetry
// can inspect a failed model call, rewrite the input via
// RetryDecision.ModifiedInputMessages, and persist the rewrite into run
// state (PersistModifiedInputMessages). The wrapper emits WillRetryError
// events (mapped to provider.retry) and fails the run honestly once
// MaxRetries is exhausted. Inspected surface: adk/retry_chatmodel.go
// (TypedRetryContext, TypedRetryDecision), adk/chatmodel.go
// (ModelRetryConfig wiring, retry wrapper position inside the model chain),
// adk/failover_chatmodel.go (not used — it swaps models, not context).
//
// Recovery performs one durable CompactSession-equivalent fold of the
// session's pre-run history (current-run rows are never folded — the
// pending turn stays verbatim) and retries the model call once. A second
// overflow propagates as the run's terminal failure.

// overflowRecoveryReason tags a retry decision so the mapper can mark the
// run as mid-overflow-recovery for the auto_retry.* projection.
const overflowRecoveryReason = "context_overflow"

// ErrContextOverflow marks a classified provider context overflow. It is
// never user-facing; it only steers the retry decision and the terminal
// cause wording.
var ErrContextOverflow = errors.New("runtime: provider reported context overflow")

// overflowPatterns adapt pi-mono's OVERFLOW_PATTERNS (packages/ai/src/utils/
// overflow.ts) to the providers Vivy actually serves (Anthropic + OpenAI
// families) plus the generic fallbacks pi keeps for compatible endpoints.
var overflowPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)prompt (?:is )?too long`),
	regexp.MustCompile(`(?i)prompt exceeds max length`),
	regexp.MustCompile(`(?i)request_too_large`),
	regexp.MustCompile(`(?i)exceeds the context window`),
	regexp.MustCompile(`(?i)exceeds (?:the )?(?:model'?s )?maximum context length`),
	regexp.MustCompile(`(?i)exceeds (?:the )?maximum allowed input length`),
	regexp.MustCompile(`(?i)context[_ ]length[_ ]exceeded`),
	regexp.MustCompile(`(?i)maximum context length is`),
	regexp.MustCompile(`(?i)maximum prompt length is`),
	regexp.MustCompile(`(?i)input token count.*exceeds`),
	regexp.MustCompile(`(?i)input is too long for requested model`),
	regexp.MustCompile(`(?i)reduce the length of the messages`),
	regexp.MustCompile(`(?i)model_context_window_exceeded`),
	regexp.MustCompile(`(?i)exceeds the limit of`),
	regexp.MustCompile(`(?i)exceeded model token limit`),
	regexp.MustCompile(`(?i)token limit exceeded`),
	regexp.MustCompile(`(?i)too many tokens`),
	regexp.MustCompile(`(?i)exceeds the available context size`),
	regexp.MustCompile(`(?i)greater than the context length`),
	regexp.MustCompile(`(?i)context window exceeds limit`),
	regexp.MustCompile(`(?i)range of input length should be`),
	regexp.MustCompile(`(?i)prompt has [\d,]+ tokens?, but the configured context size is`),
}

// nonOverflowPatterns exclude provider errors that share overflow wording
// but are transient (pi's NON_OVERFLOW_PATTERNS subset).
var nonOverflowPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)rate limit`),
	regexp.MustCompile(`(?i)too many requests`),
	regexp.MustCompile(`(?i)throttl`),
}

// isContextOverflow classifies a provider error chain as a context-window
// overflow. The whole unwrap chain is searched, bounded, so wrapped runtime
// errors still match.
func isContextOverflow(err error) bool {
	text := errorChainText(err, 4096)
	if text == "" {
		return false
	}
	for _, p := range nonOverflowPatterns {
		if p.MatchString(text) {
			return false
		}
	}
	for _, p := range overflowPatterns {
		if p.MatchString(text) {
			return true
		}
	}
	return false
}

func errorChainText(err error, maxLen int) string {
	var b strings.Builder
	for err != nil && b.Len() < maxLen {
		if b.Len() > 0 {
			b.WriteString(" | ")
		}
		b.WriteString(err.Error())
		err = errors.Unwrap(err)
	}
	out := b.String()
	if len(out) > maxLen {
		out = out[:maxLen]
	}
	return out
}

// isEmptyLengthStop detects pi's "case 3" overflow: the server truncated an
// oversized input, finished with stop_reason "length", and produced no
// usable output (Xiaomi MiMo behavior; the same guard on Anthropic/OpenAI).
func isEmptyLengthStop(msg *schema.Message) bool {
	return msg != nil && msg.ResponseMeta != nil &&
		msg.ResponseMeta.FinishReason == "length" &&
		len(msg.ToolCalls) == 0 &&
		strings.TrimSpace(msg.Content) == "" &&
		strings.TrimSpace(msg.ReasoningContent) == ""
}

// overflowRetryDecision is the service-side ModelRetryConfig.ShouldRetry
// decider. Returning nil accepts the call as-is; an overflow decision folds
// durable session history, replaces the call input with the compacted feed,
// and tags the retry for the auto_retry.* projection.
func (s *Service) overflowRetryDecision(ctx context.Context, rc *adk.RetryContext) *adk.RetryDecision {
	// A joined provider failure may still contain recognizable overflow text,
	// but a failed mandatory Journal closure must never replay that call.
	if rc != nil && isModelSettlementFailure(rc.Err) {
		return nil
	}
	if s.engine == nil || s.engine.cfg.Compaction == nil || !s.engine.cfg.Compaction.Enabled {
		return nil
	}
	overflow := rc.Err != nil && isContextOverflow(rc.Err)
	if !overflow && rc.Err == nil && isEmptyLengthStop(rc.OutputMessage) {
		overflow = true
	}
	if !overflow {
		return nil
	}
	sessionID, runID := contextSessionID(ctx), contextRunID(ctx)
	if sessionID == "" || runID == "" {
		return nil
	}
	s.mu.Lock()
	if s.overflowRecovered[runID] {
		// One recovery per run: a second overflow fails honestly instead of
		// compacting-and-retrying in a loop.
		s.mu.Unlock()
		return nil
	}
	s.overflowRecovered[runID] = true
	s.overflowAwaiting[runID] = rc.RetryAttempt
	s.mu.Unlock()

	emitGovernanceEvent(ctx, GovernanceEvent{
		Type:    domain.EventAutoRetryStarted,
		Attempt: rc.RetryAttempt,
		Reason:  overflowRecoveryReason + ": " + errorChainText(rc.Err, 256),
	})
	finish := func(ok bool) {
		emitGovernanceEvent(ctx, GovernanceEvent{Type: domain.EventAutoRetryFinished, Attempt: rc.RetryAttempt, Success: ok})
	}

	rec, tail, err := s.compactForOverflow(ctx, sessionID, runID)
	if err != nil {
		slog.Warn("overflow recovery compaction failed; failing run", "run", string(runID), "err", err)
		finish(false)
		return nil
	}
	if rec == nil || len(rc.InputMessages) == 0 {
		// Nothing foldable in pre-run history: retrying the identical input
		// would overflow again. Fail with the original error.
		finish(false)
		return nil
	}
	// The retried call's first streamed event (or its own failure) emits
	// auto_retry.finished from the consume loop via overflowAwaiting — the
	// decider only reports that the recovery attempt was dispatched.
	modified := make([]*schema.Message, 0, 2+len(tail))
	modified = append(modified, rc.InputMessages[0]) // per-run preamble stays first
	modified = append(modified, schema.UserMessage(compactionSummaryPrefix+rec.Summary))
	modified = append(modified, tail...)
	return &adk.RetryDecision{
		Retry:                        true,
		ModifiedInputMessages:        modified,
		PersistModifiedInputMessages: true,
		RejectReason:                 overflowRecoveryReason,
	}
}

// compactForOverflow runs the durable session fold against the session's
// pre-run history only. Rows belonging to the live run (pending turn) are
// never folded — they stay verbatim after the summary. The returned tail is
// the post-fold feed the retry should run on: kept pre-run tail plus the
// pending turn's projected rows.
func (s *Service) compactForOverflow(ctx context.Context, sessionID domain.SessionID, runID domain.RunID) (*storage.SessionCompaction, []*schema.Message, error) {
	if s.deps.Messages == nil || s.deps.Compactions == nil {
		return nil, nil, ErrCompactionNotWired
	}
	stored, err := s.deps.Messages.ListMessages(ctx, sessionID)
	if err != nil {
		return nil, nil, fmt.Errorf("runtime: list session messages: %w", err)
	}
	var preRun, pending []domain.Message
	for _, msg := range stored {
		if msg.RunID == runID {
			pending = append(pending, msg)
		} else {
			preRun = append(preRun, msg)
		}
	}
	feed := feedableMessages(preRun)
	if len(feed) == 0 {
		return nil, nil, nil
	}
	keep := s.engine.cfg.Compaction.KeepRecent
	if keep <= 0 {
		keep = 12
	}
	foldIdx := len(feed) - keep
	if foldIdx < 0 {
		foldIdx = 0
	}
	foldIdx = fileContextSafeFoldIndex(feed, foldIdx)
	foldIdx = timestampSafeFoldIndex(feed, foldIdx)
	if foldIdx == 0 {
		return nil, nil, nil
	}
	outcome, err := s.foldToCompactionRecord(ctx, sessionID, feed, foldIdx, stored, "")
	if err != nil {
		return nil, nil, err
	}
	emitCompactionEvent(ctx, "overflow-recovery", outcome.beforeTokens, outcome.afterTokens, foldIdx, keep)
	s.recordLastCompaction(sessionID, &LastCompaction{
		Mode:         "overflow-recovery",
		BeforeTokens: outcome.beforeTokens,
		AfterTokens:  outcome.afterTokens,
		At:           outcome.rec.CreatedAt,
	})

	// Retry input tail: the kept pre-run rows re-checked for pairs split at
	// the fold seam, plus the pending turn projected verbatim — pending tool
	// calls whose results have not journaled yet must keep their tool_calls
	// rows or the ToolNode's later results would orphan.
	tail := projectFeed(feedableMessages(feed[foldIdx:]))
	tail = append(tail, projectFeed(pending)...)
	return &outcome.rec, tail, nil
}
