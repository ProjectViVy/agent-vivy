package workflow

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestValidateCanonicalizesDefinitionAndCompilesTopologicalPlan(t *testing.T) {
	raw := []byte(`{
  "outputs":[{"template":"result: ${{ nodes.answer.output }}","name":"result"}],
  "edges":[{"to":"answer","from":"input"}],
  "nodes":[
    {"timeout_ms":1000,"config":{"template":"hello ${{ inputs.name }}"},"kind":"io","id":"input"},
    {"id":"answer","timeout_ms":1000,"kind":"model","config":{"profile":"openai","user_template":"${{ nodes.input.output }}"}}
  ],
  "title":"Demo","id":"demo","schema_version":"1",
  "inputs":{"name":{"required":true,"type":"string"}}
}`)
	ctx := ValidationContext{
		ModelProfiles: map[string]ModelCapability{"openai": {Provider: "openai", Model: "gpt-4o-mini", Active: true, Configured: true}},
		Tools:         map[string]ToolCapability{},
		Limits:        DefaultLimits(),
	}
	validated, err := Validate(raw, ctx)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if validated.Hash == "" || len(validated.Canonical) == 0 {
		t.Fatalf("missing canonical identity: %+v", validated)
	}
	var canonical map[string]any
	if err := json.Unmarshal(validated.Canonical, &canonical); err != nil {
		t.Fatalf("canonical JSON: %v", err)
	}
	if got := canonical["id"]; got != "demo" {
		t.Fatalf("canonical id = %v", got)
	}
	plan, err := Compile(validated, 3, ctx)
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	if plan.Rev != 3 || len(plan.Nodes) != 2 || plan.Nodes[0].ID != "input" || plan.Nodes[1].ID != "answer" {
		t.Fatalf("unexpected plan: %+v", plan)
	}
}

func TestValidateRejectsUncoveredNodeOutputReference(t *testing.T) {
	raw := validWorkflowJSON()
	raw = []byte(strings.Replace(string(raw), `"user_template":"hello ${{ inputs.name }}"`, `"user_template":"${{ nodes.other.output }}"`, 1))
	_, err := Validate(raw, validValidationContext())
	if err == nil || !errors.Is(err, ErrTopologyInvalid) {
		t.Fatalf("Validate error = %v, want topology error", err)
	}
}

func TestResolveTemplateUsesOnlyDeclaredReferences(t *testing.T) {
	template, err := ParseTemplate("prefix ${{ inputs.name }} / ${{ nodes.first.output }}")
	if err != nil {
		t.Fatalf("ParseTemplate: %v", err)
	}
	got, err := template.Resolve(map[string]string{"name": "Ada"}, map[string]string{"first": "answer"})
	if err != nil || got != "prefix Ada / answer" {
		t.Fatalf("Resolve = %q, %v", got, err)
	}
	if _, err := ParseTemplate("${{ inputs.name | trim }}"); err == nil {
		t.Fatal("template filter was accepted")
	}
}

func TestValidateReportsMissingModelProfileAndBudget(t *testing.T) {
	ctx := validValidationContext()
	ctx.ModelProfiles = nil
	ctx.Limits.MaxNodes = 0
	_, err := Validate(validWorkflowJSON(), ctx)
	if err == nil || !errors.Is(err, ErrCapabilityUnavailable) {
		t.Fatalf("Validate error = %v, want capability error", err)
	}
	var validationErr *ValidationError
	if !errors.As(err, &validationErr) || len(validationErr.Diagnostics) < 2 {
		t.Fatalf("diagnostics = %+v, want capability and budget diagnostics", validationErr)
	}
}

func validValidationContext() ValidationContext {
	return ValidationContext{
		ModelProfiles: map[string]ModelCapability{"openai": {Provider: "openai", Model: "gpt-4o-mini", Active: true, Configured: true}},
		Limits:        DefaultLimits(),
	}
}

func validWorkflowJSON() []byte {
	return []byte(`{
  "schema_version":"1","id":"demo","title":"Demo",
  "inputs":{"name":{"type":"string","required":true}},
  "nodes":[
    {"id":"input","kind":"io","config":{"template":"hello ${{ inputs.name }}"},"timeout_ms":1000},
    {"id":"answer","kind":"model","config":{"profile":"openai","user_template":"hello ${{ inputs.name }}"},"timeout_ms":1000}
  ],
  "edges":[{"from":"input","to":"answer"}],
  "outputs":[{"name":"result","template":"${{ nodes.answer.output }}"}]
}`)
}
