package tools

import (
	"context"
	"encoding/json"
	"testing"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/orchestration"
)

type fakeWorkflowOperations struct {
	parentID   domain.RunID
	operation  string
	descriptor orchestration.Descriptor
	result     WorkflowTaskResult
}

func (f *fakeWorkflowOperations) RunWorkflow(_ context.Context, parentID domain.RunID, operation string, descriptor orchestration.Descriptor) (WorkflowTaskResult, error) {
	f.parentID, f.operation, f.descriptor = parentID, operation, descriptor
	return f.result, nil
}

func TestWorkflowToolRunsAgentAuthoredDescriptorWithStableCallIdentity(t *testing.T) {
	operations := &fakeWorkflowOperations{result: WorkflowTaskResult{
		WorkflowRunID: "workflow-run-1", Outputs: map[string]string{"answer": "bounded result"},
	}}
	tool := NewWorkflow(operations)
	args := json.RawMessage(`{"schema_version":1,"start_nodes":["answer"],"nodes":[{"key":"answer","task":"answer the question"}],"edges":[],"outputs":["answer"]}`)
	ctx := WithToolCallID(WithRunID(context.Background(), "parent-run-1"), "call-stable-1")
	result, err := tool.InvokableRun(ctx, args)
	if err != nil {
		t.Fatalf("invoke workflow tool: %v", err)
	}
	if operations.parentID != "parent-run-1" || operations.operation == "" || operations.descriptor.Nodes[0].Task != "answer the question" {
		t.Fatalf("workflow operation = parent %q operation %q descriptor %+v", operations.parentID, operations.operation, operations.descriptor)
	}
	var returned WorkflowTaskResult
	if err := json.Unmarshal([]byte(result), &returned); err != nil {
		t.Fatalf("decode workflow result %q: %v", result, err)
	}
	if returned.WorkflowRunID != "workflow-run-1" || returned.Outputs["answer"] != "bounded result" {
		t.Fatalf("workflow result = %+v", returned)
	}
	if err := toolsValidateWorkflowArgs(tool.Spec(), args); err != nil {
		t.Fatalf("workflow descriptor schema: %v", err)
	}
	if !tool.Spec().Readonly {
		t.Fatal("workflow tool must remain inside the read-only child authority surface")
	}
}

func TestWorkflowToolRequiresRuntimeIdentities(t *testing.T) {
	tool := NewWorkflow(&fakeWorkflowOperations{})
	args := json.RawMessage(`{"schema_version":1,"start_nodes":["answer"],"nodes":[{"key":"answer","task":"answer"}],"edges":[],"outputs":["answer"]}`)
	if _, err := tool.InvokableRun(context.Background(), args); err == nil {
		t.Fatal("workflow tool accepted an unscoped invocation")
	}
}

func TestBuiltinRegistryAddsWorkflowOnlyWhenWired(t *testing.T) {
	registry := BuiltinWithWorkflow(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, &fakeWorkflowOperations{})
	found := false
	for _, spec := range registry.Specs() {
		if spec.Name == WorkflowName {
			found = true
			if !spec.Readonly || len(spec.Schema) == 0 {
				t.Fatalf("workflow tool spec = %+v", spec)
			}
		}
	}
	if !found {
		t.Fatal("workflow tool missing from registry built with WorkflowOperations")
	}
	without := BuiltinWithAgent(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	for _, spec := range without.Specs() {
		if spec.Name == WorkflowName {
			t.Fatal("workflow tool must not register without WorkflowOperations")
		}
	}
}

func toolsValidateWorkflowArgs(spec domain.ToolSpec, args json.RawMessage) error {
	return ValidateArgs(spec, args)
}
