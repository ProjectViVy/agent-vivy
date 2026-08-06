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
storage:
  backend: sqlite
  sqlite:
    path: "tmp/vivy.db"
providers:
  active: anthropic
  openai:
    env_key: OPENAI_API_KEY
    default_model: gpt-4o-mini
  anthropic:
    env_key: ANTHROPIC_API_KEY
    default_model: claude-sonnet-4-5
runtime:
  mock: true
  stream_buffer: 16
  max_event_payload_bytes: 1024
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
	if cfg.Providers.Active != "anthropic" {
		t.Errorf("active = %q", cfg.Providers.Active)
	}
	if !cfg.Runtime.Mock || cfg.Runtime.StreamBuffer != 16 || cfg.Runtime.MaxEventPayloadBytes != 1024 {
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
			"backend: sqlite", "backend: postgres", 1),
		"bad active provider": strings.Replace(validDoc,
			"active: anthropic", "active: deepseek", 1),
		"bad expiration": strings.Replace(validDoc,
			"expiration: 2m", "expiration: soon", 1),
		"empty tools": strings.Replace(validDoc,
			"  enabled:\n    - echo_info\n    - write_note", "  enabled: []", 1),
	}
	for name, doc := range cases {
		if _, err := Load(writeConfig(t, doc)); err == nil {
			t.Errorf("%s: want error, got nil", name)
		}
	}
}
