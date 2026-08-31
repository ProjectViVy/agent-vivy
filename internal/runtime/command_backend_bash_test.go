package runtime

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cloudwego/eino/schema"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage/sqlite"
	"agent-vivy/internal/tools"
)

func newBashBackendForTest(t *testing.T, mode domain.SandboxMode) (*EinoCommandBackend, string) {
	t.Helper()
	root := t.TempDir()
	manager, err := NewWorkspaceManager(root)
	if err != nil {
		t.Fatalf("workspace manager: %v", err)
	}
	sandbox, err := NewSandboxManager(mode, root, []string{"go"}, nil)
	if err != nil {
		t.Fatalf("sandbox manager: %v", err)
	}
	backend := NewEinoCommandBackend(manager, sandbox, []string{"go"}, 0)
	return backend, root
}

func TestBashBackendRunsShellOutsideAllowlist(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash is not available on this host")
	}
	backend, _ := newBashBackendForTest(t, domain.SandboxModeWorkspaceWrite)
	// The allowlist deliberately names only "go": the bash path must not be
	// gated by the per-executable whitelist.
	result, err := backend.Execute(withRunID(context.Background(), "run_bash_ok"), "run_bash_ok", tools.CommandRequest{Command: "bash", Args: []string{"-c", "echo vivy_bash_ok"}})
	if err != nil {
		t.Fatalf("bash execute: %v", err)
	}
	if !strings.Contains(result.Stdout, "vivy_bash_ok") || result.ExitCode != 0 {
		t.Fatalf("bash result = %+v, want echoed marker with exit 0", result)
	}
	if !result.Untrusted {
		t.Fatal("bash result not marked untrusted")
	}
}

func TestBashBackendDeniedInReadOnlySandbox(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash is not available on this host")
	}
	backend, _ := newBashBackendForTest(t, domain.SandboxModeReadOnly)
	_, err := backend.Execute(withRunID(context.Background(), "run_bash_ro"), "run_bash_ro", tools.CommandRequest{Command: "bash", Args: []string{"-c", "echo hi"}})
	if !errors.Is(err, ErrSandboxDenied) {
		t.Fatalf("read-only bash error = %v, want ErrSandboxDenied", err)
	}
}

func TestBashBackendDenyTableDefenseInDepth(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash is not available on this host")
	}
	backend, _ := newBashBackendForTest(t, domain.SandboxModeWorkspaceWrite)
	_, err := backend.Execute(withRunID(context.Background(), "run_bash_deny"), "run_bash_deny", tools.CommandRequest{Command: "bash", Args: []string{"-c", "rm -rf /"}})
	if err == nil || !strings.Contains(err.Error(), "deny-table") {
		t.Fatalf("backend deny-table error = %v, want deny-table rejection", err)
	}
}

func TestBashBackendRejectsMalformedInvocation(t *testing.T) {
	backend, _ := newBashBackendForTest(t, domain.SandboxModeWorkspaceWrite)
	backend.shellPath = ""
	if _, err := backend.Execute(withRunID(context.Background(), "run_bash_missing"), "run_bash_missing", tools.CommandRequest{Command: "bash", Args: []string{"-c", "echo hi"}}); err == nil || !strings.Contains(err.Error(), "bash is not available") {
		t.Fatalf("missing shell error = %v, want bash unavailable", err)
	}
	backend.shellPath = "placeholder"
	if _, err := backend.Execute(withRunID(context.Background(), "run_bash_shape"), "run_bash_shape", tools.CommandRequest{Command: "bash", Args: []string{"echo", "hi"}}); err == nil || !strings.Contains(err.Error(), "single -c script") {
		t.Fatalf("malformed args error = %v, want single -c script", err)
	}
}

func waitForBackendJob(t *testing.T, backend *EinoCommandBackend, id string) tools.JobReadResult {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		res, ok := backend.JobRead(id)
		if ok && (strings.Contains(res.Stdout, "vivy_bg_e2e") || res.Status != tools.JobRunning) {
			return res
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("job %s never produced its marker", id)
	return tools.JobReadResult{}
}

func TestBashBackendBackgroundLaunchOutputAndKill(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash is not available on this host")
	}
	backend, _ := newBashBackendForTest(t, domain.SandboxModeWorkspaceWrite)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	result, err := backend.Execute(ctx, "run_bash_bg", tools.CommandRequest{Command: "bash", Args: []string{"-c", "echo vivy_bg_e2e; sleep 30"}, Background: true})
	if err != nil {
		t.Fatalf("background launch: %v", err)
	}
	if result.JobID == "" || !result.Background || result.JobStatus != string(tools.JobRunning) {
		t.Fatalf("background result = %+v, want running job id", result)
	}
	got := waitForBackendJob(t, backend, result.JobID)
	if got.Status != tools.JobRunning || !strings.Contains(got.Stdout, "vivy_bg_e2e") {
		t.Fatalf("job read = %+v, want running with marker", got)
	}
	killed, err := backend.JobKill(result.JobID)
	if err != nil || killed.Status != tools.JobKilled {
		t.Fatalf("kill = %+v/%v, want killed", killed, err)
	}
}

func TestBashBackendForegroundTimeoutAdoptsJob(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash is not available on this host")
	}
	backend, _ := newBashBackendForTest(t, domain.SandboxModeWorkspaceWrite)
	result, err := backend.Execute(context.Background(), "run_bash_adopt", tools.CommandRequest{Command: "bash", Args: []string{"-c", "echo vivy_bg_e2e; sleep 10"}, TimeoutMS: 400})
	if err != nil {
		t.Fatalf("foreground timeout: %v", err)
	}
	if !result.TimedOut || result.JobID == "" {
		t.Fatalf("timeout result = %+v, want adopted job", result)
	}
	got := waitForBackendJob(t, backend, result.JobID)
	if got.Status != tools.JobRunning || !strings.Contains(got.Stdout, "vivy_bg_e2e") {
		t.Fatalf("adopted job = %+v, want running with marker", got)
	}
	if _, err := backend.JobKill(result.JobID); err != nil {
		t.Fatalf("kill adopted job: %v", err)
	}
}

func TestBashBackendJobsDieWithRunContext(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash is not available on this host")
	}
	backend, _ := newBashBackendForTest(t, domain.SandboxModeWorkspaceWrite)
	ctx, cancel := context.WithCancel(context.Background())
	result, err := backend.Execute(ctx, "run_bash_ctx", tools.CommandRequest{Command: "bash", Args: []string{"-c", "sleep 30"}, Background: true})
	if err != nil {
		t.Fatalf("background launch: %v", err)
	}
	cancel()
	deadline := time.Now().Add(5 * time.Second)
	for {
		res, ok := backend.JobRead(result.JobID)
		if ok && res.Status == tools.JobKilled {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("job %s survived run-context cancellation (status %+v)", result.JobID, res)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

type classifierStubTool struct {
	class    tools.InvocationClass
	findings []string
	calls    int
}

func (t *classifierStubTool) Spec() domain.ToolSpec {
	return domain.ToolSpec{Name: "stub_classifier", Description: "tiered stub", Params: map[string]domain.ToolParam{"command": {Desc: "script", Required: true}}}
}

func (t *classifierStubTool) InvokableRun(context.Context, json.RawMessage) (string, error) {
	t.calls++
	return "ran", nil
}

func (t *classifierStubTool) ClassifyInvocation(json.RawMessage) (tools.InvocationClass, []string, error) {
	return t.class, t.findings, nil
}

func TestToolAdapterTieredApprovalForClassifiedTools(t *testing.T) {
	run := func(policy domain.ApprovalPolicy, profile domain.PolicyProfile, class tools.InvocationClass, whitelist []string) (string, error, int) {
		tool := &classifierStubTool{class: class, findings: []string{"stub finding"}}
		adapter := newToolAdapter(tool, 0, nil, nil, whitelist)
		ctx := withPolicyProfile(withApprovalPolicy(context.Background(), policy), profile)
		result, err := adapter.InvokableRun(ctx, `{"command":"stub script"}`)
		return result, err, tool.calls
	}
	// Safe under auto: no interrupt, direct execution.
	result, err, calls := run(domain.ApprovalPolicyAuto, domain.PolicyProfileDefault, tools.InvocationSafe, nil)
	if err != nil || calls != 1 || !strings.Contains(result, "ran") {
		t.Fatalf("safe+auto ran = %q/%v/%d, want executed once", result, err, calls)
	}
	// Mutating under auto: approval interrupt, tool never runs.
	_, err, calls = run(domain.ApprovalPolicyAuto, domain.PolicyProfileDefault, tools.InvocationMutating, nil)
	if err == nil || calls != 0 {
		t.Fatalf("mutating+auto = err %v calls %d, want interrupt without execution", err, calls)
	}
	if errors.Is(err, ErrPolicyDenied) {
		t.Fatalf("mutating under auto surfaced policy denial %v, want an approval interrupt", err)
	}
	// Safe under ask: 'ask' consults the user for every effectful call.
	_, err, calls = run(domain.ApprovalPolicyAsk, domain.PolicyProfileDefault, tools.InvocationSafe, nil)
	if err == nil || calls != 0 {
		t.Fatalf("safe+ask = err %v calls %d, want interrupt", err, calls)
	}
	// Safe under never: denied, not run.
	_, err, calls = run(domain.ApprovalPolicyNever, domain.PolicyProfileDefault, tools.InvocationSafe, nil)
	if !errors.Is(err, ErrPolicyDenied) || calls != 0 {
		t.Fatalf("safe+never = err %v calls %d, want ErrPolicyDenied", err, calls)
	}
	// Deny table beats the full-auto profile.
	_, err, calls = run(domain.ApprovalPolicyAuto, domain.PolicyProfileFullAuto, tools.InvocationDenied, nil)
	if !errors.Is(err, ErrPolicyDenied) || calls != 0 || !strings.Contains(err.Error(), "stub finding") {
		t.Fatalf("denied+full-auto = err %v calls %d, want ErrPolicyDenied with finding", err, calls)
	}
	// Whitelisted under auto keeps the pre-classifier behavior.
	result, err, calls = run(domain.ApprovalPolicyAuto, domain.PolicyProfileDefault, tools.InvocationMutating, []string{"stub_classifier"})
	if err != nil || calls != 1 || !strings.Contains(result, "ran") {
		t.Fatalf("whitelisted+auto = %q/%v/%d, want executed once", result, err, calls)
	}
}

// TestServiceBashToolEndToEnd drives the full stack — engine, approval
// adapter, real bash backend — over sqlite with the session pinned to the
// 'auto' approval policy, so a classified-safe script runs without any
// approval interrupt and the echoed marker lands in the journal.
func TestServiceBashToolEndToEnd(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash is not available on this host")
	}
	ctx := context.Background()

	backend, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "bash-e2e.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = backend.Close() })

	commands, _ := newBashBackendForTest(t, domain.SandboxModeWorkspaceWrite)
	ts, err := tools.NewRegistry(tools.NewBash(commands)).Resolve([]string{tools.BashName})
	if err != nil {
		t.Fatalf("resolve bash: %v", err)
	}
	checkpoints, err := NewVersionedCheckpointStore(backend.Blobs(), "test-engine")
	if err != nil {
		t.Fatalf("checkpoint store: %v", err)
	}
	eng, err := NewEngine(ctx, NewScriptedModel(
		schema.AssistantMessage("", []schema.ToolCall{{
			ID:       "call-bash-e2e",
			Function: schema.FunctionCall{Name: tools.BashName, Arguments: `{"command":"echo vivy_bash_e2e"}`},
		}}),
		schema.AssistantMessage("Done: bash echoed the marker.", nil),
	), ts, EngineConfig{StreamBuffer: 8, MaxEventPayloadBytes: 64 << 10, Checkpoints: checkpoints})
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}
	svc := NewService(eng, "scripted", "scripted-v0", ServiceDeps{
		Journal: backend, Runs: backend, Messages: backend, Notes: backend, Approvals: backend,
		Questions:          backend,
		Sessions:           backend,
		ApprovalExpiration: 5 * time.Minute, Sink: newTestSink(),
	})
	if err := backend.CreateSession(ctx, domain.Session{ID: "sess-bash-e2e", Title: "bash e2e", CreatedAt: time.Now().UnixMilli()}); err != nil {
		t.Fatalf("create session: %v", err)
	}
	if err := backend.UpdateSandboxPolicy(ctx, "sess-bash-e2e", domain.SandboxModeWorkspaceWrite, domain.ApprovalPolicyAuto); err != nil {
		t.Fatalf("set session approval policy: %v", err)
	}

	runID, err := svc.Run(ctx, "sess-bash-e2e", "run a safe shell command")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	waitForRunStatus(t, backend, runID, domain.RunCompleted)

	events := replayAll(t, backend, runID)
	if i := indexOfType(events, domain.EventToolApprovalRequired); i >= 0 {
		t.Fatalf("safe script under auto policy raised an approval interrupt at %d", i)
	}
	if indexOfType(events, domain.EventToolRequested) < 0 || indexOfType(events, domain.EventToolFinished) < 0 {
		t.Fatalf("journal missing tool lifecycle events: %v", events)
	}
	var blob bytes.Buffer
	for _, ev := range events {
		blob.Write(ev.Payload)
		blob.WriteString("\n")
	}
	if !strings.Contains(blob.String(), "vivy_bash_e2e") {
		t.Fatalf("journal lost the echoed marker; events: %s", blob.String())
	}
}

// TestServiceBashToolBackgroundEndToEnd drives the full background path:
// a scripted model launches bash with run_in_background, polls the fresh
// job with job_output (readonly, auto-run), and closes the run. The job id
// is deterministic (first job of a fresh registry) so the script can name
// it; the marker must reach the journal through job_output.
func TestServiceBashToolBackgroundEndToEnd(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash is not available on this host")
	}
	ctx := context.Background()

	backend, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "bash-job-e2e.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = backend.Close() })

	commands, _ := newBashBackendForTest(t, domain.SandboxModeWorkspaceWrite)
	ts, err := tools.NewRegistry(
		tools.NewBash(commands), tools.NewJobOutput(commands), tools.NewJobKill(commands),
	).Resolve([]string{tools.BashName, tools.JobOutputName, tools.JobKillName})
	if err != nil {
		t.Fatalf("resolve job tools: %v", err)
	}
	checkpoints, err := NewVersionedCheckpointStore(backend.Blobs(), "test-engine")
	if err != nil {
		t.Fatalf("checkpoint store: %v", err)
	}
	jobOutput := func() *schema.Message {
		return schema.AssistantMessage("", []schema.ToolCall{{
			ID:       "call-job-read",
			Function: schema.FunctionCall{Name: tools.JobOutputName, Arguments: `{"job_id":"job_000001"}`},
		}})
	}
	eng, err := NewEngine(ctx, NewScriptedModel(
		schema.AssistantMessage("", []schema.ToolCall{{
			ID:       "call-bash-bg",
			Function: schema.FunctionCall{Name: tools.BashName, Arguments: `{"command":"echo vivy_bg_e2e","run_in_background":true}`},
		}}),
		jobOutput(), jobOutput(), jobOutput(),
		schema.AssistantMessage("Done: the background job echoed the marker.", nil),
	), ts, EngineConfig{StreamBuffer: 8, MaxEventPayloadBytes: 64 << 10, Checkpoints: checkpoints})
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}
	svc := NewService(eng, "scripted", "scripted-v0", ServiceDeps{
		Journal: backend, Runs: backend, Messages: backend, Notes: backend, Approvals: backend,
		Questions:          backend,
		Sessions:           backend,
		ApprovalExpiration: 5 * time.Minute, Sink: newTestSink(),
	})
	if err := backend.CreateSession(ctx, domain.Session{ID: "sess-bash-job-e2e", Title: "bash job e2e", CreatedAt: time.Now().UnixMilli()}); err != nil {
		t.Fatalf("create session: %v", err)
	}
	if err := backend.UpdateSandboxPolicy(ctx, "sess-bash-job-e2e", domain.SandboxModeWorkspaceWrite, domain.ApprovalPolicyAuto); err != nil {
		t.Fatalf("set session approval policy: %v", err)
	}

	runID, err := svc.Run(ctx, "sess-bash-job-e2e", "start a background job and watch it")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	waitForRunStatus(t, backend, runID, domain.RunCompleted)

	events := replayAll(t, backend, runID)
	if i := indexOfType(events, domain.EventToolApprovalRequired); i >= 0 {
		t.Fatalf("background flow raised an approval interrupt at %d", i)
	}
	var blob bytes.Buffer
	for _, ev := range events {
		blob.Write(ev.Payload)
		blob.WriteString("\n")
	}
	journal := blob.String()
	if !strings.Contains(journal, "job_000001") {
		t.Fatalf("journal never carried the deterministic job id; events: %s", journal)
	}
	if !strings.Contains(journal, "vivy_bg_e2e") {
		t.Fatalf("journal lost the background marker; events: %s", journal)
	}
}
