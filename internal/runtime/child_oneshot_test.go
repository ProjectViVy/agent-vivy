package runtime

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
	"agent-vivy/internal/storage/sqlite"
)

func newChildDetailsService(t *testing.T) (*Service, *sqlite.Backend) {
	t.Helper()
	backend, err := sqlite.Open(context.Background(), filepath.Join(t.TempDir(), "journal.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = backend.Close() })
	return &Service{deps: ServiceDeps{Journal: backend}}, backend
}

func TestChildRunDetailsComposesBoundedCause(t *testing.T) {
	svc, backend := newChildDetailsService(t)
	runID := domain.RunID("child_details")
	payload, err := json.Marshal(payloadChildFailed{
		CauseCategory: causeInternalError,
		Message:       "The child task could not be completed. Please retry.",
		Detail:        "tool_error: connection reset by peer",
	})
	if err != nil {
		t.Fatal(err)
	}
	validatePayloadSchema(t, "child.failed", payload)

	_, err = backend.Append(context.Background(), storage.Commit{RunID: runID, Events: []domain.RunEvent{
		{RunID: runID, Type: domain.EventChildStarted, CreatedAt: 1, PayloadVersion: 1, Payload: json.RawMessage(`{"workspace_id":"ws-1"}`)},
		{RunID: runID, Type: domain.EventChildFailed, CreatedAt: 2, PayloadVersion: 1, Payload: payload},
	}})
	if err != nil {
		t.Fatalf("append child events: %v", err)
	}

	summary, message, workspaceID, err := svc.ChildRunDetails(context.Background(), runID)
	if err != nil {
		t.Fatalf("child details: %v", err)
	}
	if summary != "" || workspaceID != "ws-1" {
		t.Fatalf("summary/workspace = %q/%q", summary, workspaceID)
	}
	if !strings.Contains(message, "The child task could not be completed") ||
		!strings.Contains(message, "\nCause: tool_error: connection reset by peer") {
		t.Fatalf("child message = %q, want the generic cause plus detail", message)
	}
}

func TestChildFailedPayloadSchemaBoundsDetail(t *testing.T) {
	tooLong := strings.Repeat("x", 1025)
	payload := []byte(`{"cause_category":"internal_error","message":"failed","detail":"` + tooLong + `"}`)
	if err := payloadSchemaError(t, "child.failed", payload); err == nil {
		t.Fatal("child.failed accepted a detail over 1024 bytes")
	}
}
