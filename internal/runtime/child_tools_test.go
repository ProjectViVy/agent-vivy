package runtime

import (
	"context"
	"encoding/json"
	"testing"

	"agent-vivy/internal/tools"
)

// get_goal is readonly, so the readonly filter used to offer it to every child
// even though its handler requires the session primary run. Advertising a tool
// the child can never satisfy turns a planning mistake into a fatal run.
func TestChildToolSurfacesExcludeGetGoal(t *testing.T) {
	h := newContractHarness(t, nil, contractHarnessOpts{extraTools: []tools.Tool{
		tools.NewGetGoal(),
		newContractTool(func(context.Context, json.RawMessage) (string, error) { return "ok", nil }),
	}})
	catalog := []string{tools.GetGoalName, contractToolName}

	for name, surface := range map[string]func([]string) []string{
		"readonly": h.svc.readOnlyChildTools,
		"one-shot": h.svc.oneShotChildTools,
		"workflow": h.svc.workflowChildTools,
	} {
		got := surface(catalog)
		if len(got) != 1 || got[0] != contractToolName {
			t.Fatalf("%s child tools = %v, want only %q", name, got, contractToolName)
		}
	}

	primary := make(map[string]struct{})
	for _, spec := range h.svc.engine.SelectTools().Specs {
		primary[spec.Name] = struct{}{}
	}
	if _, ok := primary[tools.GetGoalName]; !ok {
		t.Fatal("get_goal left the primary catalog; only the child surface may drop it")
	}
}
