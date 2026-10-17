package app

import (
	"os"
	"strings"
	"testing"

	"agent-vivy/internal/app/settings"
	"agent-vivy/internal/config"
	"agent-vivy/internal/modelhost"
	"agent-vivy/internal/provider"
	"agent-vivy/sdk/port/providerprofile"
)

func TestMain(m *testing.M) {
	os.Unsetenv("DEEPSEEK_API_KEY")
	os.Unsetenv("OPENAI_API_KEY")
	os.Unsetenv("ANTHROPIC_API_KEY")
	os.Exit(m.Run())
}

// testVendors indexes the embedded catalog; the app tests exercise the three
// compiled first-party vendors.
func testVendors(t *testing.T) map[string]provider.Vendor {
	t.Helper()
	vendors, err := provider.LoadEmbedded()
	if err != nil {
		t.Fatal(err)
	}
	index := make(map[string]provider.Vendor, len(vendors))
	for _, vendor := range vendors {
		index[vendor.Name] = vendor
	}
	for _, name := range []string{"deepseek", "openai", "anthropic"} {
		if _, ok := index[name]; !ok {
			t.Fatalf("embedded provider data lacks %q", name)
		}
	}
	return index
}

// testProfiles projects the three compiled first-party vendors onto the adapter
// Profiles the Generation seals: one Profile per sealed family, keyed by
// adapter.
func testProfiles(t *testing.T) []providerprofile.Profile {
	t.Helper()
	profiles, err := provider.AdapterProfiles()
	if err != nil {
		t.Fatal(err)
	}
	if len(profiles) != 3 {
		t.Fatalf("AdapterProfiles() = %d entries, want 3", len(profiles))
	}
	return profiles
}

func testCatalog(t *testing.T) *provider.Catalog {
	t.Helper()
	vendors, err := provider.LoadEmbedded()
	if err != nil {
		t.Fatal(err)
	}
	return provider.NewCatalog(vendors...)
}

func testModelHost(t *testing.T) *modelhost.Host {
	t.Helper()
	host, err := modelhost.New(testProfiles(t), provider.Capabilities())
	if err != nil {
		t.Fatal(err)
	}
	return host
}

// TestResolverLegacyDocumentKeepsItsVendorAndEndpoint: a pre-migration
// `settings.yaml` names a bundle, which was a vendor. Normalization turns the
// value into an adapter, and the vendor it named plus the configured default
// endpoint keep the effective selection exactly what it was.
func TestResolverLegacyDocumentKeepsItsVendorAndEndpoint(t *testing.T) {
	dir := t.TempDir()
	path := settings.Path(dir)
	if _, err := settings.Save(path, settings.Settings{
		Provider: settings.ProviderDeepSeek, ApiKey: "sk-legacy",
	}); err != nil {
		t.Fatal(err)
	}
	cur := newModelResolver(config.Default(), path, testCatalog(t), testModelHost(t)).Current()
	if !cur.Ready || cur.Frozen {
		t.Fatalf("legacy selection must be ready: %+v", cur)
	}
	if cur.Provider != "deepseek" || cur.Adapter != provider.AdapterOpenAICompletions {
		t.Fatalf("legacy selection = vendor %q adapter %q, want deepseek/openai-completions", cur.Provider, cur.Adapter)
	}
	// The model and address come from the embedded endpoint, not from config.
	if cur.Model != "deepseek-flash" || cur.BaseURL != "" {
		t.Fatalf("legacy selection = model %q base_url %q, want the endpoint's default model and no stored address", cur.Model, cur.BaseURL)
	}
}

// TestResolverThirdPartyVendorNeedsNoConfigurationBlock: the credential
// allowlist and the vendor identity are both data-derived, so a vendor that
// config.yaml never named works from its own environment variable.
func TestResolverThirdPartyVendorNeedsNoConfigurationBlock(t *testing.T) {
	t.Setenv("DEEPSEEK_API_KEY", "")
	os.Unsetenv("DEEPSEEK_API_KEY")
	os.Unsetenv("MINIMAX_API_KEY")
	dir := t.TempDir()
	path := settings.Path(dir)
	// A new-style selection: the adapter plus the address the embedded data
	// declares for MiniMax. No vendor name appears anywhere.
	if _, err := settings.Save(path, settings.Settings{
		Provider: provider.AdapterOpenAICompletions, BaseURL: "https://api.minimaxi.com/v1",
	}); err != nil {
		t.Fatal(err)
	}
	catalog := testCatalog(t)
	host := testModelHost(t)

	// Without the environment variable the selection is not ready, and it is
	// the *third-party* vendor's variable that the error names.
	cur := newModelResolver(config.Default(), path, catalog, host).Current()
	if cur.Provider != "minimax" || cur.Adapter != provider.AdapterOpenAICompletions || cur.Model != "MiniMax-M2.1" {
		t.Fatalf("third-party selection = %+v, want the minimax endpoint", cur)
	}
	if cur.Ready {
		t.Fatal("a selection whose key is unset must not be ready")
	}

	t.Setenv("MINIMAX_API_KEY", "sk-minimax")
	frozen := newModelResolver(config.Default(), path, catalog, host).Current()
	if !frozen.Ready || !frozen.Frozen {
		t.Fatalf("a third-party environment variable must freeze the session: %+v", frozen)
	}
	if frozen.Provider != "minimax" || frozen.APIKey != "sk-minimax" || frozen.Adapter != provider.AdapterOpenAICompletions {
		t.Fatalf("frozen third-party session = %+v", frozen)
	}
}

// TestResolverFrozenEnvHonoursVIVYProviderForThirdParty: VIVY_PROVIDER names
// the vendor whose environment variable wins when several are set.
func TestResolverFrozenEnvHonoursVIVYProviderForThirdParty(t *testing.T) {
	t.Setenv("DEEPSEEK_API_KEY", "sk-deepseek")
	t.Setenv("MINIMAX_API_KEY", "sk-minimax")
	t.Setenv("VIVY_PROVIDER", "minimax")
	dir := t.TempDir()
	cur := newModelResolver(config.Default(), settings.Path(dir), testCatalog(t), testModelHost(t)).Current()
	if !cur.Frozen || cur.Provider != "minimax" || cur.APIKey != "sk-minimax" {
		t.Fatalf("VIVY_PROVIDER=minimax resolved %+v", cur)
	}
	if cur.Adapter != provider.AdapterOpenAICompletions || cur.Model != "MiniMax-M2.1" {
		t.Fatalf("third-party frozen selection = adapter %q model %q", cur.Adapter, cur.Model)
	}
}

// TestResolverAdapterSelectsTheEndpointVariant: one vendor, two protocols. The
// stored adapter picks the variant and with it the address and default model.
func TestResolverAdapterSelectsTheEndpointVariant(t *testing.T) {
	dir := t.TempDir()
	path := settings.Path(dir)
	if _, err := settings.Save(path, settings.Settings{
		Provider: provider.AdapterAnthropicMessages, ApiKey: "sk-variant",
	}); err != nil {
		t.Fatal(err)
	}
	catalog := testCatalog(t)
	vendor, ok := catalog.Vendor("deepseek")
	if !ok {
		t.Fatal("embedded data must declare deepseek")
	}
	endpoint, ok := vendor.EndpointForAdapter(provider.AdapterAnthropicMessages)
	if !ok {
		t.Skip("the embedded DeepSeek vendor declares no anthropic-messages endpoint")
	}
	cur := newModelResolver(config.Default(), path, catalog, testModelHost(t)).Current()
	if cur.Adapter != provider.AdapterAnthropicMessages || cur.Model != endpoint.DefaultModel {
		t.Fatalf("variant selection = adapter %q model %q, want %q/%q", cur.Adapter, cur.Model, provider.AdapterAnthropicMessages, endpoint.DefaultModel)
	}
}

// TestConfigDefaultSelectionIsDataDerived: the pair the control plane reports
// as the configuration default comes from the configured vendor's endpoint.
func TestConfigDefaultSelectionIsDataDerived(t *testing.T) {
	cfg := config.Default()
	selection := configDefaultSelection(testCatalog(t), cfg)
	if selection.Vendor != "deepseek" || selection.Adapter != provider.AdapterOpenAICompletions || selection.Model != "deepseek-flash" {
		t.Fatalf("config default = %+v", selection)
	}
	if selection.endpoint.BaseURL == "" {
		t.Fatal("the config default must carry the declared endpoint")
	}
}

// TestResolverIsNonFatalForAnUnusableStoredValue: a document the write path
// would reject must not crash the live resolver; it reports a not-ready
// selection and the user can fix it in Settings (MIGRATION.md §3 rule 3).
func TestResolverIsNonFatalForAnUnusableStoredValue(t *testing.T) {
	dir := t.TempDir()
	path := settings.Path(dir)
	if err := os.WriteFile(path, []byte("provider: banana\ndefault_model: m\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cur := newModelResolver(config.Default(), path, testCatalog(t), testModelHost(t)).Current()
	if cur.Ready {
		t.Fatalf("an unusable stored value must not be ready: %+v", cur)
	}
	if cur.Provider != "" || cur.Adapter != "" {
		t.Fatalf("an unusable stored value must yield the zero selection: %+v", cur)
	}
}

// TestResolverProjectPinRestoresPerProjectSelection: a model picked inside a
// project is pinned under that project's root and restores over the global
// selection on the next launch in the same project (VCP F3).
func TestResolverProjectPinRestoresPerProjectSelection(t *testing.T) {
	dir := t.TempDir()
	path := settings.Path(dir)
	project := "/work/alpha"
	if _, err := settings.Save(path, settings.Settings{
		Provider: settings.ProviderDeepSeek, ApiKey: "sk-global",
		ProjectDefaults: map[string]settings.ScopedModel{
			project: {Provider: "openai", Model: "gpt-4o"},
		},
	}); err != nil {
		t.Fatal(err)
	}
	pinned := newModelResolverForProject(config.Default(), path, project, testCatalog(t), testModelHost(t)).Current()
	if pinned.Provider != "openai" || pinned.Model != "gpt-4o" {
		t.Fatalf("pinned selection = %s/%s, want openai/gpt-4o", pinned.Provider, pinned.Model)
	}
	// A fresh resolver (a relaunch) restores the same pin.
	again := newModelResolverForProject(config.Default(), path, project, testCatalog(t), testModelHost(t)).Current()
	if again.Provider != "openai" || again.Model != "gpt-4o" {
		t.Fatalf("relaunch selection = %s/%s, want openai/gpt-4o", again.Provider, again.Model)
	}
	// A different project keeps the global selection.
	other := newModelResolverForProject(config.Default(), path, "/work/beta", testCatalog(t), testModelHost(t)).Current()
	if other.Provider != "deepseek" {
		t.Fatalf("unpinned project selection = %s, want deepseek", other.Provider)
	}
}

func TestResolverEmptyWithoutSettings(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Default()
	cfg.Storage.DataDir = dir
	r := newModelResolver(cfg, settings.Path(dir), testCatalog(t), testModelHost(t))
	cur := r.Current()
	if cur.Ready || cur.Frozen || cur.Provider != "" {
		t.Fatalf("empty workspace must not be ready: %+v", cur)
	}
}

func TestResolverReadsSettingsRegistry(t *testing.T) {
	dir := t.TempDir()
	path := settings.Path(dir)
	if _, err := settings.Save(path, settings.Settings{
		Provider:     settings.ProviderOpenAI,
		DefaultModel: "deepseek-chat",
		BaseURL:      "https://gateway.example.com/v1",
		Providers: []settings.ProviderEntry{{
			ID: "custom-1", DisplayName: "Gateway", Bundle: settings.ProviderOpenAI,
			BaseURL: "https://gateway.example.com/v1", ApiKey: "sk-registry",
		}},
	}); err != nil {
		t.Fatal(err)
	}
	r := newModelResolver(config.Default(), path, testCatalog(t), testModelHost(t))
	cur := r.Current()
	if !cur.Ready || cur.Frozen {
		t.Fatalf("registry selection must be ready: %+v", cur)
	}
	if cur.APIKey != "sk-registry" || cur.BaseURL != "https://gateway.example.com/v1" || cur.Model != "deepseek-chat" {
		t.Fatalf("resolved = %+v", cur)
	}
}

func TestResolverFrozenEnvOverridesSettings(t *testing.T) {
	dir := t.TempDir()
	path := settings.Path(dir)
	if _, err := settings.Save(path, settings.Settings{
		Provider:     settings.ProviderOpenAI,
		DefaultModel: "settings-model",
		BaseURL:      "https://settings.example.com/v1",
		ApiKey:       "sk-settings",
	}); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DEEPSEEK_API_KEY", "sk-env")
	t.Setenv("VIVY_API_BASE", "https://env.example.com/v1")
	t.Setenv("VIVY_MODEL", "env-model")
	r := newModelResolver(config.Default(), path, testCatalog(t), testModelHost(t))
	cur := r.Current()
	if !cur.Frozen || !cur.Ready {
		t.Fatalf("env session must freeze: %+v", cur)
	}
	if cur.APIKey != "sk-env" || cur.BaseURL != "https://env.example.com/v1" || cur.Model != "env-model" {
		t.Fatalf("frozen = %+v", cur)
	}
}

func TestResolverInvalidateDropsCache(t *testing.T) {
	dir := t.TempDir()
	path := settings.Path(dir)
	r := newModelResolver(config.Default(), path, testCatalog(t), testModelHost(t))
	if r.Current().Ready {
		t.Fatal("expected empty")
	}
	if _, err := settings.Save(path, settings.Settings{
		Provider: settings.ProviderDeepSeek, DefaultModel: "deepseek-flash", ApiKey: "sk-later",
	}); err != nil {
		t.Fatal(err)
	}
	r.Invalidate()
	cur := r.Current()
	if !cur.Ready || cur.APIKey != "sk-later" {
		t.Fatalf("after save = %+v", cur)
	}
}

func TestResolverCannotReadyUncompiledProfile(t *testing.T) {
	dir := t.TempDir()
	path := settings.Path(dir)
	if _, err := settings.Save(path, settings.Settings{
		Provider: settings.ProviderAnthropic, DefaultModel: "claude-sonnet-4-5", ApiKey: "secret",
	}); err != nil {
		t.Fatal(err)
	}
	// The vendor and its endpoint exist in the embedded data, but this
	// Generation compiles only the openai-completions adapter, so the selection
	// cannot become ready.
	if _, ok := testVendors(t)["anthropic"]; !ok {
		t.Fatal("embedded data must still declare the anthropic vendor")
	}
	host, err := modelhost.New([]providerprofile.Profile{testProfiles(t)[0]}, provider.Capabilities())
	if err != nil {
		t.Fatal(err)
	}
	resolved := newModelResolver(config.Default(), path, testCatalog(t), host).Current()
	if resolved.Ready {
		t.Fatalf("uncompiled Profile became ready: %+v", resolved)
	}
}

func TestResolverProjectsReadyProfileIdentityWithoutConfiguration(t *testing.T) {
	dir := t.TempDir()
	path := settings.Path(dir)
	if _, err := settings.Save(path, settings.Settings{
		Provider: settings.ProviderDeepSeek, DefaultModel: "deepseek-flash", ApiKey: "secret",
	}); err != nil {
		t.Fatal(err)
	}
	host := testModelHost(t)
	catalog := testCatalog(t)
	resolver := newModelResolver(config.Default(), path, catalog, host)
	current := resolver.Current()
	// The ModelHost is keyed by adapter. DeepSeek's default endpoint speaks
	// openai-completions, which sorts second in the compiled Profile set. The
	// projection from the stored selection to that adapter is the same one the RPC
	// status closure uses.
	statuses := host.Statuses(current.Adapter, current.Ready)
	if len(statuses) != 3 || statuses[1].ID != provider.AdapterOpenAICompletions || statuses[1].State != modelhost.ProfileReady {
		t.Fatalf("resolver Profile statuses = %#v", statuses)
	}
	if statuses[1].AdapterFamily != provider.AdapterOpenAICompletions || statuses[1].EndpointClass == "" {
		t.Fatalf("resolver Profile provenance = %#v", statuses[1])
	}
	if current.Adapter != provider.AdapterOpenAICompletions {
		t.Fatalf("resolved adapter = %q, want openai-completions", current.Adapter)
	}
}

// clearModelEnv makes an environment-session test hermetic: every embedded
// vendor's credential variable is a candidate, so an ambient one would decide
// the session before the test sets anything.
func clearModelEnv(t *testing.T) {
	t.Helper()
	keys := append(provider.VendorEnvKeys(testCatalog(t).Vendors()),
		envProviderOverride, envModelOverride, provider.APIBaseEnvVar)
	for _, key := range keys {
		original, ok := os.LookupEnv(key)
		if !ok {
			continue
		}
		if err := os.Unsetenv(key); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Setenv(key, original) })
	}
}

// TestResolverSharedEnvKeyDoesNotGuessAVendor: one credential name can be the
// credential of several embedded vendors (openai and aihubmix both read
// OPENAI_API_KEY). The environment then names no provider at all, so the
// settings document stays in effect and the composition root can say why
// instead of silently running the alphabetically first candidate.
func TestResolverSharedEnvKeyDoesNotGuessAVendor(t *testing.T) {
	clearModelEnv(t)
	t.Setenv("OPENAI_API_KEY", "sk-shared")
	dir := t.TempDir()
	path := settings.Path(dir)
	if _, err := settings.Save(path, settings.Settings{Provider: settings.ProviderDeepSeek, ApiKey: "sk-settings"}); err != nil {
		t.Fatal(err)
	}
	resolver := newModelResolver(config.Default(), path, testCatalog(t), testModelHost(t))
	if resolver.Frozen() {
		t.Fatal("an environment that names no single candidate must not freeze the session")
	}
	reason := resolver.FrozenAmbiguity()
	if !strings.Contains(reason, "OPENAI_API_KEY") || !strings.Contains(reason, "aihubmix") || !strings.Contains(reason, "openai") {
		t.Fatalf("FrozenAmbiguity() = %q, want the shared variable and both candidate vendors", reason)
	}
	cur := resolver.Current()
	if cur.Frozen || cur.Provider != "deepseek" || cur.APIKey != "sk-settings" {
		t.Fatalf("current selection = %+v, want the settings document's vendor", cur)
	}
}

// TestResolverAmbiguousEnvHonoursVIVYProvider: the shared variable does not
// identify a vendor, but VIVY_PROVIDER still does, by name.
func TestResolverAmbiguousEnvHonoursVIVYProvider(t *testing.T) {
	clearModelEnv(t)
	t.Setenv("OPENAI_API_KEY", "sk-shared")
	t.Setenv(envProviderOverride, "aihubmix")
	resolver := newModelResolver(config.Default(), settings.Path(t.TempDir()), testCatalog(t), testModelHost(t))
	cur := resolver.Current()
	if !cur.Frozen || cur.Provider != "aihubmix" || cur.APIKey != "sk-shared" {
		t.Fatalf("VIVY_PROVIDER=aihubmix resolved %+v", cur)
	}
	if reason := resolver.FrozenAmbiguity(); reason != "" {
		t.Fatalf("FrozenAmbiguity() = %q, want empty for an explicit selection", reason)
	}
}

// TestResolverAmbiguousEnvHonoursConfiguredVendor: with the environment naming
// both candidates, the configured vendor is the tie-break.
func TestResolverAmbiguousEnvHonoursConfiguredVendor(t *testing.T) {
	clearModelEnv(t)
	t.Setenv("OPENAI_API_KEY", "sk-shared")
	cfg := config.Default()
	cfg.Providers.Active = "openai"
	resolver := newModelResolver(cfg, settings.Path(t.TempDir()), testCatalog(t), testModelHost(t))
	cur := resolver.Current()
	if !cur.Frozen || cur.Provider != "openai" || cur.APIKey != "sk-shared" {
		t.Fatalf("configured vendor openai resolved %+v", cur)
	}
}

// TestResolverAmbiguousEnvRejectsAmbiguousAdapterOverride: an adapter both
// candidates speak is not a selection either, so it must not pick one.
func TestResolverAmbiguousEnvRejectsAmbiguousAdapterOverride(t *testing.T) {
	clearModelEnv(t)
	t.Setenv("OPENAI_API_KEY", "sk-shared")
	t.Setenv(envProviderOverride, provider.AdapterOpenAICompletions)
	resolver := newModelResolver(config.Default(), settings.Path(t.TempDir()), testCatalog(t), testModelHost(t))
	if resolver.Frozen() {
		t.Fatal("VIVY_PROVIDER naming an adapter several candidates speak must not pick one")
	}
	if resolver.FrozenAmbiguity() == "" {
		t.Fatal("expected an ambiguity note")
	}
}

// TestResolverSingleCredentialStillFreezesTheSession: the ordinary single-key
// startup is unchanged — one candidate needs no tie-break.
func TestResolverSingleCredentialStillFreezesTheSession(t *testing.T) {
	clearModelEnv(t)
	t.Setenv("MINIMAX_API_KEY", "sk-minimax")
	resolver := newModelResolver(config.Default(), settings.Path(t.TempDir()), testCatalog(t), testModelHost(t))
	cur := resolver.Current()
	if !cur.Frozen || cur.Provider != "minimax" || cur.APIKey != "sk-minimax" {
		t.Fatalf("a single credential resolved %+v", cur)
	}
	if reason := resolver.FrozenAmbiguity(); reason != "" {
		t.Fatalf("FrozenAmbiguity() = %q, want empty when nothing is ambiguous", reason)
	}
}
