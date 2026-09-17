package workflowhost

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"agent-vivy/internal/storage/sqlite"
	workflow "agent-vivy/internal/workflow"
)

func testHostConfig(backend *sqlite.Backend) Config {
	profiles := map[string]workflow.ModelCapability{
		"openai": {Provider: "openai", Model: "gpt-4o-mini", Configured: true, Active: true},
	}
	return Config{
		Definitions: backend, WorkflowRuns: backend, Sessions: backend, Runs: backend, Journal: backend,
		Executor: UnavailableExecutor{},
		Limits:   workflow.DefaultLimits(),
		Capabilities: CapabilitySourceFunc(func(context.Context) (CapabilitySnapshot, error) {
			return CapabilitySnapshot{
				Validation: workflow.ValidationContext{
					ModelProfiles: profiles,
					Tools:         map[string]workflow.ToolCapability{},
				},
				Identity: []byte(`{"test":"profiles"}`),
			}, nil
		}),
		Now: func() time.Time { return time.UnixMilli(1_000) },
	}
}

const demoDefinition = `{
  "schema_version":"1","id":"demo","title":"Demo",
  "inputs":{"name":{"type":"string","required":true}},
  "nodes":[
    {"id":"input","kind":"io","config":{"template":"hello ${{ inputs.name }}"},"timeout_ms":1000},
    {"id":"answer","kind":"model","config":{"profile":"openai","user_template":"hello ${{ inputs.name }}"},"timeout_ms":1000}
  ],
  "edges":[{"from":"input","to":"answer"}],
  "outputs":[{"name":"result","template":"${{ nodes.answer.output }}"}]
}`

func newTestHost(t *testing.T) (*Host, *sqlite.Backend) {
	t.Helper()
	ctx := context.Background()
	backend, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "workflowhost.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = backend.Close() })
	host, err := New(testHostConfig(backend))
	if err != nil {
		t.Fatal(err)
	}
	return host, backend
}

func TestNewRequiresDefinitionStoreAndNonNegativeLimits(t *testing.T) {
	if _, err := New(Config{}); !errors.Is(err, ErrHostNotConfigured) {
		t.Fatalf("New without stores = %v, want ErrHostNotConfigured", err)
	}
	ctx := context.Background()
	backend, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "workflowhost-limits.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = backend.Close() })
	limits := workflow.DefaultLimits()
	limits.MaxNodes = -1
	if _, err := New(Config{Definitions: backend, Limits: limits}); !errors.Is(err, ErrHostNotConfigured) {
		t.Fatalf("New with negative limit = %v, want ErrHostNotConfigured", err)
	}
}

func TestRunUnavailableBeforeAnyStoreMutation(t *testing.T) {
	ctx := context.Background()
	host, backend := newTestHost(t)
	if _, err := host.Define(ctx, []byte(demoDefinition)); err != nil {
		t.Fatal(err)
	}

	_, err := host.Run(ctx, RunRequest{ID: "demo", Inputs: map[string]any{"name": "vivy"}})
	if !errors.Is(err, workflow.ErrExecutionUnavailable) {
		t.Fatalf("Run = %v, want ErrExecutionUnavailable", err)
	}
	runs, err := backend.ListWorkflowRunsByDefinition(ctx, "demo", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(runs) != 0 {
		t.Fatalf("unavailable execution persisted workflow-run rows: %+v", runs)
	}
}

func TestRunRejectsInvalidInputsBeforePreflight(t *testing.T) {
	ctx := context.Background()
	host, _ := newTestHost(t)
	if _, err := host.Define(ctx, []byte(demoDefinition)); err != nil {
		t.Fatal(err)
	}

	if _, err := host.Run(ctx, RunRequest{ID: "demo", Inputs: map[string]any{}}); !errors.Is(err, ErrInvalidInputs) {
		t.Fatalf("Run without required input = %v, want ErrInvalidInputs", err)
	}
	if _, err := host.Run(ctx, RunRequest{ID: "demo", Inputs: map[string]any{"name": 7}}); !errors.Is(err, ErrInvalidInputs) {
		t.Fatalf("Run with mistyped input = %v, want ErrInvalidInputs", err)
	}
	if _, err := host.Run(ctx, RunRequest{ID: "demo", Inputs: map[string]any{"name": "vivy", "extra": 1}}); !errors.Is(err, ErrInvalidInputs) {
		t.Fatalf("Run with undeclared input = %v, want ErrInvalidInputs", err)
	}
}

func TestPreviewDefineAndPreviewRunPersistNothing(t *testing.T) {
	ctx := context.Background()
	host, backend := newTestHost(t)

	preview, err := host.PreviewDefine(ctx, []byte(demoDefinition))
	if err != nil {
		t.Fatal(err)
	}
	if preview.ID != "demo" || preview.NextRev != 1 || preview.NodeCount != 2 || preview.Hash == "" {
		t.Fatalf("unexpected define preview: %+v", preview)
	}
	if records, err := backend.ListDefinitions(ctx); err != nil || len(records) != 0 {
		t.Fatalf("PreviewDefine persisted records: %+v, %v", records, err)
	}

	if _, err := host.Define(ctx, []byte(demoDefinition)); err != nil {
		t.Fatal(err)
	}
	runPreview, err := host.PreviewRun(ctx, RunRequest{ID: "demo", Inputs: map[string]any{"name": "vivy"}})
	if err != nil {
		t.Fatal(err)
	}
	if runPreview.ID != "demo" || runPreview.Rev != 1 || runPreview.Hash == "" || runPreview.CapabilityHash == "" || runPreview.InputsSHA256 == "" {
		t.Fatalf("unexpected run preview: %+v", runPreview)
	}
	if runs, err := backend.ListWorkflowRunsByDefinition(ctx, "demo", 10); err != nil || len(runs) != 0 {
		t.Fatalf("PreviewRun persisted runs: %+v, %v", runs, err)
	}
}

func TestCapabilitySnapshotHashesIdentityStably(t *testing.T) {
	ctx := context.Background()
	host, _ := newTestHost(t)

	first, err := host.capabilitySnapshot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	second, err := host.capabilitySnapshot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if first.Hash == "" || first.Hash != second.Hash {
		t.Fatalf("capability hash unstable: %s vs %s", first.Hash, second.Hash)
	}
	if len(first.Identity) == 0 {
		t.Fatal("capability identity must not be empty")
	}
}

func TestRunsListsNewestFirstWithLimit(t *testing.T) {
	ctx := context.Background()
	host, _ := newTestHost(t)

	if records, err := host.Runs(ctx, "missing", 5); err != nil || len(records) != 0 {
		t.Fatalf("unknown workflow returned runs: %+v, %v", records, err)
	}
}
