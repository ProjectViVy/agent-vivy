package app

import (
	"fmt"
	"os"
	"slices"
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
//
// Endpoint is set when the selection is a user-registered provider, whose
// endpoint the embedded catalog does not describe: the registry entry carries
// the whole wire identity, and Provider is only its alias.
type ResolvedModel struct {
	Provider string
	Adapter  string
	Model    string
	BaseURL  string
	APIKey   string
	Frozen   bool
	Ready    bool
	Endpoint provider.Endpoint
}

// ModelResolver chooses the active ModelSpec: a frozen ENV session wins,
// otherwise ~/.vivy/settings.yaml. Empty means the process has no model
// yet (wizard required). It never writes environment variables.
type ModelResolver struct {
	mu          sync.Mutex
	cfg         config.Config
	path        string
	projectRoot string
	catalog     *provider.Catalog
	host        *modelhost.Host
	credentials *credentialmodule.Resolver
	frozen      *ResolvedModel
	// envAmbiguity carries why the process environment named credentials that
	// could not be attributed to one vendor, so the composition root can say so
	// once instead of the process silently running someone else's endpoint.
	envAmbiguity string
}

func newModelResolver(cfg config.Config, path string, catalog *provider.Catalog, host *modelhost.Host, supplied ...*credentialmodule.Resolver) *ModelResolver {
	return newModelResolverForProject(cfg, path, "", catalog, host, supplied...)
}

func newModelResolverForProject(cfg config.Config, path, projectRoot string, catalog *provider.Catalog, host *modelhost.Host, supplied ...*credentialmodule.Resolver) *ModelResolver {
	var credentials *credentialmodule.Resolver
	if len(supplied) > 0 {
		credentials = supplied[0]
	} else {
		// The allowlist is the embedded vendors' credential names, so a
		// third-party vendor's environment variable works with no per-vendor
		// configuration block (PROV-P3).
		credentials, _ = credentialmodule.Compose(map[string][]string{"vivy/model": modelCredentialAllowlist(catalog)})
	}
	r := &ModelResolver{cfg: cfg, path: path, projectRoot: projectRoot, catalog: catalog, host: host, credentials: credentials}
	if frozen, ambiguity := freezeFromEnv(cfg, catalog, credentials); frozen.Provider != "" {
		r.frozen = &frozen
	} else {
		r.envAmbiguity = ambiguity
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
// candidate, so a third-party vendor freezes the session exactly like the
// first-party ones; the configured vendor (config.providers.active) wins when
// several are set, and VIVY_PROVIDER overrides it by vendor name or by adapter.
//
// The environment must identify exactly one candidate. One credential name can
// be the credential of more than one embedded vendor (openai and aihubmix both
// read OPENAI_API_KEY), so several candidates with no explicit selection are
// ambiguous: the empty session is returned and the caller keeps the settings
// document. The second result explains that refusal, and is empty whenever the
// environment simply named no credentials.
func freezeFromEnv(cfg config.Config, catalog *provider.Catalog, credentials *credentialmodule.Resolver) (ResolvedModel, string) {
	if catalog == nil || credentials == nil {
		return ResolvedModel{}, ""
	}
	var hits []provider.Vendor
	for _, vendor := range catalog.Vendors() {
		if credentials.IsSet("vivy/model", vendor.EnvKey) {
			hits = append(hits, vendor)
		}
	}
	if len(hits) == 0 {
		return ResolvedModel{}, ""
	}
	chosen, decided := selectFrozenVendor(hits, cfg.Providers.Active, strings.TrimSpace(os.Getenv(envProviderOverride)))
	if !decided {
		return ResolvedModel{}, ambiguousEnvReason(hits)
	}
	key, err := credentials.Resolve("vivy/model", chosen.EnvKey)
	if err != nil {
		return ResolvedModel{}, ""
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
	}, ""
}

// selectFrozenVendor decides which candidate an environment session belongs to,
// and never guesses. An explicit selection wins — VIVY_PROVIDER by vendor name
// or by a sealed adapter exactly one candidate speaks, then the configured
// vendor by name — and otherwise the environment must name exactly one
// candidate from the embedded data.
func selectFrozenVendor(hits []provider.Vendor, active, override string) (provider.Vendor, bool) {
	if override != "" {
		if vendor, ok := matchFrozenVendor(hits, override); ok {
			return vendor, true
		}
	}
	if active != "" {
		for _, vendor := range hits {
			if vendor.Name == active {
				return vendor, true
			}
		}
	}
	if len(hits) == 1 {
		return hits[0], true
	}
	return provider.Vendor{}, false
}

// ambiguousEnvReason names the variables and vendors the environment offered,
// never a value. It is a diagnostic for the composition root's log.
func ambiguousEnvReason(hits []provider.Vendor) string {
	keys := make([]string, 0, len(hits))
	names := make([]string, 0, len(hits))
	for _, vendor := range hits {
		keys = append(keys, vendor.EnvKey)
		names = append(names, vendor.Name)
	}
	keys = slices.Compact(slices.Sorted(slices.Values(keys)))
	return fmt.Sprintf("environment credentials %s are set for several providers (%s); set config.providers.active or VIVY_PROVIDER to choose one, so the settings document stays in effect",
		strings.Join(keys, ", "), strings.Join(names, ", "))
}

// matchFrozenVendor resolves a VIVY_PROVIDER override: a vendor name names
// that vendor, and a sealed adapter names the candidate that speaks it, which
// must be exactly one — several candidates sharing an adapter is the same
// ambiguity as several sharing a credential.
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
	var matched provider.Vendor
	found := false
	for _, vendor := range hits {
		if _, ok := vendor.EndpointForAdapter(adapter); !ok {
			continue
		}
		if found {
			return provider.Vendor{}, false
		}
		matched, found = vendor, true
	}
	return matched, found
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
	// A project pin restores the last model the operator picked inside this
	// project over the global selection (VCP F3). Frozen ENV sessions and
	// processes without a project root ignore it.
	if r.projectRoot != "" {
		if pin, ok := s.ProjectDefault(r.projectRoot); ok {
			s.Provider, s.DefaultModel, s.BaseURL = pin.Provider, pin.Model, pin.BaseURL
		}
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
		Endpoint: selection.endpoint,
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

// FrozenAmbiguity reports why set environment credentials did not freeze this
// process, or "" when there was nothing to attribute. It names variables and
// vendors only, so it is safe to log.
func (r *ModelResolver) FrozenAmbiguity() string {
	if r == nil {
		return ""
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.envAmbiguity
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
		Endpoint: cur.Endpoint,
	}
}
