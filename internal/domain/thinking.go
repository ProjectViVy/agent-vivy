package domain

import "context"

// ThinkingMode is the per-run extended-thinking preference a face may
// carry on a turn. "auto" (the zero value "": normalized to "auto") keeps
// the provider default; "on" asks the provider to enable thinking when the
// model supports it; "off" is the explicit no — which for models with
// always-on reasoning stays the provider default.
type ThinkingMode string

const (
	ThinkingModeAuto ThinkingMode = "auto"
	ThinkingModeOn   ThinkingMode = "on"
	ThinkingModeOff  ThinkingMode = "off"
)

// The seven-level thinking surface (VCP F1): "on" resolves to the model's
// declared default level, "off" and "auto" keep their aliased meaning.
const (
	ThinkingLevelMinimal ThinkingMode = "minimal"
	ThinkingLevelLow     ThinkingMode = "low"
	ThinkingLevelMedium  ThinkingMode = "medium"
	ThinkingLevelHigh    ThinkingMode = "high"
	ThinkingLevelXHigh   ThinkingMode = "xhigh"
	ThinkingLevelMax     ThinkingMode = "max"
)

// ThinkingLevelOrder lists the real effort levels in ascending order;
// aliases (auto/on/off) never appear in it.
var ThinkingLevelOrder = []ThinkingMode{
	ThinkingLevelMinimal,
	ThinkingLevelLow,
	ThinkingLevelMedium,
	ThinkingLevelHigh,
	ThinkingLevelXHigh,
	ThinkingLevelMax,
}

// Valid reports whether the mode is an alias or a real level.
func (m ThinkingMode) Valid() bool {
	switch m {
	case ThinkingModeAuto, ThinkingModeOn, ThinkingModeOff:
		return true
	}
	return m.LevelIndex() >= 0
}

// IsLevel reports whether the mode names a real effort level.
func (m ThinkingMode) IsLevel() bool { return m.LevelIndex() >= 0 }

// LevelIndex returns the mode's rank inside ThinkingLevelOrder, or -1 for
// aliases and unknown values.
func (m ThinkingMode) LevelIndex() int {
	for i, level := range ThinkingLevelOrder {
		if level == m {
			return i
		}
	}
	return -1
}

// ClampThinkingLevel bounds mode to the highest valid entry of supported
// (model-declared level names). Aliases pass through untouched, and a
// model with no declared levels honors whatever level was requested.
func ClampThinkingLevel(mode ThinkingMode, supported []string) ThinkingMode {
	if !mode.IsLevel() {
		return mode
	}
	max := ThinkingMode("")
	maxIdx := -1
	for _, name := range supported {
		if idx := ThinkingMode(name).LevelIndex(); idx > maxIdx {
			maxIdx = idx
			max = ThinkingMode(name)
		}
	}
	if maxIdx < 0 || mode.LevelIndex() <= maxIdx {
		return mode
	}
	return max
}

type thinkingModeContextKey struct{}

// WithThinkingMode returns a context carrying the run's thinking mode.
func WithThinkingMode(ctx context.Context, mode ThinkingMode) context.Context {
	return context.WithValue(ctx, thinkingModeContextKey{}, mode)
}

// ThinkingModeFromContext reads the run's thinking mode; the zero value
// "auto" is returned when absent.
func ThinkingModeFromContext(ctx context.Context) ThinkingMode {
	if mode, ok := ctx.Value(thinkingModeContextKey{}).(ThinkingMode); ok {
		return mode
	}
	return ThinkingModeAuto
}
