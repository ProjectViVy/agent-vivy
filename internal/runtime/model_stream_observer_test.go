package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage/sqlite"
	"agent-vivy/internal/tools"
	"path/filepath"
)

// ---------------------------------------------------------------------
// Test observer: records the whole Begin -> StreamOpened -> Chunk* -> End
// lifecycle per call.

type recordingObserver struct {
	mu       sync.Mutex
	inputs   []modelCallInput
	metas    []modelCallMeta
	opened   []modelCallMeta
	chunks   []*schema.Message
	results  []modelCallResult
	beginErr error
	chunkErr error
	endErr   error
}

func (o *recordingObserver) Begin(_ context.Context, in modelCallInput) (modelCallMeta, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.beginErr != nil {
		return modelCallMeta{}, o.beginErr
	}
	meta := modelCallMeta{CallID: fmt.Sprintf("call-%d", len(o.metas)+1)}
	o.inputs = append(o.inputs, in)
	o.metas = append(o.metas, meta)
	return meta, nil
}

func (o *recordingObserver) StreamOpened(meta modelCallMeta) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.opened = append(o.opened, meta)
}

func (o *recordingObserver) Chunk(_ context.Context, _ modelCallMeta, chunk *schema.Message) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.chunkErr != nil {
		return o.chunkErr
	}
	o.chunks = append(o.chunks, chunk)
	return nil
}

func (o *recordingObserver) End(_ context.Context, _ modelCallMeta, result modelCallResult) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.results = append(o.results, result)
	return o.endErr
}

func (o *recordingObserver) counts() (begins, opened, chunks, ends int) {
	o.mu.Lock()
	defer o.mu.Unlock()
	return len(o.metas), len(o.opened), len(o.chunks), len(o.results)
}

func observedCtx(o modelCallObserver) context.Context {
	if o == nil {
		return context.Background()
	}
	return withModelCallObserverFactory(context.Background(), func(modelCallRoute) modelCallObserver { return o })
}

// TestObserverLifecycleGenerateAndStream asserts the canonical hook order
// for both invocation modes: Begin (mode tagged, input digested) ->
// StreamOpened -> Chunk* -> End(ResponseComplete for a clean EOF).
func TestObserverLifecycleGenerateAndStream(t *testing.T) {
	streamObs := &recordingObserver{}
	sr, sw := schema.Pipe[*schema.Message](8)
	inner := &fakeChatModel{streamReader: sr}
	go func() {
		defer sw.Close()
		sw.Send(schema.AssistantMessage("hel", nil), nil)
		sw.Send(&schema.Message{
			Role:    schema.Assistant,
			Content: "lo",
			ResponseMeta: &schema.ResponseMeta{Usage: &schema.TokenUsage{
				PromptTokens: 5, CompletionTokens: 2, TotalTokens: 7,
			}},
		}, nil)
	}()

	out, err := observeChatModel(inner).Stream(observedCtx(streamObs), []*schema.Message{schema.UserMessage("hi")})
	if err != nil {
		t.Fatalf("stream: %v", err)
	}
	var got []*schema.Message
	for {
		c, err := out.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("recv: %v", err)
		}
		got = append(got, c)
	}
	if len(got) != 2 {
		t.Fatalf("delivered %d chunks, want 2", len(got))
	}
	begins, opened, chunks, ends := streamObs.counts()
	if begins != 1 || opened != 1 || chunks != 2 || ends != 1 {
		t.Fatalf("lifecycle = Begin:%d Opened:%d Chunk:%d End:%d, want 1/1/2/1", begins, opened, chunks, ends)
	}
	if streamObs.inputs[0].Mode != "stream" {
		t.Fatalf("begin mode = %q, want stream", streamObs.inputs[0].Mode)
	}
	if len(streamObs.inputs[0].Messages) != 1 {
		t.Fatalf("begin input missing messages: %+v", streamObs.inputs[0])
	}
	res := streamObs.results[0]
	if res.Err != nil || !res.ResponseComplete {
		t.Fatalf("stream end = %+v, want complete no error", res)
	}
	if res.Usage == nil || res.Usage.TotalTokens != 7 {
		t.Fatalf("stream end usage = %+v, want total 7", res.Usage)
	}

	genObs := &recordingObserver{}
	gmsg := &schema.Message{
		Role:    schema.Assistant,
		Content: "done",
		ResponseMeta: &schema.ResponseMeta{Usage: &schema.TokenUsage{
			PromptTokens: 3, CompletionTokens: 4, TotalTokens: 7,
		}},
	}
	msg, err := observeChatModel(&fakeChatModel{generateMsg: gmsg}).Generate(observedCtx(genObs), []*schema.Message{schema.UserMessage("hi")})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if msg != gmsg {
		t.Fatalf("generate returned %v, want the inner message", msg)
	}
	begins, opened, chunks, ends = genObs.counts()
	if begins != 1 || opened != 1 || chunks != 1 || ends != 1 {
		t.Fatalf("generate lifecycle = %d/%d/%d/%d, want 1/1/1/1", begins, opened, chunks, ends)
	}
	if genObs.inputs[0].Mode != "generate" {
		t.Fatalf("generate mode = %q, want generate", genObs.inputs[0].Mode)
	}
	res = genObs.results[0]
	if res.Err != nil || !res.ResponseComplete || res.Usage == nil || res.Usage.TotalTokens != 7 {
		t.Fatalf("generate end = %+v", res)
	}
}

// TestObserverRecordsStreamSetupFailure: when the inner Stream call itself
// fails, the observer still sees Begin -> End(Err) — a failed call is a
// journal fact too.
func TestObserverRecordsStreamSetupFailure(t *testing.T) {
	innerErr := errors.New("provider refused")
	obs := &recordingObserver{}
	_, err := observeChatModel(&fakeChatModel{streamErr: innerErr}).Stream(observedCtx(obs), nil)
	if !errors.Is(err, innerErr) {
		t.Fatalf("stream error = %v, want %v", err, innerErr)
	}
	begins, opened, chunks, ends := obs.counts()
	if begins != 1 || opened != 0 || chunks != 0 || ends != 1 {
		t.Fatalf("lifecycle = %d/%d/%d/%d, want 1/0/0/1", begins, opened, chunks, ends)
	}
	if !errors.Is(obs.results[0].Err, innerErr) {
		t.Fatalf("end err = %v, want %v", obs.results[0].Err, innerErr)
	}
}

func TestObservedGenerateSettlementFailure(t *testing.T) {
	settlementErr := errors.New("usage settlement unavailable")
	obs := &recordingObserver{endErr: settlementErr}
	msg := schema.AssistantMessage("must not be reported as success", nil)
	got, err := observeChatModel(&fakeChatModel{generateMsg: msg}).Generate(observedCtx(obs), nil)
	if got != nil || !errors.Is(err, settlementErr) {
		t.Fatalf("Generate = (%v, %v), want nil and settlement error", got, err)
	}
	if begins, opened, chunks, ends := obs.counts(); begins != 1 || opened != 1 || chunks != 1 || ends != 1 {
		t.Fatalf("lifecycle = %d/%d/%d/%d, want 1/1/1/1", begins, opened, chunks, ends)
	}
}

func TestObservedStreamSettlementFailure(t *testing.T) {
	settlementErr := errors.New("finish append unavailable")
	obs := &recordingObserver{endErr: settlementErr}
	upstream, writer := schema.Pipe[*schema.Message](1)
	writer.Send(schema.AssistantMessage("partial", nil), nil)
	writer.Close()
	out, err := observeChatModel(&fakeChatModel{streamReader: upstream}).Stream(observedCtx(obs), nil)
	if err != nil {
		t.Fatal(err)
	}
	if chunk, recvErr := out.Recv(); recvErr != nil || chunk.Content != "partial" {
		t.Fatalf("first Recv=(%v,%v), want partial chunk", chunk, recvErr)
	}
	if _, recvErr := out.Recv(); !errors.Is(recvErr, settlementErr) || errors.Is(recvErr, io.EOF) {
		t.Fatalf("terminal Recv error=%v, want settlement error before EOF", recvErr)
	}
	if begins, _, _, ends := obs.counts(); begins != 1 || ends != 1 {
		t.Fatalf("lifecycle = %d begins / %d ends, want 1/1", begins, ends)
	}
}

func TestObservedErrorsJoinSettlement(t *testing.T) {
	settlementErr := errors.New("journal settlement failed")
	providerErr := errors.New("provider failed")
	chunkErr := errors.New("chunk persistence failed")
	t.Run("generate provider", func(t *testing.T) {
		obs := &recordingObserver{endErr: settlementErr}
		_, err := observeChatModel(&fakeChatModel{generateErr: providerErr}).Generate(observedCtx(obs), nil)
		if !errors.Is(err, providerErr) || !errors.Is(err, settlementErr) {
			t.Fatalf("Generate error=%v, want provider and settlement causes", err)
		}
		if _, _, _, ends := obs.counts(); ends != 1 {
			t.Fatalf("End calls=%d, want 1", ends)
		}
	})
	t.Run("generate chunk", func(t *testing.T) {
		obs := &recordingObserver{chunkErr: chunkErr, endErr: settlementErr}
		_, err := observeChatModel(&fakeChatModel{generateMsg: schema.AssistantMessage("generated", nil)}).Generate(observedCtx(obs), nil)
		if !errors.Is(err, chunkErr) || !errors.Is(err, settlementErr) {
			t.Fatalf("Generate chunk error=%v, want chunk and settlement causes", err)
		}
		if _, _, _, ends := obs.counts(); ends != 1 {
			t.Fatalf("End calls=%d, want 1", ends)
		}
	})
	t.Run("stream setup", func(t *testing.T) {
		obs := &recordingObserver{endErr: settlementErr}
		_, err := observeChatModel(&fakeChatModel{streamErr: providerErr}).Stream(observedCtx(obs), nil)
		if !errors.Is(err, providerErr) || !errors.Is(err, settlementErr) {
			t.Fatalf("Stream setup error=%v, want provider and settlement causes", err)
		}
		if _, _, _, ends := obs.counts(); ends != 1 {
			t.Fatalf("End calls=%d, want 1", ends)
		}
	})
	t.Run("stream read", func(t *testing.T) {
		obs := &recordingObserver{endErr: settlementErr}
		upstream, writer := schema.Pipe[*schema.Message](1)
		writer.Send(nil, providerErr)
		out, err := observeChatModel(&fakeChatModel{streamReader: upstream}).Stream(observedCtx(obs), nil)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := out.Recv(); !errors.Is(err, providerErr) || !errors.Is(err, settlementErr) {
			t.Fatalf("stream Recv error=%v, want provider and settlement causes", err)
		}
		if _, _, _, ends := obs.counts(); ends != 1 {
			t.Fatalf("End calls=%d, want 1", ends)
		}
	})
	t.Run("chunk", func(t *testing.T) {
		obs := &recordingObserver{chunkErr: chunkErr, endErr: settlementErr}
		upstream, writer := schema.Pipe[*schema.Message](1)
		writer.Send(schema.AssistantMessage("chunk", nil), nil)
		writer.Close()
		out, err := observeChatModel(&fakeChatModel{streamReader: upstream}).Stream(observedCtx(obs), nil)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := out.Recv(); !errors.Is(err, chunkErr) || !errors.Is(err, settlementErr) {
			t.Fatalf("stream Recv error=%v, want chunk and settlement causes", err)
		}
		if _, _, _, ends := obs.counts(); ends != 1 {
			t.Fatalf("End calls=%d, want 1", ends)
		}
	})
}

func TestObservedCloseDuringSettlementDoesNotBlock(t *testing.T) {
	providerErr := errors.New("upstream stopped")
	settlementErr := errors.New("settlement failed")
	entered := make(chan struct{})
	release := make(chan struct{})
	endReturned := make(chan struct{})
	obs := &blockingEndObserver{
		entered: entered, release: release, returned: endReturned,
		endErr: settlementErr,
	}
	upstream, upstreamWriter := schema.Pipe[*schema.Message](0)
	sentProviderError := make(chan struct{})
	go func() {
		upstreamWriter.Send(schema.AssistantMessage("partial", nil), nil)
		upstreamWriter.Send(nil, providerErr)
		close(sentProviderError)
	}()
	out, err := observeChatModel(&fakeChatModel{streamReader: upstream}).Stream(observedCtx(obs), nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := out.Recv(); err != nil {
		t.Fatalf("first Recv: %v", err)
	}
	waitObserverSignal(t, entered, "End entered")
	waitObserverSignal(t, sentProviderError, "provider error delivered")
	out.Close()
	probeResult := make(chan bool, 1)
	go func() { probeResult <- upstreamWriter.Send(schema.AssistantMessage("probe", nil), nil) }()
	close(release)
	waitObserverSignal(t, endReturned, "End returned")
	select {
	case closed := <-probeResult:
		if !closed {
			t.Fatal("upstream send succeeded after pump exit; reader was not closed")
		}
	case <-time.After(time.Second):
		t.Fatal("upstream producer remained blocked after downstream close and settlement")
	}
	if got := obs.endCalls(); got != 1 {
		t.Fatalf("End calls=%d, want 1", got)
	}
}

func waitObserverSignal(t *testing.T, signal <-chan struct{}, label string) {
	t.Helper()
	select {
	case <-signal:
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for %s", label)
	}
}

type blockingEndObserver struct {
	entered  chan struct{}
	release  chan struct{}
	returned chan struct{}
	endErr   error
	mu       sync.Mutex
	ends     int
}

func (o *blockingEndObserver) Begin(context.Context, modelCallInput) (modelCallMeta, error) {
	return modelCallMeta{CallID: "blocking"}, nil
}
func (o *blockingEndObserver) StreamOpened(modelCallMeta) {}
func (o *blockingEndObserver) Chunk(context.Context, modelCallMeta, *schema.Message) error {
	return nil
}
func (o *blockingEndObserver) End(context.Context, modelCallMeta, modelCallResult) error {
	o.mu.Lock()
	o.ends++
	o.mu.Unlock()
	close(o.entered)
	<-o.release
	close(o.returned)
	return o.endErr
}
func (o *blockingEndObserver) endCalls() int {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.ends
}

// TestObserverBoundToolsPreservesCallScope: WithTools flows into the
// Begin input while each invocation still gets its own meta scope.
func TestObserverBoundToolsPreservesCallScope(t *testing.T) {
	obs := &recordingObserver{}
	wrapped := observeChatModel(&fakeChatModel{streamReader: closedStream(t)})
	bound, err := wrapped.WithTools([]*schema.ToolInfo{
		{Name: "tool_a", Desc: "a"},
		{Name: "tool_b", Desc: "b"},
	})
	if err != nil {
		t.Fatalf("with tools: %v", err)
	}
	// Rebinding appends without losing the earlier scope.
	rebound, err := bound.(model.ToolCallingChatModel).WithTools([]*schema.ToolInfo{{Name: "tool_b"}, {Name: "tool_c"}})
	if err != nil {
		t.Fatalf("rebind: %v", err)
	}
	reader, err := rebound.Stream(observedCtx(obs), nil)
	if err != nil {
		t.Fatalf("stream: %v", err)
	}
	for {
		if _, err := reader.Recv(); err != nil {
			break
		}
	}
	if len(obs.inputs) != 1 {
		t.Fatalf("begins = %d, want 1", len(obs.inputs))
	}
	names := toolInfoNames(obs.inputs[0].Tools)
	if len(names) != 3 || names[0] != "tool_a" || names[1] != "tool_b" || names[2] != "tool_c" {
		t.Fatalf("bound tools = %v, want [tool_a tool_b tool_c]", names)
	}

	// A second call on the same bound model is a new call scope.
	obs2 := &recordingObserver{}
	fresh := observeChatModel(&fakeChatModel{streamFactory: func() (*schema.StreamReader[*schema.Message], error) {
		return closedStream(t), nil
	}})
	bound2, _ := fresh.WithTools([]*schema.ToolInfo{{Name: "tool_a"}})
	reader, err = bound2.Stream(observedCtx(obs2), nil)
	if err != nil {
		t.Fatalf("stream 2: %v", err)
	}
	for {
		if _, err := reader.Recv(); err != nil {
			break
		}
	}
	if len(obs2.metas) != 1 || obs2.metas[0].CallID == "" {
		t.Fatalf("second call meta = %+v", obs2.metas)
	}
}

// closedStream returns an already-EOF stream.
func closedStream(t *testing.T) *schema.StreamReader[*schema.Message] {
	t.Helper()
	r, w := schema.Pipe[*schema.Message](1)
	w.Close()
	return r
}

// TestObserverUpstreamReadErrorEndsCall: a mid-stream provider error ends
// the call with that error and propagates it downstream.
func TestObserverUpstreamReadErrorEndsCall(t *testing.T) {
	innerErr := errors.New("read reset")
	obs := &recordingObserver{}
	sr, sw := schema.Pipe[*schema.Message](1)
	go func() {
		sw.Send(schema.AssistantMessage("partial", nil), nil)
		sw.Send(nil, innerErr)
	}()
	out, err := observeChatModel(&fakeChatModel{streamReader: sr}).Stream(observedCtx(obs), nil)
	if err != nil {
		t.Fatalf("stream: %v", err)
	}
	if _, err := out.Recv(); err != nil {
		t.Fatalf("first recv: %v", err)
	}
	if _, err := out.Recv(); !errors.Is(err, innerErr) {
		t.Fatalf("second recv = %v, want %v", err, innerErr)
	}
	begins, _, _, ends := obs.counts()
	if begins != 1 || ends != 1 {
		t.Fatalf("lifecycle = %d begins / %d ends", begins, ends)
	}
	if !errors.Is(obs.results[0].Err, innerErr) || obs.results[0].ResponseComplete {
		t.Fatalf("end = %+v", obs.results[0])
	}
}

// TestObserverChunkErrorFailsDownstream: an observer Chunk failure fails
// the whole call — journal integrity outranks delivery.
func TestObserverChunkErrorFailsDownstream(t *testing.T) {
	obs := &recordingObserver{chunkErr: errors.New("journal full")}
	sr, sw := schema.Pipe[*schema.Message](1)
	go func() {
		defer sw.Close()
		sw.Send(schema.AssistantMessage("a", nil), nil)
	}()
	out, err := observeChatModel(&fakeChatModel{streamReader: sr}).Stream(observedCtx(obs), nil)
	if err != nil {
		t.Fatalf("stream: %v", err)
	}
	if _, err := out.Recv(); err == nil || err == io.EOF {
		t.Fatalf("recv = %v, want chunk error", err)
	}
	_, _, _, ends := obs.counts()
	if ends != 1 {
		t.Fatalf("ends = %d, want 1", ends)
	}
	if obs.results[0].Err == nil {
		t.Fatal("end result missing chunk error")
	}
}

// TestObserverDownstreamCloseEndsCallOnce: closing the reader mid-stream
// ends the call exactly once with a cancellation-shaped error.
func TestObserverDownstreamCloseEndsCallOnce(t *testing.T) {
	obs := &recordingObserver{}
	gate := make(chan struct{})
	sr, sw := schema.Pipe[*schema.Message](8)
	go func() {
		defer sw.Close()
		sw.Send(schema.AssistantMessage("first", nil), nil)
		<-gate
		for i := 0; i < 8; i++ {
			if sw.Send(schema.AssistantMessage("x", nil), nil) {
				return
			}
		}
	}()
	out, err := observeChatModel(&fakeChatModel{streamReader: sr}).Stream(observedCtx(obs), nil)
	if err != nil {
		t.Fatalf("stream: %v", err)
	}
	if _, err := out.Recv(); err != nil {
		t.Fatalf("recv: %v", err)
	}
	out.Close()
	close(gate) // upstream keeps producing; the pump's next Send sees the closed reader
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if _, _, _, ends := obs.counts(); ends == 1 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	begins, _, _, ends := obs.counts()
	if begins != 1 || ends != 1 {
		t.Fatalf("lifecycle = %d/%d, want 1/1", begins, ends)
	}
	if !errors.Is(obs.results[0].Err, context.Canceled) {
		t.Fatalf("end err = %v, want context.Canceled", obs.results[0].Err)
	}
}

// TestObserverBackpressureKeepsPumpAlive: a slow downstream does not stall
// the pipeline (frame queue drains independently).
func TestObserverBackpressureKeepsPumpAlive(t *testing.T) {
	obs := &recordingObserver{}
	sr, sw := schema.Pipe[*schema.Message](4)
	go func() {
		defer sw.Close()
		for i := 0; i < 32; i++ {
			if sw.Send(schema.AssistantMessage(fmt.Sprintf("%d", i), nil), nil) {
				return
			}
		}
	}()
	out, err := observeChatModel(&fakeChatModel{streamReader: sr}).Stream(observedCtx(obs), nil)
	if err != nil {
		t.Fatalf("stream: %v", err)
	}
	n := 0
	for {
		_, err := out.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("recv: %v", err)
		}
		n++
	}
	if n != 32 {
		t.Fatalf("received %d chunks, want 32", n)
	}
	_, _, chunks, ends := obs.counts()
	if chunks != 32 || ends != 1 {
		t.Fatalf("observed %d chunks / %d ends", chunks, ends)
	}
}

// TestObserverAbsentPassesThrough: without a factory in context the model
// is a pure passthrough — no lifecycle calls, no behavioral change.
func TestObserverAbsentPassesThrough(t *testing.T) {
	sr, sw := schema.Pipe[*schema.Message](2)
	go func() {
		defer sw.Close()
		sw.Send(schema.AssistantMessage("ok", nil), nil)
	}()
	out, err := observeChatModel(&fakeChatModel{streamReader: sr}).Stream(context.Background(), nil)
	if err != nil {
		t.Fatalf("stream: %v", err)
	}
	msg, err := out.Recv()
	if err != nil || msg.Content != "ok" {
		t.Fatalf("recv = %v / %v", msg, err)
	}
}

// TestObserverBeginFailureSkipsInnerCall: a failed Begin (e.g. budget or
// persist) prevents the provider call entirely — fail-closed.
func TestObserverBeginFailureSkipsInnerCall(t *testing.T) {
	inner := &fakeChatModel{streamReader: closedStream(t)}
	obs := &recordingObserver{beginErr: errors.New("budget exhausted")}
	_, err := observeChatModel(inner).Stream(observedCtx(obs), nil)
	if err == nil {
		t.Fatal("stream succeeded despite Begin failure")
	}
	if inner.streamCalls != 0 {
		t.Fatalf("inner stream invoked %d times after failed Begin", inner.streamCalls)
	}
}

// ---------------------------------------------------------------------
// usageAccumulator normalization (OBS-02): merged usage-only messages keep
// per-bucket monotonic max, dedupe by normalized key, and mark partial
// normalization when counters contradict themselves.

func TestUsageAccumulatorDedupAndNormalization(t *testing.T) {
	var acc usageAccumulator

	acc.record(&schema.TokenUsage{PromptTokens: 10, CompletionTokens: 4, TotalTokens: 14})
	first, ok := acc.sample()
	if !ok {
		t.Fatal("first sample missing")
	}
	// Identical merged state dedupes by normalized key.
	acc.record(&schema.TokenUsage{PromptTokens: 10, CompletionTokens: 4, TotalTokens: 14})
	if dup, _ := acc.sample(); dup.key() != first.key() {
		t.Fatalf("identical report changed the key: %q vs %q", dup.key(), first.key())
	}
	// Provider reports max instead of cumulative delta: merge keeps max.
	acc.record(&schema.TokenUsage{PromptTokens: 10, CompletionTokens: 8, TotalTokens: 18})
	sample, ok := acc.sample()
	if !ok {
		t.Fatal("no accumulated sample")
	}
	if sample.PromptTokens != 10 || sample.CompletionTokens != 8 || sample.TotalTokens != 18 {
		t.Fatalf("merged = %+v, want 10/8/18", sample)
	}
	if sample.Partial {
		t.Fatal("consistent merge flagged partial")
	}

	// Counter decrease marks partial normalization.
	var acc2 usageAccumulator
	acc2.record(&schema.TokenUsage{PromptTokens: 10, CompletionTokens: 4, TotalTokens: 14})
	acc2.record(&schema.TokenUsage{PromptTokens: 6, CompletionTokens: 4, TotalTokens: 10})
	sample, ok = acc2.sample()
	if !ok || !sample.Partial {
		t.Fatalf("decrease sample = %+v ok=%v, want partial", sample, ok)
	}
	// Merged max wins even when provider regressed.
	if sample.PromptTokens != 10 || sample.TotalTokens != 14 {
		t.Fatalf("merged = %+v, want max 10/?/14", sample)
	}

	// Total != Prompt+Completion marks partial and is normalized.
	var acc3 usageAccumulator
	acc3.record(&schema.TokenUsage{PromptTokens: 10, CompletionTokens: 4, TotalTokens: 99})
	sample, _ = acc3.sample()
	if sample.TotalTokens != 14 || !sample.Partial {
		t.Fatalf("mismatched total = %+v, want total 14 + partial", sample)
	}

	// Pinned Claude fixture (architecture D2): MessageStart input=10 then
	// an output-only cumulative delta=20 normalizes to total=30 — not 20
	// (raw-last replacement) and not a sum of all samples. The prompt
	// counter's drop to zero marks the sample partial: the wire cannot
	// distinguish "omitted" from "reported zero", so the evidence stays
	// provisional instead of silently complete.
	var acc4 usageAccumulator
	acc4.record(&schema.TokenUsage{PromptTokens: 10, TotalTokens: 10})
	acc4.record(&schema.TokenUsage{CompletionTokens: 20, TotalTokens: 20})
	sample, _ = acc4.sample()
	if sample.PromptTokens != 10 || sample.CompletionTokens != 20 || sample.TotalTokens != 30 {
		t.Fatalf("claude fixture = %+v, want 10/20/30", sample)
	}
	if !sample.Partial {
		t.Fatal("claude fixture must mark partial (raw prompt counter decreased)")
	}
}

// TestUsageAccumulatorOptionalBreakdown: zero reasoning/cached scalars
// stay unknown (nil), positive values surface as known optionals.
func TestUsageAccumulatorOptionalBreakdown(t *testing.T) {
	var acc usageAccumulator
	acc.record(&schema.TokenUsage{PromptTokens: 5, CompletionTokens: 1, TotalTokens: 6})
	sample, _ := acc.sample()
	if sample.ReasoningTokens != nil || sample.CachedTokens != nil {
		t.Fatalf("zero breakdown surfaced: %+v", sample)
	}
	var acc2 usageAccumulator
	acc2.record(&schema.TokenUsage{
		PromptTokens: 5, CompletionTokens: 1, TotalTokens: 6,
		PromptTokenDetails:      schema.PromptTokenDetails{CachedTokens: 2},
		CompletionTokensDetails: schema.CompletionTokensDetails{ReasoningTokens: 3},
	})
	sample, _ = acc2.sample()
	if sample.ReasoningTokens == nil || *sample.ReasoningTokens != 3 {
		t.Fatalf("reasoning = %+v", sample.ReasoningTokens)
	}
	if sample.CachedTokens == nil || *sample.CachedTokens != 2 {
		t.Fatalf("cached = %+v", sample.CachedTokens)
	}
	// No usage at all -> no synthetic sample.
	var acc3 usageAccumulator
	if _, ok := acc3.sample(); ok {
		t.Fatal("empty accumulator produced a sample")
	}
}

// ---------------------------------------------------------------------
// Service-level observed-call tests (OBS-02).

// scriptedUsageModel is an eino ToolCallingChatModel test double that
// streams a fixed chunk script per call (chunks may carry ResponseMeta
// usage). The domain adapter cannot transport usage, so the service-level
// tests build the Engine directly.
type scriptedUsageModel struct {
	mu     sync.Mutex
	calls  int
	chunks [][]*schema.Message
}

var _ model.ToolCallingChatModel = (*scriptedUsageModel)(nil)

func usageText(text string, p, c int) *schema.Message {
	return &schema.Message{
		Role:    schema.Assistant,
		Content: text,
		ResponseMeta: &schema.ResponseMeta{Usage: &schema.TokenUsage{
			PromptTokens: p, CompletionTokens: c, TotalTokens: p + c,
		}},
	}
}

func (m *scriptedUsageModel) Generate(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	r, err := m.Stream(ctx, input, opts...)
	if err != nil {
		return nil, err
	}
	return schema.ConcatMessages(collectStream(r))
}

func (m *scriptedUsageModel) Stream(_ context.Context, _ []*schema.Message, _ ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	m.mu.Lock()
	i := m.calls
	m.calls++
	m.mu.Unlock()
	if i >= len(m.chunks) {
		return nil, fmt.Errorf("runtime: scripted usage model exhausted at call %d", i)
	}
	r, w := schema.Pipe[*schema.Message](8)
	go func() {
		defer w.Close()
		for _, c := range m.chunks[i] {
			if w.Send(c, nil) {
				return
			}
		}
	}()
	return r, nil
}

func (m *scriptedUsageModel) WithTools(_ []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	return m, nil
}

func collectStream(r *schema.StreamReader[*schema.Message]) []*schema.Message {
	var out []*schema.Message
	for {
		c, err := r.Recv()
		if err != nil {
			break
		}
		out = append(out, c)
	}
	return out
}

func newObservedService(t *testing.T, chat model.ToolCallingChatModel, budget BudgetPolicy) (*Service, *sqlite.Backend, *testSink) {
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
		Budget: budget,
	})
	return svc, backend, sink
}

// observedCallKinds classifies the journal events of one run for the
// observed-call assertions.
type observedCallKinds struct {
	requests   []domain.RunEvent
	samples    []domain.RunEvent
	settlement []domain.RunEvent
	finished   []domain.RunEvent
}

func classifyObservedCall(events []domain.RunEvent) observedCallKinds {
	var k observedCallKinds
	for _, ev := range events {
		switch ev.Type {
		case domain.EventModelRequest:
			if ev.PayloadVersion >= 3 {
				k.requests = append(k.requests, ev)
			}
		case domain.EventModelUsage:
			var probe struct {
				Settlement *bool `json:"settlement"`
			}
			_ = json.Unmarshal(ev.Payload, &probe)
			if probe.Settlement != nil && *probe.Settlement {
				k.settlement = append(k.settlement, ev)
			} else {
				k.samples = append(k.samples, ev)
			}
		case domain.EventModelCallFinished:
			k.finished = append(k.finished, ev)
		}
	}
	return k
}

// usageScript returns a single-call script with a duplicated usage report
// (the second identical report must dedupe away) and a final advance.
func usageScript() [][]*schema.Message {
	return [][]*schema.Message{{
		schema.AssistantMessage("hel", nil),
		usageText("", 5, 2),
		usageText("lo", 5, 2), // identical cumulative report -> deduped
		usageText(" world", 12, 5),
	}}
}

// TestObservedCallJournalsLifecycle: one streamed call produces exactly one
// v3 request, one deduped stream of v2 usage samples, one settlement
// sample, and one finish record — in that order.
func TestObservedCallJournalsLifecycle(t *testing.T) {
	svc, backend, _ := newObservedService(t, &scriptedUsageModel{chunks: usageScript()}, DefaultBudgetPolicy())
	mustCreateSession(t, backend, "sess-obs")

	ctx := context.Background()
	runID, err := svc.Run(ctx, "sess-obs", "hello")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	waitForRunStatus(t, backend, runID, domain.RunCompleted)

	k := classifyObservedCall(replayAll(t, backend, runID))
	if len(k.requests) != 1 {
		t.Fatalf("v3 requests = %d, want 1", len(k.requests))
	}
	var req payloadModelRequestV3
	mustUnmarshal(t, k.requests[0].Payload, &req)
	if req.CallID == "" || req.Provider != "test" || req.Model != "test-model" || req.Source != "main" || req.Mode == "" {
		t.Fatalf("request envelope = %+v", req)
	}
	if len(k.samples) != 2 {
		t.Fatalf("ordinary samples = %d, want 2 (duplicate report deduped)", len(k.samples))
	}
	var lastSample payloadModelUsageV2
	mustUnmarshal(t, k.samples[len(k.samples)-1].Payload, &lastSample)
	if lastSample.CallID != req.CallID || lastSample.TotalTokens != 17 || lastSample.UsageKind != "cumulative" {
		t.Fatalf("last sample = %+v", lastSample)
	}
	if len(k.settlement) > 1 {
		t.Fatalf("settlement samples = %d, want at most 1", len(k.settlement))
	}
	if len(k.settlement) == 1 {
		var settle payloadModelUsageV2
		mustUnmarshal(t, k.settlement[0].Payload, &settle)
		if settle.CallID != req.CallID || settle.Settlement == nil || !*settle.Settlement || settle.TotalTokens != 17 {
			t.Fatalf("settlement = %+v", settle)
		}
	}
	if len(k.finished) != 1 {
		t.Fatalf("finish records = %d, want 1", len(k.finished))
	}
	var fin payloadModelCallFinished
	mustUnmarshal(t, k.finished[0].Payload, &fin)
	if fin.CallID != req.CallID || fin.Status != "completed" || !fin.ResponseComplete || fin.Usage == nil || fin.Usage.TotalTokens != 17 {
		t.Fatalf("finish = %+v", fin)
	}

	// Ordering: request first, then samples, then settlement + finish, then
	// the mapped semantic events (model.completed).
	order := map[string]int{}
	for i, ev := range replayAll(t, backend, runID) {
		switch ev.Type {
		case domain.EventModelRequest:
			order["request"] = i
		case domain.EventModelCallFinished:
			order["finish"] = i
		case domain.EventModelCompleted:
			if _, ok := order["completed"]; !ok {
				order["completed"] = i
			}
		}
	}
	if !(order["request"] < order["finish"] && order["finish"] < order["completed"]) {
		t.Fatalf("ordering = %v, want request < finish < completed", order)
	}
}

// TestObservedCallSettlementAtMaxEvents: the settlement sample and the
// finish record are mandatory closure evidence. Sweep the event quota from
// zero up to the fully-charged count; at every limit where the run admitted
// a model call, the journal must still carry at most the request, the
// ordinary samples that fit, one changed settlement sample and one finish —
// the two exempt closure records never trip the breaker themselves.
func TestObservedCallSettlementAtMaxEvents(t *testing.T) {
	ctx := context.Background()

	// Baseline: charged-event count for this exact script at a generous
	// quota. Every non-terminal, non-delta, non-exempt journal row counts.
	svcA, backendA, _ := newObservedService(t, &scriptedUsageModel{chunks: usageScript()}, DefaultBudgetPolicy())
	mustCreateSession(t, backendA, "sess-a")
	runA, err := svcA.Run(ctx, "sess-a", "hello")
	if err != nil {
		t.Fatalf("run A: %v", err)
	}
	waitForRunStatus(t, backendA, runA, domain.RunCompleted)
	eventsA := replayAll(t, backendA, runA)
	kA := classifyObservedCall(eventsA)
	if len(kA.requests) != 1 || len(kA.finished) != 1 {
		t.Fatalf("baseline journal = %d requests / %d finish, want 1/1", len(kA.requests), len(kA.finished))
	}
	charged := 0
	for _, ev := range eventsA {
		if ev.Type.Terminal() {
			continue
		}
		switch ev.Type {
		case domain.EventModelDelta, domain.EventModelReasoningDelta, domain.EventModelCallFinished:
			continue
		case domain.EventModelUsage:
			var probe struct {
				Settlement *bool `json:"settlement"`
			}
			_ = json.Unmarshal(ev.Payload, &probe)
			if probe.Settlement != nil && *probe.Settlement {
				continue
			}
		}
		charged++
	}

	sawExemptClosure := false
	for limit := 0; limit <= charged; limit++ {
		policy := DefaultBudgetPolicy()
		policy.MaxEvents = limit
		svc, backend, _ := newObservedService(t, &scriptedUsageModel{chunks: usageScript()}, policy)
		mustCreateSession(t, backend, "sess-quota")
		runID, err := svc.Run(ctx, "sess-quota", "hello")
		if err != nil {
			continue // admission itself refused — no call was journaled
		}
		deadline := time.Now().Add(30 * time.Second)
		var run domain.Run
		for time.Now().Before(deadline) {
			run, err = backend.GetRun(ctx, runID)
			if err == nil && (run.Status == domain.RunCompleted || run.Status == domain.RunFailed || run.Status == domain.RunCancelled) {
				break
			}
			time.Sleep(10 * time.Millisecond)
		}
		if run.Status != domain.RunCompleted && run.Status != domain.RunFailed && run.Status != domain.RunCancelled {
			t.Fatalf("limit %d: run never settled (status %s)", limit, run.Status)
		}
		k := classifyObservedCall(replayAll(t, backend, runID))
		if len(k.requests) == 0 {
			continue // the call never began — nothing to exempt
		}
		// The mandatory finish is always present once a call was admitted.
		if len(k.finished) != 1 {
			t.Fatalf("limit %d: finish records = %d, want 1", limit, len(k.finished))
		}
		if len(k.settlement) > 1 {
			t.Fatalf("limit %d: settlement samples = %d, want at most 1", limit, len(k.settlement))
		}
		if len(k.settlement) == 1 {
			sawExemptClosure = true
		}
	}
	if !sawExemptClosure {
		t.Fatal("no quota level exercised the settlement emission path")
	}
}

// TestObservedCallBudgetReplayParity: replaying the journal into a fresh
// ledger reproduces the live accounting — one model call per v3 request
// (the mapped model.completed consumes the Begin credit instead of
// charging again).
func TestObservedCallBudgetReplayParity(t *testing.T) {
	ctx := context.Background()
	svc, backend, _ := newObservedService(t, &scriptedUsageModel{chunks: usageScript()}, DefaultBudgetPolicy())
	mustCreateSession(t, backend, "sess-par")
	runID, err := svc.Run(ctx, "sess-par", "hello")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	waitForRunStatus(t, backend, runID, domain.RunCompleted)
	events := replayAll(t, backend, runID)

	ledger, err := NewBudgetLedger(DefaultBudgetPolicy())
	if err != nil {
		t.Fatal(err)
	}
	for _, ev := range events {
		if err := ledger.ReplayEvent(ev); err != nil {
			t.Fatalf("replay %s: %v", ev.Type, err)
		}
	}
	usage := ledger.Snapshot().Usage

	// Independent spec-side recount of the journal.
	var wantEvents, wantCalls int
	for _, ev := range events {
		if ev.Type == domain.EventModelRequest && ev.PayloadVersion >= 3 {
			wantCalls++
		}
		if ev.Type.Terminal() {
			continue
		}
		switch ev.Type {
		case domain.EventModelDelta, domain.EventModelReasoningDelta, domain.EventModelCallFinished:
			continue
		case domain.EventModelUsage:
			var probe struct {
				Settlement *bool `json:"settlement"`
			}
			_ = json.Unmarshal(ev.Payload, &probe)
			if probe.Settlement != nil && *probe.Settlement {
				continue
			}
		}
		wantEvents++
	}
	if usage.ModelCalls != wantCalls {
		t.Fatalf("replayed model calls = %d, want %d (one per v3 request; completed must not double-charge)", usage.ModelCalls, wantCalls)
	}
	if usage.Events != wantEvents {
		t.Fatalf("replayed events = %d, want %d", usage.Events, wantEvents)
	}
	if usage.ModelCalls != 1 {
		t.Fatalf("expected exactly 1 model call for this script, got %d", usage.ModelCalls)
	}
}

// TestObservedCallFailedStreamJournalsFinish: a provider-side stream error
// still produces request + finish(failed) evidence.
func TestObservedCallFailedStreamJournalsFinish(t *testing.T) {
	failModel := &fakeChatModel{streamErr: errors.New("provider blew up")}
	svc, backend, _ := newObservedService(t, failModel, DefaultBudgetPolicy())
	mustCreateSession(t, backend, "sess-fail")
	runID, err := svc.Run(context.Background(), "sess-fail", "hello")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	waitForRunStatus(t, backend, runID, domain.RunFailed)

	k := classifyObservedCall(replayAll(t, backend, runID))
	if len(k.requests) != 1 || len(k.finished) != 1 {
		t.Fatalf("failed call journal = %d requests / %d finish, want 1/1", len(k.requests), len(k.finished))
	}
	var fin payloadModelCallFinished
	mustUnmarshal(t, k.finished[0].Payload, &fin)
	if fin.Status != "failed" || fin.Error == nil || fin.Error.Message == "" || fin.ResponseComplete {
		t.Fatalf("finish = %+v, want failed with error record", fin)
	}
}

// fakeChatModel is a minimal ToolCallingChatModel: scripted stream reader
// (or a factory for one), a generate message, fixed error otherwise.
type fakeChatModel struct {
	streamReader  *schema.StreamReader[*schema.Message]
	streamFactory func() (*schema.StreamReader[*schema.Message], error)
	streamErr     error
	generateMsg   *schema.Message
	generateErr   error
	streamCalls   int
}

var _ model.ToolCallingChatModel = (*fakeChatModel)(nil)

func (m *fakeChatModel) Generate(_ context.Context, _ []*schema.Message, _ ...model.Option) (*schema.Message, error) {
	return m.generateMsg, m.generateErr
}

func (m *fakeChatModel) Stream(_ context.Context, _ []*schema.Message, _ ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	m.streamCalls++
	if m.streamFactory != nil {
		return m.streamFactory()
	}
	return m.streamReader, m.streamErr
}

func (m *fakeChatModel) WithTools(_ []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	return m, nil
}

// TestObservedCallSourceAttribution: the run's factory resolves source and
// model per route — main/child come from the execution context, summary
// calls report "summary" with the summary model (or the run model on
// failover), and Begin tags the meta identically.
func TestObservedCallSourceAttribution(t *testing.T) {
	svc, backend, _ := newObservedService(t, &scriptedUsageModel{chunks: [][]*schema.Message{{schema.AssistantMessage("ok", nil)}}}, DefaultBudgetPolicy())
	mustCreateSession(t, backend, "sess-src")
	ctx := context.Background()
	runID, err := svc.Run(ctx, "sess-src", "hi")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	waitForRunStatus(t, backend, runID, domain.RunCompleted)

	m := newEventMapper(runID, 64<<10)
	m.setUsageRoutes("test", "run-model", "summary-model")
	ledger, err := NewBudgetLedger(DefaultBudgetPolicy())
	if err != nil {
		t.Fatal(err)
	}

	factory := modelCallObserverFactoryFrom(svc.withLiveModelStreamObserver(ctx, m, "sess-src", ledger, nil, false, nil))
	if factory == nil {
		t.Fatal("factory missing")
	}
	main := factory(modelCallRoute{}).(*runModelCallObserver)
	if main.source != "main" {
		t.Fatalf("chat source = %q, want main", main.source)
	}
	sum := factory(modelCallRoute{Source: modelCallSourceSummary}).(*runModelCallObserver)
	if sum.source != "summary" || sum.model != "summary-model" {
		t.Fatalf("summary route = %q/%q", sum.source, sum.model)
	}
	fallback := factory(modelCallRoute{Source: modelCallSourceSummary, FallbackModel: true}).(*runModelCallObserver)
	if fallback.model != "run-model" {
		t.Fatalf("summary fallback model = %q, want run-model", fallback.model)
	}
	// Begin on a run that already carries its terminal event is refused by
	// the journal — fail-closed proof that no call can be journaled after
	// run closure. The meta still shows the resolved route attribution.
	meta, beginErr := sum.Begin(ctx, modelCallInput{Mode: "generate", Messages: []*schema.Message{schema.UserMessage("sum")}})
	if beginErr == nil {
		t.Fatal("Begin on a terminal run must fail closed")
	}
	if meta.Source != "summary" || meta.Model != "summary-model" || meta.ContextViewID != "" {
		t.Fatalf("summary meta = %+v", meta)
	}

	childFactory := modelCallObserverFactoryFrom(svc.withLiveModelStreamObserver(ctx, m, "sess-src", ledger, nil, true, nil))
	child := childFactory(modelCallRoute{}).(*runModelCallObserver)
	if child.source != "child" {
		t.Fatalf("child source = %q, want child", child.source)
	}
}

// TestObservedCallCancelledStatus: caller cancellation lands as a
// "cancelled" finish, not a generic failure.
func TestObservedCallCancelledStatus(t *testing.T) {
	if got := modelCallStatus(context.Background(), nil); got != "completed" {
		t.Fatalf("status = %q", got)
	}
	if got := modelCallStatus(context.Background(), errors.New("x")); got != "failed" {
		t.Fatalf("status = %q", got)
	}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if got := modelCallStatus(cancelled, context.Canceled); got != "cancelled" {
		t.Fatalf("status = %q, want cancelled", got)
	}
}
