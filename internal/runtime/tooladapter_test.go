package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/tools"
)

func TestCompactToolResultKeepsHeadTailAndValidUTF8(t *testing.T) {
	result := "HEAD-" + strings.Repeat("中", 64) + "-TAIL"
	got := compactToolResult(result, 96)
	if len(got) > 96 {
		t.Fatalf("compacted result bytes = %d, want <= 96: %q", len(got), got)
	}
	if !strings.HasPrefix(got, "HEAD-") || !strings.HasSuffix(got, "-TAIL") {
		t.Fatalf("compacted result lost head/tail anchors: %q", got)
	}
	if !strings.Contains(got, "tool output collapsed") {
		t.Fatalf("compacted result missing tombstone: %q", got)
	}
}

func TestCompactToolResultLeavesSmallResultsUntouched(t *testing.T) {
	for _, budget := range []int{0, 100} {
		if got := compactToolResult("small", budget); got != "small" {
			t.Fatalf("budget %d changed small result to %q", budget, got)
		}
	}
}

func TestToolAdapterCompactsReadonlyResult(t *testing.T) {
	tool := &longResultTool{}
	adapter := newToolAdapter(tool, 48, nil, nil, nil)
	got, err := adapter.InvokableRun(context.Background(), `{}`)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if len(got) > 48 || !strings.Contains(got, "tool output collapsed") {
		t.Fatalf("adapter result = %q, want bounded tombstone", got)
	}
}

func TestToolAdapterRejectsToolOutsideRunSelection(t *testing.T) {
	tool := &countingTool{}
	adapter := newToolAdapter(tool, 0, nil, nil, nil)
	ctx := withSelectedTools(context.Background(), []string{"another_tool"})
	if _, err := adapter.InvokableRun(ctx, `{}`); err == nil || !strings.Contains(err.Error(), "not selected") {
		t.Fatalf("unselected tool error = %v, want fail-closed rejection", err)
	}
	if tool.calls != 0 {
		t.Fatalf("unselected tool calls = %d, want zero", tool.calls)
	}
}

// A deny-table refusal is recorded as a policy decision at the moment the
// call is refused, so the run inspector shows why it did not run.
func TestToolAdapterDenyTableRefusalEmitsPolicyEvent(t *testing.T) {
	tool := &classifierStubTool{class: tools.InvocationDenied, findings: []string{"deny-table: host shell or interpreter escape"}}
	adapter := newToolAdapter(tool, 0, nil, nil, nil)
	var events []GovernanceEvent
	ctx := withGovernanceEventSink(withPolicyProfile(context.Background(), domain.PolicyProfileDefault),
		func(_ context.Context, ev GovernanceEvent) error {
			events = append(events, ev)
			return nil
		})
	result, err := adapter.InvokableRun(ctx, `{"command":"cmd /c dir"}`)
	if err != nil {
		t.Fatalf("deny-table refusal surfaced a run-failing error: %v", err)
	}
	if tool.calls != 0 {
		t.Fatalf("refused tool calls = %d, want zero", tool.calls)
	}
	if !strings.Contains(result, "did not run") || !strings.Contains(result, "deny-table: host shell or interpreter escape") {
		t.Fatalf("refusal result = %q", result)
	}
	for _, ev := range events {
		if ev.Type == domain.EventPolicyEvaluated && ev.Decision == string(domain.PolicyDeny) && ev.ToolName == "stub_classifier" {
			return
		}
	}
	t.Fatalf("no deny governance event recorded: %+v", events)
}

// A skill_view mount extends the selected surface for the run: a tool
// absent from the base selection executes once mounted.
func TestToolAdapterAllowsMountedTool(t *testing.T) {
	tool := &countingTool{}
	adapter := newToolAdapter(tool, 0, nil, nil, nil)
	mounts := tools.NewMountedTools()
	mounts.Mount(tool.Spec().Name)
	ctx := tools.WithMountedTools(withSelectedTools(context.Background(), []string{"another_tool"}), mounts)
	if _, err := adapter.InvokableRun(ctx, `{"value":"draft"}`); err != nil {
		t.Fatalf("mounted tool run: %v", err)
	}
	if tool.calls != 1 {
		t.Fatalf("mounted tool calls = %d, want 1", tool.calls)
	}
}

// A malformed call is refused to the model instead of failing the run: the
// tool never runs, and the model can correct its own arguments.
func TestToolAdapterRefusesInvalidSchemaBeforeInvocation(t *testing.T) {
	tool := &countingTool{}
	adapter := newToolAdapter(tool, 0, nil, nil, nil)
	ctx := withSelectedTools(context.Background(), []string{tool.Spec().Name})
	result, err := adapter.InvokableRun(ctx, `{}`)
	if err != nil {
		t.Fatalf("missing required argument failed the run: %v", err)
	}
	if !strings.Contains(result, "did not run") || !strings.Contains(result, "required") {
		t.Fatalf("refusal result = %q", result)
	}
	if tool.calls != 0 {
		t.Fatalf("invalid argument calls = %d, want zero", tool.calls)
	}
}

type rawSchemaTool struct{ schema json.RawMessage }

func (tool rawSchemaTool) Spec() domain.ToolSpec {
	return domain.ToolSpec{Name: "raw_schema", Readonly: true, Schema: append(json.RawMessage(nil), tool.schema...)}
}

func (rawSchemaTool) InvokableRun(context.Context, json.RawMessage) (string, error) {
	return "ok", nil
}

func TestToolAdapterInfoPreservesUnsupportedJSONSchemaKeywords(t *testing.T) {
	const raw = `{
		"$schema":"https://json-schema.org/draft/2020-12/schema",
		"type":"object",
		"properties":{"value":{"type":"integer","minimum":9007199254740993}},
		"required":["value"],
		"unevaluatedProperties":false
	}`
	info, err := newToolAdapter(rawSchemaTool{schema: json.RawMessage(raw)}, 0, nil, nil, nil).Info(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if info.ParamsOneOf == nil {
		t.Fatal("raw schema tool lost ParamsOneOf")
	}
	schema, err := info.ParamsOneOf.ToJSONSchema()
	if err != nil {
		t.Fatal(err)
	}
	got, err := json.Marshal(schema)
	if err != nil {
		t.Fatal(err)
	}
	var gotValue, wantValue any
	if err := json.Unmarshal(got, &gotValue); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(raw), &wantValue); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(gotValue, wantValue) {
		t.Fatalf("Eino model schema lost fields\n got: %s\nwant: %s", got, raw)
	}
}

// Plan mode refuses effectful calls to the model rather than failing the run:
// exploring in plan mode is normal model behavior.
func TestToolAdapterPlanModeRefusesEffectfulToolBeforeApproval(t *testing.T) {
	tool := &planCountingTool{}
	adapter := newToolAdapter(tool, 0, nil, nil, nil)
	ctx := withRunMode(withSelectedTools(context.Background(), []string{tool.Spec().Name}), domain.RunModePlan)
	result, err := adapter.InvokableRun(ctx, `{"value":"draft"}`)
	if err != nil {
		t.Fatalf("plan mode refusal failed the run: %v", err)
	}
	if !strings.Contains(result, "did not run") || !strings.Contains(result, "plan mode") {
		t.Fatalf("plan mode refusal result = %q", result)
	}
	if tool.calls != 0 {
		t.Fatalf("plan mode effectful calls = %d, want zero", tool.calls)
	}
}

// The 'never' approval policy denies effectful calls per call; the run stays
// alive so the model can answer without them.
func TestToolAdapterApprovalPolicyNeverRefusesEffectful(t *testing.T) {
	tool := &planCountingTool{}
	adapter := newToolAdapter(tool, 0, nil, nil, nil)
	ctx := withPolicyProfile(
		withSessionSandbox(withSelectedTools(context.Background(), []string{tool.Spec().Name}), domain.SandboxModeWorkspaceWrite, domain.ApprovalPolicyNever),
		domain.PolicyProfileFullAuto,
	)
	result, err := adapter.InvokableRun(ctx, `{"value":"draft"}`)
	if err != nil {
		t.Fatalf("'never' policy refusal failed the run: %v", err)
	}
	if !strings.Contains(result, "did not run") || !strings.Contains(result, "never") {
		t.Fatalf("never policy refusal result = %q", result)
	}
	if tool.calls != 0 {
		t.Fatalf("never policy calls = %d, want zero", tool.calls)
	}
}

func TestToolAdapterApprovalPolicyAutoAllowlistsEffectful(t *testing.T) {
	tool := &planCountingTool{}
	adapter := newToolAdapter(tool, 0, nil, nil, []string{"plan_write"})
	ctx := withSessionSandbox(withSelectedTools(context.Background(), []string{tool.Spec().Name}), domain.SandboxModeDangerFullAccess, domain.ApprovalPolicyAuto)
	got, err := adapter.InvokableRun(ctx, `{"value":"draft"}`)
	if err != nil {
		t.Fatalf("auto policy: %v", err)
	}
	if tool.calls != 1 {
		t.Fatalf("auto policy calls = %d, want 1", tool.calls)
	}
	if !strings.Contains(got, "mutated") {
		t.Fatalf("auto policy result = %q", got)
	}
}

// A sandbox denial raised by the tool implementation is a per-call refusal,
// not a run failure: the call did not happen and the model is told why.
func TestToolAdapterSandboxDenialRefusesInsteadOfFailing(t *testing.T) {
	tool := &sandboxDeniedTool{}
	adapter := newToolAdapter(tool, 0, nil, nil, nil)
	ctx := withPolicyProfile(context.Background(), domain.PolicyProfileFullAuto)
	result, err := adapter.InvokableRun(ctx, `{"value":"draft"}`)
	if err != nil {
		t.Fatalf("sandbox denial failed the run: %v", err)
	}
	if !strings.Contains(result, "did not run") || !strings.Contains(result, "sandbox denied") {
		t.Fatalf("sandbox denial result = %q", result)
	}
	if tool.calls != 1 {
		t.Fatalf("sandbox denial calls = %d, want the tool to have been asked once", tool.calls)
	}
}

func TestToolAdapterRedactsAndMarksUntrustedResult(t *testing.T) {
	adapter := newToolAdapter(secretResultTool{}, 0, nil, nil, nil)
	got, err := adapter.InvokableRun(context.Background(), `{}`)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if !strings.HasPrefix(got, untrustedToolResultHeader) || strings.Contains(got, "sk-live") || strings.Contains(got, "alice@example.com") {
		t.Fatalf("secured result = %q", got)
	}
}

func TestToolAdapterRedactsProviderErrorAndPreservesCause(t *testing.T) {
	adapter := newToolAdapter(secretErrorTool{}, 0, nil, nil, nil)
	_, err := adapter.InvokableRun(context.Background(), `{}`)
	if err == nil {
		t.Fatal("secret error tool returned nil error")
	}
	if strings.Contains(err.Error(), "sk-live-abcdefghijkl") || !strings.Contains(err.Error(), "REDACTED_SECRET") {
		t.Fatalf("tool error leaked Secret material: %v", err)
	}
	if !errors.Is(err, errSecretToolFailure) {
		t.Fatalf("tool error lost cause chain: %v", err)
	}
}

type longResultTool struct{}

type secretResultTool struct{}

type secretErrorTool struct{}

var errSecretToolFailure = errors.New("provider failed with token sk-live-abcdefghijkl")

func (secretResultTool) Spec() domain.ToolSpec {
	return domain.ToolSpec{Name: "secret_result", Description: "test tool", Readonly: true}
}

func (secretResultTool) InvokableRun(context.Context, json.RawMessage) (string, error) {
	return "sk-live-abcdefghijkl alice@example.com", nil
}

func (secretErrorTool) Spec() domain.ToolSpec {
	return domain.ToolSpec{Name: "secret_error", Description: "test tool", Readonly: true}
}

func (secretErrorTool) InvokableRun(context.Context, json.RawMessage) (string, error) {
	return "", errSecretToolFailure
}

func (longResultTool) Spec() domain.ToolSpec {
	return domain.ToolSpec{Name: "long_result", Description: "test tool", Readonly: true}
}

func (longResultTool) InvokableRun(context.Context, json.RawMessage) (string, error) {
	return "head-" + strings.Repeat("x", 256) + "-tail", nil
}

type countingTool struct {
	calls int
}

// sandboxDeniedTool stands in for a workspace backend that refuses the write.
type sandboxDeniedTool struct {
	calls int
}

func (t *sandboxDeniedTool) Spec() domain.ToolSpec {
	return domain.ToolSpec{
		Name:     "sandbox_write",
		Readonly: false,
		Params: map[string]domain.ToolParam{
			"value": {Required: true},
		},
	}
}

func (t *sandboxDeniedTool) InvokableRun(context.Context, json.RawMessage) (string, error) {
	t.calls++
	return "", fmt.Errorf("%w: path escapes workspace via traversal", ErrSandboxDenied)
}

type planCountingTool struct {
	calls int
}

func (t *planCountingTool) Spec() domain.ToolSpec {
	return domain.ToolSpec{
		Name:     "plan_write",
		Readonly: false,
		Params: map[string]domain.ToolParam{
			"value": {Required: true},
		},
	}
}

func (t *planCountingTool) InvokableRun(context.Context, json.RawMessage) (string, error) {
	t.calls++
	return "mutated", nil
}

func (t *countingTool) Spec() domain.ToolSpec {
	return domain.ToolSpec{
		Name: "counting_tool",
		Params: map[string]domain.ToolParam{
			"value": {Required: true},
		},
		Readonly: true,
	}
}

func (t *countingTool) InvokableRun(context.Context, json.RawMessage) (string, error) {
	t.calls++
	return "called", nil
}

// An unapprovable proposal is a per-call refusal, not a suspended run: the
// model is told why the call did not run and can correct the arguments.
func TestToolAdapterPreflightArgErrorRefusesInsteadOfSuspending(t *testing.T) {
	tool := &proposalStubTool{
		spec:       effectfulProposalSpec("staged_write"),
		prepareErr: &tools.ArgError{Field: "cwd", Reason: "must stay inside the run workspace"},
	}
	adapter := newToolAdapter(tool, 0, nil, nil, nil)
	result, err := adapter.InvokableRun(context.Background(), `{"value":"draft"}`)
	if err != nil {
		t.Fatalf("unpreparable proposal failed the run: %v", err)
	}
	if !strings.Contains(result, "did not run") || !strings.Contains(result, "must stay inside the run workspace") {
		t.Fatalf("refusal result = %q", result)
	}
	if tool.prepareCalls != 1 || tool.invocations != 0 {
		t.Fatalf("prepare calls = %d, invocations = %d, want 1/0", tool.prepareCalls, tool.invocations)
	}
}

// A provider that stages state implements PreflightProposal so the pre-check
// never stages twice; PrepareProposal must not be consulted.
func TestToolAdapterPreflightCapabilitySkipsPrepare(t *testing.T) {
	tool := &preflightStubTool{
		proposalStubTool: proposalStubTool{
			spec:       effectfulProposalSpec("skill_manage"),
			prepareErr: errors.New("prepare must not run during preflight"),
		},
		preflightErr: &tools.ArgError{Field: "name", Reason: "is required"},
	}
	adapter := newToolAdapter(tool, 0, nil, nil, nil)
	result, err := adapter.InvokableRun(context.Background(), `{"value":"draft"}`)
	if err != nil {
		t.Fatalf("unpreparable proposal failed the run: %v", err)
	}
	if !strings.Contains(result, "did not run") {
		t.Fatalf("refusal result = %q", result)
	}
	if tool.prepareCalls != 0 || tool.preflightCalls != 1 {
		t.Fatalf("prepare calls = %d, preflight calls = %d, want 0/1", tool.prepareCalls, tool.preflightCalls)
	}
	if tool.invocations != 0 {
		t.Fatalf("refused tool invocations = %d, want 0", tool.invocations)
	}
}

// A sandbox denial raised while preparing the proposal keeps the policy
// classification so the run inspector shows a governance refusal.
func TestToolAdapterPreflightSandboxDenialRefusesAsPolicyDenied(t *testing.T) {
	tool := &preflightStubTool{
		proposalStubTool: proposalStubTool{spec: effectfulProposalSpec("staged_write")},
		preflightErr:     fmt.Errorf("%w: cwd symlink escapes the run workspace", ErrSandboxDenied),
	}
	adapter := newToolAdapter(tool, 0, nil, nil, nil)
	err := adapter.preflightToolProposal(context.Background(), json.RawMessage(`{"value":"draft"}`))
	refusal, ok := asToolRefusal(err)
	if !ok {
		t.Fatalf("preflight denial = %v, want a tool refusal", err)
	}
	if refusal.classification != toolFailureReasonPolicyDenied {
		t.Fatalf("refusal classification = %q, want %q", refusal.classification, toolFailureReasonPolicyDenied)
	}
	result, runErr := adapter.InvokableRun(context.Background(), `{"value":"draft"}`)
	if runErr != nil {
		t.Fatalf("preflight denial failed the run: %v", runErr)
	}
	if !strings.Contains(result, "did not run") {
		t.Fatalf("refusal result = %q", result)
	}
}

// A wiring failure is not something the model can correct: it stays fatal so
// the defect is visible instead of becoming a plausible-looking refusal.
func TestToolAdapterPreflightWiringFailureStaysRunFatal(t *testing.T) {
	tool := &proposalStubTool{
		spec:       effectfulProposalSpec("staged_write"),
		prepareErr: errors.New("tools: skills backend not wired"),
	}
	adapter := newToolAdapter(tool, 0, nil, nil, nil)
	if _, err := adapter.InvokableRun(context.Background(), `{"value":"draft"}`); err == nil || !strings.Contains(err.Error(), "backend not wired") {
		t.Fatalf("wiring failure = %v, want the original run-fatal error", err)
	}
	if tool.invocations != 0 {
		t.Fatalf("failed preflight invocations = %d, want 0", tool.invocations)
	}
}

func effectfulProposalSpec(name string) domain.ToolSpec {
	return domain.ToolSpec{
		Name:     name,
		Readonly: false,
		Params: map[string]domain.ToolParam{
			"value": {Required: true},
		},
	}
}

type proposalStubTool struct {
	spec         domain.ToolSpec
	prepareCalls int
	invocations  int
	prepareErr   error
}

func (t *proposalStubTool) Spec() domain.ToolSpec { return t.spec }

func (t *proposalStubTool) InvokableRun(context.Context, json.RawMessage) (string, error) {
	t.invocations++
	return "ran", nil
}

func (t *proposalStubTool) PrepareProposal(context.Context, json.RawMessage) (domain.ToolProposal, error) {
	t.prepareCalls++
	return domain.ToolProposal{}, t.prepareErr
}

type preflightStubTool struct {
	proposalStubTool
	preflightCalls int
	preflightErr   error
}

func (t *preflightStubTool) PreflightProposal(context.Context, json.RawMessage) error {
	t.preflightCalls++
	return t.preflightErr
}
