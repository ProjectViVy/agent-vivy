package runtime

// ND-4 (docs/plans/nudge/ND-4.md): integrated acceptance of the complete
// failure → feedback → correction/nudge → bounded-stop contract in the
// product path — real Service, Journal (sqlite), governed tools and the
// production engine wiring (NewEngine: enhanced adapters, dynamic tool
// search, innermost nudge middleware), with only the model scripted.
// Cases A–J mirror ND-4.md Task 1.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cloudwego/eino/schema"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/mcphost"
	"agent-vivy/internal/storage/sqlite"
	"agent-vivy/internal/toolhost"
	"agent-vivy/internal/tools"
	toolworldport "agent-vivy/sdk/port/toolworld"
)

// ---------------------------------------------------------------------------
// harness: real engine + real governed tools + scripted capture model
// ---------------------------------------------------------------------------

type acceptanceHarness struct {
	backend *sqlite.Backend
	journal *contractJournal // decorator: pause/fail hooks for durability gates
	model   *contractCaptureModel
	svc     *Service
	wsRoot  string
}

type acceptanceOpts struct {
	policy      domain.ApprovalPolicy
	autoApprove []string
	extraTools  []tools.Tool
	failures    map[int]error
	checkpoints bool
}

// newAcceptanceHarness builds the product path: WorkspaceManager +
// SandboxManager + the real filesystem/command backends behind the stock
// write_file/read_file/bash tools, plus any extra governed tools. The
// journal decorator wraps the real sqlite backend so tests can pause or
// fail specific appends; with a nil state holder it feeds nothing.
func newAcceptanceHarness(t *testing.T, script []*schema.Message, opts acceptanceOpts) *acceptanceHarness {
	t.Helper()
	ctx := context.Background()
	root := t.TempDir()

	backend, err := sqlite.Open(ctx, filepath.Join(root, "acceptance.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = backend.Close() })

	h := &acceptanceHarness{backend: backend, wsRoot: filepath.Join(root, "workspaces")}
	h.journal = &contractJournal{
		inner: backend, holder: &contractStateHolder{},
		pause: map[string]chan struct{}{}, failed: map[string]error{}, hold: map[string]bool{},
	}

	manager, err := NewWorkspaceManager(h.wsRoot)
	if err != nil {
		t.Fatalf("workspace manager: %v", err)
	}
	sandbox, err := NewSandboxManager(domain.SandboxModeWorkspaceWrite, root, nil, nil)
	if err != nil {
		t.Fatalf("sandbox manager: %v", err)
	}
	files := NewEinoFilesystemBackend(manager, sandbox)
	files.SetFileVersionRecorder(NewFileVersionRecorder(backend, nil))
	commands := NewCommandBackend(manager, sandbox, nil, 0)

	allTools := []tools.Tool{
		tools.NewWriteFile(files),
		tools.NewReadFile(files),
		tools.NewBash(commands),
	}
	allTools = append(allTools, opts.extraTools...)
	names := make([]string, 0, len(allTools))
	for _, tl := range allTools {
		names = append(names, tl.Spec().Name)
	}
	ts, err := tools.NewRegistry(allTools...).Resolve(names)
	if err != nil {
		t.Fatalf("resolve tools: %v", err)
	}

	h.model = &contractCaptureModel{script: script, failures: opts.failures}
	cfg := EngineConfig{
		StreamBuffer:         8,
		MaxEventPayloadBytes: 64 << 10,
		MaxContextBytes:      1 << 20,
		MaxToolResultBytes:   64 << 10,
		AutoApproveTools:     opts.autoApprove,
	}
	if opts.checkpoints {
		store, err := NewVersionedCheckpointStore(backend.Blobs(), "acceptance-engine")
		if err != nil {
			t.Fatalf("checkpoint store: %v", err)
		}
		cfg.Checkpoints = store
	}
	eng, err := NewEngine(ctx, h.model, ts, cfg)
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}
	h.svc = NewService(eng, "acceptance", "acceptance-v0", ServiceDeps{
		Journal: h.journal, Runs: backend, Messages: backend, Notes: backend,
		Approvals: backend, Sessions: backend, Sink: newTestSink(), Truncations: backend,
		ApprovalExpiration: 5 * time.Minute,
	})
	return h
}

// openSession creates the session row and pins its sandbox/approval
// policy before Run.
func (h *acceptanceHarness) openSession(t *testing.T, id domain.SessionID, policy domain.ApprovalPolicy) {
	t.Helper()
	ctx := context.Background()
	if err := h.backend.CreateSession(ctx, domain.Session{ID: id, Title: "nd4 " + string(id), CreatedAt: time.Now().UnixMilli()}); err != nil {
		t.Fatalf("create session: %v", err)
	}
	if err := h.backend.UpdateSandboxPolicy(ctx, id, domain.SandboxModeWorkspaceWrite, policy); err != nil {
		t.Fatalf("set session policy: %v", err)
	}
}

func (h *acceptanceHarness) waitFor(t *testing.T, msg string, cond func() bool) {
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

func acceptanceFinished(t *testing.T, events []domain.RunEvent, callID string) payloadToolFinished {
	t.Helper()
	for _, ev := range events {
		if ev.Type != domain.EventToolFinished {
			continue
		}
		var p payloadToolFinished
		mustUnmarshal(t, ev.Payload, &p)
		if p.ToolCallID == callID {
			return p
		}
	}
	t.Fatalf("no tool.finished for %s", callID)
	return payloadToolFinished{}
}

func acceptanceCall(name, id, args string) schema.ToolCall {
	return schema.ToolCall{ID: id, Function: schema.FunctionCall{Name: name, Arguments: args}}
}

// noNudges asserts the whole run stayed nudge-free end to end.
func noNudges(t *testing.T, h *acceptanceHarness) {
	t.Helper()
	if n := len(nudgeEvents(t, h.journal)); n != 0 {
		t.Fatalf("tool.nudge events = %d, want 0", n)
	}
	for i := 0; i < h.model.count(); i++ {
		if got := nudgeMessages(h.model.entry(i).input); len(got) != 0 {
			t.Fatalf("model input %d carries a runtime_nudge message", i)
		}
	}
}

// ---------------------------------------------------------------------------
// A: read a missing file -> typed error result -> model reads an existing
// file -> Run completes.
// ---------------------------------------------------------------------------
func TestNudgeAcceptanceMissingFileCorrection(t *testing.T) {
	script := []*schema.Message{
		schema.AssistantMessage("", []schema.ToolCall{
			acceptanceCall(tools.WriteFileName, "call-a0", `{"path":"ok.txt","content":"hello file"}`),
		}),
		schema.AssistantMessage("", []schema.ToolCall{
			acceptanceCall(tools.ReadFileName, "call-a1", `{"path":"missing.txt"}`),
		}),
		schema.AssistantMessage("", []schema.ToolCall{
			acceptanceCall(tools.ReadFileName, "call-a2", `{"path":"ok.txt"}`),
		}),
		schema.AssistantMessage("done", nil),
	}
	h := newAcceptanceHarness(t, script, acceptanceOpts{
		policy:      domain.ApprovalPolicyAuto,
		autoApprove: []string{tools.WriteFileName},
	})
	h.openSession(t, "sess-acc-a", domain.ApprovalPolicyAuto)
	runID, err := h.svc.Run(context.Background(), "sess-acc-a", "read missing.txt")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	waitForWalkthroughStatus(t, h.backend, runID, domain.RunCompleted)

	events := replayAll(t, h.backend, runID)
	missing := acceptanceFinished(t, events, "call-a1")
	if missing.Error == "" || missing.Reason != toolFailureReasonNotFound {
		t.Fatalf("missing-file result not typed: %+v", missing)
	}
	ok := acceptanceFinished(t, events, "call-a2")
	if ok.Error != "" || !strings.Contains(ok.Result, "hello file") {
		t.Fatalf("corrected read result = %+v", ok)
	}
	if got := countTerminal(events); got != 1 {
		t.Fatalf("terminal events = %d, want 1", got)
	}
	// A single occurrence never schedules a reminder.
	noNudges(t, h)
}

// ---------------------------------------------------------------------------
// B: a command exits 1 -> the error stays inspectable -> the model changes
// the command -> completion.
// ---------------------------------------------------------------------------
func TestNudgeAcceptanceCommandRetry(t *testing.T) {
	script := []*schema.Message{
		schema.AssistantMessage("", []schema.ToolCall{
			acceptanceCall(tools.BashName, "call-b1", `{"command":"exit 1"}`),
		}),
		schema.AssistantMessage("", []schema.ToolCall{
			acceptanceCall(tools.BashName, "call-b2", `{"command":"echo fixed"}`),
		}),
		schema.AssistantMessage("done", nil),
	}
	h := newAcceptanceHarness(t, script, acceptanceOpts{
		policy:      domain.ApprovalPolicyAuto,
		autoApprove: []string{tools.BashName},
	})
	h.openSession(t, "sess-acc-b", domain.ApprovalPolicyAuto)
	runID, err := h.svc.Run(context.Background(), "sess-acc-b", "run the probe")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	waitForWalkthroughStatus(t, h.backend, runID, domain.RunCompleted)

	events := replayAll(t, h.backend, runID)
	failed := acceptanceFinished(t, events, "call-b1")
	if failed.Error == "" || failed.Reason != toolFailureReasonCommandFailed ||
		failed.Effects != toolEffectsUnknown || !strings.Contains(failed.Error, "code 1") {
		t.Fatalf("exit-1 result not inspectable: %+v", failed)
	}
	fixed := acceptanceFinished(t, events, "call-b2")
	if fixed.Error != "" || !strings.Contains(fixed.Result, "fixed") {
		t.Fatalf("corrected command result = %+v", fixed)
	}
	noNudges(t, h)
}

// ---------------------------------------------------------------------------
// C: an MCP tool-result error (IsError) travels Session -> MCPHost ->
// ToolWorld -> ToolHost -> governed tool -> enhanced adapter as a typed
// remote_tool_error, then the model corrects within the same Run.
// ---------------------------------------------------------------------------

// acceptanceMCPSession is the SessionFactory/Session stand-in: one remote
// tool whose CallTool reports IsError — the same record ToolWorld turns
// into *mcphost.ToolExecutionError.
type acceptanceMCPSession struct{ isError bool }

func (s *acceptanceMCPSession) DiscoverTools(context.Context) ([]mcphost.RemoteTool, error) {
	return []mcphost.RemoteTool{{Name: "flaky", Description: "remote probe"}}, nil
}
func (s *acceptanceMCPSession) CallTool(context.Context, string, json.RawMessage) (mcphost.ToolResult, error) {
	if s.isError {
		return mcphost.ToolResult{Text: "remote exploded", IsError: true}, nil
	}
	return mcphost.ToolResult{Text: "remote ok"}, nil
}
func (s *acceptanceMCPSession) ListResources(context.Context) ([]mcphost.RemoteResource, error) {
	return nil, nil
}
func (s *acceptanceMCPSession) ReadResource(context.Context, string) (mcphost.ResourceContent, error) {
	return mcphost.ResourceContent{}, nil
}
func (s *acceptanceMCPSession) Close() error { return nil }

type acceptanceMCPFactory struct{ session *acceptanceMCPSession }

func (f *acceptanceMCPFactory) Open(context.Context, mcphost.InstanceConfig) (mcphost.Session, error) {
	return f.session, nil
}

// acceptanceWorldHost satisfies toolworld.Host for the in-process binding.
type acceptanceWorldHost struct{ dir string }

func (h acceptanceWorldHost) ModuleID() string  { return "acceptance/mcp" }
func (h acceptanceWorldHost) Workspace() string { return h.dir }
func (h acceptanceWorldHost) OpenRead(string) (io.ReadCloser, error) {
	return nil, errors.New("acceptance host has no file access")
}
func (h acceptanceWorldHost) OpenWrite(string) (io.WriteCloser, error) {
	return nil, errors.New("acceptance host has no file access")
}
func (h acceptanceWorldHost) Spawn(context.Context, toolworldport.SpawnSpec) (toolworldport.Proc, error) {
	return nil, errors.New("acceptance host cannot spawn")
}

// governedMCPTool mirrors internal/app's governedTool: a model-visible
// tools.Tool whose invocation is routed through the sole ToolHost.
type governedMCPTool struct {
	host *toolhost.Host
	id   string
	spec domain.ToolSpec
}

func (t *governedMCPTool) Spec() domain.ToolSpec { return t.spec }
func (t *governedMCPTool) InvokableRun(ctx context.Context, args json.RawMessage) (string, error) {
	result, err := t.host.Invoke(ctx, toolhost.Request{ID: t.id, Args: args})
	return result.Text, err
}

// mountIsErrorWorld builds the production chain for one MCP tool that
// answers every call with IsError: real mcphost.Host + ToolWorld bound
// into a real toolhost.Host, then the governedTool-shaped wrapper.
func mountIsErrorWorld(t *testing.T, dir string) *governedMCPTool {
	t.Helper()
	ctx := context.Background()
	mcpHost, err := mcphost.New(mcphost.Config{
		Factory:   &acceptanceMCPFactory{session: &acceptanceMCPSession{isError: true}},
		Instances: []mcphost.InstanceConfig{{ID: "acc", Command: "fixture"}},
	})
	if err != nil {
		t.Fatalf("mcp host: %v", err)
	}
	t.Cleanup(func() { _ = mcpHost.Close() })
	world := mcphost.NewToolWorld(mcpHost)
	t.Cleanup(func() { _ = world.Close(context.Background()) })

	host, err := toolhost.New(toolhost.Config{
		Worlds: []toolhost.WorldBinding{{
			OwnerID:  "acceptance/mcp",
			Provider: world,
			Host:     acceptanceWorldHost{dir: dir},
		}},
	})
	if err != nil {
		t.Fatalf("toolhost: %v", err)
	}
	if _, err := host.Discover(ctx); err != nil {
		t.Fatalf("discover worlds: %v", err)
	}
	var found *toolhost.Entry
	for _, def := range host.ListVisible() {
		entry, ok := host.Lookup(def.ID)
		if !ok || !entry.Dynamic {
			continue
		}
		found = &entry
	}
	if found == nil {
		t.Fatal("ToolHost did not project the MCP tool")
	}
	def := found.Definition
	return &governedMCPTool{
		host: host,
		id:   def.ID,
		spec: domain.ToolSpec{
			Name: def.ID, Description: def.Description,
			Readonly: false,
			Schema:   append(json.RawMessage(nil), def.Schema...),
		},
	}
}

func TestNudgeAcceptanceRemoteToolIsError(t *testing.T) {
	root := t.TempDir()
	governed := mountIsErrorWorld(t, root)
	callID := "call-c1"
	script := []*schema.Message{
		schema.AssistantMessage("", []schema.ToolCall{
			acceptanceCall(governed.spec.Name, callID, `{}`),
		}),
		schema.AssistantMessage("", []schema.ToolCall{
			acceptanceCall(tools.BashName, "call-c2", `{"command":"echo corrected"}`),
		}),
		schema.AssistantMessage("done", nil),
	}
	h := newAcceptanceHarness(t, script, acceptanceOpts{
		policy:      domain.ApprovalPolicyAuto,
		autoApprove: []string{tools.BashName, governed.spec.Name},
		extraTools:  []tools.Tool{governed},
	})
	h.openSession(t, "sess-acc-c", domain.ApprovalPolicyAuto)
	runID, err := h.svc.Run(context.Background(), "sess-acc-c", "call the remote probe")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	waitForWalkthroughStatus(t, h.backend, runID, domain.RunCompleted)

	events := replayAll(t, h.backend, runID)
	remote := acceptanceFinished(t, events, callID)
	if remote.Reason != toolFailureReasonRemoteTool || remote.Effects != toolEffectsUnknown ||
		remote.Error == "" || !strings.Contains(remote.Error, "remote exploded") {
		t.Fatalf("IsError did not surface as typed remote failure: %+v", remote)
	}
	fixed := acceptanceFinished(t, events, "call-c2")
	if fixed.Error != "" {
		t.Fatalf("correction call failed: %+v", fixed)
	}
	noNudges(t, h)
}

// ---------------------------------------------------------------------------
// D: six identical unsuccessful calls -> reminders at repeats 3 and 5, six
// journaled results, one failed terminal.
// ---------------------------------------------------------------------------
func TestNudgeAcceptanceBoundedLoopStop(t *testing.T) {
	call := func(i int) *schema.Message {
		return schema.AssistantMessage("", []schema.ToolCall{
			acceptanceCall(contractToolName, fmt.Sprintf("call-d%d", i), `{"text":"same"}`),
		})
	}
	script := make([]*schema.Message, 0, 7)
	for i := 1; i <= 6; i++ {
		script = append(script, call(i))
	}
	script = append(script, schema.AssistantMessage("unreachable", nil))
	h := newAcceptanceHarness(t, script, acceptanceOpts{
		policy:     domain.ApprovalPolicyAuto,
		extraTools: []tools.Tool{failingContractTool()},
	})
	h.openSession(t, "sess-acc-d", domain.ApprovalPolicyAuto)
	runID, err := h.svc.Run(context.Background(), "sess-acc-d", "repeat the bad call")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	waitForRunStatus(t, h.backend, runID, domain.RunFailed)

	events := replayAll(t, h.backend, runID)
	nudges := nudgeEvents(t, h.journal)
	if len(nudges) != 2 || nudges[0].RepeatCount != 3 || nudges[1].RepeatCount != 5 {
		t.Fatalf("tool.nudge repeats = %+v, want counts 3 and 5", nudges)
	}
	var finished int
	for _, ev := range events {
		if ev.Type == domain.EventToolFinished {
			finished++
		}
	}
	if finished != 6 {
		t.Fatalf("tool.finished events = %d, want 6", finished)
	}
	if got := countTerminal(events); got != 1 {
		t.Fatalf("terminal events = %d, want 1", got)
	}
	// The two scheduled reminders reached the model's following requests.
	var reminders int
	for i := 0; i < h.model.count(); i++ {
		reminders += len(nudgeMessages(h.model.entry(i).input))
	}
	if reminders != 2 {
		t.Fatalf("runtime_nudge injections = %d, want 2", reminders)
	}
}

// ---------------------------------------------------------------------------
// E: a session-pinned deny ("never") refuses three identical write_file
// calls -> refusal reminder, zero filesystem mutation.
// ---------------------------------------------------------------------------
func TestNudgeAcceptanceRefusedWriteNoMutation(t *testing.T) {
	call := func(i int) *schema.Message {
		return schema.AssistantMessage("", []schema.ToolCall{
			acceptanceCall(tools.WriteFileName, fmt.Sprintf("call-e%d", i), `{"path":"denied.txt","content":"nope"}`),
		})
	}
	script := []*schema.Message{call(1), call(2), call(3), schema.AssistantMessage("done", nil)}
	h := newAcceptanceHarness(t, script, acceptanceOpts{policy: domain.ApprovalPolicyNever})
	h.openSession(t, "sess-acc-e", domain.ApprovalPolicyNever)
	runID, err := h.svc.Run(context.Background(), "sess-acc-e", "write denied.txt")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	waitForRunStatus(t, h.backend, runID, domain.RunCompleted)

	events := replayAll(t, h.backend, runID)
	for i := 1; i <= 3; i++ {
		fin := acceptanceFinished(t, events, fmt.Sprintf("call-e%d", i))
		if fin.Reason != toolFailureReasonPolicyDenied && fin.Reason != toolFailureReasonUserDenied {
			t.Fatalf("denied write %d not marked refused: %+v", i, fin)
		}
		if fin.Effects != toolEffectsNotExecuted {
			t.Fatalf("denied write %d effects = %q, want not_executed", i, fin.Effects)
		}
	}
	nudges := nudgeEvents(t, h.journal)
	if len(nudges) != 1 || nudges[0].RepeatCount != 3 {
		t.Fatalf("tool.nudge events = %+v, want one at count 3", nudges)
	}
	reminders := nudgeMessages(h.model.entry(3).input)
	if len(reminders) != 1 || !strings.Contains(reminders[0].Content, "Do not bypass") {
		t.Fatalf("post-denial input lacks the refusal reminder: %v", nudgeMessages(h.model.entry(3).input))
	}
	// The refusal was honored: no workspace file exists.
	if entries, _ := filepath.Glob(filepath.Join(h.wsRoot, "*", "denied.txt")); len(entries) != 0 {
		t.Fatalf("denied write mutated the filesystem: %v", entries)
	}
}

// ---------------------------------------------------------------------------
// F: a Journal failure while the boundary waits aborts the run — no next
// model request, no deadlock; the same holds for run cancellation.
// ---------------------------------------------------------------------------
func TestNudgeAcceptanceJournalFailureWhileWaiting(t *testing.T) {
	script := []*schema.Message{
		schema.AssistantMessage("", []schema.ToolCall{
			acceptanceCall(contractToolName, "call-f1", `{"text":"x"}`),
		}),
		schema.AssistantMessage("unreachable", nil),
	}
	h := newAcceptanceHarness(t, script, acceptanceOpts{
		policy:     domain.ApprovalPolicyAuto,
		extraTools: []tools.Tool{failingContractTool()},
	})
	h.openSession(t, "sess-acc-f", domain.ApprovalPolicyAuto)
	h.journal.failed["tool.finished:call-f1"] = errors.New("disk full")

	runID, err := h.svc.Run(context.Background(), "sess-acc-f", "one failing call")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	waitForRunStatus(t, h.backend, runID, domain.RunFailed)
	// The waiting boundary was released by the abort, never by a second
	// model request.
	if got := h.model.count(); got != 1 {
		t.Fatalf("inner model calls = %d, want 1", got)
	}
}

func TestNudgeAcceptanceCancelWhileWaiting(t *testing.T) {
	script := []*schema.Message{
		schema.AssistantMessage("", []schema.ToolCall{
			acceptanceCall(contractToolName, "call-fx1", `{"text":"x"}`),
		}),
		schema.AssistantMessage("unreachable", nil),
	}
	h := newAcceptanceHarness(t, script, acceptanceOpts{
		policy:     domain.ApprovalPolicyAuto,
		extraTools: []tools.Tool{failingContractTool()},
	})
	h.openSession(t, "sess-acc-fx", domain.ApprovalPolicyAuto)
	gate := make(chan struct{})
	h.journal.mu.Lock()
	h.journal.pause["tool.finished:call-fx1"] = gate
	h.journal.mu.Unlock()

	runID, err := h.svc.Run(context.Background(), "sess-acc-fx", "one failing call")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	h.waitFor(t, "paused append", func() bool { return h.journal.pausingCount() > 0 })
	time.Sleep(150 * time.Millisecond)
	if got := h.model.count(); got != 1 {
		t.Fatalf("model advanced while a result was undurable: %d calls", got)
	}
	if !h.svc.Cancel(runID) {
		t.Fatal("Cancel did not find the active run")
	}
	close(gate)
	// The run must settle, not deadlock.
	h.waitFor(t, "cancelled run terminal", func() bool {
		r, err := h.backend.GetRun(context.Background(), runID)
		return err == nil && (r.Status == domain.RunCancelled || r.Status == domain.RunFailed)
	})
	if got := h.model.count(); got != 1 {
		t.Fatalf("a model request was issued after cancel: %d calls", got)
	}
}

// ---------------------------------------------------------------------------
// G: a model batch seals in request order; the settled boundary emits
// exactly one notice naming the earliest request position among threshold hits.
// ---------------------------------------------------------------------------
func TestNudgeAcceptanceRequestOrder(t *testing.T) {
	failing := newContractTool(func(context.Context, json.RawMessage) (string, error) {
		return "", &tools.ArgError{Field: "text", Reason: "must be a string"}
	})
	batch := func(a, b string) *schema.Message {
		return schema.AssistantMessage("", []schema.ToolCall{
			acceptanceCall(contractToolName, a, `{"text":"same"}`),
			acceptanceCall(contractToolName, b, `{"text":"same"}`),
		})
	}
	script := []*schema.Message{
		batch("call-g1", "call-g2"),
		batch("call-g3", "call-g4"),
		schema.AssistantMessage("done", nil),
	}
	h := newAcceptanceHarness(t, script, acceptanceOpts{
		policy:     domain.ApprovalPolicyAuto,
		extraTools: []tools.Tool{failing},
	})
	h.openSession(t, "sess-acc-g", domain.ApprovalPolicyAuto)
	runID, err := h.svc.Run(context.Background(), "sess-acc-g", "parallel failures")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	waitForRunStatus(t, h.backend, runID, domain.RunCompleted)

	if idx1, idx2 := h.journal.indexOf(domain.EventToolFinished, "call-g1"),
		h.journal.indexOf(domain.EventToolFinished, "call-g2"); idx1 < 0 || idx2 < 0 || idx1 > idx2 {
		t.Fatalf("tool results were not sealed in model request order (g1=%d, g2=%d)", idx1, idx2)
	}
	nudges := nudgeEvents(t, h.journal)
	if len(nudges) != 1 {
		t.Fatalf("tool.nudge events = %d, want 1 per settled boundary", len(nudges))
	}
	// Batch 2 records in request order: call-g3 hits count 3, call-g4 4
	// (skipped).
	if nudges[0].ToolCallID != "call-g3" || nudges[0].RepeatCount != 3 {
		t.Fatalf("nudge named %+v, want call-g3 at count 3", nudges[0])
	}
	// The next request carries both tool results paired with their calls,
	// then the single tagged reminder.
	in := h.model.entry(2).input
	if tagged := nudgeMessages(in); len(tagged) != 1 || tagged[0] != in[len(in)-1] {
		t.Fatal("post-batch-2 input lacks the trailing tagged reminder")
	}
	if ids := trailingToolCallIDs(in[:len(in)-1]); len(ids) != 2 ||
		ids[0] != "call-g3" || ids[1] != "call-g4" {
		t.Fatalf("tool results ahead of reminder = %v, want [call-g3 call-g4]", ids)
	}
}

// ---------------------------------------------------------------------------
// H: an approval resume leg sees a fresh detector — the decided call's
// replayed result forms an implicit singleton batch, no stale notice, and
// a later triple-failure still schedules exactly once.
// ---------------------------------------------------------------------------
func TestNudgeAcceptanceResumeNoStaleNotice(t *testing.T) {
	call := func(i int) *schema.Message {
		return schema.AssistantMessage("", []schema.ToolCall{
			acceptanceCall(contractToolName, fmt.Sprintf("call-h%d", i), `{"text":"same"}`),
		})
	}
	script := []*schema.Message{
		schema.AssistantMessage("", []schema.ToolCall{
			acceptanceCall(tools.WriteFileName, "call-h0", `{"path":"gate.txt","content":"g"}`),
		}),
		call(1), call(2), call(3),
		schema.AssistantMessage("done", nil),
	}
	h := newAcceptanceHarness(t, script, acceptanceOpts{
		policy:      domain.ApprovalPolicyAsk,
		checkpoints: true,
		extraTools:  []tools.Tool{failingContractTool()},
	})
	h.openSession(t, "sess-acc-h", domain.ApprovalPolicyAsk)
	ctx := context.Background()
	runID, err := h.svc.Run(ctx, "sess-acc-h", "write gate.txt then probe")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	approval := waitForPendingApproval(t, h.backend, runID)
	if err := h.svc.DecideApproval(ctx, approval.ID, domain.ApprovalApproved); err != nil {
		t.Fatalf("decide: %v", err)
	}
	waitForRunStatus(t, h.backend, runID, domain.RunCompleted)

	// Exactly one nudge on the fresh resume leg: the approved write's
	// replayed result must not resurrect a stale notice or double-emit.
	nudges := nudgeEvents(t, h.journal)
	if len(nudges) != 1 || nudges[0].RepeatCount != 3 || nudges[0].ToolCallID != "call-h3" {
		t.Fatalf("tool.nudge events = %+v, want one at count 3", nudges)
	}
	var injected int
	for i := 0; i < h.model.count(); i++ {
		injected += len(nudgeMessages(h.model.entry(i).input))
	}
	if injected != 1 {
		t.Fatalf("runtime_nudge injections = %d, want 1", injected)
	}
}

// ---------------------------------------------------------------------------
// I: a command mutates state then exits nonzero — marked command_failed
// with effects unknown, never automatically replayed.
// ---------------------------------------------------------------------------
func TestNudgeAcceptancePartialEffectNoReplay(t *testing.T) {
	script := []*schema.Message{
		schema.AssistantMessage("", []schema.ToolCall{
			acceptanceCall(tools.BashName, "call-i1", `{"command":"echo marker > partial.marker && exit 2"}`),
		}),
		schema.AssistantMessage("done", nil),
	}
	h := newAcceptanceHarness(t, script, acceptanceOpts{
		policy:      domain.ApprovalPolicyAuto,
		autoApprove: []string{tools.BashName},
	})
	h.openSession(t, "sess-acc-i", domain.ApprovalPolicyAuto)
	runID, err := h.svc.Run(context.Background(), "sess-acc-i", "run the partial command")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	waitForWalkthroughStatus(t, h.backend, runID, domain.RunCompleted)

	events := replayAll(t, h.backend, runID)
	fin := acceptanceFinished(t, events, "call-i1")
	if fin.Reason != toolFailureReasonCommandFailed || fin.Effects != toolEffectsUnknown {
		t.Fatalf("partial-effect command not marked uncertain: %+v", fin)
	}
	// The side effect landed (no replay semantics revert it) and the
	// runtime issued no automatic re-dispatch: the script ends with the
	// model observing the error, never retrying on its own.
	if entries, _ := filepath.Glob(filepath.Join(h.wsRoot, "*", "partial.marker")); len(entries) != 1 {
		t.Fatalf("partial effect missing (no replay to recreate it): %v", entries)
	}
	var dispatchCount int
	for _, ev := range events {
		if ev.Type != domain.EventToolRequested {
			continue
		}
		var p struct {
			ToolCallID string `json:"tool_call_id"`
		}
		mustUnmarshal(t, ev.Payload, &p)
		if p.ToolCallID == "call-i1" {
			dispatchCount++
		}
	}
	if dispatchCount != 1 {
		t.Fatalf("call-i1 dispatched %d times, want exactly 1 (no auto replay)", dispatchCount)
	}
	noNudges(t, h)
}

// ---------------------------------------------------------------------------
// J: new and legacy journal payloads both decode and project — the typed
// fields added by ND-1/2/3 stay optional on older records, and every event
// this run journaled still round-trips through its payload type.
// ---------------------------------------------------------------------------
func TestNudgeAcceptanceLegacyAndNewPayloads(t *testing.T) {
	// Legacy tool.finished shape (pre-ND-1): no outcome/reason/effects.
	legacy := json.RawMessage(`{"tool_call_id":"call-old","tool_name":"read_file","result":"x"}`)
	var legacyFin payloadToolFinished
	mustUnmarshal(t, legacy, &legacyFin)
	if legacyFin.ToolCallID != "call-old" || legacyFin.Outcome != "" || legacyFin.Reason != "" || legacyFin.Effects != "" {
		t.Fatalf("legacy finished decode = %+v", legacyFin)
	}
	// A legacy payload without tool.nudge's typed fields must not decode
	// as one; and the five scheduling fields of a new record round-trip.
	var legacyNudge payloadToolNudge
	mustUnmarshal(t, legacy, &legacyNudge)
	if legacyNudge.ToolCallID != "call-old" || legacyNudge.RepeatCount != 0 || legacyNudge.TemplateVersion != "" {
		t.Fatalf("legacy nudge decode = %+v", legacyNudge)
	}

	script := []*schema.Message{
		schema.AssistantMessage("", []schema.ToolCall{
			acceptanceCall(contractToolName, "call-j1", `{"text":"x"}`),
		}),
		schema.AssistantMessage("", []schema.ToolCall{
			acceptanceCall(contractToolName, "call-j2", `{"text":"x"}`),
		}),
		schema.AssistantMessage("", []schema.ToolCall{
			acceptanceCall(contractToolName, "call-j3", `{"text":"x"}`),
		}),
		schema.AssistantMessage("done", nil),
	}
	h := newAcceptanceHarness(t, script, acceptanceOpts{
		policy:     domain.ApprovalPolicyAuto,
		extraTools: []tools.Tool{failingContractTool()},
	})
	h.openSession(t, "sess-acc-j", domain.ApprovalPolicyAuto)
	runID, err := h.svc.Run(context.Background(), "sess-acc-j", "triple fail then done")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	waitForRunStatus(t, h.backend, runID, domain.RunCompleted)

	// Every journaled event payload decodes into its typed payload — a
	// readable projection, not opaque bytes.
	decoders := map[domain.EventType]func(json.RawMessage) error{
		domain.EventToolRequested: func(p json.RawMessage) error {
			var v payloadToolRequested
			return json.Unmarshal(p, &v)
		},
		domain.EventToolFinished: func(p json.RawMessage) error {
			var v payloadToolFinished
			return json.Unmarshal(p, &v)
		},
		domain.EventToolNudge: func(p json.RawMessage) error {
			var v payloadToolNudge
			return json.Unmarshal(p, &v)
		},
	}
	for _, ev := range replayAll(t, h.backend, runID) {
		decode, ok := decoders[ev.Type]
		if !ok {
			continue
		}
		if err := decode(ev.Payload); err != nil {
			t.Fatalf("event %s payload does not decode: %v", ev.Type, err)
		}
	}
	if got := len(nudgeEvents(t, h.journal)); got != 1 {
		t.Fatalf("tool.nudge events = %d, want 1", got)
	}
}
