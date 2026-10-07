package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage/sqlite"
	"agent-vivy/internal/tools"
)

// overflowScriptedModel fails Stream calls in the window
// (skipFirst, skipFirst+failCount] with a classified provider overflow
// error, then answers. Generate serves the compaction summarizer (system
// prompt carries the summary contract).
type overflowScriptedModel struct {
	mu          sync.Mutex
	streamCalls int
	skipFirst   int
	failCount   int
	inputs      [][]*schema.Message
	response    *schema.Message
	errText     string
	lengthStop  bool
}

func (m *overflowScriptedModel) Stream(_ context.Context, input []*schema.Message, _ ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	m.mu.Lock()
	m.streamCalls++
	m.inputs = append(m.inputs, input)
	n := m.streamCalls
	skipFirst := m.skipFirst
	failCount := m.failCount
	errText := m.errText
	lengthStop := m.lengthStop
	resp := m.response
	m.mu.Unlock()
	if n > skipFirst && n <= skipFirst+failCount {
		if errText == "" {
			errText = "maximum context length is 4096 tokens, but the prompt has 9000"
		}
		return nil, errors.New(errText)
	}
	if lengthStop && n > skipFirst && n <= skipFirst+1 {
		return oneShot(&schema.Message{
			Role:         schema.Assistant,
			ResponseMeta: &schema.ResponseMeta{FinishReason: "length"},
		}), nil
	}
	if resp == nil {
		resp = &schema.Message{Role: schema.Assistant, Content: "recovered reply"}
	}
	return oneShot(resp), nil
}

func (m *overflowScriptedModel) Generate(ctx context.Context, input []*schema.Message, _ ...model.Option) (*schema.Message, error) {
	for _, msg := range input {
		if msg != nil && msg.Role == schema.System && strings.Contains(msg.Content, "压缩") {
			return &schema.Message{Role: schema.Assistant, Content: "compacted history summary"}, nil
		}
	}
	// Non-summary Generate mirrors Stream behavior for parity.
	r, err := m.Stream(ctx, input)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return r.Recv()
}

func (m *overflowScriptedModel) WithTools(_ []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	return m, nil
}

func (m *overflowScriptedModel) calls() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.streamCalls
}

func (m *overflowScriptedModel) inputTexts(call int) []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	if call < 1 || call > len(m.inputs) {
		return nil
	}
	var out []string
	for _, msg := range m.inputs[call-1] {
		if msg != nil {
			out = append(out, string(msg.Role)+": "+msg.Content)
		}
	}
	return out
}

var _ model.ToolCallingChatModel = (*overflowScriptedModel)(nil)

// newOverflowService is newScriptedService plus a compaction store and an
// enabled compaction policy — the decider requires all three.
func newOverflowService(t *testing.T, m *overflowScriptedModel) (*Service, *sqlite.Backend) {
	t.Helper()
	ctx := context.Background()
	backend, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "overflow.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = backend.Close() })
	ts, err := tools.Builtin(backend).Resolve([]string{tools.EchoInfoName})
	if err != nil {
		t.Fatalf("resolve tools: %v", err)
	}
	checkpoints, err := NewVersionedCheckpointStore(backend.Blobs(), "test")
	if err != nil {
		t.Fatalf("checkpoint store: %v", err)
	}
	eng, err := NewEngine(ctx, m, ts, EngineConfig{
		StreamBuffer: 8, MaxEventPayloadBytes: 64 << 10, Checkpoints: checkpoints,
		Compaction: &CompactionPolicy{Enabled: true, KeepRecent: 1},
	})
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}
	svc := NewService(eng, "test", "test-model", ServiceDeps{
		Journal: backend, Runs: backend, Messages: backend, Notes: backend,
		Sessions: backend, Sink: newTestSink(), Truncations: backend,
		Compactions: backend,
	})
	return svc, backend
}

func eventPayload(t *testing.T, ev domain.RunEvent) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal(ev.Payload, &out); err != nil {
		t.Fatalf("payload %s: %v", ev.Type, err)
	}
	return out
}

func eventsOfType(events []domain.RunEvent, typ domain.EventType) []domain.RunEvent {
	var out []domain.RunEvent
	for _, ev := range events {
		if ev.Type == typ {
			out = append(out, ev)
		}
	}
	return out
}

func TestOverflowCompactionRetryRecoversRun(t *testing.T) {
	m := &overflowScriptedModel{skipFirst: 1, failCount: 1, errText: "prompt is too long"}
	svc, backend := newOverflowService(t, m)
	ctx := context.Background()
	mustCreateSession(t, backend, "sess-overflow")

	// First run seeds pre-run history (user + assistant rows).
	seedID, err := svc.Run(ctx, "sess-overflow", "first exchange")
	if err != nil {
		t.Fatalf("seed run: %v", err)
	}
	waitFor(t, "seed run completes", func() bool {
		return runTerminal(t, backend, seedID) == domain.RunCompleted
	})

	runID, err := svc.Run(ctx, "sess-overflow", "overflow me")
	if err != nil {
		t.Fatalf("overflow run: %v", err)
	}
	waitFor(t, "overflow run completes", func() bool {
		return runTerminal(t, backend, runID) == domain.RunCompleted
	})

	// Three model calls: seed answer, overflowing call, compacted retry.
	if got := m.calls(); got != 3 {
		t.Fatalf("model calls = %d, want 3", got)
	}
	retry := m.inputTexts(3)
	joined := strings.Join(retry, "\n")
	if !strings.Contains(joined, "compacted history summary") {
		t.Fatalf("retry input lacks compaction summary: %v", retry)
	}
	if !strings.Contains(joined, "overflow me") {
		t.Fatalf("retry input lacks pending user turn: %v", retry)
	}

	events := journalEvents(t, backend, runID)
	started := eventsOfType(events, domain.EventAutoRetryStarted)
	if len(started) != 1 {
		t.Fatalf("auto_retry.started count = %d, want 1", len(started))
	}
	if r := eventPayload(t, started[0])["reason"]; !strings.Contains(r.(string), overflowRecoveryReason) {
		t.Fatalf("auto_retry.started reason = %v", r)
	}
	// WillRetryError may surface as provider.retry on some Eino paths; the
	// auto_retry pair is the guaranteed observability contract either way.
	retries := eventsOfType(events, domain.EventProviderRetry)
	if len(retries) > 1 {
		t.Fatalf("provider.retry duplicated: %v", retries)
	}
	if len(retries) == 1 && eventPayload(t, retries[0])["reason"] != overflowRecoveryReason {
		t.Fatalf("provider.retry reason = %v", eventPayload(t, retries[0])["reason"])
	}
	finished := eventsOfType(events, domain.EventAutoRetryFinished)
	if len(finished) != 1 || eventPayload(t, finished[0])["success"] != true {
		t.Fatalf("auto_retry.finished events = %v", finished)
	}
	compacted := eventsOfType(events, domain.EventContextCompacted)
	if len(compacted) != 1 || eventPayload(t, compacted[0])["mode"] != "overflow-recovery" {
		t.Fatalf("context.compacted events = %v", compacted)
	}
}

func TestOverflowSecondFailureEndsRun(t *testing.T) {
	// The retry itself overflows: one recovery per run, then honest failure.
	m := &overflowScriptedModel{skipFirst: 1, failCount: 2}
	svc, backend := newOverflowService(t, m)
	ctx := context.Background()
	mustCreateSession(t, backend, "sess-overflow2")

	seedID, err := svc.Run(ctx, "sess-overflow2", "seed")
	if err != nil {
		t.Fatalf("seed run: %v", err)
	}
	waitFor(t, "seed run completes", func() bool {
		return runTerminal(t, backend, seedID) == domain.RunCompleted
	})
	runID, err := svc.Run(ctx, "sess-overflow2", "overflow twice")
	if err != nil {
		t.Fatalf("overflow run admission: %v", err)
	}
	waitFor(t, "overflow run fails", func() bool {
		return runTerminal(t, backend, runID) == domain.RunFailed
	})

	events := journalEvents(t, backend, runID)
	compacted := eventsOfType(events, domain.EventContextCompacted)
	if len(compacted) != 1 {
		t.Fatalf("context.compacted count = %d, want exactly 1 (no compaction loop)", len(compacted))
	}
	finished := eventsOfType(events, domain.EventAutoRetryFinished)
	if len(finished) != 1 || eventPayload(t, finished[0])["success"] != false {
		t.Fatalf("auto_retry.finished events = %v", finished)
	}
	failed := eventsOfType(events, domain.EventRunFailed)
	if len(failed) == 0 {
		t.Fatalf("missing run.failed terminal")
	}
}

func TestOverflowLengthStopTriggersRecovery(t *testing.T) {
	// pi case 3: silent server-side truncation — no error, stop_reason
	// "length" with empty output. Same compact-and-retry path.
	m := &overflowScriptedModel{skipFirst: 1, lengthStop: true}
	svc, backend := newOverflowService(t, m)
	ctx := context.Background()
	mustCreateSession(t, backend, "sess-lengthstop")

	seedID, err := svc.Run(ctx, "sess-lengthstop", "seed")
	if err != nil {
		t.Fatalf("seed run: %v", err)
	}
	waitFor(t, "seed run completes", func() bool {
		return runTerminal(t, backend, seedID) == domain.RunCompleted
	})
	runID, err := svc.Run(ctx, "sess-lengthstop", "length stop me")
	if err != nil {
		t.Fatalf("length-stop run: %v", err)
	}
	waitFor(t, "length-stop run completes", func() bool {
		return runTerminal(t, backend, runID) == domain.RunCompleted
	})
	if got := m.calls(); got != 3 {
		t.Fatalf("model calls = %d, want 3 (seed, truncated, retried)", got)
	}
	if got := len(eventsOfType(journalEvents(t, backend, runID), domain.EventAutoRetryFinished)); got != 1 {
		t.Fatalf("auto_retry.finished count = %d, want 1", got)
	}
}

func TestOverflowDisabledPolicyAcceptsError(t *testing.T) {
	// No compaction policy: the decider declines, the error propagates.
	m := &overflowScriptedModel{failCount: 1}
	ctx := context.Background()
	backend, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "overflow-off.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = backend.Close() })
	ts, err := tools.Builtin(backend).Resolve([]string{tools.EchoInfoName})
	if err != nil {
		t.Fatalf("resolve tools: %v", err)
	}
	eng, err := NewEngine(ctx, m, ts, EngineConfig{StreamBuffer: 8, MaxEventPayloadBytes: 64 << 10})
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}
	svc := NewService(eng, "test", "test-model", ServiceDeps{
		Journal: backend, Runs: backend, Messages: backend, Sessions: backend,
		Sink: newTestSink(), Compactions: backend,
	})
	mustCreateSession(t, backend, "sess-off")
	runID, err := svc.Run(ctx, "sess-off", "boom")
	if err != nil {
		t.Fatalf("run admission: %v", err)
	}
	waitFor(t, "run fails honestly", func() bool {
		return runTerminal(t, backend, runID) == domain.RunFailed
	})
	if got := len(eventsOfType(journalEvents(t, backend, runID), domain.EventContextCompacted)); got != 0 {
		t.Fatalf("unexpected compaction without policy: %d", got)
	}
}

func TestIsContextOverflowClassifier(t *testing.T) {
	cases := []struct {
		err  string
		want bool
	}{
		{"maximum context length is 8192 tokens", true},
		{"context_length_exceeded", true},
		{"prompt is too long", true},
		{"request_too_large", true},
		{"rate limit exceeded: 60 rpm", false},
		{"too many requests, throttled", false},
		{"connection refused", false},
		{"invalid api key", false},
	}
	for _, tc := range cases {
		if got := isContextOverflow(errors.New(tc.err)); got != tc.want {
			t.Errorf("isContextOverflow(%q) = %v, want %v", tc.err, got, tc.want)
		}
	}
	// Wrapped chains still classify.
	if !isContextOverflow(errors.Join(errors.New("model call failed"), errors.New("context length exceeded"))) {
		t.Errorf("joined overflow chain not classified")
	}
}

func TestIsEmptyLengthStop(t *testing.T) {
	truncated := &schema.Message{Role: schema.Assistant, ResponseMeta: &schema.ResponseMeta{FinishReason: "length"}}
	if !isEmptyLengthStop(truncated) {
		t.Errorf("empty length-stop message not detected")
	}
	withContent := &schema.Message{Role: schema.Assistant, Content: "partial", ResponseMeta: &schema.ResponseMeta{FinishReason: "length"}}
	if isEmptyLengthStop(withContent) {
		t.Errorf("length-stop with content should not trigger recovery")
	}
	if isEmptyLengthStop(&schema.Message{Role: schema.Assistant, ResponseMeta: &schema.ResponseMeta{FinishReason: "stop"}}) {
		t.Errorf("plain stop should not trigger recovery")
	}
	if isEmptyLengthStop(nil) {
		t.Errorf("nil message should not trigger recovery")
	}
}
