package runtime

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"agent-vivy/internal/domain"
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
	adapter := newToolAdapter(tool, 48)
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
	adapter := newToolAdapter(tool, 0)
	ctx := withSelectedTools(context.Background(), []string{"another_tool"})
	if _, err := adapter.InvokableRun(ctx, `{}`); err == nil || !strings.Contains(err.Error(), "not selected") {
		t.Fatalf("unselected tool error = %v, want fail-closed rejection", err)
	}
	if tool.calls != 0 {
		t.Fatalf("unselected tool calls = %d, want zero", tool.calls)
	}
}

func TestToolAdapterValidatesSchemaBeforeInvocation(t *testing.T) {
	tool := &countingTool{}
	adapter := newToolAdapter(tool, 0)
	ctx := withSelectedTools(context.Background(), []string{tool.Spec().Name})
	if _, err := adapter.InvokableRun(ctx, `{}`); err == nil {
		t.Fatal("missing required argument must fail")
	}
	if tool.calls != 0 {
		t.Fatalf("invalid argument calls = %d, want zero", tool.calls)
	}
}

type longResultTool struct{}

func (longResultTool) Spec() domain.ToolSpec {
	return domain.ToolSpec{Name: "long_result", Description: "test tool", Readonly: true}
}

func (longResultTool) InvokableRun(context.Context, json.RawMessage) (string, error) {
	return "head-" + strings.Repeat("x", 256) + "-tail", nil
}

type countingTool struct {
	calls int
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
