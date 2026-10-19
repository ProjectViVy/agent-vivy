package conformance

import (
	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
	"context"
	"errors"
	"testing"
)

func RunIssue58OperationProvenanceImmutable(t *testing.T, b storage.Engine) {
	ctx := context.Background()
	if err := b.CreateSession(ctx, domain.Session{ID: "prov-operation", CreatedAt: 1}); err != nil {
		t.Fatal(err)
	}
	if err := b.CreateRun(ctx, domain.Run{ID: "prov-operation-run", SessionID: "prov-operation", Status: domain.RunActive, CreatedAt: 1}); err != nil {
		t.Fatal(err)
	}
	op := conformanceToolOperation("prov-operation-run", "prov-op")
	op.ContentOrigin = "original"
	if _, _, _, err := b.AdmitToolOperation(ctx, op); err != nil {
		t.Fatal(err)
	}
	op.ContentOrigin = "notebook"
	op.ExcludeAutomaticIngest = true
	if _, _, _, err := b.AdmitToolOperation(ctx, op); !errors.Is(err, storage.ErrToolOperationConflict) {
		t.Fatalf("operation provenance identity changed: %v", err)
	}
}
