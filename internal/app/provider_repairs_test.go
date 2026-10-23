package app

import (
	"io"
	"log/slog"
	"os"
	"strings"
	"testing"

	"agent-vivy/internal/app/settings"
	"agent-vivy/internal/config"
	"agent-vivy/internal/provider"
)

func clearRepairModelEnv(t *testing.T, catalog *provider.Catalog) {
	t.Helper()
	for _, key := range append(provider.VendorEnvKeys(catalog.Vendors()), envProviderOverride, envModelOverride, provider.APIBaseEnvVar) {
		t.Setenv(key, "")
		if err := os.Unsetenv(key); err != nil {
			t.Fatal(err)
		}
	}
}

func TestProviderRepairCustomCredentialIsolation(t *testing.T) {
	for _, initial := range []string{"", "builtin-secret"} {
		t.Run(initial, func(t *testing.T) {
			catalog := testCatalog(t)
			clearRepairModelEnv(t, catalog)
			if initial != "" {
				t.Setenv("DEEPSEEK_API_KEY", initial)
			}
			before, existed := os.LookupEnv("DEEPSEEK_API_KEY")
			entry := settings.ProviderEntry{ID: "deepseek", DisplayName: "deepseek", Bundle: provider.AdapterOpenAICompletions, BaseURL: "https://custom.example/v1", DefaultModel: "custom-model", Models: []string{"custom-model"}, ApiKey: "custom-secret"}
			s := settings.Settings{Provider: entry.Bundle, BaseURL: entry.BaseURL, Providers: []settings.ProviderEntry{entry}}
			cfg := config.Default()
			applySettingsEnv(slog.New(slog.NewTextHandler(io.Discard, nil)), catalog, cfg, s)
			after, exists := os.LookupEnv("DEEPSEEK_API_KEY")
			if after != before || exists != existed {
				t.Error("custom credential modified the embedded vendor environment")
			}
			if os.Getenv(provider.APIBaseEnvVar) != entry.BaseURL {
				t.Error("base URL overlay was lost")
			}
			// Build a fresh resolver with no process credentials to exercise the settings path.
			clearRepairModelEnv(t, catalog)
			path := settings.Path(t.TempDir())
			if _, err := settings.Save(path, s); err != nil {
				t.Fatal(err)
			}
			live := newModelResolver(cfg, path, catalog, testModelHost(t)).Live()
			if !live.UserDefined || !live.Ready || live.APIKey != entry.ApiKey || live.BaseURL != entry.BaseURL {
				t.Error("custom settings credential no longer resolves")
			}
		})
	}
}

func TestProviderRepairEnvironmentSelection(t *testing.T) {
	vendor := func(name, key, adapter string) provider.Vendor {
		return provider.Vendor{Name: name, EnvKey: key, Endpoints: []provider.Endpoint{{Adapter: adapter, BaseURL: "https://" + name + ".example/v1", DefaultModel: "model"}}}
	}
	a := vendor("a", "REPAIR_A_KEY", provider.AdapterOpenAICompletions)
	b := vendor("b", "REPAIR_B_KEY", provider.AdapterOpenAICompletions)
	c := vendor("c", "REPAIR_C_KEY", provider.AdapterAnthropicMessages)
	shared := vendor("b", "REPAIR_A_KEY", provider.AdapterOpenAICompletions)
	for _, tc := range []struct {
		name, active, override, want string
		vendors                      []provider.Vendor
	}{
		{"multiple credentials", "missing", "", "", []provider.Vendor{a, b}},
		{"shared credential", "missing", "", "", []provider.Vendor{a, shared}},
		{"named override", "a", "b", "b", []provider.Vendor{a, b}},
		{"unique adapter", "a", provider.AdapterAnthropicMessages, "c", []provider.Vendor{a, c}},
		{"ambiguous adapter", "missing", provider.AdapterOpenAICompletions, "", []provider.Vendor{a, b}},
		{"configured fallback", "b", provider.AdapterOpenAICompletions, "b", []provider.Vendor{a, b}},
		{"invalid override fallback", "b", "invalid", "b", []provider.Vendor{a, b}},
		{"single candidate", "missing", "invalid", "a", []provider.Vendor{a}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			catalog := provider.NewCatalog(tc.vendors...)
			clearRepairModelEnv(t, catalog)
			for _, v := range tc.vendors {
				t.Setenv(v.EnvKey, "credential-do-not-log")
			}
			t.Setenv(envProviderOverride, tc.override)
			cfg := config.Default()
			cfg.Providers.Active = tc.active
			path := settings.Path(t.TempDir())
			if _, err := settings.Save(path, settings.Settings{Provider: settings.ProviderDeepSeek, ApiKey: "settings-secret"}); err != nil {
				t.Fatal(err)
			}
			r := newModelResolver(cfg, path, catalog, nil)
			if tc.want == "" {
				if r.Frozen() {
					t.Error("ambiguous credentials froze an arbitrary provider")
				}
				cur := r.Current()
				if cur.Frozen || cur.APIKey != "settings-secret" {
					t.Error("ambiguity did not preserve settings")
				}
			} else if cur := r.Current(); !cur.Frozen || cur.Provider != tc.want {
				t.Errorf("selected %q, want frozen %q", cur.Provider, tc.want)
			}
			diagnostics, ok := any(r).(interface{ FrozenAmbiguity() string })
			if !ok {
				t.Error("missing ambiguity diagnostic")
				return
			}
			reason := diagnostics.FrozenAmbiguity()
			if strings.Contains(reason, "credential-do-not-log") || strings.Contains(reason, "settings-secret") {
				t.Error("diagnostic leaked a credential")
			}
			if tc.want == "" && (!strings.Contains(reason, "REPAIR_A_KEY") || !strings.Contains(reason, "a") || !strings.Contains(reason, "b")) {
				t.Errorf("missing candidate diagnostic: %q", reason)
			}
			if tc.want != "" && reason != "" {
				t.Errorf("unexpected diagnostic: %s", reason)
			}
		})
	}
}
