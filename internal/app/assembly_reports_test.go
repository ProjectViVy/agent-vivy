package app

import (
	"context"
	"path/filepath"
	"testing"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage/sqlite"
)

// TestReportPurposePrecedesObserverDelivery proves the ingest exclusion
// predicate suppresses report-purpose runs before any payload reaches the
// memory or cognitive observers, while ordinary runs stay deliverable.
func TestReportPurposePrecedesObserverDelivery(t *testing.T) {
	ctx := context.Background()
	backend, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "obs.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer backend.Close()
	if err := backend.CreateSession(ctx, domain.Session{ID: "sess-obs", Title: "s", CreatedAt: 1}); err != nil {
		t.Fatal(err)
	}
	for _, run := range []domain.Run{
		{ID: "rep-1", SessionID: "sess-obs", Status: domain.RunCompleted, Kind: domain.RunKindWorkflow,
			RootID: "rep-1", Purpose: domain.RunPurposeReport, CreatedAt: 1},
		{ID: "plain-1", SessionID: "sess-obs", Status: domain.RunCompleted, Kind: domain.RunKindPrimary,
			RootID: "plain-1", CreatedAt: 1},
	} {
		if err := backend.CreateRun(ctx, run); err != nil {
			t.Fatal(err)
		}
	}
	exclude := ingestExclusionPredicate(backend)
	if excluded, err := exclude(ctx, "rep-1"); err != nil || !excluded {
		t.Fatalf("report run must be excluded from observer delivery: %v %v", excluded, err)
	}
	if excluded, err := exclude(ctx, "plain-1"); err != nil || excluded {
		t.Fatalf("ordinary run must stay deliverable: %v %v", excluded, err)
	}
}
