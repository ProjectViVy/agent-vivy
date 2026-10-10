package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/provider"
	"agent-vivy/internal/storage"
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
				OutputPerMTok:        1000,
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
	release     chan struct{}
	warmErr     error
	warmStarted chan struct{}
	singleTurn  bool

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

func warmReply(cacheHit bool) *schema.Message {
	msg := &schema.Message{
		Role:    schema.Assistant,
		Content: "ok",
		ResponseMeta: &schema.ResponseMeta{Usage: &schema.TokenUsage{
			PromptTokens: 1345, CompletionTokens: 2, TotalTokens: 1347,
		}},
		Extra: map[string]any{},
	}
	// Mirrors einoclaude's keyOfCacheCreationInputTokens message extra so
	// the accounting fold exercises the real read path.
	if cacheHit {
		msg.ResponseMeta.Usage.PromptTokenDetails.CachedTokens = 1234
	} else {
		msg.Extra["_eino_claude_cache_creation_input_tokens"] = 1234
	}
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
		idx := m.warmCalls
		m.mu.Unlock()
		if m.warmStarted != nil {
			close(m.warmStarted)
			<-ctx.Done()
			return nil, ctx.Err()
		}
		if m.warmErr != nil {
			return nil, m.warmErr
		}
		return streamOf(warmReply(idx > 1)), nil
	}
	m.mu.Lock()
	m.calls++
	idx := m.calls
	m.mu.Unlock()
	if idx == 1 {
		if m.singleTurn {
			return streamOf(schema.AssistantMessage("done", nil), usageText("", 5000, 10)), nil
		}
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
		Journal: backend, Runs: backend, Messages: backend, Notes: backend, Sessions: backend, Sink: sink, Truncations: backend,
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
	svc, backend, _ := newWarmService(t, m, "streaming", 0, 60)
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
	// Maintenance requests carry the same lifecycle and project into billable usage.
	kinds := classifyObservedCall(journalEvents(t, backend, runID))
	maintenance := 0
	var mainReq, warmReq payloadModelRequestV3
	for _, ev := range journalEvents(t, backend, runID) {
		if ev.Type != domain.EventModelRequest {
			continue
		}
		var req payloadModelRequestV3
		if err := json.Unmarshal(ev.Payload, &req); err != nil {
			t.Fatal(err)
		}
		if req.Source == "maintenance" {
			maintenance++
			warmReq = req
		} else if req.Source == "main" && mainReq.CallID == "" {
			mainReq = req
		}
	}
	if maintenance == 0 || len(kinds.requests) != calls+warm {
		t.Fatalf("requests=%d maintenance=%d real=%d warm=%d; every warm needs a lifecycle", len(kinds.requests), maintenance, calls, warm)
	}
	if payload.CallID == "" || mainReq.PreambleSHA256 != warmReq.PreambleSHA256 {
		t.Fatalf("warm preamble differs from real call: main=%+v warm=%+v", mainReq, warmReq)
	}
	if strings.Join(mainReq.SelectedTools, ",") != strings.Join(warmReq.SelectedTools, ",") {
		t.Fatalf("warm tools differ from real call: %v/%v", mainReq.SelectedTools, warmReq.SelectedTools)
	}
	rows, err := backend.ListModelUsage(ctx, 0)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	writeRows, readRows := 0, 0
	for _, row := range rows {
		if row.Source == "maintenance" {
			found = true
			if row.CacheWriteKnown && row.CacheWriteTokens == 1234 {
				writeRows++
			}
			if row.CachedKnown && row.CachedTokens == 1234 {
				readRows++
			}
			if !row.HasUsage || row.PromptTokens != 1345 || row.CompletionTokens != 2 {
				t.Fatalf("maintenance usage=%+v", row)
			}
		}
	}
	if !found {
		t.Fatal("warm usage absent from accounting projection")
	}
	if writeRows != 1 || readRows != 1 {
		t.Fatalf("warm miss/write rows=%d hit/read rows=%d; want one of each", writeRows, readRows)
	}
	var digest string
	for _, ev := range journalEvents(t, backend, runID) {
		if ev.Type == domain.EventCacheWarmed {
			var p payloadCacheWarmed
			_ = json.Unmarshal(ev.Payload, &p)
			if p.Status == "warmed" {
				if digest != "" && digest != p.PrefixSHA256 {
					t.Fatal("warm miss and hit used different reusable prefixes")
				}
				digest = p.PrefixSHA256
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

func TestCacheWarmSavingsFloorSkips(t *testing.T) {
	m := newWarmSpyModel()
	// A positive savings floor cannot be evaluated without a cached rate.
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
	if payload.Reason != "unpriced_prefix" {
		t.Fatalf("skip reason = %q, want unpriced_prefix", payload.Reason)
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

func TestCacheWarmGateUsesReusablePrefixInsteadOfConversation(t *testing.T) {
	m := newWarmSpyModel()
	// The scripted call reports 5k total input tokens ($5 at ordinary input
	// price), but the reusable system/tools prefix is under $2 for one reuse.
	svc, backend, _ := newWarmService(t, m, "streaming", 2, 60)
	mustCreateSession(t, backend, "warm-prefix-gate")
	runID, err := svc.Run(context.Background(), "warm-prefix-gate", "warm me")
	if err != nil {
		t.Fatal(err)
	}
	p := waitForCacheWarmed(t, backend, runID, "skipped")
	close(m.release)
	waitForRunStatus(t, backend, runID, domain.RunCompleted)
	if p.Reason != "unpriced_prefix" || p.EstimatedPrefixTokens >= 5000 || p.PrefixSHA256 == "" {
		t.Fatalf("gate evidence=%+v", p)
	}
	if _, warm := m.counts(); warm != 0 {
		t.Fatalf("warm calls=%d, want none", warm)
	}
}

func TestCacheWarmEmptyDependencyConfigDoesNotWarm(t *testing.T) {
	m := newWarmSpyModel()
	svc, backend, _ := newWarmService(t, m, "", 0, 60)
	mustCreateSession(t, backend, "warm-unset")
	runID, err := svc.Run(context.Background(), "warm-unset", "warm me")
	if err != nil {
		t.Fatal(err)
	}
	close(m.release)
	waitForRunStatus(t, backend, runID, domain.RunCompleted)
	if _, warm := m.counts(); warm != 0 {
		t.Fatalf("warm calls=%d with no explicit opt-in", warm)
	}
}

func TestCacheWarmProviderFailureHasMandatoryClosure(t *testing.T) {
	m := newWarmSpyModel()
	m.warmErr = errors.New("warm request rejected")
	svc, backend, _ := newWarmService(t, m, "streaming", 0, 60)
	mustCreateSession(t, backend, "warm-failure")
	runID, err := svc.Run(context.Background(), "warm-failure", "warm me")
	if err != nil {
		t.Fatal(err)
	}
	p := waitForCacheWarmed(t, backend, runID, "failed")
	close(m.release)
	waitForRunStatus(t, backend, runID, domain.RunCompleted)
	if p.CallID == "" {
		t.Fatal("failed warm missing lifecycle identity")
	}
	finishes := 0
	for _, ev := range journalEvents(t, backend, runID) {
		if ev.Type != domain.EventModelCallFinished {
			continue
		}
		var finish payloadModelCallFinished
		if err := json.Unmarshal(ev.Payload, &finish); err != nil {
			t.Fatal(err)
		}
		if finish.CallID == p.CallID {
			finishes++
			if finish.Source != "maintenance" || finish.Status != "failed" || finish.Usage != nil {
				t.Fatalf("failure settlement=%+v", finish)
			}
		}
	}
	if finishes != 1 {
		t.Fatalf("warm closures=%d, want exactly one", finishes)
	}
}

func TestCacheWarmCancellationClosesBeforeRunTerminal(t *testing.T) {
	m := newWarmSpyModel()
	m.warmStarted = make(chan struct{})
	svc, backend, _ := newWarmService(t, m, "streaming", 0, 60)
	mustCreateSession(t, backend, "warm-cancel")
	runID, err := svc.Run(context.Background(), "warm-cancel", "warm me")
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-m.warmStarted:
	case <-time.After(10 * time.Second):
		t.Fatal("maintenance call did not start")
	}
	if !svc.Cancel(runID) {
		t.Fatal("cancel run failed")
	}
	waitForRunStatus(t, backend, runID, domain.RunCancelled)
	requests, finishes := 0, 0
	for _, ev := range journalEvents(t, backend, runID) {
		if ev.Type == domain.EventModelRequest {
			var p payloadModelRequestV3
			_ = json.Unmarshal(ev.Payload, &p)
			if p.Source == "maintenance" {
				requests++
			}
		}
		if ev.Type == domain.EventModelCallFinished {
			var p payloadModelCallFinished
			_ = json.Unmarshal(ev.Payload, &p)
			if p.Source == "maintenance" {
				finishes++
				if p.Status != "cancelled" || p.Usage != nil {
					t.Fatalf("cancelled warm settlement=%+v", p)
				}
			}
		}
	}
	if requests != 1 || finishes != 1 {
		t.Fatalf("cancelled warm lifecycle requests=%d finishes=%d", requests, finishes)
	}
}

type maintenanceFailJournal struct {
	storage.Journal
	failType domain.EventType
}

func (j maintenanceFailJournal) Append(ctx context.Context, commit storage.Commit) (domain.EventSeq, error) {
	for _, ev := range commit.Events {
		var p struct {
			Source string `json:"source"`
		}
		_ = json.Unmarshal(ev.Payload, &p)
		if ev.Type == j.failType && p.Source == "maintenance" {
			return 0, errors.New("mandatory maintenance persistence failed")
		}
	}
	return j.Journal.Append(ctx, commit)
}

func TestCacheWarmMandatoryPersistenceFailureReachesOwningEnd(t *testing.T) {
	for _, eventType := range []domain.EventType{domain.EventModelRequest, domain.EventModelUsage, domain.EventModelCallFinished} {
		t.Run(string(eventType), func(t *testing.T) {
			m := newWarmSpyModel()
			svc, backend, _ := newWarmService(t, m, "streaming", 0, 60)
			mustCreateSession(t, backend, "warm-persist-fail")
			ctx := context.Background()
			runID := domain.RunID("warm-persist-fail")
			if err := backend.CreateRun(ctx, domain.Run{ID: runID, SessionID: "warm-persist-fail", Status: domain.RunActive, CreatedAt: 1}); err != nil {
				t.Fatal(err)
			}
			mapper := newEventMapper(runID, 64<<10)
			mapper.setUsageRoutes("test", "test-model", "")
			svc.deps.Journal = maintenanceFailJournal{Journal: backend, failType: eventType}
			warmer := svc.newRunCacheWarmer(ctx, mapper, "warm-persist-fail", svc.engine, nil)
			o := &runModelCallObserver{svc: svc, m: mapper, sessionID: "warm-persist-fail", source: "main", provider: "test", model: "test-model", warmer: warmer, calls: map[string]*observedModelCall{}}
			meta, err := o.Begin(ctx, modelCallInput{Mode: "generate", Messages: []*schema.Message{schema.SystemMessage("authoritative instruction"), schema.UserMessage("hello")}})
			if err != nil {
				t.Fatal(err)
			}
			if err := o.Chunk(ctx, meta, usageText("", 100, 1)); err != nil {
				t.Fatal(err)
			}
			if err := o.End(ctx, meta, modelCallResult{ResponseComplete: true}); err == nil {
				t.Fatal("owning End hid mandatory maintenance lifecycle failure")
			}
		})
	}
}

func TestCacheWarmPricedGateExcludesConversationTokens(t *testing.T) {
	m := newWarmSpyModel()
	svc, backend, _ := newWarmService(t, m, "streaming", 2, 60)
	mustCreateSession(t, backend, "warm-priced-prefix")
	ctx := context.Background()
	runID := domain.RunID("warm-priced-prefix")
	if err := backend.CreateRun(ctx, domain.Run{ID: runID, SessionID: "warm-priced-prefix", Status: domain.RunActive, CreatedAt: 1}); err != nil {
		t.Fatal(err)
	}
	mapper := newEventMapper(runID, 64<<10)
	mapper.setUsageRoutes("test", "test-model", "")
	warmer := svc.newRunCacheWarmer(ctx, mapper, "warm-priced-prefix", svc.engine, nil)
	// Synthetic declared prices isolate the gate arithmetic; production
	// metadata has no cached rate and therefore uses unpriced_prefix instead.
	warmer.info.CachedInputPerMTokens = 100
	if err := warmer.settled(normalizedUsageSample{PromptTokens: 5000}, modelCallInput{
		Messages: []*schema.Message{schema.SystemMessage("short reusable instruction"), schema.UserMessage(strings.Repeat("conversation", 2000))},
		Tools:    []*schema.ToolInfo{svc.engine.toolInfos[tools.EchoInfoName]},
	}); err != nil {
		t.Fatal(err)
	}
	p := waitForCacheWarmed(t, backend, runID, "skipped")
	if p.Reason != "below_min_savings" || p.EstimatedPrefixTokens >= 2000 {
		t.Fatalf("priced reusable-prefix gate=%+v", p)
	}
	if _, warm := m.counts(); warm != 0 {
		t.Fatalf("unrelated conversation justified %d warm calls", warm)
	}
}

func TestCacheWarmAdmissionBudgetDenialDoesNotFailOwningEnd(t *testing.T) {
	for _, policy := range []BudgetPolicy{{MaxModelCalls: 1}, {MaxEvents: 2}} {
		t.Run(fmt.Sprintf("calls-%d-events-%d", policy.MaxModelCalls, policy.MaxEvents), func(t *testing.T) {
			m := newWarmSpyModel()
			svc, backend, _ := newWarmService(t, m, "streaming", 0, 60)
			ctx := context.Background()
			sessionID := domain.SessionID("warm-budget")
			runID := domain.RunID("warm-budget")
			mustCreateSession(t, backend, sessionID)
			if err := backend.CreateRun(ctx, domain.Run{ID: runID, SessionID: sessionID, Status: domain.RunActive, CreatedAt: 1}); err != nil {
				t.Fatal(err)
			}
			mapper := newEventMapper(runID, 64<<10)
			mapper.setUsageRoutes("test", "test-model", "")
			ledger, err := NewBudgetLedger(policy)
			if err != nil {
				t.Fatal(err)
			}
			warmer := svc.newRunCacheWarmer(ctx, mapper, sessionID, svc.engine, ledger)
			o := &runModelCallObserver{svc: svc, m: mapper, sessionID: sessionID, ledger: ledger, source: "main", provider: "test", model: "test-model", warmer: warmer, calls: map[string]*observedModelCall{}}
			meta, err := o.Begin(ctx, modelCallInput{Mode: "generate", Messages: []*schema.Message{schema.SystemMessage("system"), schema.UserMessage("hello")}})
			if err != nil {
				t.Fatal(err)
			}
			if err := o.Chunk(ctx, meta, usageText("", 100, 1)); err != nil {
				t.Fatal(err)
			}
			if err := o.End(ctx, meta, modelCallResult{ResponseComplete: true}); err != nil {
				t.Fatalf("optional warm admission failed completed main call: %v", err)
			}
			if _, warm := m.counts(); warm != 0 {
				t.Fatalf("denied maintenance made %d paid calls", warm)
			}
			for _, ev := range journalEvents(t, backend, runID) {
				if ev.Type == domain.EventModelRequest {
					var p payloadModelRequestV3
					_ = json.Unmarshal(ev.Payload, &p)
					if p.Source == "maintenance" {
						t.Fatal("unadmitted warm has lifecycle request")
					}
				}
			}
			p := waitForCacheWarmed(t, backend, runID, "skipped")
			if p.Reason != "budget_exhausted" {
				t.Fatalf("denied warm diagnostic=%+v", p)
			}
		})
	}
}

func TestCacheWarmSingleMainCallBudgetCompletesRun(t *testing.T) {
	m := newWarmSpyModel()
	m.singleTurn = true
	svc, backend, _ := newWarmService(t, m, "streaming", 0, 60)
	svc.deps.Budget = BudgetPolicy{MaxModelCalls: 1}
	mustCreateSession(t, backend, "warm-single-budget")
	runID, err := svc.Run(context.Background(), "warm-single-budget", "hello")
	if err != nil {
		t.Fatal(err)
	}
	waitForRunStatus(t, backend, runID, domain.RunCompleted)
	calls, warm := m.counts()
	if calls != 1 || warm != 0 {
		t.Fatalf("main calls=%d warm calls=%d; want one main and no paid maintenance", calls, warm)
	}
	p := waitForCacheWarmed(t, backend, runID, "skipped")
	if p.Reason != "budget_exhausted" {
		t.Fatalf("diagnostic=%+v", p)
	}
}

func TestCacheWarmPaidUsageBudgetFailureRemainsMandatory(t *testing.T) {
	m := newWarmSpyModel()
	svc, backend, _ := newWarmService(t, m, "streaming", 0, 60)
	ctx := context.Background()
	mustCreateSession(t, backend, "warm-paid-budget")
	runID := domain.RunID("warm-paid-budget")
	if err := backend.CreateRun(ctx, domain.Run{ID: runID, SessionID: "warm-paid-budget", Status: domain.RunActive, CreatedAt: 1}); err != nil {
		t.Fatal(err)
	}
	mapper := newEventMapper(runID, 64<<10)
	mapper.setUsageRoutes("test", "test-model", "")
	ledger, err := NewBudgetLedger(BudgetPolicy{MaxEvents: 1})
	if err != nil {
		t.Fatal(err)
	}
	warmer := svc.newRunCacheWarmer(ctx, mapper, "warm-paid-budget", svc.engine, ledger)
	err = warmer.warm(modelCallInput{Messages: []*schema.Message{schema.SystemMessage("system")}}, payloadCacheWarmed{})
	if !errors.Is(err, ErrBudgetExceeded) {
		t.Fatalf("paid usage admission failure=%v, want mandatory budget error", err)
	}
	if _, warm := m.counts(); warm != 1 {
		t.Fatalf("paid calls=%d, want one", warm)
	}
	finishes := 0
	for _, ev := range journalEvents(t, backend, runID) {
		if ev.Type == domain.EventModelCallFinished {
			var p payloadModelCallFinished
			_ = json.Unmarshal(ev.Payload, &p)
			if p.Source == "maintenance" {
				finishes++
				if p.Status != "failed" || p.Usage == nil {
					t.Fatalf("paid failure settlement=%+v", p)
				}
			}
		}
	}
	if finishes != 1 {
		t.Fatalf("paid maintenance closures=%d, want one", finishes)
	}
}
