package storage

import (
	"encoding/json"
	"sort"

	"agent-vivy/internal/domain"
)

// CanonicalUsageEvent is one journaled event relevant to the usage
// projection, in per-run seq order. The projection only consumes
// run.started, model.request, model.usage and model.call.finished.
type CanonicalUsageEvent struct {
	Type      domain.EventType
	Seq       int
	CreatedAt int64 // unix milli
	Payload   json.RawMessage
}

// UsageRunInput bundles one run's canonical events with the join context
// the SQL readers already supply (session identity, run status). The
// projection is a pure fold — storage adapters fetch events, this file
// turns them into UsageRow attempts and legacy records.
type UsageRunInput struct {
	RunID        domain.RunID
	SessionID    domain.SessionID
	SessionTitle string
	RunStatus    domain.RunStatus
	Events       []CanonicalUsageEvent
}

// Attempt states projected onto UsageRow.AttemptState. The empty string
// marks a legacy (pre-lifecycle) usage record with no call identity.
const (
	AttemptActive      = "active"      // started, not settled, run still live
	AttemptSettled     = "settled"     // model.call.finished status=completed
	AttemptFailed      = "failed"      // model.call.finished status=failed
	AttemptCancelled   = "cancelled"   // model.call.finished status=cancelled
	AttemptInterrupted = "interrupted" // run terminal before any finish record
	AttemptUntracked   = "untracked"   // v2 evidence whose v3 request is absent
)

// wire payloads mirrored from internal/runtime/payloads.go for decoding.
// Field names match schemas/events/payloads/*.json.

type usageRequestWire struct {
	CallID   string `json:"call_id"`
	Mode     string `json:"mode"`
	Provider string `json:"provider"`
	Model    string `json:"model"`
	Source   string `json:"source"`
}

type usageSampleWire struct {
	CallID               string `json:"call_id"`
	UsageKind            string `json:"usage_kind"`
	Provider             string `json:"provider"`
	Model                string `json:"model"`
	Source               string `json:"source"`
	PromptTokens         int    `json:"prompt_tokens"`
	CompletionTokens     int    `json:"completion_tokens"`
	TotalTokens          int    `json:"total_tokens"`
	ReasoningTokens      *int   `json:"reasoning_tokens"`
	CacheWriteTokens     *int   `json:"cache_write_tokens"`
	CachedTokens         *int   `json:"cached_tokens"`
	NormalizationPartial *bool  `json:"normalization_partial"`
	Settlement           *bool  `json:"settlement"`
}

type usageFinishWire struct {
	CallID string `json:"call_id"`
	Status string `json:"status"`
	Usage  *struct {
		PromptTokens         int   `json:"prompt_tokens"`
		CompletionTokens     int   `json:"completion_tokens"`
		TotalTokens          int   `json:"total_tokens"`
		ReasoningTokens      *int  `json:"reasoning_tokens"`
		CacheWriteTokens     *int  `json:"cache_write_tokens"`
		CachedTokens         *int  `json:"cached_tokens"`
		NormalizationPartial *bool `json:"normalization_partial"`
	} `json:"usage"`
}

type runStartedWire struct {
	Provider string `json:"provider"`
	Model    string `json:"model"`
}

// usageCounts is the normalized per-attempt usage evidence.
type usageCounts struct {
	prompt     int
	completion int
	total      int
	reasoning  *int
	cached     *int
	write      *int
	partial    bool
	settlement bool
}

// validUsageCounts enforces the v2 normalized convention: nonnegative
// counters, total = prompt + completion, optional buckets inside their
// parent counter. Legacy v1 rows are not validated here — they keep their
// historical raw semantics.
func validUsageCounts(u usageCounts) bool {
	if u.prompt < 0 || u.completion < 0 || u.total < 0 {
		return false
	}
	if u.total != u.prompt+u.completion {
		return false
	}
	if u.reasoning != nil && (*u.reasoning < 0 || *u.reasoning > u.completion) {
		return false
	}
	if u.cached != nil && (*u.cached < 0 || *u.cached > u.prompt) {
		return false
	}
	if u.write != nil && (*u.write < 0 || *u.write > u.prompt) {
		return false
	}
	if u.write != nil && u.cached != nil && *u.write+*u.cached > u.prompt {
		return false
	}
	return true
}

type usageAttempt struct {
	req        *usageRequestWire
	reqAt      int64
	sample     usageCounts
	hasSample  bool
	sawInvalid bool
	orphanAt   int64
	finish     *usageFinishWire
}

// ProjectUsageRows folds one run's canonical events into usage rows:
// one row per observed attempt (selected by request start time) plus one
// row per legacy v1 usage record (selected by sample time, keyed by
// run_id+seq — no retrofitted identity). Replacement samples resolve to
// the latest valid sample; a later invalid report keeps the earlier valid
// evidence and marks the row partial.
func ProjectUsageRows(run UsageRunInput, sinceMilli int64) []UsageRow {
	var started runStartedWire
	attempts := make(map[string]*usageAttempt)
	var callOrder []string
	var legacy []CanonicalUsageEvent
	for _, ev := range run.Events {
		switch ev.Type {
		case domain.EventRunStarted:
			if started.Provider == "" && started.Model == "" {
				var p runStartedWire
				if json.Unmarshal(ev.Payload, &p) == nil {
					started = p
				}
			}
		case domain.EventModelRequest:
			var p usageRequestWire
			if json.Unmarshal(ev.Payload, &p) != nil || p.CallID == "" {
				continue
			}
			if _, ok := attempts[p.CallID]; !ok {
				callOrder = append(callOrder, p.CallID)
			}
			att := attempts[p.CallID]
			if att == nil {
				att = &usageAttempt{}
				attempts[p.CallID] = att
			}
			att.req = &p
			att.reqAt = ev.CreatedAt
		case domain.EventModelUsage:
			var p usageSampleWire
			if json.Unmarshal(ev.Payload, &p) != nil {
				continue
			}
			if p.CallID == "" {
				legacy = append(legacy, ev)
				continue
			}
			att := attempts[p.CallID]
			if att == nil {
				att = &usageAttempt{}
				attempts[p.CallID] = att
				callOrder = append(callOrder, p.CallID)
			}
			if att.orphanAt == 0 && att.req == nil {
				att.orphanAt = ev.CreatedAt
			}
			u := usageCounts{
				prompt:     p.PromptTokens,
				completion: p.CompletionTokens,
				total:      p.TotalTokens,
				reasoning:  p.ReasoningTokens,
				cached:     p.CachedTokens,
				write:      p.CacheWriteTokens,
				partial:    p.NormalizationPartial != nil && *p.NormalizationPartial,
			}
			if p.Settlement != nil {
				u.settlement = *p.Settlement
			}
			if !validUsageCounts(u) {
				att.sawInvalid = true
				continue
			}
			att.sample = u
			att.hasSample = true
		case domain.EventModelCallFinished:
			var p usageFinishWire
			if json.Unmarshal(ev.Payload, &p) != nil || p.CallID == "" {
				continue
			}
			att := attempts[p.CallID]
			if att == nil {
				att = &usageAttempt{}
				attempts[p.CallID] = att
				callOrder = append(callOrder, p.CallID)
			}
			if att.orphanAt == 0 && att.req == nil {
				att.orphanAt = ev.CreatedAt
			}
			att.finish = &p
		}
	}

	var out []UsageRow
	for _, callID := range callOrder {
		att := attempts[callID]
		switch {
		case att.req != nil:
			if att.reqAt < sinceMilli {
				continue
			}
		default:
			// Orphan evidence: the v3 request is outside the read prefix
			// (or was never journaled). Attribute by first evidence time.
			if att.orphanAt < sinceMilli {
				continue
			}
		}
		out = append(out, projectAttempt(run, callID, att, started))
	}
	for _, ev := range legacy {
		if ev.CreatedAt < sinceMilli {
			continue
		}
		if row, ok := projectLegacyUsage(run, ev, started); ok {
			out = append(out, row)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].CreatedAt < out[j].CreatedAt })
	return out
}

func projectAttempt(run UsageRunInput, callID string, att *usageAttempt, started runStartedWire) UsageRow {
	row := UsageRow{
		RunID:        run.RunID,
		SessionID:    run.SessionID,
		SessionTitle: run.SessionTitle,
		CallID:       callID,
	}
	switch {
	case att.finish != nil:
		switch att.finish.Status {
		case "failed":
			row.AttemptState = AttemptFailed
		case "cancelled":
			row.AttemptState = AttemptCancelled
		default:
			row.AttemptState = AttemptSettled
		}
	case att.req == nil:
		row.AttemptState = AttemptUntracked
	case run.RunStatus.Terminal():
		row.AttemptState = AttemptInterrupted
	default:
		row.AttemptState = AttemptActive
	}

	if att.req != nil {
		row.CreatedAt = att.reqAt
		row.Provider = att.req.Provider
		row.Model = att.req.Model
		row.Source = att.req.Source
	} else {
		row.CreatedAt = att.orphanAt
	}

	counts := att.sample
	hasUsage := att.hasSample
	if !hasUsage && att.finish != nil && att.finish.Usage != nil {
		u := att.finish.Usage
		counts = usageCounts{
			prompt:     u.PromptTokens,
			completion: u.CompletionTokens,
			total:      u.TotalTokens,
			reasoning:  u.ReasoningTokens,
			cached:     u.CachedTokens,
			write:      u.CacheWriteTokens,
			partial:    u.NormalizationPartial != nil && *u.NormalizationPartial,
		}
		hasUsage = validUsageCounts(counts)
	}
	// Orphan rows carry provider/model on the v2 sample itself; the fold
	// keeps them on the wire decode but has no request to attribute.
	if hasUsage {
		row.PromptTokens = counts.prompt
		row.CompletionTokens = counts.completion
		row.TotalTokens = counts.total
		if counts.reasoning != nil {
			row.ReasoningTokens = *counts.reasoning
			row.ReasoningKnown = true
		}
		if counts.cached != nil {
			row.CachedTokens = *counts.cached
			row.CachedKnown = true
		}
		if counts.write != nil {
			row.CacheWriteTokens = *counts.write
			row.CacheWriteKnown = true
		}
		row.NormalizationPartial = counts.partial || att.sawInvalid
		row.Settlement = counts.settlement
		row.HasUsage = true
		row.RequestCount = 1
	} else {
		row.NormalizationPartial = att.sawInvalid
	}
	// Summary and explicitly routed calls never inherit main-run pricing;
	// requests carry their own provider/model, so only the empty case falls
	// back to run.started context.
	if row.Provider == "" && row.Model == "" && att.req != nil {
		if att.req.Source != "summary" {
			row.Provider, row.Model = started.Provider, started.Model
		}
	}
	return row
}

func projectLegacyUsage(run UsageRunInput, ev CanonicalUsageEvent, started runStartedWire) (UsageRow, bool) {
	var p usageSampleWire
	if json.Unmarshal(ev.Payload, &p) != nil {
		return UsageRow{}, false
	}
	row := UsageRow{
		RunID:            run.RunID,
		SessionID:        run.SessionID,
		SessionTitle:     run.SessionTitle,
		CreatedAt:        ev.CreatedAt,
		PromptTokens:     p.PromptTokens,
		CompletionTokens: p.CompletionTokens,
		TotalTokens:      p.TotalTokens,
		RequestCount:     1,
		Source:           p.Source,
		HasUsage:         true,
	}
	if p.ReasoningTokens != nil {
		row.ReasoningTokens = *p.ReasoningTokens
		row.ReasoningKnown = true
	}
	if p.CachedTokens != nil {
		row.CachedTokens = *p.CachedTokens
		row.CachedKnown = true
	}
	if p.CacheWriteTokens != nil {
		row.CacheWriteTokens = *p.CacheWriteTokens
		row.CacheWriteKnown = true
	}
	row.Provider, row.Model = started.Provider, started.Model
	applyUsageAttribution(&row, p.Provider, p.Model, p.Source)
	return row, true
}

func applyUsageAttribution(row *UsageRow, provider, model, source string) {
	// Summary middleware may use an override and fail over to the main
	// model; a summary call without its own route is never priced as main.
	if source == "summary" {
		row.Provider, row.Model = "", ""
	}
	if provider != "" || model != "" {
		row.Provider, row.Model = "", ""
		if provider != "" && model != "" {
			row.Provider, row.Model = provider, model
		}
	}
}

// AggregateUsageRows applies the bounded per-session route grouping shared
// by both SQL backends: at most SessionUsageRouteMax priced route groups
// plus one conservative unknown overflow group.
func AggregateUsageRows(rows []UsageRow) []UsageRow {
	aggregates := make(map[string]UsageRow)
	var order []string
	for _, row := range rows {
		key := row.Provider + "\x00" + row.Model
		if _, exists := aggregates[key]; !exists {
			if len(aggregates) >= SessionUsageRouteMax {
				key = ""
				row.Provider, row.Model = "", ""
			}
			if _, exists := aggregates[key]; !exists {
				order = append(order, key)
			}
		}
		current := aggregates[key]
		if current.SessionID == "" {
			current = row
			current.PromptTokens, current.CompletionTokens, current.TotalTokens = 0, 0, 0
			current.ReasoningTokens, current.CachedTokens, current.RequestCount = 0, 0, 0
			current.CacheWriteTokens = 0
		}
		current.PromptTokens += row.PromptTokens
		current.CompletionTokens += row.CompletionTokens
		current.TotalTokens += row.TotalTokens
		current.ReasoningTokens += row.ReasoningTokens
		current.CachedTokens += row.CachedTokens
		current.CacheWriteTokens += row.CacheWriteTokens
		current.RequestCount += row.RequestCount
		if row.CreatedAt > current.CreatedAt {
			current.CreatedAt = row.CreatedAt
		}
		aggregates[key] = current
	}
	sort.Strings(order)
	out := make([]UsageRow, 0, len(order))
	for _, key := range order {
		out = append(out, aggregates[key])
	}
	return out
}
