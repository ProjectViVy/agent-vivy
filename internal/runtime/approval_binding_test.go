package runtime

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"agent-vivy/internal/domain"
)

func TestToolApprovalArgumentsCanonicalizeJSON(t *testing.T) {
	first, err := canonicalToolArguments(json.RawMessage(`{"b":2,"a":1}`))
	if err != nil {
		t.Fatal(err)
	}
	second, err := canonicalToolArguments(json.RawMessage("{\n\t\"a\": 1, \"b\": 2}"))
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(second) {
		t.Fatalf("canonical inputs = %q and %q", first, second)
	}
	otherTool, err := canonicalToolArguments(json.RawMessage(`{"a":1,"b":2}`))
	if err != nil {
		t.Fatal(err)
	}
	if string(otherTool) != string(first) {
		t.Fatal("canonical intent unexpectedly changed")
	}
}

func TestToolApprovalProposalBindingRoundTrip(t *testing.T) {
	providerData := json.RawMessage(`{"opaque":"proposal"}`)
	bound, err := bindToolApprovalProposal(providerData, json.RawMessage(`{"a":1}`))
	if err != nil {
		t.Fatal(err)
	}
	gotData, gotArguments, err := unbindToolApprovalProposal(bound)
	if err != nil {
		t.Fatal(err)
	}
	if string(gotData) != string(providerData) || string(gotArguments) != `{"a":1}` {
		t.Fatalf("unbound proposal = %s, %q", gotData, gotArguments)
	}
}

func TestNativeOrchestrationApprovalBindsRunNodeAndCheckpointTarget(t *testing.T) {
	descriptor, err := nativeOrchestrationDescriptorWithApproval("alpha", "beta", true)
	if err != nil {
		t.Fatal(err)
	}
	_ = descriptor
	input := orchestrationProofInput{
		A: nativeOrchestrationRequest{RunID: "run-1", DescriptorJSON: descriptor, NodeKey: "a", Task: "alpha", RequiresApproval: true},
		B: nativeOrchestrationRequest{RunID: "run-1", DescriptorJSON: descriptor, NodeKey: "b", Task: "beta"},
	}
	arguments, err := json.Marshal(struct {
		WorkflowRunID    string          `json:"workflow_run_id"`
		Descriptor       json.RawMessage `json:"descriptor"`
		NodeKey          string          `json:"node_key"`
		Task             string          `json:"task"`
		RequiresApproval bool            `json:"requires_approval,omitempty"`
	}{string(input.A.RunID), input.A.DescriptorJSON, input.A.NodeKey, input.A.Task, input.A.RequiresApproval})
	if err != nil {
		t.Fatal(err)
	}
	intended, err := canonicalToolArguments(arguments)
	if err != nil {
		t.Fatal(err)
	}
	proposal, err := bindToolApprovalProposal(nil, intended)
	if err != nil {
		t.Fatal(err)
	}
	approval := domain.Approval{
		RunID: input.A.RunID, ToolCallID: workflowNodeOperationID("a"), ToolName: "orchestration.node",
		ResumeTarget: nativeOrchestrationResumeTarget("runnable:vivy;node:a"), ProposalData: proposal,
	}
	if err := validateNativeOrchestrationApprovalBinding(approval, input); err != nil {
		t.Fatalf("valid native approval binding: %v", err)
	}
	approval.ToolCallID = workflowNodeOperationID("b")
	if err := validateNativeOrchestrationApprovalBinding(approval, input); err == nil {
		t.Fatal("approval for a different workflow node was accepted")
	}
}

func TestRedactedApprovalArgumentsCopiesNestedValues(t *testing.T) {
	original := map[string]any{
		"credential": "sk-live-middleware-secret",
		"nested":     []any{map[string]any{"email": "alice@example.com"}, float64(2)},
	}
	got := redactedApprovalArguments(original)
	encoded, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "sk-live-middleware-secret") || strings.Contains(string(encoded), "alice@example.com") {
		t.Fatalf("redacted arguments leaked sensitive data: %s", encoded)
	}
	if reflect.DeepEqual(got, original) {
		t.Fatal("redaction did not change sensitive values")
	}
	if original["credential"] != "sk-live-middleware-secret" {
		t.Fatal("redaction mutated the authoritative arguments")
	}
}

func TestNativeOrchestrationApprovalBindsCompleteOriginalDescriptor(t *testing.T) {
	descriptor, err := nativeOrchestrationDescriptorWithApproval("alpha", "beta", true)
	if err != nil {
		t.Fatal(err)
	}
	request := nativeOrchestrationRequest{RunID: "run-1", DescriptorJSON: descriptor, NodeKey: "a", Task: "alpha", RequiresApproval: true}
	arguments, err := nativeOrchestrationArguments(request)
	if err != nil {
		t.Fatal(err)
	}
	encoded := encodeToolApprovalInterrupt("orchestration.node", arguments, "approval")
	restored, err := nativeOrchestrationRequestFromApprovalState(encoded)
	if err != nil || string(restored.DescriptorJSON) != string(descriptor) {
		t.Fatalf("complete original descriptor was not restored: %+v %v", restored, err)
	}
	request.Task = "changed"
	arguments, err = nativeOrchestrationArguments(request)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := nativeOrchestrationRequestFromApprovalState(encodeToolApprovalInterrupt("orchestration.node", arguments, "approval")); err == nil {
		t.Fatal("changed task borrowed another original descriptor")
	}
	oldArguments := json.RawMessage(`{"workflow_run_id":"run-1","node_key":"a","task":"alpha","requires_approval":true}`)
	if _, err := nativeOrchestrationRequestFromApprovalState(encodeToolApprovalInterrupt("orchestration.node", oldArguments, "approval")); !errors.Is(err, ErrWorkflowRecoveryRequired) {
		t.Fatalf("missing original descriptor did not require recovery: %v", err)
	}
}
