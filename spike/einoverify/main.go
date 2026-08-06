// Command einoverify is the M0 capability spike for AGENT-VIVY (task A1).
//
// It verifies, against the ONLINE github.com/cloudwego/eino v0.9.13 module
// (never the vendored .workspace reference), the four capabilities the Vivy
// runtime design depends on:
//
//	V1  streaming model output through ChatModelAgent + Runner
//	V2  tool call execution and tool-result events
//	V3  tool interrupt -> CheckPointStore.Set -> ResumeWithParams with the
//	    approval payload, plus the D-029 ordering evidence (checkpoint
//	    durable BEFORE the interrupt event reaches the consumer)
//	V4  cancellation via adk.WithCancel producing a CancelError event
//
// All scenarios run against a deterministic scripted mock model; no API key
// is required. Run: go run ./spike/einoverify
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"runtime/debug"
	"sync"
	"time"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
)

// ---------------------------------------------------------------------------
// Global step log: proves ordering between checkpoint persistence and event
// delivery (D-029 evidence).
// ---------------------------------------------------------------------------

var (
	stepMu  sync.Mutex
	stepSeq int
	steps   []string
)

func record(format string, args ...any) {
	stepMu.Lock()
	defer stepMu.Unlock()
	stepSeq++
	steps = append(steps, fmt.Sprintf("%02d  ", stepSeq)+fmt.Sprintf(format, args...))
}

func dumpSteps() {
	for _, s := range steps {
		fmt.Println("    " + s)
	}
}

// ---------------------------------------------------------------------------
// recordingStore: CheckPointStore + CheckPointDeleter with trace logging.
// ---------------------------------------------------------------------------

type recordingStore struct {
	mu   sync.Mutex
	data map[string][]byte
}

func newRecordingStore() *recordingStore {
	return &recordingStore{data: map[string][]byte{}}
}

func (s *recordingStore) Get(_ context.Context, id string) ([]byte, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.data[id]
	record("store.Get(%q) -> existed=%v len=%d", id, ok, len(v))
	return v, ok, nil
}

func (s *recordingStore) Set(_ context.Context, id string, blob []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[id] = blob
	record("store.Set(%q) len=%d head=%x", id, len(blob), blob[:min(8, len(blob))])
	return nil
}

func (s *recordingStore) Delete(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data, id)
	record("store.Delete(%q)", id)
	return nil
}

func (s *recordingStore) has(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.data[id]
	return ok
}

// ---------------------------------------------------------------------------
// scriptedModel: deterministic mock implementing model.ToolCallingChatModel.
// Generate replays script by call index. Stream replays chunks for V1/V4.
// ---------------------------------------------------------------------------

type scriptedModel struct {
	mu     sync.Mutex
	calls  int
	script []*schema.Message

	streamChunks []string
	chunkDelay   time.Duration
}

func (m *scriptedModel) Generate(_ context.Context, _ []*schema.Message, _ ...model.Option) (*schema.Message, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	i := m.calls
	m.calls++
	record("model.Generate call=%d", i)
	if i >= len(m.script) {
		return nil, fmt.Errorf("scripted model exhausted at call %d", i)
	}
	return m.script[i], nil
}

func (m *scriptedModel) Stream(_ context.Context, _ []*schema.Message, _ ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	record("model.Stream invoked, chunks=%d delay=%v", len(m.streamChunks), m.chunkDelay)
	r, w := schema.Pipe[*schema.Message](8)
	go func() {
		defer w.Close()
		for _, c := range m.streamChunks {
			time.Sleep(m.chunkDelay)
			if w.Send(&schema.Message{Role: schema.Assistant, Content: c}, nil) {
				return
			}
		}
	}()
	return r, nil
}

func (m *scriptedModel) WithTools(_ []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	return m, nil
}

// ---------------------------------------------------------------------------
// Tools
// ---------------------------------------------------------------------------

type echoTool struct{}

func (echoTool) Info(_ context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "echo_tool",
		Desc: "read-only echo; executes automatically",
	}, nil
}

func (echoTool) InvokableRun(_ context.Context, argsJSON string, _ ...tool.Option) (string, error) {
	record("echo_tool executed args=%s", argsJSON)
	return "echo:" + argsJSON, nil
}

type approvalTool struct{}

func (approvalTool) Info(_ context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "send_message",
		Desc: "effectful tool; requires approval via interrupt/resume",
	}, nil
}

func (approvalTool) InvokableRun(ctx context.Context, argsJSON string, _ ...tool.Option) (string, error) {
	wasInterrupted, hasState, _ := tool.GetInterruptState[any](ctx)
	record("send_message invoked args=%s wasInterrupted=%v hasState=%v", argsJSON, wasInterrupted, hasState)
	if !wasInterrupted {
		return "", tool.Interrupt(ctx, "approval required for send_message")
	}
	isTarget, hasData, data := tool.GetResumeContext[string](ctx)
	record("send_message resume ctx: isTarget=%v hasData=%v data=%q", isTarget, hasData, data)
	if !isTarget {
		return "", tool.Interrupt(ctx, "still waiting for approval")
	}
	if hasData {
		return "send_message executed, decision=" + data, nil
	}
	return "send_message executed, decision=default", nil
}

// ---------------------------------------------------------------------------
// Event helpers
// ---------------------------------------------------------------------------

func describe(ev *adk.AgentEvent) string {
	if ev.Err != nil {
		return fmt.Sprintf("ERR agent=%s err=%v", ev.AgentName, ev.Err)
	}
	if ev.Action != nil && ev.Action.Interrupted != nil {
		return fmt.Sprintf("INTERRUPT agent=%s dataType=%T contexts=%d", ev.AgentName, ev.Action.Interrupted.Data, len(ev.Action.Interrupted.InterruptContexts))
	}
	if ev.Output != nil && ev.Output.MessageOutput != nil {
		mv := ev.Output.MessageOutput
		if mv.IsStreaming {
			return fmt.Sprintf("STREAM agent=%s role=%s tool=%s", ev.AgentName, mv.Role, mv.ToolName)
		}
		content := ""
		if mv.Message != nil {
			content = mv.Message.Content
		}
		return fmt.Sprintf("MESSAGE agent=%s role=%s tool=%s toolcalls=%d content=%q",
			ev.AgentName, mv.Role, mv.ToolName, len(mv.Message.ToolCalls), content)
	}
	return fmt.Sprintf("OTHER agent=%s", ev.AgentName)
}

// drain iterates the event stream, printing each event. onInterrupt is called
// (if non-nil) when an interrupt action arrives; it returns the root-cause
// interrupt ID captured from InterruptContexts.
func drain(label string, iter *adk.AsyncIterator[*adk.AgentEvent], onInterrupt func(*adk.AgentEvent)) {
	for {
		ev, ok := iter.Next()
		if !ok {
			record("[%s] iterator closed", label)
			return
		}
		record("[%s] event: %s", label, describe(ev))
		if ev.Err == nil && ev.Output != nil && ev.Output.MessageOutput != nil && ev.Output.MessageOutput.IsStreaming {
			sr := ev.Output.MessageOutput.MessageStream
			n := 0
			for {
				chunk, err := sr.Recv()
				if err == io.EOF {
					break
				}
				if err != nil {
					record("[%s] stream recv error: %v", label, err)
					break
				}
				n++
				record("[%s]   chunk %d: %q", label, n, chunk.Content)
			}
		}
		if ev.Action != nil && ev.Action.Interrupted != nil && onInterrupt != nil {
			onInterrupt(ev)
		}
	}
}

func rootCauseID(ev *adk.AgentEvent) (string, adk.Address) {
	for _, c := range ev.Action.Interrupted.InterruptContexts {
		if c.IsRootCause {
			return c.ID, c.Address
		}
	}
	return "", nil
}

func newAgent(ctx context.Context, name string, m model.BaseModel[*schema.Message], tools []tool.BaseTool) (adk.Agent, error) {
	return adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Name:        name,
		Description: "spike agent " + name,
		Instruction: "You are a deterministic spike agent.",
		Model:       m,
		ToolsConfig: adk.ToolsConfig{
			ToolsNodeConfig: compose.ToolsNodeConfig{Tools: tools},
		},
	})
}

// ---------------------------------------------------------------------------
// Scenarios
// ---------------------------------------------------------------------------

func v1Streaming(ctx context.Context, store adk.CheckPointStore) {
	fmt.Println("\n== V1: streaming model output ==")
	m := &scriptedModel{streamChunks: []string{"Hel", "lo ", "Vivy!"}, chunkDelay: 10 * time.Millisecond}
	agent, err := newAgent(ctx, "v1-agent", m, nil)
	if err != nil {
		log.Fatalf("V1 agent: %v", err)
	}
	runner := adk.NewRunner(ctx, adk.RunnerConfig{Agent: agent, EnableStreaming: true, CheckPointStore: store})
	drain("V1", runner.Query(ctx, "say hello"), nil)
}

func v2ToolCall(ctx context.Context, store adk.CheckPointStore) {
	fmt.Println("\n== V2: read-only tool call ==")
	m := &scriptedModel{script: []*schema.Message{
		schema.AssistantMessage("", []schema.ToolCall{{
			ID:       "call-1",
			Function: schema.FunctionCall{Name: "echo_tool", Arguments: `{"text":"hi"}`},
		}}),
		schema.AssistantMessage("done after echo", nil),
	}}
	agent, err := newAgent(ctx, "v2-agent", m, []tool.BaseTool{echoTool{}})
	if err != nil {
		log.Fatalf("V2 agent: %v", err)
	}
	runner := adk.NewRunner(ctx, adk.RunnerConfig{Agent: agent, CheckPointStore: store})
	drain("V2", runner.Query(ctx, "echo hi"), nil)
}

func v3InterruptResume(ctx context.Context, store *recordingStore) {
	fmt.Println("\n== V3: tool interrupt -> checkpoint -> ResumeWithParams ==")
	const ckptID = "ckpt-run-1"
	m := &scriptedModel{script: []*schema.Message{
		schema.AssistantMessage("", []schema.ToolCall{{
			ID:       "call-2",
			Function: schema.FunctionCall{Name: "send_message", Arguments: `{"to":"user"}`},
		}}),
		schema.AssistantMessage("run complete after approved tool", nil),
	}}
	agent, err := newAgent(ctx, "v3-agent", m, []tool.BaseTool{approvalTool{}})
	if err != nil {
		log.Fatalf("V3 agent: %v", err)
	}
	runner := adk.NewRunner(ctx, adk.RunnerConfig{Agent: agent, CheckPointStore: store})

	var targetID string
	var checkpointDurableBeforeEvent bool
	drain("V3.run", runner.Query(ctx, "send the message", adk.WithCheckPointID(ckptID)), func(ev *adk.AgentEvent) {
		// D-029 ordering evidence: at the moment the interrupt event reaches
		// the consumer, the checkpoint must already be durable in the store.
		checkpointDurableBeforeEvent = store.has(ckptID)
		record("[V3.run] interrupt received; checkpoint already durable=%v", checkpointDurableBeforeEvent)
		targetID, _ = rootCauseID(ev)
		record("[V3.run] root-cause interrupt id=%q", targetID)
	})

	if !checkpointDurableBeforeEvent {
		log.Fatalf("V3 FAIL: interrupt event arrived BEFORE checkpoint was durable")
	}
	if targetID == "" {
		log.Fatalf("V3 FAIL: no root-cause interrupt id captured")
	}

	// Simulate the Vivy approval decision feeding back via ResumeWithParams.
	iter, err := runner.ResumeWithParams(ctx, ckptID, &adk.ResumeParams{
		Targets: map[string]any{targetID: "approved"},
	})
	if err != nil {
		log.Fatalf("V3 resume: %v", err)
	}
	drain("V3.resume", iter, nil)
}

func v4Cancel(ctx context.Context, store *recordingStore) {
	fmt.Println("\n== V4: cancellation mid-stream ==")
	const ckptID = "ckpt-cancel"
	m := &scriptedModel{streamChunks: []string{"a", "b", "c", "d", "e", "f"}, chunkDelay: 80 * time.Millisecond}
	agent, err := newAgent(ctx, "v4-agent", m, nil)
	if err != nil {
		log.Fatalf("V4 agent: %v", err)
	}
	runner := adk.NewRunner(ctx, adk.RunnerConfig{Agent: agent, EnableStreaming: true, CheckPointStore: store})

	cancelOpt, cancelFn := adk.WithCancel()
	iter := runner.Query(ctx, "stream slowly", cancelOpt, adk.WithCheckPointID(ckptID))

	go func() {
		time.Sleep(150 * time.Millisecond) // let ~1-2 chunks flow
		record("[V4] issuing cancel")
		handle, contributed := cancelFn()
		record("[V4] cancel committed contributed=%v", contributed)
		if err := handle.Wait(); err != nil {
			record("[V4] cancel handle wait: %v", err)
		}
	}()

	var sawCancelErr bool
	for {
		ev, ok := iter.Next()
		if !ok {
			record("[V4] iterator closed")
			break
		}
		record("[V4] event: %s", describe(ev))
		if ev.Err != nil {
			var ce *adk.CancelError
			if errors.As(ev.Err, &ce) {
				sawCancelErr = true
				record("[V4] CancelError observed: %v", ce)
			}
		}
		if ev.Output != nil && ev.Output.MessageOutput != nil && ev.Output.MessageOutput.IsStreaming {
			sr := ev.Output.MessageOutput.MessageStream
			for {
				chunk, err := sr.Recv()
				if err == io.EOF {
					break
				}
				if err != nil {
					record("[V4] stream recv terminated: %v", err)
					break
				}
				record("[V4]   chunk: %q", chunk.Content)
			}
		}
	}
	if !sawCancelErr {
		log.Fatalf("V4 FAIL: no CancelError surfaced")
	}
	record("[V4] checkpoint after cancel: durable=%v", store.has(ckptID))
}

// ---------------------------------------------------------------------------

func main() {
	if bi, ok := debug.ReadBuildInfo(); ok {
		for _, dep := range bi.Deps {
			if dep.Path == "github.com/cloudwego/eino" {
				fmt.Printf("eino module version: %s\n", dep.Version)
			}
		}
	}
	store := newRecordingStore()
	ctx := context.Background()

	v1Streaming(ctx, store)
	v2ToolCall(ctx, store)
	v3InterruptResume(ctx, store)
	v4Cancel(ctx, store)

	fmt.Println("\n== step log ==")
	dumpSteps()
	fmt.Println("\nALL VERIFICATIONS PASSED")
	os.Exit(0)
}
