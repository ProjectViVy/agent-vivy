package runtime

import (
	"agent-vivy/internal/domain"
	"strings"
	"testing"
)

func TestIssue58NotebookAutomaticProjection(t *testing.T) {
	rows := []domain.Message{{ID: "user", Role: domain.RoleUser, Content: "keep"}, {ID: "call-a", Role: domain.RoleAssistant, ToolCallID: "a", ToolName: "secret-title", ToolArgs: []byte(`{"title":"hidden"}`), ContentOrigin: "original"}, {ID: "call-b", Role: domain.RoleAssistant, ToolCallID: "b", ToolName: "other", ToolArgs: []byte(`{}`), ContentOrigin: "original"}, {ID: "result-a", Role: domain.RoleTool, ToolCallID: "a", Content: "secret-body", ToolError: "secret-error", ContentOrigin: "notebook", ExcludeAutomaticIngest: true}, {ID: "result-b", Role: domain.RoleTool, ToolCallID: "b", Content: "ordinary sibling", ContentOrigin: "original"}, {ID: "derived", Role: domain.RoleAssistant, Content: "secret paraphrase", ContentOrigin: "notebook", ExcludeAutomaticIngest: true}}
	out, notices, err := projectAutomaticRecapOriginals(rows)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 || out[0].ID != "user" {
		t.Fatalf("inseparable tool group not excluded: %+v", out)
	}
	text := strings.Join(notices, "\n")
	if text == "" || !strings.Contains(text, "excluded") {
		t.Fatal("missing visible notice")
	}
	for _, bad := range []string{"secret-title", "hidden", "secret-body", "secret-error", "ordinary sibling", "secret paraphrase"} {
		if strings.Contains(text, bad) {
			t.Fatalf("notice leaked %s", bad)
		}
	}
	rows[2].ContentOrigin = "unknown"
	if _, _, err := projectAutomaticRecapOriginals(rows); err == nil {
		t.Fatal("known exclusion masked unknown sibling")
	}
}
