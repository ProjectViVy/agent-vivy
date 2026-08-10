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

type longResultTool struct{}

func (longResultTool) Spec() domain.ToolSpec {
	return domain.ToolSpec{Name: "long_result", Description: "test tool", Readonly: true}
}

func (longResultTool) InvokableRun(context.Context, json.RawMessage) (string, error) {
	return "head-" + strings.Repeat("x", 256) + "-tail", nil
}
