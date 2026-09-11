package provider

import (
	"context"
	"errors"
	"fmt"

	"agent-vivy/internal/domain"
	"agent-vivy/sdk/port/providerprofile"
)

var ErrAdapterFamilyMismatch = errors.New("provider Profile adapter family does not match the pinned bundle adapter")

// Catalog resolves provider names to live Refs (D-018): the pre-baked
// bundles loaded at startup.
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

// Bundle returns the loaded bundle for name, if any.
func (c *Catalog) Bundle(name string) (Bundle, bool) {
	if c == nil {
		return Bundle{}, false
	}
	b, ok := c.bundles[name]
	return b, ok
}

// For resolves name to a Ref. Bundle-backed providers require their bundle
// to be loaded.
func (c *Catalog) For(name string) (Ref, error) {
	b, ok := c.bundles[name]
	if !ok {
		return nil, fmt.Errorf("provider %q: bundle not loaded", name)
	}
	switch b.Backend {
	case BackendEinoOpenAI:
		return newOpenAIRef(b), nil
	case BackendEinoClaude:
		return newClaudeRef(b), nil
	default:
		return nil, fmt.Errorf("provider %q: unknown backend %q", name, b.Backend)
	}
}

// ForProfile resolves one ModelHost-approved declarative Profile to its
// build-owned executable adapter. The bundle supplies defaults and provider
// metadata; the Profile selects the adapter family. A mismatch fails closed.
func (c *Catalog) ForProfile(profile providerprofile.Profile) (Ref, error) {
	bundle, ok := c.Bundle(profile.ID)
	if !ok {
		return nil, fmt.Errorf("provider %q: bundle not loaded", profile.ID)
	}
	wantFamily := ProfileFromBundle(bundle).AdapterFamily
	if profile.AdapterFamily != wantFamily {
		return nil, fmt.Errorf("%w: Profile %q declares %q, bundle requires %q", ErrAdapterFamilyMismatch, profile.ID, profile.AdapterFamily, wantFamily)
	}
	switch profile.AdapterFamily {
	case AdapterFamilyOpenAICompatible:
		return newOpenAIRef(bundle), nil
	case AdapterFamilyAnthropic:
		return newClaudeRef(bundle), nil
	default:
		return nil, fmt.Errorf("provider %q: unsupported adapter family %q", profile.ID, profile.AdapterFamily)
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
