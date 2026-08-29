package runtime

// CompactionPolicy is the resolved, engine-visible context compression
// configuration. It is a plain immutable snapshot assembled at app level
// from config defaults + the settings.yaml overlay + the model's context
// window (see app.compactionPolicyFor). Zero MaxTokens means "unknown
// window"; EffectiveTriggerTokens falls back to 128000.
type CompactionPolicy struct {
	Enabled        bool
	MaxTokens      int
	TriggerPercent int
	KeepRecent     int
}

// fallbackContextWindowTokens is used when neither the overlay nor the
// provider catalog reports a model context window.
const fallbackContextWindowTokens = 128000

// EffectiveMaxTokens returns the model window cap used for compression
// decisions (>= 1).
func (p CompactionPolicy) EffectiveMaxTokens() int {
	if p.MaxTokens > 0 {
		return p.MaxTokens
	}
	return fallbackContextWindowTokens
}

// TriggerTokens computes the in-run token threshold at which compression
// activates: min(percent of the model window, the feed's own byte budget in
// tokens). The feed budget clamp keeps compression reachable even when the
// model window is far larger than runtime.max_context_bytes.
func (p CompactionPolicy) TriggerTokens(feedBudgetTokens int) int {
	pct := p.TriggerPercent
	if pct <= 0 {
		pct = 80
	}
	fromWindow := p.EffectiveMaxTokens() * pct / 100
	if feedBudgetTokens > 0 && feedBudgetTokens < fromWindow {
		return feedBudgetTokens
	}
	return fromWindow
}

// bytesToTokens converts a byte budget to an approximate token budget using
// the same 4-bytes-per-token heuristic as compaction.Measure.
func bytesToTokens(bytes int) int {
	return bytes / 4
}
