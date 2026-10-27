package runtime

// ND-1 (docs/archive/plans/nudge/ND-1.md): the §5 classification allowlist at the
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
	"net"
	"net/url"
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
			name: "delegated child failure is recoverable child_failed",
			spec: readonly,
			err:  tools.NewChildTaskError("the child model returned HTTP 500"),
			want: &toolFailure{Status: toolFailureStatusRecoverable, Reason: toolFailureReasonChildFailed, Effects: toolEffectsUnknown},
		},
		{
			name: "delegated child failure under parent cancellation stays fatal",
			ctx:  canceledCtx,
			spec: readonly,
			err:  tools.NewChildTaskError("the child was cancelled"),
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

// On a model-driven leg anything outside the runtime-integrity set becomes
// a recoverable result; network-shaped errors keep their own reason.
func TestModelLegToolFailureFallback(t *testing.T) {
	spec := domain.ToolSpec{Name: "reader", Readonly: true}
	canceledCtx, cancel := context.WithCancel(context.Background())
	cancel()
	interruptErr := einotool.Interrupt(context.Background(), "approval needed")
	cases := []struct {
		name string
		ctx  context.Context
		err  error
		want *toolFailure
	}{
		{
			name: "plain error becomes tool_error",
			err:  errors.New("tool panicked"),
			want: &toolFailure{Status: toolFailureStatusRecoverable, Reason: toolFailureReasonToolError, Effects: toolEffectsUnknown},
		},
		{
			name: "url.Error becomes network_failed",
			err:  &url.Error{Op: "Get", URL: "https://example.invalid", Err: errors.New("dial failed")},
			want: &toolFailure{Status: toolFailureStatusRecoverable, Reason: toolFailureReasonNetworkFailed, Effects: toolEffectsUnknown},
		},
		{
			name: "net.Error becomes network_failed",
			err:  &net.DNSError{Err: "no such host", Name: "example.invalid", IsNotFound: true},
			want: &toolFailure{Status: toolFailureStatusRecoverable, Reason: toolFailureReasonNetworkFailed, Effects: toolEffectsUnknown},
		},
		{
			name: "per-call timeout with a live parent becomes network_failed",
			err:  &net.DNSError{Err: "i/o timeout", Name: "example.invalid", IsTimeout: true},
			want: &toolFailure{Status: toolFailureStatusRecoverable, Reason: toolFailureReasonNetworkFailed, Effects: toolEffectsUnknown},
		},
		{
			name: "typed ArgError keeps invalid_arguments",
			err:  &tools.ArgError{Field: "q", Reason: "required"},
			want: &toolFailure{Status: toolFailureStatusRecoverable, Reason: toolFailureReasonInvalidArguments, Effects: toolEffectsUnknown},
		},
		{
			name: "parent cancellation stays unclassified",
			ctx:  canceledCtx,
			err:  errors.New("connection reset by peer"),
		},
		{
			name: "context.Canceled stays unclassified",
			err:  context.Canceled,
		},
		{
			name: "native interrupt stays unclassified",
			err:  interruptErr,
		},
		{
			name: "refusal sentinel stays a refusal",
			err:  fmt.Errorf("%w: host %q is not allowlisted", ErrPolicyDenied, "api.github.com"),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := tc.ctx
			if ctx == nil {
				ctx = context.Background()
			}
			got, ok := classifyModelLegToolFailure(ctx, spec, tc.err)
			if tc.want == nil {
				if ok {
					t.Fatalf("classified %v into %+v, want unclassified", tc.err, got)
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

func failureToolCall(id, name, args string) *schema.FunctionToolCall {
	return &schema.FunctionToolCall{CallID: id, Name: name, Arguments: args}
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
	script := []*schema.AgenticMessage{
		testToolCalls(failureToolCall(c1, contractToolName, `{"text":"bad"}`), failureToolCall(c2, "missing_read", `{"path":"/tmp/gone"}`), failureToolCall(c3, tools.BashName, `{"command":"false"}`), failureToolCall(c4, "mcp_remote", `{"q":"x"}`)),
		// The model corrects the failed call and finishes in the same Run.
		testToolCalls(failureToolCall(c5, contractToolName, `{"text":"fixed"}`)),
		testAssistant("recovered"),
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
		joined += agenticMessageText(m) + "\n"
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
	script := []*schema.AgenticMessage{
		testToolCalls(failureToolCall(d1, "stub_classifier", `{"command":"cmd /c dir"}`)),
		testAssistant("done without it"),
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
	script := []*schema.AgenticMessage{
		testToolCalls(failureToolCall(w1, "write_arg", `{"value":"x"}`)),
		testAssistant("stopping here"),
	}
	h := newContractHarness(t, script, contractHarnessOpts{streaming: true, extraTools: []tools.Tool{writeTool}})
	// The write tool only reaches its invocation under the auto approval
	// policy; under ask the call interrupts for approval.
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

// A plain invocation error on a model-driven leg is a recoverable
// tool_error result: the model sees the diagnostic and the run continues.
func TestToolFailureTransportErrorBecomesToolResult(t *testing.T) {
	t1 := "call-transport"
	flaky := &transportErrTool{}
	script := []*schema.AgenticMessage{
		testToolCalls(failureToolCall(t1, "flaky_remote", `{"q":"x"}`)),
		testAssistant("recovered after the tool error"),
	}
	h := newContractHarness(t, script, contractHarnessOpts{streaming: true, extraTools: []tools.Tool{flaky}})
	runID, err := h.svc.Run(context.Background(), "sess-failure-continuation", "run the flaky tool")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	waitForRunStatus(t, h.backend, runID, domain.RunCompleted)

	p := h.journal.finishedPayload(t, t1)
	if p.Outcome != toolFailureStatusRecoverable || p.Reason != toolFailureReasonToolError || p.Effects != toolEffectsUnknown {
		t.Fatalf("%s row = %s/%s/%s, want recoverable/tool_error/unknown", t1, p.Outcome, p.Reason, p.Effects)
	}
	if !strings.Contains(p.Error, "connection reset by peer") {
		t.Fatalf("%s diagnostic = %q", t1, p.Error)
	}
	if flaky.calls != 1 {
		t.Fatalf("tool error invocation ran %d times, want 1", flaky.calls)
	}
	joined := ""
	for _, m := range toolResults(h.model.entry(1).input) {
		joined += agenticMessageText(m) + "\n"
	}
	if !strings.Contains(joined, "connection reset by peer") {
		t.Fatalf("next model turn missing the tool diagnostic:\n%s", joined)
	}
}

// A tool's own failure is never a provider failure. Its diagnostic may quote
// provider wording or an HTTP status, but the provider answered and the tool
// could not complete its work.
func TestToolInvocationFailureIsNeverAProviderFailure(t *testing.T) {
	cause := fmt.Errorf("engine event error: %w", &toolInvocationError{
		tool: "agent",
		err:  errors.New("API key missing: status code: 500 from the child model"),
	})
	if isProviderFailure(cause) {
		t.Fatal("tool-origin failure classified as a provider failure")
	}
	if _, ok := providerFailureMessage(cause); ok {
		t.Fatal("tool-origin failure produced a provider user message")
	}
	if category := causeCategoryOf(cause); category != causeInternalError {
		t.Fatalf("tool-origin cause category = %q, want %q", category, causeInternalError)
	}
	// The same markers outside a tool invocation keep provider classification,
	// so a genuine outage still reaches the user with the key hint.
	direct := errors.New("API key missing: configure a model API key in Settings")
	if !isProviderFailure(direct) {
		t.Fatal("provider key failure lost its classification")
	}
	if message, ok := providerFailureMessage(direct); !ok || !strings.Contains(message, "API key missing") {
		t.Fatalf("provider key message = %q ok=%v", message, ok)
	}
}
