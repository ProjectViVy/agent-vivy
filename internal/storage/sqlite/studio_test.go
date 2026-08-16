package sqlite

import (
	"context"
	"testing"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
)

func TestStudioListEmpty(t *testing.T) {
	b := openBackend(t)
	ctx := context.Background()
	gens, err := b.ListGenerations(ctx)
	if err != nil || len(gens) != 0 {
		t.Fatalf("list generations = %v, %v", gens, err)
	}
	evals, err := b.ListEvalRuns(ctx)
	if err != nil || len(evals) != 0 {
		t.Fatalf("list evals = %v, %v", evals, err)
	}
	promos, err := b.ListPromotions(ctx)
	if err != nil || len(promos) != 0 {
		t.Fatalf("list promotions = %v, %v", promos, err)
	}
	events, err := b.ListStudioEvents(ctx)
	if err != nil || len(events) != 0 {
		t.Fatalf("list studio events = %v, %v", events, err)
	}
}

func TestStudioGetMissing(t *testing.T) {
	b := openBackend(t)
	_, err := b.GetGeneration(context.Background(), "gen_missing")
	if err != storage.ErrNotFound {
		t.Fatalf("get missing = %v, want ErrNotFound", err)
	}
}

func TestStudioPromotionUniqueFrom(t *testing.T) {
	b := openBackend(t)
	ctx := context.Background()
	if err := b.CreateGeneration(ctx, domain.Generation{
		ID: "gen_a", ArtifactSHA256: "aa", Phase: domain.GenerationBuilt, CreatedAt: 1,
	}); err != nil {
		t.Fatal(err)
	}
	if err := b.CreateGeneration(ctx, domain.Generation{
		ID: "gen_b", ParentID: "gen_a", ArtifactSHA256: "bb", Phase: domain.GenerationEvaluated, CreatedAt: 2,
	}); err != nil {
		t.Fatal(err)
	}
	first := domain.Promotion{
		ID: "pro_1", FromID: "gen_a", ToID: "gen_b", EvalID: "evl_1",
		Actor: domain.PromotionActorHuman, Phase: domain.PromotionAccepted,
		AppliesAt: domain.PromotionAppliesNextLaunch, CreatedAt: 3,
	}
	if err := b.CreatePromotion(ctx, first); err != nil {
		t.Fatal(err)
	}
	second := first
	second.ID = "pro_2"
	if err := b.CreatePromotion(ctx, second); err != storage.ErrConflict {
		t.Fatalf("second promotion = %v, want ErrConflict", err)
	}
}
