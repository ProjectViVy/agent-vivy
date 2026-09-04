package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/adk/middlewares/summarization"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
	"agent-vivy/internal/storage/sqlite"
	"agent-vivy/internal/testsupport"
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
		{Role: schema.User, UserInputMultiContent: []schema.MessageInputPart{{Type: schema.ChatMessagePartTypeText, Text: strings.Repeat("c", 400)}}},
	}
	got, err := countMessageTokens(msgs, nil)
	if err != nil {
		t.Fatalf("countMessageTokens: %v", err)
	}
	// 1200 text bytes + 48 envelope => ~312 tokens at 4 bytes/token.
	if got < 280 || got > 340 {
		t.Fatalf("countMessageTokens = %d, want ~312", got)
	}
}

func TestHistoryBytesTokensIncludesFileContexts(t *testing.T) {
	plain := domain.Message{Role: domain.RoleUser, Content: "inspect"}
	withFile := plain
	withFile.FileContexts = []domain.FileContext{{Path: "main.go", Name: "main.go", Size: 400, Content: []byte(strings.Repeat("x", 400))}}
	plainBytes, _ := historyBytesTokens([]domain.Message{plain})
	fileBytes, fileTokens := historyBytesTokens([]domain.Message{withFile})
	if fileBytes <= plainBytes+400 || fileTokens <= plainBytes/4 {
		t.Fatalf("file context not accounted: plain=%d file=%d/%d", plainBytes, fileBytes, fileTokens)
	}
}

func TestCompactionFoldRetainsFileContextSnapshots(t *testing.T) {
	feed := []domain.Message{
		{Content: "old"},
		{Content: "file", FileContexts: []domain.FileContext{{Path: "main.go", Content: []byte("package main"), Size: 12}}},
		{Content: "newer"},
		{Content: "newest"},
	}
	if got := fileContextSafeFoldIndex(feed, 3); got != 1 {
		t.Fatalf("safe fold index = %d, want 1", got)
	}
	if got := fileContextSafeFoldIndex(feed[1:], 2); got != 0 {
		t.Fatalf("leading snapshot fold index = %d, want 0", got)
	}
	feed[0].CreatedAt = 10
	feed[1].CreatedAt = 20
	feed[2].CreatedAt = 20
	feed[3].CreatedAt = 30
	if got := timestampSafeFoldIndex(feed, 2); got != 1 {
		t.Fatalf("same-millisecond safe fold index = %d, want 1", got)
	}
}

func TestSummarizationFinalizePreservesExactProjectFileMessage(t *testing.T) {
	fileMsg := &schema.Message{Role: schema.User, UserInputMultiContent: []schema.MessageInputPart{
		{Type: schema.ChatMessagePartTypeText, Text: "inspect"},
		{Type: schema.ChatMessagePartTypeText, Text: "\n\n[project file: main.go]\npackage main"},
	}}
	got, err := preserveProjectFileContextsFinalize(context.Background(), []*schema.Message{
		schema.SystemMessage("system"), schema.UserMessage("old"), fileMsg,
	}, schema.AssistantMessage("summary", nil))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) < 3 || got[len(got)-1] != fileMsg || !schemaMessageHasProjectFileContext(got[len(got)-1]) {
		t.Fatalf("finalized messages did not retain exact file snapshot: %+v", got)
	}
}

// TestMapperMapsSummarizationUsageEvent proves the Eino summarization
// middleware's generate_summary internal event is translated into a durable
// model.usage event so the hidden summary call is observable and accounted.
func TestMapperMapsSummarizationUsageEvent(t *testing.T) {
	m := newEventMapper("run-1", 0)
	m.setUsageRoutes("openai", "main-model", "summary-model")
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
	if p.TotalTokens != 15 || p.PromptTokens != 10 || p.Source != "summary" || p.Provider != "openai" || p.Model != "summary-model" {
		t.Fatalf("usage = %+v, want 10/5/15", p)
	}
	action.GenerateSummary.Phase = summarization.GenerateSummaryPhaseFailover
	events, err = m.onCustomizedAction(action)
	if err != nil || len(events) != 1 || json.Unmarshal(events[0].Payload, &p) != nil {
		t.Fatalf("failover usage event = %+v, err=%v", events, err)
	}
	if p.Provider != "openai" || p.Model != "main-model" || p.Source != "summary" {
		t.Fatalf("failover usage attribution = %+v", p)
	}
	// Unrelated customized actions are silent.
	if events, err := m.onCustomizedAction("whatever"); err != nil || len(events) != 0 {
		t.Fatalf("unknown customized action must be silent: %v %v", events, err)
	}
}

// TestResumeEventMapperRestoresSummaryUsageRoutesFromJournal pins the shared
// approval/question resume path. Both live and restart recovery eventually
// create this fresh mapper, so it must recover the main route from the first
// durable run.started event and retain the configured summary-model route.
func TestResumeEventMapperRestoresSummaryUsageRoutesFromJournal(t *testing.T) {
	ctx := context.Background()
	backend, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "resume-usage.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = backend.Close() })

	const (
		sessionID = domain.SessionID("sess-resume-usage")
		runID     = domain.RunID("run-resume-usage")
	)
	if err := backend.CreateSession(ctx, domain.Session{ID: sessionID, Title: "resume usage", CreatedAt: 1}); err != nil {
		t.Fatalf("create session: %v", err)
	}
	if err := backend.CreateRun(ctx, domain.Run{ID: runID, SessionID: sessionID, Status: domain.RunActive, CreatedAt: 1}); err != nil {
		t.Fatalf("create run: %v", err)
	}
	seed := newEventMapper(runID, 0)
	started := seed.build(domain.EventRunStarted, payloadRunStarted{Provider: "openai", Model: "main-model"})
	if _, err := backend.Append(ctx, storage.Commit{RunID: runID, Events: []domain.RunEvent{started}}); err != nil {
		t.Fatalf("append run.started: %v", err)
	}

	svc := &Service{
		engine: &Engine{cfg: EngineConfig{MaxEventPayloadBytes: 64 << 10, SummaryModelID: "summary-model"}},
		deps:   ServiceDeps{Journal: backend},
	}
	m := svc.newResumeEventMapper(ctx, runID)
	action := &summarization.CustomizedAction{
		Type: summarization.ActionTypeGenerateSummary,
		GenerateSummary: &summarization.GenerateSummaryAction{
			Attempt: 1,
			Phase:   summarization.GenerateSummaryPhasePrimary,
			ModelResponse: &schema.Message{Role: schema.Assistant, ResponseMeta: &schema.ResponseMeta{Usage: &schema.TokenUsage{
				PromptTokens: 8, CompletionTokens: 2, TotalTokens: 10,
			}}},
		},
	}
	primary, err := m.onCustomizedAction(action)
	if err != nil {
		t.Fatalf("map resumed primary summary: %v", err)
	}
	action.GenerateSummary.Phase = summarization.GenerateSummaryPhaseFailover
	failover, err := m.onCustomizedAction(action)
	if err != nil {
		t.Fatalf("map resumed failover summary: %v", err)
	}
	events := append(primary, failover...)
	if _, err := backend.Append(ctx, storage.Commit{RunID: runID, Events: events}); err != nil {
		t.Fatalf("append resumed summary usage: %v", err)
	}

	rows, err := backend.ListModelUsage(ctx, 0)
	if err != nil {
		t.Fatalf("list resumed summary usage: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("usage rows = %d, want 2", len(rows))
	}
	if rows[0].Provider != "openai" || rows[0].Model != "summary-model" || rows[0].Source != "summary" {
		t.Fatalf("primary resumed summary route = %+v", rows[0])
	}
	if rows[1].Provider != "openai" || rows[1].Model != "main-model" || rows[1].Source != "summary" {
		t.Fatalf("failover resumed summary route = %+v", rows[1])
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

// failoverFake stands in for the configured summary model: it records its
// inputs and fails its first failN Generate calls, then replies with the
// fixed summary text.
type failoverFake struct {
	mu     sync.Mutex
	calls  int
	failN  int
	reply  string
	called int
}

func (f *failoverFake) Generate(_ context.Context, input []*schema.Message, _ ...model.Option) (*schema.Message, error) {
	f.mu.Lock()
	f.calls++
	n := f.calls
	f.mu.Unlock()
	if n <= f.failN {
		return nil, errors.New("failoverFake: summary model down")
	}
	return schema.AssistantMessage(f.reply, nil), nil
}

func (f *failoverFake) Stream(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	msg, err := f.Generate(ctx, input, opts...)
	if err != nil {
		return nil, err
	}
	sr, sw := schema.Pipe[*schema.Message](1)
	sw.Send(msg, nil)
	sw.Close()
	return sr, nil
}

func (f *failoverFake) attempts() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}

// TestEngineSummaryModelPreferredWhenHealthy drives a run over the trigger
// with a healthy configured summary model: the summary comes from the
// summary model and the main model is only called for the final loop.
func TestEngineSummaryModelPreferredWhenHealthy(t *testing.T) {
	ctx := context.Background()
	ts, err := tools.Builtin(nil).Resolve([]string{tools.EchoInfoName})
	if err != nil {
		t.Fatalf("resolve tools: %v", err)
	}
	summary := &failoverFake{reply: "CHEAP-SUMMARY-cc"}
	rec := &recordingModel{inner: NewScriptedModel(
		schema.AssistantMessage("FINAL-ANSWER-mm", nil),
	)}
	eng, err := NewEngine(ctx, rec, ts, EngineConfig{
		StreamBuffer:         8,
		MaxEventPayloadBytes: 64 << 10,
		MaxContextBytes:      1 << 20,
		Compaction: &CompactionPolicy{
			Enabled: true, MaxTokens: 400, TriggerPercent: 50, KeepRecent: 100,
		},
		SummaryModel: summary,
	})
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}
	final := drainFinalText(t, eng.RunHistory(ctx, compactFeed(16)))
	if final != "FINAL-ANSWER-mm" {
		t.Fatalf("final answer = %q, want FINAL-ANSWER-mm", final)
	}
	if summary.attempts() != 1 {
		t.Fatalf("summary model attempts = %d, want 1", summary.attempts())
	}
	// The main model is called exactly once: the main loop after the
	// summary model already compressed the feed.
	inputs := rec.snapshot()
	if len(inputs) != 1 {
		t.Fatalf("main model calls = %d, want 1 (main never summarized)", len(inputs))
	}
	if !strings.Contains(joinContent(inputs[0]), "CHEAP-SUMMARY-cc") {
		t.Fatalf("main loop input lacks the summary model's summary: %q", joinContent(inputs[0]))
	}
}

// TestEngineSummaryModelFailsOverToMain drives the CMP-2 failover: the
// configured summary model errors, the middleware falls back to the main
// chat model exactly once, and the run still completes with the summary.
func TestEngineSummaryModelFailsOverToMain(t *testing.T) {
	ctx := context.Background()
	ts, err := tools.Builtin(nil).Resolve([]string{tools.EchoInfoName})
	if err != nil {
		t.Fatalf("resolve tools: %v", err)
	}
	summary := &failoverFake{failN: 1, reply: "NEVER-REACHED"}
	rec := &recordingModel{inner: NewScriptedModel(
		schema.AssistantMessage("MAIN-FAILOVER-SUMMARY", nil),
		schema.AssistantMessage("FINAL-ANSWER-ff", nil),
	)}
	eng, err := NewEngine(ctx, rec, ts, EngineConfig{
		StreamBuffer:         8,
		MaxEventPayloadBytes: 64 << 10,
		MaxContextBytes:      1 << 20,
		Compaction: &CompactionPolicy{
			Enabled: true, MaxTokens: 400, TriggerPercent: 50, KeepRecent: 100,
		},
		SummaryModel: summary,
	})
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}
	final := drainFinalText(t, eng.RunHistory(ctx, compactFeed(16)))
	if final != "FINAL-ANSWER-ff" {
		t.Fatalf("final answer = %q, want FINAL-ANSWER-ff", final)
	}
	if summary.attempts() != 1 {
		t.Fatalf("summary model attempts = %d, want 1 (one failover attempt, no retry)", summary.attempts())
	}
	inputs := rec.snapshot()
	if len(inputs) != 2 {
		t.Fatalf("main model calls = %d, want 2 (failover summary + main loop)", len(inputs))
	}
	// The failover summary input mirrors the default shape: the middleware's
	// system instruction first, the non-system feed in the middle, the user
	// instruction last — the original leading system message is not
	// duplicated.
	failoverInput := inputs[0]
	if len(failoverInput) == 0 || failoverInput[0].Role != schema.System {
		t.Fatalf("failover input must start with the summary system instruction, got %v", failoverInput)
	}
	for _, msg := range failoverInput[1:] {
		if msg != nil && msg.Role == schema.System {
			t.Fatalf("failover input duplicates a system message: %q", msg.Content)
		}
	}
	if !strings.Contains(joinContent(failoverInput), "execute all steps in order") {
		t.Fatalf("failover input lost the original feed: %q", joinContent(failoverInput))
	}
	if !strings.Contains(joinContent(inputs[1]), "MAIN-FAILOVER-SUMMARY") {
		t.Fatalf("main loop input lacks the failover summary: %q", joinContent(inputs[1]))
	}
}

// fixedReplyModel is a deterministic domain.ChatModel whose reply does not
// echo the input, so compaction size assertions
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
	svc := NewService(eng, "test", "test-model", ServiceDeps{
		Journal: backend, Runs: backend, Messages: backend, Notes: backend, Sessions: backend, Sink: sink, Compactions: backend,
	})
	sessionID := domain.SessionID("sess-compact-1")
	if err := backend.CreateSession(ctx, domain.Session{ID: sessionID, Title: "compact", CreatedAt: 1}); err != nil {
		t.Fatal(err)
	}
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
	if !before.TokenCountsEstimated || before.ModelLimitKnown {
		t.Fatalf("context truth flags = estimated:%v limit-known:%v, want true/false for fallback metadata", before.TokenCountsEstimated, before.ModelLimitKnown)
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
	msgs, _, _, err := svc.runMessages(ctx, sessionID, "continue", svc.engine, domain.FaceWeb)
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
	if err := svc.DeleteSession(ctx, sessionID); err != nil {
		t.Fatalf("delete compacted session: %v", err)
	}
	if remaining := replayAll(t, backend, comp.RunID); len(remaining) != 0 {
		t.Fatalf("synthetic compaction journal survived session deletion: %+v", remaining)
	}
}

// TestScheduleEngineReload applies immediately when idle and defers while a
// run is registered, landing at the next idle application point.
func TestScheduleEngineReload(t *testing.T) {
	svc, backend, _ := newTestService(t, testsupport.NewEchoModel())
	t.Cleanup(func() { _ = backend.Close() })
	var swaps atomicInt
	svc.deps.RebuildEngine = func(ctx context.Context, cfg EngineConfig) (*Engine, error) {
		swaps.add(1)
		ts, err := tools.Builtin(backend).Resolve([]string{tools.EchoInfoName})
		if err != nil {
			return nil, err
		}
		return NewEngine(ctx, WrapModel(testsupport.NewEchoModel()), ts, cfg)
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

// TestEngineReductionOffloadsClearedToolResults drives a clear over the
// trigger with the offload backend wired (CMP-1): every cleared echo result
// must land in the run workspace under compaction/clear/<call-id>, the
// placeholder must name the persisted path, and the offloaded content must
// keep the original tool output.
func TestEngineReductionOffloadsClearedToolResults(t *testing.T) {
	ctx := tools.WithRunID(context.Background(), "run-offload")
	wsRoot := t.TempDir()
	manager, err := NewWorkspaceManager(wsRoot)
	if err != nil {
		t.Fatalf("workspace manager: %v", err)
	}
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
		OffloadBackend: NewEinoFilesystemBackend(manager, nil),
	})
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}
	final := drainFinalText(t, eng.RunHistory(ctx, compactFeed(16)))
	if final != "FINAL-ANSWER-rx" {
		t.Fatalf("final answer = %q, want FINAL-ANSWER-rx", final)
	}
	inputs := rec.snapshot()
	if len(inputs) != 2 {
		t.Fatalf("model calls = %d, want 2", len(inputs))
	}
	summarizeInput := joinContent(inputs[0])
	if !strings.Contains(summarizeInput, "persisted-output") || !strings.Contains(summarizeInput, "compaction/clear/") {
		t.Fatalf("clear placeholder does not name the offload path: %q", summarizeInput)
	}
	offloadRoot := filepath.Join(wsRoot, "run-offload", "compaction", "clear")
	entries, err := os.ReadDir(offloadRoot)
	if err != nil {
		t.Fatalf("read offload dir: %v", err)
	}
	if len(entries) == 0 {
		t.Fatalf("no offload files written under %s", offloadRoot)
	}
	for _, entry := range entries {
		data, readErr := os.ReadFile(filepath.Join(offloadRoot, entry.Name()))
		if readErr != nil {
			t.Fatalf("read offload %s: %v", entry.Name(), readErr)
		}
		if !strings.Contains(string(data), strings.Repeat("b", 140)) {
			snippet := string(data)
			if len(snippet) > 160 {
				snippet = snippet[:160]
			}
			t.Fatalf("offload %s lost the tool result content: %q", entry.Name(), snippet)
		}
	}
}

// TestSafeOffloadCallID pins the call-id allowlist: provider ids pass
// through unchanged, traversal and separators fall back to generated ids.
func TestSafeOffloadCallID(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"call_0", "call_0"},
		{"chatcmpl-ABC-123", "chatcmpl-ABC-123"},
		{"", ""},
		{"../../etc/passwd", ""},
		{`..\evil`, ""},
		{"has space", ""},
		{"toolu_01abc/def", ""},
		{strings.Repeat("a", 129), ""},
		{strings.Repeat("a", 128), strings.Repeat("a", 128)},
	}
	for _, tc := range cases {
		if got := safeOffloadCallID(tc.in); got != tc.want {
			t.Fatalf("safeOffloadCallID(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
