package domain

import (
	"context"
	"io"
)

// ModelInfo carries metadata about a specific model route, including its
// capacity limits. This is used by the compaction engine to make informed
// decisions about when and how much to compress context.
type ModelInfo struct {
	// ID is the model identifier (e.g., "gpt-4", "claude-3-opus").
	ID string
	// Provider is the backend provider name (e.g., "openai", "anthropic").
	Provider string
	// ContextWindow is the maximum number of tokens this model can handle
	// in a single request (including prompt and response). Zero means
	// unknown; callers should use conservative defaults.
	ContextWindow int
	// MaxOutputTokens is the maximum response length. Zero means unbounded
	// or unknown.
	MaxOutputTokens int
	// InputPerMTokens, CachedInputPerMTokens, and OutputPerMTokens are
	// reference prices in USD per one million tokens. Zero means unknown —
	// cost math must treat the applicable dimension as unpriced, never free.
	InputPerMTokens       float64
	CachedInputPerMTokens float64
	OutputPerMTokens      float64
	// SupportsImages reports whether the model accepts image parts in
	// multimodal input. Zero-value (false) is the conservative default for
	// unknown models.
	SupportsImages bool
	// SupportsThinking reports whether the model accepts an explicit
	// extended-thinking request (per-call option). Zero-value (false) is
	// the conservative default: the UI hides the thinking selector and the
	// run path sends no thinking option for unknown models.
	SupportsThinking bool
	// ThinkingLevels declares the effort levels the model accepts, in any
	// order (minimal..max). Empty means the legacy auto/on/off surface —
	// the run path honors an explicitly requested level anyway.
	ThinkingLevels []string
	// DefaultThinking is the model's own default level; empty means the
	// provider default stands.
	DefaultThinking string
	// ThinkingSampling carries per-level sampling overrides (temperature,
	// top_p) merged into the request when that level is effective.
	ThinkingSampling map[string]ThinkingSampling
}

// ThinkingSampling holds the sampling overrides a model declares for one
// thinking level. Pointers keep "unset" distinct from a real 0 value.
type ThinkingSampling struct {
	Temperature *float64
	TopP        *float64
}

// Valid reports whether the ModelInfo has been properly initialized with
// at least an ID and Provider.
func (m ModelInfo) Valid() bool {
	return m.ID != "" && m.Provider != ""
}

// ChatModel is the only model shape the runtime needs. It is satisfied by
// real providers behind internal/provider; the Eino adapter seam lives in
// internal/runtime. Defined here so domain never imports Eino (D-007).
type ChatModel interface {
	Stream(ctx context.Context, input []*Message) (Stream[*Message], error)
}

// Stream is a pull-style iterator. Recv returns io.EOF once the stream
// is exhausted.
type Stream[T any] interface {
	Recv() (T, error)
}

// Collect drains a stream into a slice. It is a convenience for tests
// and adapters; production paths consume Recv directly so cancellation
// stays responsive.
func Collect[T any](s Stream[T]) ([]T, error) {
	var out []T
	for {
		v, err := s.Recv()
		if err == io.EOF {
			return out, nil
		}
		if err != nil {
			return out, err
		}
		out = append(out, v)
	}
}
