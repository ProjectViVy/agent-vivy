package tools

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"agent-vivy/internal/domain"
)

type fakeDeliverableOps struct {
	request domain.PresentRequest
	set     domain.DeliverySet
	err     error
}

func (f *fakeDeliverableOps) Present(_ context.Context, request domain.PresentRequest) (domain.DeliverySet, error) {
	f.request = request
	return f.set, f.err
}

func (f *fakeDeliverableOps) List(context.Context, domain.SessionID, string, int) (domain.DeliverySetPage, error) {
	return domain.DeliverySetPage{}, f.err
}

func (f *fakeDeliverableOps) Get(context.Context, domain.SessionID, string) (domain.DeliverySet, error) {
	return domain.DeliverySet{}, f.err
}

func (f *fakeDeliverableOps) Read(context.Context, domain.DeliveryReadRequest) (domain.DeliveryChunk, error) {
	return domain.DeliveryChunk{}, f.err
}

func (f *fakeDeliverableOps) CloseTransfer(context.Context, string) error { return f.err }

func newPresentFilesTool(t *testing.T, ops DeliverableOperations) *presentFilesTool {
	t.Helper()
	tool, ok := NewPresentFiles(ops).(*presentFilesTool)
	if !ok {
		t.Fatal("present files tool type mismatch")
	}
	return tool
}

func TestPresentFilesToolSpec(t *testing.T) {
	tool := newPresentFilesTool(t, &fakeDeliverableOps{})
	spec := tool.Spec()
	if spec.Name != PresentFilesName || spec.Readonly {
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
	if len(schema.Required) != 1 || schema.Required[0] != "files" || schema.AdditionalProperties {
		t.Fatalf("schema contract = %#v", schema)
	}
	var fileShape struct {
		Required             []string `json:"required"`
		AdditionalProperties bool     `json:"additionalProperties"`
	}
	if err := json.Unmarshal([]byte(mustMarshal(t, schema.Properties["files"]["items"])), &fileShape); err != nil {
		t.Fatalf("file schema: %v", err)
	}
	// Files carry a workspace-relative path and a description only: no url,
	// upload, workspace_id or actor can ever enter through the model tool.
	if len(fileShape.Required) != 2 || fileShape.Required[0] != "path" || fileShape.Required[1] != "description" || fileShape.AdditionalProperties {
		t.Fatalf("file schema contract = %#v", fileShape)
	}
}

func TestPresentFilesCommitsSet(t *testing.T) {
	ops := &fakeDeliverableOps{set: domain.DeliverySet{
		ID:     "dvs_committed",
		Status: domain.DeliveryStatusPartial,
		Items: []domain.Deliverable{
			{ID: "dvl_a", Path: "a.txt", Name: "a.txt", Size: 5, SHA256: "aa"},
		},
		Failures: []domain.DeliveryFailure{{Path: "b.txt", Reason: "missing"}},
	}}
	tool := newPresentFilesTool(t, ops)
	out, err := tool.InvokableRun(context.Background(), json.RawMessage(`{"files":[{"path":"a.txt","description":"first"},{"path":"b.txt","description":"second"}],"title":"notes"}`))
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if len(ops.request.Files) != 2 || ops.request.Files[0].Path != "a.txt" || ops.request.Title != "notes" {
		t.Fatalf("request = %#v", ops.request)
	}
	var result presentFilesResult
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("result: %v", err)
	}
	if result.SetID != "dvs_committed" || result.Status != domain.DeliveryStatusPartial || len(result.Items) != 1 || len(result.Failures) != 1 {
		t.Fatalf("result = %#v", result)
	}
}

func TestPresentFilesRejectsForgedAndUnknownFields(t *testing.T) {
	ops := &fakeDeliverableOps{}
	tool := newPresentFilesTool(t, ops)
	for _, args := range []string{
		`{"files":[{"path":"a.txt","description":"d","url":"https://x"}]}`,
		`{"files":[{"path":"a.txt","description":"d","upload":true}]}`,
		`{"files":[{"path":"a.txt","description":"d","workspace_id":"/tmp"}]}`,
		`{"files":[{"path":"a.txt","description":"d","actor":"root"}]}`,
		`{"files":[{"path":"a.txt","description":"d"}],"root":"/etc"}`,
	} {
		if _, err := tool.InvokableRun(context.Background(), json.RawMessage(args)); err == nil {
			t.Fatalf("forged args %s succeeded", args)
		}
	}
	if len(ops.request.Files) != 0 {
		t.Fatal("forged call reached the service")
	}
}

func TestPresentFilesPropagatesBackendFailure(t *testing.T) {
	ops := &fakeDeliverableOps{err: errors.New("commit unavailable")}
	tool := newPresentFilesTool(t, ops)
	if _, err := tool.InvokableRun(context.Background(), json.RawMessage(`{"files":[{"path":"a.txt","description":"d"}]}`)); err == nil {
		t.Fatal("backend failure swallowed")
	}
	if _, err := tool.InvokableRun(context.Background(), json.RawMessage(`{"files":[{"path":"a.txt","description":"d"}]}`)); err == nil {
		t.Fatal("backend failure swallowed on retry")
	}
}

func TestPresentFilesUnwiredFailsClosed(t *testing.T) {
	tool := newPresentFilesTool(t, nil)
	if _, err := tool.InvokableRun(context.Background(), json.RawMessage(`{"files":[{"path":"a.txt","description":"d"}]}`)); err == nil {
		t.Fatal("unwired tool succeeded")
	}
}

func TestPresentFilesRegistryResolution(t *testing.T) {
	ops := &fakeDeliverableOps{}
	registry := Builtin(nil).WithDeliverables(ops)
	resolved, ok := registry.Lookup(PresentFilesName)
	if !ok {
		t.Fatal("present_files not discoverable")
	}
	if resolved.Spec().Readonly {
		t.Fatal("present_files must stay effectful through the policy host")
	}
}

func mustMarshal(t *testing.T, value any) string {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return string(data)
}
