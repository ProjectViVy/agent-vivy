package tools

import (
	"context"
	"encoding/json"
	"testing"

	"agent-vivy/internal/domain"
)

type fakeWorkflowOperations struct {
	parentID   domain.RunID
	operation  string
	definition json.RawMessage
	result     WorkflowTaskResult
}

func (f *fakeWorkflowOperations) RunWorkflow(_ context.Context, parentID domain.RunID, operation string, definition json.RawMessage) (WorkflowTaskResult, error) {
	f.parentID, f.operation, f.definition = parentID, operation, append([]byte(nil), definition...)
	return f.result, nil
}

func (f *fakeWorkflowOperations) DefinitionSchema() json.RawMessage {
	return json.RawMessage(`{"type":"object","additionalProperties":false,"required":["schema_version","graph"],"properties":{"schema_version":{"const":"inofy.workflow/v1"},"graph":{"type":"object"}}}`)
}

func TestWorkflowToolRunsAgentAuthoredDefinitionWithStableCallIdentity(t *testing.T) {
	operations := &fakeWorkflowOperations{result: WorkflowTaskResult{
		WorkflowRunID: "workflow-run-1", Outputs: map[string]string{"answer": "bounded result"},
	}}
	tool := NewWorkflow(operations)
	args := json.RawMessage(`{"schema_version":"inofy.workflow/v1","graph":{"nodes":[{"id":"answer","kind":"call","type":"vivy.child-task@1","config":{"task":"answer the question"}}],"edges":[],"exits":["answer"]}}`)
	ctx := WithToolCallID(WithRunID(context.Background(), "parent-run-1"), "call-stable-1")
	result, err := tool.InvokableRun(ctx, args)
	if err != nil {
		t.Fatalf("invoke workflow tool: %v", err)
	}
	if operations.parentID != "parent-run-1" || operations.operation == "" || string(operations.definition) != string(args) {
		t.Fatalf("workflow operation = parent %q operation %q definition %s", operations.parentID, operations.operation, operations.definition)
	}
	var returned WorkflowTaskResult
	if err := json.Unmarshal([]byte(result), &returned); err != nil {
		t.Fatalf("decode workflow result %q: %v", result, err)
	}
	if returned.WorkflowRunID != "workflow-run-1" || returned.Outputs["answer"] != "bounded result" {
		t.Fatalf("workflow result = %+v", returned)
	}
	if err := toolsValidateWorkflowArgs(tool.Spec(), args); err != nil {
		t.Fatalf("workflow definition schema: %v", err)
	}
	if !tool.Spec().Readonly {
		t.Fatal("workflow tool must remain inside the read-only child authority surface")
	}
}

func TestWorkflowToolRequiresRuntimeIdentities(t *testing.T) {
	tool := NewWorkflow(&fakeWorkflowOperations{})
	args := json.RawMessage(`{"schema_version":"inofy.workflow/v1","graph":{}}`)
	if _, err := tool.InvokableRun(context.Background(), args); err == nil {
		t.Fatal("workflow tool accepted an unscoped invocation")
	}
}

func TestWorkflowToolRejectsLegacyDescriptor(t *testing.T) {
	tool := NewWorkflow(&fakeWorkflowOperations{})
	legacy := json.RawMessage(`{"schema_version":1,"start_nodes":["a"],"nodes":[{"key":"a","task":"answer"}],"edges":[],"outputs":["a"]}`)
	ctx := WithToolCallID(WithRunID(context.Background(), "parent-run-1"), "call-1")
	if _, err := tool.InvokableRun(ctx, legacy); err == nil {
		t.Fatal("legacy descriptor entered Definition tool path")
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
