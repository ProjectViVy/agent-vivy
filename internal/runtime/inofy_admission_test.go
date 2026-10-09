package runtime

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/testsupport"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

func taskDefinition() map[string]any {
	return map[string]any{
		"schema_version": "inofy.workflow/v1",
		"graph": map[string]any{
			"nodes": []any{
				map[string]any{"id": "research", "kind": "call", "type": "vivy.child-task@1", "config": map[string]any{"task": "Research the question", "tool_names": []string{"read_file"}}},
				map[string]any{"id": "summary", "kind": "call", "type": "vivy.child-task@1", "config": map[string]any{"task": "Summarize"}, "inputs": map[string]any{"research": map[string]any{"source": "research", "pointer": "/result"}}},
			},
			"edges":   []any{map[string]any{"from": "research", "to": "summary"}},
			"exits":   []string{"summary"},
			"outputs": map[string]any{"answer": map[string]any{"source": "summary", "pointer": "/result"}},
		},
	}
}

func TestINOFYToolSchemaExposesOnlyHostChildTasks(t *testing.T) {
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(WorkflowDefinitionSchema()))
	if err != nil {
		t.Fatal(err)
	}
	c := jsonschema.NewCompiler()
	if err := c.AddResource("vivy-definition.json", doc); err != nil {
		t.Fatal(err)
	}
	schema, err := c.Compile("vivy-definition.json")
	if err != nil {
		t.Fatal(err)
	}
	d := taskDefinition()
	encoded, _ := json.Marshal(d)
	var value map[string]any
	if err := json.Unmarshal(encoded, &value); err != nil {
		t.Fatal(err)
	}
	if err := schema.Validate(value); err != nil {
		t.Fatalf("supported graph rejected: %v", err)
	}
	value["graph"].(map[string]any)["nodes"].([]any)[0].(map[string]any)["type"] = "unknown@1"
	if err := schema.Validate(value); err == nil {
		t.Fatal("tool advertised an unsupported node type")
	}
	value["graph"].(map[string]any)["nodes"].([]any)[0].(map[string]any)["type"] = workflowChildType
	value["graph"].(map[string]any)["nodes"].([]any)[0].(map[string]any)["config"].(map[string]any)["tool_names"] = []any{"read_file", "read_file"}
	if err := schema.Validate(value); err == nil || !strings.Contains(err.Error(), "tool_names") {
		t.Fatalf("tool schema did not reject duplicate tool names at tool_names: %v", err)
	}
}

func TestINOFYAdmissionRejectsDuplicateToolNames(t *testing.T) {
	for _, tc := range []struct {
		name  string
		names []string
		cause string
	}{
		{name: "literal duplicate", names: []string{"read_file", "read_file"}, cause: "duplicate child tool name"},
		{name: "duplicate after trimming", names: []string{"read_file", " read_file "}, cause: "duplicate child tool name"},
		{name: "empty name", names: []string{""}, cause: "child tool name is empty"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			definition := taskDefinition()
			node := definition["graph"].(map[string]any)["nodes"].([]any)[0].(map[string]any)
			node["config"].(map[string]any)["tool_names"] = tc.names
			raw, err := json.Marshal(definition)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := validateINOFYDefinition(t.Context(), raw, []string{"read_file"}); err == nil {
				t.Fatal("invalid tool-name list was admitted")
			} else if !strings.Contains(err.Error(), "research") || !strings.Contains(err.Error(), tc.cause) {
				t.Fatalf("rejection lacks node/cause %q: %v", tc.cause, err)
			}
		})
	}
}

func TestINOFYAdmissionRejectsUnsupportedOrWidenedDefinitions(t *testing.T) {
	cases := []struct {
		name   string
		change func(map[string]any)
		want   string
	}{
		{"valid dependent graph", func(map[string]any) {}, ""},
		{"valid parallel join", func(d map[string]any) {
			graph := d["graph"].(map[string]any)
			graph["nodes"] = []any{
				map[string]any{"id": "left", "kind": "call", "type": workflowChildType, "config": map[string]any{"task": "First answer"}},
				map[string]any{"id": "right", "kind": "call", "type": workflowChildType, "config": map[string]any{"task": "Second answer"}},
				map[string]any{"id": "join", "kind": "call", "type": workflowChildType, "config": map[string]any{"task": "Combine"},
					"inputs": map[string]any{"left": map[string]any{"source": "left", "pointer": "/result"}, "right": map[string]any{"source": "right", "pointer": "/result"}}},
			}
			graph["edges"] = []any{map[string]any{"from": "left", "to": "join"}, map[string]any{"from": "right", "to": "join"}}
			graph["exits"] = []string{"join"}
			graph["outputs"] = map[string]any{"answer": map[string]any{"source": "join", "pointer": "/result"}}
		}, ""},
		{"old descriptor", func(d map[string]any) { d["schema_version"] = 1 }, "schema_version"},
		{"unknown node", func(d map[string]any) {
			d["graph"].(map[string]any)["nodes"].([]any)[0].(map[string]any)["type"] = "unknown@1"
		}, "type"},
		{"cross-kind fields", func(d map[string]any) {
			d["graph"].(map[string]any)["nodes"].([]any)[0].(map[string]any)["kind"] = "switch"
		}, "unknown_field"},
		{"write tool", func(d map[string]any) {
			d["graph"].(map[string]any)["nodes"].([]any)[0].(map[string]any)["config"].(map[string]any)["tool_names"] = []string{"write_file"}
		}, "tool"},
		{"long task", func(d map[string]any) {
			d["graph"].(map[string]any)["nodes"].([]any)[0].(map[string]any)["config"].(map[string]any)["task"] = strings.Repeat("x", 4097)
		}, "task"},
		{"too many outputs", func(d map[string]any) {
			out := d["graph"].(map[string]any)["outputs"].(map[string]any)
			for _, k := range []string{"b", "c", "d", "e"} {
				out[k] = map[string]any{"source": "summary", "pointer": "/result"}
			}
		}, "outputs"},
		{"ambiguous binding", func(d map[string]any) {
			d["graph"].(map[string]any)["nodes"].([]any)[1].(map[string]any)["inputs"].(map[string]any)["research"].(map[string]any)["literal"] = "false"
		}, "binding"},
		{"unknown config", func(d map[string]any) {
			d["graph"].(map[string]any)["nodes"].([]any)[0].(map[string]any)["config"].(map[string]any)["command"] = "run"
		}, "config"},
		{"unsupported retry", func(d map[string]any) {
			d["graph"].(map[string]any)["nodes"].([]any)[0].(map[string]any)["retry"] = map[string]any{"max_attempts": 2, "delay_ms": 0}
		}, "retry"},
		{"oversized literal input", func(d map[string]any) {
			d["graph"].(map[string]any)["nodes"].([]any)[0].(map[string]any)["inputs"] = map[string]any{"text": map[string]any{"literal": strings.Repeat("x", 16<<10)}}
		}, "input"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := taskDefinition()
			tc.change(d)
			raw, err := json.Marshal(d)
			if err != nil {
				t.Fatal(err)
			}
			admitted, err := validateINOFYDefinition(t.Context(), raw, []string{"read_file"})
			if tc.want == "" {
				if err != nil || admitted.Program == nil || admitted.Meta.ProgramDigest == "" || len(admitted.CanonicalJSON) == 0 {
					t.Fatalf("valid definition rejected: admission=%+v error=%v", admitted, err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("want %q rejection, got %v", tc.want, err)
			}
		})
	}
}

func TestINOFYAdmissionRejectsDuplicateConfigKeys(t *testing.T) {
	raw, _ := json.Marshal(taskDefinition())
	raw = bytes.Replace(raw, []byte(`"task":"Research the question"`), []byte(`"task":"safe","task":"Research the question"`), 1)
	if _, err := validateINOFYDefinition(t.Context(), raw, []string{"read_file"}); err == nil {
		t.Fatal("duplicate config key was accepted")
	}
}

func TestINOFYStartRejectsWithoutPersisting(t *testing.T) {
	svc, backend, _ := newTestService(t, testsupport.NewEchoModel())
	workspaces, err := NewSessionWorkspaceManager(t.TempDir(), backend, backend)
	if err != nil {
		t.Fatal(err)
	}
	svc.deps.Workspaces = workspaces
	parentID := domain.RunID("inofy-authorizer")
	prepareChildSessionAuthorizer(t, svc, backend, domain.SessionID("inofy-session"), parentID, nil)
	definition := taskDefinition()
	delete(definition["graph"].(map[string]any)["nodes"].([]any)[0].(map[string]any)["config"].(map[string]any), "task")
	raw, _ := json.Marshal(definition)
	if _, err := svc.StartINOFYWorkflow(t.Context(), parentID, "first", raw); err == nil {
		t.Fatal("invalid definition was admitted")
	}
	revisions, err := backend.ListWorkflowRevisions(t.Context(), parentID)
	if err != nil || len(revisions) != 0 {
		t.Fatalf("guard persisted workflow revisions: %v, %v", revisions, err)
	}
}
