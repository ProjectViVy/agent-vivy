package provider

import (
	"context"
	"fmt"
	"os"

	einoopenai "github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/model"

	"agent-vivy/internal/domain"
)

// openaiRef wires the openai bundle to the online eino-ext OpenAI chat
// model component. Construction happens at Model() call time so the API
// key is read from the environment per request and never cached or
// persisted (D-010); no network traffic happens before the first
// Generate/Stream.
type openaiRef struct {
	bundle Bundle
}

func newOpenAIRef(b Bundle) Ref { return &openaiRef{bundle: b} }

func (r *openaiRef) Name() string { return r.bundle.Name }

// APIBaseEnvVar overrides the bundle's default_api_base when set, letting
// one binary talk to any OpenAI-compatible gateway (M4 smoke) without a
// bundle edit. It carries a URL, not a secret, and stays read inside this
// package (D-010 boundary, E3 audit).
const APIBaseEnvVar = "VIVY_API_BASE"

// resolveAPIBase picks the effective base URL: environment override wins,
// the bundle default is the fallback.
func resolveAPIBase(b Bundle) string {
	if base := os.Getenv(APIBaseEnvVar); base != "" {
		return base
	}
	return b.DefaultAPIBase
}

// knownOpenAIContextWindows maps well-known OpenAI-compatible model IDs to
// their documented context windows (in tokens). This table is conservative;
// unknown models fall back to zero (caller must use defaults).
var knownOpenAIContextWindows = map[string]int{
	"gpt-4":                  8192,
	"gpt-4-0613":             8192,
	"gpt-4-32k":              32768,
	"gpt-4-32k-0613":         32768,
	"gpt-4-turbo":            128000,
	"gpt-4-turbo-2024-04-09": 128000,
	"gpt-4o":                 128000,
	"gpt-4o-mini":            128000,
	"gpt-3.5-turbo":          16385,
	"gpt-3.5-turbo-16k":      16385,
	"o1":                     200000,
	"o1-mini":                128000,
	"o3-mini":                200000,
}

func (r *openaiRef) Model(ctx context.Context, modelID string) (model.ToolCallingChatModel, error) {
	if modelID == "" {
		modelID = r.bundle.DefaultModel
	}
	key := os.Getenv(r.bundle.EnvKey)
	if key == "" {
		return nil, &KeyMissingError{Provider: r.bundle.Name, EnvKey: r.bundle.EnvKey}
	}
	cm, err := einoopenai.NewChatModel(ctx, &einoopenai.ChatModelConfig{
		APIKey:  key,
		BaseURL: resolveAPIBase(r.bundle),
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

	ctxWindow := 0
	if cw, ok := knownOpenAIContextWindows[modelID]; ok {
		ctxWindow = cw
	}

	info := domain.ModelInfo{
		ID:              modelID,
		Provider:        r.bundle.Name,
		ContextWindow:   ctxWindow, // zero means unknown; callers use defaults
		MaxOutputTokens: 0,         // varies by model; let API decide
	}
	return info, nil
}
