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

// Valid reports whether the mode is one of the three known values.
func (m ThinkingMode) Valid() bool {
	switch m {
	case ThinkingModeAuto, ThinkingModeOn, ThinkingModeOff:
		return true
	}
	return false
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
