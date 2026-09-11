package observer

import (
	"encoding/json"
	"testing"
)

func TestStableEventID(t *testing.T) {
	id := NewEventID("run-42", 17)
	if id.String() != "run-42:17" {
		t.Fatalf("event id = %q, want run-42:17", id.String())
	}
	if id.RunID != "run-42" || id.Seq != 17 {
		t.Fatalf("event id fields = %#v", id)
	}
}

func TestRunEventCopiesPayload(t *testing.T) {
	payload := json.RawMessage(`{"token":"redacted"}`)
	event := NewRunEvent(NewEventID("run-1", 1), "tool.completed", 123, payload)
	payload[2] = 'X'
	if string(event.Payload) != `{"token":"redacted"}` {
		t.Fatalf("run event payload mutated through caller slice: %s", event.Payload)
	}
}

func TestDiagnosticBoundsMessageAndFields(t *testing.T) {
	diagnostic := NewDiagnostic("lsp", "warning", string(make([]byte, MaxDiagnosticMessageBytes+32)), map[string]string{
		"path":  string(make([]byte, MaxDiagnosticFieldBytes+32)),
		"extra": "drop-me",
	})
	if len(diagnostic.Message) > MaxDiagnosticMessageBytes {
		t.Fatalf("diagnostic message bytes = %d", len(diagnostic.Message))
	}
	if len(diagnostic.Fields) > MaxDiagnosticFields {
		t.Fatalf("diagnostic fields = %d", len(diagnostic.Fields))
	}
	for _, value := range diagnostic.Fields {
		if len(value) > MaxDiagnosticFieldBytes {
			t.Fatalf("diagnostic field exceeds bound: %d", len(value))
		}
	}
}
