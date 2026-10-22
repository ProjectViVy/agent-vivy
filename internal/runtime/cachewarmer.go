package runtime

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	einoclaude "github.com/cloudwego/eino-ext/components/model/agenticclaude"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	"agent-vivy/internal/domain"
)

// Prompt-cache warming (VCP F2, pi cache warming). Anthropic-family
// providers keep a server-side prompt cache with a short lifetime
// (ephemeral = 300s); a long pause between turns lets it expire and the
// next turn re-reads the full context at uncached prices. The warmer
// spends one minimal request — the run's stable prefix (static
// instruction + selected tool schemas) plus a throwaway user marker —
// so the adapter's AutoCacheControl breakpoints refresh the cached
// system/tools blocks. The response never enters the transcript; paid work
// uses the existing model-call budget and mandatory maintenance lifecycle.
//
// runtime.cache_warming selects the trigger: "off" disables the
// scheduler; "streaming" refreshes after each settled model call;
// "idle" refreshes once the cache approaches its declared lifetime.
// runtime.cache_warming_min_savings gates each warm on the avoided
// re-read cost (input price x last prompt tokens; a token floor stands
// in when the model is unpriced).

const (
	cacheWarmModeOff       = "off"
	cacheWarmModeStreaming = "streaming"
	cacheWarmModeIdle      = "idle"

	// cacheWarmPrompt is the fixed trailing marker of a warm request. It
	// sits after the cache breakpoint, so its content is irrelevant to
	// what gets refreshed and the reply is discarded.
	cacheWarmPrompt = "Reply with one word: ok."
	// cacheWarmMaxTokens keeps the discarded reply minimal; Anthropic
	// requires max_tokens >= 1 and the marker needs only a few tokens.
	cacheWarmMaxTokens = 16
	// cacheWarmCallTimeout bounds one warm attempt; it is the outer cap,
	// the run context still wins when the run is cancelled first.
	cacheWarmCallTimeout = 60 * time.Second
	// cacheWarmTokenFloor is the savings proxy for unpriced models: below
	// it a warm costs more than the uncached re-read it avoids.
	cacheWarmTokenFloor = 2048
	// cacheWarmIdleHeadroom refreshes the cache at this fraction of the
	// declared lifetime in idle mode, leaving margin before expiry.
	cacheWarmIdleHeadroom = 0.8
)

// runCacheWarmer is bound to one run's model-call stream: it sees each
// settled call's usage sample (its prompt size feeds the savings gate)
// and schedules warm requests that cancel with the run. The scheduler
// exists only when config asks for warming AND the model declares
// supports_warming + a cache lifetime.
type runCacheWarmer struct {
	svc       *Service
	m         *eventMapper
	sessionID domain.SessionID
	eng       *Engine
	runCtx    context.Context
	mode      string
	minUSD    float64
	info      domain.ModelInfo
	tools     []*schema.ToolInfo
	ledger    *BudgetLedger
	cancel    context.CancelFunc

	mu       sync.Mutex
	timer    *time.Timer
	inflight bool
	closed   bool
	failed   error
	wg       sync.WaitGroup
}

type runCacheWarmerKey struct{}

func withRunCacheWarmer(ctx context.Context, warmer *runCacheWarmer) context.Context {
	if warmer == nil {
		return ctx
	}
	ctx = context.WithValue(ctx, runCacheWarmerKey{}, warmer)
	warmer.cancel()
	warmer.runCtx, warmer.cancel = context.WithCancel(ctx)
	return ctx
}

func drainRunCacheWarmer(ctx context.Context) error {
	warmer, _ := ctx.Value(runCacheWarmerKey{}).(*runCacheWarmer)
	return warmer.stop()
}

// stop closes admission before waiting: a timer racing shutdown cannot add
// another paid call, and the owning drive/resume stays alive until settlement.
func (w *runCacheWarmer) stop() error {
	if w == nil {
		return nil
	}
	w.mu.Lock()
	w.closed = true
	if w.timer != nil {
		w.timer.Stop()
	}
	w.cancel()
	w.mu.Unlock()
	w.wg.Wait()
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.failed
}

func (w *runCacheWarmer) recordFailure(err error) {
	if err == nil {
		return
	}
	w.mu.Lock()
	w.failed = errors.Join(w.failed, err)
	w.mu.Unlock()
	// Wake the existing Run owner even if its foreground provider is blocked.
	// The owner drains this call and publishes the failure; maintenance never
	// emits a terminal from its own goroutine.
	w.svc.mu.Lock()
	cancelRun := w.svc.active[w.m.runID]
	w.svc.mu.Unlock()
	if cancelRun != nil {
		cancelRun()
	}
}

// newRunCacheWarmer binds the scheduler; nil when warming cannot apply
// (mode off, no engine, model not warmable, missing lifetime).
func (s *Service) newRunCacheWarmer(runCtx context.Context, m *eventMapper, sessionID domain.SessionID, eng *Engine, selected []string) *runCacheWarmer {
	mode := s.deps.CacheWarmingMode
	if mode == "" {
		mode = cacheWarmModeStreaming
	}
	if mode == cacheWarmModeOff || eng == nil || eng.chatModel == nil || m == nil {
		return nil
	}
	info := s.GetModelInfo(runCtx)
	if !info.SupportsWarming || info.CacheLifetimeSeconds <= 0 {
		return nil
	}
	infos := make([]*schema.ToolInfo, 0, len(selected))
	for _, name := range selected {
		if ti, ok := eng.toolInfos[name]; ok && ti != nil {
			infos = append(infos, ti)
		}
	}
	warmCtx, cancel := context.WithCancel(runCtx)
	return &runCacheWarmer{
		svc: s, m: m, sessionID: sessionID, eng: eng, runCtx: warmCtx, cancel: cancel,
		mode: mode, minUSD: s.deps.CacheWarmingMinSavingsUSD,
		info: info, tools: infos,
	}
}

// settled observes one finished model call (main/child routes only, and
// only on success). Streaming mode warms immediately; idle mode arms the
// refresh timer for just before the cache lifetime expires.
func (w *runCacheWarmer) settled(sample normalizedUsageSample) error {
	if w == nil {
		return nil
	}
	w.mu.Lock()
	closed := w.closed
	w.mu.Unlock()
	if closed {
		return nil
	}
	if reason := w.gate(sample.PromptTokens); reason != "" {
		return w.journal(w.mode, "skipped", reason, 0, 0)
	}
	switch w.mode {
	case cacheWarmModeIdle:
		w.armIdle()
	default: // streaming
		w.schedule(cacheWarmModeStreaming)
	}
	return nil
}

// gate returns "" when a warm is worth its cost, else the skip reason.
func (w *runCacheWarmer) gate(promptTokens int) string {
	if w.info.InputPerMTokens > 0 {
		savings := float64(promptTokens) * w.info.InputPerMTokens / 1e6
		if savings < w.minUSD {
			return "below_min_savings"
		}
		return ""
	}
	// Unpriced model: no cost basis, fall back to the token proxy.
	if promptTokens < cacheWarmTokenFloor {
		return "below_token_floor"
	}
	return ""
}

// armIdle (re)arms the refresh timer for just before the declared
// lifetime; each settled call slides the deadline, so a warm only fires
// after the model has actually been quiet that long.
func (w *runCacheWarmer) armIdle() {
	delay := time.Duration(float64(w.info.CacheLifetimeSeconds)*cacheWarmIdleHeadroom) * time.Second
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return
	}
	if w.timer != nil {
		w.timer.Stop()
	}
	w.timer = time.AfterFunc(delay, func() { w.schedule(cacheWarmModeIdle) })
}

// schedule fires one warm unless another is already in flight.
func (w *runCacheWarmer) schedule(mode string) {
	w.mu.Lock()
	if w.inflight || w.closed || w.runCtx.Err() != nil {
		w.mu.Unlock()
		return
	}
	w.inflight = true
	w.wg.Add(1)
	w.mu.Unlock()
	go func() { defer w.wg.Done(); w.warm(mode) }()
}

// warm uses native Generate with the same run observer and budget owner.
// Its maintenance observer has no warmer and cannot recursively schedule work.
func (w *runCacheWarmer) warm(mode string) {
	defer func() {
		w.mu.Lock()
		w.inflight = false
		w.mu.Unlock()
	}()
	// The scheduler cancels with the run: a warm queued behind run
	// termination silently drops instead of journaling a false failure.
	if w.runCtx.Err() != nil {
		return
	}
	callCtx, cancel := context.WithTimeout(w.runCtx, cacheWarmCallTimeout)
	defer cancel()
	opts := []model.Option{model.WithMaxTokens(cacheWarmMaxTokens)}
	if len(w.tools) > 0 {
		opts = append(opts, model.WithTools(w.tools))
	}
	msgs := []*schema.AgenticMessage{
		agenticSystemMessage(w.eng.instruction),
		agenticUserMessage(cacheWarmPrompt),
	}
	core := observedModelCallCore{route: modelCallRoute{Source: modelCallSourceMaintenance}}
	resp, err := observeGenerate(callCtx, &core, w.eng.chatModel, msgs, opts...)
	if err != nil {
		// Closing a leg can cancel an unadmitted queued request between the
		// initial check and Begin. No paid work exists to settle in that case.
		if errors.Is(err, errModelCallAdmission) && errors.Is(err, context.Canceled) && w.runCtx.Err() != nil {
			return
		}
		if errors.Is(err, ErrBudgetExceeded) {
			w.recordFailure(w.journal(mode, "skipped", "budget_exceeded", 0, 0))
			return
		}
		if errors.Is(err, errModelCallObservation) {
			w.recordFailure(err)
		}
		// A run-cancelled warm is a silent drop, not a failure record;
		// every other error journals as the silent diagnostic.
		if w.runCtx.Err() != nil {
			return
		}
		w.recordFailure(w.journal(mode, "failed", fmt.Sprintf("%T", err), 0, 0))
		return
	}
	prompt, cacheWrite := 0, 0
	if usage := agenticUsage(resp); usage != nil {
		prompt = usage.PromptTokens
	}
	if resp != nil {
		if v, ok := einoclaude.GetCacheCreationInputTokens(resp); ok {
			cacheWrite = v
		}
	}
	w.recordFailure(w.journal(mode, "warmed", "", prompt, cacheWrite))
}

// journal writes the diagnostic event on a detached, bounded context so
// a cancelled run cannot strand the record — the same rule the call
// settlement follows.
func (w *runCacheWarmer) journal(mode, status, reason string, promptTokens, cacheWrite int) error {
	if w.ledger != nil {
		if err := w.ledger.ReserveEvent(); err != nil {
			if errors.Is(err, ErrBudgetExceeded) {
				return nil
			}
			return err
		}
	}
	payload := payloadCacheWarmed{
		Provider:         w.m.runProvider,
		Model:            w.m.runModel,
		Mode:             mode,
		Status:           status,
		Reason:           reason,
		PromptTokens:     promptTokens,
		CacheWriteTokens: cacheWrite,
	}
	persistCtx, cancel := context.WithTimeout(context.WithoutCancel(w.runCtx), terminalPersistTimeout)
	defer cancel()
	return w.svc.persistAndPublishEvent(persistCtx, w.sessionID, w.m.build(domain.EventCacheWarmed, payload), false)
}
