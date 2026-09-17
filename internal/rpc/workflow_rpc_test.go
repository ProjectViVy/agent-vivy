package rpc

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	workflow "agent-vivy/internal/workflow"
	"agent-vivy/internal/workflowhost"
)

// wireWorkflowHost builds a WorkflowHost over the env backend and arms the
// optional control-plane seam the same way the app assembly does.
func wireWorkflowHost(t *testing.T, env *controlTestEnv) {
	t.Helper()
	host, err := workflowhost.New(workflowhost.Config{
		Definitions: env.backend, WorkflowRuns: env.backend, Sessions: env.backend, Runs: env.backend, Journal: env.backend,
		Executor: workflowhost.UnavailableExecutor{},
		Limits:   workflow.DefaultLimits(),
		Capabilities: workflowhost.CapabilitySourceFunc(func(context.Context) (workflowhost.CapabilitySnapshot, error) {
			return workflowhost.CapabilitySnapshot{
				Validation: workflow.ValidationContext{
					ModelProfiles: map[string]workflow.ModelCapability{
						"openai": {Provider: "openai", Model: "gpt-4o-mini", Configured: true, Active: true},
					},
					Tools: map[string]workflow.ToolCapability{},
				},
				Identity: []byte(`{"test":"rpc"}`),
			}, nil
		}),
		Now: func() time.Time { return time.UnixMilli(1_000) },
	})
	if err != nil {
		t.Fatal(err)
	}
	env.handler.(*controlHandler).deps.Workflow = host
}

const rpcDemoDefinition = `{
  "schema_version":"1","id":"demo","title":"Demo",
  "inputs":{"name":{"type":"string","required":true}},
  "nodes":[
    {"id":"input","kind":"io","config":{"template":"hello ${{ inputs.name }}"},"timeout_ms":1000},
    {"id":"answer","kind":"model","config":{"profile":"openai","user_template":"hello ${{ inputs.name }}"},"timeout_ms":1000}
  ],
  "edges":[{"from":"input","to":"answer"}],
  "outputs":[{"name":"result","template":"${{ nodes.answer.output }}"}]
}`

func TestControlWorkflowMethodsAreMethodNotFoundWithoutSeam(t *testing.T) {
	env := newControlTestEnv(t)
	methods := []string{"workflow/list", "workflow/get", "workflow/validate", "workflow/define", "workflow/run", "workflow/runs"}
	for _, method := range methods {
		_, rpcErr := callControl(t, env.handler, method, map[string]any{"id": "demo", "inputs": map[string]any{}, "definition": map[string]any{}})
		if rpcErr == nil || rpcErr.Code != MethodNotFound {
			t.Fatalf("%s without Workflow seam = %v, want MethodNotFound", method, rpcErr)
		}
	}
	caps, rpcErr := callControl(t, env.handler, "capabilities", map[string]any{})
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	for _, capability := range caps.(map[string]any)["capabilities"].([]string) {
		if capability == "workflow.definitions" || capability == "workflow.run" {
			t.Fatalf("nil seam advertised %s", capability)
		}
	}
}

func TestControlWorkflowDefineListGetRoundTrip(t *testing.T) {
	env := newControlTestEnv(t)
	wireWorkflowHost(t, env)

	defined, rpcErr := callControl(t, env.handler, "workflow/define", map[string]any{"definition": jsonRaw(t, rpcDemoDefinition)})
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	definedMap := defined.(workflowhost.DefineResult)
	if definedMap.ID != "demo" || definedMap.Rev != 1 || definedMap.Hash == "" {
		t.Fatalf("unexpected define result: %+v", definedMap)
	}

	listed, rpcErr := callControl(t, env.handler, "workflow/list", map[string]any{})
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	workflows := listed.(map[string]any)["workflows"].([]workflowhost.DefinitionSummary)
	if len(workflows) != 1 || workflows[0].ID != "demo" || workflows[0].Title != "Demo" {
		t.Fatalf("unexpected list: %+v", workflows)
	}

	view, rpcErr := callControl(t, env.handler, "workflow/get", map[string]any{"id": "demo"})
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	record := view.(map[string]any)["workflow"].(workflowhost.DefinitionView)
	if record.Rev != 1 || len(record.Definition) == 0 {
		t.Fatalf("unexpected view: %+v", record)
	}
}

func TestControlWorkflowValidateReturnsDiagnosticsForInvalidDefinition(t *testing.T) {
	env := newControlTestEnv(t)
	wireWorkflowHost(t, env)

	result, rpcErr := callControl(t, env.handler, "workflow/validate", map[string]any{
		"definition": jsonRaw(t, `{"schema_version":"1","id":"demo","title":"Demo","nodes":[],"edges":[]}`),
	})
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	validated := result.(map[string]any)
	if validated["valid"] != false {
		t.Fatalf("invalid definition reported valid: %+v", validated)
	}
	if _, ok := validated["diagnostics"].([]workflow.Diagnostic); !ok {
		t.Fatalf("diagnostics not structured: %+v", validated["diagnostics"])
	}
}

func TestControlWorkflowGetUnknownIDMapsToNotFound(t *testing.T) {
	env := newControlTestEnv(t)
	wireWorkflowHost(t, env)

	_, rpcErr := callControl(t, env.handler, "workflow/get", map[string]any{"id": "missing"})
	if rpcErr == nil || rpcErr.Code != CodeNotFound {
		t.Fatalf("workflow/get missing = %v, want CodeNotFound", rpcErr)
	}
}

func TestControlWorkflowRunReturnsUnavailableWithoutRunRow(t *testing.T) {
	env := newControlTestEnv(t)
	wireWorkflowHost(t, env)

	if _, rpcErr := callControl(t, env.handler, "workflow/define", map[string]any{"definition": jsonRaw(t, rpcDemoDefinition)}); rpcErr != nil {
		t.Fatal(rpcErr)
	}
	_, rpcErr := callControl(t, env.handler, "workflow/run", map[string]any{"id": "demo", "inputs": map[string]any{"name": "vivy"}})
	if rpcErr == nil || rpcErr.Code != CodeUnavailable {
		t.Fatalf("workflow/run = %v, want CodeUnavailable(-32011)", rpcErr)
	}
	if rpcErr.Message != "workflow execution is not configured in this generation" {
		t.Fatalf("unavailable message = %q", rpcErr.Message)
	}
	runs, err := env.backend.ListWorkflowRunsByDefinition(context.Background(), "demo", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(runs) != 0 {
		t.Fatalf("workflow/run persisted orphan rows: %+v", runs)
	}
}

func TestControlWorkflowCapabilitiesAdvertisedWithSeam(t *testing.T) {
	env := newControlTestEnv(t)
	wireWorkflowHost(t, env)

	caps, rpcErr := callControl(t, env.handler, "capabilities", map[string]any{})
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	var found, run bool
	for _, capability := range caps.(map[string]any)["capabilities"].([]string) {
		if capability == "workflow.definitions" {
			found = true
		}
		if capability == "workflow.run" {
			run = true
		}
	}
	if !found || !run {
		t.Fatalf("capabilities missing workflow advertisement: %+v", caps)
	}
}

func jsonRaw(t *testing.T, value string) json.RawMessage {
	t.Helper()
	if !json.Valid([]byte(value)) {
		t.Fatalf("fixture is not valid JSON: %s", value)
	}
	return json.RawMessage(value)
}
