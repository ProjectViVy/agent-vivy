package provider

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cloudwego/eino/schema"

	"agent-vivy/internal/domain"
	"agent-vivy/sdk/port/providerprofile"
)

func TestDeepSeekRefKeyMissing(t *testing.T) {
	ref, err := NewCatalog(testDeepSeekVendor("https://api.deepseek.com")).For("deepseek")
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

// TestDeepSeekModelInfoMetadata pins the reference metadata shape for the
// default provider: deepseek-flash carries the published V4.1-Flash numbers
// and thinking support; deepseek-chat stays a non-thinking row; unknown ids
// remain all-zero (unknown, never free).
func TestDeepSeekModelInfoMetadata(t *testing.T) {
	ref, err := NewCatalog(testDeepSeekVendor("https://api.deepseek.com")).For("deepseek")
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
// other OpenAI-compatible vendors never receive the fields, because the
// deepseek-thinking capability is declared per endpoint.
func TestDeepSeekThinkingRequest(t *testing.T) {
	cases := []struct {
		name             string
		vendor           Vendor
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
			name: "openai gateway requests stay untouched", vendor: testOpenAIVendor(""), model: "gpt-4o", mode: domain.ThinkingModeOn,
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

			vendor := tc.vendor
			if vendor.Name == "" {
				vendor = testDeepSeekVendor(server.URL)
			}
			profile := testProfile(t, vendor)
			profile.EndpointClass = providerprofile.EndpointGateway
			host := routedHost(t, profile)
			cm := NewResolvingChatModel(host, NewCatalog(vendor), staticSpecSource{live: LiveSpec{
				Provider: vendor.Name, Model: tc.model, BaseURL: server.URL,
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
