package domain

import (
	"bytes"
	"encoding/json"
	jsonschema "github.com/santhosh-tekuri/jsonschema/v6"
	"os"
	"path/filepath"
	"testing"
)

func TestDeliverablePresentedSchemaMatchesCurrentMetadata(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "schemas", "events", "payloads", "deliverables.presented.json"))
	if err != nil {
		t.Fatal(err)
	}
	document, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource("presented", document); err != nil {
		t.Fatal(err)
	}
	schema, err := compiler.Compile("presented")
	if err != nil {
		t.Fatal(err)
	}
	set := DeliverySet{ID: "set", SessionID: "session", RunID: "run", ToolCallID: "call", CreatedAt: 1, Status: DeliveryStatusOK, Items: []Deliverable{{ID: "item", SessionID: "session", RunID: "run", WorkspaceID: "workspace", Path: "out.txt", Name: "out.txt", Description: "output", Size: 3, MediaType: "text/plain", CapturedAt: 1, OriginToolCallID: "call"}}, Failures: []DeliveryFailure{}}
	encoded, err := json.Marshal(struct {
		DeliverySet DeliverySet `json:"delivery_set"`
	}{set})
	if err != nil {
		t.Fatal(err)
	}
	instance, err := jsonschema.UnmarshalJSON(bytes.NewReader(encoded))
	if err != nil {
		t.Fatal(err)
	}
	if err := schema.Validate(instance); err != nil {
		t.Fatalf("current producer metadata rejected: %v", err)
	}
	item := instance.(map[string]any)["delivery_set"].(map[string]any)["items"].([]any)[0].(map[string]any)
	item["sha256"] = "obsolete"
	if err := schema.Validate(instance); err == nil {
		t.Fatal("obsolete digest accepted by closed current schema")
	}
}
