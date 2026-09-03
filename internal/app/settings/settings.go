// Package settings holds the operator-managed runtime configuration for
// Vivy itself: the active model provider selection (bundle, default model,
// optional OpenAI-compatible base URL, optional API key overlay), the
// user-defined provider registry (custom providers with their own base URL,
// model list, and optional API key), the network_search preference, the
// execute ceiling override, the sandbox overlay, the MCP server overlay,
// and the per-channel knobs overlay. These live in the shared user
// workspace (~/.vivy/settings.yaml) so every Vivy version reads the same
// API and system configuration. The web process normally keeps its Journal
// beside this file; independent code-face processes deliberately point back
// to this settings file while storing their Journals elsewhere.
//
// Secrets: committed config still holds env_key names only (D-010). This
// runtime settings document may hold plaintext api_key values (file mode
// 0600); they are never logged and never returned by the control plane
// (only api_key_set booleans cross the wire). A frozen ENV session is the
// only case the UI cannot change. MCP server writes also ReplaceServers on
// the live catalog immediately.
package settings

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"

	"agent-vivy/internal/domain"
)

// DefaultDir is the independent agent working directory under the user data
// root when no explicit data directory is configured. It is not data/ and
// not the tenant Journal. App resolves the actual root from config
// DataDirectory (see config.UserDataDir); this constant is the fallback used
// by Path with an empty root.
const DefaultDir = "data/dev-home"

// FileName is the settings document name inside DefaultDir.
const FileName = "settings.yaml"

// Valid provider bundle names V0 can activate.
const (
	ProviderOpenAI    = "openai"
	ProviderAnthropic = "anthropic"
)

// legacyProviderMock is the provider name written by builds that still had
// the deterministic mock runtime. It is only used to migrate old runtime
// settings; mock is not a supported provider and must never be accepted by
// validation or provider resolution.
const legacyProviderMock = "mock"

// apiBasePattern bounds the base URL to http(s) absolute URLs. It carries a
// URL, not a secret, but is still validated before use.
var apiBasePattern = regexp.MustCompile(`^https?://[^\s/]+(:\d+)?(/.*)?$`)

// envKeyPattern constrains auth_env to an environment variable NAME.
// Anything else (a literal token) fails validation (D-010).
var envKeyPattern = regexp.MustCompile(`^[A-Z][A-Z0-9_]*$`)

// channelNamePattern constrains a channels overlay name to the same
// plugin-name slug config.yaml requires for its channels.<name> keys.
var channelNamePattern = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// maxExecuteTimeoutSeconds mirrors config.maxExecuteTimeoutSeconds and the
// runtime hard cap (10m): a settings override above it would be silently
// clamped, so it is rejected here up front.
const maxExecuteTimeoutSeconds = 600

// maxHTTPTimeoutSeconds mirrors the runtime HTTP backend clamp ceiling: the
// backend clamps rather than fails, but a settings override beyond it would
// never take effect, so it is rejected here up front.
const maxHTTPTimeoutSeconds = 120

// Settings is the persisted, non-secret model provider selection plus the
// network_search preference and the execute ceiling override.
type Settings struct {
	// Provider is the active bundle name; empty means "use config default".
	Provider string `yaml:"provider"`
	// DefaultModel overrides the bundle's default model; empty means "use
	// bundle default".
	DefaultModel string `yaml:"default_model"`
	// BaseURL is an optional OpenAI-compatible gateway; empty means "use
	// bundle default". It is applied through the existing VIVY_API_BASE
	// mechanism.
	BaseURL string `yaml:"base_url"`
	// ApiKey optionally overlays the active bundle's API key (see package
	// doc: plaintext runtime data only, never logged or returned). Empty
	// means "no overlay" — the bundle's env_key environment variable stands.
	ApiKey string `yaml:"api_key"`
	// Providers is the user-defined provider registry. Entries hold a base
	// URL, a model list, and an optional API key (write-only on the wire);
	// the UI creates, edits, and deletes them through the control plane.
	Providers []ProviderEntry `yaml:"providers"`
	// NetworkSearch overrides tools.network_search.provider from the
	// Settings UI; empty keeps the config value. Provider credentials stay
	// environment-only (D-010).
	NetworkSearch NetworkSearchSettings `yaml:"network_search"`
	// ExecuteMaxTimeoutSeconds overrides the execute/commandline ceiling
	// (config runtime.execute_max_timeout_seconds); 0 means "use config
	// value". Same bounds as config: 1–600, mirroring the runtime hard cap.
	ExecuteMaxTimeoutSeconds int `yaml:"execute_max_timeout_seconds"`
	// MCPServers overlays config runtime.mcp_servers. A nil pointer means
	// "use config default"; a non-nil (including empty) slice is the
	// operator-managed list and replaces the config default entirely.
	// Credentials stay env references only (D-010).
	MCPServers *[]MCPServer `yaml:"mcp_servers"`
	// ToolsEnabled overlays config tools.enabled: the active tool surface
	// bound on every request. A nil pointer means "use config default"; a
	// non-nil (including empty) slice is the operator-managed active set
	// and replaces the config default entirely — an explicit empty list is
	// the legal chat-only mode. Names must resolve against the registry
	// (unknown names fail engine Resolve, FR-10).
	ToolsEnabled *[]string `yaml:"tools_enabled,omitempty"`
	// Sandbox is the operator-managed default permission preset and network
	// policy for new sessions. Empty keeps the production config.
	Sandbox SandboxSettings `yaml:"sandbox"`
	// Compaction overlays config runtime.compaction (context compression).
	// A nil pointer means "use config default"; zero numeric fields inside a
	// present overlay also keep the config values. Enabled distinguishes
	// "unset" from an explicit false.
	Compaction *CompactionSettings `yaml:"compaction"`
	// HTTP overlays config runtime.http_allowed_hosts / http_timeout_seconds
	// for the read-only http_request tool. A nil pointer means "use config
	// default"; a nil AllowedHosts inside a present overlay keeps the config
	// hosts. The runtime backend clamps the timeout to 1..120.
	HTTP *HTTPSettings `yaml:"http"`
	// Channels is the per-channel overlay for compiled-in channel plugins.
	// Each entry carries at most the three UI knobs (enabled, allow_from,
	// token_env) and merges over the config.yaml channels envelope; the
	// envelope's opaque per-plugin Settings block stays in config.yaml
	// untouched. Entries name channels that may no longer be compiled-in:
	// the startup overlay silently drops those (with a warning) so a stale
	// overlay can never fail startup.
	Channels []ChannelOverlay `yaml:"channels,omitempty"`
}

// CompactionSettings is the UI-managed context compression overlay. Zero
// values mean "use config default"; Enabled uses a pointer so an explicit
// false is distinguishable from unset.
type CompactionSettings struct {
	Enabled        *bool `yaml:"enabled,omitempty"`
	MaxTokens      int   `yaml:"max_tokens,omitempty"`
	TriggerPercent int   `yaml:"trigger_percent,omitempty"`
	KeepRecent     int   `yaml:"keep_recent,omitempty"`
}

// HTTPSettings is the UI-managed overlay for the read-only http_request
// tool (settings.yaml http). Nil AllowedHosts keeps the config allowlist;
// TimeoutSeconds 0 keeps the config value; the runtime backend clamps the
// effective timeout to 1..120 seconds.
type HTTPSettings struct {
	AllowedHosts   *[]string `yaml:"allowed_hosts,omitempty"`
	TimeoutSeconds int       `yaml:"timeout_seconds,omitempty"`
}

// ChannelOverlay is the UI-managed overlay for one compiled-in channel
// (settings.yaml channels). It carries knobs only; the channel plugin's
// opaque settings stay in config.yaml untouched. Pointer fields keep
// "not set" (keep the config.yaml value) distinct from "set empty" —
// an explicit empty allow_from is the fail-closed deny-start state the
// Host enforces at the next process restart.
type ChannelOverlay struct {
	// Name must match a channel plugin name compiled into the running
	// generation. Unknown names are dropped with a warning at startup.
	Name string `yaml:"name"`
	// Enabled replaces the config envelope's enabled when set.
	Enabled *bool `yaml:"enabled,omitempty"`
	// AllowFrom replaces the config envelope's inbound sender allow-list
	// when set, including an explicit empty list (deny-start).
	AllowFrom *[]string `yaml:"allow_from,omitempty"`
	// TokenEnv replaces the config envelope's token_env when set. It is an
	// environment variable NAME; the secret value never appears here (D-010).
	TokenEnv *string `yaml:"token_env,omitempty"`
}

// SandboxSettings is the UI-managed sandbox overlay. DefaultPreset is one of
// cautious/smart/trusted; empty keeps the config default.
type SandboxSettings struct {
	DefaultPreset domain.PermissionPreset `yaml:"default_preset"`
	Network       SandboxNetworkSettings  `yaml:"network"`
}

// SandboxNetworkSettings overlays runtime.sandbox.network. Nil pointer /
// nil slice mean "keep config".
type SandboxNetworkSettings struct {
	DenyPrivateIPs *bool    `yaml:"deny_private_ips"`
	AllowedDomains []string `yaml:"allowed_domains"`
}

// ProviderEntry is one user-defined provider in the registry. The API key is
// plaintext runtime data (same boundary as Settings.ApiKey): 0600 document,
// never logged, never returned by the control plane — the wire carries
// api_key_set only. Runtime key resolution matches the active selection to an
// entry by (bundle, base_url).
type ProviderEntry struct {
	// ID is a stable UI-generated key (e.g. custom-<uuid>); rename/edit
	// keeps it so the UI can address the entry without restating its key.
	ID string `yaml:"id"`
	// DisplayName is the user-facing alias shown in the Settings UI.
	DisplayName string `yaml:"display_name"`
	// Bundle is the runtime bundle: openai or anthropic.
	Bundle string `yaml:"bundle"`
	// BaseURL is the OpenAI-compatible gateway address for this entry.
	BaseURL string `yaml:"base_url"`
	// DefaultModel is the entry's preferred model; empty means "use bundle
	// default".
	DefaultModel string `yaml:"default_model"`
	// Models is the model list shown in the Settings UI (raw model ids).
	Models []string `yaml:"models"`
	// ApiKey optionally overlays the bundle's env_key when this entry is the
	// active selection; empty means "no overlay".
	ApiKey string `yaml:"api_key"`
}

// NetworkSearchSettings is the UI-managed network_search preference.
// Provider credentials stay environment-only (D-010).
type NetworkSearchSettings struct {
	// Provider is the preferred provider name, or empty for automatic.
	Provider string `yaml:"provider"`
}

// MCPServer is one operator-managed Streamable HTTP MCP server. AuthEnv is
// an environment variable name, never a secret value.
type MCPServer struct {
	Name     string `yaml:"name"`
	Endpoint string `yaml:"endpoint"`
	AuthEnv  string `yaml:"auth_env,omitempty"`
	// Enabled defaults to true when omitted. A pointer distinguishes
	// "unset" from an explicit false (YAML bool zero is false).
	Enabled *bool `yaml:"enabled,omitempty"`
}

// MCPServerEnabled reports whether the server joins the live catalog.
// Missing enabled is true.
func MCPServerEnabled(server MCPServer) bool {
	return server.Enabled == nil || *server.Enabled
}

// BoolPtr returns a pointer to v for YAML/JSON optional booleans.
func BoolPtr(v bool) *bool { return &v }

// StringPtr returns a pointer to v for YAML/JSON optional strings. An
// explicit empty string stays distinct from an absent field.
func StringPtr(v string) *string { return &v }

// Path returns the absolute settings file path for the given data root.
// An empty root falls back to the conventional DefaultDir.
func Path(dataRoot string) string {
	if dataRoot == "" {
		dataRoot = DefaultDir
	}
	return filepath.Join(dataRoot, FileName)
}

// Default returns the empty (config-driven) settings.
func Default() Settings {
	return Settings{}
}

// IsZero reports whether the document carries no runtime overlay at all: no
// active selection, no legacy key, no registry, no network/execute override.
// A missing document and an empty document are equivalent for the overlay.
func (s Settings) IsZero() bool {
	return s.Provider == "" &&
		s.DefaultModel == "" &&
		s.BaseURL == "" &&
		s.ApiKey == "" &&
		len(s.Providers) == 0 &&
		s.NetworkSearch == (NetworkSearchSettings{}) &&
		s.ExecuteMaxTimeoutSeconds == 0 &&
		s.MCPServers == nil &&
		s.ToolsEnabled == nil &&
		s.Sandbox.DefaultPreset == "" &&
		s.Sandbox.Network.DenyPrivateIPs == nil &&
		len(s.Sandbox.Network.AllowedDomains) == 0 &&
		s.Compaction == nil &&
		s.HTTP == nil &&
		len(s.Channels) == 0
}

// Load reads and validates the settings document at path. A missing file is
// not an error: it returns the zero Settings so the config defaults stand.

// fileMu serializes access to the settings document file. On Windows,
// renaming over a file another goroutine is reading fails with access
// denied, and concurrent writes to a shared temp file could publish a
// corrupt document; readers and writers take this lock around the file
// I/O window. Update holds it across the whole read-modify-write, so a
// fn that itself calls Load/Save/Update would deadlock and must not.
var fileMu sync.Mutex

func Load(path string) (Settings, error) {
	fileMu.Lock()
	defer fileMu.Unlock()
	return load(path)
}

// load is the unlocked Load body; callers must hold fileMu.
func load(path string) (Settings, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Settings{}, nil
		}
		return Settings{}, fmt.Errorf("settings: read %s: %w", path, err)
	}
	var s Settings
	dec := yaml.NewDecoder(bytes.NewReader(data))
	if err := dec.Decode(&s); err != nil {
		return Settings{}, fmt.Errorf("settings: parse %s: %w", path, err)
	}
	// Older builds could persist a mock provider selection or registry rows.
	// The mock runtime no longer exists, so treat those records as an absent
	// overlay instead of making every later settings write fail validation.
	// The migrated value is returned in memory; the next successful settings
	// write rewrites the document without the retired records.
	migrateLegacyMock(&s)
	// Normalize a decoded empty registry to nil so a document round-trip is
	// stable (yaml marshals nil and empty slices identically).
	if len(s.Providers) == 0 {
		s.Providers = nil
	}
	if len(s.Sandbox.Network.AllowedDomains) == 0 {
		s.Sandbox.Network.AllowedDomains = nil
	}
	if s.Compaction != nil &&
		s.Compaction.Enabled == nil &&
		s.Compaction.MaxTokens == 0 &&
		s.Compaction.TriggerPercent == 0 &&
		s.Compaction.KeepRecent == 0 {
		// An explicitly-empty compaction overlay means "config default";
		// normalize to nil so a document round-trip is stable.
		s.Compaction = nil
	}
	if s.HTTP != nil && s.HTTP.AllowedHosts == nil && s.HTTP.TimeoutSeconds == 0 {
		// An explicitly-empty http overlay means "config default"; normalize
		// to nil so a document round-trip is stable.
		s.HTTP = nil
	}
	if len(s.Channels) == 0 {
		s.Channels = nil
	} else {
		next := make([]ChannelOverlay, 0, len(s.Channels))
		for _, entry := range s.Channels {
			if entry.Enabled == nil && entry.AllowFrom == nil && entry.TokenEnv == nil {
				// An all-empty overlay entry carries no knob; normalize it away
				// so it cannot mark an unconfigured channel as configured and
				// a document round-trip stays stable.
				continue
			}
			next = append(next, entry)
		}
		s.Channels = next
	}
	if err := s.Validate(); err != nil {
		return Settings{}, fmt.Errorf("settings: %s: %w", path, err)
	}
	return s, nil
}

func migrateLegacyMock(s *Settings) {
	if s.Provider == legacyProviderMock {
		// The old mock selection had no real provider semantics. Clear all
		// selection-scoped fields so the configured production default applies.
		s.Provider = ""
		s.DefaultModel = ""
		s.BaseURL = ""
		s.ApiKey = ""
	}
	if len(s.Providers) == 0 {
		return
	}
	kept := s.Providers[:0]
	for _, entry := range s.Providers {
		if entry.Bundle == legacyProviderMock {
			continue
		}
		kept = append(kept, entry)
	}
	if len(kept) == 0 {
		s.Providers = nil
	} else {
		s.Providers = kept
	}
}

// Validate rejects secret-shaped or structurally invalid values.
func (s Settings) Validate() error {
	switch s.Provider {
	case "":
		// empty => config default; allowed
	case ProviderOpenAI, ProviderAnthropic:
	default:
		return fmt.Errorf("settings: provider %q unsupported; want openai or anthropic", s.Provider)
	}
	if s.Provider == "" && s.DefaultModel != "" {
		return errors.New("settings: default_model requires a provider to be set")
	}
	if s.BaseURL != "" && !apiBasePattern.MatchString(s.BaseURL) {
		return fmt.Errorf("settings: base_url %q must be an http(s) absolute URL", s.BaseURL)
	}
	if s.ApiKey != "" && strings.ContainsAny(s.ApiKey, "\r\n") {
		return errors.New("settings: api_key must not contain newlines")
	}
	if err := validateProviderEntries(s.Providers); err != nil {
		return err
	}
	switch s.NetworkSearch.Provider {
	case "":
		// empty => automatic; allowed
	case "bing", "google", "duckduckgo", "searxng", "wikipedia":
	default:
		return fmt.Errorf("settings: network_search.provider %q unsupported; want bing, google, duckduckgo, searxng, or wikipedia", s.NetworkSearch.Provider)
	}
	// 0 keeps the config value; anything else shares the config bounds so a
	// UI override can never exceed the runtime hard cap.
	if s.ExecuteMaxTimeoutSeconds < 0 || s.ExecuteMaxTimeoutSeconds > maxExecuteTimeoutSeconds {
		return fmt.Errorf("settings: execute_max_timeout_seconds must be 0 (config default) or between 1 and %d", maxExecuteTimeoutSeconds)
	}
	if err := validateMCPServers(s.MCPServers); err != nil {
		return err
	}
	if err := validateToolsEnabled(s.ToolsEnabled); err != nil {
		return err
	}
	if s.Sandbox.DefaultPreset != "" && !s.Sandbox.DefaultPreset.ValidSwitch() {
		return fmt.Errorf("settings: sandbox.default_preset %q unsupported; want cautious, smart, or trusted", s.Sandbox.DefaultPreset)
	}
	for i, domainName := range s.Sandbox.Network.AllowedDomains {
		if strings.TrimSpace(domainName) == "" {
			return fmt.Errorf("settings: sandbox.network.allowed_domains[%d] must not be empty", i)
		}
	}
	if s.Compaction != nil {
		if s.Compaction.MaxTokens < 0 {
			return errors.New("settings: compaction.max_tokens must not be negative")
		}
		// 0 keeps the config value; anything else shares the config bounds.
		if s.Compaction.TriggerPercent < 0 || s.Compaction.TriggerPercent > 100 {
			return errors.New("settings: compaction.trigger_percent must be 0 (config default) or between 1 and 100")
		}
		if s.Compaction.KeepRecent < 0 {
			return errors.New("settings: compaction.keep_recent must be 0 (config default) or at least 1")
		}
	}
	if s.HTTP != nil {
		// 0 keeps the config value; anything above the runtime clamp ceiling
		// would never take effect, so it is rejected up front.
		if s.HTTP.TimeoutSeconds < 0 || s.HTTP.TimeoutSeconds > maxHTTPTimeoutSeconds {
			return fmt.Errorf("settings: http.timeout_seconds must be 0 (config default) or between 1 and %d", maxHTTPTimeoutSeconds)
		}
		if s.HTTP.AllowedHosts != nil {
			for i, host := range *s.HTTP.AllowedHosts {
				if strings.TrimSpace(host) == "" {
					return fmt.Errorf("settings: http.allowed_hosts[%d] must not be empty", i)
				}
			}
		}
	}
	if err := validateChannels(s.Channels); err != nil {
		return err
	}
	return nil
}

// validateChannels checks the per-channel overlay with the same rules
// config.Validate applies to its channels envelopes: slug names, env-name
// token_env, non-empty explicit senders, and no "*" wildcard (the Host
// refuses an unrestricted ear in this generation). Names are unique so
// the startup merge is unambiguous.
func validateChannels(entries []ChannelOverlay) error {
	seen := make(map[string]int, len(entries))
	for i, entry := range entries {
		if !channelNamePattern.MatchString(entry.Name) {
			return fmt.Errorf("settings: channels[%d].name %q must be lowercase digits and hyphens", i, entry.Name)
		}
		if prev, ok := seen[entry.Name]; ok {
			return fmt.Errorf("settings: channels[%d] name %q duplicates channels[%d]", i, entry.Name, prev)
		}
		seen[entry.Name] = i
		if entry.TokenEnv != nil && *entry.TokenEnv != "" && !envKeyPattern.MatchString(*entry.TokenEnv) {
			return fmt.Errorf("settings: channels[%d].token_env must be an environment variable name", i)
		}
		if entry.AllowFrom != nil {
			for j, allow := range *entry.AllowFrom {
				if strings.TrimSpace(allow) == "" {
					return fmt.Errorf("settings: channels[%d].allow_from[%d] must not be empty; empty allow_from means deny-start is decided by the Host at Start time", i, j)
				}
				if allow == "*" {
					return fmt.Errorf("settings: channels[%d].allow_from[%d] %q is not allowed in this generation; list explicit senders", i, j, allow)
				}
			}
		}
	}
	return nil
}

// validateToolsEnabled checks the active-tool overlay structurally: names
// are trimmed, never empty, and unique so the merge onto tools.enabled is
// unambiguous. Registry membership is enforced by engine Resolve (FR-10).
func validateToolsEnabled(enabled *[]string) error {
	if enabled == nil {
		return nil
	}
	seen := make(map[string]int, len(*enabled))
	for i, name := range *enabled {
		trimmed := strings.TrimSpace(name)
		if trimmed == "" {
			return fmt.Errorf("settings: tools_enabled[%d] must not be empty", i)
		}
		if prev, ok := seen[trimmed]; ok {
			return fmt.Errorf("settings: tools_enabled[%d] %q duplicates tools_enabled[%d]", i, trimmed, prev)
		}
		seen[trimmed] = i
		(*enabled)[i] = trimmed
	}
	return nil
}

func validateMCPServers(servers *[]MCPServer) error {
	if servers == nil {
		return nil
	}
	seen := make(map[string]int, len(*servers))
	for i, server := range *servers {
		name := strings.TrimSpace(server.Name)
		endpoint := strings.TrimSpace(server.Endpoint)
		authEnv := strings.TrimSpace(server.AuthEnv)
		if name == "" || endpoint == "" {
			return fmt.Errorf("settings: mcp_servers[%d] requires name and endpoint", i)
		}
		if !apiBasePattern.MatchString(endpoint) {
			return fmt.Errorf("settings: mcp_servers[%d].endpoint %q must be an http(s) absolute URL", i, endpoint)
		}
		if authEnv != "" && !envKeyPattern.MatchString(authEnv) {
			return fmt.Errorf("settings: mcp_servers[%d].auth_env must be an environment variable name", i)
		}
		key := strings.ToLower(name)
		if prev, ok := seen[key]; ok {
			return fmt.Errorf("settings: mcp_servers[%d] name %q duplicates mcp_servers[%d]", i, name, prev)
		}
		seen[key] = i
		(*servers)[i].Name = name
		(*servers)[i].Endpoint = endpoint
		(*servers)[i].AuthEnv = authEnv
	}
	return nil
}

// validateProviderEntries checks every registry entry. Entries are optional;
// when present, all structural fields must be valid and no two entries may
// share the same (bundle, base_url) — that pair is the runtime identity the
// active selection resolves its key from.
func validateProviderEntries(entries []ProviderEntry) error {
	seen := make(map[string]string, len(entries))
	for i, e := range entries {
		if strings.TrimSpace(e.ID) == "" {
			return fmt.Errorf("settings: providers[%d].id must not be empty", i)
		}
		if strings.TrimSpace(e.DisplayName) == "" {
			return fmt.Errorf("settings: providers[%d].display_name must not be empty", i)
		}
		switch e.Bundle {
		case ProviderOpenAI, ProviderAnthropic:
		default:
			return fmt.Errorf("settings: providers[%d].bundle %q unsupported; want openai or anthropic", i, e.Bundle)
		}
		if !apiBasePattern.MatchString(e.BaseURL) {
			return fmt.Errorf("settings: providers[%d].base_url %q must be an http(s) absolute URL", i, e.BaseURL)
		}
		for j, m := range e.Models {
			if strings.TrimSpace(m) == "" {
				return fmt.Errorf("settings: providers[%d].models[%d] must not be empty", i, j)
			}
		}
		if e.ApiKey != "" && strings.ContainsAny(e.ApiKey, "\r\n") {
			return fmt.Errorf("settings: providers[%d].api_key must not contain newlines", i)
		}
		key := e.Bundle + "\x00" + e.BaseURL
		if prev, ok := seen[key]; ok {
			return fmt.Errorf("settings: providers[%d] (%s, %s) duplicates providers entry %q", i, e.Bundle, e.BaseURL, prev)
		}
		seen[key] = e.DisplayName
	}
	return nil
}

// FindProvider returns the registry entry whose (bundle, base_url) matches
// the live selection, and whether a match exists. The active selection
// resolves its API key from this entry (authoritative over the legacy
// Settings.ApiKey overlay when the UI writes the registry).
func (s Settings) FindProvider(bundle, baseURL string) (ProviderEntry, bool) {
	for _, e := range s.Providers {
		if e.Bundle == bundle && e.BaseURL == baseURL {
			return e, true
		}
	}
	return ProviderEntry{}, false
}

// ActiveKey resolves the environment overlay key for the active selection:
// the registry entry matching (bundle, base_url) carries the key
// authoritatively (the UI no longer echoes secrets on select), and the
// legacy Settings.ApiKey overlay remains the fallback for older documents
// and non-registry selections. Empty means "no overlay" — the bundle's
// env_key environment variable stands.
func ActiveKey(s Settings, provider, baseURL string) string {
	if e, ok := s.FindProvider(provider, baseURL); ok && e.ApiKey != "" {
		return e.ApiKey
	}
	return s.ApiKey
}

// UpsertProvider inserts or replaces one registry entry by id. When the id
// already exists the whole entry is replaced (keeping the id stable, so the
// UI can address it without restating secrets); otherwise it is appended.
// The returned Settings is the candidate with the entry applied; validation
// happens in Save.
func (s Settings) UpsertProvider(entry ProviderEntry) Settings {
	for i, e := range s.Providers {
		if e.ID == entry.ID {
			next := append([]ProviderEntry(nil), s.Providers...)
			next[i] = entry
			s.Providers = next
			return s
		}
	}
	s.Providers = append(append([]ProviderEntry(nil), s.Providers...), entry)
	return s
}

// MCPServersOrEmpty returns the operator-managed MCP list, or nil when the
// overlay has never been written (config default stands).
func (s Settings) MCPServersOrEmpty() []MCPServer {
	if s.MCPServers == nil {
		return nil
	}
	out := make([]MCPServer, len(*s.MCPServers))
	copy(out, *s.MCPServers)
	return out
}

// UpsertMCPServer inserts or replaces one MCP server by case-insensitive
// name. A nil overlay becomes an explicit list. Validation happens in Save.
func (s Settings) UpsertMCPServer(entry MCPServer) Settings {
	entry.Name = strings.TrimSpace(entry.Name)
	entry.Endpoint = strings.TrimSpace(entry.Endpoint)
	entry.AuthEnv = strings.TrimSpace(entry.AuthEnv)
	current := s.MCPServersOrEmpty()
	key := strings.ToLower(entry.Name)
	for i, existing := range current {
		if strings.ToLower(existing.Name) == key {
			next := append([]MCPServer(nil), current...)
			next[i] = entry
			s.MCPServers = &next
			return s
		}
	}
	next := append(append([]MCPServer(nil), current...), entry)
	s.MCPServers = &next
	return s
}

// DeleteMCPServer removes one MCP server by case-insensitive name. The
// overlay becomes an explicit (possibly empty) list so a delete is not
// confused with "use config default". ok is false when the name is absent.
func (s Settings) DeleteMCPServer(name string) (Settings, bool) {
	current := s.MCPServersOrEmpty()
	key := strings.ToLower(strings.TrimSpace(name))
	next := make([]MCPServer, 0, len(current))
	found := false
	for _, existing := range current {
		if strings.ToLower(existing.Name) == key {
			found = true
			continue
		}
		next = append(next, existing)
	}
	if !found {
		return s, false
	}
	s.MCPServers = &next
	return s, true
}

// UpsertChannelOverlay inserts or replaces one channel overlay entry by
// name. Pointers pass straight through so an unset field keeps the
// config.yaml value and a set field (including an explicit empty
// allow_from) replaces it. Validation happens in Save.
func (s Settings) UpsertChannelOverlay(entry ChannelOverlay) Settings {
	for i, existing := range s.Channels {
		if existing.Name == entry.Name {
			next := append([]ChannelOverlay(nil), s.Channels...)
			next[i] = entry
			s.Channels = next
			return s
		}
	}
	s.Channels = append(append([]ChannelOverlay(nil), s.Channels...), entry)
	return s
}

// Save writes the settings atomically to path, creating parent directories.
// It returns the persisted document so callers can echo it back. Cross-
// handler read-modify-write cycles belong in Update, which keeps another
// writer's concurrent change between load and commit instead of dropping it.
func Save(path string, s Settings) (Settings, error) {
	if err := s.Validate(); err != nil {
		return Settings{}, err
	}
	fileMu.Lock()
	defer fileMu.Unlock()
	return write(path, s)
}

// write is the unlocked Save body; callers must hold fileMu and pass an
// already-validated document.
func write(path string, s Settings) (Settings, error) {
	if dir := filepath.Dir(path); dir != "" {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return Settings{}, fmt.Errorf("settings: create dir: %w", err)
		}
	}
	data, err := yaml.Marshal(s)
	if err != nil {
		return Settings{}, fmt.Errorf("settings: marshal: %w", err)
	}
	// Unique temp file per call, under fileMu: two concurrent Saves sharing
	// one temp file could interleave their writes and publish a corrupt
	// document, and on Windows renaming over a file a concurrent reader
	// holds open fails with access denied.
	f, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".*.tmp")
	if err != nil {
		return Settings{}, fmt.Errorf("settings: tmp: %w", err)
	}
	tmp := f.Name()
	if _, err := f.Write(data); err != nil {
		f.Close()
		os.Remove(tmp)
		return Settings{}, fmt.Errorf("settings: write: %w", err)
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return Settings{}, fmt.Errorf("settings: write: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return Settings{}, fmt.Errorf("settings: commit: %w", err)
	}
	return s, nil
}

// ValidationError marks a settings document rejected by Validate at a write
// boundary. Callers map it onto bad-request errors instead of internal
// failures.
type ValidationError struct{ Err error }

func (e *ValidationError) Error() string { return e.Err.Error() }
func (e *ValidationError) Unwrap() error { return e.Err }

// IsValidationError reports whether err is (or wraps) a *ValidationError.
func IsValidationError(err error) bool {
	var ve *ValidationError
	return errors.As(err, &ve)
}

// Update runs fn against the current document under one fileMu hold —
// load → fn → validate → write — so a handler-level read-modify-write can
// never lose another writer's concurrent change. fn receives the decoded
// document and returns the candidate to persist; returning an error aborts
// the write and passes the error through verbatim, so callers can carry
// their own domain errors across the transaction. A rejected candidate is
// wrapped in *ValidationError; load, fn, and write errors come back as-is.
// fn must not call Load/Save/Update (deadlock on fileMu) and must not retain
// the passed document beyond the call. The returned Settings is the
// persisted document.
func Update(path string, fn func(Settings) (Settings, error)) (Settings, error) {
	fileMu.Lock()
	defer fileMu.Unlock()
	current, err := load(path)
	if err != nil {
		return Settings{}, err
	}
	next, err := fn(current)
	if err != nil {
		return Settings{}, err
	}
	if err := next.Validate(); err != nil {
		return Settings{}, &ValidationError{Err: err}
	}
	return write(path, next)
}
