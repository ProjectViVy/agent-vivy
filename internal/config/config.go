// Package config loads and validates the typed configuration store
// (config.yaml) at startup. It is the boundary for the secret rule:
// non-secret values (model id, base URL, tool enablement) are queryable;
// secrets (provider keys) are never read into config and never persisted
// (PRD FR-10, D-010). Invalid config aborts startup.
//
// Secret boundary enforcement:
//   - decoding is strict (yaml KnownFields): an unknown field such as
//     `api_key:` is a hard error, so credentials cannot sneak into the
//     document shape;
//   - provider credentials are referenced ONLY by env_key (an environment
//     variable name matching ^[A-Z][A-Z0-9_]*$), resolved at request time.
package config

import (
	"bytes"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// envKeyPattern constrains env_key to an environment variable NAME.
// Anything else (a literal key value) fails validation.
var envKeyPattern = regexp.MustCompile(`^[A-Z][A-Z0-9_]*$`)

// defaultMaxToolTurns bounds tool-call turns per run when config omits
// runtime.max_tool_turns (MA-4): well above a healthy turn count, well
// below eino's 20 default, so a runaway loop fails fast and classified.
const defaultMaxToolTurns = 8

// maxExecuteTimeoutSeconds mirrors the runtime hard cap
// (runtime.hardMaxCommandTimeout = 10m): a configured execute ceiling above
// it would be silently clamped, so Validate rejects it up front.
const maxExecuteTimeoutSeconds = 600

// envUserHome optionally overrides the user data root. Without it the root
// resolves to the OS user home (diva-style), and without a home at all it
// falls back to the repo-relative dev layout.
const envUserHome = "VIVY_USER_HOME"

// userDataRoot returns the system-level default data root for a single-user
// install: VIVY_USER_HOME when set, else <os-user-home>/.vivy (diva-style,
// per-user and independent of the working directory), else "data" when the
// OS cannot name a home directory (CI/dev fallback). Explicit config paths
// (config.yaml, VIVY_CONFIG, docker overlays) always take precedence over
// this derived root.
func userDataRoot() string {
	if v := strings.TrimSpace(os.Getenv(envUserHome)); v != "" {
		return v
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		return filepath.Join(home, ".vivy")
	}
	return "data"
}

const (
	defaultMaxContextBytes    = 256 << 10
	defaultMaxHistoryMessages = 64
	defaultMaxRunEvents       = 512
	defaultMaxModelCalls      = 32
	defaultMaxRunToolCalls    = 64
	defaultMaxRunRetries      = 3
	// defaultCompactionMaxTokens = 0 means "derive from the model's context
	// window"; the runtime falls back to 128000 when the window is unknown.
	defaultCompactionMaxTokens  = 0
	defaultCompactionTriggerPct = 80
	defaultCompactionKeepRecent = 12
)

// Config is the typed, validated configuration store.
type Config struct {
	Server     Server     `yaml:"server"`
	Storage    Storage    `yaml:"storage"`
	Providers  Providers  `yaml:"providers"`
	Runtime    Runtime    `yaml:"runtime"`
	Tools      Tools      `yaml:"tools"`
	Governance Governance `yaml:"governance"`
}

type Server struct {
	// Addr is the local HTTP bootstrap/WebSocket control-plane listen address.
	Addr string `yaml:"addr"`
	// AllowedOrigins lists exact loopback browser origins that may use the
	// control plane when the UI is served separately. Empty keeps same-origin
	// access only.
	AllowedOrigins []string `yaml:"allowed_origins"`
}

type Storage struct {
	// Backend selects the storage implementation: "sqlite" (default) or
	// "postgres" (optional server Journal).
	Backend string `yaml:"backend"`
	// DataDir holds settings, evals, and other non-Journal files. Empty
	// means dirname(sqlite.path) for sqlite, or "data" for postgres.
	DataDir  string   `yaml:"data_dir"`
	SQLite   SQLite   `yaml:"sqlite"`
	Postgres Postgres `yaml:"postgres"`
}

type SQLite struct {
	Path string `yaml:"path"`
}

// Postgres names the environment variable that holds the server DSN.
// The DSN itself never appears in config (D-010).
type Postgres struct {
	DSNEnv string `yaml:"dsn_env"`
}

type Providers struct {
	// Active is the pre-baked bundle to activate: "openai" or "anthropic"
	// (D-018, D-023).
	Active string `yaml:"active"`
	// BundleDir is the directory holding the pre-baked bundle YAML files
	// (openai.yaml, anthropic.yaml; A2 fixtures).
	BundleDir string   `yaml:"bundle_dir"`
	OpenAI    Provider `yaml:"openai"`
	Anthropic Provider `yaml:"anthropic"`
}

// Provider holds non-secret provider settings. The credential itself is
// never part of config; EnvKey names the environment variable read at
// request time (D-010).
type Provider struct {
	EnvKey       string `yaml:"env_key"`
	DefaultModel string `yaml:"default_model"`
}

type Runtime struct {
	// Mock enables the deterministic mock provider for tests and offline
	// development (FR-3).
	Mock bool `yaml:"mock"`
	// MockScenario selects a deterministic tool-calling scenario when Mock is
	// enabled. It is test-only and intentionally has no production default.
	MockScenario string `yaml:"mock_scenario"`
	// StreamBuffer bounds buffered stream chunks (NFR: bounded).
	StreamBuffer int `yaml:"stream_buffer"`
	// MaxEventPayloadBytes bounds a single event payload (NFR: bounded).
	MaxEventPayloadBytes int `yaml:"max_event_payload_bytes"`
	// MaxToolTurns caps tool-call turns per run (MA-4); omitted keeps the
	// default.
	MaxToolTurns int `yaml:"max_tool_turns"`
	// MaxContextBytes bounds transient current-session context.
	MaxContextBytes int `yaml:"max_context_bytes"`
	// MaxHistoryMessages bounds retained user/assistant history rows.
	MaxHistoryMessages int `yaml:"max_history_messages"`
	// MaxToolResultBytes bounds one tool result entering the model context.
	MaxToolResultBytes int `yaml:"max_tool_result_bytes"`
	// MaxRunEvents bounds non-terminal durable events in one run tree.
	MaxRunEvents int `yaml:"max_run_events"`
	// MaxModelCalls bounds model generations across initial and resumed work.
	MaxModelCalls int `yaml:"max_model_calls"`
	// MaxRunToolCalls bounds tool calls across initial and resumed work.
	MaxRunToolCalls int `yaml:"max_run_tool_calls"`
	// MaxRunRetries bounds explicit retry reservations for one run tree.
	MaxRunRetries int `yaml:"max_run_retries"`
	// WorkspaceRoot contains one private sandbox directory per background run.
	WorkspaceRoot string `yaml:"workspace_root"`
	// SkillsRoot is a trusted, non-executable directory containing SKILL.md
	// packages. Skill content remains untrusted data at runtime.
	SkillsRoot string `yaml:"skills_root"`
	// HTTPAllowedHosts is the explicit host surface for the read-only HTTP tool.
	HTTPAllowedHosts []string `yaml:"http_allowed_hosts"`
	// HTTPMaxResponseBytes bounds one HTTP response entering the model context.
	HTTPMaxResponseBytes int `yaml:"http_max_response_bytes"`
	// MCPServers are explicitly configured Streamable HTTP JSON-RPC servers.
	MCPServers []MCPServer `yaml:"mcp_servers"`
	// ExecuteAllowedCommands is the executable allowlist for local process tools.
	ExecuteAllowedCommands []string `yaml:"execute_allowed_commands"`
	// ExecuteMaxTimeoutSeconds bounds one execute/commandline run. Requests
	// asking for more are clamped to it. Raise it (up to 600) for real work
	// such as go test or git clone; values above the runtime hard cap are
	// rejected so a typo cannot silently re-clamp the ceiling.
	ExecuteMaxTimeoutSeconds int `yaml:"execute_max_timeout_seconds"`
	// Compaction controls automatic context compression (Eino native
	// reduction + summarization middlewares).
	Compaction CompactionConfig `yaml:"compaction"`
	// Sandbox controls the file-effect policy boundary (D-021).
	Sandbox SandboxConfig `yaml:"sandbox"`
}

// CompactionConfig is the operator default for context compression. Zero
// values keep the safe defaults (except TriggerPercent/KeepRecent which have
// explicit bounds). The settings overlay (settings.yaml compaction) can
// override these per user.
type CompactionConfig struct {
	// Enabled turns the automatic in-run compression middlewares on.
	// Defaults to true.
	Enabled bool `yaml:"enabled"`
	// MaxTokens is the model context window cap used for the trigger
	// calculation and the UI meter. Zero means "use the provider's
	// ContextWindow metadata or 128000 when unknown".
	MaxTokens int `yaml:"max_tokens"`
	// TriggerPercent is the percentage of MaxTokens at which compression
	// triggers. 1..100; default 80.
	TriggerPercent int `yaml:"trigger_percent"`
	// KeepRecent is how many most-recent tool-call rounds the deterministic
	// reduction layer retains verbatim. >= 1; default 12.
	KeepRecent int `yaml:"keep_recent"`
}

// DefaultCompactionConfig returns the safe built-in compaction defaults.
func DefaultCompactionConfig() CompactionConfig {
	return CompactionConfig{
		Enabled:        true,
		MaxTokens:      defaultCompactionMaxTokens,
		TriggerPercent: defaultCompactionTriggerPct,
		KeepRecent:     defaultCompactionKeepRecent,
	}
}

type MCPServer struct {
	Name     string `yaml:"name"`
	Endpoint string `yaml:"endpoint"`
	AuthEnv  string `yaml:"auth_env"`
}

// SandboxConfig controls the file-effect policy boundary (D-021). It
// mirrors the DeepSeek Harness three-tier permission model.
type SandboxConfig struct {
	// DefaultMode is the initial sandbox mode for new sessions.
	DefaultMode string `yaml:"default_mode"`
	// WorkspaceRoot overrides the global workspace root for sandbox enforcement.
	WorkspaceRoot string `yaml:"workspace_root"`
	// Approval controls approval behavior under different policies.
	Approval SandboxApprovalConfig `yaml:"approval"`
	// Network defines allowed network destinations for HTTP requests.
	Network SandboxNetworkConfig `yaml:"network"`
}

// SandboxApprovalConfig controls approval behavior for effectful tools.
type SandboxApprovalConfig struct {
	// DefaultPolicy is the initial approval policy: ask, never, or auto.
	DefaultPolicy string `yaml:"default_policy"`
	// TimeoutSeconds bounds how long a pending approval stays valid before
	// being automatically expired and denied.
	TimeoutSeconds int `yaml:"timeout_seconds"`
	// AutoApproveTools lists tool names that are auto-approved under the
	// "auto" policy (typically readonly tools).
	AutoApproveTools []string `yaml:"auto_approve_tools"`
}

// SandboxNetworkConfig defines network access restrictions.
type SandboxNetworkConfig struct {
	// AllowedDomains is the whitelist of permitted domains for HTTP requests.
	AllowedDomains []string `yaml:"allowed_domains"`
	// DenyPrivateIPs blocks RFC1918 private IP ranges when true.
	DenyPrivateIPs bool `yaml:"deny_private_ips"`
}

type Tools struct {
	// Enabled lists the registered tool names. echo_info stays registered
	// for verification and tests but is off by default (it is a plumbing
	// probe, not a runtime capability).
	Enabled []string `yaml:"enabled"`
	// NetworkSearch is the preferred network_search provider for requests
	// that do not name one (bing, google, duckduckgo, searxng, wikipedia).
	// Empty means automatic: the first usable provider wins, degrading to
	// the keyless duckduckgo/wikipedia providers. Provider credentials are
	// environment-only (D-010) — this field never holds them.
	NetworkSearch NetworkSearchConfig `yaml:"network_search"`
	Approval      Approval            `yaml:"approval"`
}

// NetworkSearchConfig selects the preferred network_search provider.
type NetworkSearchConfig struct {
	// Provider is an allowlisted provider name, or empty for automatic
	// selection: the first usable provider wins, degrading to the keyless
	// duckduckgo/wikipedia providers when no API key is configured.
	Provider string `yaml:"provider"`
}

type Approval struct {
	// Expiration bounds how long a pending approval stays valid.
	Expiration time.Duration `yaml:"-"`
	// expirationRaw carries the YAML string ("5m"); parsed in Validate.
	expirationRaw string
}

// Governance contains the declarative execution profiles. Empty profile
// defaults are interpreted by runtime from the tool's readonly flag, which
// keeps the legacy configuration behavior stable.
type Governance struct {
	Profile        string                       `yaml:"profile"`
	Profiles       map[string]GovernanceProfile `yaml:"profiles"`
	HookTimeout    time.Duration                `yaml:"-"`
	HookTimeoutRaw string                       `yaml:"hook_timeout"`
}

type GovernanceProfile struct {
	Default string           `yaml:"default"`
	Rules   []GovernanceRule `yaml:"rules"`
}

type GovernanceRule struct {
	Tool     string `yaml:"tool"`
	Field    string `yaml:"field"`
	Equals   string `yaml:"equals"`
	Prefix   string `yaml:"prefix"`
	Decision string `yaml:"decision"`
	Reason   string `yaml:"reason"`
}

// toolsDoc mirrors the tools mapping with expiration kept as a raw
// string so that validation, not the decoder, owns duration parsing.
type toolsDoc struct {
	Enabled       []string `yaml:"enabled"`
	NetworkSearch struct {
		Provider string `yaml:"provider"`
	} `yaml:"network_search"`
	Approval struct {
		Expiration string `yaml:"expiration"`
	} `yaml:"approval"`
}

// UnmarshalYAML reads the tools mapping and stashes the approval
// expiration as an unparsed duration string.
func (t *Tools) UnmarshalYAML(node *yaml.Node) error {
	var doc toolsDoc
	if err := node.Decode(&doc); err != nil {
		return fmt.Errorf("tools: %w", err)
	}
	t.Enabled = doc.Enabled
	t.NetworkSearch.Provider = doc.NetworkSearch.Provider
	t.Approval.expirationRaw = doc.Approval.Expiration
	return nil
}

// Default returns the built-in configuration used when no config file is
// present. It mirrors config.example.yaml. All data paths resolve under the
// user data root (diva-style user home), so a fresh single-user install owns
// its workspace, settings, skills, and Journal regardless of the working
// directory. An explicit config.yaml / VIVY_CONFIG always overrides these.
func Default() Config {
	root := userDataRoot()
	return Config{
		Server:  Server{Addr: "127.0.0.1:8787"},
		Storage: Storage{Backend: "sqlite", SQLite: SQLite{Path: filepath.Join(root, "vivy.db")}},
		Providers: Providers{
			Active:    "openai",
			BundleDir: "fixtures/provider",
			OpenAI:    Provider{EnvKey: "OPENAI_API_KEY", DefaultModel: "gpt-4o-mini"},
			Anthropic: Provider{EnvKey: "ANTHROPIC_API_KEY", DefaultModel: "claude-sonnet-4-5"},
		},
		Runtime: Runtime{
			Mock:                     false,
			MockScenario:             "",
			StreamBuffer:             256,
			MaxEventPayloadBytes:     65536,
			MaxToolTurns:             defaultMaxToolTurns,
			MaxContextBytes:          defaultMaxContextBytes,
			MaxHistoryMessages:       defaultMaxHistoryMessages,
			MaxToolResultBytes:       32 << 10,
			MaxRunEvents:             defaultMaxRunEvents,
			MaxModelCalls:            defaultMaxModelCalls,
			MaxRunToolCalls:          defaultMaxRunToolCalls,
			MaxRunRetries:            defaultMaxRunRetries,
			WorkspaceRoot:            filepath.Join(root, "workspace"),
			SkillsRoot:               filepath.Join(root, "skills"),
			HTTPAllowedHosts:         []string{"localhost", "127.0.0.1", "::1"},
			HTTPMaxResponseBytes:     1 << 20,
			ExecuteAllowedCommands:   []string{"go", "git", "rg"},
			ExecuteMaxTimeoutSeconds: 30,
			Compaction:               DefaultCompactionConfig(),
			Sandbox: SandboxConfig{
				DefaultMode:   "workspace_write",
				WorkspaceRoot: "",
				Approval: SandboxApprovalConfig{
					DefaultPolicy:    "ask",
					TimeoutSeconds:   300, // 5 minutes
					AutoApproveTools: []string{"list_dir", "read_file", "search_files", "list_notes", "read_note", "skills_list", "skill_view", "network_search"},
				},
				Network: SandboxNetworkConfig{
					AllowedDomains: []string{},
					DenyPrivateIPs: true,
				},
			},
		},
		Tools: Tools{
			Enabled:       []string{"write_note", "list_notes", "read_note", "ask_user", "read_file", "search_files", "write_file", "patch", "skills_list", "skill_view", "skill_manage", "task_create", "task_get", "task_update", "task_list", "network_search", "http_request", "mcp_list_tools", "mcp_call", "sequential_thinking", "execute", "commandline", "tool_search"},
			NetworkSearch: NetworkSearchConfig{Provider: ""},
			Approval:      Approval{Expiration: 5 * time.Minute, expirationRaw: "5m"},
		},
		Governance: Governance{
			Profile:        "default",
			HookTimeout:    time.Second,
			HookTimeoutRaw: "1s",
			Profiles: map[string]GovernanceProfile{
				"default":   {},
				"plan":      {Default: "deny"},
				"read_only": {Default: "deny"},
				"full_auto": {Default: "allow"},
			},
		},
	}
}

// Load reads the YAML document at path, decodes it strictly on top of
// Default(), and validates the result. Unknown fields are hard errors.
func Load(path string) (Config, error) {
	cfg := Default()

	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read config: %w", err)
	}

	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&cfg); err != nil {
		return Config{}, fmt.Errorf("parse config %s: %w", path, err)
	}

	if err := cfg.Validate(); err != nil {
		return Config{}, fmt.Errorf("invalid config %s: %w", path, err)
	}
	return cfg, nil
}

// Validate checks every field the runtime depends on and the secret
// boundary. Any failure aborts startup. It is pointer-received because it
// normalizes Approval.Expiration from its raw YAML string.
func (c *Config) Validate() error {
	host, port, err := net.SplitHostPort(c.Server.Addr)
	if err != nil || host == "" || port == "" {
		return fmt.Errorf("server.addr %q is not a valid host:port", c.Server.Addr)
	}
	if err := validateServerOrigins(host, c.Server.AllowedOrigins); err != nil {
		return err
	}

	switch c.Storage.Backend {
	case "sqlite":
		if c.Storage.SQLite.Path == "" {
			return errors.New("storage.sqlite.path must not be empty")
		}
	case "postgres":
		if !envKeyPattern.MatchString(c.Storage.Postgres.DSNEnv) {
			return fmt.Errorf("storage.postgres.dsn_env %q is not an environment variable name; "+
				"the DSN must never appear in config (D-010)", c.Storage.Postgres.DSNEnv)
		}
	default:
		return fmt.Errorf("storage.backend %q unsupported; V0 supports sqlite and postgres", c.Storage.Backend)
	}

	switch c.Providers.Active {
	case "openai", "anthropic":
	default:
		return fmt.Errorf("providers.active %q unsupported; V0 ships openai and anthropic only", c.Providers.Active)
	}
	if c.Providers.BundleDir == "" {
		return errors.New("providers.bundle_dir must not be empty")
	}
	for name, p := range map[string]Provider{
		"openai": c.Providers.OpenAI, "anthropic": c.Providers.Anthropic,
	} {
		if !envKeyPattern.MatchString(p.EnvKey) {
			return fmt.Errorf("providers.%s.env_key %q is not an environment variable name; "+
				"secrets must never appear in config (D-010)", name, p.EnvKey)
		}
		if p.DefaultModel == "" {
			return fmt.Errorf("providers.%s.default_model must not be empty", name)
		}
	}

	if c.Runtime.StreamBuffer <= 0 {
		return errors.New("runtime.stream_buffer must be positive")
	}
	if c.Runtime.MockScenario != "" {
		if !c.Runtime.Mock {
			return errors.New("runtime.mock_scenario requires runtime.mock=true")
		}
		switch c.Runtime.MockScenario {
		case "hitl", "approval", "question", "timeout", "stale":
		default:
			return fmt.Errorf("runtime.mock_scenario %q is unsupported", c.Runtime.MockScenario)
		}
	}
	if c.Runtime.MaxEventPayloadBytes <= 0 {
		return errors.New("runtime.max_event_payload_bytes must be positive")
	}
	if c.Runtime.MaxToolTurns < 0 {
		return errors.New("runtime.max_tool_turns must not be negative")
	}
	if c.Runtime.MaxContextBytes <= 0 {
		return errors.New("runtime.max_context_bytes must be positive")
	}
	if c.Runtime.MaxHistoryMessages <= 0 {
		return errors.New("runtime.max_history_messages must be positive")
	}
	if c.Runtime.MaxToolResultBytes <= 0 {
		return errors.New("runtime.max_tool_result_bytes must be positive")
	}
	if c.Runtime.MaxRunEvents <= 0 {
		return errors.New("runtime.max_run_events must be positive")
	}
	if c.Runtime.MaxModelCalls <= 0 {
		return errors.New("runtime.max_model_calls must be positive")
	}
	if c.Runtime.MaxRunToolCalls <= 0 {
		return errors.New("runtime.max_run_tool_calls must be positive")
	}
	if c.Runtime.MaxRunRetries < 0 {
		return errors.New("runtime.max_run_retries must not be negative")
	}
	if c.Runtime.WorkspaceRoot == "" {
		return errors.New("runtime.workspace_root must not be empty")
	}
	if c.Runtime.SkillsRoot == "" {
		return errors.New("runtime.skills_root must not be empty")
	}
	if c.Runtime.HTTPMaxResponseBytes <= 0 {
		return errors.New("runtime.http_max_response_bytes must be positive")
	}
	if c.Runtime.ExecuteMaxTimeoutSeconds <= 0 || c.Runtime.ExecuteMaxTimeoutSeconds > maxExecuteTimeoutSeconds {
		return fmt.Errorf("runtime.execute_max_timeout_seconds must be between 1 and %d seconds", maxExecuteTimeoutSeconds)
	}
	if c.Runtime.Compaction.MaxTokens < 0 {
		return errors.New("runtime.compaction.max_tokens must not be negative")
	}
	if c.Runtime.Compaction.TriggerPercent < 1 || c.Runtime.Compaction.TriggerPercent > 100 {
		return errors.New("runtime.compaction.trigger_percent must be between 1 and 100")
	}
	if c.Runtime.Compaction.KeepRecent < 1 {
		return errors.New("runtime.compaction.keep_recent must be at least 1")
	}
	for i, server := range c.Runtime.MCPServers {
		if server.Name == "" || server.Endpoint == "" {
			return fmt.Errorf("runtime.mcp_servers[%d] requires name and endpoint", i)
		}
		parsed, err := url.Parse(server.Endpoint)
		if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
			return fmt.Errorf("runtime.mcp_servers[%d].endpoint must be an absolute HTTP(S) URL", i)
		}
		if server.AuthEnv != "" && !envKeyPattern.MatchString(server.AuthEnv) {
			return fmt.Errorf("runtime.mcp_servers[%d].auth_env must be an environment variable name", i)
		}
	}

	if len(c.Tools.Enabled) == 0 {
		return errors.New("tools.enabled must list at least one tool")
	}
	if provider := c.Tools.NetworkSearch.Provider; provider != "" {
		switch provider {
		case "bing", "google", "duckduckgo", "searxng", "wikipedia":
		default:
			return fmt.Errorf("tools.network_search.provider %q unsupported; want bing, google, duckduckgo, searxng, or wikipedia", provider)
		}
	}
	if c.Tools.Approval.expirationRaw != "" {
		d, err := time.ParseDuration(c.Tools.Approval.expirationRaw)
		if err != nil || d <= 0 {
			return fmt.Errorf("tools.approval.expiration %q must be a positive duration", c.Tools.Approval.expirationRaw)
		}
		c.Tools.Approval.Expiration = d
	}
	if c.Tools.Approval.Expiration <= 0 {
		return errors.New("tools.approval.expiration must be positive")
	}

	if c.Governance.Profile == "" {
		c.Governance.Profile = "default"
	}
	if !validGovernanceProfile(c.Governance.Profile) {
		return fmt.Errorf("governance.profile %q is unsupported", c.Governance.Profile)
	}
	if c.Governance.HookTimeoutRaw != "" {
		d, err := time.ParseDuration(c.Governance.HookTimeoutRaw)
		if err != nil || d <= 0 {
			return fmt.Errorf("governance.hook_timeout %q must be a positive duration", c.Governance.HookTimeoutRaw)
		}
		c.Governance.HookTimeout = d
	}
	if c.Governance.HookTimeout <= 0 {
		return errors.New("governance.hook_timeout must be positive")
	}
	for name, profile := range c.Governance.Profiles {
		if !validGovernanceProfile(name) {
			return fmt.Errorf("governance.profiles.%s is unsupported", name)
		}
		if profile.Default != "" && !validGovernanceDecision(profile.Default) {
			return fmt.Errorf("governance.profiles.%s.default %q is unsupported", name, profile.Default)
		}
		for i, rule := range profile.Rules {
			if strings.TrimSpace(rule.Tool) == "" {
				return fmt.Errorf("governance.profiles.%s.rules[%d].tool must not be empty", name, i)
			}
			if rule.Field != "" && !validGovernanceField(rule.Field) {
				return fmt.Errorf("governance.profiles.%s.rules[%d].field %q is unsupported", name, i, rule.Field)
			}
			if rule.Equals != "" && rule.Prefix != "" {
				return fmt.Errorf("governance.profiles.%s.rules[%d] cannot set both equals and prefix", name, i)
			}
			if !validGovernanceDecision(rule.Decision) {
				return fmt.Errorf("governance.profiles.%s.rules[%d].decision %q is unsupported", name, i, rule.Decision)
			}
		}
	}

	// Sandbox configuration validation (D-021).
	if c.Runtime.Sandbox.DefaultMode != "" {
		switch c.Runtime.Sandbox.DefaultMode {
		case "read_only", "workspace_write", "danger_full_access":
		default:
			return fmt.Errorf("runtime.sandbox.default_mode %q is unsupported; use read_only, workspace_write, or danger_full_access", c.Runtime.Sandbox.DefaultMode)
		}
	}
	if c.Runtime.Sandbox.WorkspaceRoot != "" && c.Runtime.Sandbox.WorkspaceRoot != c.Runtime.WorkspaceRoot {
		// Sandbox workspace root can override the global one, but must still be valid.
		if err := validatePath(c.Runtime.Sandbox.WorkspaceRoot); err != nil {
			return fmt.Errorf("runtime.sandbox.workspace_root: %w", err)
		}
	}
	if c.Runtime.Sandbox.Approval.DefaultPolicy != "" {
		switch c.Runtime.Sandbox.Approval.DefaultPolicy {
		case "ask", "never", "auto":
		default:
			return fmt.Errorf("runtime.sandbox.approval.default_policy %q is unsupported; use ask, never, or auto", c.Runtime.Sandbox.Approval.DefaultPolicy)
		}
	}
	if c.Runtime.Sandbox.Approval.TimeoutSeconds < 0 {
		return errors.New("runtime.sandbox.approval.timeout_seconds must not be negative")
	}
	if c.Runtime.Sandbox.Network.DenyPrivateIPs {
		// Validation only; actual enforcement happens at request time.
	}

	return nil
}

// DataDirectory is the process working directory for settings, evals, and
// sidecar files. The Journal itself may live in SQLite under this tree or
// in Postgres; this path is never a DSN. The fallback resolves under the
// user data root so a config without explicit paths still lands in the
// per-user layout (settings.yaml beside the Journal and workspace).
func (c Config) DataDirectory() string {
	if dir := strings.TrimSpace(c.Storage.DataDir); dir != "" {
		return dir
	}
	if c.Storage.Backend == "postgres" {
		return userDataRoot()
	}
	dir := filepath.Dir(c.Storage.SQLite.Path)
	if dir != "" && dir != "." {
		return dir
	}
	return userDataRoot()
}

func validGovernanceProfile(value string) bool {
	switch value {
	case "default", "plan", "read_only", "full_auto":
		return true
	default:
		return false
	}
}

func validGovernanceDecision(value string) bool {
	switch value {
	case "allow", "prompt", "deny":
		return true
	default:
		return false
	}
}

func validGovernanceField(value string) bool {
	switch value {
	case "command", "cmd", "path", "filepath", "file_path":
		return true
	default:
		return false
	}
}

// validatePath checks that a path is non-empty and does not contain
// traversal sequences. It is used for sandbox configuration validation.
func validatePath(path string) error {
	if path == "" {
		return errors.New("path must not be empty")
	}
	if strings.Contains(path, "..") {
		return errors.New("path must not contain ..")
	}
	return nil
}

func validateServerOrigins(serverHost string, origins []string) error {
	if len(origins) == 0 {
		return nil
	}
	if !isLoopbackHost(serverHost) {
		return fmt.Errorf("server.addr host %q must be loopback when server.allowed_origins is configured", serverHost)
	}
	seen := make(map[string]struct{}, len(origins))
	for i, raw := range origins {
		origin, err := normalizeOrigin(raw)
		if err != nil {
			return fmt.Errorf("server.allowed_origins[%d] %q: %w", i, raw, err)
		}
		if _, ok := seen[origin]; ok {
			return fmt.Errorf("server.allowed_origins[%d] %q is duplicated", i, raw)
		}
		seen[origin] = struct{}{}
		parsed, _ := url.Parse(origin)
		if !isLoopbackHost(parsed.Hostname()) {
			return fmt.Errorf("server.allowed_origins[%d] %q must use a loopback host", i, raw)
		}
	}
	return nil
}

func normalizeOrigin(raw string) (string, error) {
	value := strings.TrimSpace(raw)
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", errors.New("must be an absolute HTTP(S) origin")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", errors.New("must use http or https")
	}
	if parsed.User != nil || parsed.Path != "" || parsed.RawQuery != "" || parsed.Fragment != "" || parsed.Opaque != "" {
		return "", errors.New("must not include credentials, path, query, or fragment")
	}
	if parsed.Hostname() == "" {
		return "", errors.New("must include a host")
	}
	if port := parsed.Port(); port != "" {
		if n, err := strconv.Atoi(port); err != nil || n < 1 || n > 65535 {
			return "", errors.New("contains an invalid port")
		}
	}
	return strings.ToLower(parsed.Scheme) + "://" + strings.ToLower(parsed.Host), nil
}

func isLoopbackHost(host string) bool {
	host = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(host)), ".")
	if host == "localhost" || host == "127.0.0.1" || host == "::1" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
