package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cloudwego/eino/schema"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage/sqlite"
	"agent-vivy/internal/tools"
)

// barrierProbeTool blocks until the configured number of same-batch callers
// are in flight, then releases them together. A serialized executor deadlocks
// on the watchdog instead — that is the proof the batch ran in parallel.
type barrierProbeTool struct {
	name    string
	need    int32
	release chan struct{}
	once    sync.Once

	calls    atomic.Int32
	inflight atomic.Int32
	peak     atomic.Int32
}

func (t *barrierProbeTool) Spec() domain.ToolSpec {
	return domain.ToolSpec{
		Name: t.name, Description: "concurrency probe", Readonly: true,
		Params: map[string]domain.ToolParam{"marker": {Required: true}},
	}
}

func (t *barrierProbeTool) InvokableRun(ctx context.Context, args json.RawMessage) (string, error) {
	now := t.inflight.Add(1)
	defer t.inflight.Add(-1)
	for {
		peak := t.peak.Load()
		if now <= peak || t.peak.CompareAndSwap(peak, now) {
			break
		}
	}
	var parsed struct {
		Marker string `json:"marker"`
	}
	if err := json.Unmarshal(args, &parsed); err != nil {
		return "", err
	}
	if t.calls.Add(1) >= t.need {
		t.once.Do(func() { close(t.release) })
	}
	select {
	case <-t.release:
		return "probe-ok:" + parsed.Marker, nil
	case <-ctx.Done():
		return "", ctx.Err()
	case <-time.After(15 * time.Second):
		return "", errors.New("probe timed out waiting for sibling calls")
	}
}

// Three sibling calls in one model batch must overlap in flight; the barrier
// can only release when all three hold the gate's shared side at once.
func TestParallelToolBatchRunsConcurrently(t *testing.T) {
	ctx := context.Background()
	backend, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "parallel-batch.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = backend.Close() })
	mustCreateSession(t, backend, "sess-parallel")

	probe := &barrierProbeTool{name: "parallel_probe", need: 3, release: make(chan struct{})}
	toolset, err := tools.NewRegistry(probe).Resolve([]string{probe.name})
	if err != nil {
		t.Fatal(err)
	}
	model := NewScriptedModel(
		schema.AssistantMessage("", []schema.ToolCall{
			{ID: "call-1", Function: schema.FunctionCall{Name: probe.name, Arguments: `{"marker":"a"}`}},
			{ID: "call-2", Function: schema.FunctionCall{Name: probe.name, Arguments: `{"marker":"b"}`}},
			{ID: "call-3", Function: schema.FunctionCall{Name: probe.name, Arguments: `{"marker":"c"}`}},
		}),
		schema.AssistantMessage("probes settled.", nil),
	)
	engine, err := NewEngine(ctx, model, toolset, EngineConfig{StreamBuffer: 8, MaxEventPayloadBytes: 64 << 10})
	if err != nil {
		t.Fatal(err)
	}
	svc := NewService(engine, "scripted", "scripted-v0", ServiceDeps{
		Journal: backend, Work: backend, Runs: backend, Messages: backend, Sessions: backend,
		PrimaryRuns: backend, Sink: newTestSink(), PolicyDefaultProfile: domain.PolicyProfileFullAuto,
	})
	runID, err := svc.Run(ctx, "sess-parallel", "probe in parallel")
	if err != nil {
		t.Fatal(err)
	}
	waitForRunStatus(t, backend, runID, domain.RunCompleted)

	if got := probe.calls.Load(); got != 3 {
		t.Fatalf("probe invocations = %d, want 3", got)
	}
	if got := probe.peak.Load(); got != 3 {
		t.Fatalf("peak in-flight probes = %d, want 3 concurrent calls", got)
	}
	// Every call journaled its own tool.finished; Eino restores the
	// original call order inside the result message the model sees.
	finished := map[string]bool{}
	for _, event := range replayAll(t, backend, runID) {
		if event.Type != domain.EventToolFinished {
			continue
		}
		var payload payloadToolFinished
		mustUnmarshal(t, event.Payload, &payload)
		finished[payload.ToolCallID] = true
	}
	for _, id := range []string{"call-1", "call-2", "call-3"} {
		if !finished[id] {
			t.Fatalf("tool.finished missing %s: %v", id, finished)
		}
	}
}

// The exclusive side is a full barrier: it waits for in-flight shared
// dispatches, commits the terminal fence, and only then do new shared
// dispatches observe the fence — the check-to-invoke boundary survives
// parallel admission.
func TestWorkGateExclusiveDrainsSharedDispatches(t *testing.T) {
	svc := NewService(nil, "scripted", "scripted-v0", ServiceDeps{})
	const runID = domain.RunID("run-gate-rw")
	svc.workGates[runID] = &sync.RWMutex{}
	ctx := withRunID(context.Background(), runID)

	entered := make(chan struct{})
	release := make(chan struct{})
	sharedDone := make(chan struct{})
	go func() {
		defer close(sharedDone)
		_, _ = svc.WorkToolCall(ctx, false, func() (string, error) {
			close(entered)
			<-release
			return "a", nil
		})
	}()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("shared dispatch never entered")
	}

	commitStarted := make(chan struct{})
	commitDone := make(chan struct{})
	go func() {
		defer close(commitDone)
		_, _ = svc.WorkToolCall(ctx, true, func() (string, error) {
			close(commitStarted)
			svc.fenceWorkRun(runID)
			return "b", nil
		})
	}()
	select {
	case <-commitStarted:
		t.Fatal("exclusive commit started while a shared dispatch was in flight")
	case <-time.After(150 * time.Millisecond):
	}
	if svc.WorkRunFenced(ctx) {
		t.Fatal("terminal fence committed while a shared dispatch was in flight")
	}

	close(release)
	select {
	case <-sharedDone:
	case <-time.After(5 * time.Second):
		t.Fatal("shared dispatch did not settle")
	}
	select {
	case <-commitDone:
	case <-time.After(5 * time.Second):
		t.Fatal("exclusive commit did not proceed after shared dispatches drained")
	}
	if !svc.WorkRunFenced(ctx) {
		t.Fatal("terminal fence missing after the exclusive commit")
	}
	// The same check dispatchUngated performs must see the fence for any
	// call admitted after the commit.
	observed := false
	if _, err := svc.WorkToolCall(ctx, false, func() (string, error) {
		observed = svc.WorkRunFenced(ctx)
		return "c", nil
	}); err != nil {
		t.Fatal(err)
	}
	if !observed {
		t.Fatal("post-commit shared dispatch did not observe the terminal fence")
	}
}

// A terminal work commit in an earlier batch fences later batches for the
// rest of the run: the next turn's ordinary call is refused before reaching
// the tool.
func TestTerminalWorkCommitFencesNextBatchCalls(t *testing.T) {
	ctx := context.Background()
	backend, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "fenced-batch.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = backend.Close() })
	const sessionID = domain.SessionID("sess-fenced")
	mustCreateSession(t, backend, sessionID)
	if _, err := backend.CommitWork(ctx, domain.WorkMutation{
		SessionID: sessionID, ExpectedVersion: 0, RequestID: "seed-goal", RequestHash: "seed-goal",
		Kind: domain.WorkEventGoalCreated, Goal: domain.GoalRef{ID: "goal-1", Revision: 1},
		Objective: "bounded objective", MaxRounds: 3,
	}); err != nil {
		t.Fatal(err)
	}

	probe := &barrierProbeTool{name: "late_probe", need: 1, release: make(chan struct{})}
	toolset, err := tools.NewRegistry(probe, tools.NewReportGoal()).Resolve([]string{probe.name, tools.ReportGoalName})
	if err != nil {
		t.Fatal(err)
	}
	model := NewScriptedModel(
		schema.AssistantMessage("", []schema.ToolCall{
			{ID: "call-report", Function: schema.FunctionCall{
				Name:      tools.ReportGoalName,
				Arguments: `{"goal_id":"goal-1","revision":1,"status":"completed","reason":"done"}`,
			}},
		}),
		schema.AssistantMessage("", []schema.ToolCall{
			{ID: "call-late", Function: schema.FunctionCall{Name: probe.name, Arguments: `{"marker":"late"}`}},
		}),
		schema.AssistantMessage("finished.", nil),
	)
	engine, err := NewEngine(ctx, model, toolset, EngineConfig{StreamBuffer: 8, MaxEventPayloadBytes: 64 << 10,
		AutoApproveTools: []string{tools.ReportGoalName}})
	if err != nil {
		t.Fatal(err)
	}
	svc := NewService(engine, "scripted", "scripted-v0", ServiceDeps{
		Journal: backend, Work: backend, Runs: backend, Messages: backend, Sessions: backend,
		PrimaryRuns: backend, GoalRuns: backend, Sink: newTestSink(), PolicyDefaultProfile: domain.PolicyProfileFullAuto,
	})
	runID, err := svc.RunWithOptions(ctx, sessionID, "report then probe", RunOptions{GoalRound: &GoalRoundAdmission{
		ExpectedVersion: 1, RequestID: "round-admit-1", RequestHash: "round-admit-1",
		Goal: domain.GoalRef{ID: "goal-1", Revision: 1}, Round: 1,
	}})
	if err != nil {
		t.Fatal(err)
	}
	waitForRunStatus(t, backend, runID, domain.RunCompleted)
	if got := probe.calls.Load(); got != 0 {
		t.Fatalf("fenced probe invocations = %d, want zero", got)
	}
	var sawRefusal bool
	for _, event := range replayAll(t, backend, runID) {
		if event.Type == domain.EventToolFinished && strings.Contains(string(event.Payload), "terminal work-control action") {
			sawRefusal = true
		}
	}
	if !sawRefusal {
		t.Fatal("later-batch call was not refused by the terminal fence")
	}
}

// Two effectful calls in one batch interrupt together; the run must surface
// them as sequential suspends so every gated effect still gets its own human
// decision.
func TestTwoEffectfulCallsInOneBatchSuspendSequentially(t *testing.T) {
	model := NewScriptedModel(
		schema.AssistantMessage("", []schema.ToolCall{
			{ID: "call-note-a", Function: schema.FunctionCall{Name: tools.WriteNoteName, Arguments: `{"content":"first"}`}},
			{ID: "call-note-b", Function: schema.FunctionCall{Name: tools.WriteNoteName, Arguments: `{"content":"second"}`}},
		}),
		schema.AssistantMessage("both notes saved.", nil),
	)
	svc, backend, _ := newApprovalServiceWithModel(t, 5*time.Minute, model)
	ctx := context.Background()
	mustCreateSession(t, backend, "sess-two-approvals")

	runID, err := svc.Run(ctx, "sess-two-approvals", "save two notes")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	first := waitForPendingApproval(t, backend, runID)
	if err := svc.DecideApproval(ctx, first.ID, domain.ApprovalApproved); err != nil {
		t.Fatalf("approve first: %v", err)
	}
	second := waitForPendingApproval(t, backend, runID)
	if second.ID == first.ID {
		t.Fatalf("second suspend re-used approval %q", second.ID)
	}
	t.Logf("approvals: first=%s call=%s second=%s call=%s", first.ID, first.ToolCallID, second.ID, second.ToolCallID)
	if err := svc.DecideApproval(ctx, second.ID, domain.ApprovalApproved); err != nil {
		t.Fatalf("approve second: %v", err)
	}
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		run, err := backend.GetRun(ctx, runID)
		if err != nil {
			t.Fatal(err)
		}
		if run.Status == domain.RunCompleted {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if run, _ := backend.GetRun(ctx, runID); run.Status != domain.RunCompleted {
		t.Fatalf("run status = %s", run.Status)
	}

	notes, err := backend.ListNotes(ctx)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, note := range notes {
		got[note.Content] = true
	}
	if !got["first"] || !got["second"] {
		t.Fatalf("approved effects = %v, want both notes", got)
	}
	var approvalEvents int
	for _, event := range replayAll(t, backend, runID) {
		if event.Type == domain.EventToolApprovalRequired {
			approvalEvents++
		}
	}
	if approvalEvents != 2 {
		t.Fatalf("tool.approval_required events = %d, want 2 sequential suspends", approvalEvents)
	}
}
