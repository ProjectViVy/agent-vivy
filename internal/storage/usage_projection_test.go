package storage

import (
	"encoding/json"
	"fmt"
	"testing"

	"agent-vivy/internal/domain"
)

func usageEv(t *testing.T, eventType domain.EventType, seq int, at int64, payload any) CanonicalUsageEvent {
	t.Helper()
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	return CanonicalUsageEvent{Type: eventType, Seq: seq, CreatedAt: at, Payload: raw}
}

func requestEv(t *testing.T, seq int, at int64, callID, source string) CanonicalUsageEvent {
	t.Helper()
	return usageEv(t, domain.EventModelRequest, seq, at, map[string]any{
		"call_id": callID, "mode": "stream", "provider": "p1", "model": "m1",
		"source": source, "digest": "d",
	})
}

func usageV2(t *testing.T, seq int, at int64, callID string, prompt, completion, total int) CanonicalUsageEvent {
	t.Helper()
	return usageEv(t, domain.EventModelUsage, seq, at, map[string]any{
		"call_id": callID, "usage_kind": "cumulative", "provider": "p1", "model": "m1",
		"source": "main", "prompt_tokens": prompt, "completion_tokens": completion,
		"total_tokens": total,
	})
}

func finishEv(t *testing.T, seq int, at int64, callID, status string) CanonicalUsageEvent {
	t.Helper()
	return usageEv(t, domain.EventModelCallFinished, seq, at, map[string]any{
		"call_id": callID, "mode": "stream", "provider": "p1", "model": "m1",
		"source": "main", "status": status, "response_complete": status == "completed",
	})
}

func startedEv(t *testing.T, seq int, at int64) CanonicalUsageEvent {
	t.Helper()
	return usageEv(t, domain.EventRunStarted, seq, at, map[string]any{
		"provider": "p1", "model": "m1",
	})
}

func legacyUsage(t *testing.T, seq int, at int64, prompt, completion, total int) CanonicalUsageEvent {
	t.Helper()
	return usageEv(t, domain.EventModelUsage, seq, at, map[string]any{
		"provider": "p1", "model": "m1", "source": "main",
		"prompt_tokens": prompt, "completion_tokens": completion, "total_tokens": total,
	})
}

func usageRun(events ...CanonicalUsageEvent) UsageRunInput {
	return UsageRunInput{
		RunID: "r1", SessionID: "s1", SessionTitle: "session",
		RunStatus: domain.RunCompleted, Events: events,
	}
}

// TestUsageProjectionLatestSamplePerAttempt: cumulative samples replace —
// 10 then 15 contributes 15, not 25; a second attempt contributes its own
// latest; folding twice is identical; legacy rows keep (run,seq) identity.
func TestUsageProjectionLatestSamplePerAttempt(t *testing.T) {
	run := usageRun(
		startedEv(t, 1, 100),
		requestEv(t, 2, 200, "c1", "main"),
		usageV2(t, 3, 210, "c1", 10, 4, 14),
		usageV2(t, 4, 220, "c1", 15, 5, 20),
		finishEv(t, 5, 230, "c1", "completed"),
		requestEv(t, 6, 240, "c2", "main"),
		usageV2(t, 7, 250, "c2", 7, 3, 10),
		finishEv(t, 8, 260, "c2", "completed"),
	)
	rows := ProjectUsageRows(run, 0)
	if len(rows) != 2 {
		t.Fatalf("rows = %d, want 2", len(rows))
	}
	if rows[0].CallID != "c1" || rows[0].TotalTokens != 20 || !rows[0].HasUsage {
		t.Fatalf("c1 row = %+v, want latest sample total 20", rows[0])
	}
	if rows[1].CallID != "c2" || rows[1].TotalTokens != 10 {
		t.Fatalf("c2 row = %+v, want total 10", rows[1])
	}
	if rows[0].CreatedAt != 200 || rows[0].AttemptState != AttemptSettled {
		t.Fatalf("c1 start/state = %d/%q, want 200/settled", rows[0].CreatedAt, rows[0].AttemptState)
	}
	if again := ProjectUsageRows(run, 0); fmt.Sprintf("%+v", again) != fmt.Sprintf("%+v", rows) {
		t.Fatal("re-fold is not deterministic")
	}

	// Legacy rows remain keyed by (run,seq): two v1 records stay two rows.
	legacy := usageRun(startedEv(t, 1, 100), legacyUsage(t, 2, 300, 1, 2, 3), legacyUsage(t, 3, 310, 4, 5, 9))
	legacyRows := ProjectUsageRows(legacy, 0)
	if len(legacyRows) != 2 {
		t.Fatalf("legacy rows = %d, want 2", len(legacyRows))
	}
	if legacyRows[0].CallID != "" || legacyRows[0].TotalTokens != 3 || legacyRows[1].TotalTokens != 9 {
		t.Fatalf("legacy rows = %+v", legacyRows)
	}
}

// TestUsageProjectionAttemptStates: nil usage vs reported zero, active vs
// settled vs failed vs cancelled vs interrupted, untracked evidence.
func TestUsageProjectionAttemptStates(t *testing.T) {
	run := usageRun(
		startedEv(t, 1, 100),
		// c1: settled with real zero usage — reported, not missing.
		requestEv(t, 2, 200, "c1", "main"),
		usageV2(t, 3, 210, "c1", 0, 0, 0),
		finishEv(t, 4, 220, "c1", "completed"),
		// c2: finished completed, no usage at all — missing, not zero.
		requestEv(t, 5, 230, "c2", "main"),
		finishEv(t, 6, 240, "c2", "completed"),
		// c3: failed finish with valid sample — partial evidence.
		requestEv(t, 7, 250, "c3", "main"),
		usageV2(t, 8, 260, "c3", 5, 2, 7),
		finishEv(t, 9, 270, "c3", "failed"),
		// c4: cancelled finish, no usage — missing.
		requestEv(t, 10, 280, "c4", "main"),
		finishEv(t, 11, 290, "c4", "cancelled"),
		// c5: request but run went terminal before finish — interrupted.
		requestEv(t, 12, 300, "c5", "main"),
		usageV2(t, 13, 310, "c5", 3, 1, 4),
		// c6: v2 evidence with no request in the prefix — untracked.
		usageV2(t, 14, 320, "c6", 2, 1, 3),
	)
	rows := ProjectUsageRows(run, 0)
	if len(rows) != 6 {
		t.Fatalf("rows = %d, want 6", len(rows))
	}
	byID := make(map[string]UsageRow)
	for _, r := range rows {
		byID[r.CallID] = r
	}
	if r := byID["c1"]; r.AttemptState != AttemptSettled || !r.HasUsage || r.TotalTokens != 0 {
		t.Fatalf("c1 = %+v, want settled with reported zero", r)
	}
	if r := byID["c2"]; r.AttemptState != AttemptSettled || r.HasUsage || r.RequestCount != 0 {
		t.Fatalf("c2 = %+v, want settled missing (RequestCount 0)", r)
	}
	if r := byID["c3"]; r.AttemptState != AttemptFailed || !r.HasUsage || r.TotalTokens != 7 {
		t.Fatalf("c3 = %+v, want failed with partial usage", r)
	}
	if r := byID["c4"]; r.AttemptState != AttemptCancelled || r.HasUsage {
		t.Fatalf("c4 = %+v, want cancelled missing", r)
	}
	if r := byID["c5"]; r.AttemptState != AttemptInterrupted || !r.HasUsage || r.TotalTokens != 4 {
		t.Fatalf("c5 = %+v, want interrupted with usage", r)
	}
	if r := byID["c6"]; r.AttemptState != AttemptUntracked || !r.HasUsage || r.TotalTokens != 3 {
		t.Fatalf("c6 = %+v, want untracked with usage", r)
	}
}

// TestUsageProjectionActiveCall: no finish on a live run stays active —
// not failed, not zero-cost.
func TestUsageProjectionActiveCall(t *testing.T) {
	run := usageRun(
		startedEv(t, 1, 100),
		requestEv(t, 2, 200, "c1", "main"),
		usageV2(t, 3, 210, "c1", 10, 4, 14),
	)
	run.RunStatus = domain.RunActive
	rows := ProjectUsageRows(run, 0)
	if len(rows) != 1 || rows[0].AttemptState != AttemptActive {
		t.Fatalf("rows = %+v, want one active attempt", rows)
	}
	if !rows[0].HasUsage || rows[0].TotalTokens != 14 {
		t.Fatalf("active row = %+v, want live sample 14", rows[0])
	}
}

// TestUsageProjectionUnknownBuckets: absent optional buckets stay unknown
// (known=false) instead of reading as exact zero.
func TestUsageProjectionUnknownBuckets(t *testing.T) {
	run := usageRun(
		startedEv(t, 1, 100),
		requestEv(t, 2, 200, "c1", "main"),
		usageEv(t, domain.EventModelUsage, 3, 210, map[string]any{
			"call_id": "c1", "usage_kind": "cumulative", "provider": "p1", "model": "m1",
			"source": "main", "prompt_tokens": 10, "completion_tokens": 4, "total_tokens": 14,
			"reasoning_tokens": 3,
		}),
		finishEv(t, 4, 220, "c1", "completed"),
	)
	rows := ProjectUsageRows(run, 0)
	if len(rows) != 1 {
		t.Fatalf("rows = %+v", rows)
	}
	r := rows[0]
	if !r.ReasoningKnown || r.ReasoningTokens != 3 {
		t.Fatalf("reasoning = %v/%d, want known 3", r.ReasoningKnown, r.ReasoningTokens)
	}
	if r.CachedKnown {
		t.Fatal("cached must stay unknown when the evidence omits it")
	}
}

// TestUsageProjectionInvalidEvidence: negative/inconsistent/subset-
// violating samples are excluded; an earlier valid sample survives and the
// row marks partial evidence rather than plausible totals.
func TestUsageProjectionInvalidEvidence(t *testing.T) {
	run := usageRun(
		startedEv(t, 1, 100),
		requestEv(t, 2, 200, "c1", "main"),
		usageV2(t, 3, 210, "c1", 10, 4, 14),
		// contradictory total: 99 != 10+4 — must not rewrite totals.
		usageV2(t, 4, 220, "c1", 10, 4, 99),
		finishEv(t, 5, 230, "c1", "completed"),
		// c2: only invalid evidence — missing usage, partial flag.
		requestEv(t, 6, 240, "c2", "main"),
		usageV2(t, 7, 250, "c2", -5, 0, 0),
		finishEv(t, 8, 260, "c2", "completed"),
	)
	rows := ProjectUsageRows(run, 0)
	byID := make(map[string]UsageRow)
	for _, r := range rows {
		byID[r.CallID] = r
	}
	if r := byID["c1"]; !r.HasUsage || r.TotalTokens != 14 || !r.NormalizationPartial {
		t.Fatalf("c1 = %+v, want valid 14 + partial", r)
	}
	if r := byID["c2"]; r.HasUsage || !r.NormalizationPartial {
		t.Fatalf("c2 = %+v, want missing + partial", r)
	}
}

// TestUsageProjectionStartTimeSelection: attempts select by request start;
// a sample crossing the boundary belongs to an attempt started before the
// window and must not enter it.
func TestUsageProjectionStartTimeSelection(t *testing.T) {
	run := usageRun(
		startedEv(t, 1, 50),
		requestEv(t, 2, 90, "c1", "main"), // starts before window
		usageV2(t, 3, 150, "c1", 10, 4, 14),
		finishEv(t, 4, 160, "c1", "completed"),
		requestEv(t, 5, 170, "c2", "main"), // starts inside window
		usageV2(t, 6, 180, "c2", 5, 2, 7),
		finishEv(t, 7, 190, "c2", "completed"),
		legacyUsage(t, 8, 100, 1, 1, 2), // legacy sample before window
		legacyUsage(t, 9, 200, 3, 3, 6), // legacy sample inside window
	)
	rows := ProjectUsageRows(run, 120)
	var ids []string
	for _, r := range rows {
		ids = append(ids, r.CallID+"/"+r.AttemptState)
	}
	if len(rows) != 2 {
		t.Fatalf("rows = %v, want c2 + one legacy", ids)
	}
	if rows[0].CallID != "c2" || rows[0].TotalTokens != 7 {
		t.Fatalf("first row = %+v, want c2", rows[0])
	}
	if rows[1].CallID != "" || rows[1].TotalTokens != 6 {
		t.Fatalf("second row = %+v, want in-window legacy", rows[1])
	}
}

// TestUsageProjectionFinishUsageEvidence: a settled call with no v2 sample
// but usage recorded on its finish record still reports usage.
func TestUsageProjectionFinishUsageEvidence(t *testing.T) {
	run := usageRun(
		startedEv(t, 1, 100),
		usageEv(t, domain.EventModelRequest, 2, 200, map[string]any{
			"call_id": "c1", "mode": "generate", "provider": "p1", "model": "m1",
			"source": "summary", "digest": "d",
		}),
		usageEv(t, domain.EventModelCallFinished, 3, 210, map[string]any{
			"call_id": "c1", "mode": "generate", "provider": "p1", "model": "m1",
			"source": "summary", "status": "completed", "response_complete": true,
			"usage": map[string]any{
				"prompt_tokens": 8, "completion_tokens": 3, "total_tokens": 11,
			},
		}),
	)
	rows := ProjectUsageRows(run, 0)
	if len(rows) != 1 || !rows[0].HasUsage || rows[0].TotalTokens != 11 {
		t.Fatalf("rows = %+v, want finish usage 11", rows)
	}
	if rows[0].AttemptState != AttemptSettled || rows[0].Source != "summary" {
		t.Fatalf("row = %+v, want settled summary", rows[0])
	}
}

// TestUsageProjectionSummaryAttribution: a summary-route request keeps its
// own provider/model; a request without route fields does not silently
// inherit main-run pricing.
func TestUsageProjectionSummaryAttribution(t *testing.T) {
	run := usageRun(
		startedEv(t, 1, 100),
		usageEv(t, domain.EventModelRequest, 2, 200, map[string]any{
			"call_id": "c1", "mode": "generate", "provider": "p-sum", "model": "m-sum",
			"source": "summary", "digest": "d",
		}),
		usageV2(t, 3, 210, "c1", 5, 2, 7),
		finishEv(t, 4, 220, "c1", "completed"),
		usageEv(t, domain.EventModelRequest, 5, 230, map[string]any{
			"call_id": "c2", "mode": "stream", "source": "child", "digest": "d",
		}),
		usageV2(t, 6, 240, "c2", 2, 1, 3),
		finishEv(t, 7, 250, "c2", "completed"),
	)
	rows := ProjectUsageRows(run, 0)
	byID := make(map[string]UsageRow)
	for _, r := range rows {
		byID[r.CallID] = r
	}
	if r := byID["c1"]; r.Provider != "p-sum" || r.Model != "m-sum" || r.Source != "summary" {
		t.Fatalf("c1 = %+v, want summary's own route", r)
	}
	if r := byID["c2"]; r.Source != "child" {
		t.Fatalf("c2 = %+v, want child source", r)
	}
}

func TestUsageProjectionPreservesMaintenanceCacheWrite(t *testing.T) {
	run := usageRun(requestEv(t, 1, 100, "warm", "maintenance"),
		usageEv(t, domain.EventModelUsage, 2, 110, map[string]any{
			"call_id": "warm", "usage_kind": "cumulative", "provider": "p1", "model": "m1", "source": "maintenance",
			"prompt_tokens": 1500, "completion_tokens": 2, "total_tokens": 1502, "cache_write_tokens": 1200,
		}), finishEv(t, 3, 120, "warm", "completed"))
	rows := ProjectUsageRows(run, 0)
	if len(rows) != 1 || rows[0].Source != "maintenance" {
		t.Fatalf("rows=%+v", rows)
	}
	raw, _ := json.Marshal(rows[0])
	var got map[string]any
	_ = json.Unmarshal(raw, &got)
	if got["CacheWriteTokens"] != float64(1200) || got["CacheWriteKnown"] != true {
		t.Fatalf("cache write accounting missing: %s", raw)
	}
}
