package tools

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	workflow "agent-vivy/internal/workflow"
	"agent-vivy/internal/workflowhost"
)

// fakeWorkflowOperations records which mutating operations actually fired so
// tests can assert that proposal preparation never persists or executes.
type fakeWorkflowOperations struct {
	list   []workflowhost.DefinitionSummary
	view   workflowhost.DefinitionView
	define int
	run    int
}

func (f *fakeWorkflowOperations) List(context.Context) ([]workflowhost.DefinitionSummary, error) {
	return f.list, nil
}

func (f *fakeWorkflowOperations) Get(_ context.Context, id string, rev int64) (workflowhost.DefinitionView, error) {
	if id != f.view.ID {
		return workflowhost.DefinitionView{}, workflowhost.ErrHostNotConfigured
	}
	f.view.Rev = rev
	return f.view, nil
}

func (f *fakeWorkflowOperations) Validate(_ context.Context, raw []byte) (workflow.ValidationResult, error) {
	if strings.Contains(string(raw), "broken") {
		return workflow.ValidationResult{}, &workflow.ValidationError{Diagnostics: []workflow.Diagnostic{{Check: "schema", Path: "id", Message: "invalid slug"}}}
	}
	return workflow.ValidationResult{Diagnostics: []workflow.Diagnostic{}}, nil
}

func (f *fakeWorkflowOperations) PreviewDefine(context.Context, []byte) (workflowhost.DefinePreview, error) {
	return workflowhost.DefinePreview{ID: "demo", NextRev: 2, Hash: "abc123", NodeCount: 3, Title: "Demo"}, nil
}

func (f *fakeWorkflowOperations) Define(context.Context, []byte) (workflowhost.DefineResult, error) {
	f.define++
	return workflowhost.DefineResult{ID: "demo", Rev: 2, Hash: "abc123"}, nil
}

func (f *fakeWorkflowOperations) PreviewRun(context.Context, workflowhost.RunRequest) (workflowhost.RunPreview, error) {
	return workflowhost.RunPreview{ID: "demo", Rev: 1, Hash: "abc123", CapabilityHash: "cap", InputsSHA256: "dig"}, nil
}

func (f *fakeWorkflowOperations) Run(context.Context, workflowhost.RunRequest) (workflowhost.RunResult, error) {
	f.run++
	return workflowhost.RunResult{}, workflow.ErrExecutionUnavailable
}

func (f *fakeWorkflowOperations) Runs(context.Context, string, int) ([]workflowhost.RunSummary, error) {
	return []workflowhost.RunSummary{}, nil
}

const demoWorkflowArgs = `{"definition":{"schema_version":"1","id":"demo","title":"Demo","nodes":[{"id":"n","kind":"io","config":{"template":"hi"},"timeout_ms":1000}],"edges":[]}}`

var workflowToolReadonly = map[string]bool{
	WorkflowListName: true, WorkflowGetName: true, WorkflowValidateName: true,
	WorkflowRunsName: true, WorkflowDefineName: false, WorkflowRunName: false,
}

func TestWorkflowToolSpecsAreGoverned(t *testing.T) {
	toolsList := NewWorkflowTools(&fakeWorkflowOperations{})
	if len(toolsList) != 6 {
		t.Fatalf("workflow tool count = %d, want 6", len(toolsList))
	}
	for _, tool := range toolsList {
		spec := tool.Spec()
		want, ok := workflowToolReadonly[spec.Name]
		if !ok {
			t.Fatalf("unexpected workflow tool %q", spec.Name)
		}
		if spec.Readonly != want {
			t.Fatalf("%s Readonly = %v, want %v", spec.Name, spec.Readonly, want)
		}
		var schema map[string]any
		if err := json.Unmarshal(spec.Schema, &schema); err != nil {
			t.Fatalf("%s schema: %v", spec.Name, err)
		}
		if schema["additionalProperties"] != false || schema["type"] != "object" {
			t.Fatalf("%s schema is not a strict object: %v", spec.Name, schema)
		}
		if !IsWorkflowTool(spec.Name) {
			t.Fatalf("%s not recognized by IsWorkflowTool", spec.Name)
		}
	}
	if IsWorkflowTool("read_file") {
		t.Fatal("read_file must not be classified as a workflow tool")
	}
}

func TestWorkflowMutatingToolsPrepareBoundedProposalsWithoutSideEffects(t *testing.T) {
	ctx := context.Background()
	fake := &fakeWorkflowOperations{}
	for _, tool := range NewWorkflowTools(fake) {
		provider, ok := tool.(ProposalProvider)
		if !ok {
			t.Fatalf("%s must implement ProposalProvider", tool.Spec().Name)
		}
		if want := workflowToolReadonly[tool.Spec().Name]; !want {
			args := json.RawMessage(demoWorkflowArgs)
			if tool.Spec().Name == WorkflowRunName {
				args = json.RawMessage(`{"id":"demo","inputs":{"name":"vivy"}}`)
			}
			proposal, err := provider.PrepareProposal(ctx, args)
			if err != nil {
				t.Fatal(err)
			}
			if proposal.Action != tool.Spec().Name || proposal.Target != "demo" {
				t.Fatalf("unexpected proposal identity: %+v", proposal)
			}
			if len(proposal.Preview) == 0 || len(proposal.RiskFindings) == 0 {
				t.Fatalf("proposal lacks bounded preview/risk: %+v", proposal)
			}
			continue
		}
		if _, err := provider.PrepareProposal(ctx, json.RawMessage(`{}`)); err == nil || !strings.Contains(err.Error(), "not effectful") {
			t.Fatalf("readonly %s proposal = %v, want not-effectful error", tool.Spec().Name, err)
		}
	}
	if fake.define != 0 || fake.run != 0 {
		t.Fatalf("proposal preparation mutated state: define=%d run=%d", fake.define, fake.run)
	}
}

func TestWorkflowInvokableRunPaths(t *testing.T) {
	ctx := context.Background()
	fake := &fakeWorkflowOperations{list: []workflowhost.DefinitionSummary{{ID: "demo", LatestRev: 1, Title: "Demo"}}}
	fake.view = workflowhost.DefinitionView{ID: "demo", Rev: 1, Hash: "abc123", Definition: json.RawMessage(`{"id":"demo"}`)}
	byName := map[string]Tool{}
	for _, tool := range NewWorkflowTools(fake) {
		byName[tool.Spec().Name] = tool
	}

	listResult, err := byName[WorkflowListName].InvokableRun(ctx, json.RawMessage(`{}`))
	if err != nil || !strings.Contains(listResult, `"workflows"`) {
		t.Fatalf("workflow_list = %q, %v", listResult, err)
	}

	_, err = byName[WorkflowGetName].InvokableRun(ctx, json.RawMessage(`{"id":""}`))
	var argErr *ArgError
	if !errors.As(err, &argErr) {
		t.Fatalf("workflow_get empty id = %v, want ArgError", err)
	}

	validateResult, err := byName[WorkflowValidateName].InvokableRun(ctx, json.RawMessage(demoWorkflowArgs))
	if err != nil || !strings.Contains(validateResult, `"valid":true`) {
		t.Fatalf("workflow_validate = %q, %v", validateResult, err)
	}
	invalid, err := byName[WorkflowValidateName].InvokableRun(ctx, json.RawMessage(`{"definition":{"broken":true}}`))
	if err != nil || !strings.Contains(invalid, `"valid":false`) || !strings.Contains(invalid, "diagnostics") {
		t.Fatalf("workflow_validate invalid = %q, %v", invalid, err)
	}

	defineResult, err := byName[WorkflowDefineName].InvokableRun(ctx, json.RawMessage(demoWorkflowArgs))
	if err != nil || fake.define != 1 {
		t.Fatalf("workflow_define = %q, %v, define=%d", defineResult, err, fake.define)
	}

	_, runErr := byName[WorkflowRunName].InvokableRun(ctx, json.RawMessage(`{"id":"demo","inputs":{"name":"vivy"}}`))
	if !errors.Is(runErr, workflow.ErrExecutionUnavailable) {
		t.Fatalf("workflow_run = %v, want ErrExecutionUnavailable", runErr)
	}

	_, err = byName[WorkflowRunsName].InvokableRun(ctx, json.RawMessage(`{"id":"demo","limit":-1}`))
	if !errors.As(err, &argErr) {
		t.Fatalf("workflow_runs negative limit = %v, want ArgError", err)
	}
}
