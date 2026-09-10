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

	"agent-vivy/internal/modelhost"
	"agent-vivy/sdk/port/providerprofile"
)

func routedHost(t *testing.T, profiles ...providerprofile.Profile) *modelhost.Host {
	t.Helper()
	host, err := modelhost.New(profiles, modelhost.Capabilities{
		AdapterFamilyOpenAICompatible: modelhost.CapabilitySupported,
		AdapterFamilyAnthropic:        modelhost.CapabilitySupported,
	})
	if err != nil {
		t.Fatalf("modelhost.New: %v", err)
	}
	return host
}

func TestEveryModelCallUsesModelHost(t *testing.T) {
	bundle := newOpenAITestBundle("https://network-must-not-run.invalid/v1")
	model := NewResolvingChatModel(nil, NewCatalog(bundle), staticSpecSource{live: LiveSpec{
		Provider: bundle.Name, Model: "gpt-4o", APIKey: "secret", Ready: true,
	}})
	_, err := model.Generate(context.Background(), []*schema.Message{schema.UserMessage("hi")})
	if !errors.Is(err, modelhost.ErrHostRequired) {
		t.Fatalf("Generate() error = %v, want ErrHostRequired", err)
	}
}

func TestModelHostRejectsProfileAdapterMismatch(t *testing.T) {
	bundle := newOpenAITestBundle("https://network-must-not-run.invalid/v1")
	profile := ProfileFromBundle(bundle)
	profile.AdapterFamily = AdapterFamilyAnthropic
	host := routedHost(t, profile)
	model := NewResolvingChatModel(host, NewCatalog(bundle), staticSpecSource{live: LiveSpec{
		Provider: bundle.Name, Model: "gpt-4o", APIKey: "secret", Ready: true,
	}})
	_, err := model.Generate(context.Background(), []*schema.Message{schema.UserMessage("hi")})
	if !errors.Is(err, ErrAdapterFamilyMismatch) {
		t.Fatalf("Generate() error = %v, want ErrAdapterFamilyMismatch", err)
	}
}

func TestModelHostPreservesOpenAIGatewayRawModelID(t *testing.T) {
	var outbound struct {
		Model string `json:"model"`
	}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		body, _ := io.ReadAll(request.Body)
		_ = json.Unmarshal(body, &outbound)
		writer.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(writer, `{"id":"chatcmpl-test","object":"chat.completion","created":1,"model":"test","choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`)
	}))
	defer server.Close()

	bundle := newOpenAITestBundle(server.URL)
	profile := ProfileFromBundle(bundle)
	profile.EndpointClass = providerprofile.EndpointGateway
	profile.ModelIDs = []string{"anthropic/claude-sonnet-4"}
	host := routedHost(t, profile)
	model := NewResolvingChatModel(host, NewCatalog(bundle), staticSpecSource{live: LiveSpec{
		Provider: bundle.Name, Model: "anthropic/claude-sonnet-4", BaseURL: server.URL,
		APIKey: "secret", Ready: true,
	}})
	if _, err := model.Generate(context.Background(), []*schema.Message{schema.UserMessage("hi")}); err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if outbound.Model != "anthropic/claude-sonnet-4" {
		t.Fatalf("outbound model = %q, want exact raw gateway ID", outbound.Model)
	}
}

func newOpenAITestBundle(baseURL string) Bundle {
	return Bundle{
		Name: "openai", APIType: "openai", EnvKey: "OPENAI_API_KEY",
		DisplayName: "OpenAI", DefaultModel: "gpt-4o", DefaultAPIBase: baseURL,
		Backend: BackendEinoOpenAI, Models: []string{"gpt-4o"},
		Provenance: Provenance{Source: "test", Entry: "openai", DerivedAt: "2026-09-10"},
	}
}
