package app

import (
	"os"
	"path/filepath"
	"testing"

	"agent-vivy/internal/app/settings"
	"agent-vivy/internal/config"
	"agent-vivy/internal/provider"
)

func TestMain(m *testing.M) {
	os.Unsetenv("OPENAI_API_KEY")
	os.Unsetenv("ANTHROPIC_API_KEY")
	os.Exit(m.Run())
}

func testCatalog(t *testing.T) *provider.Catalog {
	t.Helper()
	openai, err := provider.LoadBundle(filepath.Join("..", "..", "fixtures", "provider", "openai.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	anthropic, err := provider.LoadBundle(filepath.Join("..", "..", "fixtures", "provider", "anthropic.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	return provider.NewCatalog(openai, anthropic)
}

func TestResolverEmptyWithoutSettings(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Default()
	cfg.Storage.DataDir = dir
	r := newModelResolver(cfg, settings.Path(dir), testCatalog(t))
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
	r := newModelResolver(config.Default(), path, testCatalog(t))
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
	t.Setenv("OPENAI_API_KEY", "sk-env")
	t.Setenv("VIVY_API_BASE", "https://env.example.com/v1")
	t.Setenv("VIVY_MODEL", "env-model")
	r := newModelResolver(config.Default(), path, testCatalog(t))
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
	r := newModelResolver(config.Default(), path, testCatalog(t))
	if r.Current().Ready {
		t.Fatal("expected empty")
	}
	if _, err := settings.Save(path, settings.Settings{
		Provider: settings.ProviderOpenAI, DefaultModel: "gpt-4o-mini", ApiKey: "sk-later",
	}); err != nil {
		t.Fatal(err)
	}
	r.Invalidate()
	cur := r.Current()
	if !cur.Ready || cur.APIKey != "sk-later" {
		t.Fatalf("after save = %+v", cur)
	}
}
