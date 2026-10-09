package runtime

import (
	"context"
	"encoding/json"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/provider"
	"agent-vivy/internal/storage/sqlite"
	"agent-vivy/internal/tools"
)

// cache-warming (VCP F2) tests. The warmer is bound per run inside drive;
// these tests run real runs against a scripted model whose second call
// blocks, so a settle-triggered warm has an open window to fire in.

// warmVendor declares a warmable model for the test catalog: priced high
// enough that a few-thousand-token prompt clears the default savings
// floor, with a controllable cache lifetime.
func warmVendor(lifetimeSeconds int) provider.Vendor {
	return provider.Vendor{
		Name: "test",
		Endpoints: []provider.Endpoint{{
			Adapter:      "anthropic-messages",
			BaseURL:      "http://warm.invalid",
			DefaultModel: "test-model",
			Models: []provider.Model{{
				ID:                   "test-model",
				InputPerMTok:         1000, // $1 per 1k tokens: 5k prompt = $5
				SupportsWarming:      true,
				CacheLifetimeSeconds: lifetimeSeconds,
			}},
		}},
	}
}

// warmSpyModel is a ToolCallingChatModel double: call 1 requests the
// echo_info tool and reports usage; every later real call blocks until
// release; a call whose last message is the warm marker counts itself
// and replies immediately with usage + a cache-write extra.
type warmSpyModel struct {
	release chan struct{}

	mu        sync.Mutex
	calls     int
	warmCalls int
}

var _ model.ToolCallingChatModel = (*warmSpyModel)(nil)

func newWarmSpyModel() *warmSpyModel {
	return &warmSpyModel{release: make(chan struct{})}
}

func (m *warmSpyModel) counts() (calls, warm int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.calls, m.warmCalls
}

func warmReply() *schema.Message {
	msg := &schema.Message{
		Role:    schema.Assistant,
		Content: "ok",
		ResponseMeta: &schema.ResponseMeta{Usage: &schema.TokenUsage{
			PromptTokens: 111, CompletionTokens: 2, TotalTokens: 113,
		}},
		Extra: map[string]any{},
	}
	// Mirrors einoclaude's keyOfCacheCreationInputTokens message extra so
	// the accounting fold exercises the real read path.
	msg.Extra["_eino_claude_cache_creation_input_tokens"] = 1234
	return msg
}

func (m *warmSpyModel) Generate(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	r, err := m.Stream(ctx, input, opts...)
	if err != nil {
		return nil, err
	}
	return schema.ConcatMessages(collectStream(r))
}

func (m *warmSpyModel) Stream(ctx context.Context, input []*schema.Message, _ ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	if n := len(input); n > 0 && input[n-1].Role == schema.User && input[n-1].Content == cacheWarmPrompt {
		m.mu.Lock()
		m.warmCalls++
		m.mu.Unlock()
		return streamOf(warmReply()), nil
	}
	m.mu.Lock()
	m.calls++
	idx := m.calls
	m.mu.Unlock()
	if idx == 1 {
		return streamOf(toolCallMessage("call_warm_1", tools.EchoInfoName, `{}`), usageText("", 5000, 10)), nil
	}
	select {
	case <-m.release:
		return streamOf(schema.AssistantMessage("done", nil), usageText("", 5000, 10)), nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (m *warmSpyModel) WithTools(_ []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	return m, nil
}

func streamOf(msgs ...*schema.Message) *schema.StreamReader[*schema.Message] {
	r, w := schema.Pipe[*schema.Message](8)
	go func() {
		defer w.Close()
		for _, c := range msgs {
			if w.Send(c, nil) {
				return
			}
		}
	}()
	return r
}

func newWarmService(t *testing.T, chat model.ToolCallingChatModel, mode string, minSavings float64, lifetime int) (*Service, *sqlite.Backend, *testSink) {
	t.Helper()
	ctx := context.Background()
	backend, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "journal.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = backend.Close() })
	ts, err := tools.Builtin(backend).Resolve([]string{tools.EchoInfoName})
	if err != nil {
		t.Fatalf("resolve tools: %v", err)
	}
	eng, err := NewEngine(ctx, chat, ts, EngineConfig{StreamBuffer: 8, MaxEventPayloadBytes: 64 << 10})
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}
	sink := newTestSink()
	svc := NewService(eng, "test", "test-model", ServiceDeps{
		Journal: backend, Runs: backend, Messages: backend, Sessions: backend, Sink: sink, Truncations: backend,
		CacheWarmingMode:          mode,
		CacheWarmingMinSavingsUSD: minSavings,
	})
	svc.SetCatalog(provider.NewCatalog(warmVendor(lifetime)))
	return svc, backend, sink
}

// waitForCacheWarmed polls the run's journal until a cache.warmed event
// with the wanted status appears (bounded so failures fail fast).
func waitForCacheWarmed(t *testing.T, backend *sqlite.Backend, runID domain.RunID, status string) payloadCacheWarmed {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		for _, ev := range journalEvents(t, backend, runID) {
			if ev.Type != domain.EventCacheWarmed {
				continue
			}
			var p payloadCacheWarmed
			if err := json.Unmarshal(ev.Payload, &p); err != nil {
				t.Fatalf("cache.warmed payload: %v", err)
			}
			if status == "" || p.Status == status {
				return p
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("no cache.warmed status=%q within deadline", status)
	return payloadCacheWarmed{}
}

func TestCacheWarmStreamingFiresAfterSettle(t *testing.T) {
	m := newWarmSpyModel()
	svc, backend, _ := newWarmService(t, m, "streaming", 0.05, 60)
	ctx := context.Background()
	mustCreateSession(t, backend, "warm-streaming")
	sessionID := domain.SessionID("warm-streaming")
	runID, err := svc.Run(ctx, sessionID, "warm me")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	payload := waitForCacheWarmed(t, backend, runID, "warmed")
	close(m.release)
	waitForRunStatus(t, backend, runID, domain.RunCompleted)

	calls, warm := m.counts()
	if warm < 1 {
		t.Fatalf("warm calls = %d, want >= 1 (every settle refreshes)", warm)
	}
	if calls < 2 {
		t.Fatalf("model calls = %d, want >= 2", calls)
	}
	if payload.Mode != "streaming" {
		t.Fatalf("warm mode = %q", payload.Mode)
	}
	// Warm usage folds into the diagnostic's cache-write bucket: prompt
	// tokens + Anthropic cache_creation_input_tokens are reported as
	// accounting, never as a model.usage lifecycle row.
	if payload.PromptTokens != 111 || payload.CacheWriteTokens != 1234 {
		t.Fatalf("warm accounting = prompt:%d write:%d, want 111/1234", payload.PromptTokens, payload.CacheWriteTokens)
	}
	// The warm request is unobserved: no model.request/model.call.finished
	// carries it, and no message or usage row grows beyond the real calls.
	kinds := classifyObservedCall(journalEvents(t, backend, runID))
	if len(kinds.requests) != calls {
		t.Fatalf("model.request count = %d, want %d (warm must stay unobserved)", len(kinds.requests), calls)
	}
	for _, ev := range journalEvents(t, backend, runID) {
		if ev.Type == domain.EventModelUsage {
			var probe payloadModelUsageV2
			if err := json.Unmarshal(ev.Payload, &probe); err == nil && probe.CacheWriteTokens != nil {
				t.Fatalf("warm leaked a cache_write_tokens usage row: %+v", probe)
			}
		}
	}
	msgs, err := backend.ListMessages(ctx, sessionID)
	if err != nil {
		t.Fatalf("list messages: %v", err)
	}
	for _, msg := range msgs {
		if msg.Content == cacheWarmPrompt {
			t.Fatal("warm prompt leaked into stored messages")
		}
	}
}

func TestCacheWarmIdleArmsBeforeLifetime(t *testing.T) {
	m := newWarmSpyModel()
	// 1s lifetime: idle warm fires ~0.8s after settle while call 2 blocks.
	svc, backend, _ := newWarmService(t, m, "idle", 0.05, 1)
	ctx := context.Background()
	mustCreateSession(t, backend, "warm-idle")
	sessionID := domain.SessionID("warm-idle")
	runID, err := svc.Run(ctx, sessionID, "warm me")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	payload := waitForCacheWarmed(t, backend, runID, "warmed")
	close(m.release)
	waitForRunStatus(t, backend, runID, domain.RunCompleted)
	if payload.Mode != "idle" {
		t.Fatalf("warm mode = %q, want idle", payload.Mode)
	}
	if _, warm := m.counts(); warm < 1 {
		t.Fatalf("warm calls = %d, want >= 1", warm)
	}
}

func TestCacheWarmSavingsFloorSkips(t *testing.T) {
	m := newWarmSpyModel()
	// $100 floor: a 5k-token prompt at $1/1k = $5 < floor → skipped.
	svc, backend, _ := newWarmService(t, m, "streaming", 100, 60)
	ctx := context.Background()
	mustCreateSession(t, backend, "warm-skip")
	sessionID := domain.SessionID("warm-skip")
	runID, err := svc.Run(ctx, sessionID, "warm me")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	payload := waitForCacheWarmed(t, backend, runID, "skipped")
	close(m.release)
	waitForRunStatus(t, backend, runID, domain.RunCompleted)
	if payload.Reason != "below_min_savings" {
		t.Fatalf("skip reason = %q, want below_min_savings", payload.Reason)
	}
	if _, warm := m.counts(); warm != 0 {
		t.Fatalf("warm calls = %d, want 0", warm)
	}
}

func TestCacheWarmOffMakesZeroCalls(t *testing.T) {
	m := newWarmSpyModel()
	svc, backend, _ := newWarmService(t, m, "off", 0.05, 60)
	ctx := context.Background()
	mustCreateSession(t, backend, "warm-off")
	sessionID := domain.SessionID("warm-off")
	runID, err := svc.Run(ctx, sessionID, "warm me")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	close(m.release)
	waitForRunStatus(t, backend, runID, domain.RunCompleted)
	if _, warm := m.counts(); warm != 0 {
		t.Fatalf("warm calls = %d, want 0", warm)
	}
	for _, ev := range journalEvents(t, backend, runID) {
		if ev.Type == domain.EventCacheWarmed {
			t.Fatalf("cache.warmed event exists in off mode: %s", string(ev.Payload))
		}
	}
}
