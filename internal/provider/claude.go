package provider

import (
	"context"
	"fmt"
	"strings"

	einoclaude "github.com/cloudwego/eino-ext/components/model/claude"
	"github.com/cloudwego/eino/components/model"

	"agent-vivy/internal/domain"
)

// claudeDefaultMaxTokens is the protocol-required max_tokens sent on every
// request: the Anthropic Messages API has no "0 = API decides" mode, unlike
// the OpenAI-compatible path. 8192 is a conservative output budget across
// current-generation models; per-call model.WithMaxTokens overrides it.
const claudeDefaultMaxTokens = 8192

// claudeThinkingBudgetTokens is the extended-thinking token budget applied
// when a run asks for thinking (domain.ThinkingModeOn). It must stay below
// claudeDefaultMaxTokens: the Anthropic API requires max_tokens to exceed
// the thinking budget.
const claudeThinkingBudgetTokens = 4096

// claudeRef wires the anthropic bundle to the online eino-ext Claude
// component (Anthropic Messages API). Construction happens at Model() call
// time from the supplied ModelSpec; no network traffic happens before the
// first Generate/Stream.
type claudeRef struct {
	bundle Bundle
}

func newClaudeRef(b Bundle) Ref { return &claudeRef{bundle: b} }

func (r *claudeRef) Name() string { return r.bundle.Name }

func (r *claudeRef) Model(ctx context.Context, spec ModelSpec) (model.ToolCallingChatModel, error) {
	modelID := strings.TrimSpace(spec.ID)
	if modelID == "" {
		modelID = r.bundle.DefaultModel
	}
	key := strings.TrimSpace(spec.APIKey)
	// Refuse before any SDK construction: a zero-value client would fall
	// back to reading ANTHROPIC_API_KEY from the process environment,
	// which violates D-010 (credentials travel with the spec only).
	if key == "" {
		return nil, &KeyMissingError{Provider: r.bundle.Name, EnvKey: r.bundle.EnvKey}
	}
	baseURL := strings.TrimSpace(spec.BaseURL)
	if baseURL == "" {
		baseURL = r.bundle.DefaultAPIBase
	}
	cfg := &einoclaude.Config{
		APIKey:    key,
		Model:     modelID,
		BaseURL:   &baseURL,
		MaxTokens: claudeDefaultMaxTokens,
	}
	// First consumer of the bundle cache flag: automatic cache breakpoints
	// on system + tools + last message (SDK-default 5m TTL). Crush's system
	// + last-2-messages placement is a known, accepted difference (§8.5).
	if r.bundle.SupportsPromptCaching {
		cfg.AutoCacheControl = &einoclaude.CacheControl{}
	}
	cm, err := einoclaude.NewChatModel(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("provider %s: construct claude model %q: %w", r.bundle.Name, modelID, err)
	}
	return cm, nil
}

// anthropicModelMeta is the static reference metadata for one well-known
// Anthropic model id (D9: metadata logic lives with the provider catalog,
// not a second store). Prices are published reference values in USD per
// one million tokens at the time of writing. Zero values mean unknown and
// callers must use conservative defaults — never treat unpriced as free.
type anthropicModelMeta struct {
	contextWindow  int
	inputPerMTok   float64
	outputPerMTok  float64
	supportsImages bool
	// supportsThinking marks models that accept the Anthropic extended
	// thinking parameter (Claude 3.7 Sonnet and later generations).
	supportsThinking bool
}

var knownAnthropicModels = map[string]anthropicModelMeta{
	"claude-opus-4-5":            {contextWindow: 200000, inputPerMTok: 5.0, outputPerMTok: 25.0, supportsImages: true, supportsThinking: true},
	"claude-opus-4-1":            {contextWindow: 200000, inputPerMTok: 15.0, outputPerMTok: 75.0, supportsImages: true, supportsThinking: true},
	"claude-opus-4":              {contextWindow: 200000, inputPerMTok: 15.0, outputPerMTok: 75.0, supportsImages: true, supportsThinking: true},
	"claude-sonnet-4-5":          {contextWindow: 200000, inputPerMTok: 3.0, outputPerMTok: 15.0, supportsImages: true, supportsThinking: true},
	"claude-sonnet-4":            {contextWindow: 200000, inputPerMTok: 3.0, outputPerMTok: 15.0, supportsImages: true, supportsThinking: true},
	"claude-haiku-4-5":           {contextWindow: 200000, inputPerMTok: 1.0, outputPerMTok: 5.0, supportsImages: true, supportsThinking: true},
	"claude-3-7-sonnet":          {contextWindow: 200000, inputPerMTok: 3.0, outputPerMTok: 15.0, supportsImages: true, supportsThinking: true},
	"claude-3-7-sonnet-20250219": {contextWindow: 200000, inputPerMTok: 3.0, outputPerMTok: 15.0, supportsImages: true, supportsThinking: true},
	"claude-3-5-sonnet":          {contextWindow: 200000, inputPerMTok: 3.0, outputPerMTok: 15.0, supportsImages: true},
	"claude-3-5-sonnet-20241022": {contextWindow: 200000, inputPerMTok: 3.0, outputPerMTok: 15.0, supportsImages: true},
	"claude-3-5-haiku":           {contextWindow: 200000, inputPerMTok: 0.8, outputPerMTok: 4.0},
	"claude-3-5-haiku-20241022":  {contextWindow: 200000, inputPerMTok: 0.8, outputPerMTok: 4.0},
	"claude-3-opus":              {contextWindow: 200000, inputPerMTok: 15.0, outputPerMTok: 75.0, supportsImages: true},
	"claude-3-haiku":             {contextWindow: 200000, inputPerMTok: 0.25, outputPerMTok: 1.25},
}

func (r *claudeRef) ModelInfo(_ context.Context, modelID string) (domain.ModelInfo, error) {
	if modelID == "" {
		modelID = r.bundle.DefaultModel
	}
	meta := knownAnthropicModels[modelID]
	info := domain.ModelInfo{
		ID:               modelID,
		Provider:         r.bundle.Name,
		ContextWindow:    meta.contextWindow, // zero means unknown; callers use defaults
		MaxOutputTokens:  0,                  // varies by model; ref default covers the protocol floor
		InputPerMTokens:  meta.inputPerMTok,
		OutputPerMTokens: meta.outputPerMTok,
		SupportsImages:   meta.supportsImages,
		SupportsThinking: meta.supportsThinking,
	}
	return info, nil
}
