package app

import (
	"context"
	"io"
	"log/slog"
	"os"
	"testing"

	"agent-vivy/internal/app/settings"
	"agent-vivy/internal/config"
)

// TestApplySettingsOverlayAppliesAPIKey pins the end-to-end key path:
// an api_key stored in the settings overlay is applied to the active
// bundle's env_key environment variable at startup.
func TestApplySettingsOverlayAppliesAPIKey(t *testing.T) {
	keyEnv := "VIVY_TEST_API_KEY_OVERLAY"
	t.Setenv(keyEnv, "")

	dir := t.TempDir()
	cfg := config.Config{
		Storage:   config.Storage{DataDir: dir, Backend: "sqlite"},
		Providers: config.Providers{OpenAI: config.Provider{EnvKey: keyEnv}},
	}
	if _, err := settings.Save(settings.Path(dir), settings.Settings{
		Provider: settings.ProviderOpenAI, DefaultModel: "gpt-4o", ApiKey: "sk-overlay",
	}); err != nil {
		t.Fatal(err)
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	applied := applySettingsOverlay(context.Background(), logger, cfg)
	if applied.Providers.Active != "openai" {
		t.Fatalf("active provider not overlaid: %q", applied.Providers.Active)
	}
	if got := os.Getenv(keyEnv); got != "sk-overlay" {
		t.Fatalf("api_key env = %q, want sk-overlay", got)
	}
}

// TestApplySettingsOverlayEmptyKeyKeepsEnv ensures an empty api_key does not
// clobber the bundle's environment variable, so env-injected keys keep
// working for catalog/default flows.
func TestApplySettingsOverlayEmptyKeyKeepsEnv(t *testing.T) {
	keyEnv := "VIVY_TEST_API_KEY_EMPTY"
	t.Setenv(keyEnv, "")

	dir := t.TempDir()
	cfg := config.Config{
		Storage:   config.Storage{DataDir: dir, Backend: "sqlite"},
		Providers: config.Providers{OpenAI: config.Provider{EnvKey: keyEnv}},
	}
	if _, err := settings.Save(settings.Path(dir), settings.Settings{
		Provider: settings.ProviderOpenAI, ApiKey: "",
	}); err != nil {
		t.Fatal(err)
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	applySettingsOverlay(context.Background(), logger, cfg)
	if got := os.Getenv(keyEnv); got != "" {
		t.Fatalf("empty api_key must not touch env, got %q", got)
	}
}

// TestApplySettingsOverlayNoDocumentIsNoop guards the config-default path:
// with no settings document the overlay leaves cfg untouched.
func TestApplySettingsOverlayNoDocumentIsNoop(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Config{
		Storage:   config.Storage{DataDir: dir, Backend: "sqlite"},
		Providers: config.Providers{OpenAI: config.Provider{EnvKey: "VIVY_TEST_API_KEY_MISSING"}},
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	applied := applySettingsOverlay(context.Background(), logger, cfg)
	if applied.Providers.Active != "" {
		t.Fatalf("missing settings document must not change cfg, active = %q", applied.Providers.Active)
	}
}
