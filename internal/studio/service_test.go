package studio

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
	"agent-vivy/internal/storage/sqlite"
)

func newStudio(t *testing.T) (*Service, *sqlite.Backend) {
	t.Helper()
	backend, err := sqlite.Open(context.Background(), filepath.Join(t.TempDir(), "studio.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = backend.Close() })
	return NewService(backend), backend
}

func TestListGenerationsEmpty(t *testing.T) {
	svc, _ := newStudio(t)
	got, err := svc.ListGenerations(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || len(got) != 0 {
		t.Fatalf("empty list = %#v, want []", got)
	}
}

func TestCreateThenListGeneration(t *testing.T) {
	svc, _ := newStudio(t)
	ctx := context.Background()
	created, err := svc.CreateGeneration(ctx, domain.Generation{
		ArtifactSHA256: "abc",
		Recipe:         domain.AssemblyRecipe{Loop: "eino", World: "sandbox", Tools: []string{"notes"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.Phase != domain.GenerationBuilt || created.ID == "" {
		t.Fatalf("created = %+v", created)
	}
	listed, err := svc.ListGenerations(ctx)
	if err != nil || len(listed) != 1 || listed[0].ID != created.ID {
		t.Fatalf("list = %+v, err %v", listed, err)
	}
	if listed[0].Recipe.Loop != "eino" || len(listed[0].Recipe.Plugins) != 0 {
		t.Fatalf("recipe = %+v", listed[0].Recipe)
	}
}

func TestPromoteRequiresEval(t *testing.T) {
	svc, _ := newStudio(t)
	ctx := context.Background()
	from, err := svc.CreateGeneration(ctx, domain.Generation{ArtifactSHA256: "from"})
	if err != nil {
		t.Fatal(err)
	}
	to, err := svc.CreateGeneration(ctx, domain.Generation{ParentID: from.ID, ArtifactSHA256: "to"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = svc.Promote(ctx, from.ID, to.ID, "", domain.PromotionActorHuman)
	if !errors.Is(err, ErrNotReady) {
		t.Fatalf("promote without eval = %v, want ErrNotReady", err)
	}
}

func TestPromoteFirstWriterWins(t *testing.T) {
	svc, _ := newStudio(t)
	ctx := context.Background()
	from, err := svc.CreateGeneration(ctx, domain.Generation{ArtifactSHA256: "from"})
	if err != nil {
		t.Fatal(err)
	}
	to, err := svc.CreateGeneration(ctx, domain.Generation{ParentID: from.ID, ArtifactSHA256: "to"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.RecordEval(ctx, domain.EvalRun{
		CandidateID: to.ID, BaselineID: from.ID, Suite: "s1", Verdict: domain.EvalBetter,
	}); err != nil {
		t.Fatal(err)
	}
	first, err := svc.Promote(ctx, from.ID, to.ID, "", domain.PromotionActorHuman)
	if err != nil {
		t.Fatal(err)
	}
	if first.AppliesAt != domain.PromotionAppliesNextLaunch || first.Actor != domain.PromotionActorHuman {
		t.Fatalf("promotion = %+v", first)
	}
	_, err = svc.Promote(ctx, from.ID, to.ID, "", domain.PromotionActorHuman)
	if !errors.Is(err, storage.ErrConflict) {
		t.Fatalf("second promote = %v, want ErrConflict", err)
	}
	events, err := svc.ListStudioEvents(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) < 3 {
		t.Fatalf("studio events = %d, want at least created+eval+promote", len(events))
	}
	if events[0].Type != domain.StudioGenerationCreated || events[len(events)-1].Type != domain.StudioPromotionAccepted {
		t.Fatalf("event order = %+v", events)
	}
}

func TestPromoteRejectsNonHuman(t *testing.T) {
	svc, _ := newStudio(t)
	_, err := svc.Promote(context.Background(), "from", "to", "", "rule:auto")
	if !errors.Is(err, ErrNotHuman) {
		t.Fatalf("err = %v, want ErrNotHuman", err)
	}
}

func TestRejectDoesNotCreatePromotion(t *testing.T) {
	svc, _ := newStudio(t)
	ctx := context.Background()
	from, err := svc.CreateGeneration(ctx, domain.Generation{ArtifactSHA256: "from"})
	if err != nil {
		t.Fatal(err)
	}
	to, err := svc.CreateGeneration(ctx, domain.Generation{ParentID: from.ID, ArtifactSHA256: "to"})
	if err != nil {
		t.Fatal(err)
	}
	got, err := svc.Reject(ctx, to.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Phase != domain.GenerationRejected {
		t.Fatalf("phase = %s", got.Phase)
	}
	promos, err := svc.ListPromotions(ctx)
	if err != nil || len(promos) != 0 {
		t.Fatalf("promotions = %+v, err %v", promos, err)
	}
	if _, err := svc.Reject(ctx, to.ID); !errors.Is(err, ErrAlreadyDecided) {
		t.Fatalf("second reject = %v", err)
	}
	other, err := svc.CreateGeneration(ctx, domain.Generation{ParentID: from.ID, ArtifactSHA256: "other"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.RecordEval(ctx, domain.EvalRun{
		CandidateID: other.ID, BaselineID: from.ID, Suite: "s1", Verdict: domain.EvalBetter,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Promote(ctx, from.ID, other.ID, "", domain.PromotionActorHuman); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Reject(ctx, other.ID); !errors.Is(err, ErrAlreadyDecided) {
		t.Fatalf("reject promoted = %v", err)
	}
}
