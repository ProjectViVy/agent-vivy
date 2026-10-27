package runtime

// Native Eino/Service fixtures with an actual SQLite owner and controllable Journal.

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/model"
	einotool "github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
	"agent-vivy/internal/storage/sqlite"
	"agent-vivy/internal/tools"
)

type contractJournal struct {
	inner storage.Journal

	mu      sync.Mutex
	events  []domain.RunEvent
	pause   map[string]chan struct{} // "<type>:<callID>" → the append waits on the channel
	failed  map[string]error         // same key → the append returns this error without writing
	pausing int
}

func (j *contractJournal) gateKey(ev domain.RunEvent) string {
	id := j.toolCallID(ev)
	if id == "" {
		return ""
	}
	return string(ev.Type) + ":" + id
}

func (j *contractJournal) toolCallID(ev domain.RunEvent) string {
	var p struct {
		ToolCallID string `json:"tool_call_id"`
	}
	if err := json.Unmarshal(ev.Payload, &p); err != nil {
		return ""
	}
	return p.ToolCallID
}

func (j *contractJournal) Append(ctx context.Context, commit storage.Commit) (domain.EventSeq, error) {
	for _, ev := range commit.Events {
		key := j.gateKey(ev)
		if key == "" {
			continue
		}
		j.mu.Lock()
		ferr, hasFail := j.failed[key]
		gate, hasPause := j.pause[key]
		j.mu.Unlock()
		if hasFail {
			return 0, ferr
		}
		if hasPause {
			j.mu.Lock()
			j.pausing++
			j.mu.Unlock()
			select {
			case <-gate:
			case <-ctx.Done():
				j.mu.Lock()
				j.pausing--
				j.mu.Unlock()
				return 0, ctx.Err()
			}
			j.mu.Lock()
			j.pausing--
			j.mu.Unlock()
		}
	}
	seq, err := j.inner.Append(ctx, commit)
	if err != nil {
		return 0, err
	}
	j.mu.Lock()
	j.events = append(j.events, commit.Events...)
	j.mu.Unlock()
	return seq, nil
}

func (j *contractJournal) Replay(ctx context.Context, runID domain.RunID, after domain.EventSeq) (storage.Iterator[storage.Entry], error) {
	return j.inner.Replay(ctx, runID, after)
}

func (j *contractJournal) hasEvent(typ domain.EventType, callID string) bool {
	j.mu.Lock()
	defer j.mu.Unlock()
	for _, ev := range j.events {
		if ev.Type == typ && (callID == "" || j.toolCallID(ev) == callID) {
			return true
		}
	}
	return false
}

func (j *contractJournal) indexOf(typ domain.EventType, callID string) int {
	j.mu.Lock()
	defer j.mu.Unlock()
	for i, ev := range j.events {
		if ev.Type == typ && (callID == "" || j.toolCallID(ev) == callID) {
			return i
		}
	}
	return -1
}

func (j *contractJournal) pausingCount() int {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.pausing
}

func (j *contractJournal) finishedPayload(t *testing.T, callID string) payloadToolFinished {
	t.Helper()
	j.mu.Lock()
	defer j.mu.Unlock()
	for _, ev := range j.events {
		if ev.Type != domain.EventToolFinished || j.toolCallID(ev) != callID {
			continue
		}
		var p payloadToolFinished
		if err := json.Unmarshal(ev.Payload, &p); err != nil {
			t.Fatalf("unmarshal tool.finished payload: %v", err)
		}
		return p
	}
	t.Fatalf("no tool.finished event for %s", callID)
	return payloadToolFinished{}
}

// ---------------------------------------------------------------------------
// capture model + recording tool
// ---------------------------------------------------------------------------

type contractModelEntry struct {
	mode  string
	input []*schema.AgenticMessage
}

// contractCaptureModel replays a script like ScriptedModel but records the
// mode and full input of every inner entry (including retried attempts);
// failing entries do not consume a script slot.
type contractCaptureModel struct {
	mu       sync.Mutex
	script   []*schema.AgenticMessage
	failures map[int]error // entry index → fail this entry once
	pos      int
	entries  []contractModelEntry
}

var _ model.AgenticModel = (*contractCaptureModel)(nil)

func (m *contractCaptureModel) Generate(_ context.Context, input []*schema.AgenticMessage, _ ...model.Option) (*schema.AgenticMessage, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	idx := len(m.entries)
	m.entries = append(m.entries, contractModelEntry{mode: "generate", input: append([]*schema.AgenticMessage(nil), input...)})
	if err := m.failures[idx]; err != nil {
		delete(m.failures, idx)
		return nil, err
	}
	if m.pos >= len(m.script) {
		return nil, fmt.Errorf("contract model exhausted at entry %d", idx)
	}
	msg := m.script[m.pos]
	m.pos++
	return msg, nil
}

func (m *contractCaptureModel) Stream(_ context.Context, input []*schema.AgenticMessage, _ ...model.Option) (*schema.StreamReader[*schema.AgenticMessage], error) {
	m.mu.Lock()
	idx := len(m.entries)
	m.entries = append(m.entries, contractModelEntry{mode: "stream", input: append([]*schema.AgenticMessage(nil), input...)})
	ferr := m.failures[idx]
	delete(m.failures, idx)
	var msg *schema.AgenticMessage
	if ferr == nil {
		if m.pos >= len(m.script) {
			m.mu.Unlock()
			return nil, fmt.Errorf("contract model exhausted at entry %d", idx)
		}
		msg = m.script[m.pos]
		m.pos++
	}
	m.mu.Unlock()
	if ferr != nil {
		return nil, ferr
	}
	r, w := schema.Pipe[*schema.AgenticMessage](1)
	go func() {
		defer w.Close()
		w.Send(msg, nil)
	}()
	return r, nil
}

func (m *contractCaptureModel) count() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.entries)
}

func (m *contractCaptureModel) entry(i int) contractModelEntry {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.entries[i]
}

// contractTool records the tool-call id Eino hands each execution via
// compose.GetToolCallID plus the raw arguments, then delegates to run.
type contractTool struct {
	spec domain.ToolSpec
	run  func(ctx context.Context, args json.RawMessage) (string, error)

	mu      sync.Mutex
	entries []contractToolEntry
}

type contractToolEntry struct {
	callID string
	args   string
}

var _ tools.Tool = (*contractTool)(nil)

const contractToolName = "contract_tool"

func newContractTool(run func(context.Context, json.RawMessage) (string, error)) *contractTool {
	return &contractTool{
		spec: domain.ToolSpec{
			Name:        contractToolName,
			Description: "Contract test tool: records the dispatched call id, echoes text.",
			Readonly:    true,
			Params: map[string]domain.ToolParam{
				"text": {Desc: "text payload", Required: true},
			},
		},
		run: run,
	}
}

func (t *contractTool) Spec() domain.ToolSpec { return t.spec }

func (t *contractTool) InvokableRun(ctx context.Context, args json.RawMessage) (string, error) {
	t.mu.Lock()
	t.entries = append(t.entries, contractToolEntry{callID: compose.GetToolCallID(ctx), args: string(args)})
	t.mu.Unlock()
	return t.run(ctx, args)
}

func (t *contractTool) snapshot() []contractToolEntry {
	t.mu.Lock()
	defer t.mu.Unlock()
	return append([]contractToolEntry(nil), t.entries...)
}

// ---------------------------------------------------------------------------
// harness
// ---------------------------------------------------------------------------

type contractHarnessOpts struct {
	enhanced    bool
	streaming   bool
	checkpoints bool
	retry       *adk.TypedModelRetryConfig[*schema.AgenticMessage]
	extraTools  []tools.Tool
	failures    map[int]error
	// prodNudge installs the production boundary middleware.
	prodNudge bool
}

type contractHarness struct {
	journal *contractJournal
	model   *contractCaptureModel
	tool    *contractTool
	svc     *Service
	backend *sqlite.Backend
}

func newContractHarness(t *testing.T, script []*schema.AgenticMessage, opts contractHarnessOpts) *contractHarness {
	t.Helper()
	ctx := context.Background()

	backend, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "contract.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = backend.Close() })

	h := &contractHarness{backend: backend}
	h.journal = &contractJournal{
		inner:  backend,
		pause:  map[string]chan struct{}{},
		failed: map[string]error{},
	}

	var allTools []tools.Tool
	if len(opts.extraTools) > 0 {
		allTools = append(allTools, opts.extraTools...)
	} else {
		h.tool = newContractTool(func(_ context.Context, args json.RawMessage) (string, error) {
			var p struct {
				Text string `json:"text"`
			}
			if err := json.Unmarshal(args, &p); err != nil {
				return "", &tools.ArgError{Field: "text", Reason: err.Error()}
			}
			return "ok-" + p.Text, nil
		})
		allTools = append(allTools, h.tool)
	}

	adapters := make([]einotool.BaseTool, 0, len(allTools))
	for _, tl := range allTools {
		inner := newToolAdapter(tl, 64<<10, nil, nil)
		var bt einotool.BaseTool = inner
		if opts.enhanced {
			bt = newEnhancedToolAdapter(inner)
		}
		adapters = append(adapters, bt)
	}

	h.model = &contractCaptureModel{script: script, failures: opts.failures}
	handlers := []adk.TypedChatModelAgentMiddleware[*schema.AgenticMessage]{}
	if opts.prodNudge {
		handlers = append(handlers, newNudgeMiddleware(1<<20))
	}
	agent, err := adk.NewTypedChatModelAgent[*schema.AgenticMessage](ctx, &adk.TypedChatModelAgentConfig[*schema.AgenticMessage]{
		Name:             "contract-agent",
		Description:      "ND-0 contract probe",
		Model:            observeChatModel(h.model),
		Handlers:         handlers,
		ToolsConfig:      adk.ToolsConfig{ToolsNodeConfig: compose.ToolsNodeConfig{Tools: adapters}},
		MaxIterations:    16,
		ModelRetryConfig: opts.retry,
	})
	if err != nil {
		t.Fatalf("new agent: %v", err)
	}

	runnerCfg := adk.TypedRunnerConfig[*schema.AgenticMessage]{Agent: agent, EnableStreaming: opts.streaming}
	var checkpointStore *VersionedCheckpointStore
	if opts.checkpoints {
		checkpointStore, err = NewVersionedCheckpointStore(backend.Blobs(), "contract-engine")
		if err != nil {
			t.Fatalf("checkpoint store: %v", err)
		}
		runnerCfg.CheckPointStore = NewEinoCheckpointAdapter(checkpointStore)
	}
	runner := adk.NewTypedRunner(runnerCfg)

	specs := make([]domain.ToolSpec, 0, len(allTools))
	byName := map[string]tools.Tool{}
	for _, tl := range allTools {
		specs = append(specs, tl.Spec())
		byName[tl.Spec().Name] = tl
	}
	policy, err := NewPolicyEngine(nil)
	if err != nil {
		t.Fatal(err)
	}
	eng := &Engine{
		runner:      runner,
		cfg:         EngineConfig{StreamBuffer: 8, MaxEventPayloadBytes: 64 << 10, MaxContextBytes: 1 << 20, Checkpoints: checkpointStore, Policy: policy},
		chatModel:   h.model,
		toolSpecs:   specs,
		activeTools: allTools,
		toolByName:  byName,
	}

	h.svc = NewService(eng, "contract", "contract-v0", ServiceDeps{Admission: backend, GenerationID: "test-generation",
		Journal: h.journal, ToolOperations: backend, Runs: backend, Messages: backend, Notes: backend,
		Sessions: backend, Sink: newTestSink(), Truncations: backend,
	})
	if opts.checkpoints {
		h.svc.deps.Approvals = backend
		h.svc.deps.ApprovalExpiration = 5 * time.Minute
	}
	return h
}

func (h *contractHarness) waitFor(t *testing.T, msg string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timed out: %s", msg)
}

func (h *contractHarness) setPause(key string) chan struct{} {
	gate := make(chan struct{})
	h.journal.mu.Lock()
	h.journal.pause[key] = gate
	h.journal.mu.Unlock()
	return gate
}

func (h *contractHarness) setFail(key string, err error) {
	h.journal.mu.Lock()
	h.journal.failed[key] = err
	h.journal.mu.Unlock()
}

func contractToolCall(id, text string) *schema.FunctionToolCall {
	return &schema.FunctionToolCall{CallID: id, Name: contractToolName, Arguments: fmt.Sprintf(`{"text":%q}`, text)}
}

// toolResults returns every tool-result message in the input — the batch
// that just settled sits at the tail right before the injected reminder
// user message, so a pure tail scan is unreliable.
func toolResults(input []*schema.AgenticMessage) []*schema.AgenticMessage {
	var out []*schema.AgenticMessage
	for _, msg := range input {
		if msg != nil && agenticIsToolResult(msg) {
			out = append(out, msg)
		}
	}
	return out
}

// ---------------------------------------------------------------------------
// scenarios
// ---------------------------------------------------------------------------

func TestNudgeContract(t *testing.T) {
	for _, streaming := range []bool{false, true} {
		for _, enhanced := range []bool{false, true} {
			t.Run(fmt.Sprintf("stream=%v/enhanced=%v", streaming, enhanced), func(t *testing.T) {
				h := newContractHarness(t, []*schema.AgenticMessage{testToolCalls(contractToolCall("native-id", "nonce")), testAssistant("done")}, contractHarnessOpts{streaming: streaming, enhanced: enhanced, prodNudge: true})
				run, err := h.svc.Run(context.Background(), "native-session", "echo nonce")
				if err != nil {
					t.Fatal(err)
				}
				waitForRunStatus(t, h.backend, run, domain.RunCompleted)
				if h.model.count() != 2 {
					t.Fatalf("model calls: %d", h.model.count())
				}
				results := toolResults(h.model.entry(1).input)
				if len(results) != 1 || agenticToolResults(results[0])[0].CallID != "native-id" {
					t.Fatalf("result correlation: %+v", results)
				}
				op, err := h.backend.GetToolOperation(context.Background(), run, "native-id")
				if err != nil || op.State != domain.ToolOperationCompleted || op.OutcomeStatus != "success" {
					t.Fatalf("durable outcome: %+v %v", op, err)
				}
				entries := h.tool.snapshot()
				if len(entries) != 1 || entries[0].callID != "native-id" {
					t.Fatalf("adapter identity: %+v", entries)
				}
			})
		}
	}
}
