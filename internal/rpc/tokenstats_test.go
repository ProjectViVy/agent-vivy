package rpc

import (
	"context"
	"encoding/json"
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
		{SessionID: "s1", SessionTitle: "Chat A", CreatedAt: 1000, PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15, ReasoningTokens: 3, Model: "deepseek-flash", Provider: "deepseek"},
		{SessionID: "s1", SessionTitle: "Chat A", CreatedAt: 2000, PromptTokens: 20, CompletionTokens: 10, TotalTokens: 30, ReasoningTokens: 7, Model: "deepseek-flash", Provider: "deepseek"},
		{SessionID: "s2", SessionTitle: "Chat B", CreatedAt: 3000, PromptTokens: 5, CompletionTokens: 3, TotalTokens: 8, ReasoningTokens: 0, Model: "deepseek-chat", Provider: "deepseek"},
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

	// Models: deepseek-flash=45, deepseek-chat=8
	if len(snap.Models) != 2 {
		t.Fatalf("models count = %d, want 2", len(snap.Models))
	}
	if snap.Models[0].Model != "deepseek-flash" || snap.Models[0].TotalTokens != 45 {
		t.Fatalf("first model = %+v, want deepseek-flash/45", snap.Models[0])
	}
	if snap.Models[1].Model != "deepseek-chat" || snap.Models[1].TotalTokens != 8 {
		t.Fatalf("second model = %+v, want deepseek-chat/8", snap.Models[1])
	}

	// Providers: all deepseek
	if len(snap.Providers) != 1 {
		t.Fatalf("providers count = %d, want 1", len(snap.Providers))
	}
	if snap.Providers[0].Key != "deepseek" || snap.Providers[0].RequestCount != 3 {
		t.Fatalf("provider = %+v, want deepseek/3", snap.Providers[0])
	}

	// Sessions: s1=45 tokens, s2=8 tokens → sorted desc
	if len(snap.Sessions) != 2 {
		t.Fatalf("sessions count = %d, want 2", len(snap.Sessions))
	}
	if snap.Sessions[0].ID != "s1" || snap.Sessions[0].TotalTokens != 45 {
		t.Fatalf("first session = %+v, want s1/45", snap.Sessions[0])
	}
	if snap.Sessions[0].Model != "deepseek-flash" {
		t.Fatalf("first session primary model = %q, want deepseek-flash", snap.Sessions[0].Model)
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
		if model == "priced-model" {
			// Synthetic prices (arithmetic coverage only; real DeepSeek
			// reference prices are pinned in internal/provider metadata tests).
			return domain.ModelInfo{ID: model, Provider: provider, InputPerMTokens: 2.5, CachedInputPerMTokens: 1.25, OutputPerMTokens: 10.0}
		}
		return domain.ModelInfo{}
	}
	rows := []storage.UsageRow{
		{SessionID: "s1", SessionTitle: "priced", Model: "priced-model", Provider: "deepseek",
			PromptTokens: 1_000_000, CompletionTokens: 100_000, TotalTokens: 1_100_000, CachedTokens: 400_000},
		{SessionID: "s2", SessionTitle: "unpriced", Model: "custom-model", Provider: "deepseek",
			PromptTokens: 1_000_000, CompletionTokens: 1_000_000, TotalTokens: 2_000_000},
	}
	snap := buildTokenSnapshot(ctx, rows, "1m", 0, 50, meta)

	if snap.Total.CostKnown || snap.Total.TotalCostUSD != 0 {
		t.Fatalf("mixed priced/unpriced total must be unknown with a zero placeholder: %+v", snap.Total)
	}
	// priced row: 0.6M*2.5 + 0.4M*1.25 + 0.1M*10 = 3.0
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
	if m := byModel["priced-model"]; !m.CostKnown || m.CostUSD != 3.0 {
		t.Fatalf("priced-model share = %+v, want cost 3.0 known", m)
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
	if s := bySession["s1"]; !s.CostKnown || s.CostUSD != 3.0 {
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

	missingCacheRate := buildTokenSnapshot(ctx, rows[:1], "1d", 0, 50, func(_ context.Context, provider, model string) domain.ModelInfo {
		return domain.ModelInfo{ID: model, Provider: provider, InputPerMTokens: 2.5, OutputPerMTokens: 10}
	})
	if missingCacheRate.Total.CostKnown || missingCacheRate.Total.TotalCostUSD != 0 {
		t.Fatalf("cached usage without a cache rate was priced: %+v", missingCacheRate.Total)
	}

	invalidCached := rows[0]
	invalidCached.CachedTokens = invalidCached.PromptTokens + 1
	if _, known := rowCostUSD(ctx, meta, invalidCached); known {
		t.Fatal("cached tokens greater than prompt tokens were priced")
	}
}

// TestBuildTokenSnapshotCoverage: projection v2 distinguishes usage reports
// from observed attempts, partitions coverage honestly and never turns an
// incomplete scope into a known cost.
func TestBuildTokenSnapshotCoverage(t *testing.T) {
	ctx := context.Background()
	meta := func(_ context.Context, provider, model string) domain.ModelInfo {
		return domain.ModelInfo{ID: model, Provider: provider, InputPerMTokens: 1, CachedInputPerMTokens: 1, OutputPerMTokens: 1}
	}
	rows := []storage.UsageRow{
		// settled, fully known buckets → completed
		{CallID: "c1", AttemptState: storage.AttemptSettled, HasUsage: true, RequestCount: 1,
			ReasoningKnown: true, CachedKnown: true, PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15,
			Model: "m1", Provider: "p1", SessionID: "s1"},
		// settled without any sample → missing
		{CallID: "c2", AttemptState: storage.AttemptSettled, SessionID: "s1", Model: "m1", Provider: "p1"},
		// failed finish with valid sample → partial
		{CallID: "c3", AttemptState: storage.AttemptFailed, HasUsage: true, RequestCount: 1,
			ReasoningKnown: true, CachedKnown: true, PromptTokens: 5, CompletionTokens: 2, TotalTokens: 7,
			Model: "m1", Provider: "p1", SessionID: "s1"},
		// live stream — active, reported but never a zero-cost failure
		{CallID: "c4", AttemptState: storage.AttemptActive, HasUsage: true, RequestCount: 1,
			ReasoningKnown: true, CachedKnown: true, PromptTokens: 3, CompletionTokens: 1, TotalTokens: 4,
			Model: "m1", Provider: "p1", SessionID: "s2"},
		// settled but optional buckets unknown → completed but flags buckets
		{CallID: "c5", AttemptState: storage.AttemptSettled, HasUsage: true, RequestCount: 1,
			PromptTokens: 8, CompletionTokens: 4, TotalTokens: 12,
			Model: "m2", Provider: "p1", SessionID: "s2"},
		// legacy v1 record — counted, not retrofitted
		{SessionID: "s1", PromptTokens: 4, CompletionTokens: 2, TotalTokens: 6, Model: "m1", Provider: "p1", RequestCount: 1},
	}
	snap := buildTokenSnapshot(ctx, rows, "1d", 0, 50, meta)

	if snap.ProjectionVersion != 2 {
		t.Fatalf("projection_version = %d, want 2", snap.ProjectionVersion)
	}
	cov := snap.Coverage
	if cov.ObservedCalls != 5 || cov.CompletedWithUsage != 2 || cov.PartialUsageCalls != 1 ||
		cov.MissingUsageCalls != 1 || cov.ActiveCalls != 1 || cov.LegacyUsageRecords != 1 {
		t.Fatalf("coverage = %+v", cov)
	}
	if cov.ReportedCalls != 4 {
		t.Fatalf("reported_calls = %d, want 4 (usage reports incl. active)", cov.ReportedCalls)
	}
	if cov.ObservedCalls != cov.CompletedWithUsage+cov.PartialUsageCalls+cov.MissingUsageCalls+cov.ActiveCalls {
		t.Fatalf("coverage partition broken: %+v", cov)
	}
	if cov.State != "partial" {
		t.Fatalf("coverage state = %q, want partial", cov.State)
	}
	if len(cov.UnknownBuckets) != 2 {
		t.Fatalf("unknown_buckets = %v, want [reasoning cached]", cov.UnknownBuckets)
	}
	if cov.HiddenRetriesObservable {
		t.Fatal("provider-internal retries are never observable")
	}
	// request_count counts usage reports, not observed attempts.
	if snap.Total.RequestCount != 5 {
		t.Fatalf("request_count = %d, want 5 (4 reported + 1 legacy)", snap.Total.RequestCount)
	}
	// cost_known stays false for partial coverage even though every priced
	// row resolved — never read the zero placeholder as "free" or complete.
	if snap.Total.CostKnown || snap.Total.TotalCostUSD != 0 {
		t.Fatalf("partial coverage cost = %+v, want known=false", snap.Total)
	}
	// Per-aggregate coverage differs: s1 mixes settled+missing+failed,
	// s2 has an active call plus unknown buckets.
	bySession := map[string]tokenSessionUsage{}
	for _, s := range snap.Sessions {
		bySession[s.ID] = s
	}
	if s := bySession["s1"]; s.Coverage.State != "partial" || s.Coverage.LegacyUsageRecords != 1 {
		t.Fatalf("s1 coverage = %+v", s.Coverage)
	}
	if s := bySession["s2"]; s.Coverage.ActiveCalls != 1 || len(s.Coverage.UnknownBuckets) == 0 {
		t.Fatalf("s2 coverage = %+v", s.Coverage)
	}
}

// TestBuildTokenSnapshotCoverageStates: empty / legacy / complete states.
func TestBuildTokenSnapshotCoverageStates(t *testing.T) {
	ctx := context.Background()
	meta := func(_ context.Context, provider, model string) domain.ModelInfo {
		return domain.ModelInfo{ID: model, Provider: provider, InputPerMTokens: 1, CachedInputPerMTokens: 1, OutputPerMTokens: 1}
	}
	empty := buildTokenSnapshot(ctx, nil, "1d", 0, 50, meta)
	if empty.Coverage.State != "empty" || empty.Coverage.ObservedCalls != 0 {
		t.Fatalf("empty coverage = %+v", empty.Coverage)
	}
	legacy := buildTokenSnapshot(ctx, []storage.UsageRow{
		{SessionID: "s1", PromptTokens: 4, CompletionTokens: 2, TotalTokens: 6, Model: "m", Provider: "p", RequestCount: 1},
	}, "1d", 0, 50, meta)
	if legacy.Coverage.State != "legacy" {
		t.Fatalf("legacy coverage = %+v", legacy.Coverage)
	}
	// A pure-legacy scope can still claim known cost when every row priced.
	if !legacy.Total.CostKnown {
		t.Fatal("pure legacy priced scope should keep cost_known semantics")
	}
	complete := buildTokenSnapshot(ctx, []storage.UsageRow{
		{CallID: "c1", AttemptState: storage.AttemptSettled, HasUsage: true, RequestCount: 1,
			ReasoningKnown: true, CachedKnown: true, PromptTokens: 4, CompletionTokens: 2, TotalTokens: 6,
			Model: "m", Provider: "p", SessionID: "s1"},
	}, "1d", 0, 50, meta)
	if complete.Coverage.State != "complete" {
		t.Fatalf("complete coverage = %+v", complete.Coverage)
	}
	if !complete.Total.CostKnown {
		t.Fatalf("fully observed priced scope must be cost_known: %+v", complete.Total)
	}
}

func TestRowCostCacheWriteWithoutDeclaredRateIsUnknown(t *testing.T) {
	var row storage.UsageRow
	if err := json.Unmarshal([]byte(`{"PromptTokens":1500,"CompletionTokens":2,"CacheWriteTokens":1200,"CacheWriteKnown":true}`), &row); err != nil {
		t.Fatal(err)
	}
	meta := ModelMeta(func(context.Context, string, string) domain.ModelInfo {
		return domain.ModelInfo{InputPerMTokens: 3, CachedInputPerMTokens: 0.3, OutputPerMTokens: 15}
	})
	if cost, known := rowCostUSD(context.Background(), meta, row); known || cost != 0 {
		t.Fatalf("undeclared cache write price priced as ordinary input: %v/%v", cost, known)
	}
}
