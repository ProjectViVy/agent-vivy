package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func writeConfig(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

const validDoc = `
server:
  addr: "127.0.0.1:9090"
  allowed_origins:
    - "http://127.0.0.1:3015"
storage:
  backend: sqlite
  sqlite:
    path: "tmp/vivy.db"
providers:
  active: anthropic
  bundle_dir: fixtures/provider
  openai:
    env_key: OPENAI_API_KEY
    default_model: gpt-4o-mini
  anthropic:
    env_key: ANTHROPIC_API_KEY
    default_model: claude-sonnet-4-5
runtime:
  mock: true
  mock_scenario: hitl
  stream_buffer: 16
  max_event_payload_bytes: 1024
  execute_max_timeout_seconds: 210
tools:
  enabled:
    - echo_info
    - write_note
  approval:
    expiration: 2m
`

func TestLoadValid(t *testing.T) {
	cfg, err := Load(writeConfig(t, validDoc))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Server.Addr != "127.0.0.1:9090" {
		t.Errorf("addr = %q", cfg.Server.Addr)
	}
	if len(cfg.Server.AllowedOrigins) != 1 || cfg.Server.AllowedOrigins[0] != "http://127.0.0.1:3015" {
		t.Errorf("allowed origins = %#v", cfg.Server.AllowedOrigins)
	}
	if cfg.Providers.Active != "anthropic" {
		t.Errorf("active = %q", cfg.Providers.Active)
	}
	if !cfg.Runtime.Mock || cfg.Runtime.StreamBuffer != 16 || cfg.Runtime.MaxEventPayloadBytes != 1024 ||
		cfg.Runtime.MockScenario != "hitl" ||
		cfg.Runtime.MaxContextBytes != 256<<10 || cfg.Runtime.MaxHistoryMessages != 64 ||
		cfg.Runtime.MaxToolResultBytes != 32<<10 || cfg.Runtime.MaxRunEvents != 512 ||
		cfg.Runtime.MaxModelCalls != 32 || cfg.Runtime.MaxRunToolCalls != 64 ||
		cfg.Runtime.MaxRunRetries != 3 || cfg.Runtime.WorkspaceRoot != "data/workspaces" ||
		cfg.Runtime.ExecuteMaxTimeoutSeconds != 210 {
		t.Errorf("runtime = %+v", cfg.Runtime)
	}
	if cfg.Tools.Approval.Expiration != 2*time.Minute {
		t.Errorf("expiration = %v, want 2m", cfg.Tools.Approval.Expiration)
	}
}

func TestLoadMissingFile(t *testing.T) {
	if _, err := Load(filepath.Join(t.TempDir(), "nope.yaml")); err == nil {
		t.Fatal("want error for missing file")
	}
}

func TestDefaultIsValid(t *testing.T) {
	cfg := Default()
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Default(): %v", err)
	}
	if cfg.Runtime.Sandbox.WorkspaceRoot != "" {
		t.Fatalf("default sandbox workspace root = %q, want empty so overlays inherit runtime.workspace_root", cfg.Runtime.Sandbox.WorkspaceRoot)
	}
	if cfg.Runtime.ExecuteMaxTimeoutSeconds != 30 {
		t.Fatalf("default execute_max_timeout_seconds = %d, want 30", cfg.Runtime.ExecuteMaxTimeoutSeconds)
	}
}

// echo_info stays registered for verification and tests but must not be
// enabled by default.
func TestDefaultEnabledOmitsEchoInfo(t *testing.T) {
	for _, name := range Default().Tools.Enabled {
		if name == "echo_info" {
			t.Fatal("echo_info must not be enabled by default; it is a verification tool")
		}
	}
}

// The network_search provider preference parses, defaults to auto, and
// rejects unknown provider names.
func TestNetworkSearchProviderConfig(t *testing.T) {
	doc := strings.Replace(validDoc,
		"  approval:\n    expiration: 2m",
		"  network_search:\n    provider: searxng\n  approval:\n    expiration: 2m", 1)
	cfg, err := Load(writeConfig(t, doc))
	if err != nil {
		t.Fatalf("network_search provider: %v", err)
	}
	if cfg.Tools.NetworkSearch.Provider != "searxng" {
		t.Fatalf("provider = %q", cfg.Tools.NetworkSearch.Provider)
	}
	if Default().Tools.NetworkSearch.Provider != "" {
		t.Fatal("default network_search provider must be empty (automatic)")
	}

	bad := strings.Replace(validDoc,
		"  approval:\n    expiration: 2m",
		"  network_search:\n    provider: alta vista\n  approval:\n    expiration: 2m", 1)
	if _, err := Load(writeConfig(t, bad)); err == nil {
		t.Fatal("want error for unsupported network_search provider")
	}
}

// The secret boundary: a credential field that is not part of the shape
// must be rejected by strict decoding, and a literal secret in env_key
// must be rejected by validation (D-010).
func TestSecretInjectionRejected(t *testing.T) {
	unknownField := strings.Replace(validDoc,
		"    env_key: OPENAI_API_KEY",
		"    env_key: OPENAI_API_KEY\n    api_key: sk-not-a-secret-boundary", 1)
	if _, err := Load(writeConfig(t, unknownField)); err == nil {
		t.Fatal("want error for unknown field api_key")
	}

	literalKey := strings.Replace(validDoc,
		"    env_key: OPENAI_API_KEY",
		"    env_key: sk-live-abc123", 1)
	if _, err := Load(writeConfig(t, literalKey)); err == nil {
		t.Fatal("want error for literal secret in env_key")
	}
}

func TestInvalidValuesRejected(t *testing.T) {
	cases := map[string]string{
		"bad addr": strings.Replace(validDoc, `"127.0.0.1:9090"`, `"not-an-addr"`, 1),
		"bad storage backend": strings.Replace(validDoc,
			"backend: sqlite", "backend: mariadb", 1),
		"bad active provider": strings.Replace(validDoc,
			"active: anthropic", "active: deepseek", 1),
		"bad expiration": strings.Replace(validDoc,
			"expiration: 2m", "expiration: soon", 1),
		"empty tools": strings.Replace(validDoc,
			"  enabled:\n    - echo_info\n    - write_note", "  enabled: []", 1),
		"mock scenario without mock": strings.Replace(validDoc,
			"  mock: true", "  mock: false", 1),
		"unknown mock scenario": strings.Replace(validDoc,
			"  mock_scenario: hitl", "  mock_scenario: unknown", 1),
		"non-loopback origin": strings.Replace(validDoc,
			"http://127.0.0.1:3015", "http://example.test:3015", 1),
		"origin path": strings.Replace(validDoc,
			"http://127.0.0.1:3015", "http://127.0.0.1:3015/app", 1),
		"non-loopback listen with origin": strings.Replace(validDoc,
			`"127.0.0.1:9090"`, `"0.0.0.0:9090"`, 1),
"unsupported network_search provider": strings.Replace(validDoc,
			"  approval:\n    expiration: 2m", "  approval:\n    expiration: 2m\n  network_search:\n    provider: yandex", 1),
		"zero execute timeout": strings.Replace(validDoc,
			"execute_max_timeout_seconds: 210", "execute_max_timeout_seconds: 0", 1),
		"negative execute timeout": strings.Replace(validDoc,
			"execute_max_timeout_seconds: 210", "execute_max_timeout_seconds: -5", 1),
		"execute timeout above hard cap": strings.Replace(validDoc,
			"execute_max_timeout_seconds: 210", "execute_max_timeout_seconds: 601", 1),
	}
	for name, doc := range cases {
		if _, err := Load(writeConfig(t, doc)); err == nil {
			t.Errorf("%s: want error, got nil", name)
		}
	}
}

// TestLoadNetworkSearchProvider pins the network_search preference parsing:
// a valid provider is loaded, and a bare (empty) provider means automatic.
func TestLoadNetworkSearchProvider(t *testing.T) {
	doc := strings.Replace(validDoc,
		"  approval:\n    expiration: 2m", "  approval:\n    expiration: 2m\n  network_search:\n    provider: duckduckgo", 1)
	cfg, err := Load(writeConfig(t, doc))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Tools.NetworkSearch.Provider != "duckduckgo" {
		t.Fatalf("provider = %q, want duckduckgo", cfg.Tools.NetworkSearch.Provider)
	}
}

// TestExecuteTimeoutOmittedKeepsDefault pins the config default: a config
// without execute_max_timeout_seconds falls back to the built-in 30s.
func TestExecuteTimeoutOmittedKeepsDefault(t *testing.T) {
	doc := strings.Replace(validDoc, "  execute_max_timeout_seconds: 210\n", "", 1)
	cfg, err := Load(writeConfig(t, doc))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Runtime.ExecuteMaxTimeoutSeconds != 30 {
		t.Fatalf("execute_max_timeout_seconds = %d, want default 30", cfg.Runtime.ExecuteMaxTimeoutSeconds)
	}
}

func TestDefaultAllowsSameOriginOnly(t *testing.T) {
	cfg := Default()
	if len(cfg.Server.AllowedOrigins) != 0 {
		t.Fatalf("default allowed origins = %#v, want empty", cfg.Server.AllowedOrigins)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestZeroAddrAllowedWhenOriginsEmpty(t *testing.T) {
	cfg := Default()
	cfg.Server.Addr = "0.0.0.0:8787"
	if err := cfg.Validate(); err != nil {
		t.Fatalf("0.0.0.0 with empty origins: %v", err)
	}
	cfg.Server.AllowedOrigins = []string{"http://127.0.0.1:3015"}
	if err := cfg.Validate(); err == nil {
		t.Fatal("want error for 0.0.0.0 with allowed_origins")
	}
}

func TestDevOverlayEnablesMock(t *testing.T) {
	cfg, err := Load(filepath.Join("..", "..", "config.dev.yaml"))
	if err != nil {
		t.Fatalf("dev overlay: %v", err)
	}
	if !cfg.Runtime.Mock {
		t.Fatal("dev overlay must enable runtime.mock")
	}
	if cfg.Server.Addr != "127.0.0.1:8787" {
		t.Fatalf("addr = %q", cfg.Server.Addr)
	}
}

func TestDockerOverlayLoads(t *testing.T) {
	cfg, err := Load(filepath.Join("..", "..", "docker", "config.yaml"))
	if err != nil {
		t.Fatalf("docker overlay: %v", err)
	}
	if cfg.Server.Addr != "0.0.0.0:8787" {
		t.Fatalf("addr = %q", cfg.Server.Addr)
	}
	if len(cfg.Server.AllowedOrigins) != 0 {
		t.Fatalf("allowed origins = %#v, want empty", cfg.Server.AllowedOrigins)
	}
	if cfg.Storage.Backend != "sqlite" || cfg.Storage.SQLite.Path != "/data/vivy.db" {
		t.Fatalf("storage = %+v", cfg.Storage)
	}
	if cfg.Runtime.WorkspaceRoot != "/data/workspaces" || cfg.Runtime.SkillsRoot != "/data/skills" {
		t.Fatalf("runtime paths = %+v", cfg.Runtime)
	}
	sandboxRoot := cfg.Runtime.Sandbox.WorkspaceRoot
	if sandboxRoot == "" {
		sandboxRoot = cfg.Runtime.WorkspaceRoot
	}
	if sandboxRoot != "/data/workspaces" {
		t.Fatalf("effective sandbox root = %q", sandboxRoot)
	}
}

func TestDockerPostgresOverlayLoads(t *testing.T) {
	cfg, err := Load(filepath.Join("..", "..", "docker", "config.postgres.yaml"))
	if err != nil {
		t.Fatalf("postgres overlay: %v", err)
	}
	if cfg.Storage.Backend != "postgres" || cfg.Storage.Postgres.DSNEnv != "VIVY_POSTGRES_DSN" {
		t.Fatalf("storage = %+v", cfg.Storage)
	}
	if cfg.DataDirectory() != "/data" {
		t.Fatalf("data dir = %q", cfg.DataDirectory())
	}
	if cfg.Storage.Postgres.DSNEnv == "postgres://" || strings.Contains(cfg.Storage.Postgres.DSNEnv, "://") {
		t.Fatal("dsn_env must be an env var name, not a DSN")
	}
}

func TestDockerPackagingContracts(t *testing.T) {
	root := filepath.Join("..", "..")
	compose, err := os.ReadFile(filepath.Join(root, "docker-compose.yml"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(compose)
	if !strings.Contains(text, `"127.0.0.1:8787:8787"`) {
		t.Fatal("compose must publish 8787 on host loopback only")
	}
	if strings.Contains(text, `- "8787:8787"`) || strings.Contains(text, `- 8787:8787`) ||
		strings.Contains(text, "0.0.0.0:8787:8787") {
		t.Fatal("compose must not publish 8787 on all host interfaces")
	}
	for _, forbidden := range []string{"image: postgres", "image: redis", "image: mariadb", "image: mysql"} {
		if strings.Contains(strings.ToLower(text), forbidden) {
			t.Fatalf("compose must not add %s in this cut", forbidden)
		}
	}

	dockerfile, err := os.ReadFile(filepath.Join(root, "Dockerfile"))
	if err != nil {
		t.Fatal(err)
	}
	df := string(dockerfile)
	if strings.Contains(df, "vivy_headless") {
		t.Fatal("image must embed the UI; do not build vivy_headless")
	}
	if !strings.Contains(df, "VIVY_ADDR=0.0.0.0:8787") {
		t.Fatal("image must listen on 0.0.0.0:8787")
	}
	if !strings.Contains(df, "fixtures/provider") {
		t.Fatal("image must include provider fixtures")
	}
}

func TestPostgresConfigRequiresDSNEnv(t *testing.T) {
	doc := strings.Replace(validDoc, "backend: sqlite", "backend: postgres", 1)
	if _, err := Load(writeConfig(t, doc)); err == nil {
		t.Fatal("want error for postgres without dsn_env")
	}
	doc = strings.Replace(validDoc,
		"backend: sqlite\n  sqlite:\n    path: \"tmp/vivy.db\"",
		"backend: postgres\n  postgres:\n    dsn_env: VIVY_POSTGRES_DSN", 1)
	cfg, err := Load(writeConfig(t, doc))
	if err != nil {
		t.Fatalf("postgres overlay: %v", err)
	}
	if cfg.Storage.Backend != "postgres" || cfg.Storage.Postgres.DSNEnv != "VIVY_POSTGRES_DSN" {
		t.Fatalf("storage = %+v", cfg.Storage)
	}
	if cfg.DataDirectory() != "data" {
		t.Fatalf("data dir = %q", cfg.DataDirectory())
	}
}

func TestGovernanceProfilesLoadAndValidate(t *testing.T) {
	doc := validDoc + `
governance:
  profile: default
  hook_timeout: 250ms
  profiles:
    default:
      rules:
        - tool: write_note
          field: path
          prefix: "data/"
          decision: allow
          reason: "private notes"
    full_auto:
      default: allow
`
	cfg, err := Load(writeConfig(t, doc))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Governance.HookTimeout != 250*time.Millisecond {
		t.Fatalf("hook timeout = %v", cfg.Governance.HookTimeout)
	}
	rules := cfg.Governance.Profiles["default"].Rules
	if len(rules) != 1 || rules[0].Decision != "allow" || rules[0].Prefix != "data/" {
		t.Fatalf("governance rules = %+v", rules)
	}
}

func TestGovernanceInvalidRuleRejected(t *testing.T) {
	doc := validDoc + `
governance:
  profiles:
    default:
      rules:
        - tool: write_note
          field: network
          decision: allow
`
	if _, err := Load(writeConfig(t, doc)); err == nil {
		t.Fatal("want invalid governance field error")
	}
}
