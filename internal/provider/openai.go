package provider

import (
	"context"
	"fmt"
	"strings"

	einoopenai "github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/model"

	"agent-vivy/internal/domain"
)

// openaiRef wires one embedded openai-completions endpoint to the online
// eino-ext OpenAI chat model component. Construction happens at Model() call
// time from the supplied ModelSpec; no network traffic happens before the
// first Generate/Stream.
type openaiRef struct {
	vendor   Vendor
	endpoint Endpoint
}

func newOpenAIRef(vendor Vendor, endpoint Endpoint) Ref {
	return &openaiRef{vendor: vendor, endpoint: endpoint}
}

func (r *openaiRef) Name() string { return r.vendor.Name }

// APIBaseEnvVar is the process environment name that freezes a temporary
// gateway URL for one process. Resolver reads it; this package does not.
const APIBaseEnvVar = "VIVY_API_BASE"

func (r *openaiRef) Model(ctx context.Context, spec ModelSpec) (model.ToolCallingChatModel, error) {
	modelID := strings.TrimSpace(spec.ID)
	if modelID == "" {
		modelID = r.endpoint.DefaultModel
	}
	key := strings.TrimSpace(spec.APIKey)
	if key == "" {
		return nil, &KeyMissingError{Provider: r.vendor.Name, EnvKey: r.vendor.EnvKey}
	}
	baseURL := strings.TrimSpace(spec.BaseURL)
	if baseURL == "" {
		baseURL = r.endpoint.BaseURL
	}
	cm, err := einoopenai.NewChatModel(ctx, &einoopenai.ChatModelConfig{
		APIKey:  key,
		BaseURL: baseURL,
		Model:   modelID,
	})
	if err != nil {
		return nil, fmt.Errorf("provider %s: construct openai model %q: %w", r.vendor.Name, modelID, err)
	}
	return cm, nil
}

// ModelInfo reports the metadata the embedded data declares for modelID.
// Zero values mean unknown and callers must use conservative defaults — never
// treat an unpriced model as free.
func (r *openaiRef) ModelInfo(_ context.Context, modelID string) (domain.ModelInfo, error) {
	if modelID == "" {
		modelID = r.endpoint.DefaultModel
	}
	meta, _ := r.endpoint.Model(modelID)
	return domain.ModelInfo{
		ID:               modelID,
		Provider:         r.vendor.Name,
		ContextWindow:    meta.ContextWindow, // zero means unknown; callers use defaults
		MaxOutputTokens:  0,                  // varies by model; let API decide
		InputPerMTokens:  meta.InputPerMTok,
		OutputPerMTokens: meta.OutputPerMTok,
		SupportsImages:   meta.SupportsImages,
		SupportsThinking: meta.SupportsThinking,
	}, nil
}
