package runtime

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	jsonschema "github.com/santhosh-tekuri/jsonschema/v6"

	"agent-vivy/internal/domain"
)

func TestObserverProjectionPayloadSchemasPreserveLegacyAndValidateV2(t *testing.T) {
	m := newEventMapper("run-schema", 64<<10)
	m.setRunScope("tenant", "workspace", "session")
	m.setContextViewID("view")
	m.lastSummary = "summary"
	tests := []struct {
		name  string
		event domain.RunEvent
	}{
		{"run.completed", m.build(domain.EventRunCompleted, payloadRunCompleted{})},
		{"run.failed", m.build(domain.EventRunFailed, payloadRunFailed{CauseCategory: causeInternalError, Message: "failed"})},
		{"run.cancelled", m.build(domain.EventRunCancelled, payloadRunCancelled{Reason: reasonUserRequested})},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.event.PayloadVersion != 2 {
				t.Fatalf("terminal payload version = %d, want 2", test.event.PayloadVersion)
			}
			validatePayloadSchema(t, test.name, test.event.Payload)
		})
	}

	t.Run("model.request legacy v1", func(t *testing.T) {
		event := m.build(domain.EventModelRequest, payloadModelRequest{
			SelectedTools: []string{}, PreambleSHA256: string(make([]byte, 64)), Messages: []payloadModelRequestMessage{},
		})
		if event.PayloadVersion != 1 {
			t.Fatalf("legacy request payload version = %d, want 1", event.PayloadVersion)
		}
		validatePayloadSchema(t, "model.request", event.Payload)
	})

	t.Run("model.request Context View v2", func(t *testing.T) {
		event := m.build(domain.EventModelRequest, payloadModelRequest{
			SelectedTools: []string{}, PreambleSHA256: string(make([]byte, 64)), Messages: []payloadModelRequestMessage{}, ContextView: "view",
		})
		if event.PayloadVersion != 2 {
			t.Fatalf("Context View request payload version = %d, want 2", event.PayloadVersion)
		}
		validatePayloadSchema(t, "model.request", event.Payload)
	})
}

func validatePayloadSchema(t *testing.T, eventType string, payload json.RawMessage) {
	t.Helper()
	rawSchema, err := os.ReadFile(filepath.Join("..", "..", "schemas", "events", "payloads", eventType+".json"))
	if err != nil {
		t.Fatal(err)
	}
	document, err := jsonschema.UnmarshalJSON(bytes.NewReader(rawSchema))
	if err != nil {
		t.Fatal(err)
	}
	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource(eventType, document); err != nil {
		t.Fatal(err)
	}
	compiled, err := compiler.Compile(eventType)
	if err != nil {
		t.Fatal(err)
	}
	instance, err := jsonschema.UnmarshalJSON(bytes.NewReader(payload))
	if err != nil {
		t.Fatal(err)
	}
	if err := compiled.Validate(instance); err != nil {
		t.Fatalf("payload %s violates schema: %v", payload, err)
	}
}
