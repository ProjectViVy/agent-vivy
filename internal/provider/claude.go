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

// claudeRef wires one embedded anthropic-messages endpoint to the online
// eino-ext Claude component (Anthropic Messages API). Construction happens at
// Model() call time from the supplied ModelSpec; no network traffic happens
// before the first Generate/Stream.
//
// A zero vendor means the sealed, vendor-neutral adapter returned by
// Catalog.Adapter: there is no address, default model or environment key to
// fall back to, so every one of them has to come from the ModelSpec.
type claudeRef struct {
	vendor   Vendor
	endpoint Endpoint
}

func newClaudeRef(vendor Vendor, endpoint Endpoint) Ref {
	return &claudeRef{vendor: vendor, endpoint: endpoint}
}

func (r *claudeRef) Name() string {
	if r.vendor.Name == "" {
		return AdapterAnthropicMessages
	}
	return r.vendor.Name
}

func (r *claudeRef) Model(ctx context.Context, spec ModelSpec) (model.ToolCallingChatModel, error) {
	modelID := strings.TrimSpace(spec.ID)
	if modelID == "" {
		modelID = r.endpoint.DefaultModel
	}
	key := strings.TrimSpace(spec.APIKey)
	// Refuse before any SDK construction: a zero-value client would fall
	// back to reading ANTHROPIC_API_KEY from the process environment,
	// which violates D-010 (credentials travel with the spec only).
	if key == "" {
		return nil, &KeyMissingError{Provider: r.Name(), EnvKey: r.vendor.EnvKey}
	}
	baseURL := strings.TrimSpace(spec.BaseURL)
	if baseURL == "" {
		baseURL = r.endpoint.BaseURL
	}
	cfg := &einoclaude.Config{
		APIKey:    key,
		Model:     modelID,
		BaseURL:   &baseURL,
		MaxTokens: claudeDefaultMaxTokens,
	}
	// The endpoint's cache flag is the first consumer of the data-driven
	// prompt-caching field: automatic cache breakpoints on system + tools +
	// last message (SDK-default 5m TTL). Crush's system + last-2-messages
	// placement is a known, accepted difference (§8.5).
	if r.endpoint.SupportsPromptCaching {
		cfg.AutoCacheControl = &einoclaude.CacheControl{}
	}
	cm, err := einoclaude.NewChatModel(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("provider %s: construct claude model %q: %w", r.Name(), modelID, err)
	}
	return cm, nil
}

// ModelInfo reports the metadata the embedded data declares for modelID.
// Zero values mean unknown and callers must use conservative defaults — never
// treat an unpriced model as free.
func (r *claudeRef) ModelInfo(_ context.Context, modelID string) (domain.ModelInfo, error) {
	if modelID == "" {
		modelID = r.endpoint.DefaultModel
	}
	meta, _ := r.endpoint.Model(modelID)
	return domain.ModelInfo{
		ID:               modelID,
		Provider:         r.Name(),
		ContextWindow:    meta.ContextWindow, // zero means unknown; callers use defaults
		MaxOutputTokens:  0,                  // varies by model; ref default covers the protocol floor
		InputPerMTokens:  meta.InputPerMTok,
		OutputPerMTokens: meta.OutputPerMTok,
		SupportsImages:   meta.SupportsImages,
		SupportsThinking: meta.SupportsThinking,
	}, nil
}
