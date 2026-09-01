package rpc

import (
	"context"
	"testing"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
)

func TestBuildTokenSnapshotEmpty(t *testing.T) {
	snap := buildTokenSnapshot(context.Background(), nil, "1d", 0, 50, nil)
	if snap.Period != "1d" {
		t.Fatalf("period = %q, want 1d", snap.Period)
	}
	if snap.Total.RequestCount != 0 || snap.Total.TotalTokens != 0 {
		t.Fatalf("empty snapshot should have zero totals: %+v", snap.Total)
	}
	if len(snap.Models) != 0 {
		t.Fatalf("models should be empty: %+v", snap.Models)
	}
	if len(snap.Providers) != 0 {
		t.Fatalf("providers should be empty: %+v", snap.Providers)
	}
	if len(snap.Sessions) != 0 {
		t.Fatalf("sessions should be empty: %+v", snap.Sessions)
	}
	if len(snap.Timeline) == 0 {
		t.Fatal("timeline should have pre-filled buckets even when empty")
	}
}

func TestBuildTokenSnapshotAggregation(t *testing.T) {
	rows := []storage.UsageRow{
		{SessionID: "s1", SessionTitle: "Chat A", CreatedAt: 1000, PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15, ReasoningTokens: 3, Model: "deepseek-chat", Provider: "openai"},
		{SessionID: "s1", SessionTitle: "Chat A", CreatedAt: 2000, PromptTokens: 20, CompletionTokens: 10, TotalTokens: 30, ReasoningTokens: 7, Model: "deepseek-chat", Provider: "openai"},
		{SessionID: "s2", SessionTitle: "Chat B", CreatedAt: 3000, PromptTokens: 5, CompletionTokens: 3, TotalTokens: 8, ReasoningTokens: 0, Model: "gpt-4o", Provider: "openai"},
	}
	snap := buildTokenSnapshot(context.Background(), rows, "1d", 0, 50, nil)

	if snap.Total.TotalInput != 35 {
		t.Fatalf("total_input = %d, want 35", snap.Total.TotalInput)
	}
	if snap.Total.TotalOutput != 18 {
		t.Fatalf("total_output = %d, want 18", snap.Total.TotalOutput)
	}
	if snap.Total.TotalTokens != 53 {
		t.Fatalf("total_tokens = %d, want 53", snap.Total.TotalTokens)
	}
	if snap.Total.TotalReasoning != 10 {
		t.Fatalf("total_reasoning = %d, want 10", snap.Total.TotalReasoning)
	}
	if snap.Total.RequestCount != 3 {
		t.Fatalf("request_count = %d, want 3", snap.Total.RequestCount)
	}

	// Models: deepseek-chat=45, gpt-4o=8
	if len(snap.Models) != 2 {
		t.Fatalf("models count = %d, want 2", len(snap.Models))
	}
	if snap.Models[0].Model != "deepseek-chat" || snap.Models[0].TotalTokens != 45 {
		t.Fatalf("first model = %+v, want deepseek-chat/45", snap.Models[0])
	}
	if snap.Models[1].Model != "gpt-4o" || snap.Models[1].TotalTokens != 8 {
		t.Fatalf("second model = %+v, want gpt-4o/8", snap.Models[1])
	}

	// Providers: all openai
	if len(snap.Providers) != 1 {
		t.Fatalf("providers count = %d, want 1", len(snap.Providers))
	}
	if snap.Providers[0].Key != "openai" || snap.Providers[0].RequestCount != 3 {
		t.Fatalf("provider = %+v, want openai/3", snap.Providers[0])
	}

	// Sessions: s1=45 tokens, s2=8 tokens → sorted desc
	if len(snap.Sessions) != 2 {
		t.Fatalf("sessions count = %d, want 2", len(snap.Sessions))
	}
	if snap.Sessions[0].ID != "s1" || snap.Sessions[0].TotalTokens != 45 {
		t.Fatalf("first session = %+v, want s1/45", snap.Sessions[0])
	}
	if snap.Sessions[0].Model != "deepseek-chat" {
		t.Fatalf("first session primary model = %q, want deepseek-chat", snap.Sessions[0].Model)
	}
	if snap.Sessions[1].ID != "s2" || snap.Sessions[1].TotalTokens != 8 {
		t.Fatalf("second session = %+v, want s2/8", snap.Sessions[1])
	}
}

func TestBuildTokenSnapshotSessionLimit(t *testing.T) {
	rows := make([]storage.UsageRow, 10)
	for i := range rows {
		rows[i] = storage.UsageRow{
			SessionID: "s", CreatedAt: int64(i * 1000),
			PromptTokens: 1, CompletionTokens: 1, TotalTokens: 2,
			Model: "m", Provider: "p",
		}
	}
	snap := buildTokenSnapshot(context.Background(), rows, "1d", 0, 3, nil)
	if len(snap.Sessions) > 3 {
		t.Fatalf("sessions = %d, want <= 3 (limit)", len(snap.Sessions))
	}
}

func TestSinceForPeriod(t *testing.T) {
	_, err := sinceForPeriod("invalid", 0)
	if err == nil {
		t.Fatal("expected error for invalid period")
	}
	for _, p := range []string{"1d", "3d", "1w", "1m", "6m", "1y"} {
		ms, err := sinceForPeriod(p, 0)
		if err != nil {
			t.Fatalf("period %q: %v", p, err)
		}
		if ms <= 0 {
			t.Fatalf("period %q: since = %d, want > 0", p, ms)
		}
	}
}

func TestBuildTokenSnapshotMissingRunStarted(t *testing.T) {
	rows := []storage.UsageRow{
		{SessionID: "s1", CreatedAt: 1000, PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15, Model: "", Provider: ""},
	}
	snap := buildTokenSnapshot(context.Background(), rows, "1d", 0, 50, nil)
	if snap.Total.RequestCount != 1 {
		t.Fatalf("request_count = %d, want 1", snap.Total.RequestCount)
	}
	// Model should be "unknown" when empty
	if len(snap.Models) != 1 || snap.Models[0].Model != "unknown" {
		t.Fatalf("models = %+v, want [{unknown ...}]", snap.Models)
	}
	if len(snap.Providers) != 1 || snap.Providers[0].Key != "unknown" {
		t.Fatalf("providers = %+v, want [{unknown ...}]", snap.Providers)
	}
}

// TestBuildTokenSnapshotCost covers D9 cost math: priced rows sum per
// model/session/total, unpriced rows are excluded (never read as free),
// and the cost_known flags distinguish the two.
func TestBuildTokenSnapshotCost(t *testing.T) {
	ctx := context.Background()
	meta := func(_ context.Context, provider, model string) domain.ModelInfo {
		if model == "gpt-4o" {
			return domain.ModelInfo{ID: model, Provider: provider, InputPerMTokens: 2.5, OutputPerMTokens: 10.0}
		}
		return domain.ModelInfo{}
	}
	rows := []storage.UsageRow{
		{SessionID: "s1", SessionTitle: "priced", Model: "gpt-4o", Provider: "openai",
			PromptTokens: 1_000_000, CompletionTokens: 100_000, TotalTokens: 1_100_000, CachedTokens: 400_000},
		{SessionID: "s2", SessionTitle: "unpriced", Model: "custom-model", Provider: "openai",
			PromptTokens: 1_000_000, CompletionTokens: 1_000_000, TotalTokens: 2_000_000},
	}
	snap := buildTokenSnapshot(ctx, rows, "1m", 0, 50, meta)

	if !snap.Total.CostKnown {
		t.Fatal("total cost must be known when any row is priced")
	}
	// priced row: 1M*2.5/1M + 0.1M*10/1M = 2.5 + 1.0 = 3.5
	if snap.Total.TotalCostUSD != 3.5 {
		t.Fatalf("total cost = %v, want 3.5", snap.Total.TotalCostUSD)
	}
	if snap.Total.TotalCached != 400_000 {
		t.Fatalf("total cached = %d, want 400000", snap.Total.TotalCached)
	}
	if len(snap.Models) != 2 {
		t.Fatalf("models = %d, want 2", len(snap.Models))
	}
	byModel := map[string]tokenModelShare{}
	for _, m := range snap.Models {
		byModel[m.Model] = m
	}
	if m := byModel["gpt-4o"]; !m.CostKnown || m.CostUSD != 3.5 {
		t.Fatalf("gpt-4o share = %+v, want cost 3.5 known", m)
	}
	if m := byModel["custom-model"]; m.CostKnown || m.CostUSD != 0 {
		t.Fatalf("custom-model share = %+v, want unpriced (not free)", m)
	}
	if len(snap.Sessions) != 2 {
		t.Fatalf("sessions = %d, want 2", len(snap.Sessions))
	}
	bySession := map[string]tokenSessionUsage{}
	for _, s := range snap.Sessions {
		bySession[s.ID] = s
	}
	if s := bySession["s1"]; !s.CostKnown || s.CostUSD != 3.5 {
		t.Fatalf("priced session = %+v", s)
	}
	if s := bySession["s2"]; s.CostKnown || s.CostUSD != 0 {
		t.Fatalf("unpriced session = %+v", s)
	}

	// All-unpriced snapshot: zero cost with cost_known=false.
	allUnpriced := buildTokenSnapshot(ctx, rows[:1:1], "1d", 0, 50, func(context.Context, string, string) domain.ModelInfo {
		return domain.ModelInfo{}
	})
	if allUnpriced.Total.CostKnown || allUnpriced.Total.TotalCostUSD != 0 {
		t.Fatalf("all-unpriced total = %+v, want known=false cost=0", allUnpriced.Total)
	}

	// Nil resolver: nothing is ever priced.
	nilMeta := buildTokenSnapshot(ctx, rows, "1d", 0, 50, nil)
	if nilMeta.Total.CostKnown || nilMeta.Total.TotalCostUSD != 0 {
		t.Fatalf("nil-resolver total = %+v", nilMeta.Total)
	}
}
