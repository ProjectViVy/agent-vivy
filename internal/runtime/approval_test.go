package runtime

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage/sqlite"
	"agent-vivy/internal/tools"
)

// newApprovalService wires the full C6 stack over sqlite: checkpoint
// bridge, write_note gate, approval store, and the scripted model that
// requests the effectful call on turn one.
func newApprovalService(t *testing.T, expiration time.Duration) (*Service, *sqlite.Backend, *testSink) {
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
	eng, err := NewEngine(ctx, NewApprovalFlowModel(), ts, EngineConfig{
		StreamBuffer: 8, MaxEventPayloadBytes: 64 << 10, Checkpoints: checkpoints,
	})
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}
	sink := newTestSink()
	svc := NewService(eng, "scripted", "scripted-v0", ServiceDeps{
		Journal: backend, Runs: backend, Messages: backend, Notes: backend, Approvals: backend,
		ApprovalExpiration: expiration, Sink: sink,
	})
	return svc, backend, sink
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
	want := []domain.EventType{
		domain.EventToolStarted, domain.EventToolFinished,
		domain.EventModelDelta, domain.EventModelCompleted, domain.EventRunCompleted,
	}
	if len(types) != len(want) {
		t.Fatalf("post-approval events = %v", types)
	}
	for i, w := range want {
		if domain.EventType(types[i]) != w {
			t.Fatalf("post-approval event %d = %s, want %s (%v)", i, types[i], w, types)
		}
	}
	var fin payloadToolFinished
	mustUnmarshal(t, events[ai+2].Payload, &fin)
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

func TestServicePlanModeDoesNotOpenApprovalOrMutate(t *testing.T) {
	svc, backend, _ := newApprovalService(t, 5*time.Minute)
	ctx := context.Background()

	runID, err := svc.RunWithOptions(ctx, "sess-plan", "note that I need milk", RunOptions{Mode: domain.RunModePlan})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	waitForRunStatus(t, backend, runID, domain.RunFailed)

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
	m.openCalls = append(m.openCalls, openToolCall{id: "call-9", name: "write_note", args: map[string]any{"content": "x"}})

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
