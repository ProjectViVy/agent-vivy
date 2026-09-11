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
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"agent-vivy/internal/commandpolicy"

	"gopkg.in/yaml.v3"
)

// envKeyPattern constrains env_key to an environment variable NAME.
// Anything else (a literal key value) fails validation.
var envKeyPattern = regexp.MustCompile(`^[A-Z][A-Z0-9_]*$`)

// ValidEnvKey reports whether name is a well-formed environment variable
// name usable as an env_key declaration (config fields and the opaque
// channel settings `*_env` walk share this single pattern, CH-C6-N2).
func ValidEnvKey(name string) bool { return envKeyPattern.MatchString(name) }

// channelNamePattern constrains a channels map key to a plugin-name slug:
// the Host (C3) matches it against the channel plugins compiled into the
// running generation.
var channelNamePattern = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// mcpNamespacePattern is intentionally stricter than generic config slugs.
// The literal server name is embedded in model-visible MCP Tool IDs; unsafe
// values must be rejected, never trimmed or rewritten into another identity.
var mcpNamespacePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,127}$`)

// ValidateMCPServerEndpoint accepts only a credential-free absolute HTTP(S)
// URL. MCP endpoints are projected into browser-facing configuration, so
// userinfo and credential-shaped query keys are rejected at the persistence
// boundary instead of being redacted after a secret has already crossed it.
func ValidateMCPServerEndpoint(endpoint string) error {
	parsed, err := url.Parse(strings.TrimSpace(endpoint))
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Hostname() == "" {
		return errors.New("must be an absolute HTTP(S) URL")
	}
	if parsed.User != nil {
		return errors.New("must not include URL credentials")
	}
	query, err := url.ParseQuery(parsed.RawQuery)
	if err != nil {
		return errors.New("must not include credential-like query parameters")
	}
	for key := range query {
		if mcpCredentialLikeQueryKey(key) {
			return errors.New("must not include credential-like query parameters")
		}
	}
	return nil
}

func mcpCredentialLikeQueryKey(key string) bool {
	normalized := strings.ToLower(strings.NewReplacer("-", "_", ".", "_").Replace(strings.TrimSpace(key)))
	if normalized == "" {
		return false
	}
	for _, part := range strings.Split(normalized, "_") {
		switch part {
		case "token", "secret", "password", "passwd", "credential", "authorization", "apikey", "key":
			return true
		}
	}
	return normalized == "access_token" || normalized == "api_key"
}

// defaultMaxToolTurns bounds tool-call turns per run when config omits
// runtime.max_tool_turns (MA-4): well above a healthy turn count, well
// below eino's 20 default, so a runaway loop fails fast and classified.
const defaultMaxToolTurns = 8

// maxExecuteTimeoutSeconds mirrors the runtime hard cap
// (runtime.hardMaxCommandTimeout = 10m): a configured execute ceiling above
// it would be silently clamped, so Validate rejects it up front.
const maxExecuteTimeoutSeconds = 600

const (
	maxMCPArgs       = 128
	maxMCPArgBytes   = 4096
	maxMCPArgsBytes  = 64 << 10
	maxMCPEnvEntries = 64
)

// DefaultSkillsMarketplaceURL is the public skills.sh directory backing the
// skill marketplace (runtime.skills_marketplace_url default).
const DefaultSkillsMarketplaceURL = "https://skills.sh"

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
	Logging    Logging    `yaml:"logging"`
	TUI        TUI        `yaml:"tui"`
	Channels   Channels   `yaml:"channels"`
}

// TUI configures presentation-only behavior for terminal faces. It does not
// change tool execution, Journal persistence, or model context.
type TUI struct {
	// Debug shows complete tool results. The default keeps tool cards bounded.
	Debug bool `yaml:"debug"`
}

// Logging configures the kernel's slog output (see internal/logging and
// docs/architecture/LOGGING.md). It never carries secrets: the sink is a
// local directory, and D-010 keeps provider keys out of every payload.
type Logging struct {
	// Level is the minimum severity: debug, info (default), warn, error.
	Level string `yaml:"level"`
	// Format selects the line encoding: json (default) or text.
	Format string `yaml:"format"`
	// Dir is the log sink directory. Empty derives <data_dir>/logs.
	Dir string `yaml:"dir"`
	// RetentionDays deletes rotated vivy.log.* files older than this many
	// days at startup. 0 keeps every file (explicitly opting out).
	RetentionDays int `yaml:"retention_days"`
	// Stdout mirrors log lines to the console in addition to the file.
	Stdout bool `yaml:"stdout"`
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
	// World selects the filesystem world exposed to runs. "sandbox" keeps
	// one private directory per run; "local" mounts WorkspaceRoot itself.
	// Local is intended for the interactive code face and remains bounded by
	// the same sandbox and approval policy as every other run.
	World string `yaml:"world"`
	// SkillsRoot is a trusted, non-executable directory containing SKILL.md
	// packages. Skill content remains untrusted data at runtime.
	SkillsRoot string `yaml:"skills_root"`
	// SkillsMarketplaceURL is the base URL of the skills.sh directory used by
	// the skill marketplace (search/featured/install). Empty keeps the
	// default; the VIVY_SKILLS_MARKETPLACE_URL environment override wins at
	// service construction.
	SkillsMarketplaceURL string `yaml:"skills_marketplace_url"`
	// HTTPAllowedHosts is the explicit host surface for the read-only HTTP tool.
	HTTPAllowedHosts []string `yaml:"http_allowed_hosts"`
	// HTTPMaxResponseBytes bounds one HTTP response entering the model context.
	HTTPMaxResponseBytes int `yaml:"http_max_response_bytes"`
	// HTTPTimeoutSeconds bounds one HTTP tool request (default 10). Values
	// outside 1..120 are clamped by the runtime backend, so a typo can
	// neither disable the timeout nor stall a run for minutes.
	HTTPTimeoutSeconds int `yaml:"http_timeout_seconds"`
	// MCPServers are explicitly configured MCP servers. Each entry selects
	// exactly one transport: endpoint for Streamable HTTP or command for
	// local stdio. Stdio environment values are indirect references only.
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
	// Cron controls the scheduled-job scheduler behind the CRON panel.
	Cron CronConfig `yaml:"cron"`
	// Sandbox controls the file-effect policy boundary (D-021).
	Sandbox SandboxConfig `yaml:"sandbox"`
	// Hooks configures user hook scripts around tool execution (D8).
	Hooks HooksConfig `yaml:"hooks"`
	// SmallModel optionally names a cheaper model from the same active
	// provider for utility generations such as session auto-titles (VC-2).
	// It rides the active provider's base URL and key, so provider/model
	// management stays one data source; empty keeps the main model only.
	// Failures fall back down the utility chain.
	SmallModel string `yaml:"small_model"`
}

// HooksConfig is the user hook surface (D8). A hook script is a plain
// command the operator registers in config; it never runs until the same
// entry is explicitly marked approved — registration and arming are two
// separate human gestures, so no script can start executing silently.
type HooksConfig struct {
	// PreToolUse lists scripts run before each tool call. Decisions ride
	// the existing ToolHookChain and are journalled per run as
	// hook.started / hook.completed / hook.blocked events.
	PreToolUse []PreToolUseHook `yaml:"pre_tool_use"`
}

// PreToolUseHook is one user hook script (Claude-Code-style protocol:
// the call payload goes to stdin as JSON; exit 2 denies with stderr as
// the reason; exit 0 allows, with an optional stdout JSON envelope
// carrying decision / reason / updated_input shallow-merged into the
// tool arguments; any other exit fails closed).
type PreToolUseHook struct {
	// Matcher filters which tools trigger the hook: a glob over the tool
	// name (path.Match semantics). Empty or "*" matches every tool.
	Matcher string `yaml:"matcher"`
	// Command is the shell command line executed per matching call. It is
	// interpreted by the platform shell (cmd /c on Windows, sh -c
	// elsewhere) exactly as written.
	Command string `yaml:"command"`
	// TimeoutMs bounds one hook invocation; 0 keeps the governance
	// default (governance.hook_timeout). The effective bound is the
	// shorter of the two.
	TimeoutMs int `yaml:"timeout_ms"`
	// Approved is the human arming switch (D8: first registration needs
	// ask). A hook with approved=false is registered but inert — it is
	// never executed — and startup logs loudly until it is approved.
	Approved bool `yaml:"approved"`
}

// CronConfig is the operator switch for the cron scheduler. Jobs are only
// fired while this is enabled; with it off the store keeps its rows but
// nothing runs.
type CronConfig struct {
	// Enabled turns the scheduler on. Defaults to true.
	Enabled bool `yaml:"enabled"`
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
	// SummaryModel optionally names a cheaper model on the same active
	// provider that generates compaction summaries instead of the main
	// chat model. Empty keeps the main model (CMP-2). The main model
	// remains the automatic one-shot failover when the summary model
	// errors.
	SummaryModel string `yaml:"summary_model"`
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
	Name string `yaml:"name"`
	// Endpoint selects the Streamable HTTP transport. It is mutually
	// exclusive with Command.
	Endpoint string `yaml:"endpoint,omitempty"`
	// Command selects the local stdio transport. A PATH executable name or an
	// absolute executable path is allowed; relative paths are not.
	Command string `yaml:"command,omitempty"`
	// Args is one argv item per YAML list entry. It is never shell-parsed.
	Args []string `yaml:"args,omitempty"`
	// EnvFrom maps child environment variable names to host environment
	// variable names. Values are references, never secret values (D-010).
	EnvFrom map[string]string `yaml:"env_from,omitempty"`
	// Cwd is relative to runtime.workspace_root for stdio servers.
	Cwd     string `yaml:"cwd,omitempty"`
	AuthEnv string `yaml:"auth_env,omitempty"`
	// ResourceBridge opts this server into the explicit MCP resources ->
	// ContextHost projection. Prompts remain control-plane-only.
	ResourceBridge bool `yaml:"resource_bridge,omitempty"`
	// DeferredReason keeps an explicitly deferred instance visible without
	// attempting to activate it. The reason is an operator diagnostic, never a
	// secret value.
	DeferredReason string `yaml:"deferred_reason,omitempty"`
	// Enabled defaults to true when omitted. Settings overlays may carry an
	// explicit false through startup.
	Enabled *bool `yaml:"enabled,omitempty"`
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

// Channels maps a compiled-in channel plugin name to its envelope
// (VIVY-CHANNEL-PACK.md §11). The map key must match the name of a
// seam-channel plugin in the running generation; that match is enforced
// by the Host (C3) at start time, not here. Zero value (absent section)
// means no channel configuration.
type Channels map[string]ChannelEnvelope

// ChannelEnvelope is the kernel-owned envelope of one channel. It holds
// knobs only — it is not a plugin system, and per-channel settings stay
// opaque to the kernel.
type ChannelEnvelope struct {
	// Enabled=false keeps the channel compiled-in but not Started; the
	// channel still shows as compiled-in to inspect.
	Enabled bool `yaml:"enabled"`
	// AllowFrom is the inbound sender allow-list. Empty means deny-start
	// is decided by the Host at Start time (C3). "*" is not allowed in
	// this generation.
	AllowFrom []string `yaml:"allow_from"`
	// TokenEnv names the environment variable holding the platform token.
	// The token itself must never appear in config (D-010).
	TokenEnv string `yaml:"token_env"`
	// Settings is opaque to the kernel: the owning channel plugin decodes
	// it (fail-closed on its own unknown fields). yaml.Node keeps inner
	// keys out of strict decoding and out of this package.
	Settings yaml.Node `yaml:"settings"`
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

// LegacyToolSearchName is the name used by Vivy builds that exposed a
// hand-written tool_search implementation. The name remains reserved for
// the official Eino meta-tool, but old config/settings inputs are normalized
// before they reach the tool registry.
const LegacyToolSearchName = "tool_search"

// NormalizeLegacyToolSearch removes the retired Vivy tool_search entry while
// preserving the order of every other configured name. A non-nil input keeps
// a non-nil result, including when the legacy-only list becomes explicit
// chat-only (an empty active surface).
func NormalizeLegacyToolSearch(names []string) []string {
	if names == nil {
		return nil
	}
	out := make([]string, 0, len(names))
	for _, name := range names {
		if strings.TrimSpace(name) == LegacyToolSearchName {
			continue
		}
		out = append(out, name)
	}
	return out
}

// UnmarshalYAML reads the tools mapping and stashes the approval
// expiration as an unparsed duration string. An omitted enabled key keeps
// the code default shipped by Default() instead of clobbering it with nil.
// An explicit empty list is preserved as a non-nil empty slice so it can
// select the legal chat-only mode.
func (t *Tools) UnmarshalYAML(node *yaml.Node) error {
	var doc toolsDoc
	if err := node.Decode(&doc); err != nil {
		return fmt.Errorf("tools: %w", err)
	}
	hasEnabled := false
	if node.Kind == yaml.MappingNode {
		for i := 0; i+1 < len(node.Content); i += 2 {
			if node.Content[i].Value == "enabled" {
				hasEnabled = true
				break
			}
		}
	}
	if hasEnabled {
		if doc.Enabled == nil {
			t.Enabled = []string{}
		} else {
			t.Enabled = doc.Enabled
		}
	}
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
			World:                    "sandbox",
			SkillsRoot:               filepath.Join(root, "skills"),
			SkillsMarketplaceURL:     DefaultSkillsMarketplaceURL,
			HTTPAllowedHosts:         []string{"localhost", "127.0.0.1", "::1"},
			HTTPMaxResponseBytes:     1 << 20,
			HTTPTimeoutSeconds:       10,
			ExecuteAllowedCommands:   []string{"go", "git", "rg"},
			ExecuteMaxTimeoutSeconds: 30,
			Compaction:               DefaultCompactionConfig(),
			Cron:                     CronConfig{Enabled: true},
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
			Enabled:       []string{"write_note", "list_notes", "read_note", "ask_user", "list_dir", "read_file", "search_files", "write_file", "patch", "multiedit", "skills_list", "skill_view", "skill_manage", "task_create", "task_get", "task_update", "task_list", "network_search", "http_request", "web_fetch", "download", "mcp_list_tools", "sequential_thinking", "execute", "commandline", "bash", "job_output", "job_kill", "grep", "glob", "agent"},
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
		Logging: Logging{
			Level:         "info",
			Format:        "json",
			Dir:           "",
			RetentionDays: 30,
			Stdout:        true,
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
	cfg.Tools.Enabled = NormalizeLegacyToolSearch(cfg.Tools.Enabled)

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
	if c.Runtime.MaxEventPayloadBytes < 1024 {
		return errors.New("runtime.max_event_payload_bytes must be at least 1024")
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
	// small_model is an open-ended model id on the active provider; only
	// structural sanity is checkable here. Unknown ids fail at generation
	// time and fall back down the utility chain.
	c.Runtime.SmallModel = strings.TrimSpace(c.Runtime.SmallModel)
	if strings.ContainsAny(c.Runtime.SmallModel, "\r\n\x00") {
		return errors.New("runtime.small_model must be a single model id")
	}
	if c.Runtime.WorkspaceRoot == "" {
		return errors.New("runtime.workspace_root must not be empty")
	}
	c.Runtime.World = strings.ToLower(strings.TrimSpace(c.Runtime.World))
	if c.Runtime.World == "" {
		c.Runtime.World = "sandbox"
	}
	if c.Runtime.World != "sandbox" && c.Runtime.World != "local" {
		return fmt.Errorf("runtime.world %q is unsupported; use sandbox or local", c.Runtime.World)
	}
	if c.Runtime.SkillsRoot == "" {
		return errors.New("runtime.skills_root must not be empty")
	}
	if c.Runtime.SkillsMarketplaceURL != "" {
		parsed, err := url.Parse(c.Runtime.SkillsMarketplaceURL)
		if err != nil || parsed.Host == "" || parsed.Scheme != "http" && parsed.Scheme != "https" {
			return errors.New("runtime.skills_marketplace_url must be an absolute http(s) URL")
		}
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
	// summary_model is an open-ended model id on the active provider; only
	// structural sanity is checkable here (same as runtime.small_model).
	// Unknown ids surface as summary-model call errors and fail over to
	// the main model.
	c.Runtime.Compaction.SummaryModel = strings.TrimSpace(c.Runtime.Compaction.SummaryModel)
	if strings.ContainsAny(c.Runtime.Compaction.SummaryModel, "\r\n\x00") {
		return errors.New("runtime.compaction.summary_model must be a single model id")
	}
	seenMCPNames := make(map[string]int, len(c.Runtime.MCPServers))
	for i := range c.Runtime.MCPServers {
		server := &c.Runtime.MCPServers[i]
		if !mcpNamespacePattern.MatchString(server.Name) {
			return fmt.Errorf("runtime.mcp_servers[%d].name %q is not a safe namespace identifier", i, server.Name)
		}
		server.Name = strings.TrimSpace(server.Name)
		server.Endpoint = strings.TrimSpace(server.Endpoint)
		server.Command = strings.TrimSpace(server.Command)
		server.Cwd = strings.TrimSpace(server.Cwd)
		server.AuthEnv = strings.TrimSpace(server.AuthEnv)
		server.DeferredReason = strings.TrimSpace(server.DeferredReason)
		if err := validateMCPDeferredReason(server.DeferredReason); err != nil {
			return fmt.Errorf("runtime.mcp_servers[%d].deferred_reason: %w", i, err)
		}
		if server.Name == "" {
			return fmt.Errorf("runtime.mcp_servers[%d].name must not be empty", i)
		}
		nameKey := strings.ToLower(server.Name)
		if previous, ok := seenMCPNames[nameKey]; ok {
			return fmt.Errorf("runtime.mcp_servers[%d].name %q duplicates runtime.mcp_servers[%d]", i, server.Name, previous)
		}
		seenMCPNames[nameKey] = i
		if (server.Endpoint == "") == (server.Command == "") {
			return fmt.Errorf("runtime.mcp_servers[%d] must set exactly one of endpoint or command", i)
		}
		if server.Endpoint != "" {
			if err := ValidateMCPServerEndpoint(server.Endpoint); err != nil {
				return fmt.Errorf("runtime.mcp_servers[%d].endpoint: %w", i, err)
			}
			if server.Cwd != "" || len(server.Args) > 0 || len(server.EnvFrom) > 0 {
				return fmt.Errorf("runtime.mcp_servers[%d] stdio fields require command", i)
			}
		} else {
			if server.AuthEnv != "" {
				return fmt.Errorf("runtime.mcp_servers[%d].auth_env requires endpoint", i)
			}
			if err := validateMCPCommand(server.Command); err != nil {
				return fmt.Errorf("runtime.mcp_servers[%d].command: %w", i, err)
			}
			if err := validateMCPArgs(server.Command, server.Args); err != nil {
				return fmt.Errorf("runtime.mcp_servers[%d].args: %w", i, err)
			}
			if err := validateMCPWorkingDir(server.Cwd); err != nil {
				return fmt.Errorf("runtime.mcp_servers[%d].cwd: %w", i, err)
			}
			if len(server.EnvFrom) > maxMCPEnvEntries {
				return fmt.Errorf("runtime.mcp_servers[%d].env_from has too many entries", i)
			}
			for child, host := range server.EnvFrom {
				if !envKeyPattern.MatchString(strings.TrimSpace(child)) || !envKeyPattern.MatchString(strings.TrimSpace(host)) {
					return fmt.Errorf("runtime.mcp_servers[%d].env_from must map environment variable names", i)
				}
			}
		}
		if server.AuthEnv != "" && !envKeyPattern.MatchString(server.AuthEnv) {
			return fmt.Errorf("runtime.mcp_servers[%d].auth_env must be an environment variable name", i)
		}
		server.Args = append([]string(nil), server.Args...)
		if server.EnvFrom != nil {
			envFrom := make(map[string]string, len(server.EnvFrom))
			for child, host := range server.EnvFrom {
				envFrom[strings.TrimSpace(child)] = strings.TrimSpace(host)
			}
			server.EnvFrom = envFrom
		}
	}

	// An empty enabled list is an intentional chat-only configuration. The
	// runtime installs no static or dynamic tools for that request.
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

	// Channel envelope validation (VIVY-CHANNEL-PACK.md §11). The envelope
	// is fixed by the kernel; per-channel Settings are opaque here — the
	// owning plugin decodes them fail-closed, so inner keys are never
	// validated in this package.
	for name, ch := range c.Channels {
		if !channelNamePattern.MatchString(name) {
			return fmt.Errorf("channels.%s must be lowercase digits and hyphens", name)
		}
		if ch.TokenEnv != "" && !envKeyPattern.MatchString(ch.TokenEnv) {
			return fmt.Errorf("channels.%s.token_env %q is not an environment variable name; "+
				"secrets must never appear in config (D-010)", name, ch.TokenEnv)
		}
		for i, allow := range ch.AllowFrom {
			if strings.TrimSpace(allow) == "" {
				return fmt.Errorf("channels.%s.allow_from[%d] must not be empty; empty allow_from means "+
					"deny-start is decided by the Host at Start time", name, i)
			}
			if allow == "*" {
				return fmt.Errorf("channels.%s.allow_from[%d] %q is not allowed in this generation; "+
					"list explicit senders (VIVY-CHANNEL-PACK.md §11)", name, i, allow)
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

	// Hook configuration validation (D8): a typo must not silently
	// disable or mis-clamp a governance script.
	const hookTimeoutCapMs = 60 * 60 * 1000
	for i, hook := range c.Runtime.Hooks.PreToolUse {
		if strings.TrimSpace(hook.Command) == "" {
			return fmt.Errorf("runtime.hooks.pre_tool_use[%d].command must not be empty", i)
		}
		if hook.TimeoutMs < 0 || hook.TimeoutMs > hookTimeoutCapMs {
			return fmt.Errorf("runtime.hooks.pre_tool_use[%d].timeout_ms %d must be 0 (governance default) or 1..%d", i, hook.TimeoutMs, hookTimeoutCapMs)
		}
		if hook.Matcher != "" && hook.Matcher != "*" {
			if _, err := path.Match(hook.Matcher, "probe"); err != nil {
				return fmt.Errorf("runtime.hooks.pre_tool_use[%d].matcher %q is not a valid glob: %v", i, hook.Matcher, err)
			}
		}
	}

	switch strings.ToLower(strings.TrimSpace(c.Logging.Level)) {
	case "", "debug", "info", "warn", "warning", "error":
	default:
		return fmt.Errorf("logging.level %q must be debug, info, warn, or error", c.Logging.Level)
	}
	switch strings.ToLower(strings.TrimSpace(c.Logging.Format)) {
	case "", "json", "text":
	default:
		return fmt.Errorf("logging.format %q must be json or text", c.Logging.Format)
	}
	if c.Logging.RetentionDays < 0 {
		return errors.New("logging.retention_days must not be negative")
	}

	return nil
}

func validateMCPDeferredReason(reason string) error {
	if strings.ContainsAny(reason, "\r\n\x00") {
		return errors.New("must be a single-line diagnostic")
	}
	if len([]rune(reason)) > 256 {
		return errors.New("must not exceed 256 characters")
	}
	return nil
}

func validateMCPCommand(command string) error {
	command = strings.TrimSpace(command)
	if command == "" {
		return errors.New("must not be empty")
	}
	abs := filepath.IsAbs(command)
	if strings.ContainsAny(command, "\t\r\n;&|><$()\"'`") || (!abs && strings.ContainsRune(command, ' ')) {
		return errors.New("must be a PATH executable name or absolute path without shell syntax")
	}
	if strings.ContainsAny(command, `/\\`) && !abs {
		return errors.New("relative executable paths are not allowed")
	}
	base := strings.ToLower(filepath.Base(command))
	ext := filepath.Ext(base)
	if ext == ".exe" || ext == ".cmd" || ext == ".bat" {
		base = strings.TrimSuffix(base, ext)
	}
	if commandpolicy.IsDeniedExecutable(command) {
		return fmt.Errorf("executable %q is denied by the MCP command safety policy", base)
	}
	return nil
}

func validateMCPArgs(command string, args []string) error {
	if len(args) > maxMCPArgs {
		return fmt.Errorf("too many arguments (maximum %d)", maxMCPArgs)
	}
	total := 0
	for _, arg := range args {
		if len(arg) > maxMCPArgBytes {
			return fmt.Errorf("argument exceeds %d bytes", maxMCPArgBytes)
		}
		if strings.IndexByte(arg, 0) >= 0 || strings.ContainsAny(arg, "\r\n") {
			return errors.New("argument contains NUL or newline")
		}
		if strings.HasSuffix(strings.ToLower(command), ".cmd") || strings.HasSuffix(strings.ToLower(command), ".bat") {
			if strings.ContainsAny(arg, "&|<>^%") {
				return errors.New("batch-file arguments cannot contain shell metacharacters")
			}
		}
		total += len(arg)
		if total > maxMCPArgsBytes {
			return fmt.Errorf("argument payload exceeds %d bytes", maxMCPArgsBytes)
		}
	}
	return nil
}

func validateMCPWorkingDir(cwd string) error {
	if cwd == "" {
		return nil
	}
	if strings.IndexByte(cwd, 0) >= 0 || filepath.IsAbs(cwd) {
		return errors.New("must be relative to runtime.workspace_root")
	}
	clean := filepath.Clean(cwd)
	if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return errors.New("must stay within runtime.workspace_root")
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

// LogDirectory resolves the log sink directory: logging.dir when set,
// else logs/ under the data directory, so rotated files sit beside the
// rest of the runtime's scratch without touching the Journal itself.
func (c Config) LogDirectory() string {
	if dir := strings.TrimSpace(c.Logging.Dir); dir != "" {
		return dir
	}
	return filepath.Join(c.DataDirectory(), "logs")
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
