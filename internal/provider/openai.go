package provider

import (
	"context"
	"fmt"
	"strings"

	einoopenai "github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/model"

	"agent-vivy/internal/domain"
)

// openaiRef wires the openai bundle to the online eino-ext OpenAI chat
// model component. Construction happens at Model() call time from the
// supplied ModelSpec; no network traffic happens before the first
// Generate/Stream.
type openaiRef struct {
	bundle Bundle
}

func newOpenAIRef(b Bundle) Ref { return &openaiRef{bundle: b} }

func (r *openaiRef) Name() string { return r.bundle.Name }

// APIBaseEnvVar is the process environment name that freezes a temporary
// gateway URL for one process. Resolver reads it; this package does not.
const APIBaseEnvVar = "VIVY_API_BASE"

// openAIModelMeta is the static reference metadata for one well-known
// OpenAI-compatible model id (D9: metadata logic lives with the provider
// catalog, not a second store). Prices are published reference values in
// USD per one million tokens at the time of writing; a custom gateway
// reselling the same model id may differ. Zero values mean unknown and
// callers must use conservative defaults — never treat unpriced as free.
type openAIModelMeta struct {
	contextWindow  int
	inputPerMTok   float64
	outputPerMTok  float64
	supportsImages bool
}

// knownOpenAIModels maps well-known model IDs to their reference metadata.
// Unknown models fall back to the zero value (caller must use defaults).
var knownOpenAIModels = map[string]openAIModelMeta{
	"gpt-4":                  {contextWindow: 8192, inputPerMTok: 30.0, outputPerMTok: 60.0},
	"gpt-4-0613":             {contextWindow: 8192, inputPerMTok: 30.0, outputPerMTok: 60.0},
	"gpt-4-32k":              {contextWindow: 32768, inputPerMTok: 30.0, outputPerMTok: 60.0},
	"gpt-4-32k-0613":         {contextWindow: 32768, inputPerMTok: 30.0, outputPerMTok: 60.0},
	"gpt-4-turbo":            {contextWindow: 128000, inputPerMTok: 10.0, outputPerMTok: 30.0, supportsImages: true},
	"gpt-4-turbo-2024-04-09": {contextWindow: 128000, inputPerMTok: 10.0, outputPerMTok: 30.0, supportsImages: true},
	"gpt-4o":                 {contextWindow: 128000, inputPerMTok: 2.5, outputPerMTok: 10.0, supportsImages: true},
	"gpt-4o-mini":            {contextWindow: 128000, inputPerMTok: 0.15, outputPerMTok: 0.6, supportsImages: true},
	"gpt-3.5-turbo":          {contextWindow: 16385, inputPerMTok: 0.5, outputPerMTok: 1.5},
	"gpt-3.5-turbo-16k":      {contextWindow: 16385, inputPerMTok: 0.5, outputPerMTok: 1.5},
	"o1":                     {contextWindow: 200000, inputPerMTok: 15.0, outputPerMTok: 60.0, supportsImages: true},
	"o1-mini":                {contextWindow: 128000, inputPerMTok: 1.1, outputPerMTok: 4.4},
	"o3-mini":                {contextWindow: 200000, inputPerMTok: 1.1, outputPerMTok: 4.4},
}

func (r *openaiRef) Model(ctx context.Context, spec ModelSpec) (model.ToolCallingChatModel, error) {
	modelID := strings.TrimSpace(spec.ID)
	if modelID == "" {
		modelID = r.bundle.DefaultModel
	}
	key := strings.TrimSpace(spec.APIKey)
	if key == "" {
		return nil, &KeyMissingError{Provider: r.bundle.Name, EnvKey: r.bundle.EnvKey}
	}
	baseURL := strings.TrimSpace(spec.BaseURL)
	if baseURL == "" {
		baseURL = r.bundle.DefaultAPIBase
	}
	cm, err := einoopenai.NewChatModel(ctx, &einoopenai.ChatModelConfig{
		APIKey:  key,
		BaseURL: baseURL,
		Model:   modelID,
	})
	if err != nil {
		return nil, fmt.Errorf("provider %s: construct openai model %q: %w", r.bundle.Name, modelID, err)
	}
	return cm, nil
}

func (r *openaiRef) ModelInfo(_ context.Context, modelID string) (domain.ModelInfo, error) {
	if modelID == "" {
		modelID = r.bundle.DefaultModel
	}

	meta := knownOpenAIModels[modelID]

	info := domain.ModelInfo{
		ID:               modelID,
		Provider:         r.bundle.Name,
		ContextWindow:    meta.contextWindow, // zero means unknown; callers use defaults
		MaxOutputTokens:  0,                  // varies by model; let API decide
		InputPerMTokens:  meta.inputPerMTok,
		OutputPerMTokens: meta.outputPerMTok,
		SupportsImages:   meta.supportsImages,
	}
	return info, nil
}
