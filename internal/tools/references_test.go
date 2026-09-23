package tools

import (
	"context"
	"encoding/json"
	"testing"

	"agent-vivy/internal/domain"
)

type fakeReferenceOps struct {
	selection domain.ReferenceSelection
	preview   domain.ReferencePreview
	previewOf domain.HistorySelection
	reference domain.ContextReference
	err       error
}

func (f *fakeReferenceOps) Preview(_ context.Context, selection domain.HistorySelection) (domain.ReferencePreview, error) {
	f.previewOf = selection
	return f.preview, f.err
}

func (f *fakeReferenceOps) Attach(_ context.Context, selection domain.ReferenceSelection) (domain.ContextReference, error) {
	f.selection = selection
	return f.reference, f.err
}

func newReferenceContextTool(t *testing.T, ops ReferenceOperations) *referenceContextTool {
	t.Helper()
	tool, ok := NewReferenceContext(ops).(*referenceContextTool)
	if !ok {
		t.Fatal("reference tool type mismatch")
	}
	return tool
}

func TestReferenceContextToolSpec(t *testing.T) {
	tool := newReferenceContextTool(t, &fakeReferenceOps{})
	spec := tool.Spec()
	if spec.Name != ReferenceContextName || spec.Readonly {
		t.Fatalf("spec = %#v", spec)
	}
	var schema struct {
		Required             []string                  `json:"required"`
		AdditionalProperties bool                      `json:"additionalProperties"`
		Properties           map[string]map[string]any `json:"properties"`
	}
	if err := json.Unmarshal(spec.Schema, &schema); err != nil {
		t.Fatalf("schema: %v", err)
	}
	if len(schema.Required) != 1 || schema.Required[0] != "selection" || schema.AdditionalProperties {
		t.Fatalf("schema contract = %#v", schema)
	}
	if _, ok := schema.Properties["selection"]; !ok {
		t.Fatal("schema missing selection property")
	}
}

func TestReferenceContextToolAttachesSelection(t *testing.T) {
	ops := &fakeReferenceOps{reference: domain.ContextReference{
		ID: "ref_1", DestinationSessionID: "B", SourceSessionID: "A", Origin: "model_tool",
		Items:  []domain.HistoryItem{{Text: "captured"}},
		Digest: "d1",
	}}
	tool := newReferenceContextTool(t, ops)
	out, err := tool.InvokableRun(context.Background(), json.RawMessage(`{"selection":{"source_session_id":"A","refs":[{"session_id":"A","message_id":"m1","kind":"message"}]},"expected_digest":"d1","description":"why"}`))
	if err != nil {
		t.Fatalf("invoke: %v", err)
	}
	if len(ops.selection.Selection.Refs) != 1 || ops.selection.Selection.Refs[0].MessageID != "m1" || ops.selection.ExpectedDigest != "d1" {
		t.Fatalf("forwarded selection = %#v", ops.selection)
	}
	var result referenceContextResult
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("result: %v", err)
	}
	if result.ReferenceID != "ref_1" || result.Origin != "model_tool" || len(result.Excerpt) != 1 {
		t.Fatalf("result = %#v", result)
	}
}

func TestReferenceContextToolRejectsForgedArgs(t *testing.T) {
	tool := newReferenceContextTool(t, &fakeReferenceOps{})
	for _, args := range []string{
		`{"selection":{"source_session_id":"A","refs":[],"items":[{"text":"forged"}]}}`,
		`{"selection":{"source_session_id":"A","refs":[]},"authority":"operator"}`,
		`{"selection":{"source_session_id":"A","refs":[]},"request_id":"forged"}`,
		`{"selection":{"source_session_id":"A","refs":[]},"destination_session_id":"B"}`,
	} {
		if _, err := tool.InvokableRun(context.Background(), json.RawMessage(args)); err == nil {
			t.Fatalf("forged args accepted: %s", args)
		}
	}
}

func TestReferenceContextToolWithoutOps(t *testing.T) {
	tool := newReferenceContextTool(t, nil)
	if _, err := tool.InvokableRun(context.Background(), json.RawMessage(`{"selection":{"source_session_id":"A","refs":[]}}`)); err == nil {
		t.Fatal("attach without reference operations succeeded")
	}
}
