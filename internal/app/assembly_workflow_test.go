package app

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"agent-vivy/internal/config"
	"agent-vivy/internal/domain"
	"agent-vivy/internal/modelhost"
	"agent-vivy/internal/storage/sqlite"
	workflow "agent-vivy/internal/workflow"
	"agent-vivy/internal/workflowhost"
)

func workflowComposeSource() workflowCapabilitySource {
	return workflowCapabilitySource{
		GenerationID: "gen-compose-test",
		CurrentModel: func() ResolvedModel {
			return ResolvedModel{Provider: "openai", Model: "gpt-4o-mini", Ready: true}
		},
		ModelStatuses: func(string, bool) []modelhost.ProfileStatus {
			return []modelhost.ProfileStatus{{ID: "openai", State: modelhost.ProfileReady, ModelIDs: []string{"gpt-4o-mini"}}}
		},
		ToolSpecs: func() []domain.ToolSpec {
			return []domain.ToolSpec{{Name: "read_file", Readonly: true}}
		},
	}
}

func TestNewWorkflowOperationsComposesHostOverSingleBackend(t *testing.T) {
	ctx := context.Background()
	backend, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "workflow-compose.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = backend.Close() })

	ops, err := newWorkflowOperations(config.Default(), backend, workflowComposeSource())
	if err != nil {
		t.Fatal(err)
	}

	defined, err := ops.Define(ctx, []byte(`{
  "schema_version":"1","id":"demo","title":"Demo",
  "inputs":{"name":{"type":"string","required":true}},
  "nodes":[
    {"id":"input","kind":"io","config":{"template":"hello ${{ inputs.name }}"},"timeout_ms":1000},
    {"id":"answer","kind":"model","config":{"profile":"openai","user_template":"hello ${{ inputs.name }}"},"timeout_ms":1000}
  ],
  "edges":[{"from":"input","to":"answer"}],
  "outputs":[{"name":"result","template":"${{ nodes.answer.output }}"}]
}`))
	if err != nil {
		t.Fatalf("Define: %v", err)
	}
	if defined.Rev != 1 || defined.Hash == "" {
		t.Fatalf("unexpected define result: %+v", defined)
	}

	workflows, err := ops.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(workflows) != 1 || workflows[0].ID != "demo" || workflows[0].Title != "Demo" || workflows[0].LatestRev != 1 {
		t.Fatalf("unexpected list: %+v", workflows)
	}

	view, err := ops.Get(ctx, "demo", 0)
	if err != nil {
		t.Fatal(err)
	}
	if view.Rev != 1 || len(view.Definition) == 0 {
		t.Fatalf("unexpected view: %+v", view)
	}
}

func TestNewWorkflowOperationsCapabilitySnapshotFlowsIntoValidation(t *testing.T) {
	ctx := context.Background()
	backend, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "workflow-capability.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = backend.Close() })

	source := workflowComposeSource()
	ops, err := newWorkflowOperations(config.Default(), backend, source)
	if err != nil {
		t.Fatal(err)
	}
	raw := []byte(`{
  "schema_version":"1","id":"demo","title":"Demo",
  "nodes":[
    {"id":"answer","kind":"model","config":{"profile":"ghost","user_template":"hi"},"timeout_ms":1000}
  ],
  "edges":[]
}`)
	if _, err := ops.Validate(ctx, raw); !errors.Is(err, workflow.ErrCapabilityUnavailable) {
		t.Fatalf("Validate with unknown profile = %v, want ErrCapabilityUnavailable", err)
	}

	source.ModelStatuses = func(string, bool) []modelhost.ProfileStatus {
		return []modelhost.ProfileStatus{{ID: "openai", State: modelhost.ProfileUnconfigured, ModelIDs: []string{"gpt-4o-mini"}}}
	}
	inactiveOps, err := newWorkflowOperations(config.Default(), backend, source)
	if err != nil {
		t.Fatal(err)
	}
	readyRaw := []byte(`{
  "schema_version":"1","id":"demo","title":"Demo",
  "nodes":[
    {"id":"answer","kind":"model","config":{"profile":"openai","user_template":"hi"},"timeout_ms":1000}
  ],
  "edges":[]
}`)
	if _, err := inactiveOps.Validate(ctx, readyRaw); !errors.Is(err, workflow.ErrCapabilityUnavailable) {
		t.Fatalf("Validate with inactive profile = %v, want ErrCapabilityUnavailable", err)
	}
}

func TestNewWorkflowOperationsRunRefusesUnavailableWithoutOrphanRows(t *testing.T) {
	ctx := context.Background()
	backend, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "workflow-run.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = backend.Close() })

	ops, err := newWorkflowOperations(config.Default(), backend, workflowComposeSource())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ops.Define(ctx, []byte(`{
  "schema_version":"1","id":"demo","title":"Demo",
  "nodes":[
    {"id":"answer","kind":"model","config":{"profile":"openai","user_template":"hi"},"timeout_ms":1000}
  ],
  "edges":[]
}`)); err != nil {
		t.Fatal(err)
	}

	_, runErr := ops.Run(ctx, workflowhost.RunRequest{ID: "demo", Inputs: map[string]any{}})
	if !errors.Is(runErr, workflow.ErrExecutionUnavailable) {
		t.Fatalf("Run = %v, want ErrExecutionUnavailable", runErr)
	}
	runs, err := ops.Runs(ctx, "demo", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(runs) != 0 {
		t.Fatalf("unavailable execution persisted orphan run rows: %+v", runs)
	}
}
