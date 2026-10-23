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
		name        string
		stored      settings.Settings
		vendor      string
		adapter     string
		model       string
		ok          bool
		userDefined bool
	}{
		{
			name: "declared address identifies its vendor",
			stored: settings.Settings{
				Provider: provider.AdapterOpenAICompletions, BaseURL: "https://token.sensenova.cn/v1",
			},
			vendor: "sensenova", adapter: provider.AdapterOpenAICompletions, model: "sensenova-6.8-flash-lite", ok: true,
		},
		{
			name:   "legacy vendor without an address keeps its default endpoint",
			stored: settings.Settings{Provider: settings.ProviderDeepSeek},
			vendor: "deepseek", adapter: provider.AdapterOpenAICompletions, model: "deepseek-flash", ok: true,
		},
		{
			name:   "adapter without an address falls back to the configured vendor",
			stored: settings.Settings{Provider: provider.AdapterAnthropicMessages},
			// config.Default() activates DeepSeek, which declares an
			// anthropic-messages endpoint of its own.
			vendor: "deepseek", adapter: provider.AdapterAnthropicMessages, model: "deepseek-chat", ok: true,
		},
		{
			// A vendor the catalog no longer carries resolves non-fatally: the
			// stored name is kept, the endpoint is not.
			name: "a pruned pre-migration vendor stays readable but unusable",
			stored: settings.Settings{
				Provider: settings.ProviderOpenAI, BaseURL: "https://my-gateway.example/v1",
			},
			vendor: "openai", adapter: provider.AdapterOpenAICompletions, model: "", ok: false,
		},
		{
			name: "an address no vendor declares falls back to the configured vendor",
			stored: settings.Settings{
				Provider: provider.AdapterOpenAICompletions, BaseURL: "https://nothing.example/v1",
			},
			vendor: "deepseek", adapter: provider.AdapterOpenAICompletions, model: "deepseek-flash", ok: true,
		},
		{
			name: "a registered custom provider labels itself, not the configured vendor",
			stored: settings.Settings{
				Provider: provider.AdapterOpenAICompletions, BaseURL: "https://llm.acme.example/v1",
				Providers: []settings.ProviderEntry{{
					ID: "acme", DisplayName: "Acme LLM",
					Bundle: provider.AdapterOpenAICompletions, BaseURL: "https://llm.acme.example/v1",
					DefaultModel: "acme-1", Models: []string{"acme-1"},
				}},
			},
			// The label and endpoint come from the user's registry entry; the
			// sealed OpenAI-compatible adapter can execute it without vendor data.
			vendor: "Acme LLM", adapter: provider.AdapterOpenAICompletions, model: "acme-1", ok: true, userDefined: true,
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
			if selection.UserDefined != test.userDefined {
				t.Fatalf("selection.UserDefined = %v, want %v", selection.UserDefined, test.userDefined)
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

// applySettingsEnv writes a key into the env var of the vendor the selection
// resolves to. Before PROV-P3 it switched on the three config blocks, so
// selecting a third-party endpoint through the openai-compatible adapter wrote
// that vendor's key into OPENAI_API_KEY.
func TestApplySettingsEnvWritesTheVendorsOwnVariable(t *testing.T) {
	t.Setenv(provider.APIBaseEnvVar, "")
	t.Setenv("MINIMAX_API_KEY", "")
	t.Setenv("SENSENOVA_API_KEY", "")
	openAIKeyBefore := os.Getenv("OPENAI_API_KEY")

	cfg := config.Default()
	cfg.Storage.DataDir = t.TempDir()
	longLived := settings.Settings{
		Provider:     provider.AdapterOpenAICompletions,
		BaseURL:      "https://token.sensenova.cn/v1",
		DefaultModel: "sensenova-6.8-flash-lite",
		Providers: []settings.ProviderEntry{{
			ID: "custom-1", DisplayName: "SenseNova", Bundle: provider.AdapterOpenAICompletions,
			BaseURL: "https://token.sensenova.cn/v1", ApiKey: "sk-sensenova",
		}},
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	applySettingsEnv(logger, testCatalog(t), cfg, longLived)

	if got := os.Getenv("SENSENOVA_API_KEY"); got != "sk-sensenova" {
		t.Fatalf("SENSENOVA_API_KEY = %q, want the registry key", got)
	}
	if got := os.Getenv("OPENAI_API_KEY"); got != openAIKeyBefore {
		t.Fatalf("OPENAI_API_KEY changed to %q; a third-party key must never be written into another vendor's variable", got)
	}
	if got := os.Getenv(provider.APIBaseEnvVar); got != "https://token.sensenova.cn/v1" {
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
