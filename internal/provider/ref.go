package provider

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/components/model"

	"agent-vivy/internal/domain"
)

// ModelSpec is the per-call construction input for a chat model. Credentials
// and the gateway URL travel with the spec; implementations must not read
// process environment variables to find keys (D-010). An empty ID falls
// back to the bundle default_model. An empty BaseURL falls back to the
// bundle default_api_base.
type ModelSpec struct {
	ID      string
	APIKey  string
	BaseURL string
}

// Ref is the ProviderRef boundary (A1 finalized seam): the rest of Vivy
// asks a provider for a chat model by spec and never touches credentials
// or vendor SDKs directly. The key itself is never persisted by this
// package (D-010).
type Ref interface {
	// Name returns the provider/bundle name this ref serves.
	Name() string

	// Model builds a tool-calling chat model for spec. An empty ID falls
	// back to the bundle's default_model.
	Model(ctx context.Context, spec ModelSpec) (model.ToolCallingChatModel, error)

	// ModelInfo returns capacity metadata for the given modelID. If
	// modelID is empty, it uses the bundle's default_model. Returns
	// domain.ModelInfo with zero ContextWindow when the information is
	// unavailable; callers should use conservative defaults in that case.
	ModelInfo(ctx context.Context, modelID string) (domain.ModelInfo, error)
}

// KeyMissingError reports that no API key was supplied for this provider.
// It is structured so callers can surface an actionable message instead of
// a raw SDK failure (FR-11). EnvKey is optional context when a frozen ENV
// session named a variable; the message never carries a key value.
type KeyMissingError struct {
	Provider string
	EnvKey   string
}

func (e *KeyMissingError) Error() string {
	if e.EnvKey != "" {
		return fmt.Sprintf("provider %s: API key missing: set the %s environment variable or configure it in Settings → Model", e.Provider, e.EnvKey)
	}
	return fmt.Sprintf("provider %s: API key missing: configure it in the welcome wizard or Settings → Model", e.Provider)
}
