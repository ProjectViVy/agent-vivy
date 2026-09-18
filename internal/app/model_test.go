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

func testProfile(t *testing.T, vendor provider.Vendor) providerprofile.Profile {
	t.Helper()
	endpoint, ok := vendor.DefaultEndpoint()
	if !ok {
		t.Fatalf("vendor %q has no default endpoint", vendor.Name)
	}
	return provider.ProfileFromEndpoint(vendor, endpoint)
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
	index := testVendors(t)
	host, err := modelhost.New([]providerprofile.Profile{
		testProfile(t, index["deepseek"]), testProfile(t, index["openai"]), testProfile(t, index["anthropic"]),
	}, modelhost.Capabilities{
		provider.AdapterFamilyOpenAICompatible: modelhost.CapabilitySupported,
		provider.AdapterFamilyAnthropic:        modelhost.CapabilitySupported,
	})
	if err != nil {
		t.Fatal(err)
	}
	return host
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
	openai := testVendors(t)["openai"]
	host, err := modelhost.New([]providerprofile.Profile{testProfile(t, openai)}, modelhost.Capabilities{
		provider.AdapterFamilyOpenAICompatible: modelhost.CapabilitySupported,
	})
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
	resolver := newModelResolver(config.Default(), path, testCatalog(t), host)
	current := resolver.Current()
	statuses := host.Statuses(current.Provider, current.Ready)
	if len(statuses) != 3 || statuses[1].ID != "deepseek" || statuses[1].State != modelhost.ProfileReady {
		t.Fatalf("resolver Profile statuses = %#v", statuses)
	}
	if statuses[1].AdapterFamily != provider.AdapterFamilyOpenAICompatible || statuses[1].EndpointClass == "" {
		t.Fatalf("resolver Profile provenance = %#v", statuses[1])
	}
}
