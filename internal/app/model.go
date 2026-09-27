package app

import (
	"os"
	"strings"
	"sync"

	"agent-vivy/internal/app/settings"
	"agent-vivy/internal/config"
	"agent-vivy/internal/modelhost"
	credentialmodule "agent-vivy/internal/modules/credential"
	"agent-vivy/internal/provider"
)

const (
	envModelOverride    = "VIVY_MODEL"
	envProviderOverride = "VIVY_PROVIDER"
)

// ResolvedModel is the live provider selection for one process. Frozen is
// true when a process environment variable named a vendor env_key, which
// makes the selection read-only for this process.
//
// Provider is the vendor the selection resolves to (the credential owner and
// the default address); Adapter is the sealed protocol it speaks, which is what
// the compiled Generation and the availability surface key on.
type ResolvedModel struct {
	Provider string
	Adapter  string
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
	mu          sync.Mutex
	cfg         config.Config
	path        string
	catalog     *provider.Catalog
	host        *modelhost.Host
	credentials *credentialmodule.Resolver
	frozen      *ResolvedModel
}

func newModelResolver(cfg config.Config, path string, catalog *provider.Catalog, host *modelhost.Host, supplied ...*credentialmodule.Resolver) *ModelResolver {
	var credentials *credentialmodule.Resolver
	if len(supplied) > 0 {
		credentials = supplied[0]
	} else {
		// The allowlist is the embedded vendors' credential names, so a
		// third-party vendor's environment variable works with no per-vendor
		// configuration block (PROV-P3).
		credentials, _ = credentialmodule.Compose(map[string][]string{"vivy/model": modelCredentialAllowlist(catalog)})
	}
	r := &ModelResolver{cfg: cfg, path: path, catalog: catalog, host: host, credentials: credentials}
	if frozen, ok := freezeFromEnv(cfg, catalog, credentials); ok {
		r.frozen = &frozen
	}
	return r
}

// modelCredentialAllowlist is the environment names the model module may read,
// derived from the embedded vendor data.
func modelCredentialAllowlist(catalog *provider.Catalog) []string {
	if catalog == nil {
		return nil
	}
	return provider.VendorEnvKeys(catalog.Vendors())
}

// freezeFromEnv builds the read-only ENV session. Every embedded vendor is a
// candidate now, so a third-party vendor freezes the session exactly like the
// first-party ones; the configured vendor (config.providers.active) wins when
// several are set, and VIVY_PROVIDER overrides it by vendor name or by adapter.
func freezeFromEnv(cfg config.Config, catalog *provider.Catalog, credentials *credentialmodule.Resolver) (ResolvedModel, bool) {
	if catalog == nil || credentials == nil {
		return ResolvedModel{}, false
	}
	var hits []provider.Vendor
	for _, vendor := range catalog.Vendors() {
		if credentials.IsSet("vivy/model", vendor.EnvKey) {
			hits = append(hits, vendor)
		}
	}
	if len(hits) == 0 {
		return ResolvedModel{}, false
	}
	chosen := hits[0]
	if active := cfg.Providers.Active; active != "" {
		for _, vendor := range hits {
			if vendor.Name == active {
				chosen = vendor
				break
			}
		}
	}
	if override := strings.TrimSpace(os.Getenv(envProviderOverride)); override != "" {
		if vendor, ok := matchFrozenVendor(hits, override); ok {
			chosen = vendor
		}
	}
	key, err := credentials.Resolve("vivy/model", chosen.EnvKey)
	if err != nil {
		return ResolvedModel{}, false
	}
	key = strings.TrimSpace(key)
	base := strings.TrimSpace(os.Getenv(provider.APIBaseEnvVar))
	modelID := strings.TrimSpace(os.Getenv(envModelOverride))
	selection, _ := resolveVendorSelection(catalog, chosen.Name, "", base, modelID)
	return ResolvedModel{
		Provider: chosen.Name,
		Adapter:  selection.Adapter,
		Model:    selection.Model,
		BaseURL:  base,
		APIKey:   key,
		Frozen:   true,
		Ready:    key != "",
	}, true
}

// matchFrozenVendor resolves a VIVY_PROVIDER override: a vendor name names
// that vendor, and a sealed adapter names the first candidate that speaks it.
func matchFrozenVendor(hits []provider.Vendor, override string) (provider.Vendor, bool) {
	for _, vendor := range hits {
		if vendor.Name == override {
			return vendor, true
		}
	}
	adapter := settings.NormalizeAdapter(override)
	if adapter == "" {
		return provider.Vendor{}, false
	}
	for _, vendor := range hits {
		if _, ok := vendor.EndpointForAdapter(adapter); ok {
			return vendor, true
		}
	}
	return provider.Vendor{}, false
}

// Current returns the live selection. Frozen ENV sessions ignore the
// settings document. Missing documents yield a not-ready zero value.
func (r *ModelResolver) Current() ResolvedModel {
	if r == nil {
		return ResolvedModel{}
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	current := r.currentLocked()
	if current.Provider != "" && r.host != nil {
		// The compiled Generation seals adapters, so readiness is decided by
		// the adapter the selection speaks.
		if _, err := r.host.ResolveExecutable(current.Adapter); err != nil {
			current.Ready = false
		}
	}
	return current
}

func (r *ModelResolver) currentLocked() ResolvedModel {
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
	if s.Provider == "" {
		return ResolvedModel{}
	}
	// The stored vocabulary is the adapter; the vendor, endpoint variant and
	// default model come from the embedded data (PROV-P3).
	selection, _ := resolveStoredSelection(r.catalog, r.cfg, s)
	key := resolveSettingsAPIKey(s, selection)
	return ResolvedModel{
		Provider: selection.Vendor,
		Adapter:  selection.Adapter,
		Model:    selection.Model,
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
		Adapter:  cur.Adapter,
		Model:    cur.Model,
		BaseURL:  cur.BaseURL,
		APIKey:   cur.APIKey,
		Ready:    cur.Ready,
	}
}
