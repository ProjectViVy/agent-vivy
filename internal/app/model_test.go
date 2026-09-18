package app

import (
	"os"
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
