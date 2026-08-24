package provider

import (
	"context"
	"fmt"

	"agent-vivy/internal/domain"
)

// Catalog resolves provider names to live Refs (D-018): the pre-baked
// bundles loaded at startup, plus the always-available deterministic
// mock (FR-3, selected via config.Runtime.Mock).
type Catalog struct {
	bundles map[string]Bundle
}

// NewCatalog indexes the given bundles by name. Later duplicates win;
// callers load one bundle per provider (A2 fixtures).
func NewCatalog(bundles ...Bundle) *Catalog {
	c := &Catalog{bundles: make(map[string]Bundle, len(bundles))}
	for _, b := range bundles {
		c.bundles[b.Name] = b
	}
	return c
}

// For resolves name to a Ref. "mock" always works; bundle-backed
// providers require their bundle to be loaded, and the Vivy-owned
// Anthropic Messages API adapter is not wired yet (later milestone).
func (c *Catalog) For(name string) (Ref, error) {
	if name == "mock" {
		return newMockRef(), nil
	}
	b, ok := c.bundles[name]
	if !ok {
		return nil, fmt.Errorf("provider %q: bundle not loaded", name)
	}
	switch b.Backend {
	case BackendEinoOpenAI:
		return newOpenAIRef(b), nil
	case BackendVivyAnthropic:
		return nil, fmt.Errorf("provider %q: backend %s not wired yet; the Vivy-owned Anthropic Messages API adapter lands in a later milestone", name, b.Backend)
	default:
		return nil, fmt.Errorf("provider %q: unknown backend %q", name, b.Backend)
	}
}

// ResolveModelInfo looks up capacity metadata for a specific provider and
// model combination. It delegates to the provider's Ref implementation,
// which may return zero ContextWindow when the information is unavailable.
// Callers should use conservative defaults in that case.
func (c *Catalog) ResolveModelInfo(ctx context.Context, providerName, modelID string) (domain.ModelInfo, error) {
	ref, err := c.For(providerName)
	if err != nil {
		return domain.ModelInfo{}, err
	}
	return ref.ModelInfo(ctx, modelID)
}
