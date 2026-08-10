package runtime

import (
	"context"
	"io"
	"strings"
	"testing"

	"agent-vivy/internal/provider"
	"agent-vivy/internal/tools"
)

func newTestEngine(t *testing.T) *Engine {
	t.Helper()
	ctx := context.Background()
	ts, err := tools.Builtin(nil).Resolve([]string{tools.EchoInfoName})
	if err != nil {
		t.Fatalf("resolve tools: %v", err)
	}
	eng, err := NewEngine(ctx, WrapModel(provider.NewMock()), ts, EngineConfig{
		StreamBuffer:         8,
		MaxEventPayloadBytes: 64 << 10,
	})
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}
	return eng
}

// drainReassembled iterates the raw engine event stream and returns the
// concatenated assistant stream chunks. It fails the test on any event
// error or stream recv error.
func drainReassembled(t *testing.T, eng *Engine, query string) string {
	t.Helper()
	iter := eng.Query(context.Background(), query)
	var (
		chunks       []string
		sawStreaming bool
	)
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
		if !mv.IsStreaming || mv.MessageStream == nil {
			continue
		}
		sawStreaming = true
		for {
			chunk, err := mv.MessageStream.Recv()
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Fatalf("stream recv: %v", err)
			}
			chunks = append(chunks, chunk.Content)
		}
	}
	if !sawStreaming {
		t.Fatal("expected at least one streaming message event")
	}
	return strings.Join(chunks, "")
}

// TestEngineQueryStreamsReply drives a real Eino Runner over the mock
// provider and asserts the reassembled stream equals the deterministic
// mock reply byte-for-byte.
func TestEngineQueryStreamsReply(t *testing.T) {
	eng := newTestEngine(t)
	got := drainReassembled(t, eng, "hello vivy")
	want := "mock reply to: hello vivy"
	if got != want {
		t.Fatalf("reassembled reply = %q, want %q", got, want)
	}
}

// TestEngineQueryIsDeterministic proves two runs with the same input
// produce the same byte stream (FR-3).
func TestEngineQueryIsDeterministic(t *testing.T) {
	eng := newTestEngine(t)
	first := drainReassembled(t, eng, "repeat after me")
	second := drainReassembled(t, eng, "repeat after me")
	if first != second {
		t.Fatalf("runs diverged: %q vs %q", first, second)
	}
}

func TestToolAdapterInfoAndRun(t *testing.T) {
	ctx := context.Background()
	ad := newToolAdapter(tools.NewEchoInfo(), 0)

	info, err := ad.Info(ctx)
	if err != nil {
		t.Fatalf("info: %v", err)
	}
	if info.Name != tools.EchoInfoName {
		t.Fatalf("tool name = %q, want %q", info.Name, tools.EchoInfoName)
	}
	if info.Desc == "" {
		t.Fatal("tool description must not be empty")
	}

	// The argument schema must reach the model: without it real gateways
	// hallucinate parameter names (AS-2 walkthrough).
	if info.ParamsOneOf == nil {
		t.Fatal("tool info must carry a parameter schema")
	}
	js, err := info.ParamsOneOf.ToJSONSchema()
	if err != nil || js == nil {
		t.Fatalf("parameter schema conversion: %v", err)
	}
	textSchema, ok := js.Properties.Get("text")
	if !ok || textSchema.Type != "string" {
		t.Fatalf("parameter schema for text = %+v, want required string", textSchema)
	}
	required := false
	for _, name := range js.Required {
		if name == "text" {
			required = true
		}
	}
	if !required {
		t.Fatal("text parameter must be marked required")
	}

	// Effectful tools publish their schema too.
	wnInfo, err := newToolAdapter(tools.NewWriteNote(nil), 0).Info(ctx)
	if err != nil {
		t.Fatalf("write_note info: %v", err)
	}
	wnSchema, err := wnInfo.ParamsOneOf.ToJSONSchema()
	if err != nil {
		t.Fatalf("write_note schema conversion: %v", err)
	}
	if _, ok := wnSchema.Properties.Get("content"); !ok {
		t.Fatal("write_note schema must declare the content parameter")
	}

	out, err := ad.InvokableRun(ctx, `{"text":"hi there"}`)
	if err != nil {
		t.Fatalf("invokable run: %v", err)
	}
	if out != "hi there" {
		t.Fatalf("tool output = %q, want %q", out, "hi there")
	}

	// Malformed args must surface as an error, never a panic.
	if _, err := ad.InvokableRun(ctx, `{}`); err == nil {
		t.Fatal("expected argument error for empty text")
	}
	if _, err := ad.InvokableRun(ctx, `{"text":"x","extra":1}`); err == nil {
		t.Fatal("expected argument error for unknown field")
	}
}

func TestNewEngineRejectsNilModel(t *testing.T) {
	if _, err := NewEngine(context.Background(), nil, nil, EngineConfig{}); err == nil {
		t.Fatal("expected error for nil model")
	}
}
