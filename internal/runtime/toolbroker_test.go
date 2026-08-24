package runtime

import (
	"context"
	"encoding/json"
	"testing"

	"agent-vivy/internal/domain"
)

type brokerTestTool struct {
	spec   domain.ToolSpec
	called bool
}

func (t *brokerTestTool) Spec() domain.ToolSpec { return t.spec }

func (t *brokerTestTool) InvokableRun(_ context.Context, args json.RawMessage) (string, error) {
	t.called = true
	return string(args), nil
}

func TestExecuteBrokerToolKeepsParentPolicyAuthority(t *testing.T) {
	policy, err := NewPolicyEngine(nil)
	if err != nil {
		t.Fatal(err)
	}
	tool := &brokerTestTool{spec: domain.ToolSpec{
		Name: "write_note", Params: map[string]domain.ToolParam{"text": {Required: true}},
	}}
	_, err = ExecuteBrokerTool(context.Background(), tool, policy, nil, "child-1", domain.PolicyProfileDefault, map[string]string{"text": "blocked"}, 1024)
	if err == nil {
		t.Fatal("default profile must not let a worker bypass approval")
	}
	if tool.called {
		t.Fatal("policy rejection must happen before invocation")
	}

	result, err := ExecuteBrokerTool(context.Background(), tool, policy, nil, "child-1", domain.PolicyProfileFullAuto, map[string]string{"text": "allowed"}, 1024)
	if err != nil {
		t.Fatalf("full_auto broker call: %v", err)
	}
	if !tool.called || result == "" {
		t.Fatalf("broker result = %q, called = %v", result, tool.called)
	}
}
