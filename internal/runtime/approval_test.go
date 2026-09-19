package runtime

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage/sqlite"
	"agent-vivy/internal/tools"
)

// newApprovalService wires the full C6 stack over sqlite: checkpoint
// bridge, write_note gate, approval store, and the scripted model that
// requests the effectful call on turn one.
func newApprovalService(t *testing.T, expiration time.Duration) (*Service, *sqlite.Backend, *testSink) {
	return newApprovalServiceWithModel(t, expiration, NewApprovalFlowModel())
}

func newApprovalServiceWithModel(t *testing.T, expiration time.Duration, chatModel model.ToolCallingChatModel) (*Service, *sqlite.Backend, *testSink) {
	t.Helper()
	ctx := context.Background()

	backend, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "approvals.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = backend.Close() })

	ts, err := tools.Builtin(backend).Resolve([]string{tools.EchoInfoName, tools.WriteNoteName})
	if err != nil {
		t.Fatalf("resolve tools: %v", err)
	}
	checkpoints, err := NewVersionedCheckpointStore(backend.Blobs(), "test-engine")
	if err != nil {
		t.Fatalf("checkpoint store: %v", err)
	}
	eng, err := NewEngine(ctx, chatModel, ts, EngineConfig{
		StreamBuffer: 8, MaxEventPayloadBytes: 64 << 10, Checkpoints: checkpoints,
	})
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}
	sink := newTestSink()
	svc := NewService(eng, "scripted", "scripted-v0", ServiceDeps{
		Journal: backend, Runs: backend, Messages: backend, Notes: backend, Approvals: backend,
		Questions:          backend,
		ApprovalExpiration: expiration, Sink: sink,
	})
	return svc, backend, sink
}

type gatedApprovalResumeModel struct {
	mu      sync.Mutex
	calls   int
	blocked chan struct{}
	release chan struct{}
}

func (m *gatedApprovalResumeModel) Generate(context.Context, []*schema.Message, ...model.Option) (*schema.Message, error) {
	return nil, errors.New("gated approval model requires streaming")
}

func (m *gatedApprovalResumeModel) Stream(context.Context, []*schema.Message, ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	m.mu.Lock()
	call := m.calls
	m.calls++
	m.mu.Unlock()
	reader, writer := schema.Pipe[*schema.Message](1)
	switch call {
	case 0:
		go func() {
			defer writer.Close()
			writer.Send(schema.AssistantMessage("", []schema.ToolCall{{
				ID: ApprovalFlowCallID, Function: schema.FunctionCall{Name: tools.WriteNoteName, Arguments: `{"content":"buy milk"}`},
			}}), nil)
		}()
	case 1:
		go func() {
			defer writer.Close()
			writer.Send(schema.AssistantMessage("恢", nil), nil)
			close(m.blocked)
			<-m.release
			writer.Send(schema.AssistantMessage("复完成", nil), nil)
		}()
	default:
		writer.Close()
		return nil, fmt.Errorf("unexpected model stream call %d", call)
	}
	return reader, nil
}

func (m *gatedApprovalResumeModel) WithTools([]*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	return m, nil
}

// waitForPendingApproval polls until the run's approval row exists.
func waitForPendingApproval(t *testing.T, backend *sqlite.Backend, runID domain.RunID) domain.Approval {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		rows, err := backend.ListPendingApprovals(context.Background())
		if err != nil {
			t.Fatalf("list pending approvals: %v", err)
		}
		for _, a := range rows {
			if a.RunID == runID {
				return a
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("run %s never produced a pending approval", runID)
	return domain.Approval{}
}

func indexOfType(events []domain.RunEvent, want domain.EventType) int {
	for i, ev := range events {
		if ev.Type == want {
			return i
		}
	}
	return -1
}

func TestServiceApprovalApproveFlow(t *testing.T) {
	svc, backend, sink := newApprovalService(t, 5*time.Minute)
	ctx := context.Background()

	runID, err := svc.Run(ctx, "sess-1", "note that I need milk")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	approval := waitForPendingApproval(t, backend, runID)

	// The run row stays active while suspended: no terminal yet.
	r, err := backend.GetRun(ctx, runID)
	if err != nil {
		t.Fatalf("get run: %v", err)
	}
	if r.Status != domain.RunActive {
		t.Fatalf("suspended run status = %s, want active", r.Status)
	}

	// Approval row carries the pending decision and the resume target.
	if approval.Decision != domain.ApprovalPending {
		t.Fatalf("approval decision = %q, want pending", approval.Decision)
	}
	if approval.ResumeTarget == "" {
		t.Fatal("approval must persist the resume target")
	}
	if approval.ToolCallID != ApprovalFlowCallID {
		t.Fatalf("approval tool_call_id = %q, want %q", approval.ToolCallID, ApprovalFlowCallID)
	}
	if approval.ExpiresAt <= time.Now().UnixMilli() {
		t.Fatal("approval must expire in the future")
	}

	// The approval row is deliberately persisted before its journal event.
	// Wait for the durable suspension barrier before asserting event order.
	waitForApprovalEvent(t, backend, runID)

	// Journal so far: run.started, tool.requested, then the single
	// tool.approval_required commit (D-029 order).
	pre := replayAll(t, backend, runID)
	if i := indexOfType(pre, domain.EventToolApprovalRequired); i != len(pre)-1 {
		t.Fatalf("tool.approval_required at index %d of %d, want last", i, len(pre))
	}
	req := pre[indexOfType(pre, domain.EventToolRequested)]
	if req.Seq >= pre[len(pre)-1].Seq {
		t.Fatal("tool.requested must precede tool.approval_required")
	}
	var ap payloadToolApprovalRequired
	mustUnmarshal(t, pre[len(pre)-1].Payload, &ap)
	if ap.ApprovalID != approval.ID || ap.ToolCallID != ApprovalFlowCallID || ap.ToolName != tools.WriteNoteName {
		t.Fatalf("approval payload = %+v", ap)
	}
	if ap.Args["content"] != "buy milk" {
		t.Fatalf("approval payload args = %+v", ap.Args)
	}
	if ap.ExpiresAt != approval.ExpiresAt {
		t.Fatalf("payload expires_at = %d, want %d", ap.ExpiresAt, approval.ExpiresAt)
	}
	selectedWrite := false
	for _, name := range ap.SelectedTools {
		if name == tools.WriteNoteName {
			selectedWrite = true
			break
		}
	}
	if !selectedWrite {
		t.Fatalf("approval payload selected_tools = %v, want %q", ap.SelectedTools, tools.WriteNoteName)
	}
	// The sink saw the non-terminal approval event for live UI fan-out.
	sawApproval := false
	for _, ev := range sink.snapshot() {
		if ev.Type == domain.EventToolApprovalRequired {
			sawApproval = true
		}
	}
	if !sawApproval {
		t.Fatal("tool.approval_required was not published to the sink")
	}

	// Approve: the resume executes the tool and closes the run.
	if err := svc.DecideApproval(ctx, approval.ID, domain.ApprovalApproved); err != nil {
		t.Fatalf("decide: %v", err)
	}
	waitForRunStatus(t, backend, runID, domain.RunCompleted)

	events := replayAll(t, backend, runID)
	ai := indexOfType(events, domain.EventToolApprovalRequired)
	var types []string
	for _, ev := range events[ai+1:] {
		types = append(types, string(ev.Type))
	}
	prefix := []domain.EventType{
		domain.EventToolApprovalDecided,
		domain.EventPolicyEvaluated,
		domain.EventToolStarted, domain.EventToolFinished,
	}
	if len(types) < len(prefix)+3 {
		t.Fatalf("post-approval events = %v", types)
	}
	for i, w := range prefix {
		if domain.EventType(types[i]) != w {
			t.Fatalf("post-approval event %d = %s, want %s (%v)", i, types[i], w, types)
		}
	}
	for i := len(prefix); i < len(types)-2; i++ {
		if domain.EventType(types[i]) != domain.EventModelDelta {
			t.Fatalf("post-approval body event %d = %s, want model.delta (%v)", i, types[i], types)
		}
	}
	if domain.EventType(types[len(types)-2]) != domain.EventModelCompleted || domain.EventType(types[len(types)-1]) != domain.EventRunCompleted {
		t.Fatalf("post-approval terminal boundary = %v", types)
	}
	var fin payloadToolFinished
	mustUnmarshal(t, events[ai+4].Payload, &fin)
	if fin.ToolCallID != ApprovalFlowCallID {
		t.Fatalf("tool.finished call id = %q, want %q", fin.ToolCallID, ApprovalFlowCallID)
	}
	if !strings.Contains(fin.Result, "saved (1 total)") || !strings.Contains(fin.Result, "note_") {
		t.Fatalf("approved tool must execute: result = %q", fin.Result)
	}
	if n := countTerminal(events); n != 1 {
		t.Fatalf("terminal events = %d, want exactly 1", n)
	}

	// The assistant reply reached the message log.
	msgs, err := backend.ListMessages(ctx, "sess-1")
	if err != nil {
		t.Fatalf("list messages: %v", err)
	}
	last := msgs[len(msgs)-1]
	if last.Role != domain.RoleAssistant || last.Content != "Done: the note has been handled." {
		t.Fatalf("assistant message = %+v", last)
	}
}

// A run that resumed for one approved tool must still dispatch every other
// tool call: the human approval binds the decided call, not the rest of the
// run. Regression for run_9b5ae7e437a91008, where the run-scoped
// approved-arguments hash made the next read-only call report "approved tool
// checkpoint state is unavailable", failed the run, and stamped the
// already-approved approval stale.
func TestServiceResumedRunDispatchesToolsAfterApproval(t *testing.T) {
	const (
		echoCallID = "call-echo-after-approval"
		echoText   = "resumed turn still dispatches tools"
	)
	model := NewScriptedModel(
		schema.AssistantMessage("", []schema.ToolCall{{
			ID: ApprovalFlowCallID, Function: schema.FunctionCall{Name: tools.WriteNoteName, Arguments: `{"content":"buy milk"}`},
		}}),
		// The resumed segment asks for a read-only tool that carries no
		// interrupt state of its own.
		schema.AssistantMessage("", []schema.ToolCall{{
			ID: echoCallID, Function: schema.FunctionCall{Name: tools.EchoInfoName, Arguments: `{"text":"` + echoText + `"}`},
		}}),
		schema.AssistantMessage("Done after approval.", nil),
	)
	svc, backend, _ := newApprovalServiceWithModel(t, 5*time.Minute, model)
	ctx := context.Background()

	runID, err := svc.Run(ctx, "sess-after-approval", "note that I need milk, then echo")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	approval := waitForPendingApproval(t, backend, runID)
	if err := svc.DecideApproval(ctx, approval.ID, domain.ApprovalApproved); err != nil {
		t.Fatalf("decide: %v", err)
	}
	waitForRunStatus(t, backend, runID, domain.RunCompleted)

	events := replayAll(t, backend, runID)
	if i := indexOfType(events, domain.EventToolProposalStale); i >= 0 {
		t.Fatalf("a later call marked the approval stale at event %d: %s", i, events[i].Payload)
	}
	sawEcho := false
	for _, ev := range events {
		if ev.Type != domain.EventToolFinished {
			continue
		}
		var fin payloadToolFinished
		mustUnmarshal(t, ev.Payload, &fin)
		if fin.ToolCallID == echoCallID {
			sawEcho = true
			if !strings.Contains(fin.Result, echoText) {
				t.Fatalf("echo result = %q", fin.Result)
			}
		}
	}
	if !sawEcho {
		t.Fatal("the read-only call issued after the approved call never finished")
	}
	if n := countTerminal(events); n != 1 {
		t.Fatalf("terminal events = %d, want exactly 1", n)
	}
	stored, err := backend.GetApproval(ctx, approval.ID)
	if err != nil {
		t.Fatalf("get approval: %v", err)
	}
	if stored.Decision != domain.ApprovalApproved {
		t.Fatalf("approval decision = %q, want approved (a later call must not restamp it)", stored.Decision)
	}
}

// waitUntilPast blocks until the given unix-milli deadline has passed, so a
// sweeper assertion never depends on scheduler timing.
func waitUntilPast(t *testing.T, unixMilli int64) {
	t.Helper()
	if wait := time.Until(time.UnixMilli(unixMilli)); wait > 0 {
		time.Sleep(wait + 20*time.Millisecond)
	}
}

// Timed auto-approval: a smart-preset (workspace-write + ask) approval that
// nobody answers is approved on the user's behalf when the sweep reaches its
// deadline, and the decision is recorded against the system actor.
func TestServiceSweepAutoApprovesSmartApprovalOnTimeout(t *testing.T) {
	svc, backend, _ := newApprovalService(t, 50*time.Millisecond)
	svc.SetApprovalSettleTimeout(50 * time.Millisecond)
	ctx := context.Background()

	runID, err := svc.Run(ctx, "sess-auto-1", "note that I need milk")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	approval := waitForPendingApproval(t, backend, runID)
	if approval.SandboxMode != string(domain.SandboxModeWorkspaceWrite) || approval.ApprovalPolicy != string(domain.ApprovalPolicyAsk) {
		t.Fatalf("approval preset = %s/%s, want workspace_write/ask", approval.SandboxMode, approval.ApprovalPolicy)
	}
	// The row must be durable before the sweep settles it (D-029).
	waitForApprovalEvent(t, backend, runID)
	waitUntilPast(t, approval.ExpiresAt)

	if err := svc.SweepExpired(ctx); err != nil {
		t.Fatalf("sweep: %v", err)
	}
	waitForRunStatus(t, backend, runID, domain.RunCompleted)

	stored, err := backend.GetApproval(ctx, approval.ID)
	if err != nil {
		t.Fatalf("get approval: %v", err)
	}
	if stored.Decision != domain.ApprovalApproved || stored.Actor != "system" {
		t.Fatalf("approval after sweep = %s by %s (%q)", stored.Decision, stored.Actor, stored.DecisionReason)
	}
	if !strings.Contains(stored.DecisionReason, "auto-approved") {
		t.Fatalf("auto-approval reason = %q", stored.DecisionReason)
	}

	events := replayAll(t, backend, runID)
	if indexOfType(events, domain.EventToolProposalStale) >= 0 {
		t.Fatal("auto-approved run reported a stale proposal")
	}
	sawDecided := false
	for _, ev := range events {
		if ev.Type != domain.EventToolApprovalDecided {
			continue
		}
		var p payloadApprovalDecided
		mustUnmarshal(t, ev.Payload, &p)
		if p.Actor != "system" || !strings.Contains(p.Reason, "auto-approved") {
			t.Fatalf("tool.approval_decided = %+v, want a system auto-approval", p)
		}
		sawDecided = true
	}
	if !sawDecided {
		t.Fatal("missing tool.approval_decided for the timed auto-approval")
	}
	sawTool := false
	for _, ev := range events {
		if ev.Type != domain.EventToolFinished {
			continue
		}
		var fin payloadToolFinished
		mustUnmarshal(t, ev.Payload, &fin)
		if fin.ToolCallID == ApprovalFlowCallID && strings.Contains(fin.Result, "saved (1 total)") {
			sawTool = true
		}
	}
	if !sawTool {
		t.Fatal("the auto-approved tool did not execute")
	}
	if n := countTerminal(events); n != 1 {
		t.Fatalf("terminal events = %d, want exactly 1", n)
	}
}

// The "never auto-approve" setting: the same due approval expires and closes
// its run with the human_timeout cause instead of being approved.
func TestServiceSweepExpiresSmartApprovalWhenAutoApproveDisabled(t *testing.T) {
	svc, backend, _ := newApprovalService(t, 50*time.Millisecond)
	svc.SetApprovalSettleTimeout(0)
	ctx := context.Background()

	runID, err := svc.Run(ctx, "sess-auto-2", "note that I need milk")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	approval := waitForPendingApproval(t, backend, runID)
	waitForApprovalEvent(t, backend, runID)
	waitUntilPast(t, approval.ExpiresAt)

	if err := svc.SweepExpired(ctx); err != nil {
		t.Fatalf("sweep: %v", err)
	}
	waitForRunStatus(t, backend, runID, domain.RunFailed)

	stored, err := backend.GetApproval(ctx, approval.ID)
	if err != nil {
		t.Fatalf("get approval: %v", err)
	}
	if stored.Decision != domain.ApprovalExpired {
		t.Fatalf("approval after sweep = %s by %s, want an expiry", stored.Decision, stored.Actor)
	}
	events := replayAll(t, backend, runID)
	if indexOfType(events, domain.EventToolApprovalExpired) < 0 {
		t.Fatal("missing tool.approval_expired event")
	}
	failed := false
	for _, ev := range events {
		if ev.Type != domain.EventRunFailed {
			continue
		}
		var p payloadRunFailed
		mustUnmarshal(t, ev.Payload, &p)
		failed = p.CauseCategory == causeHumanTimeout
	}
	if !failed {
		t.Fatalf("run.failed cause = %v, want %s", events, causeHumanTimeout)
	}
}

// The auto-approval gate reads the preset recorded when the human was asked.
func TestAutoApprovesOnTimeoutPresetGate(t *testing.T) {
	cases := []struct {
		mode   domain.SandboxMode
		policy domain.ApprovalPolicy
		want   bool
	}{
		{domain.SandboxModeWorkspaceWrite, domain.ApprovalPolicyAsk, true},
		{domain.SandboxModeReadOnly, domain.ApprovalPolicyAsk, false},
		{domain.SandboxModeDangerFullAccess, domain.ApprovalPolicyAuto, false},
		{domain.SandboxModeWorkspaceWrite, domain.ApprovalPolicyNever, false},
		{"", "", false},
	}
	for _, tc := range cases {
		got := autoApprovesOnTimeout(domain.Approval{SandboxMode: string(tc.mode), ApprovalPolicy: string(tc.policy)})
		if got != tc.want {
			t.Fatalf("autoApprovesOnTimeout(%q, %q) = %v, want %v", tc.mode, tc.policy, got, tc.want)
		}
	}
}

func TestServiceApprovalResumePersistsChunkBeforeProviderEOF(t *testing.T) {
	model := &gatedApprovalResumeModel{blocked: make(chan struct{}), release: make(chan struct{})}
	released := false
	defer func() {
		if !released {
			close(model.release)
		}
	}()
	svc, backend, _ := newApprovalServiceWithModel(t, 5*time.Minute, model)
	runID, err := svc.Run(context.Background(), "sess-resume-stream", "note that I need milk")
	if err != nil {
		t.Fatal(err)
	}
	approval := waitForPendingApproval(t, backend, runID)
	waitForApprovalEvent(t, backend, runID)
	if err := svc.DecideApproval(context.Background(), approval.ID, domain.ApprovalApproved); err != nil {
		t.Fatal(err)
	}
	select {
	case <-model.blocked:
	case <-time.After(2 * time.Second):
		t.Fatal("resumed provider did not expose its first chunk")
	}
	deadline := time.Now().Add(2 * time.Second)
	seen := false
	for time.Now().Before(deadline) && !seen {
		for _, event := range replayAll(t, backend, runID) {
			if event.Type == domain.EventModelDelta && payloadDeltaOf(t, event.Payload) == "恢" {
				seen = true
				break
			}
		}
		if !seen {
			time.Sleep(5 * time.Millisecond)
		}
	}
	if !seen {
		t.Fatal("resumed first chunk was not durable while provider remained open")
	}
	close(model.release)
	released = true
	waitForRunStatus(t, backend, runID, domain.RunCompleted)
	var got strings.Builder
	for _, event := range replayAll(t, backend, runID) {
		if event.Type == domain.EventModelDelta {
			got.WriteString(payloadDeltaOf(t, event.Payload))
		}
	}
	if got.String() != "恢复完成" {
		t.Fatalf("resumed deltas = %q, want exactly-once provider text", got.String())
	}
}

func TestServicePlanModeDoesNotOpenApprovalOrMutate(t *testing.T) {
	svc, backend, _ := newApprovalService(t, 5*time.Minute)
	ctx := context.Background()

	runID, err := svc.RunWithOptions(ctx, "sess-plan", "note that I need milk", RunOptions{Mode: domain.RunModePlan})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	// The effectful call is refused to the model and the run finishes: plan
	// mode is a read-only boundary, not a way to lose the conversation.
	waitForRunStatus(t, backend, runID, domain.RunCompleted)

	approvals, err := backend.ListPendingApprovals(ctx)
	if err != nil {
		t.Fatalf("list approvals: %v", err)
	}
	for _, approval := range approvals {
		if approval.RunID == runID {
			t.Fatalf("plan mode opened approval %+v", approval)
		}
	}
	notes, err := backend.ListNotes(ctx)
	if err != nil {
		t.Fatalf("list notes: %v", err)
	}
	if len(notes) != 0 {
		t.Fatalf("plan mode mutated notes: %+v", notes)
	}

	events := replayAll(t, backend, runID)
	if indexOfType(events, domain.EventToolApprovalRequired) >= 0 {
		t.Fatal("plan mode must not emit tool.approval_required")
	}
	if i := indexOfType(events, domain.EventRunFailed); i >= 0 {
		t.Fatalf("plan mode refusal failed the run: %s", events[i].Payload)
	}
	fi := indexOfType(events, domain.EventToolFinished)
	if fi < 0 {
		t.Fatalf("no tool.finished in %+v", events)
	}
	var fin payloadToolFinished
	mustUnmarshal(t, events[fi].Payload, &fin)
	if !strings.Contains(fin.Result, "did not run") || !strings.Contains(fin.Result, "plan mode") {
		t.Fatalf("plan mode tool result = %q, want a model-visible refusal", fin.Result)
	}
	var started payloadRunStarted
	mustUnmarshal(t, events[0].Payload, &started)
	if started.Mode != string(domain.RunModePlan) {
		t.Fatalf("run.started mode = %q, want plan", started.Mode)
	}
}

func TestServiceApprovalDenyFlow(t *testing.T) {
	svc, backend, _ := newApprovalService(t, 5*time.Minute)
	ctx := context.Background()

	runID, err := svc.Run(ctx, "sess-1", "note something")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	approval := waitForPendingApproval(t, backend, runID)
	if err := svc.DecideApproval(ctx, approval.ID, domain.ApprovalDenied); err != nil {
		t.Fatalf("decide: %v", err)
	}
	waitForRunStatus(t, backend, runID, domain.RunCompleted)

	events := replayAll(t, backend, runID)
	fi := indexOfType(events, domain.EventToolFinished)
	if fi < 0 {
		t.Fatalf("no tool.finished in %+v", events)
	}
	var fin payloadToolFinished
	mustUnmarshal(t, events[fi].Payload, &fin)
	if !strings.Contains(fin.Result, "denied") {
		t.Fatalf("denied tool must not execute: result = %q", fin.Result)
	}
	if strings.Contains(fin.Result, "saved (") {
		t.Fatalf("denied tool executed: result = %q", fin.Result)
	}
	// The model still closes the run afterwards.
	last := events[len(events)-1]
	if last.Type != domain.EventRunCompleted {
		t.Fatalf("last event = %s, want run.completed", last.Type)
	}
}

func TestServiceApprovalDecisionGuards(t *testing.T) {
	svc, backend, _ := newApprovalService(t, 5*time.Minute)
	ctx := context.Background()

	if err := svc.DecideApproval(ctx, "apr_unknown", domain.ApprovalApproved); !errors.Is(err, ErrApprovalNotFound) {
		t.Fatalf("unknown approval: %v, want ErrApprovalNotFound", err)
	}

	runID, err := svc.Run(ctx, "sess-1", "note something")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	approval := waitForPendingApproval(t, backend, runID)

	if err := svc.DecideApproval(ctx, approval.ID, "maybe"); !errors.Is(err, ErrApprovalInvalidDecision) {
		t.Fatalf("invalid decision: %v, want ErrApprovalInvalidDecision", err)
	}
	if err := svc.DecideApproval(ctx, approval.ID, domain.ApprovalApproved); err != nil {
		t.Fatalf("first decision: %v", err)
	}
	// First-writer-wins: the second decision loses.
	if err := svc.DecideApproval(ctx, approval.ID, domain.ApprovalDenied); !errors.Is(err, ErrApprovalAlreadyDecided) {
		t.Fatalf("second decision: %v, want ErrApprovalAlreadyDecided", err)
	}
	waitForRunStatus(t, backend, runID, domain.RunCompleted)
}

func TestServiceApprovalExpired(t *testing.T) {
	svc, backend, _ := newApprovalService(t, time.Millisecond)
	ctx := context.Background()

	runID, err := svc.Run(ctx, "sess-1", "note something")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	approval := waitForPendingApproval(t, backend, runID)
	time.Sleep(10 * time.Millisecond)

	if err := svc.DecideApproval(ctx, approval.ID, domain.ApprovalApproved); !errors.Is(err, ErrApprovalExpired) {
		t.Fatalf("expired decision: %v, want ErrApprovalExpired", err)
	}
	// The suspended run still closes via Cancel (minimal E1 coverage).
	if !svc.Cancel(runID) {
		t.Fatal("cancel of a pending run must report true")
	}
	waitForRunStatus(t, backend, runID, domain.RunCancelled)
}

func TestServiceExpirySweeperClosesPendingApproval(t *testing.T) {
	svc, backend, _ := newApprovalService(t, time.Millisecond)
	ctx := context.Background()
	runID, err := svc.Run(ctx, "sess-1", "note that I need milk")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	approval := waitForPendingApproval(t, backend, runID)
	time.Sleep(10 * time.Millisecond)
	if err := svc.SweepExpired(ctx); err != nil {
		t.Fatalf("sweep: %v", err)
	}
	waitForRunStatus(t, backend, runID, domain.RunFailed)
	stored, err := backend.GetApproval(ctx, approval.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Decision != domain.ApprovalExpired {
		t.Fatalf("approval decision = %q, want expired", stored.Decision)
	}
	if indexOfType(replayAll(t, backend, runID), domain.EventToolApprovalExpired) < 0 {
		t.Fatal("missing tool.approval_expired event")
	}
}

func TestServiceCancelPendingRun(t *testing.T) {
	svc, backend, _ := newApprovalService(t, 5*time.Minute)
	ctx := context.Background()

	runID, err := svc.Run(ctx, "sess-1", "note something")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	waitForPendingApproval(t, backend, runID)

	if !svc.Cancel(runID) {
		t.Fatal("cancel of a pending run must report true")
	}
	waitForRunStatus(t, backend, runID, domain.RunCancelled)

	events := replayAll(t, backend, runID)
	last := events[len(events)-1]
	if last.Type != domain.EventRunCancelled {
		t.Fatalf("last event = %s, want run.cancelled", last.Type)
	}
	if n := countTerminal(events); n != 1 {
		t.Fatalf("terminal events = %d, want exactly 1", n)
	}
	if svc.Cancel(runID) {
		t.Fatal("second cancel of a closed run must report false")
	}
}

type cancelOnApprovalPublishSink struct {
	service  *Service
	delegate *testSink
	once     sync.Once
}

func (sink *cancelOnApprovalPublishSink) Publish(event domain.RunEvent) {
	sink.delegate.Publish(event)
	if event.Type == domain.EventToolApprovalRequired {
		sink.once.Do(func() { sink.service.Cancel(event.RunID) })
	}
}

func TestServiceCancelDuringApprovalPublishClosesDurableApproval(t *testing.T) {
	svc, backend, sink := newApprovalService(t, 5*time.Minute)
	svc.deps.Sink = &cancelOnApprovalPublishSink{service: svc, delegate: sink}

	runID, err := svc.Run(context.Background(), "sess-1", "note something")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	waitForRunStatus(t, backend, runID, domain.RunCancelled)

	approvals, err := backend.ListPendingApprovals(context.Background())
	if err != nil {
		t.Fatalf("list pending approvals: %v", err)
	}
	for _, approval := range approvals {
		if approval.RunID == runID {
			t.Fatalf("cancel during approval publication left approval %s pending", approval.ID)
		}
	}

	events := replayAll(t, backend, runID)
	if indexOfType(events, domain.EventToolApprovalRequired) < 0 {
		t.Fatal("missing tool.approval_required event")
	}
	if indexOfType(events, domain.EventToolApprovalCancelled) < 0 {
		t.Fatal("missing tool.approval_cancelled event")
	}
	if n := countTerminal(events); n != 1 {
		t.Fatalf("terminal events = %d, want exactly 1", n)
	}
}

func TestMapperInterruptDetails(t *testing.T) {
	m := newEventMapper("run-test", 0)
	if _, err := m.onEvent(&adk.AgentEvent{
		Output: &adk.AgentOutput{MessageOutput: &adk.TypedMessageVariant[*schema.Message]{
			Message: &schema.Message{
				Role: schema.Assistant,
				ToolCalls: []schema.ToolCall{{
					ID:       ApprovalFlowCallID,
					Function: schema.FunctionCall{Name: "write_note", Arguments: `{"content":"buy milk"}`},
				}},
			},
		}},
	}); err != nil {
		t.Fatalf("map tool call: %v", err)
	}

	interrupt := &adk.AgentEvent{Action: &adk.AgentAction{Interrupted: &adk.InterruptInfo{
		InterruptContexts: []*adk.InterruptCtx{{
			ID: "agent:vivy;tools:write_note;" + ApprovalFlowCallID,
			Address: adk.Address{
				{ID: "vivy"},
				{ID: "write_note", SubID: ApprovalFlowCallID},
			},
			IsRootCause: true,
		}},
	}}}
	if _, err := m.onEvent(interrupt); !errors.Is(err, errRunInterrupted) {
		t.Fatalf("interrupt must surface errRunInterrupted, got %v", err)
	}
	d := m.interrupt
	if d == nil {
		t.Fatal("interrupt details not captured")
	}
	if d.ResumeTarget != "agent:vivy;tools:write_note;"+ApprovalFlowCallID {
		t.Fatalf("resume target = %q", d.ResumeTarget)
	}
	if d.ToolCallID != ApprovalFlowCallID || d.ToolName != "write_note" {
		t.Fatalf("tool identity = %q / %q", d.ToolCallID, d.ToolName)
	}
	if d.Args["content"] != "buy milk" {
		t.Fatalf("args = %+v", d.Args)
	}
}

// Without a SubID in the address, the mapper falls back to the most
// recent open tool.requested record.
func TestMapperInterruptDetailsFallback(t *testing.T) {
	m := newEventMapper("run-test", 0)
	m.registerOpenCall(openToolCall{id: "call-9", name: "write_note", args: map[string]any{"content": "x"}})

	if _, err := m.onEvent(&adk.AgentEvent{Action: &adk.AgentAction{Interrupted: &adk.InterruptInfo{
		InterruptContexts: []*adk.InterruptCtx{{ID: "intr-9", IsRootCause: true}},
	}}}); !errors.Is(err, errRunInterrupted) {
		t.Fatalf("interrupt must surface errRunInterrupted, got %v", err)
	}
	d := m.interrupt
	if d.ResumeTarget != "intr-9" || d.ToolCallID != "call-9" || d.ToolName != "write_note" {
		t.Fatalf("fallback details = %+v", d)
	}
	if d.Args["content"] != "x" {
		t.Fatalf("fallback args = %+v", d.Args)
	}
}
