package app

import (
	"context"
	"io"
	"log/slog"
	"path/filepath"
	"testing"

	"agent-vivy/internal/app/settings"
	"agent-vivy/internal/config"
)

func TestApplySettingsOverlayAppliesNetworkSearchProvider(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Config{
		Storage:   config.Storage{DataDir: dir, Backend: "sqlite"},
		Providers: config.Providers{OpenAI: config.Provider{EnvKey: "VIVY_TEST_API_KEY_NS"}},
		Tools:     config.Tools{NetworkSearch: config.NetworkSearchConfig{Provider: "bing"}},
	}
	if _, err := settings.Save(settings.Path(dir), settings.Settings{
		NetworkSearch: settings.NetworkSearchSettings{Provider: "searxng"},
	}); err != nil {
		t.Fatal(err)
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	applied := applySettingsOverlay(context.Background(), logger, cfg)
	if applied.Tools.NetworkSearch.Provider != "searxng" {
		t.Fatalf("network_search provider = %q, want searxng (settings override)", applied.Tools.NetworkSearch.Provider)
	}

	dir2 := t.TempDir()
	cfg2 := config.Config{
		Storage:   config.Storage{DataDir: dir2, Backend: "sqlite"},
		Providers: config.Providers{OpenAI: config.Provider{EnvKey: "VIVY_TEST_API_KEY_NS2"}},
		Tools:     config.Tools{NetworkSearch: config.NetworkSearchConfig{Provider: "wikipedia"}},
	}
	if _, err := settings.Save(settings.Path(dir2), settings.Settings{}); err != nil {
		t.Fatal(err)
	}
	applied2 := applySettingsOverlay(context.Background(), logger, cfg2)
	if applied2.Tools.NetworkSearch.Provider != "wikipedia" {
		t.Fatalf("network_search provider = %q, want config default wikipedia", applied2.Tools.NetworkSearch.Provider)
	}
}

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

func TestApplySettingsOverlayIgnoresMockProvider(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Config{
		Storage:   config.Storage{DataDir: dir, Backend: "sqlite"},
		Providers: config.Providers{Active: "openai", OpenAI: config.Provider{EnvKey: "VIVY_TEST_API_KEY_MOCK"}},
	}
	if _, err := settings.Save(settings.Path(dir), settings.Settings{Provider: settings.ProviderOpenAI}); err != nil {
		t.Fatal(err)
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	applied := applySettingsOverlay(context.Background(), logger, cfg)
	if applied.Providers.Active != "openai" {
		t.Fatalf("active = %q, want openai", applied.Providers.Active)
	}
}

func TestApplySettingsOverlayExecuteTimeout(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	base := config.Default()
	if base.Runtime.ExecuteMaxTimeoutSeconds != 30 {
		t.Fatalf("config default ceiling = %d, want 30", base.Runtime.ExecuteMaxTimeoutSeconds)
	}
	base.Storage.SQLite.Path = filepath.Join(t.TempDir(), "vivy.db")

	got := applySettingsOverlay(context.Background(), logger, base)
	if got.Runtime.ExecuteMaxTimeoutSeconds != 30 {
		t.Fatalf("empty overlay changed ceiling to %d", got.Runtime.ExecuteMaxTimeoutSeconds)
	}

	if _, err := settings.Save(settings.Path(base.DataDirectory()), settings.Settings{ExecuteMaxTimeoutSeconds: 300}); err != nil {
		t.Fatal(err)
	}
	got = applySettingsOverlay(context.Background(), logger, base)
	if got.Runtime.ExecuteMaxTimeoutSeconds != 300 {
		t.Fatalf("overlay ceiling = %d, want 300", got.Runtime.ExecuteMaxTimeoutSeconds)
	}
}

func TestApplySettingsOverlayMCPServers(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	dir := t.TempDir()
	cfg := config.Config{
		Storage: config.Storage{DataDir: dir, Backend: "sqlite"},
		Runtime: config.Runtime{MCPServers: []config.MCPServer{{Name: "from-config", Endpoint: "https://config.example.com/mcp"}}},
	}

	got := applySettingsOverlay(context.Background(), logger, cfg)
	if len(got.Runtime.MCPServers) != 1 || got.Runtime.MCPServers[0].Name != "from-config" {
		t.Fatalf("missing overlay changed mcp = %+v", got.Runtime.MCPServers)
	}

	list := []settings.MCPServer{
		{Name: "docs", Endpoint: "https://docs.example.com/mcp"},
		{Name: "idle", Endpoint: "http://127.0.0.1:9/mcp", Enabled: settings.BoolPtr(false)},
	}
	if _, err := settings.Save(settings.Path(dir), settings.Settings{MCPServers: &list}); err != nil {
		t.Fatal(err)
	}
	got = applySettingsOverlay(context.Background(), logger, cfg)
	if len(got.Runtime.MCPServers) != 1 || got.Runtime.MCPServers[0].Name != "docs" {
		t.Fatalf("overlay mcp = %+v, want enabled docs only", got.Runtime.MCPServers)
	}

	empty := []settings.MCPServer{}
	dir2 := t.TempDir()
	cfg2 := config.Config{
		Storage: config.Storage{DataDir: dir2, Backend: "sqlite"},
		Runtime: config.Runtime{MCPServers: []config.MCPServer{{Name: "from-config", Endpoint: "https://config.example.com/mcp"}}},
	}
	if _, err := settings.Save(settings.Path(dir2), settings.Settings{MCPServers: &empty}); err != nil {
		t.Fatal(err)
	}
	got = applySettingsOverlay(context.Background(), logger, cfg2)
	if got.Runtime.MCPServers == nil {
		got.Runtime.MCPServers = []config.MCPServer{}
	}
	if len(got.Runtime.MCPServers) != 0 {
		t.Fatalf("explicit empty overlay must replace config, got %+v", got.Runtime.MCPServers)
	}
}

func TestApplySettingsOverlaySandboxPreset(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	base := config.Default()
	base.Storage.SQLite.Path = filepath.Join(t.TempDir(), "vivy.db")
	deny := false
	if _, err := settings.Save(settings.Path(base.DataDirectory()), settings.Settings{
		Sandbox: settings.SandboxSettings{
			DefaultPreset: "cautious",
			Network:       settings.SandboxNetworkSettings{DenyPrivateIPs: &deny, AllowedDomains: []string{"example.com"}},
		},
	}); err != nil {
		t.Fatal(err)
	}
	got := applySettingsOverlay(context.Background(), logger, base)
	if got.Runtime.Sandbox.DefaultMode != "read_only" || got.Runtime.Sandbox.Approval.DefaultPolicy != "ask" {
		t.Fatalf("sandbox overlay = %+v", got.Runtime.Sandbox)
	}
	if got.Runtime.Sandbox.Network.DenyPrivateIPs {
		t.Fatal("deny_private_ips overlay not applied")
	}
	if len(got.Runtime.Sandbox.Network.AllowedDomains) != 1 || got.Runtime.Sandbox.Network.AllowedDomains[0] != "example.com" {
		t.Fatalf("allowed domains = %+v", got.Runtime.Sandbox.Network.AllowedDomains)
	}
}
