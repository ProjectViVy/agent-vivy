package runtime

import (
	"agent-vivy/internal/domain"
	"encoding/base64"
	"testing"
)

func TestWorkflowChildIdentityUsesExistingOperation(t *testing.T) {
	operation := "workflow-opaque/graph/nodes/a"
	child := inofyChildRunID(operation)
	expected := domain.RunID("workflow_child_" + base64.RawURLEncoding.EncodeToString([]byte(operation)))
	if child != expected || !validWorkflowChildRunID(child) {
		t.Fatalf("owner key was not retained: %q", child)
	}
	for _, bad := range []domain.RunID{"workflow_child_", "workflow_child_%%%%", "unrelated", "workflow_child_Zg="} {
		if validWorkflowChildRunID(bad) {
			t.Fatalf("accepted invalid identity %q", bad)
		}
	}
}
