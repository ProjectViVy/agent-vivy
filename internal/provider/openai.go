package provider

import (
	"context"
	"fmt"
	"os"

	einoopenai "github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/model"
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
		BaseURL: r.bundle.DefaultAPIBase,
		Model:   modelID,
	})
	if err != nil {
		return nil, fmt.Errorf("provider %s: construct openai model %q: %w", r.bundle.Name, modelID, err)
	}
	return cm, nil
}
