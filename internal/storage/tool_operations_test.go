package storage

import (
	"bytes"
	"encoding/json"
	"testing"

	"agent-vivy/internal/domain"
)

func TestToolOperationAdmissionInputs(t *testing.T) {
	valid := domain.ToolOperation{RunID: "run", OperationID: "call", ToolName: "tool",
		RequestArguments: []byte(`null`), MiddlewareInputArguments: []byte(`{}`), EffectiveArguments: []byte(`[]`)}
	if err := ValidateToolOperationAdmission(valid); err != nil {
		t.Fatal(err)
	}
	oversizedJSON := append([]byte{'"'}, bytes.Repeat([]byte("x"), 1<<20)...)
	oversizedJSON = append(oversizedJSON, '"')
	for _, field := range []string{"request", "middleware", "effective"} {
		for _, value := range [][]byte{nil, {}, []byte(`{`), oversizedJSON} {
			op := valid
			switch field {
			case "request":
				op.RequestArguments = value
			case "middleware":
				op.MiddlewareInputArguments = value
			case "effective":
				op.EffectiveArguments = value
			}
			if err := ValidateToolOperationAdmission(op); err == nil {
				t.Fatalf("accepted %s input of length %d", field, len(value))
			}
		}
	}
	valid.RequestArguments = append([]byte{'"'}, bytes.Repeat([]byte("x"), (1<<20)-2)...)
	valid.RequestArguments = append(valid.RequestArguments, '"')
	if err := ValidateToolOperationAdmission(valid); err != nil {
		t.Fatalf("exact size limit rejected: %v", err)
	}
}

func TestToolOperationOutcomeValidation(t *testing.T) {
	for _, outcome := range []domain.ToolOperationOutcome{
		{Result: "ok", Status: "success", Effects: "unknown"},
		{Result: "correct input", Status: "recoverable", Reason: "invalid_arguments", Effects: "not_executed"},
		{Result: "denied", Status: "refused", Reason: "user_denied", Effects: "not_executed"},
		{Failure: "broken", Status: "fatal", Effects: "unknown"},
	} {
		if err := ValidateToolOperationOutcome(outcome); err != nil {
			t.Fatal(err)
		}
	}
	for _, outcome := range []domain.ToolOperationOutcome{
		{}, {Status: "success"}, {Status: "success", Reason: "user_denied", Effects: "none"},
		{Status: "recoverable", Effects: "unknown"}, {Status: "refused", Reason: "invented", Effects: "not_executed"},
		{Status: "fatal", Reason: "remote_tool_error", Effects: "unknown"},
	} {
		if err := ValidateToolOperationOutcome(outcome); err == nil {
			t.Fatalf("accepted %+v", outcome)
		}
	}
}

func TestToolOperationEventDoesNotExposeBinding(t *testing.T) {
	op := domain.ToolOperation{RunID: "run", OperationID: "call", ToolName: "tool", State: domain.ToolOperationAdmitted,
		RequestArguments: []byte(`{"private":"canary-request"}`), MiddlewareInputArguments: []byte(`"canary-middleware"`), EffectiveArguments: []byte(`"canary-effective"`)}
	event, err := NewToolOperationEvent(op)
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload) != 3 || bytes.Contains(event.Payload, []byte("canary")) {
		t.Fatalf("public binding leak: %s", event.Payload)
	}
}
