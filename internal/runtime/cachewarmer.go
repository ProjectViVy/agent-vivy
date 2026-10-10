package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	einoclaude "github.com/cloudwego/eino-ext/components/model/claude"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	"agent-vivy/internal/domain"
)

// Cache warming is optional maintenance during an active run. Its exact
// system/tool prefix comes from the observed call, never a stale Engine
// instruction or the unrelated conversation token count. All provider calls
// use the mandatory observation/accounting lifecycle; replies are discarded.
const (
	cacheWarmModeOff           = "off"
	cacheWarmModeStreaming     = "streaming"
	modelCallSourceMaintenance = "maintenance"
	cacheWarmPrompt            = "Reply with one word: ok."
	cacheWarmMaxTokens         = 16
	cacheWarmCallTimeout       = 60 * time.Second
)

type runCacheWarmer struct {
	svc       *Service
	m         *eventMapper
	sessionID domain.SessionID
	eng       *Engine
	runCtx    context.Context
	ledger    *BudgetLedger
	minUSD    float64
	info      domain.ModelInfo
	mu        sync.Mutex
	inflight  bool
}

func (s *Service) newRunCacheWarmer(runCtx context.Context, m *eventMapper, sessionID domain.SessionID, eng *Engine, ledger *BudgetLedger) *runCacheWarmer {
	// Empty dependency config is conservative too. There is no timer/service
	// owner beyond the run; the previous idle mode could not warm between runs.
	if s.deps.CacheWarmingMode != cacheWarmModeStreaming || eng == nil || eng.chatModel == nil || m == nil {
		return nil
	}
	info := s.GetModelInfo(runCtx)
	if !info.SupportsWarming || info.CacheLifetimeSeconds <= 0 {
		return nil
	}
	return &runCacheWarmer{svc: s, m: m, sessionID: sessionID, eng: eng, runCtx: runCtx, ledger: ledger, minUSD: s.deps.CacheWarmingMinSavingsUSD, info: info}
}

func (w *runCacheWarmer) settled(_ normalizedUsageSample, in modelCallInput) error {
	if w == nil {
		return nil
	}
	// Only the leading system messages are reusable. No history/user/tool
	// results are copied, and selected tools are the exact bound invocation set.
	prefix := modelCallInput{Tools: append([]*schema.ToolInfo(nil), in.Tools...)}
	for _, msg := range in.Messages {
		if msg == nil || msg.Role != schema.System {
			break
		}
		copy := *msg
		prefix.Messages = append(prefix.Messages, &copy)
	}
	estimated, err := countMessageTokens(prefix.Messages, prefix.Tools)
	if err != nil {
		w.journal(payloadCacheWarmed{Status: "skipped", Reason: "prefix_estimate_failed"})
		return nil
	}
	evidence, err := json.Marshal(prefix)
	if err != nil {
		w.journal(payloadCacheWarmed{Status: "skipped", Reason: "prefix_digest_failed"})
		return nil
	}
	diagnostic := payloadCacheWarmed{PrefixSHA256: sha256Hex(evidence), EstimatedPrefixTokens: estimated}
	if len(prefix.Messages) == 0 {
		diagnostic.Status = "skipped"
		diagnostic.Reason = "no_reusable_prefix"
		w.journal(diagnostic)
		return nil
	}
	if reason := w.gate(estimated); reason != "" {
		diagnostic.Status = "skipped"
		diagnostic.Reason = reason
		w.journal(diagnostic)
		return nil
	}
	// End can settle different calls concurrently. Coalesce only overlapping
	// refreshes on this warmer, without waiting or admitting duplicate work.
	w.mu.Lock()
	if w.inflight {
		w.mu.Unlock()
		return nil
	}
	w.inflight = true
	w.mu.Unlock()
	defer func() { w.mu.Lock(); w.inflight = false; w.mu.Unlock() }()
	// Settle within the owning call before the run can seal its Journal.
	// Opt-in maintenance adds a bounded provider round trip to this boundary.
	return w.warm(prefix, diagnostic)
}

// This estimate is only the gross price difference of ONE possible future
// read of this prefix. It excludes the cost of warming and does not predict
// future reuse, cache retention, a hit, or positive net savings.
func (w *runCacheWarmer) gate(prefixTokens int) string {
	if w.minUSD <= 0 {
		return ""
	}
	if w.info.InputPerMTokens <= 0 || w.info.CachedInputPerMTokens <= 0 {
		return "unpriced_prefix"
	}
	gross := float64(prefixTokens) * (w.info.InputPerMTokens - w.info.CachedInputPerMTokens) / 1e6
	if gross < w.minUSD {
		return "below_min_savings"
	}
	return ""
}

func (w *runCacheWarmer) warm(prefix modelCallInput, diagnostic payloadCacheWarmed) error {
	if w.runCtx.Err() != nil {
		return nil
	}
	callCtx, cancel := context.WithTimeout(w.runCtx, cacheWarmCallTimeout)
	defer cancel()
	// A dedicated observer shares the owning run's Journal and budget. No
	// warmer is attached, so maintenance settlement cannot trigger recursion.
	observer := &runModelCallObserver{svc: w.svc, m: w.m, sessionID: w.sessionID, ledger: w.ledger, source: modelCallSourceMaintenance, provider: w.m.runProvider, model: w.m.runModel, calls: map[string]*observedModelCall{}}
	opts := []model.Option{model.WithMaxTokens(cacheWarmMaxTokens)}
	if len(prefix.Tools) > 0 {
		opts = append(opts, model.WithTools(prefix.Tools))
	}
	msgs := append(append([]*schema.Message(nil), prefix.Messages...), schema.UserMessage(cacheWarmPrompt))
	// Capture the admitted call identity from Begin without a second lifecycle.
	tracked := &cacheWarmObserver{modelCallObserver: observer, callID: &diagnostic.CallID}
	callCtx = withModelCallObserverFactory(callCtx, func(modelCallRoute) modelCallObserver { return tracked })
	resp, err := observeChatModel(w.eng.chatModel).Generate(callCtx, msgs, opts...)
	if tracked.admissionDenied {
		diagnostic.Status = "skipped"
		diagnostic.Reason = "budget_exhausted"
		w.journal(diagnostic)
		return nil
	}
	if tracked.err != nil {
		return tracked.err
	}
	if err != nil {
		diagnostic.Status = "failed"
		diagnostic.Reason = fmt.Sprintf("%T", err)
		w.journal(diagnostic)
		return nil
	}
	diagnostic.Status = "warmed"
	if u := usageOfMessage(resp); u != nil {
		diagnostic.PromptTokens = u.PromptTokens
		diagnostic.CompletionTokens = u.CompletionTokens
		diagnostic.CachedTokens = u.PromptTokenDetails.CachedTokens
	}
	if resp != nil {
		if v, ok := einoclaude.GetCacheCreationInputTokens(resp); ok {
			diagnostic.CacheWriteTokens = v
		}
	}
	w.journal(diagnostic)
	return nil
}

type cacheWarmObserver struct {
	modelCallObserver
	callID          *string
	err             error
	admissionDenied bool
}

func (o *cacheWarmObserver) Begin(ctx context.Context, in modelCallInput) (modelCallMeta, error) {
	meta, err := o.modelCallObserver.Begin(ctx, in)
	if err == nil {
		*o.callID = meta.CallID
	} else {
		// No provider work was admitted. Optional maintenance must not turn
		// successful foreground settlement into a budget failure. Later usage
		// or finish failures remain mandatory because the call may already be paid.
		o.admissionDenied = errors.Is(err, ErrBudgetExceeded)
		o.err = err
	}
	return meta, err
}

func (o *cacheWarmObserver) Chunk(ctx context.Context, meta modelCallMeta, msg *schema.Message) error {
	err := o.modelCallObserver.Chunk(ctx, meta, msg)
	if err != nil {
		o.err = err
	}
	return err
}
func (o *cacheWarmObserver) End(ctx context.Context, meta modelCallMeta, result modelCallResult) error {
	err := o.modelCallObserver.End(ctx, meta, result)
	if err != nil {
		o.err = err
	}
	return err
}

func (w *runCacheWarmer) journal(payload payloadCacheWarmed) {
	// Diagnostics are optional Journal events, not settlement exemptions.
	// Lack of admission drops the marker without failing foreground work.
	if w.ledger != nil {
		if err := w.ledger.ReserveEvent(); err != nil {
			return
		}
	}
	payload.Provider = w.m.runProvider
	payload.Model = w.m.runModel
	payload.Mode = cacheWarmModeStreaming
	persistCtx, cancel := context.WithTimeout(context.WithoutCancel(w.runCtx), terminalPersistTimeout)
	defer cancel()
	_ = w.svc.persistAndPublish(persistCtx, w.sessionID, w.m.build(domain.EventCacheWarmed, payload))
}
