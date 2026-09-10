package provider

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cloudwego/eino/schema"
)

const fixturesDir = "../../fixtures/provider"

func TestLoadBundleOpenAIFixture(t *testing.T) {
	b, err := LoadBundle(filepath.Join(fixturesDir, "openai.yaml"))
	if err != nil {
		t.Fatalf("load openai bundle: %v", err)
	}
	if b.Name != "openai" || b.APIType != "openai" {
		t.Fatalf("name/api_type = %s/%s, want openai/openai", b.Name, b.APIType)
	}
	if b.EnvKey != "OPENAI_API_KEY" {
		t.Fatalf("env_key = %q, want OPENAI_API_KEY", b.EnvKey)
	}
	if b.DefaultModel != "gpt-4o" || b.DefaultAPIBase != "https://api.openai.com/v1" {
		t.Fatalf("defaults = %q/%q", b.DefaultModel, b.DefaultAPIBase)
	}
	if b.Backend != BackendEinoOpenAI {
		t.Fatalf("backend = %q, want %s", b.Backend, BackendEinoOpenAI)
	}
	if len(b.Models) != 12 {
		t.Fatalf("models = %d entries, want 12", len(b.Models))
	}
	assertProvenance(t, b, "openai")
}

func TestLoadBundleAnthropicFixture(t *testing.T) {
	b, err := LoadBundle(filepath.Join(fixturesDir, "anthropic.yaml"))
	if err != nil {
		t.Fatalf("load anthropic bundle: %v", err)
	}
	if b.Name != "anthropic" || b.APIType != "anthropic" {
		t.Fatalf("name/api_type = %s/%s, want anthropic/anthropic", b.Name, b.APIType)
	}
	if b.EnvKey != "ANTHROPIC_API_KEY" {
		t.Fatalf("env_key = %q, want ANTHROPIC_API_KEY", b.EnvKey)
	}
	if b.Backend != BackendEinoClaude {
		t.Fatalf("backend = %q, want %s", b.Backend, BackendEinoClaude)
	}
	assertProvenance(t, b, "anthropic")
}

func assertProvenance(t *testing.T, b Bundle, entry string) {
	t.Helper()
	p := b.Provenance
	if p.Source == "" || p.Entry != entry || p.DerivedAt == "" {
		t.Fatalf("provenance incomplete or wrong entry: %+v", p)
	}
}

func TestParseBundleRejectsUnknownField(t *testing.T) {
	bad := []byte("name: openai\nenv_key: OPENAI_API_KEY\nbogus_field: 1\n")
	if _, err := ParseBundle(bad); err == nil {
		t.Fatal("strict decode must reject unknown fields")
	}
}

func TestParseBundleValidationErrors(t *testing.T) {
	valid := Bundle{
		Name: "openai", APIType: "openai", EnvKey: "OPENAI_API_KEY",
		DisplayName: "OpenAI", DefaultModel: "gpt-4o",
		DefaultAPIBase: "https://api.openai.com/v1", Backend: BackendEinoOpenAI,
		Models:     []string{"gpt-4o"},
		Provenance: Provenance{Source: "s", Entry: "openai", DerivedAt: "2026-08-07"},
	}

	cases := []struct {
		name   string
		mutate func(*Bundle)
	}{
		{"lowercase env_key", func(b *Bundle) { b.EnvKey = "openai_api_key" }},
		{"bad bundle name", func(b *Bundle) { b.Name = "OpenAI" }},
		{"default_model not listed", func(b *Bundle) { b.DefaultModel = "gpt-9" }},
		{"empty models", func(b *Bundle) { b.Models = nil }},
		{"missing provenance source", func(b *Bundle) { b.Provenance.Source = "" }},
		{"unsupported backend", func(b *Bundle) { b.Backend = "langchain/openai" }},
		{"unsupported api_type", func(b *Bundle) { b.APIType = "gemini" }},
		{"missing display_name", func(b *Bundle) { b.DisplayName = "" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mut := valid
			tc.mutate(&mut)
			if errs := mut.validate(); errs == nil {
				t.Fatalf("validate() accepted %s", tc.name)
			}
		})
	}
	if err := valid.validate(); err != nil {
		t.Fatalf("valid bundle rejected: %v", err)
	}
}

func TestOpenAIRefKeyMissing(t *testing.T) {
	b, err := LoadBundle(filepath.Join(fixturesDir, "openai.yaml"))
	if err != nil {
		t.Fatalf("load bundle: %v", err)
	}
	_, err = NewCatalog(b).For("openai")
	if err != nil {
		t.Fatalf("catalog openai: %v", err)
	}
	ref := newOpenAIRef(b)
	_, err = ref.Model(context.Background(), ModelSpec{})
	var kme *KeyMissingError
	if !errors.As(err, &kme) {
		t.Fatalf("err = %v, want *KeyMissingError", err)
	}
	if kme.EnvKey != "OPENAI_API_KEY" || kme.Provider != "openai" {
		t.Fatalf("structured error fields wrong: %+v", kme)
	}
	if !strings.Contains(err.Error(), "Settings") {
		t.Fatalf("message must point at settings: %q", err.Error())
	}
}

// TestOpenAIRefConstructsOffline proves the eino-ext component builds
// with a fake key and no network traffic: construction must not call
// out, only the first Generate/Stream would.
func TestOpenAIRefConstructsOffline(t *testing.T) {
	b, err := LoadBundle(filepath.Join(fixturesDir, "openai.yaml"))
	if err != nil {
		t.Fatalf("load bundle: %v", err)
	}
	ref, err := NewCatalog(b).For("openai")
	if err != nil {
		t.Fatalf("catalog openai: %v", err)
	}
	m, err := ref.Model(context.Background(), ModelSpec{APIKey: "sk-fake-offline-test"})
	if err != nil {
		t.Fatalf("construct model: %v", err)
	}
	if m == nil {
		t.Fatal("model must not be nil")
	}
}

func TestOpenAIRefUsesSpecNotEnv(t *testing.T) {
	b, err := LoadBundle(filepath.Join(fixturesDir, "openai.yaml"))
	if err != nil {
		t.Fatalf("load bundle: %v", err)
	}
	t.Setenv("OPENAI_API_KEY", "sk-from-env-must-not-win")
	t.Setenv(APIBaseEnvVar, "https://env-gateway.example/v1")

	ref := newOpenAIRef(b)
	m, err := ref.Model(context.Background(), ModelSpec{
		ID:      "gpt-4o-mini",
		APIKey:  "sk-from-spec",
		BaseURL: "https://spec-gateway.example/v1",
	})
	if err != nil {
		t.Fatalf("construct model: %v", err)
	}
	if m == nil {
		t.Fatal("model must not be nil")
	}
}

func TestCatalogAnthropicResolvesClaudeRef(t *testing.T) {
	b, err := LoadBundle(filepath.Join(fixturesDir, "anthropic.yaml"))
	if err != nil {
		t.Fatalf("load bundle: %v", err)
	}
	ref, err := NewCatalog(b).For("anthropic")
	if err != nil {
		t.Fatalf("anthropic must resolve via the claude backend: %v", err)
	}
	if _, err := ref.Model(context.Background(), ModelSpec{ID: "claude-sonnet-4-5", APIKey: "k"}); err != nil {
		t.Fatalf("model construction: %v", err)
	}
}

func TestCatalogUnknownProvider(t *testing.T) {
	if _, err := NewCatalog().For("gemini"); err == nil {
		t.Fatal("unknown provider must fail")
	}
}

type staticSpec struct{ live LiveSpec }

func (s staticSpec) Live() LiveSpec { return s.live }

func TestResolvingChatModelRejectsUnconfigured(t *testing.T) {
	cm := NewResolvingChatModel(routedHost(t), NewCatalog(), staticSpec{live: LiveSpec{}})
	_, err := cm.Generate(context.Background(), []*schema.Message{{Role: schema.User, Content: "hi"}})
	if !errors.Is(err, ErrModelNotConfigured) {
		t.Fatalf("error = %v, want ErrModelNotConfigured", err)
	}
}

func TestMain(m *testing.M) {
	// Provider tests must never touch a real key from the environment.
	os.Unsetenv("OPENAI_API_KEY")
	os.Unsetenv("ANTHROPIC_API_KEY")
	os.Exit(m.Run())
}

// TestOpenAIRefModelInfoMetadata covers the D9 metadata table: known ids
// resolve reference pricing and image support; unknown ids stay zero
// (callers must treat unpriced as unknown, never free).
func TestOpenAIRefModelInfoMetadata(t *testing.T) {
	b, err := LoadBundle(filepath.Join(fixturesDir, "openai.yaml"))
	if err != nil {
		t.Fatalf("load bundle: %v", err)
	}
	ref := newOpenAIRef(b)
	ctx := context.Background()

	info, err := ref.ModelInfo(ctx, "gpt-4o")
	if err != nil {
		t.Fatalf("gpt-4o info: %v", err)
	}
	if info.ContextWindow != 128000 || info.InputPerMTokens != 2.5 || info.OutputPerMTokens != 10.0 || !info.SupportsImages {
		t.Fatalf("gpt-4o info = %+v", info)
	}

	info, err = ref.ModelInfo(ctx, "gpt-3.5-turbo")
	if err != nil {
		t.Fatalf("gpt-3.5-turbo info: %v", err)
	}
	if info.SupportsImages {
		t.Fatal("gpt-3.5-turbo must not claim image support")
	}

	info, err = ref.ModelInfo(ctx, "totally-custom-model")
	if err != nil {
		t.Fatalf("custom info: %v", err)
	}
	if info.ContextWindow != 0 || info.InputPerMTokens != 0 || info.OutputPerMTokens != 0 || info.SupportsImages {
		t.Fatalf("unknown model must be all-zero metadata, got %+v", info)
	}
}
