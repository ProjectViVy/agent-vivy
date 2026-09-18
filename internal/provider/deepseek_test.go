package provider

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cloudwego/eino/schema"

	"agent-vivy/internal/domain"
	"agent-vivy/sdk/port/providerprofile"
)

func TestLoadBundleDeepSeekFixture(t *testing.T) {
	b, err := LoadBundle(filepath.Join(fixturesDir, "deepseek.yaml"))
	if err != nil {
		t.Fatalf("load deepseek bundle: %v", err)
	}
	if b.Name != "deepseek" || b.APIType != "openai" {
		t.Fatalf("name/api_type = %s/%s, want deepseek/openai", b.Name, b.APIType)
	}
	if b.EnvKey != "DEEPSEEK_API_KEY" {
		t.Fatalf("env_key = %q, want DEEPSEEK_API_KEY", b.EnvKey)
	}
	if b.DefaultModel != "deepseek-flash" || b.DefaultAPIBase != "https://api.deepseek.com" {
		t.Fatalf("defaults = %q/%q, want deepseek-flash/https://api.deepseek.com", b.DefaultModel, b.DefaultAPIBase)
	}
	if b.Backend != BackendEinoOpenAI {
		t.Fatalf("backend = %q, want %s", b.Backend, BackendEinoOpenAI)
	}
	if len(b.Models) != 6 {
		t.Fatalf("models = %d entries, want 6", len(b.Models))
	}
	assertProvenance(t, b, "deepseek")
}

func newDeepSeekTestBundle(baseURL string) Bundle {
	return Bundle{
		Name: "deepseek", APIType: "openai", EnvKey: "DEEPSEEK_API_KEY",
		DisplayName: "DeepSeek", DefaultModel: "deepseek-flash", DefaultAPIBase: baseURL,
		Backend: BackendEinoOpenAI, Models: []string{"deepseek-flash", "deepseek-chat"},
		Provenance: Provenance{Source: "test", Entry: "deepseek", DerivedAt: "2026-09-16"},
	}
}

func TestDeepSeekRefKeyMissing(t *testing.T) {
	b, err := LoadBundle(filepath.Join(fixturesDir, "deepseek.yaml"))
	if err != nil {
		t.Fatalf("load bundle: %v", err)
	}
	ref, err := NewCatalog(b).For("deepseek")
	if err != nil {
		t.Fatalf("catalog deepseek: %v", err)
	}
	_, err = ref.Model(context.Background(), ModelSpec{})
	var kme *KeyMissingError
	if !errors.As(err, &kme) {
		t.Fatalf("err = %v, want *KeyMissingError", err)
	}
	if kme.EnvKey != "DEEPSEEK_API_KEY" || kme.Provider != "deepseek" {
		t.Fatalf("structured error fields wrong: %+v", kme)
	}
}

// TestDeepSeekModelInfoMetadata pins the D9 reference metadata for the
// default provider: deepseek-flash carries the published V4.1-Flash numbers
// and thinking support; deepseek-chat stays a non-thinking row; unknown ids
// remain all-zero (unknown, never free).
func TestDeepSeekModelInfoMetadata(t *testing.T) {
	b, err := LoadBundle(filepath.Join(fixturesDir, "deepseek.yaml"))
	if err != nil {
		t.Fatalf("load bundle: %v", err)
	}
	ref, err := NewCatalog(b).For("deepseek")
	if err != nil {
		t.Fatalf("catalog deepseek: %v", err)
	}
	ctx := context.Background()

	info, err := ref.ModelInfo(ctx, "deepseek-flash")
	if err != nil {
		t.Fatalf("deepseek-flash info: %v", err)
	}
	if info.ContextWindow != 1000000 || info.InputPerMTokens != 0.30 || info.OutputPerMTokens != 1.20 {
		t.Fatalf("deepseek-flash info = %+v", info)
	}
	if !info.SupportsThinking || !info.SupportsImages {
		t.Fatalf("deepseek-flash must support thinking and images: %+v", info)
	}

	info, err = ref.ModelInfo(ctx, "deepseek-chat")
	if err != nil {
		t.Fatalf("deepseek-chat info: %v", err)
	}
	if info.SupportsThinking {
		t.Fatal("deepseek-chat must not be marked as a thinking model")
	}

	info, err = ref.ModelInfo(ctx, "deepseek-unknown-future")
	if err != nil {
		t.Fatalf("unknown info: %v", err)
	}
	if info.ContextWindow != 0 || info.InputPerMTokens != 0 || info.OutputPerMTokens != 0 || info.SupportsThinking {
		t.Fatalf("unknown model must be all-zero metadata, got %+v", info)
	}
}

// TestDeepSeekThinkingRequest pins the outbound DeepSeek request body for
// every thinking mode: "auto" and "on" send the documented canonical request
// (thinking enabled + reasoning_effort high), "off" disables thinking, and
// the raw model id always crosses the wire verbatim. Non-thinking models and
// non-deepseek OpenAI-compatible bundles never receive the fields.
func TestDeepSeekThinkingRequest(t *testing.T) {
	cases := []struct {
		name             string
		bundle           Bundle
		model            string
		mode             domain.ThinkingMode
		wantModel        string
		wantThinkingType string
		wantEffort       string
	}{
		{
			name: "auto sends canonical enabled and high", model: "deepseek-flash", mode: domain.ThinkingModeAuto,
			wantModel: "deepseek-flash", wantThinkingType: "enabled", wantEffort: "high",
		},
		{
			name: "on sends canonical enabled and high", model: "deepseek-flash", mode: domain.ThinkingModeOn,
			wantModel: "deepseek-flash", wantThinkingType: "enabled", wantEffort: "high",
		},
		{
			name: "off disables thinking", model: "deepseek-flash", mode: domain.ThinkingModeOff,
			wantModel: "deepseek-flash", wantThinkingType: "disabled", wantEffort: "",
		},
		{
			name: "non-thinking model never receives the fields", model: "deepseek-chat", mode: domain.ThinkingModeOn,
			wantModel: "deepseek-chat", wantThinkingType: "", wantEffort: "",
		},
		{
			name: "openai gateway requests stay untouched", bundle: newOpenAITestBundle(""), model: "gpt-4o", mode: domain.ThinkingModeOn,
			wantModel: "gpt-4o", wantThinkingType: "", wantEffort: "",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var outbound struct {
				Model           string         `json:"model"`
				Thinking        map[string]any `json:"thinking"`
				ReasoningEffort string         `json:"reasoning_effort"`
			}
			var outboundPath string
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				outboundPath = request.URL.Path
				body, _ := io.ReadAll(request.Body)
				if err := json.Unmarshal(body, &outbound); err != nil {
					t.Errorf("unmarshal outbound body: %v", err)
				}
				writer.Header().Set("Content-Type", "application/json")
				_, _ = io.WriteString(writer, `{"id":"chatcmpl-test","object":"chat.completion","created":1,"model":"test","choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`)
			}))
			defer server.Close()

			bundle := tc.bundle
			if bundle.Name == "" {
				bundle = newDeepSeekTestBundle(server.URL)
			}
			profile := ProfileFromBundle(bundle)
			profile.EndpointClass = providerprofile.EndpointGateway
			host := routedHost(t, profile)
			cm := NewResolvingChatModel(host, NewCatalog(bundle), staticSpecSource{live: LiveSpec{
				Provider: bundle.Name, Model: tc.model, BaseURL: server.URL,
				APIKey: "secret", Ready: true,
			}})
			ctx := domain.WithThinkingMode(context.Background(), tc.mode)
			if _, err := cm.Generate(ctx, []*schema.Message{schema.UserMessage("hi")}); err != nil {
				t.Fatalf("Generate() error = %v", err)
			}
			if outbound.Model != tc.wantModel {
				t.Fatalf("outbound model = %q, want raw %q", outbound.Model, tc.wantModel)
			}
			var gotThinking string
			if outbound.Thinking != nil {
				gotThinking, _ = outbound.Thinking["type"].(string)
			}
			if gotThinking != tc.wantThinkingType {
				t.Fatalf("outbound thinking.type = %q, want %q", gotThinking, tc.wantThinkingType)
			}
			if outbound.ReasoningEffort != tc.wantEffort {
				t.Fatalf("outbound reasoning_effort = %q, want %q", outbound.ReasoningEffort, tc.wantEffort)
			}
			if outboundPath != "/chat/completions" {
				t.Fatalf("outbound path = %q, want /chat/completions", outboundPath)
			}
		})
	}
}

// TestDeepSeekBaseURLPath asserts the configured base URL is exactly the
// documented DeepSeek host with no /v1 segment; the resolved request path is
// asserted independently by TestDeepSeekThinkingRequest.
func TestDeepSeekBaseURLPath(t *testing.T) {
	b, err := LoadBundle(filepath.Join(fixturesDir, "deepseek.yaml"))
	if err != nil {
		t.Fatalf("load bundle: %v", err)
	}
	if !strings.HasSuffix(b.DefaultAPIBase, "api.deepseek.com") || strings.Contains(b.DefaultAPIBase, "/v1") {
		t.Fatalf("default_api_base = %q, want https://api.deepseek.com (no /v1)", b.DefaultAPIBase)
	}
}
