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

type brokerReplayProbeTool struct {
	spec  domain.ToolSpec
	calls int
}

func (t *brokerReplayProbeTool) Spec() domain.ToolSpec { return t.spec }

func (t *brokerReplayProbeTool) InvokableRun(_ context.Context, _ json.RawMessage) (string, error) {
	t.calls++
	return "fixture invoked", nil
}

func TestGraphConformanceBrokerReplayRisk(t *testing.T) {
	// This intentionally failing, same-process probe makes two direct calls to
	// ExecuteBrokerTool with the same run and input. It uses an in-memory fixture
	// counter only: it does not create a Service, Engine, checkpoint, or fresh
	// store, inject or recover a crash, or perform a real external side effect.
	// It observes that this broker API seam has no durable operation/result
	// identity that would let it return a prior result on a repeated direct call.
	policy, err := NewPolicyEngine(nil)
	if err != nil {
		t.Fatal(err)
	}
	tool := &brokerReplayProbeTool{spec: domain.ToolSpec{
		Name: "write_note", Params: map[string]domain.ToolParam{"text": {Required: true}},
	}}
	args := map[string]string{"text": "same direct input"}

	for attempt := 0; attempt < 2; attempt++ {
		if _, err := ExecuteBrokerTool(context.Background(), tool, policy, nil, "child-replay", domain.PolicyProfileFullAuto, args, 1024); err != nil {
			t.Fatalf("broker replay attempt %d: %v", attempt+1, err)
		}
	}

	if tool.calls != 1 {
		t.Fatalf("same-process direct broker calls re-invoked in-memory fixture %d times; crash/restart safety remains unproven", tool.calls)
	}
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
