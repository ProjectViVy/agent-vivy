package rpc

import (
	"context"
	"fmt"
	"math"
	"sort"
	"time"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
)

// TokenUsageSnapshot is the wire shape returned by stats/tokens. Field names
// are snake_case to match the existing UI contract.
type TokenUsageSnapshot struct {
	Period    string               `json:"period"`
	Total     tokenUsageTotal      `json:"total"`
	Models    []tokenModelShare    `json:"models"`
	Providers []tokenProviderGroup `json:"providers"`
	Timeline  []tokenTimelinePoint `json:"timeline"`
	Sessions  []tokenSessionUsage  `json:"sessions"`
}

type tokenUsageTotal struct {
	TotalInput     int     `json:"total_input"`
	TotalOutput    int     `json:"total_output"`
	TotalTokens    int     `json:"total_tokens"`
	TotalReasoning int     `json:"total_reasoning"`
	TotalCached    int     `json:"total_cached"`
	RequestCount   int     `json:"request_count"`
	TotalCostUSD   float64 `json:"total_cost_usd"`
	// CostKnown reports whether at least one usage row resolved to
	// reference pricing. False means every row was unpriced (unknown
	// model) and total_cost_usd is a zero placeholder — never read it as
	// "free".
	CostKnown bool `json:"cost_known"`
}

type tokenModelShare struct {
	Model       string  `json:"model"`
	Percentage  float64 `json:"percentage"`
	TotalTokens int     `json:"total_tokens"`
	CostUSD     float64 `json:"cost_usd"`
	CostKnown   bool    `json:"cost_known"`
}

type tokenProviderGroup struct {
	Key          string `json:"key"`
	TotalTokens  int    `json:"total_tokens"`
	RequestCount int    `json:"request_count"`
}

type tokenTimelinePoint struct {
	TimeBucket  string `json:"time_bucket"`
	Label       string `json:"label"`
	TotalInput  int    `json:"total_input"`
	TotalOutput int    `json:"total_output"`
	TotalTokens int    `json:"total_tokens"`
}

type tokenSessionUsage struct {
	ID           string  `json:"id"`
	Title        string  `json:"title"`
	Model        string  `json:"model"`
	RequestCount int     `json:"request_count"`
	TotalInput   int     `json:"total_input"`
	TotalOutput  int     `json:"total_output"`
	TotalTokens  int     `json:"total_tokens"`
	CostUSD      float64 `json:"cost_usd"`
	CostKnown    bool    `json:"cost_known"`
}

// validTokenPeriods enumerates accepted period values.
var validTokenPeriods = map[string]bool{
	"1d": true, "3d": true, "1w": true, "1m": true, "6m": true, "1y": true,
}

// sinceForPeriod computes the UTC start of the local day minus the period
// offset. tzOffsetMinutes follows JavaScript Date.getTimezoneOffset()
// convention: positive = west of UTC.
func sinceForPeriod(period string, tzOffsetMinutes int) (int64, error) {
	if !validTokenPeriods[period] {
		return 0, fmt.Errorf("invalid period: %s", period)
	}
	loc := time.FixedZone("local", -tzOffsetMinutes*60)
	now := time.Now().In(loc)
	midnight := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	var daysBack int
	switch period {
	case "1d":
		daysBack = 0
	case "3d":
		daysBack = 2
	case "1w":
		daysBack = 6
	case "1m":
		daysBack = 29
	case "6m":
		daysBack = 182
	case "1y":
		daysBack = 364
	}
	since := midnight.AddDate(0, 0, -daysBack)
	return since.UnixMilli(), nil
}

// ModelMeta resolves reference metadata for one (provider, model) route.
// Returning zero rates means unpriced/unknown — never treat that as free.
type ModelMeta func(ctx context.Context, provider, model string) domain.ModelInfo

// rowCostUSD prices one usage row. The second return is false when the
// route has no reference pricing.
func rowCostUSD(ctx context.Context, meta ModelMeta, r storage.UsageRow) (float64, bool) {
	if meta == nil {
		return 0, false
	}
	info := meta(ctx, r.Provider, r.Model)
	if info.InputPerMTokens == 0 || info.OutputPerMTokens == 0 {
		return 0, false
	}
	cost := float64(r.PromptTokens)/1e6*info.InputPerMTokens +
		float64(r.CompletionTokens)/1e6*info.OutputPerMTokens
	return math.Round(cost*1e4) / 1e4, true
}

// buildTokenSnapshot aggregates raw usage rows into the snapshot shape.
// Pure function — no I/O, fully testable.
func buildTokenSnapshot(ctx context.Context, rows []storage.UsageRow, period string, tzOffsetMinutes int, sessionLimit int, meta ModelMeta) TokenUsageSnapshot {
	snap := TokenUsageSnapshot{Period: period}
	if len(rows) == 0 {
		snap.Models = []tokenModelShare{}
		snap.Providers = []tokenProviderGroup{}
		snap.Timeline = buildEmptyTimeline(period, tzOffsetMinutes)
		snap.Sessions = []tokenSessionUsage{}
		return snap
	}

	// Totals (cost from priced rows only; unpriced rows never read as free)
	for _, r := range rows {
		snap.Total.TotalInput += r.PromptTokens
		snap.Total.TotalOutput += r.CompletionTokens
		snap.Total.TotalTokens += r.TotalTokens
		snap.Total.TotalReasoning += r.ReasoningTokens
		snap.Total.TotalCached += r.CachedTokens
		snap.Total.RequestCount++
		if cost, ok := rowCostUSD(ctx, meta, r); ok {
			snap.Total.TotalCostUSD += cost
			snap.Total.CostKnown = true
		}
	}
	snap.Total.TotalCostUSD = math.Round(snap.Total.TotalCostUSD*1e4) / 1e4

	// Model distribution
	type modelAgg struct {
		tokens    int
		cost      float64
		costKnown bool
	}
	modelAggs := make(map[string]*modelAgg)
	for _, r := range rows {
		key := r.Model
		if key == "" {
			key = "unknown"
		}
		agg, ok := modelAggs[key]
		if !ok {
			agg = &modelAgg{}
			modelAggs[key] = agg
		}
		agg.tokens += r.TotalTokens
		if cost, ok := rowCostUSD(ctx, meta, r); ok {
			agg.cost += cost
			agg.costKnown = true
		}
	}
	totalTokens := snap.Total.TotalTokens
	for model, agg := range modelAggs {
		pct := 0.0
		if totalTokens > 0 {
			pct = math.Round(float64(agg.tokens)*1000/float64(totalTokens)) / 10
		}
		snap.Models = append(snap.Models, tokenModelShare{
			Model: model, Percentage: pct, TotalTokens: agg.tokens,
			CostUSD: math.Round(agg.cost*1e4) / 1e4, CostKnown: agg.costKnown,
		})
	}
	sort.Slice(snap.Models, func(i, j int) bool {
		if snap.Models[i].TotalTokens != snap.Models[j].TotalTokens {
			return snap.Models[i].TotalTokens > snap.Models[j].TotalTokens
		}
		return snap.Models[i].Model < snap.Models[j].Model
	})

	// Provider grouping
	provData := make(map[string]*tokenProviderGroup)
	for _, r := range rows {
		key := r.Provider
		if key == "" {
			key = "unknown"
		}
		g, ok := provData[key]
		if !ok {
			g = &tokenProviderGroup{Key: key}
			provData[key] = g
		}
		g.TotalTokens += r.TotalTokens
		g.RequestCount++
	}
	for _, g := range provData {
		snap.Providers = append(snap.Providers, *g)
	}
	sort.Slice(snap.Providers, func(i, j int) bool {
		if snap.Providers[i].TotalTokens != snap.Providers[j].TotalTokens {
			return snap.Providers[i].TotalTokens > snap.Providers[j].TotalTokens
		}
		return snap.Providers[i].Key < snap.Providers[j].Key
	})

	// Timeline
	snap.Timeline = buildTimeline(rows, period, tzOffsetMinutes)

	// Sessions
	snap.Sessions = buildSessionList(ctx, rows, sessionLimit, meta)

	return snap
}

func buildEmptyTimeline(period string, tzOffsetMinutes int) []tokenTimelinePoint {
	buckets := timelineBuckets(period, tzOffsetMinutes)
	points := make([]tokenTimelinePoint, len(buckets))
	for i, b := range buckets {
		points[i] = tokenTimelinePoint{TimeBucket: b.key, Label: b.label}
	}
	return points
}

type bucketDef struct {
	key   string
	label string
	start int64 // unix milli inclusive
	end   int64 // unix milli exclusive
}

func timelineBuckets(period string, tzOffsetMinutes int) []bucketDef {
	loc := time.FixedZone("local", -tzOffsetMinutes*60)
	now := time.Now().In(loc)
	midnight := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)

	var daysBack int
	switch period {
	case "1d":
		daysBack = 0
	case "3d":
		daysBack = 2
	case "1w":
		daysBack = 6
	case "1m":
		daysBack = 29
	case "6m":
		daysBack = 182
	case "1y":
		daysBack = 364
	}
	since := midnight.AddDate(0, 0, -daysBack)

	var interval time.Duration
	var labelFmt string
	switch period {
	case "1d":
		interval = 30 * time.Minute
		labelFmt = "15:04"
	case "3d":
		interval = time.Hour
		labelFmt = "15:04"
	default:
		interval = 24 * time.Hour
		labelFmt = "1/2"
	}

	var buckets []bucketDef
	cur := since
	for cur.Before(now.Add(time.Millisecond)) {
		next := cur.Add(interval)
		buckets = append(buckets, bucketDef{
			key:   cur.UTC().Format(time.RFC3339),
			label: cur.In(loc).Format(labelFmt),
			start: cur.UnixMilli(),
			end:   next.UnixMilli(),
		})
		cur = next
	}
	return buckets
}

func buildTimeline(rows []storage.UsageRow, period string, tzOffsetMinutes int) []tokenTimelinePoint {
	buckets := timelineBuckets(period, tzOffsetMinutes)
	type accum struct {
		input  int
		output int
		total  int
	}
	data := make([]accum, len(buckets))
	for _, r := range rows {
		for i, b := range buckets {
			if r.CreatedAt >= b.start && r.CreatedAt < b.end {
				data[i].input += r.PromptTokens
				data[i].output += r.CompletionTokens
				data[i].total += r.TotalTokens
				break
			}
		}
	}
	points := make([]tokenTimelinePoint, len(buckets))
	for i, b := range buckets {
		points[i] = tokenTimelinePoint{
			TimeBucket:  b.key,
			Label:       b.label,
			TotalInput:  data[i].input,
			TotalOutput: data[i].output,
			TotalTokens: data[i].total,
		}
	}
	return points
}

func buildSessionList(ctx context.Context, rows []storage.UsageRow, limit int, meta ModelMeta) []tokenSessionUsage {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	type sessAccum struct {
		title        string
		requestCount int
		totalInput   int
		totalOutput  int
		totalTokens  int
		cost         float64
		costKnown    bool
		modelTokens  map[string]int
		lastActivity int64
	}
	grouped := make(map[string]*sessAccum)
	for _, r := range rows {
		key := string(r.SessionID)
		if key == "" {
			key = "unknown"
		}
		g, ok := grouped[key]
		if !ok {
			g = &sessAccum{title: r.SessionTitle, modelTokens: make(map[string]int)}
			grouped[key] = g
		}
		g.requestCount++
		g.totalInput += r.PromptTokens
		g.totalOutput += r.CompletionTokens
		g.totalTokens += r.TotalTokens
		if cost, ok := rowCostUSD(ctx, meta, r); ok {
			g.cost += cost
			g.costKnown = true
		}
		model := r.Model
		if model == "" {
			model = "unknown"
		}
		g.modelTokens[model] += r.TotalTokens
		if r.CreatedAt > g.lastActivity {
			g.lastActivity = r.CreatedAt
		}
	}

	sessions := make([]tokenSessionUsage, 0, len(grouped))
	for id, g := range grouped {
		primaryModel := ""
		bestTokens := 0
		for m, t := range g.modelTokens {
			if t > bestTokens || (t == bestTokens && (primaryModel == "" || m < primaryModel)) {
				bestTokens = t
				primaryModel = m
			}
		}
		sessions = append(sessions, tokenSessionUsage{
			ID:           id,
			Title:        g.title,
			Model:        primaryModel,
			RequestCount: g.requestCount,
			TotalInput:   g.totalInput,
			TotalOutput:  g.totalOutput,
			TotalTokens:  g.totalTokens,
			CostUSD:      math.Round(g.cost*1e4) / 1e4,
			CostKnown:    g.costKnown,
		})
	}
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].TotalTokens > sessions[j].TotalTokens
	})
	if len(sessions) > limit {
		sessions = sessions[:limit]
	}
	return sessions
}

// statsTokensHandler implements the stats/tokens RPC method.
func (h *controlHandler) statsTokens(ctx context.Context, request Request) (any, *Error) {
	if h.deps.TokenUsage == nil {
		return nil, &Error{Code: MethodNotFound, Message: "token usage store is not configured"}
	}
	var params struct {
		Period       string `json:"period"`
		TZOffsetMin  int    `json:"tz_offset_minutes"`
		SessionLimit int    `json:"session_limit"`
	}
	if err := decodeParams(request, &params); err != nil {
		return nil, err
	}
	if params.Period == "" {
		params.Period = "1d"
	}
	sinceMs, err := sinceForPeriod(params.Period, params.TZOffsetMin)
	if err != nil {
		return nil, &Error{Code: InvalidParams, Message: err.Error()}
	}
	rows, err := h.deps.TokenUsage.ListModelUsage(ctx, sinceMs)
	if err != nil {
		return nil, internalError(err)
	}
	return buildTokenSnapshot(ctx, rows, params.Period, params.TZOffsetMin, params.SessionLimit, h.deps.ModelMeta), nil
}
