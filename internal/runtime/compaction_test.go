package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/adk/middlewares/summarization"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/provider"
	"agent-vivy/internal/storage/sqlite"
	"agent-vivy/internal/tools"
)

func TestCompactionPolicyTriggerTokens(t *testing.T) {
	p := CompactionPolicy{Enabled: true, MaxTokens: 0, TriggerPercent: 80, KeepRecent: 12}
	if got := p.EffectiveMaxTokens(); got != fallbackContextWindowTokens {
		t.Fatalf("EffectiveMaxTokens = %d, want %d", got, fallbackContextWindowTokens)
	}
	// No feed budget clamp: 80% of 128000.
	if got := p.TriggerTokens(0); got != 102400 {
		t.Fatalf("TriggerTokens(0) = %d, want 102400", got)
	}
	// Feed budget clamp wins when smaller than the percent-window threshold.
	if got := p.TriggerTokens(100_000); got != 100_000 {
		t.Fatalf("TriggerTokens(100000) = %d, want 100000", got)
	}
	explicit := CompactionPolicy{Enabled: true, MaxTokens: 8000, TriggerPercent: 50, KeepRecent: 1}
	if got := explicit.TriggerTokens(1 << 20); got != 4000 {
		t.Fatalf("TriggerTokens = %d, want 4000", got)
	}
	if got := explicit.TriggerTokens(2000); got != 2000 {
		t.Fatalf("clamped TriggerTokens = %d, want 2000", got)
	}
}

func TestCountMessageTokens(t *testing.T) {
	msgs := []*schema.Message{
		schema.UserMessage(strings.Repeat("a", 400)),
		schema.AssistantMessage(strings.Repeat("b", 400), nil),
	}
	got, err := countMessageTokens(msgs, nil)
	if err != nil {
		t.Fatalf("countMessageTokens: %v", err)
	}
	// 800 content bytes + 32 envelope => ~208 tokens at 4 bytes/token.
	if got < 180 || got > 240 {
		t.Fatalf("countMessageTokens = %d, want ~208", got)
	}
}

// TestMapperMapsSummarizationUsageEvent proves the Eino summarization
// middleware's generate_summary internal event is translated into a durable
// model.usage event so the hidden summary call is observable and accounted.
func TestMapperMapsSummarizationUsageEvent(t *testing.T) {
	m := newEventMapper("run-1", 0)
	action := &summarization.CustomizedAction{
		Type: summarization.ActionTypeGenerateSummary,
		GenerateSummary: &summarization.GenerateSummaryAction{
			Attempt: 1,
			Phase:   summarization.GenerateSummaryPhasePrimary,
			ModelResponse: &schema.Message{
				Role: schema.Assistant, Content: "summary",
				ResponseMeta: &schema.ResponseMeta{Usage: &schema.TokenUsage{PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15}},
			},
		},
	}
	events, err := m.onCustomizedAction(action)
	if err != nil {
		t.Fatalf("onCustomizedAction: %v", err)
	}
	if len(events) != 1 || events[0].Type != domain.EventModelUsage {
		t.Fatalf("events = %+v, want one model.usage", events)
	}
	var p payloadModelUsage
	if err := json.Unmarshal(events[0].Payload, &p); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if p.TotalTokens != 15 || p.PromptTokens != 10 {
		t.Fatalf("usage = %+v, want 10/5/15", p)
	}
	// Unrelated customized actions are silent.
	if events, err := m.onCustomizedAction("whatever"); err != nil || len(events) != 0 {
		t.Fatalf("unknown customized action must be silent: %v %v", events, err)
	}
}

// drainFinalText reassembles the assistant output of a run, whether it
// arrives as whole messages or streaming chunks.
func drainFinalText(t *testing.T, iter *adk.AsyncIterator[*adk.AgentEvent]) string {
	t.Helper()
	var sb strings.Builder
	for {
		ev, ok := iter.Next()
		if !ok {
			break
		}
		if ev.Err != nil {
			t.Fatalf("agent event error: %v", ev.Err)
		}
		if ev.Output == nil || ev.Output.MessageOutput == nil {
			continue
		}
		mv := ev.Output.MessageOutput
		if mv.MessageStream != nil && mv.IsStreaming {
			for {
				chunk, err := mv.MessageStream.Recv()
				if err == io.EOF {
					break
				}
				if err != nil {
					t.Fatalf("stream recv: %v", err)
				}
				if chunk != nil {
					sb.WriteString(chunk.Content)
				}
			}
			continue
		}
		if mv.Message != nil {
			sb.WriteString(mv.Message.Content)
		}
	}
	return sb.String()
}

// compactFeed builds a well-formed over-budget historical feed: a system
// message, one user message, and n assistant-toolcall + tool-result rounds.
func compactFeed(n int) []*schema.Message {
	msgs := []*schema.Message{
		schema.SystemMessage("you are a precise test agent"),
		schema.UserMessage("execute all steps in order"),
	}
	for i := 0; i < n; i++ {
		id := fmt.Sprintf("call-%d", i)
		msgs = append(msgs, schema.AssistantMessage("", []schema.ToolCall{{
			ID:       id,
			Function: schema.FunctionCall{Name: tools.EchoInfoName, Arguments: strings.Repeat("a", 120)},
		}}))
		msgs = append(msgs, schema.ToolMessage(strings.Repeat("b", 140), id))
	}
	return msgs
}

// recordingModel wraps a ScriptedModel and records every model input so
// tests can assert what the model actually saw after compaction.
type recordingModel struct {
	inner  model.ToolCallingChatModel
	mu     sync.Mutex
	inputs [][]*schema.Message
}

func (m *recordingModel) record(input []*schema.Message) {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := make([]*schema.Message, len(input))
	copy(cp, input)
	m.inputs = append(m.inputs, cp)
}

func (m *recordingModel) snapshot() [][]*schema.Message {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([][]*schema.Message, len(m.inputs))
	for i, input := range m.inputs {
		cp := make([]*schema.Message, len(input))
		copy(cp, input)
		out[i] = cp
	}
	return out
}

func (m *recordingModel) Generate(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	m.record(input)
	return m.inner.Generate(ctx, input, opts...)
}

func (m *recordingModel) Stream(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	m.record(input)
	return m.inner.Stream(ctx, input, opts...)
}

func (m *recordingModel) WithTools(tools []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	return m.inner.WithTools(tools)
}

func joinContent(msgs []*schema.Message) string {
	var sb strings.Builder
	for _, msg := range msgs {
		if msg == nil {
			continue
		}
		sb.WriteString(msg.Content)
	}
	return sb.String()
}

func countBytes(msgs []*schema.Message) int {
	n := 0
	for _, msg := range msgs {
		if msg == nil {
			continue
		}
		n += len(msg.Content)
		for _, tc := range msg.ToolCalls {
			n += len(tc.Function.Name) + len(tc.Function.Arguments)
		}
	}
	return n
}

// TestEngineSummarizationCompaction drives a real run whose feed crosses the
// trigger: the official summarization middleware calls the provider model
// once to compress history, and the final model input carries the summary.
func TestEngineSummarizationCompaction(t *testing.T) {
	ctx := context.Background()
	ts, err := tools.Builtin(nil).Resolve([]string{tools.EchoInfoName})
	if err != nil {
		t.Fatalf("resolve tools: %v", err)
	}
	scripted := NewScriptedModel(
		schema.AssistantMessage("SUMMARY-DIGEST-xyz", nil),
		schema.AssistantMessage("FINAL-ANSWER-yz", nil),
	)
	rec := &recordingModel{inner: scripted}
	eng, err := NewEngine(ctx, rec, ts, EngineConfig{
		StreamBuffer:         8,
		MaxEventPayloadBytes: 64 << 10,
		MaxContextBytes:      1 << 20, // large: feed budget clamp stays far away
		Compaction: &CompactionPolicy{
			Enabled: true, MaxTokens: 400, TriggerPercent: 50, KeepRecent: 100,
		},
	})
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}
	feed := compactFeed(16) // ~1k tokens, far above the 200-token trigger
	final := drainFinalText(t, eng.RunHistory(ctx, feed))
	if final != "FINAL-ANSWER-yz" {
		t.Fatalf("final answer = %q, want FINAL-ANSWER-yz", final)
	}
	inputs := rec.snapshot()
	if len(inputs) != 2 {
		t.Fatalf("model calls = %d, want 2 (summary generation + main loop)", len(inputs))
	}
	mainInput := inputs[1]
	if len(mainInput) >= len(inputs[0]) {
		t.Fatalf("summarized input (%d msgs) must be smaller than the summary input (%d msgs)", len(mainInput), len(inputs[0]))
	}
	if !strings.Contains(joinContent(mainInput), "SUMMARY-DIGEST-xyz") {
		t.Fatalf("summarized model input lacks the summary content: %q", joinContent(mainInput))
	}
}

// TestEngineReductionRunsBeforeSummarization drives a run whose feed crosses
// the trigger with a tight retention window: the deterministic reduction
// layer clears old tool payloads first (visible in the summarization input
// as placeholders), then summarization compresses what remains. This pins
// the middleware ordering (reduction first, summarization second).
func TestEngineReductionRunsBeforeSummarization(t *testing.T) {
	ctx := context.Background()
	ts, err := tools.Builtin(nil).Resolve([]string{tools.EchoInfoName})
	if err != nil {
		t.Fatalf("resolve tools: %v", err)
	}
	scripted := NewScriptedModel(
		schema.AssistantMessage("REDUCTION-SUMMARY-zz", nil),
		schema.AssistantMessage("FINAL-ANSWER-rx", nil),
	)
	rec := &recordingModel{inner: scripted}
	eng, err := NewEngine(ctx, rec, ts, EngineConfig{
		StreamBuffer:         8,
		MaxEventPayloadBytes: 64 << 10,
		MaxContextBytes:      1 << 20,
		Compaction: &CompactionPolicy{
			Enabled: true, MaxTokens: 800, TriggerPercent: 50, KeepRecent: 1,
		},
	})
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}
	feed := compactFeed(16)
	final := drainFinalText(t, eng.RunHistory(ctx, feed))
	if final != "FINAL-ANSWER-rx" {
		t.Fatalf("final answer = %q, want FINAL-ANSWER-rx", final)
	}
	inputs := rec.snapshot()
	if len(inputs) != 2 {
		t.Fatalf("model calls = %d, want 2 (summary generation + main loop)", len(inputs))
	}
	// The summary-generation input is the state after reduction: old tool
	// results carry the cleared placeholder instead of their real payload.
	summarizeInput := joinContent(inputs[0])
	if !strings.Contains(summarizeInput, "Old tool result content cleared") {
		t.Fatalf("reduction did not clear old tool results before summarization: %q", summarizeInput)
	}
	// The main loop then receives the final summarized history, whose size
	// is far below what the model would have seen without compaction.
	if !strings.Contains(joinContent(inputs[1]), "REDUCTION-SUMMARY-zz") {
		t.Fatalf("main model input lacks the summary content: %q", joinContent(inputs[1]))
	}
	if countBytes(inputs[1]) >= countBytes(inputs[0]) {
		t.Fatalf("summarized main input must be smaller than the summary-generation input")
	}
	if len(inputs[1]) >= len(inputs[0]) {
		t.Fatalf("summarized main input (%d msgs) must be smaller than the summary-generation input (%d msgs)", len(inputs[1]), len(inputs[0]))
	}
}

// fixedReplyModel is a deterministic domain.ChatModel whose reply does not
// echo the input (unlike provider.NewMock), so compaction size assertions
// stay meaningful.
type fixedReplyModel struct{ reply string }

func (m *fixedReplyModel) Stream(_ context.Context, _ []*domain.Message) (domain.Stream[*domain.Message], error) {
	return &fixedReplyStream{replies: []string{m.reply}}, nil
}

type fixedReplyStream struct {
	replies []string
	index   int
}

func (s *fixedReplyStream) Recv() (*domain.Message, error) {
	if s.index >= len(s.replies) {
		return nil, io.EOF
	}
	msg := &domain.Message{Role: domain.RoleAssistant, Content: s.replies[s.index]}
	s.index++
	return msg, nil
}

// TestServiceContextStatusAndCompactSession covers the durable session-level
// path: real measurements, manual compaction, durable summary folding, and
// the journaled context.compacted event.
func TestServiceContextStatusAndCompactSession(t *testing.T) {
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
	eng, err := NewEngine(ctx, WrapModel(&fixedReplyModel{reply: "summary text."}), ts, EngineConfig{
		StreamBuffer:         8,
		MaxEventPayloadBytes: 64 << 10,
		MaxContextBytes:      1 << 20,
		Compaction:           &CompactionPolicy{Enabled: true, MaxTokens: 400, TriggerPercent: 50, KeepRecent: 6},
	})
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}
	sink := newTestSink()
	svc := NewService(eng, "mock", "mock-v0", ServiceDeps{
		Journal: backend, Runs: backend, Messages: backend, Notes: backend, Sink: sink, Compactions: backend,
	})
	sessionID := domain.SessionID("sess-compact-1")
	for i := 0; i < 40; i++ {
		if err := backend.AppendMessage(ctx, domain.Message{
			ID: fmt.Sprintf("msg-%d", i), SessionID: sessionID, Role: domain.RoleUser,
			CreatedAt: int64(1_000 + i), Content: strings.Repeat("会话内容", 60),
		}); err != nil {
			t.Fatalf("append message: %v", err)
		}
	}

	before, err := svc.ContextStatus(ctx, sessionID)
	if err != nil {
		t.Fatalf("context status: %v", err)
	}
	if before.TotalMessages != 40 || !before.CompactionEnabled || before.ModelLimitTokens <= 0 {
		t.Fatalf("unexpected before status: %+v", before)
	}
	if !before.WouldCompact {
		t.Fatalf("40 long messages should read as over budget: %+v", before)
	}

	result, err := svc.CompactSession(ctx, sessionID)
	if err != nil {
		t.Fatalf("compact session: %v", err)
	}
	if result.Skipped || result.Folded == 0 {
		t.Fatalf("compaction result = %+v, want folded > 0", result)
	}

	comp, ok, err := backend.LatestSessionCompaction(ctx, sessionID)
	if err != nil || !ok || !strings.Contains(comp.Summary, "summary text.") {
		t.Fatalf("latest compaction = %+v ok=%v err=%v", comp, ok, err)
	}

	// The journal now holds a durable context.compacted event under the
	// synthetic compaction run id.
	events := replayAll(t, backend, comp.RunID)
	compacted := 0
	for _, ev := range events {
		if ev.Type == domain.EventContextCompacted {
			compacted++
		}
	}
	if compacted == 0 {
		t.Fatal("expected a journaled context.compacted event")
	}

	after, err := svc.ContextStatus(ctx, sessionID)
	if err != nil {
		t.Fatalf("context status after: %v", err)
	}
	if !after.HasCompactionSummary {
		t.Fatal("has_compaction_summary must be true after manual compaction")
	}
	if after.FeedMessages != 6+1 { // retained tail (6) + folded summary message
		t.Fatalf("feed messages = %d, want 7 (summary + 6 kept)", after.FeedMessages)
	}
	if after.FeedTokens >= before.FeedTokens {
		t.Fatalf("feed must shrink after compaction: %d -> %d", before.FeedTokens, after.FeedTokens)
	}
	if after.LastCompaction == nil || after.LastCompaction.Mode != "session" {
		t.Fatalf("last compaction = %+v, want session mode", after.LastCompaction)
	}

	// The next run's actual feed starts with the durable summary.
	msgs, _, _, err := svc.runMessages(ctx, sessionID, "continue", svc.engine)
	if err != nil {
		t.Fatalf("run messages: %v", err)
	}
	found := false
	for _, msg := range msgs {
		if strings.Contains(msg.Content, compactionSummaryPrefix) {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("next run feed must include the durable compaction summary")
	}
}

// TestScheduleEngineReload applies immediately when idle and defers while a
// run is registered, landing at the next idle application point.
func TestScheduleEngineReload(t *testing.T) {
	svc, backend, _ := newTestService(t, provider.NewMock())
	t.Cleanup(func() { _ = backend.Close() })
	var swaps atomicInt
	svc.deps.RebuildEngine = func(ctx context.Context, cfg EngineConfig) (*Engine, error) {
		swaps.add(1)
		ts, err := tools.Builtin(backend).Resolve([]string{tools.EchoInfoName})
		if err != nil {
			return nil, err
		}
		return NewEngine(ctx, WrapModel(provider.NewMock()), ts, cfg)
	}
	nextCfg := svc.engine.cfg
	nextCfg.Compaction = &CompactionPolicy{Enabled: true, MaxTokens: 999, TriggerPercent: 50, KeepRecent: 1}
	if err := svc.ScheduleEngineReload(nextCfg); err != nil {
		t.Fatalf("schedule reload: %v", err)
	}
	// Idle: applied immediately.
	if svc.engine.cfg.Compaction == nil || svc.engine.cfg.Compaction.MaxTokens != 999 {
		t.Fatalf("idle reload not applied: %+v", svc.engine.cfg.Compaction)
	}
	if swaps.value() != 1 {
		t.Fatalf("swaps = %d, want 1", swaps.value())
	}

	// Busy: deferred.
	ctx, cancel := context.WithCancel(context.Background())
	svc.mu.Lock()
	svc.active["run-busy"] = cancel
	svc.mu.Unlock()
	other := svc.engine.cfg
	other.Compaction = &CompactionPolicy{Enabled: true, MaxTokens: 1111, TriggerPercent: 60, KeepRecent: 2}
	if err := svc.ScheduleEngineReload(other); err != nil {
		t.Fatalf("schedule reload while busy: %v", err)
	}
	if swaps.value() != 1 {
		t.Fatalf("busy reload must not swap: swaps = %d", swaps.value())
	}
	// Still busy at the next run entry: nothing applied.
	if err := svc.applyPendingEngineReload(ctx, nil); err != nil {
		t.Fatalf("apply pending while busy: %v", err)
	}
	if svc.engine.cfg.Compaction.MaxTokens != 999 {
		t.Fatal("busy apply must not swap the engine")
	}
	// Idle again: the deferred rebuild lands.
	svc.mu.Lock()
	delete(svc.active, "run-busy")
	svc.mu.Unlock()
	if err := svc.applyPendingEngineReload(context.Background(), nil); err != nil {
		t.Fatalf("apply pending idle: %v", err)
	}
	if svc.engine.cfg.Compaction.MaxTokens != 1111 {
		t.Fatalf("deferred reload not applied: %+v", svc.engine.cfg.Compaction)
	}
	if swaps.value() != 2 {
		t.Fatalf("swaps = %d, want 2", swaps.value())
	}
}

// atomicInt is a tiny lock-free counter for the reload test.
type atomicInt struct {
	mu sync.Mutex
	v  int
}

func (a *atomicInt) add(n int)  { a.mu.Lock(); a.v += n; a.mu.Unlock() }
func (a *atomicInt) value() int { a.mu.Lock(); defer a.mu.Unlock(); return a.v }
