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
	// PerModel carries unresolved per-model overrides keyed by model ID.
	// For resolves them into the flat fields; the stored policy is always
	// the resolved form (PerModel is nil after For).
	PerModel map[string]CompactionOverride
}

// CompactionOverride overrides selected policy fields for one model. Zero
// values inherit the global policy.
type CompactionOverride struct {
	MaxTokens      int `json:"max_tokens,omitempty" yaml:"max_tokens,omitempty"`
	TriggerPercent int `json:"trigger_percent,omitempty" yaml:"trigger_percent,omitempty"`
	KeepRecent     int `json:"keep_recent,omitempty" yaml:"keep_recent,omitempty"`
}

// For returns the policy resolved against a concrete model: matching
// PerModel entries overlay the global fields. The result drops PerModel —
// it is a model-bound snapshot.
func (p CompactionPolicy) For(model string) CompactionPolicy {
	if o, ok := p.PerModel[model]; ok {
		if o.MaxTokens > 0 {
			p.MaxTokens = o.MaxTokens
		}
		if o.TriggerPercent > 0 {
			p.TriggerPercent = o.TriggerPercent
		}
		if o.KeepRecent > 0 {
			p.KeepRecent = o.KeepRecent
		}
	}
	p.PerModel = nil
	return p
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
