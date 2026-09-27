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
