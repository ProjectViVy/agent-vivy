package domain

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestChildRequestInputKeepsExactTaskAndCanonicalTools(t *testing.T) {
	original, err := ChildRequestInput("operation", "task-AA", []string{" tool-b ", "tool-a"})
	if err != nil {
		t.Fatal(err)
	}
	repeated, err := ChildRequestInput("operation", "task-AA", []string{"tool-a", "tool-b"})
	if err != nil || repeated != original {
		t.Fatalf("normalized retry changed: %q %v", repeated, err)
	}
	for _, input := range []struct {
		task  string
		tools []string
	}{
		{"task-BB", []string{"tool-a", "tool-b"}},
		{"task-AA", []string{"tool-a", "tool-c"}},
	} {
		changed, err := ChildRequestInput("operation", input.task, input.tools)
		if err != nil || changed == original {
			t.Fatalf("same-length change was lost: %q %v", changed, err)
		}
	}
	var actual struct {
		Task      string   `json:"task"`
		ToolNames []string `json:"tool_names"`
	}
	if err := json.Unmarshal([]byte(original), &actual); err != nil || actual.Task != "task-AA" || len(actual.ToolNames) != 2 {
		t.Fatalf("actual input not persisted: %+v %v", actual, err)
	}
	if _, err := ChildRequestInput("operation", "task", []string{"tool-a", "tool-a"}); err == nil {
		t.Fatal("duplicate tool selection accepted")
	}
}

func TestChildAuthorityComparesCompleteValuesAndStaysPrivate(t *testing.T) {
	ceiling := ChildAuthorityCeiling{PolicyProfile: PolicyProfileDefault, PolicyDefinition: "policy-A", SandboxMode: SandboxModeWorkspaceWrite, ApprovalPolicy: ApprovalPolicyAsk, ToolNames: []string{"tool-a", "tool-b"}}
	reordered := ceiling
	reordered.ToolNames = []string{"tool-b", "tool-a"}
	if !SameChildAuthority(ceiling, reordered) {
		t.Fatal("equivalent ceiling rejected")
	}
	changed := ceiling
	changed.PolicyDefinition = "policy-B"
	if SameChildAuthority(ceiling, changed) {
		t.Fatal("same-length changed policy accepted")
	}
	changed = ceiling
	changed.ToolNames = []string{"tool-a", "tool-c"}
	if SameChildAuthority(ceiling, changed) {
		t.Fatal("same-length changed tools accepted")
	}
	encoded, err := json.Marshal(ChildSessionBinding{RequestInput: "private-task", ActivationRequestInput: "private-activation", AuthorityCeiling: ceiling})
	if err != nil || strings.Contains(string(encoded), "private-") || strings.Contains(string(encoded), "policy-A") {
		t.Fatalf("private child inputs leaked: %s %v", encoded, err)
	}
}
