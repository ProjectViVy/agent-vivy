package runtime

// ND-1 (docs/plans/nudge/ND-1.md): the §5 classification allowlist at the
// common governed adapter — typed ArgError/not_found/command_failed/
// remote_tool_error records become model-visible results inside one Run
// while cancellation, native interrupts and unlisted errors keep their
// fatal path. The service-level tests drive Host.Invoke and governedTool
// through the contract harness, not only the classifier constructor.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	einotool "github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/mcphost"
	"agent-vivy/internal/tools"
)

// ---------------------------------------------------------------------------
// §5 classification table
// ---------------------------------------------------------------------------

func TestToolFailureClassification(t *testing.T) {
	readonly := domain.ToolSpec{Name: "reader", Readonly: true}
	write := domain.ToolSpec{Name: "writer", Readonly: false}
	canceledCtx, cancel := context.WithCancel(context.Background())
	cancel()
	interruptErr := einotool.Interrupt(context.Background(), "approval needed")
	if _, ok := compose.IsInterruptRerunError(interruptErr); !ok {
		t.Fatalf("test could not fabricate an interrupt error: %v", interruptErr)
	}

	cases := []struct {
		name string
		ctx  context.Context
		spec domain.ToolSpec
		err  error
		want *toolFailure
	}{
		{
			name: "invocation ArgError is recoverable invalid_arguments",
			spec: readonly,
			err:  &tools.ArgError{Field: "text", Reason: "must be a string"},
			want: &toolFailure{Status: toolFailureStatusRecoverable, Reason: toolFailureReasonInvalidArguments, Effects: toolEffectsUnknown},
		},
		{
			name: "wrapped ArgError keeps unwrap",
			spec: readonly,
			err:  fmt.Errorf("decode arguments: %w", &tools.ArgError{Field: "path", Reason: "required"}),
			want: &toolFailure{Status: toolFailureStatusRecoverable, Reason: toolFailureReasonInvalidArguments, Effects: toolEffectsUnknown},
		},
		{
			name: "readonly invocation wrapping fs.ErrNotExist is not_found",
			spec: readonly,
			err:  fmt.Errorf("read %q: %w", "/tmp/gone", fs.ErrNotExist),
			want: &toolFailure{Status: toolFailureStatusRecoverable, Reason: toolFailureReasonNotFound, Effects: toolEffectsNone},
		},
		{
			name: "write invocation wrapping fs.ErrNotExist stays fatal",
			spec: write,
			err:  fmt.Errorf("write %q: %w", "/tmp/gone", fs.ErrNotExist),
		},
		{
			name: "parent ctx cancelled plus ArgError keeps the cancellation",
			ctx:  canceledCtx,
			spec: readonly,
			err:  &tools.ArgError{Field: "text", Reason: "bad"},
		},
		{
			name: "context.Canceled is not a failure",
			spec: readonly,
			err:  context.Canceled,
		},
		{
			name: "context.DeadlineExceeded is not a failure",
			spec: readonly,
			err:  context.DeadlineExceeded,
		},
		{
			name: "native interrupt is not a failure",
			spec: readonly,
			err:  interruptErr,
		},
		{
			name: "MCP IsError result is remote_tool_error",
			spec: readonly,
			err:  &mcphost.ToolExecutionError{Text: "remote said no"},
			want: &toolFailure{Status: toolFailureStatusRecoverable, Reason: toolFailureReasonRemoteTool, Effects: toolEffectsUnknown},
		},
		{
			name: "unknown transport error stays fatal",
			spec: readonly,
			err:  errors.New("connection reset by peer"),
		},
		{
			name: "storage failure stays fatal",
			spec: readonly,
			err:  fmt.Errorf("journal append: %w", errors.New("disk full")),
		},
		{
			name: "nil is not a failure",
			spec: readonly,
			err:  nil,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := tc.ctx
			if ctx == nil {
				ctx = context.Background()
			}
			got, ok := classifyToolFailure(ctx, tc.spec, tc.err)
			if tc.want == nil {
				if ok {
					t.Fatalf("classified %+v into %+v, want unlisted cause", tc.err, got)
				}
				return
			}
			if !ok {
				t.Fatalf("%v classified fatal, want %+v", tc.err, *tc.want)
			}
			if got.Status != tc.want.Status || got.Reason != tc.want.Reason || got.Effects != tc.want.Effects {
				t.Fatalf("classification = %+v, want %+v", got, *tc.want)
			}
			if got.Diagnostic == "" {
				t.Fatal("classified failure lost its diagnostic")
			}
		})
	}
}

// The mark seam is gated on a bound leg: outside a model leg nothing is
// published, inside one a missing call id is the §5 invariant failure.
func TestToolFailureMarkGatesOnBoundLeg(t *testing.T) {
	failure := toolFailure{Status: toolFailureStatusRecoverable, Reason: toolFailureReasonInvalidArguments, Diagnostic: "bad args", Effects: toolEffectsUnknown}
	if err := markInvocationFailure(context.Background(), failure); err != nil {
		t.Fatalf("unbound mark = %v, want silent no-op", err)
	}
	ctx := withNudgeState(context.Background(), newNudgeState())
	if err := markInvocationFailure(ctx, failure); err == nil {
		t.Fatal("bound mark with no call id must fail closed")
	}
}

// ---------------------------------------------------------------------------
// service-level correction (Host.Invoke → governedTool → adapter → model)
// ---------------------------------------------------------------------------

// notFoundReadTool is a readonly tool whose invocation loses its path:
// fs.ErrNotExist stays wrapped so the classifier sees not_found.
type notFoundReadTool struct{ calls int }

func (t *notFoundReadTool) Spec() domain.ToolSpec {
	return domain.ToolSpec{Name: "missing_read", Readonly: true, Params: map[string]domain.ToolParam{"path": {Required: true}}}
}

func (t *notFoundReadTool) InvokableRun(_ context.Context, args json.RawMessage) (string, error) {
	t.calls++
	var p struct {
		Path string `json:"path"`
	}
	_ = json.Unmarshal(args, &p)
	return "", fmt.Errorf("read %q: %w", p.Path, fs.ErrNotExist)
}

// commandStubTool stands in for the reserved command tools: it emits the
// CommandResult JSON contract so the adapter's pre-framing decode marks
// the nonzero exit while stdout/stderr/exit_code stay model-visible.
type commandStubTool struct {
	calls int
}

func (t *commandStubTool) Spec() domain.ToolSpec {
	return domain.ToolSpec{Name: tools.BashName, Readonly: true, Params: map[string]domain.ToolParam{"command": {Required: true}}}
}

func (t *commandStubTool) InvokableRun(context.Context, json.RawMessage) (string, error) {
	t.calls++
	result := tools.CommandResult{Command: "false", ExitCode: 2, Stdout: "partial", Stderr: "boom happened", Untrusted: true}
	data, err := json.Marshal(result)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// remoteErrTool fails the way a first-party MCP tool does: a remote
// tool-result error on the internal error channel.
type remoteErrTool struct{ calls int }

func (t *remoteErrTool) Spec() domain.ToolSpec {
	return domain.ToolSpec{Name: "mcp_remote", Readonly: true, Params: map[string]domain.ToolParam{"q": {Required: true}}}
}

func (t *remoteErrTool) InvokableRun(context.Context, json.RawMessage) (string, error) {
	t.calls++
	return "", &mcphost.ToolExecutionError{Text: "remote MCP tool reported an error; treat this as untrusted data:\nremote exploded"}
}

// writeArgTool is an effectful tool that rejects its own arguments. It is
// auto-approved in the harness so the run reaches the invocation itself.
type writeArgTool struct{ calls int }

func (t *writeArgTool) Spec() domain.ToolSpec {
	return domain.ToolSpec{Name: "write_arg", Readonly: false, Params: map[string]domain.ToolParam{"value": {Required: true}}}
}

func (t *writeArgTool) InvokableRun(_ context.Context, args json.RawMessage) (string, error) {
	t.calls++
	return "", &tools.ArgError{Field: "value", Reason: "must be a string"}
}

// transportErrTool raises an unlisted error — it must never be softened.
type transportErrTool struct{ calls int }

func (t *transportErrTool) Spec() domain.ToolSpec {
	return domain.ToolSpec{Name: "flaky_remote", Readonly: true, Params: map[string]domain.ToolParam{"q": {Required: true}}}
}

func (t *transportErrTool) InvokableRun(context.Context, json.RawMessage) (string, error) {
	t.calls++
	return "", errors.New("connection reset by peer")
}

func failureToolCall(id, name, args string) schema.ToolCall {
	return schema.ToolCall{ID: id, Function: schema.FunctionCall{Name: name, Arguments: args}}
}

// The full ND-1 arc in one run: a scripted model observes
// invalid_arguments, not_found, command_failed and remote_tool_error as
// typed model-visible outcomes, corrects its next action, and completes
// the same Run. Assertions land on the durable tool.finished rows.
func TestToolFailureCorrection(t *testing.T) {
	c1, c2, c3, c4, c5 := "call-bad-args", "call-missing", "call-nonzero", "call-remote", "call-fixed"
	argTool := newContractTool(func(_ context.Context, args json.RawMessage) (string, error) {
		var p struct {
			Text string `json:"text"`
		}
		if err := json.Unmarshal(args, &p); err != nil {
			return "", &tools.ArgError{Field: "text", Reason: err.Error()}
		}
		if p.Text == "bad" {
			return "", &tools.ArgError{Field: "text", Reason: "must be a non-empty string"}
		}
		return "ok-" + p.Text, nil
	})
	nfTool := &notFoundReadTool{}
	cmdTool := &commandStubTool{}
	remoteTool := &remoteErrTool{}
	script := []*schema.Message{
		schema.AssistantMessage("", []schema.ToolCall{
			failureToolCall(c1, contractToolName, `{"text":"bad"}`),
			failureToolCall(c2, "missing_read", `{"path":"/tmp/gone"}`),
			failureToolCall(c3, tools.BashName, `{"command":"false"}`),
			failureToolCall(c4, "mcp_remote", `{"q":"x"}`),
		}),
		// The model corrects the failed call and finishes in the same Run.
		schema.AssistantMessage("", []schema.ToolCall{failureToolCall(c5, contractToolName, `{"text":"fixed"}`)}),
		schema.AssistantMessage("recovered", nil),
	}
	h := newContractHarness(t, script, contractHarnessOpts{streaming: true, extraTools: []tools.Tool{argTool, nfTool, cmdTool, remoteTool}})

	runID, err := h.svc.Run(context.Background(), "sess-failure-correction", "run the failing tools")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	waitForRunStatus(t, h.backend, runID, domain.RunCompleted)

	assertFinished := func(callID, reason, effects string) payloadToolFinished {
		p := h.journal.finishedPayload(t, callID)
		if p.Outcome != toolFailureStatusRecoverable {
			t.Fatalf("%s outcome = %q, want recoverable", callID, p.Outcome)
		}
		if p.Reason != reason || p.Effects != effects {
			t.Fatalf("%s failure = %s/%s, want %s/%s", callID, p.Reason, p.Effects, reason, effects)
		}
		return p
	}
	if p := assertFinished(c1, toolFailureReasonInvalidArguments, toolEffectsUnknown); !strings.Contains(p.Error, "non-empty string") {
		t.Fatalf("%s diagnostic = %q, want the ArgError text", c1, p.Error)
	}
	assertFinished(c2, toolFailureReasonNotFound, toolEffectsNone)
	if p := assertFinished(c3, toolFailureReasonCommandFailed, toolEffectsUnknown); !strings.Contains(p.Result, `"exit_code":2`) {
		t.Fatalf("%s result lost the CommandResult fields: %q", c3, p.Result)
	}
	assertFinished(c4, toolFailureReasonRemoteTool, toolEffectsUnknown)
	if p := h.journal.finishedPayload(t, c5); p.Outcome != "" || !strings.Contains(p.Result, "ok-fixed") {
		t.Fatalf("corrected call %s finished with %+v, want a clean success", c5, p)
	}
	if argTool.snapshot()[0].callID != c1 || argTool.snapshot()[1].callID != c5 {
		t.Fatalf("arg tool call ids = %+v", argTool.snapshot())
	}
	if nfTool.calls != 1 || cmdTool.calls != 1 || remoteTool.calls != 1 {
		t.Fatalf("invocation counts nf=%d cmd=%d remote=%d, want 1 each", nfTool.calls, cmdTool.calls, remoteTool.calls)
	}
	// The diagnostics were model-visible: the correction turn's input
	// carries each failed call's result text.
	tail := toolResults(h.model.entry(1).input)
	joined := ""
	for _, m := range tail {
		joined += m.Content + "\n"
	}
	for _, want := range []string{"non-empty string", "gone", "boom happened", "remote exploded"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("correction input missing diagnostic %q:\n%s", want, joined)
		}
	}
}

// A denied write is a refusal: the invocation never executes, the durable
// row is refused/policy_denied/not_executed, and the run completes.
func TestToolFailureRefusalNeverExecutes(t *testing.T) {
	d1 := "call-denied"
	denied := &classifierStubTool{class: tools.InvocationDenied, findings: []string{"deny-table: host shell or interpreter escape"}}
	script := []*schema.Message{
		schema.AssistantMessage("", []schema.ToolCall{failureToolCall(d1, "stub_classifier", `{"command":"cmd /c dir"}`)}),
		schema.AssistantMessage("done without it", nil),
	}
	h := newContractHarness(t, script, contractHarnessOpts{streaming: true, extraTools: []tools.Tool{denied}})
	runID, err := h.svc.Run(context.Background(), "sess-failure-refusal", "run the denied tool")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	waitForRunStatus(t, h.backend, runID, domain.RunCompleted)

	p := h.journal.finishedPayload(t, d1)
	if p.Outcome != toolFailureStatusRefused || p.Reason != toolFailureReasonPolicyDenied || p.Effects != toolEffectsNotExecuted {
		t.Fatalf("%s refusal row = %s/%s/%s, want refused/policy_denied/not_executed", d1, p.Outcome, p.Reason, p.Effects)
	}
	if !strings.Contains(p.Result, "did not run") {
		t.Fatalf("%s refusal result = %q", d1, p.Result)
	}
	if denied.calls != 0 {
		t.Fatalf("denied write executed %d times, want zero", denied.calls)
	}
}

// An invocation whose arguments fail carries effects=unknown: the record
// cannot prove the tool did nothing, so nothing may assume it. The model
// saw the diagnostic and did not blindly retry — the count stays one.
func TestToolFailureUnknownEffectsCountOnce(t *testing.T) {
	w1 := "call-write"
	writeTool := &writeArgTool{}
	script := []*schema.Message{
		schema.AssistantMessage("", []schema.ToolCall{failureToolCall(w1, "write_arg", `{"value":"x"}`)}),
		schema.AssistantMessage("stopping here", nil),
	}
	h := newContractHarness(t, script, contractHarnessOpts{streaming: true, extraTools: []tools.Tool{writeTool}, autoApprove: []string{"write_arg"}})
	// The write tool only reaches its invocation under an auto-approve
	// session profile — without it the call interrupts for approval.
	if err := h.backend.CreateSession(context.Background(), domain.Session{
		ID:             "sess-failure-write",
		Title:          "write failure",
		CreatedAt:      1,
		SandboxMode:    string(domain.SandboxModeWorkspaceWrite),
		ApprovalPolicy: string(domain.ApprovalPolicyAuto),
	}); err != nil {
		t.Fatalf("create session: %v", err)
	}
	runID, err := h.svc.Run(context.Background(), "sess-failure-write", "run the write tool")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	waitForRunStatus(t, h.backend, runID, domain.RunCompleted)

	p := h.journal.finishedPayload(t, w1)
	if p.Outcome != toolFailureStatusRecoverable || p.Reason != toolFailureReasonInvalidArguments || p.Effects != toolEffectsUnknown {
		t.Fatalf("%s row = %s/%s/%s, want recoverable/invalid_arguments/unknown", w1, p.Outcome, p.Reason, p.Effects)
	}
	if writeTool.calls != 1 {
		t.Fatalf("unknown-effects invocation ran %d times, want exactly one", writeTool.calls)
	}
}

// An unlisted invocation error (transport) stays fatal: no soft
// conversion, no fabricated result, the run fails.
func TestToolFailureTransportErrorStaysFatal(t *testing.T) {
	t1 := "call-transport"
	flaky := &transportErrTool{}
	script := []*schema.Message{
		schema.AssistantMessage("", []schema.ToolCall{failureToolCall(t1, "flaky_remote", `{"q":"x"}`)}),
		schema.AssistantMessage("should never arrive", nil),
	}
	h := newContractHarness(t, script, contractHarnessOpts{streaming: true, extraTools: []tools.Tool{flaky}})
	runID, err := h.svc.Run(context.Background(), "sess-failure-fatal", "run the flaky tool")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	waitForRunStatus(t, h.backend, runID, domain.RunFailed)
	if h.journal.indexOf(domain.EventToolFinished, t1) >= 0 {
		p := h.journal.finishedPayload(t, t1)
		if p.Outcome != "" {
			t.Fatalf("fatal error was soft-converted: finished row %+v", p)
		}
	}
	if flaky.calls != 1 {
		t.Fatalf("fatal invocation ran %d times, want 1", flaky.calls)
	}
}

// Reported incident (run_8d376d0f457fa5e5): under a read-only sandbox the
// model probes the run workspace with list_dir. Both "." and "./" must list
// the root, and a genuinely bad path must come back as a recoverable
// invalid_arguments result instead of failing the whole Run through Eino's
// ToolsNode ("failed to stream tool call ... filesystem: invalid path").
func TestFilesystemPathMistakeRecoversInsideRun(t *testing.T) {
	manager, err := NewWorkspaceManager(filepath.Join(t.TempDir(), "workspaces"))
	if err != nil {
		t.Fatalf("workspace manager: %v", err)
	}
	sandbox, err := NewSandboxManager(domain.SandboxModeReadOnly, manager.root, nil, nil)
	if err != nil {
		t.Fatalf("sandbox manager: %v", err)
	}
	backend := NewEinoFilesystemBackend(manager, sandbox)

	rootCall, badCall := "call-list-root", "call-list-rooted"
	script := []*schema.Message{
		schema.AssistantMessage("", []schema.ToolCall{
			failureToolCall(rootCall, tools.ListDirName, `{"path":"./"}`),
			failureToolCall(badCall, tools.ListDirName, `{"path":"/"}`),
		}),
		schema.AssistantMessage("corrected", nil),
	}
	h := newContractHarness(t, script, contractHarnessOpts{streaming: true, extraTools: []tools.Tool{tools.NewListDir(backend)}})
	runID, err := h.svc.Run(context.Background(), "sess-fs-path-recovery", "list the workspace")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	waitForRunStatus(t, h.backend, runID, domain.RunCompleted)

	if p := h.journal.finishedPayload(t, rootCall); p.Outcome != "" || !strings.Contains(p.Result, `"path":"."`) {
		t.Fatalf("list_dir ./ = %+v, want a clean root listing", p)
	}
	p := h.journal.finishedPayload(t, badCall)
	if p.Outcome != toolFailureStatusRecoverable || p.Reason != toolFailureReasonInvalidArguments {
		t.Fatalf("list_dir / = %s/%s, want recoverable/invalid_arguments", p.Outcome, p.Reason)
	}
	if !strings.Contains(p.Error, "workspace-relative") {
		t.Fatalf("list_dir / diagnostic = %q, want the model-visible reason", p.Error)
	}
}

// ---------------------------------------------------------------------------
// service-level: argument defects and boundary denials
// ---------------------------------------------------------------------------

// filesystemRunFixture is a real filesystem backend over one fixed local
// workspace, so a service-level Run drives the real read_file/patch tools and
// their real proposal builders.
type filesystemRunFixture struct {
	backend *EinoFilesystemBackend
	root    string
}

func newFilesystemRunFixture(t *testing.T, files map[string]string) *filesystemRunFixture {
	t.Helper()
	root := t.TempDir()
	for name, content := range files {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatalf("create fixture dir: %v", err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatalf("write fixture %s: %v", name, err)
		}
	}
	manager, err := NewLocalWorkspaceManager(root)
	if err != nil {
		t.Fatalf("new local workspace manager: %v", err)
	}
	sandbox, err := NewSandboxManager(domain.SandboxModeWorkspaceWrite, root, nil, nil)
	if err != nil {
		t.Fatalf("new sandbox manager: %v", err)
	}
	return &filesystemRunFixture{backend: NewEinoFilesystemBackend(manager, sandbox), root: root}
}

func (f *filesystemRunFixture) read(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(f.root, name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return string(data)
}

func assertNoPendingApproval(t *testing.T, h *contractHarness, callID string) {
	t.Helper()
	pending, err := h.backend.ListPendingApprovals(context.Background())
	if err != nil {
		t.Fatalf("list pending approvals: %v", err)
	}
	if len(pending) != 0 {
		t.Fatalf("pending approvals = %+v, want none: the call cannot be built", pending)
	}
	if h.journal.indexOf(domain.EventToolApprovalRequired, callID) >= 0 {
		t.Fatalf("call %s raised an approval prompt for an argument defect", callID)
	}
}

// The default `smart` preset (workspace_write + approval ask) reviews every
// effectful call before it runs. A patch whose old_string matches nothing can
// never be approved, so the pre-flight must hand the model the recoverable
// invalid_arguments result instead of suspending for a review that cannot be
// built — the failure that used to end the run with "The run could not be
// paused for approval."
func TestPatchArgumentMistakeRecoversWithoutApprovalUnderAsk(t *testing.T) {
	fixture := newFilesystemRunFixture(t, map[string]string{"notes.txt": "alpha\nbeta\n"})

	badCall, missingCall := "call-patch-unmatched", "call-patch-missing"
	script := []*schema.Message{
		schema.AssistantMessage("", []schema.ToolCall{
			failureToolCall(badCall, tools.PatchName, `{"path":"notes.txt","old_string":"gamma","new_string":"x"}`),
			failureToolCall(missingCall, tools.PatchName, `{"path":"absent.txt","old_string":"alpha","new_string":"x"}`),
		}),
		schema.AssistantMessage("corrected", nil),
	}
	h := newContractHarness(t, script, contractHarnessOpts{streaming: true, checkpoints: true, extraTools: []tools.Tool{tools.NewPatch(fixture.backend)}})
	mustCreateSession(t, h.backend, "sess-patch-ask")

	run, err := h.svc.Run(context.Background(), "sess-patch-ask", "patch the notes")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	waitForRunStatus(t, h.backend, run, domain.RunCompleted)

	p := h.journal.finishedPayload(t, badCall)
	if p.Outcome != toolFailureStatusRecoverable || p.Reason != toolFailureReasonInvalidArguments {
		t.Fatalf("patch outcome = %s/%s (%q), want recoverable/invalid_arguments", p.Outcome, p.Reason, p.Error)
	}
	if !strings.Contains(p.Error, "was not found") {
		t.Fatalf("patch diagnostic = %q, want the model-visible reason", p.Error)
	}
	assertNoPendingApproval(t, h, badCall)
	if got := fixture.read(t, "notes.txt"); got != "alpha\nbeta\n" {
		t.Fatalf("notes.txt = %q, want the target untouched", got)
	}
	// A patch of a path that does not exist is the same class of mistake: a
	// patch cannot create the file, so the model must be told to write it.
	missing := h.journal.finishedPayload(t, missingCall)
	if missing.Outcome != toolFailureStatusRecoverable || missing.Reason != toolFailureReasonInvalidArguments {
		t.Fatalf("missing target = %s/%s (%q), want recoverable/invalid_arguments", missing.Outcome, missing.Reason, missing.Error)
	}
	if !strings.Contains(missing.Error, "does not exist") {
		t.Fatalf("missing-target diagnostic = %q", missing.Error)
	}
	assertNoPendingApproval(t, h, missingCall)
}

// The same argument defect must also recover on the invocation seam, so an
// auto-approved (or non-interactive) call cannot fail the run either.
func TestPatchArgumentMistakeRecoversWhenAutoApproved(t *testing.T) {
	fixture := newFilesystemRunFixture(t, map[string]string{"notes.txt": "alpha\nbeta\n"})

	badCall := "call-patch-auto"
	script := []*schema.Message{
		schema.AssistantMessage("", []schema.ToolCall{
			failureToolCall(badCall, tools.PatchName, `{"path":"notes.txt","old_string":"gamma","new_string":"x"}`),
		}),
		schema.AssistantMessage("corrected", nil),
	}
	h := newContractHarness(t, script, contractHarnessOpts{
		streaming:   true,
		extraTools:  []tools.Tool{tools.NewPatch(fixture.backend)},
		autoApprove: []string{tools.PatchName},
	})
	mustCreateSession(t, h.backend, "sess-patch-auto")
	if err := h.backend.UpdateSandboxPolicy(context.Background(), "sess-patch-auto", domain.SandboxModeWorkspaceWrite, domain.ApprovalPolicyAuto); err != nil {
		t.Fatalf("update sandbox policy: %v", err)
	}

	run, err := h.svc.Run(context.Background(), "sess-patch-auto", "patch the notes")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	waitForRunStatus(t, h.backend, run, domain.RunCompleted)

	p := h.journal.finishedPayload(t, badCall)
	if p.Outcome != toolFailureStatusRecoverable || p.Reason != toolFailureReasonInvalidArguments {
		t.Fatalf("patch outcome = %s/%s (%q), want recoverable/invalid_arguments", p.Outcome, p.Reason, p.Error)
	}
	if got := fixture.read(t, "notes.txt"); got != "alpha\nbeta\n" {
		t.Fatalf("notes.txt = %q, want the target untouched", got)
	}
}

// A protected path is a governance denial: the model is told the call did not
// run, and the run continues (refused/policy_denied), on a read-only tool that
// needs no approval.
func TestProtectedPathReadIsRefusedInsideRun(t *testing.T) {
	fixture := newFilesystemRunFixture(t, map[string]string{".env": "SECRET=1", "notes.txt": "alpha\n"})

	call := "call-read-env"
	script := []*schema.Message{
		schema.AssistantMessage("", []schema.ToolCall{failureToolCall(call, tools.ReadFileName, `{"path":".env"}`)}),
		schema.AssistantMessage("understood", nil),
	}
	h := newContractHarness(t, script, contractHarnessOpts{streaming: true, extraTools: []tools.Tool{tools.NewReadFile(fixture.backend)}})
	mustCreateSession(t, h.backend, "sess-read-env")

	run, err := h.svc.Run(context.Background(), "sess-read-env", "read the env file")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	waitForRunStatus(t, h.backend, run, domain.RunCompleted)

	p := h.journal.finishedPayload(t, call)
	if p.Outcome != toolFailureStatusRefused || p.Reason != toolFailureReasonPolicyDenied {
		t.Fatalf("protected read = %s/%s (%q), want refused/policy_denied", p.Outcome, p.Reason, p.Error)
	}
	if strings.Contains(p.Result, "SECRET=1") {
		t.Fatalf("protected content reached the model: %q", p.Result)
	}
	assertNoPendingApproval(t, h, call)
}

// The §5 allowlist is an allowlist: infrastructure and lifecycle errors keep
// their fatal path, and a missing target is only recovered for a readonly tool.
func TestToolFailureAllowlistRejectsInfrastructureErrors(t *testing.T) {
	ctx := context.Background()
	effectful := domain.ToolSpec{Name: tools.PatchName}
	readonly := domain.ToolSpec{Name: tools.ReadFileName, Readonly: true}

	unhandled := []error{
		errors.New("boom"),
		context.Canceled,
		context.DeadlineExceeded,
		fmt.Errorf("filesystem: read patch target: %w", fs.ErrNotExist),
		errors.New("runtime: persist tool operation completion: disk full"),
	}
	for _, err := range unhandled {
		if failure, ok := classifyToolFailure(ctx, effectful, err); ok {
			t.Fatalf("effectful %v was soft-converted: %+v", err, failure)
		}
		if _, ok := asToolRefusal(err); ok {
			t.Fatalf("unlisted %v became a refusal", err)
		}
	}
	// A readonly miss is the documented not_found recovery, not a general one.
	failure, ok := classifyToolFailure(ctx, readonly, fmt.Errorf("filesystem: read notes.txt: %w", fs.ErrNotExist))
	if !ok || failure.Reason != toolFailureReasonNotFound || failure.Status != toolFailureStatusRecoverable {
		t.Fatalf("readonly miss = %+v (ok=%v), want recoverable/not_found", failure, ok)
	}
}
