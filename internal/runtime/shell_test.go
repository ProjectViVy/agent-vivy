package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cloudwego/eino/schema"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
	"agent-vivy/internal/storage/sqlite"
	"agent-vivy/internal/tools"
)

type shellFixture struct {
	service   *Service
	backend   *sqlite.Backend
	workspace *WorkspaceManager
	sessionID domain.SessionID
}

func newShellFixture(t *testing.T, approval domain.ApprovalPolicy, hooks ...*ToolHookChain) shellFixture {
	t.Helper()
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash is not available on this host")
	}
	ctx := context.Background()
	backend, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "shell.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = backend.Close() })
	commands, root := newBashBackendForTest(t, domain.SandboxModeWorkspaceWrite)
	workspace, err := NewWorkspaceManager(root)
	if err != nil {
		t.Fatalf("workspace manager: %v", err)
	}
	ts, err := tools.NewRegistry(tools.NewBash(commands)).Resolve([]string{tools.BashName})
	if err != nil {
		t.Fatalf("resolve bash: %v", err)
	}
	config := EngineConfig{StreamBuffer: 8, MaxEventPayloadBytes: 64 << 10}
	if len(hooks) > 0 {
		config.ToolHooks = hooks[0]
	}
	eng, err := NewEngine(ctx, NewScriptedModel(schema.AssistantMessage("unused", nil)), ts, config)
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}
	sessionID := domain.SessionID("shell-session-" + newControlTestSuffix())
	if err := backend.CreateSession(ctx, domain.Session{ID: sessionID, Title: "shell test", CreatedAt: time.Now().UnixMilli()}); err != nil {
		t.Fatalf("create session: %v", err)
	}
	if err := backend.UpdateSandboxPolicy(ctx, sessionID, domain.SandboxModeWorkspaceWrite, approval); err != nil {
		t.Fatalf("set session policy: %v", err)
	}
	svc := NewService(eng, "", "", ServiceDeps{
		Journal: backend, Runs: backend, Messages: backend, Approvals: backend, Questions: backend,
		Sessions: backend, Workspaces: workspace, ShellState: backend.Blobs(),
		ApprovalExpiration: 5 * time.Minute, Sink: newTestSink(),
	})
	t.Cleanup(func() {
		svc.CancelAll()
		svc.WaitIdle(context.Background())
	})
	return shellFixture{service: svc, backend: backend, workspace: workspace, sessionID: sessionID}
}

func newControlTestSuffix() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

func waitForShellApproval(t *testing.T, backend storage.ApprovalStore, runID domain.RunID) domain.Approval {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		items, err := backend.ListPendingApprovals(context.Background())
		if err == nil {
			for _, item := range items {
				if item.RunID == runID {
					return item
				}
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("run %s never opened a shell approval", runID)
	return domain.Approval{}
}

func shellEventString(t *testing.T, events []domain.RunEvent) string {
	t.Helper()
	var out strings.Builder
	for _, event := range events {
		out.Write(event.Payload)
		out.WriteByte('\n')
	}
	return out.String()
}

func assertNoModelEvents(t *testing.T, events []domain.RunEvent) {
	t.Helper()
	for _, event := range events {
		if event.Type == domain.EventModelRequest || event.Type == domain.EventModelCompleted || event.Type == domain.EventModelDelta {
			t.Fatalf("direct shell emitted model event %s", event.Type)
		}
	}
}

func TestRunShellRejectsDeletedSessionTombstone(t *testing.T) {
	f := newShellFixture(t, domain.ApprovalPolicyAsk)
	if err := f.service.DeleteSession(context.Background(), f.sessionID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.service.RunShell(context.Background(), f.sessionID, "echo must-not-run"); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("RunShell after delete error = %v, want not found", err)
	}
}

func eventIndex(events []domain.RunEvent, typ domain.EventType) int {
	for index, event := range events {
		if event.Type == typ {
			return index
		}
	}
	return -1
}

func assertShellStateDeleted(t *testing.T, blobs storage.BlobStore, approval domain.Approval) {
	t.Helper()
	ref := shellStateRef(approval.ProposalData)
	if ref == "" {
		t.Fatal("shell approval has no state reference")
	}
	if _, found, err := blobs.Get(context.Background(), ref); err != nil || found {
		t.Fatalf("protected shell state %s remains: found=%v err=%v", ref, found, err)
	}
}

func simulateShellProcessDeath(service *Service, runID domain.RunID) {
	service.mu.Lock()
	if cancel := service.active[runID]; cancel != nil {
		cancel()
	}
	delete(service.active, runID)
	delete(service.shellPending, runID)
	delete(service.ledgers, runID)
	delete(service.snapshots, runID)
	service.mu.Unlock()
}

func TestShellAvailabilityRequiresCompleteGovernedBackend(t *testing.T) {
	f := newShellFixture(t, domain.ApprovalPolicyAuto)
	if !f.service.ShellAvailable() {
		t.Fatal("fully wired governed shell is unavailable")
	}
	unwired := NewService(f.service.engine, "", "", ServiceDeps{})
	if unwired.ShellAvailable() {
		t.Fatal("shell advertised without workspace, stores, and sink")
	}
}

func TestRunShellSafeAutoUsesUnifiedLifecycleWithoutModel(t *testing.T) {
	f := newShellFixture(t, domain.ApprovalPolicyAuto)
	runID, err := f.service.RunShell(context.Background(), f.sessionID, "echo direct_shell_ok")
	if err != nil {
		t.Fatalf("run shell: %v", err)
	}
	waitForRunStatus(t, f.backend, runID, domain.RunCompleted)
	events := replayAll(t, f.backend, runID)
	assertNoModelEvents(t, events)
	want := []domain.EventType{domain.EventRunStarted, domain.EventToolRequested, domain.EventToolStarted, domain.EventToolFinished, domain.EventRunCompleted}
	last := -1
	for _, typ := range want {
		index := eventIndex(events, typ)
		if index <= last {
			t.Fatalf("event %s out of order in %v", typ, events)
		}
		last = index
	}
	if eventIndex(events, domain.EventToolApprovalRequired) >= 0 {
		t.Fatal("safe shell unexpectedly opened approval")
	}
	if !strings.Contains(shellEventString(t, events), "direct_shell_ok") {
		t.Fatalf("shell output marker missing: %s", shellEventString(t, events))
	}
	messages, err := f.backend.ListMessages(context.Background(), f.sessionID)
	if err != nil {
		t.Fatalf("list messages: %v", err)
	}
	for _, message := range messages {
		if strings.Contains(string(message.ToolArgs), "echo direct_shell_ok") || strings.Contains(message.Content, "echo direct_shell_ok") {
			t.Fatal("raw shell script entered ordinary message history")
		}
	}
	if len(messages) != 2 || messages[0].ToolName != tools.BashName || messages[1].ToolName != tools.BashName {
		t.Fatalf("sanitized shell history = %+v, want tool call/result pair", messages)
	}
}

func TestRunShellApprovalApproveRunsAfterDurableDecision(t *testing.T) {
	f := newShellFixture(t, domain.ApprovalPolicyAsk)
	runID, err := f.service.RunShell(context.Background(), f.sessionID, "echo approved > approved_marker")
	if err != nil {
		t.Fatalf("run shell: %v", err)
	}
	approval := waitForShellApproval(t, f.backend, runID)
	if approval.ProposalData == nil || strings.Contains(string(approval.ProposalData), "echo approved > approved_marker") {
		t.Fatal("approval stored raw shell input")
	}
	if strings.Contains(approval.Preview, "echo approved > approved_marker") || strings.Contains(approval.Target, "echo approved > approved_marker") {
		t.Fatal("approval preview stored raw shell input")
	}
	if err := f.service.DecideApproval(context.Background(), approval.ID, domain.ApprovalApproved); err != nil {
		t.Fatalf("approve shell: %v", err)
	}
	waitForRunStatus(t, f.backend, runID, domain.RunCompleted)
	workspace, err := f.workspace.Ensure(context.Background(), runID)
	if err != nil {
		t.Fatalf("shell workspace: %v", err)
	}
	if _, err := os.Stat(filepath.Join(workspace.Path, "approved_marker")); err != nil {
		t.Logf("approval events: %s", shellEventString(t, replayAll(t, f.backend, runID)))
		t.Fatalf("approved shell did not create marker: %v", err)
	}
	events := replayAll(t, f.backend, runID)
	assertNoModelEvents(t, events)
	for _, typ := range []domain.EventType{domain.EventRunStarted, domain.EventToolRequested, domain.EventToolApprovalRequired, domain.EventToolApprovalDecided, domain.EventToolStarted, domain.EventToolFinished, domain.EventRunCompleted} {
		if eventIndex(events, typ) < 0 {
			t.Fatalf("shell approval lifecycle missing %s: %v", typ, events)
		}
	}
	required := eventIndex(events, domain.EventToolApprovalRequired)
	decided := eventIndex(events, domain.EventToolApprovalDecided)
	started := eventIndex(events, domain.EventToolStarted)
	if !(required < decided && decided < started) {
		t.Fatalf("approval barrier order required=%d decided=%d started=%d", required, decided, started)
	}
	assertShellStateDeleted(t, f.backend.Blobs(), approval)
}

func TestRunShellApprovalDenyDoesNotExecute(t *testing.T) {
	f := newShellFixture(t, domain.ApprovalPolicyAsk)
	runID, err := f.service.RunShell(context.Background(), f.sessionID, "echo denied > denied_marker")
	if err != nil {
		t.Fatalf("run shell: %v", err)
	}
	approval := waitForShellApproval(t, f.backend, runID)
	if err := f.service.DecideApproval(context.Background(), approval.ID, domain.ApprovalDenied); err != nil {
		t.Fatalf("deny shell: %v", err)
	}
	waitForRunStatus(t, f.backend, runID, domain.RunCompleted)
	workspace, err := f.workspace.Ensure(context.Background(), runID)
	if err != nil {
		t.Fatalf("shell workspace: %v", err)
	}
	if _, err := os.Stat(filepath.Join(workspace.Path, "denied_marker")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("denied shell created marker, stat error = %v", err)
	}
	events := replayAll(t, f.backend, runID)
	if eventIndex(events, domain.EventToolStarted) >= 0 {
		t.Fatal("denied shell emitted tool.started despite zero execution")
	}
	if eventIndex(events, domain.EventRunCompleted) < 0 || eventIndex(events, domain.EventRunFailed) >= 0 || eventIndex(events, domain.EventRunCancelled) >= 0 {
		t.Fatalf("denied shell terminal = %v", events)
	}
	if n := countTerminal(events); n != 1 {
		t.Fatalf("denied shell terminal count = %d, want 1", n)
	}
	assertShellStateDeleted(t, f.backend.Blobs(), approval)
}

func TestRunShellDangerousAndNetworkInputsFailClosedBeforeRun(t *testing.T) {
	f := newShellFixture(t, domain.ApprovalPolicyAuto)
	for _, script := range []string{"rm -rf /", "curl https://example.com", "cat /etc/passwd"} {
		if _, err := f.service.RunShell(context.Background(), f.sessionID, script); !errors.Is(err, ErrPolicyDenied) {
			t.Fatalf("RunShell(%q) error = %v, want ErrPolicyDenied", script, err)
		}
	}
	runs, err := f.backend.ListRunsBySession(context.Background(), f.sessionID)
	if err != nil {
		t.Fatalf("list runs: %v", err)
	}
	if len(runs) != 0 {
		t.Fatalf("fail-closed shell inputs created %d runs", len(runs))
	}
}

func TestRunShellCancelPendingApproval(t *testing.T) {
	f := newShellFixture(t, domain.ApprovalPolicyAsk)
	runID, err := f.service.RunShell(context.Background(), f.sessionID, "echo cancelled > cancelled_marker")
	if err != nil {
		t.Fatalf("run shell: %v", err)
	}
	approval := waitForShellApproval(t, f.backend, runID)
	if !f.service.Cancel(runID) {
		t.Fatal("cancel pending shell returned false")
	}
	waitForRunStatus(t, f.backend, runID, domain.RunCancelled)
	if got, err := f.backend.GetApproval(context.Background(), approval.ID); err == nil && got.Decision == domain.ApprovalPending {
		t.Fatal("cancel left shell approval pending")
	}
	workspace, err := f.workspace.Ensure(context.Background(), runID)
	if err != nil {
		t.Fatalf("shell workspace: %v", err)
	}
	if _, err := os.Stat(filepath.Join(workspace.Path, "cancelled_marker")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("cancelled shell created marker, stat error = %v", err)
	}
	assertShellStateDeleted(t, f.backend.Blobs(), approval)
}

func TestRunShellExpirePendingApprovalDeletesState(t *testing.T) {
	f := newShellFixture(t, domain.ApprovalPolicyAsk)
	runID, err := f.service.RunShell(context.Background(), f.sessionID, "echo expired > expired_marker")
	if err != nil {
		t.Fatalf("run shell: %v", err)
	}
	approval := waitForShellApproval(t, f.backend, runID)
	deadline := time.Now().Add(5 * time.Second)
	for eventIndex(replayAll(t, f.backend, runID), domain.EventToolApprovalRequired) < 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if err := f.service.expireApproval(context.Background(), approval, "test timeout"); err != nil {
		t.Fatalf("expire shell approval: %v", err)
	}
	waitForRunStatus(t, f.backend, runID, domain.RunFailed)
	if n := countTerminal(replayAll(t, f.backend, runID)); n != 1 {
		t.Fatalf("expired shell terminal count = %d", n)
	}
	assertShellStateDeleted(t, f.backend.Blobs(), approval)
}

func TestRunShellCancelExecutingAndBoundedRedactedResult(t *testing.T) {
	f := newShellFixture(t, domain.ApprovalPolicyAuto)
	runID, err := f.service.RunShell(context.Background(), f.sessionID, "printf 'password=hunter2\\n'; printf '%100000s' sk-live-abcdefghijklmnop")
	if err != nil {
		t.Fatalf("run shell: %v", err)
	}
	waitForRunStatus(t, f.backend, runID, domain.RunCompleted)
	events := replayAll(t, f.backend, runID)
	joined := shellEventString(t, events)
	if strings.Contains(joined, "sk-live-") || strings.Contains(joined, "hunter2") {
		t.Fatal("secret canary entered shell journal")
	}
	for _, event := range events {
		if event.Type == domain.EventToolFinished && len(event.Payload) > maxShellResultBytes {
			t.Fatalf("tool.finished payload is unbounded: %d", len(event.Payload))
		}
	}
	// A second run proves Cancel reaches the same detached context that the
	// direct shell process uses, rather than merely closing a UI bubble.
	longRun, err := f.service.RunShell(context.Background(), f.sessionID, "sleep 30")
	if err != nil {
		t.Fatalf("long shell: %v", err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		events = replayAll(t, f.backend, longRun)
		if eventIndex(events, domain.EventToolStarted) >= 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if eventIndex(events, domain.EventToolStarted) < 0 {
		t.Fatal("long shell never started")
	}
	if !f.service.Cancel(longRun) {
		t.Fatal("cancel executing shell returned false")
	}
	waitForRunStatus(t, f.backend, longRun, domain.RunCancelled)
}

func TestRunShellApprovalRecoversProtectedStateAfterRestart(t *testing.T) {
	f := newShellFixture(t, domain.ApprovalPolicyAsk)
	runID, err := f.service.RunShell(context.Background(), f.sessionID, "echo recovered > recovered_marker")
	if err != nil {
		t.Fatalf("run shell: %v", err)
	}
	approval := waitForShellApproval(t, f.backend, runID)
	deadline := time.Now().Add(5 * time.Second)
	for eventIndex(replayAll(t, f.backend, runID), domain.EventToolApprovalRequired) < 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if eventIndex(replayAll(t, f.backend, runID), domain.EventToolApprovalRequired) < 0 {
		t.Fatal("shell approval row became visible without its durable review event")
	}
	// A fresh Service over the same stores has no Eino checkpoint dependency;
	// Recover rebuilds only the waiting direct shell state.
	// Simulate the old process disappearing without invoking its cancellation
	// path; both Services intentionally share the durable test stores.
	simulateShellProcessDeath(f.service, runID)
	restarted := NewService(f.service.engine, "", "", ServiceDeps{
		Journal: f.backend, Runs: f.backend, Messages: f.backend, Approvals: f.backend, Questions: f.backend,
		Sessions: f.backend, Workspaces: f.workspace, ShellState: f.backend.Blobs(), ApprovalExpiration: 5 * time.Minute,
		Sink: newTestSink(),
	})
	if err := restarted.Recover(context.Background()); err != nil {
		t.Fatalf("recover shell approval: %v", err)
	}
	if err := restarted.DecideApproval(context.Background(), approval.ID, domain.ApprovalApproved); err != nil {
		t.Fatalf("approve recovered shell: %v", err)
	}
	waitForRunStatus(t, f.backend, runID, domain.RunCompleted)
	workspace, err := f.workspace.Ensure(context.Background(), runID)
	if err != nil {
		t.Fatalf("shell workspace: %v", err)
	}
	if _, err := os.Stat(filepath.Join(workspace.Path, "recovered_marker")); err != nil {
		t.Logf("recovered events: %s", shellEventString(t, replayAll(t, f.backend, runID)))
		t.Fatalf("recovered shell did not execute: %v", err)
	}
	assertShellStateDeleted(t, f.backend.Blobs(), approval)
	restarted.CancelAll()
	restarted.WaitIdle(context.Background())
}

func TestDeleteSessionSealsRecoveredShellPendingRun(t *testing.T) {
	f := newShellFixture(t, domain.ApprovalPolicyAsk)
	runID, err := f.service.RunShell(context.Background(), f.sessionID, "echo must-not-recover")
	if err != nil {
		t.Fatal(err)
	}
	approval := waitForShellApproval(t, f.backend, runID)
	deadline := time.Now().Add(5 * time.Second)
	for eventIndex(replayAll(t, f.backend, runID), domain.EventToolApprovalRequired) < 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	simulateShellProcessDeath(f.service, runID)
	restarted := NewService(f.service.engine, "", "", ServiceDeps{
		Journal: f.backend, Runs: f.backend, Messages: f.backend, Approvals: f.backend, Questions: f.backend,
		Sessions: f.backend, Workspaces: f.workspace, ShellState: f.backend.Blobs(), ApprovalExpiration: 5 * time.Minute,
		Sink: newTestSink(),
	})
	if err := restarted.Recover(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := restarted.DeleteSession(context.Background(), f.sessionID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.backend.GetRun(context.Background(), runID); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("recovered shell run after delete = %v, want not found", err)
	}
	if err := restarted.DecideApproval(context.Background(), approval.ID, domain.ApprovalApproved); err == nil {
		t.Fatal("deleted recovered shell approval remained actionable")
	}
	restarted.CancelAll()
	restarted.WaitIdle(context.Background())
}

func TestRunShellRecoveryFailureCancelsApprovalAndDeletesState(t *testing.T) {
	f := newShellFixture(t, domain.ApprovalPolicyAsk)
	runID, err := f.service.RunShell(context.Background(), f.sessionID, "echo never-runs > marker")
	if err != nil {
		t.Fatalf("run shell: %v", err)
	}
	approval := waitForShellApproval(t, f.backend, runID)
	deadline := time.Now().Add(5 * time.Second)
	for eventIndex(replayAll(t, f.backend, runID), domain.EventToolApprovalRequired) < 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	ref := shellStateRef(approval.ProposalData)
	if err := f.backend.Blobs().Put(context.Background(), ref, []byte(`{"args":`)); err != nil {
		t.Fatalf("corrupt shell state: %v", err)
	}
	simulateShellProcessDeath(f.service, runID)
	restarted := NewService(f.service.engine, "", "", ServiceDeps{
		Journal: f.backend, Runs: f.backend, Messages: f.backend, Approvals: f.backend, Questions: f.backend,
		Sessions: f.backend, Workspaces: f.workspace, ShellState: f.backend.Blobs(), Sink: newTestSink(),
	})
	if err := restarted.Recover(context.Background()); err != nil {
		t.Fatalf("recover corrupt shell state: %v", err)
	}
	waitForRunStatus(t, f.backend, runID, domain.RunFailed)
	stored, err := f.backend.GetApproval(context.Background(), approval.ID)
	if err != nil || stored.Decision == domain.ApprovalPending {
		t.Fatalf("recovery failure approval = %+v, err=%v", stored, err)
	}
	assertShellStateDeleted(t, f.backend.Blobs(), approval)
}

func TestRunShellHookCannotInjectExecutionControlsOrLeakReason(t *testing.T) {
	canary := "raw-hook-canary echo should-not-leak"
	hook := &testToolHook{name: "shell-guard", pre: PreToolUseResult{
		Decision:    hookRewrite,
		UpdatedArgs: json.RawMessage(`{"command":"echo blocked","run_in_background":true}`),
		Reason:      canary,
	}}
	f := newShellFixture(t, domain.ApprovalPolicyAuto, NewToolHookChain(time.Second, hook))
	runID, err := f.service.RunShell(context.Background(), f.sessionID, "echo original")
	if err != nil {
		t.Fatalf("run shell: %v", err)
	}
	waitForRunStatus(t, f.backend, runID, domain.RunFailed)
	events := replayAll(t, f.backend, runID)
	if eventIndex(events, domain.EventToolStarted) >= 0 {
		t.Fatal("hook-injected background control reached execution")
	}
	if strings.Contains(shellEventString(t, events), canary) {
		t.Fatal("untrusted hook reason entered shell journal")
	}
}

func TestShellStateReferenceIsOpaqueJSON(t *testing.T) {
	args, _ := json.Marshal(map[string]string{"command": "touch secret-marker"})
	ref := shellStateRefData("shell_state_test")
	if strings.Contains(string(ref), "touch secret-marker") || shellStateRef(ref) != "shell_state_test" {
		t.Fatalf("opaque shell state reference malformed: %s", ref)
	}
	if !strings.Contains(shellAuditLabel(args), "redacted") {
		t.Fatal("shell audit label is not redacted")
	}
}

func TestShellApprovalHashBindsCompleteScriptHookGeneration(t *testing.T) {
	args := json.RawMessage(`{"command":"echo safe"}`)
	first := NewToolHookChain(time.Second, NewScriptHook("guard --mode one", "bash", time.Second))
	second := NewToolHookChain(time.Second, NewScriptHook("guard --mode two", "bash", time.Second))
	if shellApprovalHash(args, "policy", first) == shellApprovalHash(args, "policy", second) {
		t.Fatal("approval hash ignored behavior-affecting script-hook arguments")
	}
}

func TestRecoverRejectsCrossRunShellStateReference(t *testing.T) {
	f := newShellFixture(t, domain.ApprovalPolicyAsk)
	runID, err := f.service.RunShell(context.Background(), f.sessionID, "echo safe")
	if err != nil {
		t.Fatal(err)
	}
	approval := waitForShellApproval(t, f.backend, runID)
	run, err := f.backend.GetRun(context.Background(), runID)
	if err != nil {
		t.Fatal(err)
	}
	approval.ProposalData = shellStateRefData("shell_state_different-run")
	if err := f.service.rebuildShellPending(context.Background(), run, approval); err == nil || !strings.Contains(err.Error(), "reference does not match run") {
		t.Fatalf("cross-run state reference error = %v", err)
	}
}

func TestRecoverGarbageCollectsTerminalShellState(t *testing.T) {
	f := newShellFixture(t, domain.ApprovalPolicyAuto)
	runID := domain.RunID("terminal-shell-gc")
	if err := f.backend.CreateRun(context.Background(), domain.Run{ID: runID, SessionID: f.sessionID, Status: domain.RunCompleted, CreatedAt: time.Now().UnixMilli()}); err != nil {
		t.Fatal(err)
	}
	ref := shellStateRefForRun(runID)
	if err := f.backend.Blobs().Put(context.Background(), ref, []byte(`{"args":{"command":"secret"}}`)); err != nil {
		t.Fatal(err)
	}
	if err := f.service.Recover(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := f.backend.Blobs().Get(context.Background(), ref); err != nil || ok {
		t.Fatalf("terminal shell state still present: ok=%v err=%v", ok, err)
	}
}
