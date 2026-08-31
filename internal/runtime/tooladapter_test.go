package runtime

import (
	"context"
	"encoding/json"
	"errors"
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

func TestToolAdapterValidatesSchemaBeforeInvocation(t *testing.T) {
	tool := &countingTool{}
	adapter := newToolAdapter(tool, 0, nil, nil, nil)
	ctx := withSelectedTools(context.Background(), []string{tool.Spec().Name})
	if _, err := adapter.InvokableRun(ctx, `{}`); err == nil {
		t.Fatal("missing required argument must fail")
	}
	if tool.calls != 0 {
		t.Fatalf("invalid argument calls = %d, want zero", tool.calls)
	}
}

func TestToolAdapterPlanModeBlocksEffectfulToolBeforeApproval(t *testing.T) {
	tool := &planCountingTool{}
	adapter := newToolAdapter(tool, 0, nil, nil, nil)
	ctx := withRunMode(withSelectedTools(context.Background(), []string{tool.Spec().Name}), domain.RunModePlan)
	_, err := adapter.InvokableRun(ctx, `{"value":"draft"}`)
	if !errors.Is(err, ErrPlanModeToolDenied) {
		t.Fatalf("plan mode error = %v, want %v", err, ErrPlanModeToolDenied)
	}
	if tool.calls != 0 {
		t.Fatalf("plan mode effectful calls = %d, want zero", tool.calls)
	}
}

func TestToolAdapterApprovalPolicyNeverDeniesEffectful(t *testing.T) {
	tool := &planCountingTool{}
	adapter := newToolAdapter(tool, 0, nil, nil, nil)
	ctx := withSessionSandbox(withSelectedTools(context.Background(), []string{tool.Spec().Name}), domain.SandboxModeWorkspaceWrite, domain.ApprovalPolicyNever)
	_, err := adapter.InvokableRun(ctx, `{"value":"draft"}`)
	if !errors.Is(err, ErrPolicyDenied) {
		t.Fatalf("never policy error = %v, want %v", err, ErrPolicyDenied)
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

type longResultTool struct{}

type secretResultTool struct{}

func (secretResultTool) Spec() domain.ToolSpec {
	return domain.ToolSpec{Name: "secret_result", Description: "test tool", Readonly: true}
}

func (secretResultTool) InvokableRun(context.Context, json.RawMessage) (string, error) {
	return "sk-live-abcdefghijkl alice@example.com", nil
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
