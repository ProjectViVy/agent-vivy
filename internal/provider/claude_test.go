package provider

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cloudwego/eino/schema"
)

const anthropicTestReply = `{"id":"msg_test","type":"message","role":"assistant","model":"claude-sonnet-4-5","content":[{"type":"text","text":"ok"}],"stop_reason":"end_turn","stop_sequence":null,"usage":{"input_tokens":1,"output_tokens":1}}`

func newClaudeTestBundle(defaultAPIBase string) Bundle {
	return Bundle{
		Name: "anthropic", APIType: "anthropic", EnvKey: "ANTHROPIC_API_KEY",
		DisplayName: "Anthropic", DefaultModel: "claude-sonnet-4-5",
		DefaultAPIBase: defaultAPIBase, Backend: BackendEinoClaude,
		SupportsPromptCaching: true,
		Models:                []string{"claude-sonnet-4-5"},
		Provenance:            Provenance{Source: "s", Entry: "anthropic", DerivedAt: "2026-08-07"},
	}
}

func TestClaudeRefRequiresKeyBeforeConstruction(t *testing.T) {
	// Poison the environment: the ref must refuse on the empty spec key
	// before any SDK construction, so the ANTHROPIC_API_KEY fallback path
	// is unreachable (D-010).
	t.Setenv("ANTHROPIC_API_KEY", "env-key-must-not-rescue")
	ref := newClaudeRef(newClaudeTestBundle("https://api.anthropic.com"))
	_, err := ref.Model(context.Background(), ModelSpec{ID: "claude-sonnet-4-5", BaseURL: "https://api.anthropic.com"})
	var keyErr *KeyMissingError
	if err == nil || !errors.As(err, &keyErr) {
		t.Fatalf("err = %v, want KeyMissingError raised before SDK construction", err)
	}
}

// TestClaudeRefOutboundProtocol drives a real Generate against a local
// Anthropic-shaped server and asserts the outbound contract: the spec's key
// rides x-api-key (never ANTHROPIC_API_KEY from the environment), the
// outbound model field is the spec id (never ANTHROPIC_MODEL), and the
// protocol-required max_tokens floor is sent.
func TestClaudeRefOutboundProtocol(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "from-env")
	t.Setenv("ANTHROPIC_MODEL", "from-env-model")

	var gotPath, gotKey string
	var body struct {
		Model     string `json:"model"`
		MaxTokens int    `json:"max_tokens"`
	}
	var rawBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotKey = r.Header.Get("X-Api-Key")
		rawBody, _ = io.ReadAll(r.Body)
		_ = json.Unmarshal(rawBody, &body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, anthropicTestReply)
	}))
	defer srv.Close()

	ref := newClaudeRef(newClaudeTestBundle(srv.URL))
	cm, err := ref.Model(context.Background(), ModelSpec{ID: "claude-sonnet-4-5", APIKey: "spec-key", BaseURL: srv.URL})
	if err != nil {
		t.Fatalf("model: %v", err)
	}
	resp, err := cm.Generate(context.Background(), []*schema.Message{schema.UserMessage("hi")})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if resp.Content != "ok" {
		t.Fatalf("content = %q, want ok", resp.Content)
	}
	if gotPath != "/v1/messages" {
		t.Fatalf("path = %q, want /v1/messages", gotPath)
	}
	if gotKey != "spec-key" {
		t.Fatalf("x-api-key = %q, want the spec key (env fallback must not win)", gotKey)
	}
	if body.Model != "claude-sonnet-4-5" {
		t.Fatalf("outbound model = %q, want the spec id (env fallback must not win)", body.Model)
	}
	if body.MaxTokens != claudeDefaultMaxTokens {
		t.Fatalf("max_tokens = %d, want protocol floor %d", body.MaxTokens, claudeDefaultMaxTokens)
	}
	// The bundle enables prompt caching, so an ephemeral auto breakpoint
	// must ride the request (§8.5 acceptance: caching 生效).
	if !strings.Contains(string(rawBody), `"cache_control":{"type":"ephemeral"}`) {
		t.Fatalf("outbound body must carry an ephemeral cache breakpoint, got %s", rawBody)
	}
}

func TestClaudeRefFallsBackToBundleDefaults(t *testing.T) {
	var body struct {
		Model string `json:"model"`
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, anthropicTestReply)
	}))
	defer srv.Close()

	ref := newClaudeRef(newClaudeTestBundle(srv.URL))
	cm, err := ref.Model(context.Background(), ModelSpec{APIKey: "k"})
	if err != nil {
		t.Fatalf("model: %v", err)
	}
	if _, err := cm.Generate(context.Background(), []*schema.Message{schema.UserMessage("hi")}); err != nil {
		t.Fatalf("generate: %v", err)
	}
	if body.Model != "claude-sonnet-4-5" {
		t.Fatalf("outbound model = %q, want the bundle default_model", body.Model)
	}
}

func TestClaudeModelInfoKnownAndUnknown(t *testing.T) {
	ref := newClaudeRef(newClaudeTestBundle("https://api.anthropic.com"))
	info, err := ref.ModelInfo(context.Background(), "claude-sonnet-4-5")
	if err != nil {
		t.Fatalf("model info: %v", err)
	}
	if info.ContextWindow != 200000 || info.InputPerMTokens != 3.0 || info.OutputPerMTokens != 15.0 || !info.SupportsImages {
		t.Fatalf("known meta = %+v", info)
	}
	unknown, err := ref.ModelInfo(context.Background(), "claude-opus-4-6")
	if err != nil {
		t.Fatalf("model info: %v", err)
	}
	if unknown.ContextWindow != 0 || unknown.InputPerMTokens != 0 || unknown.OutputPerMTokens != 0 {
		t.Fatalf("unknown meta must be zero-valued, got %+v", unknown)
	}
	if empty, _ := ref.ModelInfo(context.Background(), ""); empty.ID != "claude-sonnet-4-5" {
		t.Fatalf("empty id must fall back to bundle default, got %+v", empty)
	}
}

func TestCatalogForClaudeBackend(t *testing.T) {
	c := NewCatalog(newClaudeTestBundle("https://api.anthropic.com"))
	ref, err := c.For("anthropic")
	if err != nil {
		t.Fatalf("catalog for: %v", err)
	}
	if ref.Name() != "anthropic" {
		t.Fatalf("ref name = %q", ref.Name())
	}
	cm, err := ref.Model(context.Background(), ModelSpec{ID: "claude-sonnet-4-5", APIKey: "k"})
	if err != nil {
		t.Fatalf("model construction: %v", err)
	}
	if cm == nil {
		t.Fatal("model must construct offline (no network before Generate)")
	}
}
