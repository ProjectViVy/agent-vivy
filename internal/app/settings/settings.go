// Package settings holds the operator-managed model provider selection and
// network tool preferences for Vivy itself. These are user-facing
// preferences (which provider bundle is active, which default model, an
// optional OpenAI-compatible base URL, an optional API key overlay, and the
// preferred network_search provider). They live in an independent agent
// working directory (data/agent-home/settings.yaml) so the running species
// never rewrites its own production config.yaml and never touches the
// Journal (data/vivy.db).
//
// Secrets: committed config still holds env_key names only (D-010). This
// runtime settings document may additionally hold an optional plaintext
// api_key overlay (file mode 0600, data dir is gitignored runtime state); it
// is applied to the bundle's env_key environment variable at startup, and is
// never logged and never returned by the control plane. An empty api_key
// means "no overlay" — the environment variable stands.
// On startup, app overlays these values onto the validated config before the
// provider/model are built, so a save takes effect on the next launch (no
// live hot-swap of the running engine).
package settings

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// DefaultDir is the independent agent working directory under the process
// data root. It is not data/ and not the tenant Journal.
const DefaultDir = "data/agent-home"

// FileName is the settings document name inside DefaultDir.
const FileName = "settings.yaml"

// Valid provider bundle names V0 can activate.
const (
	ProviderOpenAI    = "openai"
	ProviderAnthropic = "anthropic"
	ProviderMock      = "mock"
)

// apiBasePattern bounds the base URL to http(s) absolute URLs. It carries a
// URL, not a secret, but is still validated before use.
var apiBasePattern = regexp.MustCompile(`^https?://[^\s/]+(:\d+)?(/.*)?$`)

// maxExecuteTimeoutSeconds mirrors config.maxExecuteTimeoutSeconds and the
// runtime hard cap (10m): a settings override above it would be silently
// clamped, so it is rejected here up front.
const maxExecuteTimeoutSeconds = 600

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
	// NetworkSearch overrides tools.network_search.provider from the
	// Settings UI; empty keeps the config value. Provider credentials stay
	// environment-only (D-010).
	NetworkSearch NetworkSearchSettings `yaml:"network_search"`
	// ExecuteMaxTimeoutSeconds overrides the execute/commandline ceiling
	// (config runtime.execute_max_timeout_seconds); 0 means "use config
	// value". Same bounds as config: 1–600, mirroring the runtime hard cap.
	ExecuteMaxTimeoutSeconds int `yaml:"execute_max_timeout_seconds"`
}

// NetworkSearchSettings is the UI-managed network_search preference.
// Provider credentials stay environment-only (D-010).
type NetworkSearchSettings struct {
	// Provider is the preferred provider name, or empty for automatic.
	Provider string `yaml:"provider"`
}

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

// Load reads and validates the settings document at path. A missing file is
// not an error: it returns the zero Settings so the config defaults stand.
func Load(path string) (Settings, error) {
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
	if err := s.Validate(); err != nil {
		return Settings{}, fmt.Errorf("settings: %s: %w", path, err)
	}
	return s, nil
}

// Validate rejects secret-shaped or structurally invalid values.
func (s Settings) Validate() error {
	switch s.Provider {
	case "":
		// empty => config default; allowed
	case ProviderOpenAI, ProviderAnthropic, ProviderMock:
	default:
		return fmt.Errorf("settings: provider %q unsupported; want openai, anthropic or mock", s.Provider)
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
	return nil
}

// Save writes the settings atomically to path, creating parent directories.
// It returns the persisted document so callers can echo it back.
func Save(path string, s Settings) (Settings, error) {
	if err := s.Validate(); err != nil {
		return Settings{}, err
	}
	if dir := filepath.Dir(path); dir != "" {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return Settings{}, fmt.Errorf("settings: create dir: %w", err)
		}
	}
	data, err := yaml.Marshal(s)
	if err != nil {
		return Settings{}, fmt.Errorf("settings: marshal: %w", err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return Settings{}, fmt.Errorf("settings: write: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return Settings{}, fmt.Errorf("settings: commit: %w", err)
	}
	return s, nil
}
