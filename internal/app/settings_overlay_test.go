package app

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"

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
	applied := applySettingsOverlay(context.Background(), logger, cfg, nil)
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
	applied2 := applySettingsOverlay(context.Background(), logger, cfg2, nil)
	if applied2.Tools.NetworkSearch.Provider != "wikipedia" {
		t.Fatalf("network_search provider = %q, want config default wikipedia", applied2.Tools.NetworkSearch.Provider)
	}
}

func TestApplySettingsOverlayNormalizesLegacyToolSearch(t *testing.T) {
	dir := t.TempDir()
	path := settings.Path(dir)
	if err := os.WriteFile(path, []byte("tools_enabled:\n  - tool_search\n  - list_dir\n  - tool_search\n  - read_file\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{
		Storage: config.Storage{DataDir: dir, Backend: "sqlite"},
		Tools:   config.Tools{Enabled: []string{"tool_search", "write_file"}},
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	got := applySettingsOverlayAt(context.Background(), logger, cfg, path, nil)
	want := []string{"list_dir", "read_file"}
	if !sameStrings(got.Tools.Enabled, want) {
		t.Fatalf("effective tools.enabled = %#v, want %#v", got.Tools.Enabled, want)
	}

	legacyOnly := []byte("tools_enabled:\n  - tool_search\n")
	if err := os.WriteFile(path, legacyOnly, 0o600); err != nil {
		t.Fatal(err)
	}
	got = applySettingsOverlayAt(context.Background(), logger, cfg, path, nil)
	if got.Tools.Enabled == nil || len(got.Tools.Enabled) != 0 {
		t.Fatalf("legacy-only effective tools.enabled = %#v, want explicit empty", got.Tools.Enabled)
	}
}

func TestApplySettingsOverlayNoDocumentIsNoop(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Config{
		Storage:   config.Storage{DataDir: dir, Backend: "sqlite"},
		Providers: config.Providers{OpenAI: config.Provider{EnvKey: "VIVY_TEST_API_KEY_MISSING"}},
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	applied := applySettingsOverlay(context.Background(), logger, cfg, nil)
	if applied.Providers.Active != "" {
		t.Fatalf("missing settings document must not change cfg, active = %q", applied.Providers.Active)
	}
}

func TestApplySettingsOverlayAtCanBeSharedOutsideRuntimeData(t *testing.T) {
	sharedRoot := t.TempDir()
	privateRoot := t.TempDir()
	cfg := config.Config{
		Storage: config.Storage{DataDir: privateRoot, Backend: "sqlite"},
		Providers: config.Providers{
			Active:    settings.ProviderOpenAI,
			Anthropic: config.Provider{EnvKey: "ANTHROPIC_API_KEY", DefaultModel: "old-model"},
		},
	}
	path := settings.Path(sharedRoot)
	if _, err := settings.Save(path, settings.Settings{Provider: settings.ProviderAnthropic, DefaultModel: "claude-sonnet-4-5"}); err != nil {
		t.Fatalf("save shared settings: %v", err)
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	got := applySettingsOverlayAt(context.Background(), logger, cfg, path, nil)
	if got.Providers.Active != settings.ProviderAnthropic || got.Providers.Anthropic.DefaultModel != "claude-sonnet-4-5" {
		t.Fatalf("shared settings not applied: %+v", got.Providers)
	}
	if got.DataDirectory() != privateRoot {
		t.Fatalf("runtime data root = %q, want private %q", got.DataDirectory(), privateRoot)
	}
}

func TestProviderConfigBaselineSurvivesSettingsOverlay(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Config{
		Storage: config.Storage{DataDir: dir, Backend: "sqlite"},
		Providers: config.Providers{
			Active:    settings.ProviderOpenAI,
			OpenAI:    config.Provider{DefaultModel: "gpt-config"},
			Anthropic: config.Provider{DefaultModel: "claude-config"},
		},
	}
	path := settings.Path(dir)
	if _, err := settings.Save(path, settings.Settings{Provider: settings.ProviderAnthropic, DefaultModel: "claude-overlay"}); err != nil {
		t.Fatal(err)
	}
	providerName, modelID := providerConfigBaseline(cfg)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	applied := applySettingsOverlayAt(context.Background(), logger, cfg, path, nil)
	if providerName != settings.ProviderOpenAI || modelID != "gpt-config" {
		t.Fatalf("captured baseline = %q/%q", providerName, modelID)
	}
	if applied.Providers.Active != settings.ProviderAnthropic || applied.Providers.Anthropic.DefaultModel != "claude-overlay" {
		t.Fatalf("overlay was not independently applied: %+v", applied.Providers)
	}
}

func TestApplySettingsOverlayAppliesProviderSelection(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Config{
		Storage:   config.Storage{DataDir: dir, Backend: "sqlite"},
		Providers: config.Providers{Active: "openai", OpenAI: config.Provider{EnvKey: "VIVY_TEST_API_KEY_SELECTION"}},
	}
	if _, err := settings.Save(settings.Path(dir), settings.Settings{Provider: settings.ProviderOpenAI}); err != nil {
		t.Fatal(err)
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	applied := applySettingsOverlay(context.Background(), logger, cfg, nil)
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

	got := applySettingsOverlay(context.Background(), logger, base, nil)
	if got.Runtime.ExecuteMaxTimeoutSeconds != 30 {
		t.Fatalf("empty overlay changed ceiling to %d", got.Runtime.ExecuteMaxTimeoutSeconds)
	}

	if _, err := settings.Save(settings.Path(base.DataDirectory()), settings.Settings{ExecuteMaxTimeoutSeconds: 300}); err != nil {
		t.Fatal(err)
	}
	got = applySettingsOverlay(context.Background(), logger, base, nil)
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

	got := applySettingsOverlay(context.Background(), logger, cfg, nil)
	if len(got.Runtime.MCPServers) != 1 || got.Runtime.MCPServers[0].Name != "from-config" {
		t.Fatalf("missing overlay changed mcp = %+v", got.Runtime.MCPServers)
	}

	list := []settings.MCPServer{
		{Name: "docs", Endpoint: "https://docs.example.com/mcp"},
		{Name: "local", Command: "node", Args: []string{"server.js", "--stdio"}, EnvFrom: map[string]string{"MCP_TOKEN": "HOST_TOKEN"}, Cwd: "tools"},
		{Name: "idle", Endpoint: "http://127.0.0.1:9/mcp", Enabled: settings.BoolPtr(false)},
	}
	if _, err := settings.Save(settings.Path(dir), settings.Settings{MCPServers: &list}); err != nil {
		t.Fatal(err)
	}
	got = applySettingsOverlay(context.Background(), logger, cfg, nil)
	if len(got.Runtime.MCPServers) != 2 || got.Runtime.MCPServers[0].Name != "docs" || got.Runtime.MCPServers[1].Command != "node" || got.Runtime.MCPServers[1].Cwd != "tools" {
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
	got = applySettingsOverlay(context.Background(), logger, cfg2, nil)
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
	got := applySettingsOverlay(context.Background(), logger, base, nil)
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

// TestApplySettingsOverlayChannels: the per-channel overlay replaces only
// the three knobs it carries, preserves the config envelope's opaque
// Settings node and untouched fields, creates a fresh envelope for a
// compiled-in channel that config.yaml never configured, and drops stale
// entries naming channels that are not compiled into this generation.
func TestApplySettingsOverlayChannels(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	dir := t.TempDir()

	// Build the config envelope through YAML so the opaque Settings node is
	// a real decoded subtree, exactly as config.Load would produce it.
	var cfg config.Config
	if err := yaml.Unmarshal([]byte(`
storage:
  data_dir: `+dir+`
channels:
  telegram:
    enabled: false
    allow_from: [alice]
    token_env: TELEGRAM_BOT_TOKEN
    settings:
      parse_mode: HTML
      poll_timeout: 7
`), &cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.Channels["telegram"].Settings.Kind != yaml.MappingNode {
		t.Fatalf("fixture settings node = %+v, want a mapping", cfg.Channels["telegram"].Settings)
	}

	enabled := true
	senders := []string{"bob", "carol"}
	if _, err := settings.Save(settings.Path(dir), settings.Settings{Channels: []settings.ChannelOverlay{
		{Name: "telegram", Enabled: &enabled, AllowFrom: &senders},
		{Name: "ghost", Enabled: &enabled}, // not compiled into this generation
	}}); err != nil {
		t.Fatal(err)
	}

	got := applySettingsOverlay(context.Background(), logger, cfg, []string{"telegram"})
	if _, dropped := got.Channels["ghost"]; dropped {
		t.Fatal("stale overlay entry for a non-compiled-in channel must be dropped")
	}
	env := got.Channels["telegram"]
	if !env.Enabled {
		t.Fatal("overlay did not enable the channel")
	}
	if len(env.AllowFrom) != 2 || env.AllowFrom[0] != "bob" || env.AllowFrom[1] != "carol" {
		t.Fatalf("allow_from = %+v, want the overlay list", env.AllowFrom)
	}
	if env.TokenEnv != "TELEGRAM_BOT_TOKEN" {
		t.Fatalf("token_env = %q, want the config value preserved (overlay did not carry it)", env.TokenEnv)
	}
	if env.Settings.Kind != yaml.MappingNode {
		t.Fatalf("opaque settings node = %+v, want the config.yaml node preserved", env.Settings)
	}
	var inner struct {
		ParseMode   string `yaml:"parse_mode"`
		PollTimeout int    `yaml:"poll_timeout"`
	}
	if err := env.Settings.Decode(&inner); err != nil {
		t.Fatal(err)
	}
	if inner.ParseMode != "HTML" || inner.PollTimeout != 7 {
		t.Fatalf("settings inner keys lost: %+v", inner)
	}

	// A compiled-in channel with no config envelope gets a fresh one from
	// the overlay entry alone.
	discEnabled := true
	discSenders := []string{"carol"}
	if _, err := settings.Save(settings.Path(dir), settings.Settings{Channels: []settings.ChannelOverlay{
		{Name: "discord", Enabled: &discEnabled, AllowFrom: &discSenders},
	}}); err != nil {
		t.Fatal(err)
	}
	got = applySettingsOverlay(context.Background(), logger, cfg, []string{"telegram", "discord"})
	disc := got.Channels["discord"]
	if !disc.Enabled || len(disc.AllowFrom) != 1 || disc.TokenEnv != "" || disc.Settings.Kind != 0 {
		t.Fatalf("fresh overlay envelope = %+v", disc)
	}

	// No overlay entries: the config envelopes stand untouched.
	if _, err := settings.Save(settings.Path(dir), settings.Settings{}); err != nil {
		t.Fatal(err)
	}
	got = applySettingsOverlay(context.Background(), logger, cfg, []string{"telegram"})
	if len(got.Channels) != 1 || got.Channels["telegram"].Enabled {
		t.Fatalf("no-overlay config must pass through: %+v", got.Channels)
	}
}
