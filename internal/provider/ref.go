package provider

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/components/model"

	"agent-vivy/internal/domain"
)

// Ref is the ProviderRef boundary (A1 finalized seam): the rest of Vivy
// asks a provider for a chat model by id and never touches credentials
// or vendor SDKs directly. Implementations read the API key from the
// environment variable named by the bundle's env_key at Model() call
// time; the key itself is never persisted (D-010).
type Ref interface {
	// Name returns the provider/bundle name this ref serves.
	Name() string

	// Model builds a tool-calling chat model for modelID. An empty
	// modelID falls back to the bundle's default_model.
	Model(ctx context.Context, modelID string) (model.ToolCallingChatModel, error)

	// ModelInfo returns capacity metadata for the given modelID. If
	// modelID is empty, it uses the bundle's default_model. Returns
	// domain.ModelInfo with zero ContextWindow when the information is
	// unavailable; callers should use conservative defaults in that case.
	ModelInfo(ctx context.Context, modelID string) (domain.ModelInfo, error)
}

// KeyMissingError reports that the environment variable holding the API
// key is unset or empty. It is structured so callers can surface an
// actionable message instead of a raw SDK failure (FR-11).
type KeyMissingError struct {
	Provider string
	EnvKey   string
}

func (e *KeyMissingError) Error() string {
	return fmt.Sprintf("provider %s: API key missing: set the %s environment variable", e.Provider, e.EnvKey)
}
