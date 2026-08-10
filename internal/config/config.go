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
	"os"
	"regexp"
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

const (
	defaultMaxContextBytes    = 256 << 10
	defaultMaxHistoryMessages = 64
)

// Config is the typed, validated configuration store.
type Config struct {
	Server    Server    `yaml:"server"`
	Storage   Storage   `yaml:"storage"`
	Providers Providers `yaml:"providers"`
	Runtime   Runtime   `yaml:"runtime"`
	Tools     Tools     `yaml:"tools"`
}

type Server struct {
	// Addr is the HTTP API + SSE listen address for the UI.
	Addr string `yaml:"addr"`
}

type Storage struct {
	// Backend selects the storage implementation. V0 supports exactly
	// "sqlite" (D-031).
	Backend string `yaml:"backend"`
	SQLite  SQLite `yaml:"sqlite"`
}

type SQLite struct {
	Path string `yaml:"path"`
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
}

type Tools struct {
	// Enabled lists the registered tool names. V0 ships exactly one
	// read-only auto-execute tool and one effectful approval-gated tool
	// (D-012).
	Enabled  []string `yaml:"enabled"`
	Approval Approval `yaml:"approval"`
}

type Approval struct {
	// Expiration bounds how long a pending approval stays valid.
	Expiration time.Duration `yaml:"-"`
	// expirationRaw carries the YAML string ("5m"); parsed in Validate.
	expirationRaw string
}

// toolsDoc mirrors the tools mapping with expiration kept as a raw
// string so that validation, not the decoder, owns duration parsing.
type toolsDoc struct {
	Enabled  []string `yaml:"enabled"`
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
	t.Approval.expirationRaw = doc.Approval.Expiration
	return nil
}

// Default returns the built-in configuration used when no config file is
// present. It mirrors config.example.yaml.
func Default() Config {
	return Config{
		Server:  Server{Addr: "127.0.0.1:8787"},
		Storage: Storage{Backend: "sqlite", SQLite: SQLite{Path: "data/vivy.db"}},
		Providers: Providers{
			Active:    "openai",
			BundleDir: "fixtures/provider",
			OpenAI:    Provider{EnvKey: "OPENAI_API_KEY", DefaultModel: "gpt-4o-mini"},
			Anthropic: Provider{EnvKey: "ANTHROPIC_API_KEY", DefaultModel: "claude-sonnet-4-5"},
		},
		Runtime: Runtime{
			Mock:                 false,
			StreamBuffer:         256,
			MaxEventPayloadBytes: 65536,
			MaxToolTurns:         defaultMaxToolTurns,
			MaxContextBytes:      defaultMaxContextBytes,
			MaxHistoryMessages:   defaultMaxHistoryMessages,
			MaxToolResultBytes:   32 << 10,
		},
		Tools: Tools{
			Enabled:  []string{"echo_info", "write_note", "list_notes", "read_note", "ask_user"},
			Approval: Approval{Expiration: 5 * time.Minute, expirationRaw: "5m"},
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

	if c.Storage.Backend != "sqlite" {
		return fmt.Errorf("storage.backend %q unsupported; V0 supports only sqlite", c.Storage.Backend)
	}
	if c.Storage.SQLite.Path == "" {
		return errors.New("storage.sqlite.path must not be empty")
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

	if len(c.Tools.Enabled) == 0 {
		return errors.New("tools.enabled must list at least one tool")
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

	return nil
}
