package runtime

// ND-0 (docs/plans/nudge/ND-0.md): prove the pinned Eino v0.9.13 surfaces can
// carry the nudge design (docs/architecture/NUDGE-DESIGN.md §4/§6) without a
// second runtime. A test-only WrapModel middleware acts as the request
// barrier: before delegating to the inner model it waits for every tool call
// whose results sit at the tail of the input to be durably journaled —
// tracked through a storage.Journal decorator feeding a test-local state
// that mirrors the §4 nudgeState contract. The file pins call-ID fidelity at
// the adapters, result ordering vs. journal durability, Abort on journal
// failure, cancellation while waiting, per-attempt re-entry under provider
// retry, fresh state on resume, and compaction-vs-wrapper ordering.
// Test-only: no production code is touched.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
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

// ---------------------------------------------------------------------------
// test-local mirror of the §4 nudgeState contract
// ---------------------------------------------------------------------------

// contractNotice is the reminder the barrier schedules after a batch seals.
// Its text is an unmistakable marker tests grep inside captured inputs.
type contractNotice struct {
	batch int
	text  string
}

// contractBatch is one model turn's tool calls in request order. The batch
// seals when every outstanding id has a durable result.
type contractBatch struct {
	seq       int
	ids       []string
	remaining map[string]bool
	sealed    bool
}

// contractState mirrors the production nudgeState surface the design
// specifies: Register (request observed), Complete (result durable),
// Abort (journal failure), and Await (the wrapper's peek-Take: it blocks
// until the batch covering ids is sealed, then returns its notice plus
// whether this batch is handed out for the first time — a retry attempt
// must see the same notice with first=false).
type contractState struct {
	mu          sync.Mutex
	changed     chan struct{}
	seq         int
	cur         *contractBatch
	notice      *contractNotice
	takenSeq    int
	aborted     error
	waiting     int
	emitNotices bool
}

func newContractState(emitNotices bool) *contractState {
	return &contractState{changed: make(chan struct{}), emitNotices: emitNotices}
}

// Register observes a dispatched tool call id. The first id of a turn opens
// a fresh batch once the previous one sealed.
func (s *contractState) Register(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cur == nil || s.cur.sealed {
		s.seq++
		s.cur = &contractBatch{seq: s.seq, remaining: map[string]bool{}}
	}
	s.cur.ids = append(s.cur.ids, id)
	s.cur.remaining[id] = true
	s.broadcastLocked()
}

// Complete records a durably journaled tool result for id. A completed id
// with no open batch (e.g. a resumed leg replaying the decided tool result
// into a fresh state) opens and immediately seals a singleton batch, so the
// next wrapper entry is not held for work the prior leg already finished.
func (s *contractState) Complete(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cur == nil || s.cur.sealed {
		if s.cur != nil {
			for _, seen := range s.cur.ids {
				if seen == id {
					return // duplicate completion for a sealed batch
				}
			}
		}
		s.seq++
		b := &contractBatch{seq: s.seq, ids: []string{id}, remaining: map[string]bool{}}
		s.cur = b
		s.sealLocked(b)
		return
	}
	delete(s.cur.remaining, id)
	if len(s.cur.remaining) == 0 {
		s.sealLocked(s.cur)
	}
}

func (s *contractState) sealLocked(b *contractBatch) {
	b.sealed = true
	if s.emitNotices {
		s.notice = &contractNotice{batch: b.seq, text: fmt.Sprintf("contract-nudge-%d", b.seq)}
	}
	s.broadcastLocked()
}

// Abort releases every current and future waiter with err — the test-local
// twin of nudgeState.Abort on journal failure.
func (s *contractState) Abort(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.aborted == nil {
		s.aborted = err
	}
	s.broadcastLocked()
}

func (s *contractState) broadcastLocked() {
	close(s.changed)
	s.changed = make(chan struct{})
}

func (s *contractState) sealedLocked(ids []string) bool {
	if s.cur == nil || !s.cur.sealed {
		return false
	}
	registered := make(map[string]bool, len(s.cur.ids))
	for _, id := range s.cur.ids {
		registered[id] = true
	}
	for _, id := range ids {
		if !registered[id] {
			return false
		}
	}
	return true
}

// Await waits until every id is registered and its batch is sealed — all of
// the batch's tool results durable in the journal — then returns the batch's
// pending notice plus first=true exactly once per batch (peek semantics, so
// a provider retry re-entering the wrapper cannot schedule a second
// reminder). A nil notice means nothing to inject.
func (s *contractState) Await(ctx context.Context, ids []string) (*contractNotice, bool, error) {
	if len(ids) == 0 {
		return nil, false, nil
	}
	s.mu.Lock()
	s.waiting++
	defer func() {
		s.mu.Lock()
		s.waiting--
		s.mu.Unlock()
	}()
	for {
		if s.aborted != nil {
			err := s.aborted
			s.mu.Unlock()
			return nil, false, err
		}
		if s.sealedLocked(ids) {
			notice := s.notice
			first := s.cur.seq != s.takenSeq
			if first {
				s.takenSeq = s.cur.seq
			}
			s.mu.Unlock()
			return notice, first, nil
		}
		changed := s.changed
		s.mu.Unlock()
		select {
		case <-changed:
		case <-ctx.Done():
			return nil, false, ctx.Err()
		}
		s.mu.Lock()
	}
}

func (s *contractState) waitingCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.waiting
}

// contractStateHolder lets the harness install a fresh state per drive leg:
// the barrier and the journal both load the current state through it — the
// only seam available for resume, since resumeRun rebuilds ctx from
// context.Background and ctx values cannot carry state across legs.
type contractStateHolder struct{ p atomic.Pointer[contractState] }

func (h *contractStateHolder) load() *contractState    { return h.p.Load() }
func (h *contractStateHolder) swap(s *contractState)   { h.p.Store(s) }
func (h *contractStateHolder) current() *contractState { return h.p.Load() }

// ---------------------------------------------------------------------------
// test-only WrapModel barrier (the seam ND-3's middleware will own)
// ---------------------------------------------------------------------------

type contractBarrier struct {
	*adk.BaseChatModelAgentMiddleware
	holder *contractStateHolder
	emit   func(context.Context, *contractNotice) error

	wraps    atomic.Int32
	mu       sync.Mutex
	prepared [][]*schema.Message // every input the wrapper observed, pre-injection
}

func (b *contractBarrier) WrapModel(_ context.Context, m model.BaseModel[*schema.Message], _ *adk.ModelContext) (model.BaseModel[*schema.Message], error) {
	b.wraps.Add(1)
	return &contractWrappedModel{inner: m, barrier: b}, nil
}

func (b *contractBarrier) record(input []*schema.Message) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.prepared = append(b.prepared, append([]*schema.Message(nil), input...))
}

// contractWrappedModel implements the §6 request barrier: it waits for the
// durable seal of the batch whose results sit at the input tail BEFORE the
// inner Generate/Stream call, then appends the scheduled reminder to a COPY
// of the input — the request slice is never mutated in place.
type contractWrappedModel struct {
	inner   model.BaseModel[*schema.Message]
	barrier *contractBarrier
}

func (w *contractWrappedModel) Generate(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	prepared, err := w.prepare(ctx, input)
	if err != nil {
		return nil, err
	}
	return w.inner.Generate(ctx, prepared, opts...)
}

func (w *contractWrappedModel) Stream(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	prepared, err := w.prepare(ctx, input)
	if err != nil {
		return nil, err
	}
	return w.inner.Stream(ctx, prepared, opts...)
}

func (w *contractWrappedModel) prepare(ctx context.Context, input []*schema.Message) ([]*schema.Message, error) {
	w.barrier.record(input)
	ids := trailingToolCallIDs(input)
	if len(ids) == 0 {
		return input, nil
	}
	st := w.barrier.holder.load()
	if st == nil {
		return input, nil
	}
	notice, first, err := st.Await(ctx, ids)
	if err != nil {
		return nil, err
	}
	if notice == nil {
		return input, nil
	}
	if first && w.barrier.emit != nil {
		if err := w.barrier.emit(ctx, notice); err != nil {
			return nil, err
		}
	}
	out := make([]*schema.Message, len(input), len(input)+1)
	copy(out, input)
	return append(out, schema.UserMessage(notice.text)), nil
}

// trailingToolCallIDs returns the call ids of the contiguous tool-result
// messages at the input tail — the batch that just settled and must be
// durable before the inner model may run. Keying on the input (not on "the
// latest registered batch") matters: the engine can reach the next model
// call before the consumer finishes journaling the request events.
func trailingToolCallIDs(input []*schema.Message) []string {
	var ids []string
	for i := len(input) - 1; i >= 0; i-- {
		msg := input[i]
		if msg == nil || msg.Role != schema.Tool || msg.ToolCallID == "" {
			break
		}
		ids = append(ids, msg.ToolCallID)
	}
	for i, j := 0, len(ids)-1; i < j; i, j = i+1, j-1 {
		ids[i], ids[j] = ids[j], ids[i]
	}
	return ids
}

// ---------------------------------------------------------------------------
// journal decorator: feeds the state from durable events, with hooks to
// pause/fail appends and withhold seals
// ---------------------------------------------------------------------------

type contractJournal struct {
	inner  storage.Journal
	holder *contractStateHolder

	mu      sync.Mutex
	events  []domain.RunEvent
	pause   map[string]chan struct{} // "<type>:<callID>" → the append waits on the channel
	failed  map[string]error         // same key → the append returns this error without writing
	hold    map[string]bool          // callIDs whose finished append lands but is never Completed
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
			if st := j.holder.load(); st != nil {
				st.Abort(ferr)
			}
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
		if st := j.holder.load(); st != nil {
			st.Abort(err)
		}
		return 0, err
	}
	j.mu.Lock()
	j.events = append(j.events, commit.Events...)
	j.mu.Unlock()
	// Feed the state only after the append landed: the seal boundary is
	// durability, not event emission.
	if st := j.holder.load(); st != nil {
		for _, ev := range commit.Events {
			switch ev.Type {
			case domain.EventToolRequested:
				if id := j.toolCallID(ev); id != "" {
					st.Register(id)
				}
			case domain.EventToolFinished:
				id := j.toolCallID(ev)
				if id == "" {
					continue
				}
				j.mu.Lock()
				held := j.hold[id]
				j.mu.Unlock()
				if !held {
					st.Complete(id)
				}
			}
		}
	}
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
	input []*schema.Message
}

// contractCaptureModel replays a script like ScriptedModel but records the
// mode and full input of every inner entry (including retried attempts);
// failing entries do not consume a script slot.
type contractCaptureModel struct {
	mu       sync.Mutex
	script   []*schema.Message
	failures map[int]error // entry index → fail this entry once
	pos      int
	entries  []contractModelEntry
}

var _ model.ToolCallingChatModel = (*contractCaptureModel)(nil)

func (m *contractCaptureModel) Generate(_ context.Context, input []*schema.Message, _ ...model.Option) (*schema.Message, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	idx := len(m.entries)
	m.entries = append(m.entries, contractModelEntry{mode: "generate", input: append([]*schema.Message(nil), input...)})
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

func (m *contractCaptureModel) Stream(_ context.Context, input []*schema.Message, _ ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	m.mu.Lock()
	idx := len(m.entries)
	m.entries = append(m.entries, contractModelEntry{mode: "stream", input: append([]*schema.Message(nil), input...)})
	ferr := m.failures[idx]
	delete(m.failures, idx)
	var msg *schema.Message
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
	r, w := schema.Pipe[*schema.Message](1)
	go func() {
		defer w.Close()
		w.Send(msg, nil)
	}()
	return r, nil
}

func (m *contractCaptureModel) WithTools([]*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	return m, nil
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
	noBarrier   bool
	checkpoints bool
	emitNotices bool
	retry       *adk.ModelRetryConfig
	extraTools  []tools.Tool
	failures    map[int]error
}

type contractHarness struct {
	holder  *contractStateHolder
	state   *contractState
	journal *contractJournal
	model   *contractCaptureModel
	tool    *contractTool
	barrier *contractBarrier
	svc     *Service
	backend *sqlite.Backend

	emits atomic.Int32
	marks sync.Map // callID → recoverable failure mark (the ND-1 seam)
}

func newContractHarness(t *testing.T, script []*schema.Message, opts contractHarnessOpts) *contractHarness {
	t.Helper()
	ctx := context.Background()

	backend, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "contract.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = backend.Close() })

	h := &contractHarness{backend: backend, holder: &contractStateHolder{}}
	h.state = newContractState(opts.emitNotices)
	h.holder.swap(h.state)
	h.journal = &contractJournal{
		inner: backend, holder: h.holder,
		pause:  map[string]chan struct{}{},
		failed: map[string]error{}, hold: map[string]bool{},
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
		inner := newToolAdapter(tl, 64<<10, nil, nil, nil)
		var bt einotool.BaseTool = inner
		if opts.enhanced {
			bt = newEnhancedToolAdapter(inner)
		}
		adapters = append(adapters, bt)
	}

	h.model = &contractCaptureModel{script: script, failures: opts.failures}
	h.barrier = &contractBarrier{
		BaseChatModelAgentMiddleware: &adk.BaseChatModelAgentMiddleware{},
		holder:                       h.holder,
		emit: func(context.Context, *contractNotice) error {
			h.emits.Add(1)
			return nil
		},
	}

	handlers := []adk.ChatModelAgentMiddleware{}
	if !opts.noBarrier {
		handlers = append(handlers, h.barrier)
	}
	agent, err := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Name:             "contract-agent",
		Description:      "ND-0 contract probe",
		Model:            observeModelStreams(h.model),
		Handlers:         handlers,
		ToolsConfig:      adk.ToolsConfig{ToolsNodeConfig: compose.ToolsNodeConfig{Tools: adapters}},
		MaxIterations:    16,
		ModelRetryConfig: opts.retry,
	})
	if err != nil {
		t.Fatalf("new agent: %v", err)
	}

	runnerCfg := adk.RunnerConfig{Agent: agent, EnableStreaming: opts.streaming}
	var checkpointStore *VersionedCheckpointStore
	if opts.checkpoints {
		checkpointStore, err = NewVersionedCheckpointStore(backend.Blobs(), "contract-engine")
		if err != nil {
			t.Fatalf("checkpoint store: %v", err)
		}
		runnerCfg.CheckPointStore = NewEinoCheckpointAdapter(checkpointStore)
	}
	runner := adk.NewRunner(ctx, runnerCfg)

	specs := make([]domain.ToolSpec, 0, len(allTools))
	byName := map[string]tools.Tool{}
	for _, tl := range allTools {
		specs = append(specs, tl.Spec())
		byName[tl.Spec().Name] = tl
	}
	eng := &Engine{
		runner:      runner,
		cfg:         EngineConfig{StreamBuffer: 8, MaxEventPayloadBytes: 64 << 10, MaxContextBytes: 1 << 20, Checkpoints: checkpointStore},
		chatModel:   h.model,
		toolSpecs:   specs,
		activeTools: allTools,
		toolByName:  byName,
	}

	h.svc = NewService(eng, "contract", "contract-v0", ServiceDeps{
		Journal: h.journal, Runs: backend, Messages: backend, Notes: backend,
		Sessions: backend, Sink: newTestSink(), Truncations: backend,
	})
	if opts.checkpoints {
		h.svc.deps.Approvals = backend
		h.svc.deps.ApprovalExpiration = 5 * time.Minute
	}
	return h
}

// freshState installs a brand-new state the way ND-2 will attach a fresh
// nudgeState to every resume leg, and returns the displaced one.
func (h *contractHarness) freshState(emitNotices bool) *contractState {
	old := h.holder.current()
	h.state = newContractState(emitNotices)
	h.holder.swap(h.state)
	return old
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

func (h *contractHarness) setHold(callID string) {
	h.journal.mu.Lock()
	h.journal.hold[callID] = true
	h.journal.mu.Unlock()
}

func contractToolCall(id, text string) schema.ToolCall {
	return schema.ToolCall{ID: id, Function: schema.FunctionCall{Name: contractToolName, Arguments: fmt.Sprintf(`{"text":%q}`, text)}}
}

// toolResults returns every tool-result message in the input — the batch
// that just settled sits at the tail right before the injected reminder
// user message, so a pure tail scan is unreliable.
func toolResults(input []*schema.Message) []*schema.Message {
	var out []*schema.Message
	for _, msg := range input {
		if msg != nil && msg.Role == schema.Tool && msg.ToolCallID != "" {
			out = append(out, msg)
		}
	}
	return out
}

// ---------------------------------------------------------------------------
// scenarios
// ---------------------------------------------------------------------------

// runParallelBatch drives the core ND-0 scenario on either adapter flavor:
// the model requests c1+c2 in one turn, c2 finishes first, the journal
// append for c1's result is held, and the barrier must keep the inner model
// out until the append lands.
func runParallelBatch(t *testing.T, enhanced bool) {
	c1, c2 := "call-c1", "call-c2"
	script := []*schema.Message{
		schema.AssistantMessage("", []schema.ToolCall{contractToolCall(c1, "first"), contractToolCall(c2, "second")}),
		schema.AssistantMessage("final answer", nil),
	}
	h := newContractHarness(t, script, contractHarnessOpts{enhanced: enhanced, streaming: true, emitNotices: true})

	releaseC1Tool := make(chan struct{})
	releaseC1Append := h.setPause("tool.finished:" + c1)
	// c1's tool result is held at the tool until c2's result is durable, so
	// finished events land in fixed order (c2 before c1).
	h.tool.run = func(_ context.Context, args json.RawMessage) (string, error) {
		var p struct {
			Text string `json:"text"`
		}
		if err := json.Unmarshal(args, &p); err != nil {
			return "", err
		}
		if p.Text == "first" {
			<-releaseC1Tool
		}
		return "ok-" + p.Text, nil
	}

	ctx := context.Background()
	runID, err := h.svc.Run(ctx, "sess-contract", "run the contract tools")
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	// Wait until c2's result is durable; c1's tool is still held.
	h.waitFor(t, "tool.finished for "+c2, func() bool { return h.journal.hasEvent(domain.EventToolFinished, c2) })
	close(releaseC1Tool)

	// The consumer parks on the paused append of c1's finished event.
	h.waitFor(t, "paused append for "+c1, func() bool { return h.journal.pausingCount() > 0 })

	// The engine has already re-entered the wrapper for the next model call;
	// the barrier must hold it out of the inner model while c1's result is
	// not durable.
	h.waitFor(t, "wrapper awaiting seal", func() bool { return h.state.waitingCount() > 0 })
	if got := h.model.count(); got != 1 {
		t.Fatalf("inner model entered %d times while a result was not durable, want 1", got)
	}
	time.Sleep(150 * time.Millisecond)
	if got := h.model.count(); got != 1 {
		t.Fatalf("inner model entered %d times while a result was not durable, want 1", got)
	}
	// While the wrapper waits, Eino must still yield the finished tool
	// results to the consumer — c2's finished event is already journaled and
	// c1's finished event exists (the append is paused inside the journal,
	// not missing from the event stream).
	close(releaseC1Append)
	waitForRunStatus(t, h.backend, runID, domain.RunCompleted)

	// Journal order: results out of order (c2 before c1), each preceded by
	// its request.
	j := h.journal
	if j.indexOf(domain.EventToolRequested, c1) < 0 || j.indexOf(domain.EventToolRequested, c2) < 0 {
		t.Fatal("tool.requested events missing for c1/c2")
	}
	if j.indexOf(domain.EventToolFinished, c2) >= j.indexOf(domain.EventToolFinished, c1) {
		t.Fatal("expected c2's finished event to precede c1's in journal order")
	}
	if j.indexOf(domain.EventToolRequested, c1) > j.indexOf(domain.EventToolFinished, c1) ||
		j.indexOf(domain.EventToolRequested, c2) > j.indexOf(domain.EventToolFinished, c2) {
		t.Fatal("a finished event preceded its requested event in the journal")
	}

	// ID fidelity: each adapter execution saw the exact requested call id.
	entries := h.tool.snapshot()
	if len(entries) != 2 {
		t.Fatalf("tool entries = %d, want 2", len(entries))
	}
	byArgs := map[string]string{}
	for _, e := range entries {
		byArgs[e.args] = e.callID
	}
	if got := byArgs[fmt.Sprintf(`{"text":%q}`, "first")]; got != c1 {
		t.Fatalf("call id for first args = %q, want %s", got, c1)
	}
	if got := byArgs[fmt.Sprintf(`{"text":%q}`, "second")]; got != c2 {
		t.Fatalf("call id for second args = %q, want %s", got, c2)
	}

	// The next model call carried exactly one result per requested id plus
	// the injected reminder as the tail message.
	if h.model.count() != 2 {
		t.Fatalf("inner model calls = %d, want 2", h.model.count())
	}
	input2 := h.model.entry(1).input
	seen := map[string]int{}
	for _, m := range toolResults(input2) {
		seen[m.ToolCallID]++
	}
	// The trailing tool results are contiguous before the reminder; the
	// reminder is a UserMessage appended after them.
	if last := input2[len(input2)-1]; last.Role != schema.User || last.Content != "contract-nudge-1" {
		t.Fatalf("injected reminder missing from tail: role=%s content=%q", last.Role, last.Content)
	}
	if seen[c1] != 1 || seen[c2] != 1 || len(seen) != 2 {
		t.Fatalf("next input results by call id = %v, want exactly one each of %s,%s", seen, c1, c2)
	}
	if got := h.emits.Load(); got != 1 {
		t.Fatalf("scheduling emissions = %d, want 1", got)
	}

	// Result contents correlate to their ids.
	if got := h.journal.finishedPayload(t, c1).Result; !strings.Contains(got, "ok-first") {
		t.Fatalf("finished(c1).Result = %q, want ok-first", got)
	}
	if got := h.journal.finishedPayload(t, c2).Result; !strings.Contains(got, "ok-second") {
		t.Fatalf("finished(c2).Result = %q, want ok-second", got)
	}
	if got := h.state.waitingCount(); got != 0 {
		t.Fatalf("leaked waiters = %d", got)
	}
}

func TestNudgeContract(t *testing.T) {
	t.Run("OrdinaryAdapterBatchOrderAndDurability", func(t *testing.T) {
		runParallelBatch(t, false)
	})

	t.Run("EnhancedAdapterBatchOrderAndDurability", func(t *testing.T) {
		runParallelBatch(t, true)
	})

	// Baseline characterization: with no barrier installed, nothing orders
	// the next model call after journal durability — the engine may enter
	// the inner model while a result's append is still paused. Either
	// outcome is informative; an early entry proves the need for the
	// specified barrier rather than a failure.
	t.Run("BaselineNoBarrierMayEnterBeforeDurability", func(t *testing.T) {
		c1, c2 := "call-b1", "call-b2"
		script := []*schema.Message{
			schema.AssistantMessage("", []schema.ToolCall{contractToolCall(c1, "x"), contractToolCall(c2, "y")}),
			schema.AssistantMessage("done", nil),
		}
		h := newContractHarness(t, script, contractHarnessOpts{streaming: true, noBarrier: true})
		releaseC1Append := h.setPause("tool.finished:" + c1)
		h.tool.run = func(_ context.Context, args json.RawMessage) (string, error) {
			var p struct {
				Text string `json:"text"`
			}
			if err := json.Unmarshal(args, &p); err != nil {
				return "", err
			}
			return "ok-" + p.Text, nil
		}

		runID, err := h.svc.Run(context.Background(), "sess-base", "baseline")
		if err != nil {
			t.Fatalf("run: %v", err)
		}
		h.waitFor(t, "paused append for "+c1, func() bool { return h.journal.pausingCount() > 0 })

		earlyEntry := false
		deadline := time.Now().Add(3 * time.Second)
		for time.Now().Before(deadline) {
			if h.model.count() >= 2 {
				earlyEntry = true
				break
			}
			time.Sleep(5 * time.Millisecond)
		}
		t.Logf("baseline: inner model entered before durable results = %v (calls=%d)", earlyEntry, h.model.count())
		close(releaseC1Append)
		waitForRunStatus(t, h.backend, runID, domain.RunCompleted)
		if h.model.count() != 2 {
			t.Fatalf("inner model calls = %d, want 2", h.model.count())
		}
	})

	// Non-streamed runner (EnableStreaming=false → Generate path): the
	// whole assistant tool-request message arrives at once; every finished
	// event must still be preceded by its requested event in the journal
	// and adapters see the exact requested ids.
	t.Run("GeneratePathRegisteredBeforeResults", func(t *testing.T) {
		c1, c2 := "call-g1", "call-g2"
		script := []*schema.Message{
			schema.AssistantMessage("", []schema.ToolCall{contractToolCall(c1, "p"), contractToolCall(c2, "q")}),
			schema.AssistantMessage("gen done", nil),
		}
		h := newContractHarness(t, script, contractHarnessOpts{streaming: false, emitNotices: true})
		runID, err := h.svc.Run(context.Background(), "sess-gen", "generate path")
		if err != nil {
			t.Fatalf("run: %v", err)
		}
		waitForRunStatus(t, h.backend, runID, domain.RunCompleted)

		if h.model.count() != 2 {
			t.Fatalf("inner model calls = %d, want 2", h.model.count())
		}
		for i := 0; i < 2; i++ {
			if mode := h.model.entry(i).mode; mode != "generate" {
				t.Fatalf("entry %d mode = %s, want generate", i, mode)
			}
		}
		for _, id := range []string{c1, c2} {
			req, fin := h.journal.indexOf(domain.EventToolRequested, id), h.journal.indexOf(domain.EventToolFinished, id)
			if req < 0 || fin < 0 || req > fin {
				t.Fatalf("journal order broken for %s: requested@%d finished@%d", id, req, fin)
			}
		}
		seen := map[string]int{}
		for _, m := range toolResults(h.model.entry(1).input) {
			seen[m.ToolCallID]++
		}
		if seen[c1] != 1 || seen[c2] != 1 {
			t.Fatalf("generate-path next input results = %v", seen)
		}
	})

	// A failed (recoverable) invocation keeps its metadata keyed to the
	// exact call id: the adapter path sees nil error while the failure mark
	// joins to c1's finished event and never to c2's.
	t.Run("FailureMetadataCorrelatesByCallID", func(t *testing.T) {
		c1, c2 := "call-f1", "call-f2"
		script := []*schema.Message{
			schema.AssistantMessage("", []schema.ToolCall{contractToolCall(c1, "fail"), contractToolCall(c2, "fine")}),
			schema.AssistantMessage("handled", nil),
		}
		h := newContractHarness(t, script, contractHarnessOpts{streaming: true})
		h.tool.run = func(ctx context.Context, args json.RawMessage) (string, error) {
			var p struct {
				Text string `json:"text"`
			}
			if err := json.Unmarshal(args, &p); err != nil {
				return "", err
			}
			if p.Text == "fail" {
				// The recoverable-failure seam: metadata keyed to the exact
				// call id, while the adapter still returns a nil error to
				// Eino so the result reaches the model.
				h.marks.Store(compose.GetToolCallID(ctx), "recoverable: simulated timeout")
				return "ERR: simulated read timeout", nil
			}
			return "ok-" + p.Text, nil
		}
		runID, err := h.svc.Run(context.Background(), "sess-fail", "failure metadata")
		if err != nil {
			t.Fatalf("run: %v", err)
		}
		waitForRunStatus(t, h.backend, runID, domain.RunCompleted)

		if _, ok := h.marks.Load(c1); !ok {
			t.Fatalf("no failure mark recorded under %s", c1)
		}
		if _, ok := h.marks.Load(c2); ok {
			t.Fatalf("failure metadata leaked onto %s", c2)
		}
		if got := h.journal.finishedPayload(t, c1).Result; !strings.Contains(got, "ERR:") {
			t.Fatalf("finished(%s).Result = %q, want the failure text", c1, got)
		}
		if got := h.journal.finishedPayload(t, c2).Result; !strings.Contains(got, "ok-fine") {
			t.Fatalf("finished(%s).Result = %q", c2, got)
		}
	})

	// Journal append failure must Abort the barrier wait: the wrapper
	// releases, the inner model count stays at the pre-failure value, and no
	// waiter leaks.
	t.Run("JournalAppendFailureAbortsBarrier", func(t *testing.T) {
		c1 := "call-j1"
		script := []*schema.Message{
			schema.AssistantMessage("", []schema.ToolCall{contractToolCall(c1, "x")}),
			schema.AssistantMessage("never", nil),
		}
		h := newContractHarness(t, script, contractHarnessOpts{streaming: true})
		sentinel := errors.New("contract journal write failed")
		h.setFail("tool.finished:"+c1, sentinel)

		runID, err := h.svc.Run(context.Background(), "sess-jfail", "journal failure")
		if err != nil {
			t.Fatalf("run: %v", err)
		}
		waitForRunStatus(t, h.backend, runID, domain.RunFailed)
		if got := h.model.count(); got != 1 {
			t.Fatalf("inner model calls after journal failure = %d, want 1", got)
		}
		if got := h.state.waitingCount(); got != 0 {
			t.Fatalf("leaked waiters after abort = %d", got)
		}
		if h.state.aborted == nil {
			t.Fatal("state was not aborted on journal failure")
		}
	})

	// Cancelling the run while the barrier waits must release the wrapper
	// with a cancellation error — no leaked wait, no hang.
	t.Run("CancelWhileAwaitingSeal", func(t *testing.T) {
		c1 := "call-x1"
		script := []*schema.Message{
			schema.AssistantMessage("", []schema.ToolCall{contractToolCall(c1, "x")}),
			schema.AssistantMessage("never", nil),
		}
		h := newContractHarness(t, script, contractHarnessOpts{streaming: true})
		// The append lands but the seal is withheld, so the wrapper waits.
		h.setHold(c1)

		runID, err := h.svc.Run(context.Background(), "sess-cancel", "cancel")
		if err != nil {
			t.Fatalf("run: %v", err)
		}
		h.waitFor(t, "tool.finished journaled", func() bool { return h.journal.hasEvent(domain.EventToolFinished, c1) })
		h.waitFor(t, "wrapper waiting", func() bool { return h.state.waitingCount() > 0 })
		if !h.svc.Cancel(runID) {
			t.Fatal("cancel returned false")
		}
		waitForRunStatus(t, h.backend, runID, domain.RunCancelled)
		if got := h.model.count(); got != 1 {
			t.Fatalf("inner model calls = %d, want 1", got)
		}
		if got := h.state.waitingCount(); got != 0 {
			t.Fatalf("leaked waiters after cancel = %d", got)
		}
	})

	// Provider retry re-invokes WrapModel AND the wrapped model with the
	// same input per attempt (adk/wrappers.go): the barrier must produce an
	// identical prepared input and emit the scheduling action exactly once
	// for the batch.
	t.Run("ProviderRetrySeesIdenticalPreparedInput", func(t *testing.T) {
		c1 := "call-r1"
		injected := errors.New("injected provider failure")
		script := []*schema.Message{
			schema.AssistantMessage("", []schema.ToolCall{contractToolCall(c1, "go")}),
			schema.AssistantMessage("recovered", nil),
		}
		h := newContractHarness(t, script, contractHarnessOpts{
			streaming:   true,
			emitNotices: true,
			failures:    map[int]error{1: injected}, // second entry = first attempt of turn 2
			retry: &adk.ModelRetryConfig{
				MaxRetries:  1,
				BackoffFunc: func(context.Context, int) time.Duration { return 0 },
			},
		})
		runID, err := h.svc.Run(context.Background(), "sess-retry", "retry")
		if err != nil {
			t.Fatalf("run: %v", err)
		}
		waitForRunStatus(t, h.backend, runID, domain.RunCompleted)

		if got := h.model.count(); got != 3 {
			t.Fatalf("inner model entries = %d, want 3 (turn1, attempt1, attempt2)", got)
		}
		attempt1, attempt2 := h.model.entry(1).input, h.model.entry(2).input
		if !reflect.DeepEqual(attempt1, attempt2) {
			t.Fatalf("retry attempt inputs differ:\nattempt1=%v\nattempt2=%v", attempt1, attempt2)
		}
		if last := attempt2[len(attempt2)-1]; last.Content != "contract-nudge-1" {
			t.Fatalf("retried input tail lacks reminder: %q", last.Content)
		}
		if got := h.emits.Load(); got != 1 {
			t.Fatalf("scheduling emissions = %d, want 1 (deduped across retries)", got)
		}
	})

	// Approval interrupt: the pre-decision leg converts nothing to a
	// failure, resume installs a fresh state, and the replayed tool result
	// seals a singleton batch so the resumed model call proceeds.
	t.Run("ApprovalInterruptResumeFreshState", func(t *testing.T) {
		callID := ApprovalFlowCallID
		noteStore, err := sqlite.Open(context.Background(), filepath.Join(t.TempDir(), "notes.db"))
		if err != nil {
			t.Fatalf("open note store: %v", err)
		}
		t.Cleanup(func() { _ = noteStore.Close() })
		ts, err := tools.Builtin(noteStore).Resolve([]string{tools.WriteNoteName})
		if err != nil {
			t.Fatalf("resolve write_note: %v", err)
		}
		script := []*schema.Message{
			schema.AssistantMessage("", []schema.ToolCall{{
				ID:       callID,
				Function: schema.FunctionCall{Name: tools.WriteNoteName, Arguments: `{"content":"buy milk"}`},
			}}),
			schema.AssistantMessage("Done: the note has been handled.", nil),
		}
		h := newContractHarness(t, script, contractHarnessOpts{streaming: true, checkpoints: true, emitNotices: true, extraTools: ts})

		runID, err := h.svc.Run(context.Background(), "sess-approval", "note that I need milk")
		if err != nil {
			t.Fatalf("run: %v", err)
		}
		approval := waitForPendingApproval(t, h.backend, runID)
		if got := h.model.count(); got != 1 {
			t.Fatalf("inner model calls during suspend = %d, want 1", got)
		}
		// Pre-decision: no finished event for the call — no conversion.
		if h.journal.hasEvent(domain.EventToolFinished, callID) {
			t.Fatal("suspended call produced a tool.finished event before decision")
		}

		old := h.freshState(true)
		if err := h.svc.DecideApproval(context.Background(), approval.ID, domain.ApprovalApproved); err != nil {
			t.Fatalf("decide approval: %v", err)
		}
		waitForRunStatus(t, h.backend, runID, domain.RunCompleted)

		if h.holder.current() == old {
			t.Fatal("resume still used the pre-interrupt state")
		}
		fin := h.journal.finishedPayload(t, callID)
		if fin.Error != "" {
			t.Fatalf("approved call finished with error %q — interrupt converted to failure", fin.Error)
		}
		if got := h.model.count(); got != 2 {
			t.Fatalf("inner model calls after resume = %d, want 2", got)
		}
		if got := h.emits.Load(); got != 1 {
			t.Fatalf("scheduling emissions = %d, want 1", got)
		}
		if got := h.state.waitingCount(); got != 0 {
			t.Fatalf("leaked waiters after resume = %d", got)
		}
	})

	// Compaction ordering: the barrier wrapper must observe the FINAL,
	// already-rewritten request, while the compaction-internal summary call
	// must bypass the WrapModel chain entirely (it owns the raw model).
	// Driven at engine level — no Service — like the existing compaction
	// tests.
	t.Run("CompactionRunsBeforeBarrierSummaryBypassesIt", func(t *testing.T) {
		ctx := context.Background()
		holder := &contractStateHolder{}
		st := newContractState(true)
		holder.swap(st)

		feed := compactFeed(16) // ~1k tokens, far above the 200-token trigger
		// The feed's tool results are durable history: pre-seal every call id.
		for _, msg := range feed {
			if msg != nil && msg.Role == schema.Tool && msg.ToolCallID != "" {
				st.Register(msg.ToolCallID)
			}
		}
		for _, msg := range feed {
			if msg != nil && msg.Role == schema.Tool && msg.ToolCallID != "" {
				st.Complete(msg.ToolCallID)
			}
		}

		emits := atomic.Int32{}
		barrier := &contractBarrier{
			BaseChatModelAgentMiddleware: &adk.BaseChatModelAgentMiddleware{},
			holder:                       holder,
			emit: func(context.Context, *contractNotice) error {
				emits.Add(1)
				return nil
			},
		}
		mainModel := &contractCaptureModel{
			script:   []*schema.Message{schema.AssistantMessage("FINAL-9x", nil)},
			failures: map[int]error{},
		}
		summaryModel := &contractCaptureModel{
			script:   []*schema.Message{schema.AssistantMessage("CONTRACT-SUMMARY-9x", nil)},
			failures: map[int]error{},
		}
		compHandlers, err := buildCompactionHandlers(ctx, mainModel, summaryModel,
			CompactionPolicy{Enabled: true, MaxTokens: 400, TriggerPercent: 50, KeepRecent: 100},
			1<<20, nil)
		if err != nil {
			t.Fatalf("build compaction handlers: %v", err)
		}
		// Production order: compaction handlers first, barrier innermost.
		handlers := append(append([]adk.ChatModelAgentMiddleware{}, compHandlers...), barrier)

		agent, err := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
			Name:          "contract-compaction",
			Description:   "ND-0 compaction ordering probe",
			Model:         observeModelStreams(mainModel),
			Handlers:      handlers,
			MaxIterations: 8,
		})
		if err != nil {
			t.Fatalf("new agent: %v", err)
		}
		runner := adk.NewRunner(ctx, adk.RunnerConfig{Agent: agent, EnableStreaming: true})
		final := drainFinalText(t, runner.Run(ctx, feed))
		if final != "FINAL-9x" {
			t.Fatalf("final answer = %q, want FINAL-9x", final)
		}

		if got := summaryModel.count(); got == 0 {
			t.Fatal("summary model never invoked — compaction did not run")
		}
		// The summary path must bypass the wrapper: every WrapModel call
		// corresponds to an agent model entry, never to a summary call.
		if got := int(barrier.wraps.Load()); got != mainModel.count() {
			t.Fatalf("WrapModel invocations = %d, main model calls = %d — summary call must bypass the barrier", got, mainModel.count())
		}
		for i := 0; i < summaryModel.count(); i++ {
			in := summaryModel.entry(i).input
			if strings.Contains(joinContent(in), "contract-nudge") {
				t.Fatalf("summary input %d carries a nudge marker", i)
			}
		}
		// The wrapper observed the final post-rewrite request: the summary
		// content is inside what the wrapper prepared.
		barrier.mu.Lock()
		prepared := append([][]*schema.Message(nil), barrier.prepared...)
		barrier.mu.Unlock()
		if len(prepared) == 0 {
			t.Fatal("wrapper never observed an input")
		}
		last := prepared[len(prepared)-1]
		if !strings.Contains(joinContent(last), "CONTRACT-SUMMARY-9x") {
			t.Fatalf("wrapper input lacks the summary — it did not observe the final rewritten request: %q", joinContent(last))
		}
		if strings.Contains(joinContent(last), "execute all steps in order") {
			t.Fatal("wrapper input still carries the uncompacted feed")
		}
	})
}
