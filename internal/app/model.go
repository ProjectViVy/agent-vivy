package app

import (
	"os"
	"strings"
	"sync"

	"agent-vivy/internal/app/settings"
	"agent-vivy/internal/config"
	"agent-vivy/internal/provider"
)

const (
	envModelOverride    = "VIVY_MODEL"
	envProviderOverride = "VIVY_PROVIDER"
)

// ResolvedModel is the live provider selection for one process. Frozen is
// true when a process environment variable named a bundle env_key, which
// makes the selection read-only for this process.
type ResolvedModel struct {
	Provider string
	Model    string
	BaseURL  string
	APIKey   string
	Frozen   bool
	Ready    bool
}

// ModelResolver chooses the active ModelSpec: a frozen ENV session wins,
// otherwise ~/.vivy/settings.yaml. Empty means the process has no model
// yet (wizard required). It never writes environment variables.
type ModelResolver struct {
	mu      sync.Mutex
	cfg     config.Config
	path    string
	catalog *provider.Catalog
	frozen  *ResolvedModel
}

func newModelResolver(cfg config.Config, path string, catalog *provider.Catalog) *ModelResolver {
	r := &ModelResolver{cfg: cfg, path: path, catalog: catalog}
	if !cfg.Runtime.Mock {
		if frozen, ok := freezeFromEnv(cfg, catalog); ok {
			r.frozen = &frozen
		}
	}
	return r
}

func freezeFromEnv(cfg config.Config, catalog *provider.Catalog) (ResolvedModel, bool) {
	type candidate struct {
		name   string
		envKey string
		model  string
	}
	var hits []candidate
	for _, c := range []candidate{
		{name: settings.ProviderOpenAI, envKey: cfg.Providers.OpenAI.EnvKey, model: cfg.Providers.OpenAI.DefaultModel},
		{name: settings.ProviderAnthropic, envKey: cfg.Providers.Anthropic.EnvKey, model: cfg.Providers.Anthropic.DefaultModel},
	} {
		if c.envKey == "" {
			continue
		}
		if strings.TrimSpace(os.Getenv(c.envKey)) != "" {
			hits = append(hits, c)
		}
	}
	if len(hits) == 0 {
		return ResolvedModel{}, false
	}
	chosen := hits[0]
	if override := strings.TrimSpace(os.Getenv(envProviderOverride)); override != "" {
		for _, c := range hits {
			if c.name == override {
				chosen = c
				break
			}
		}
	}
	key := strings.TrimSpace(os.Getenv(chosen.envKey))
	base := strings.TrimSpace(os.Getenv(provider.APIBaseEnvVar))
	modelID := strings.TrimSpace(os.Getenv(envModelOverride))
	if modelID == "" {
		modelID = chosen.model
	}
	if catalog != nil {
		if b, ok := catalog.Bundle(chosen.name); ok && modelID == "" {
			modelID = b.DefaultModel
		}
	}
	return ResolvedModel{
		Provider: chosen.name,
		Model:    modelID,
		BaseURL:  base,
		APIKey:   key,
		Frozen:   true,
		Ready:    key != "",
	}, true
}

// Current returns the live selection. Frozen ENV sessions ignore the
// settings document. Missing documents yield a not-ready zero value.
func (r *ModelResolver) Current() ResolvedModel {
	if r == nil {
		return ResolvedModel{}
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.currentLocked()
}

func (r *ModelResolver) currentLocked() ResolvedModel {
	if r.cfg.Runtime.Mock {
		modelID := "mock"
		if r.cfg.Runtime.MockScenario != "" {
			modelID = "mock:" + r.cfg.Runtime.MockScenario
		}
		return ResolvedModel{
			Provider: settings.ProviderMock,
			Model:    modelID,
			Ready:    true,
		}
	}
	if r.frozen != nil {
		return *r.frozen
	}
	if r.path == "" {
		return ResolvedModel{}
	}
	s, err := settings.Load(r.path)
	if err != nil || s.IsZero() {
		return ResolvedModel{}
	}
	providerName := s.Provider
	if providerName == "" || providerName == settings.ProviderMock {
		return ResolvedModel{}
	}
	modelID := s.DefaultModel
	if modelID == "" {
		switch providerName {
		case settings.ProviderAnthropic:
			modelID = r.cfg.Providers.Anthropic.DefaultModel
		default:
			modelID = r.cfg.Providers.OpenAI.DefaultModel
		}
	}
	key := settings.ActiveKey(s, providerName, s.BaseURL)
	return ResolvedModel{
		Provider: providerName,
		Model:    modelID,
		BaseURL:  s.BaseURL,
		APIKey:   key,
		Frozen:   false,
		Ready:    key != "",
	}
}

// Frozen reports whether this process is locked to an ENV session.
func (r *ModelResolver) Frozen() bool {
	if r == nil {
		return false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.frozen != nil
}

// Invalidate is retained for callers that previously cached a model. The
// resolving ChatModel now re-reads Live() on each call.
func (r *ModelResolver) Invalidate() {}

// Live implements provider.SpecSource so the resolving ChatModel can stay
// inside internal/provider (D-007).
func (r *ModelResolver) Live() provider.LiveSpec {
	cur := r.Current()
	return provider.LiveSpec{
		Provider: cur.Provider,
		Model:    cur.Model,
		BaseURL:  cur.BaseURL,
		APIKey:   cur.APIKey,
		Ready:    cur.Ready,
	}
}
