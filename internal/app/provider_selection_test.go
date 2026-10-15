package app

import (
	"io"
	"log/slog"
	"os"
	"testing"

	"agent-vivy/internal/app/settings"
	"agent-vivy/internal/config"
	"agent-vivy/internal/provider"
)

// The selection chain after PROV-P3 (DESIGN.md §5): the stored value names an
// adapter, the address names a vendor when an embedded endpoint declares it,
// a pre-migration value names its vendor directly, and the configured vendor is
// the last resort. These cases pin each rung.
func TestResolveStoredSelectionChain(t *testing.T) {
	catalog := testCatalog(t)
	cfg := config.Default()

	for _, test := range []struct {
		name    string
		stored  settings.Settings
		vendor  string
		adapter string
		model   string
		ok      bool
	}{
		{
			name: "declared address identifies its vendor",
			stored: settings.Settings{
				Provider: provider.AdapterOpenAICompletions, BaseURL: "https://api.minimaxi.com/v1",
			},
			vendor: "minimax", adapter: provider.AdapterOpenAICompletions, model: "MiniMax-M2.1", ok: true,
		},
		{
			name:   "legacy vendor without an address keeps its default endpoint",
			stored: settings.Settings{Provider: settings.ProviderAnthropic},
			vendor: "anthropic", adapter: provider.AdapterAnthropicMessages, model: "claude-sonnet-4-5", ok: true,
		},
		{
			name:   "adapter without an address falls back to the configured vendor",
			stored: settings.Settings{Provider: provider.AdapterAnthropicMessages},
			// config.Default() activates DeepSeek, which declares an
			// anthropic-messages endpoint of its own.
			vendor: "deepseek", adapter: provider.AdapterAnthropicMessages, model: "deepseek-chat", ok: true,
		},
		{
			name: "a pre-migration vendor keeps ownership of an undeclared gateway",
			stored: settings.Settings{
				Provider: settings.ProviderOpenAI, BaseURL: "https://my-gateway.example/v1",
			},
			vendor: "openai", adapter: provider.AdapterOpenAICompletions, model: "gpt-4o", ok: true,
		},
		{
			name: "an address no vendor declares falls back to the configured vendor",
			stored: settings.Settings{
				Provider: provider.AdapterOpenAICompletions, BaseURL: "https://nothing.example/v1",
			},
			vendor: "deepseek", adapter: provider.AdapterOpenAICompletions, model: "deepseek-flash", ok: true,
		},
		{
			name: "a registered custom provider resolves to its own endpoint",
			stored: settings.Settings{
				Provider: provider.AdapterOpenAICompletions, BaseURL: "https://llm.acme.example/v1",
				DefaultModel: "acme-1",
				Providers: []settings.ProviderEntry{{
					ID: "acme", DisplayName: "Acme LLM",
					Bundle: provider.AdapterOpenAICompletions, BaseURL: "https://llm.acme.example/v1",
					DefaultModel: "acme-1", Models: []string{"acme-1", "acme-2"},
				}},
			},
			// The alias labels the selection; the endpoint the registry entry
			// declares is what the runtime builds the model from, so the
			// embedded catalog never has to know the name (REGRESSION: a run
			// used to fail with `provider "Acme LLM": no embedded vendor data`).
			vendor: "Acme LLM", adapter: provider.AdapterOpenAICompletions, model: "acme-1", ok: true,
		},
		{
			name: "a registered custom provider fills an absent model from its own default",
			stored: settings.Settings{
				Provider: provider.AdapterOpenAICompletions, BaseURL: "https://llm.acme.example/v1",
				Providers: []settings.ProviderEntry{{
					ID: "acme", DisplayName: "Acme LLM",
					Bundle: provider.AdapterOpenAICompletions, BaseURL: "https://llm.acme.example/v1",
					DefaultModel: "acme-1", Models: []string{"acme-1"},
				}},
			},
			vendor: "Acme LLM", adapter: provider.AdapterOpenAICompletions, model: "acme-1", ok: true,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			selection, ok := resolveStoredSelection(catalog, cfg, test.stored)
			if ok != test.ok {
				t.Fatalf("ok = %v, want %v (%+v)", ok, test.ok, selection)
			}
			if selection.Vendor != test.vendor || selection.Adapter != test.adapter {
				t.Fatalf("selection = vendor %q adapter %q, want %q/%q", selection.Vendor, selection.Adapter, test.vendor, test.adapter)
			}
			if test.model != "" && selection.Model != test.model {
				t.Fatalf("model = %q, want %q", selection.Model, test.model)
			}
		})
	}
}

// A zero document names nothing; the resolver must not invent a selection.
func TestResolveStoredSelectionEmptyDocument(t *testing.T) {
	if selection, ok := resolveStoredSelection(testCatalog(t), config.Default(), settings.Settings{}); ok || selection.Vendor != "" {
		t.Fatalf("empty document resolved to %+v, %v", selection, ok)
	}
}

// A user-registered provider (Settings → Models, a `custom-<uuid>` entry) must
// run without the embedded catalog knowing its name. Before this regression
// test, the registry alias became the vendor name and the run died with
// `provider "WXYG": no embedded vendor data` (`run failed` in the backend log).
func TestRegisteredCustomProviderRunsFromItsOwnEndpoint(t *testing.T) {
	entry := settings.ProviderEntry{
		ID: "custom-05e534a9", DisplayName: "WXYG", Bundle: provider.AdapterOpenAICompletions,
		BaseURL: "https://api.gateway.example/v1", DefaultModel: "gw-chat",
		Models: []string{"gw-chat", "gw-reasoner"}, ApiKey: "sk-registry",
	}
	stored := settings.Settings{
		Provider: provider.AdapterOpenAICompletions, DefaultModel: "gw-chat",
		BaseURL: entry.BaseURL, Providers: []settings.ProviderEntry{entry},
	}

	selection, ok := resolveStoredSelection(testCatalog(t), config.Default(), stored)
	if !ok {
		t.Fatalf("a registered provider must resolve: %+v", selection)
	}
	if selection.endpoint.Adapter != provider.AdapterOpenAICompletions ||
		selection.endpoint.BaseURL != entry.BaseURL ||
		selection.endpoint.DefaultModel != "gw-chat" {
		t.Fatalf("custom selection must carry the registry endpoint, got %+v", selection.endpoint)
	}
	if len(selection.endpoint.Models) != 2 {
		t.Fatalf("custom endpoint models = %+v, want the registry's two ids", selection.endpoint.Models)
	}

	dir := t.TempDir()
	path := settings.Path(dir)
	if _, err := settings.Save(path, stored); err != nil {
		t.Fatal(err)
	}
	live := newModelResolver(config.Default(), path, testCatalog(t), testModelHost(t)).Live()
	if live.Provider != "WXYG" || live.Adapter != provider.AdapterOpenAICompletions || !live.Ready {
		t.Fatalf("live selection = %+v", live)
	}
	if live.Endpoint.BaseURL != entry.BaseURL || live.Endpoint.Adapter != provider.AdapterOpenAICompletions {
		t.Fatalf("live endpoint = %+v, want the registered gateway", live.Endpoint)
	}
	if live.APIKey != entry.ApiKey {
		t.Fatalf("live credential = %q, want the registry key", live.APIKey)
	}
}

// applySettingsEnv writes a key into the env var of the vendor the selection
// resolves to. Before PROV-P3 it switched on the three config blocks, so
// selecting a third-party endpoint through the openai-compatible adapter wrote
// that vendor's key into OPENAI_API_KEY.
func TestApplySettingsEnvWritesTheVendorsOwnVariable(t *testing.T) {
	t.Setenv(provider.APIBaseEnvVar, "")
	os.Unsetenv("MINIMAX_API_KEY")
	openAIKeyBefore := os.Getenv("OPENAI_API_KEY")

	cfg := config.Default()
	cfg.Storage.DataDir = t.TempDir()
	longLived := settings.Settings{
		Provider:     provider.AdapterOpenAICompletions,
		BaseURL:      "https://api.minimaxi.com/v1",
		DefaultModel: "MiniMax-M2.1",
		Providers: []settings.ProviderEntry{{
			ID: "custom-1", DisplayName: "MiniMax", Bundle: provider.AdapterOpenAICompletions,
			BaseURL: "https://api.minimaxi.com/v1", ApiKey: "sk-minimax",
		}},
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	applySettingsEnv(logger, testCatalog(t), cfg, longLived)

	if got := os.Getenv("MINIMAX_API_KEY"); got != "sk-minimax" {
		t.Fatalf("MINIMAX_API_KEY = %q, want the registry key", got)
	}
	if got := os.Getenv("OPENAI_API_KEY"); got != openAIKeyBefore {
		t.Fatalf("OPENAI_API_KEY changed to %q; a third-party key must never be written into another vendor's variable", got)
	}
	if got := os.Getenv(provider.APIBaseEnvVar); got != "https://api.minimaxi.com/v1" {
		t.Fatalf("%s = %q, want the selected address", provider.APIBaseEnvVar, got)
	}
}

// A selection that resolves to no vendor leaves the environment alone, and the
// legacy overlay field still supplies the key.
func TestApplySettingsEnvEmptySelectionTouchesNothing(t *testing.T) {
	t.Setenv(provider.APIBaseEnvVar, "")
	cfg := config.Default()
	cfg.Storage.DataDir = t.TempDir()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	applySettingsEnv(logger, testCatalog(t), cfg, settings.Settings{})
	if got := os.Getenv(provider.APIBaseEnvVar); got != "" {
		t.Fatalf("%s = %q, want untouched", provider.APIBaseEnvVar, got)
	}
}
