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
				InputPerMTok:         1000, // $1 per 1k tokens: 5k prompt = $5
				SupportsWarming:      true,
				CacheLifetimeSeconds: lifetimeSeconds,
			}},
		}},
	}
}

// warmSpyModel is an AgenticModel double: call 1 requests the
// echo_info tool and reports usage; every later real call blocks until
// release; a call whose last message is the warm marker counts itself
// and replies immediately with usage + a cache-write extra.
type warmSpyModel struct {
	release chan struct{}

	mu        sync.Mutex
	calls     int
	warmCalls int
}

var _ model.AgenticModel = (*warmSpyModel)(nil)

func newWarmSpyModel() *warmSpyModel {
	return &warmSpyModel{release: make(chan struct{})}
}

func (m *warmSpyModel) counts() (calls, warm int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.calls, m.warmCalls
}

func warmReply() *schema.AgenticMessage {
	msg := agenticTestMessage(schema.AgenticRoleTypeAssistant, "ok", nil, &schema.TokenUsage{
		PromptTokens: 111, CompletionTokens: 2, TotalTokens: 113,
	}, map[string]any{})
	// Mirrors einoclaude's keyOfCacheCreationInputTokens message extra so
	// the accounting fold exercises the real read path.
	msg.Extra["_eino_claude_cache_creation_input_tokens"] = 1234
	return msg
}

func (m *warmSpyModel) Generate(ctx context.Context, input []*schema.AgenticMessage, opts ...model.Option) (*schema.AgenticMessage, error) {
	r, err := m.Stream(ctx, input, opts...)
	if err != nil {
		return nil, err
	}
	return schema.ConcatAgenticMessages(collectStream(r))
}

func (m *warmSpyModel) Stream(ctx context.Context, input []*schema.AgenticMessage, _ ...model.Option) (*schema.StreamReader[*schema.AgenticMessage], error) {
	if n := len(input); n > 0 && input[n-1].Role == schema.AgenticRoleTypeUser && agenticMessageText(input[n-1]) == cacheWarmPrompt {
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
		return streamOf(testAssistant("done"), usageText("", 5000, 10)), nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func streamOf(msgs ...*schema.AgenticMessage) *schema.StreamReader[*schema.AgenticMessage] {
	r, w := schema.Pipe[*schema.AgenticMessage](8)
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

func newWarmService(t *testing.T, chat model.AgenticModel, mode string, minSavings float64, lifetime int) (*Service, *sqlite.Backend, *testSink) {
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
	// The existing cache-write diagnostic remains alongside the paid
	// maintenance lifecycle; it does not replace completion-token accounting.
	if payload.PromptTokens != 111 || payload.CacheWriteTokens != 1234 {
		t.Fatalf("warm accounting = prompt:%d write:%d, want 111/1234", payload.PromptTokens, payload.CacheWriteTokens)
	}
	// Maintenance has its own observed calls without entering chat messages.
	kinds := classifyObservedCall(journalEvents(t, backend, runID))
	if len(kinds.requests) != calls+warm {
		t.Fatalf("model.request count = %d, want %d (foreground + maintenance)", len(kinds.requests), calls+warm)
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

// The maintenance boundary keeps its cap/cancellation and diagnostic accounting
// even while loop-extension factories remain entirely unconstructed.
type warmBoundaryModel struct {
	*warmSpyModel
	boundary func(context.Context) error
}

func (m *warmBoundaryModel) Generate(ctx context.Context, input []*schema.AgenticMessage, opts ...model.Option) (*schema.AgenticMessage, error) {
	if len(input) > 0 && agenticMessageText(input[len(input)-1]) == cacheWarmPrompt {
		return nil, m.boundary(ctx)
	}
	return m.warmSpyModel.Generate(ctx, input, opts...)
}

func TestCacheWarmFailureRemainsDiagnostic(t *testing.T) {
	m := &warmBoundaryModel{warmSpyModel: newWarmSpyModel(), boundary: func(context.Context) error { return errors.New("private-warm-failure-canary") }}
	svc, backend, _ := newWarmService(t, m, "streaming", 0.05, 60)
	mustCreateSession(t, backend, "warm-failure")
	run, err := svc.Run(context.Background(), "warm-failure", "warm without altering chat")
	if err != nil {
		t.Fatal(err)
	}
	payload := waitForCacheWarmed(t, backend, run, "failed")
	if strings.Contains(payload.Reason, "private-warm") || payload.PromptTokens != 0 || payload.CacheWriteTokens != 0 {
		t.Fatalf("failure diagnostic/accounting: %+v", payload)
	}
	close(m.release)
	waitForRunStatus(t, backend, run, domain.RunCompleted)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if !svc.WaitIdle(ctx) {
		t.Fatal("chat did not return idle")
	}
}

func TestCacheWarmCancellationKeepsDeadline(t *testing.T) {
	entered := make(chan time.Duration, 1)
	returned := make(chan struct{})
	m := &warmBoundaryModel{warmSpyModel: newWarmSpyModel(), boundary: func(ctx context.Context) error {
		deadline, ok := ctx.Deadline()
		if !ok {
			entered <- 0
		} else {
			entered <- time.Until(deadline)
		}
		<-ctx.Done()
		close(returned)
		return ctx.Err()
	}}
	svc, backend, _ := newWarmService(t, m, "streaming", 0.05, 60)
	mustCreateSession(t, backend, "warm-cancel")
	run, err := svc.Run(context.Background(), "warm-cancel", "cancel maintenance with run")
	if err != nil {
		t.Fatal(err)
	}
	select {
	case remaining := <-entered:
		if remaining <= 0 || remaining > cacheWarmCallTimeout {
			t.Fatalf("maintenance deadline: %v", remaining)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("maintenance did not start")
	}
	if !svc.Cancel(run) {
		t.Fatal("active Run missing")
	}
	select {
	case <-returned:
	case <-time.After(3 * time.Second):
		t.Fatal("maintenance ignored Run cancellation")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if !svc.WaitIdle(ctx) {
		t.Fatal("cancelled Run did not return idle")
	}
	for _, event := range journalEvents(t, backend, run) {
		if event.Type == domain.EventCacheWarmed {
			t.Fatalf("cancelled maintenance created diagnostic: %s", event.Payload)
		}
	}
}

func directWarmFixture(t *testing.T, policy BudgetPolicy) (*runCacheWarmer, *BudgetLedger, *warmSpyModel, *sqlite.Backend) {
	t.Helper()
	chat := newWarmSpyModel()
	svc, backend, _ := newWarmService(t, chat, "streaming", 0.05, 60)
	mustCreateSession(t, backend, "direct-warm")
	if err := backend.CreateRun(context.Background(), domain.Run{ID: "direct-warm-run", SessionID: "direct-warm", Status: domain.RunActive, CreatedAt: 1}); err != nil {
		t.Fatal(err)
	}
	ledger, err := NewBudgetLedger(policy)
	if err != nil {
		t.Fatal(err)
	}
	mapper := newEventMapper("direct-warm-run", 64<<10)
	mapper.setUsageRoutes("test", "test-model", "")
	ctx := svc.withLiveModelStreamObserver(context.Background(), mapper, "direct-warm", ledger, nil, false, nil)
	warmer := svc.newRunCacheWarmer(ctx, mapper, "direct-warm", svc.engine, nil)
	warmer.ledger = ledger
	return warmer, ledger, chat, backend
}

func TestCacheWarmExhaustedBudgetSkipsPaidProvider(t *testing.T) {
	w, ledger, chat, backend := directWarmFixture(t, BudgetPolicy{MaxModelCalls: 1})
	if err := ledger.ReserveModelCall(); err != nil {
		t.Fatal(err)
	}
	w.warm(cacheWarmModeStreaming)
	if _, got := chat.counts(); got != 0 {
		t.Fatalf("paid warming calls after budget exhaustion = %d", got)
	}
	if got := classifyObservedCall(journalEvents(t, backend, "direct-warm-run")); len(got.requests) != 0 {
		t.Fatal("budget refusal admitted a maintenance request")
	}
}

func TestCacheWarmPaidCallHasMandatoryUsageAndFinish(t *testing.T) {
	w, ledger, chat, backend := directWarmFixture(t, DefaultBudgetPolicy())
	w.warm(cacheWarmModeStreaming)
	if _, got := chat.counts(); got != 1 {
		t.Fatalf("paid warming calls = %d", got)
	}
	events := journalEvents(t, backend, "direct-warm-run")
	kinds := classifyObservedCall(events)
	if len(kinds.requests) != 1 || len(kinds.settlement) != 1 || len(kinds.finished) != 1 {
		t.Fatalf("maintenance lifecycle = request:%d settlement:%d finish:%d", len(kinds.requests), len(kinds.settlement), len(kinds.finished))
	}
	var request payloadModelRequestV4
	mustUnmarshal(t, kinds.requests[0].Payload, &request)
	var finish payloadModelCallFinished
	mustUnmarshal(t, kinds.finished[0].Payload, &finish)
	if request.Source != "maintenance" || finish.Source != "maintenance" || finish.Usage == nil || finish.Usage.CompletionTokens != 2 || finish.Usage.TotalTokens != 113 || !finish.ResponseComplete {
		t.Fatalf("maintenance request/finish = %+v / %+v", request, finish)
	}
	if ledger.Snapshot().Usage.ModelCalls != 1 {
		t.Fatal("paid warming did not consume the shared model-call budget")
	}
	if w.m.consumeObservedCallCharge() || w.m.takeObservedStream() {
		t.Fatal("maintenance credited a foreground call or stream")
	}
	for _, event := range events {
		if event.Type == domain.EventModelRequest || event.Type == domain.EventModelUsage || event.Type == domain.EventModelCallFinished {
			validatePayloadSchema(t, string(event.Type), event.Payload)
		}
		if event.Type == domain.EventModelDelta || event.Type == domain.EventModelCompleted {
			t.Fatal("maintenance entered the transcript")
		}
	}
	projection := storage.UsageRunInput{RunID: "direct-warm-run", SessionID: "direct-warm", RunStatus: domain.RunActive}
	for _, event := range events {
		projection.Events = append(projection.Events, storage.CanonicalUsageEvent{Type: event.Type, Seq: int(event.Seq), CreatedAt: event.CreatedAt, Payload: event.Payload})
	}
	rows := storage.ProjectUsageRows(projection, 0)
	if len(rows) != 1 || rows[0].Source != "maintenance" || rows[0].CompletionTokens != 2 || rows[0].TotalTokens != 113 {
		t.Fatalf("paid usage projection = %+v", rows)
	}
}

func TestCacheWarmDiagnosticQuotaCannotDropPaidSettlement(t *testing.T) {
	w, _, _, backend := directWarmFixture(t, BudgetPolicy{MaxEvents: 1, MaxModelCalls: 1})
	w.warm(cacheWarmModeStreaming)
	if err := w.stop(); err != nil {
		t.Fatal(err)
	}
	events := journalEvents(t, backend, "direct-warm-run")
	kinds := classifyObservedCall(events)
	if len(kinds.requests) != 1 || len(kinds.settlement) != 1 || len(kinds.finished) != 1 {
		t.Fatalf("quota-limited lifecycle = %d/%d/%d", len(kinds.requests), len(kinds.settlement), len(kinds.finished))
	}
	for _, event := range events {
		if event.Type == domain.EventCacheWarmed {
			t.Fatal("optional diagnostic exceeded the event quota")
		}
	}
}

type maintenanceFailJournal struct {
	storage.Journal
	event  domain.EventType
	failed chan struct{}
}

func (j maintenanceFailJournal) Append(ctx context.Context, commit storage.Commit) (domain.EventSeq, error) {
	for _, event := range commit.Events {
		var routed struct {
			Source string `json:"source"`
		}
		_ = json.Unmarshal(event.Payload, &routed)
		if routed.Source == "maintenance" && event.Type == j.event {
			if j.failed != nil {
				select {
				case j.failed <- struct{}{}:
				default:
				}
			}
			return 0, errors.New("maintenance persistence unavailable")
		}
	}
	return j.Journal.Append(ctx, commit)
}

func TestCacheWarmSettlementFailureFailsOwningRun(t *testing.T) {
	for _, event := range []domain.EventType{domain.EventModelRequest, domain.EventModelUsage, domain.EventModelCallFinished} {
		t.Run(string(event), func(t *testing.T) {
			chat := newWarmSpyModel()
			svc, backend, _ := newWarmService(t, chat, "streaming", 0.05, 60)
			failed := make(chan struct{}, 1)
			svc.deps.Journal = maintenanceFailJournal{Journal: backend, event: event, failed: failed}
			mustCreateSession(t, backend, "warm-persist-failure")
			run, err := svc.Run(context.Background(), "warm-persist-failure", "warm safely")
			if err != nil {
				t.Fatal(err)
			}
			select {
			case <-failed:
			case <-time.After(3 * time.Second):
				close(chat.release)
				t.Fatal("maintenance failure was not exercised")
			}
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			idle := svc.WaitIdle(ctx)
			close(chat.release)
			if !idle {
				t.Fatal("mandatory maintenance failure did not stop the blocked foreground call")
			}
			stored, err := backend.GetRun(context.Background(), run)
			if err != nil || stored.Status != domain.RunFailed {
				t.Fatalf("mandatory maintenance persistence failure = %s, %v", stored.Status, err)
			}
			if event == domain.EventModelRequest {
				if _, paid := chat.counts(); paid != 0 {
					t.Fatal("provider ran before durable maintenance admission")
				}
			}
			var terminal, maintenanceFinish int
			for _, ev := range journalEvents(t, backend, run) {
				if ev.Type.Terminal() {
					terminal++
				}
				if ev.Type == domain.EventModelCallFinished {
					var finish payloadModelCallFinished
					mustUnmarshal(t, ev.Payload, &finish)
					if finish.Source == "maintenance" {
						maintenanceFinish++
						if finish.Usage == nil || finish.Usage.TotalTokens != 113 {
							t.Fatalf("paid usage missing from closure: %+v", finish)
						}
					}
				}
			}
			if terminal != 1 {
				t.Fatalf("owning Run terminal count = %d", terminal)
			}
			if event == domain.EventModelUsage && maintenanceFinish != 1 {
				t.Fatal("usage failure prevented the mandatory finish attempt")
			}
		})
	}
}

func TestCacheWarmWaitIdleDrainsCancelledPaidCall(t *testing.T) {
	entered, draining, release := make(chan struct{}), make(chan struct{}), make(chan struct{})
	chat := &warmBoundaryModel{warmSpyModel: newWarmSpyModel(), boundary: func(ctx context.Context) error {
		close(entered)
		<-ctx.Done()
		close(draining)
		<-release
		return ctx.Err()
	}}
	svc, backend, _ := newWarmService(t, chat, "streaming", 0.05, 60)
	mustCreateSession(t, backend, "warm-drain")
	run, err := svc.Run(context.Background(), "warm-drain", "cancel and drain paid call")
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("maintenance did not start")
	}
	svc.Cancel(run)
	select {
	case <-draining:
	case <-time.After(3 * time.Second):
		t.Fatal("maintenance was not cancelled")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Millisecond)
	defer cancel()
	idle := svc.WaitIdle(ctx)
	close(release)
	if idle {
		t.Fatal("WaitIdle returned before the paid maintenance call drained")
	}
	ctx2, cancel2 := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel2()
	if !svc.WaitIdle(ctx2) {
		t.Fatal("cancelled maintenance failed to settle")
	}
	kinds := classifyObservedCall(journalEvents(t, backend, run))
	var maintenanceFinish int
	for _, ev := range kinds.finished {
		var p payloadModelCallFinished
		mustUnmarshal(t, ev.Payload, &p)
		if p.Source == "maintenance" {
			maintenanceFinish++
			if p.Status != "cancelled" {
				t.Fatalf("maintenance finish=%+v", p)
			}
		}
	}
	if maintenanceFinish != 1 {
		t.Fatalf("cancelled maintenance finishes = %d", maintenanceFinish)
	}
}

type paidWarmFailureModel struct {
	*warmSpyModel
	generate func(context.Context, []model.Option) (*schema.AgenticMessage, error)
}

func (m *paidWarmFailureModel) Generate(ctx context.Context, input []*schema.AgenticMessage, opts ...model.Option) (*schema.AgenticMessage, error) {
	return m.generate(ctx, opts)
}

func TestCacheWarmReturnedUsageSettlesOnProviderFailureAndCancellation(t *testing.T) {
	for _, cancelled := range []bool{false, true} {
		t.Run(fmt.Sprint(cancelled), func(t *testing.T) {
			w, _, chat, backend := directWarmFixture(t, DefaultBudgetPolicy())
			ctx, cancel := context.WithCancel(w.runCtx)
			defer cancel()
			w.runCtx = ctx
			w.eng.chatModel = &paidWarmFailureModel{warmSpyModel: chat, generate: func(ctx context.Context, opts []model.Option) (*schema.AgenticMessage, error) {
				common := model.GetCommonOptions(nil, opts...)
				if common.MaxTokens == nil || *common.MaxTokens != cacheWarmMaxTokens {
					t.Fatalf("maintenance max tokens = %v", common.MaxTokens)
				}
				if cancelled {
					cancel()
					return warmReply(), ctx.Err()
				}
				return warmReply(), errors.New("provider response incomplete")
			}}
			w.warm(cacheWarmModeStreaming)
			kinds := classifyObservedCall(journalEvents(t, backend, "direct-warm-run"))
			if len(kinds.settlement) != 1 || len(kinds.finished) != 1 {
				t.Fatalf("paid failure lifecycle = %d/%d", len(kinds.settlement), len(kinds.finished))
			}
			var finish payloadModelCallFinished
			mustUnmarshal(t, kinds.finished[0].Payload, &finish)
			status := "failed"
			if cancelled {
				status = "cancelled"
			}
			if finish.Status != status || finish.ResponseComplete || finish.Usage == nil || finish.Usage.CompletionTokens != 2 || finish.Usage.TotalTokens != 113 {
				t.Fatalf("paid failure finish = %+v", finish)
			}
		})
	}
}

type warmingSteerModel struct {
	*scriptedToolModel
	firstWarm, firstCancelled, firstRelease, secondWarm chan struct{}
	resumed, resumeRelease                              chan struct{}
	warmCount                                           int
}

func (m *warmingSteerModel) Generate(ctx context.Context, _ []*schema.AgenticMessage, _ ...model.Option) (*schema.AgenticMessage, error) {
	m.mu.Lock()
	m.warmCount++
	call := m.warmCount
	m.mu.Unlock()
	if call == 1 {
		close(m.firstWarm)
		<-ctx.Done()
		close(m.firstCancelled)
		<-m.firstRelease
		return warmReply(), ctx.Err()
	}
	if call == 2 {
		close(m.secondWarm)
	}
	return warmReply(), nil
}

func (m *warmingSteerModel) Stream(ctx context.Context, input []*schema.AgenticMessage, _ ...model.Option) (*schema.StreamReader[*schema.AgenticMessage], error) {
	call := m.record(input)
	switch call {
	case 1:
		return streamOf(toolCallMessage("warm-steer-1", tools.EchoInfoName, `{}`), usageText("", 5000, 10)), nil
	case 2:
		close(m.entered)
		select {
		case <-m.release:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
		return streamOf(toolCallMessage("warm-steer-2", tools.EchoInfoName, `{}`), usageText("", 5000, 10)), nil
	case 3:
		close(m.resumed)
		select {
		case <-m.resumeRelease:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
		return streamOf(toolCallMessage("warm-steer-3", tools.EchoInfoName, `{}`), usageText("", 5000, 10)), nil
	default:
		<-ctx.Done()
		return nil, ctx.Err()
	}
}

func TestCacheWarmSteerDrainsPreviousLegAndRebindsMaintenance(t *testing.T) {
	chat := &warmingSteerModel{scriptedToolModel: newScriptedToolModel(), firstWarm: make(chan struct{}), firstCancelled: make(chan struct{}), firstRelease: make(chan struct{}), secondWarm: make(chan struct{}), resumed: make(chan struct{}), resumeRelease: make(chan struct{})}
	svc, backend := newScriptedService(t, chat)
	svc.deps.CacheWarmingMode = "streaming"
	svc.SetCatalog(provider.NewCatalog(warmVendor(60)))
	mustCreateSession(t, backend, "warm-steer")
	var oldRelease, mainRelease, resumedRelease sync.Once
	t.Cleanup(func() {
		oldRelease.Do(func() { close(chat.firstRelease) })
		mainRelease.Do(func() { close(chat.release) })
		resumedRelease.Do(func() { close(chat.resumeRelease) })
		svc.CancelAll()
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		svc.WaitIdle(ctx)
	})
	run, err := svc.Run(context.Background(), "warm-steer", "warm then steer")
	if err != nil {
		t.Fatal(err)
	}
	for label, ready := range map[string]<-chan struct{}{"first maintenance": chat.firstWarm, "second foreground": chat.entered} {
		select {
		case <-ready:
		case <-time.After(3 * time.Second):
			t.Fatalf("%s did not start", label)
		}
	}
	if _, err := svc.Steer(context.Background(), "warm-steer", "continue same Run with warming"); err != nil {
		t.Fatal(err)
	}
	mainRelease.Do(func() { close(chat.release) })
	select {
	case <-chat.firstCancelled:
	case <-time.After(3 * time.Second):
		t.Fatal("prior-leg maintenance was not cancelled")
	}
	select {
	case <-chat.resumed:
		t.Fatal("replacement leg started before paid maintenance drained")
	case <-time.After(60 * time.Millisecond):
	}
	oldRelease.Do(func() { close(chat.firstRelease) })
	select {
	case <-chat.resumed:
	case <-time.After(3 * time.Second):
		t.Fatal("steering did not resume")
	}
	resumedRelease.Do(func() { close(chat.resumeRelease) })
	select {
	case <-chat.secondWarm:
	case <-time.After(3 * time.Second):
		t.Fatal("replacement leg did not rebind a fresh warmer")
	}
	if !svc.Cancel(run) {
		t.Fatal("steered Run lost its cancellation owner")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if !svc.WaitIdle(ctx) {
		t.Fatal("steered maintenance did not drain with its Run")
	}
	runs, err := backend.ListRunsBySession(context.Background(), "warm-steer")
	if err != nil || len(runs) != 1 || runs[0].ID != run || runs[0].Status != domain.RunCancelled {
		t.Fatalf("same-Run steering/cancel = %+v, %v", runs, err)
	}
}
