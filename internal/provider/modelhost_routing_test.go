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
	host, err := modelhost.New(profiles, AdapterFamilies())
	if err != nil {
		t.Fatalf("modelhost.New: %v", err)
	}
	return host
}

func TestEveryModelCallUsesModelHost(t *testing.T) {
	vendor := testOpenAIVendor("https://network-must-not-run.invalid/v1")
	model := NewResolvingChatModel(nil, NewCatalog(vendor), staticSpecSource{live: LiveSpec{
		Provider: vendor.Name, Model: "gpt-4o", APIKey: "secret", Ready: true,
	}})
	_, err := model.Generate(context.Background(), []*schema.AgenticMessage{testUser("hi")})
	if !errors.Is(err, modelhost.ErrHostRequired) {
		t.Fatalf("Generate() error = %v, want ErrHostRequired", err)
	}
}

func TestUncompiledProfileFailsBeforeCredentialState(t *testing.T) {
	vendor := testOpenAIVendor("https://network-must-not-run.invalid/v1")
	model := NewResolvingChatModel(routedHost(t), NewCatalog(vendor), staticSpecSource{live: LiveSpec{
		Provider: vendor.Name, Model: "gpt-4o", Ready: false,
	}})
	_, err := model.Generate(context.Background(), []*schema.AgenticMessage{testUser("hi")})
	if !errors.Is(err, modelhost.ErrProfileNotFound) {
		t.Fatalf("Generate() error = %v, want ErrProfileNotFound before credential evaluation", err)
	}
}

// A Generation that does not compile the adapter a vendor endpoint speaks
// cannot execute that endpoint: the mismatch is now a missing Profile, not a
// family disagreement, and it still fails before any network call.
func TestModelHostRejectsUncompiledAdapterFamily(t *testing.T) {
	vendor := testClaudeVendor("https://network-must-not-run.invalid")
	model := NewResolvingChatModel(routedHost(t), NewCatalog(vendor), staticSpecSource{live: LiveSpec{
		Provider: vendor.Name, Model: "claude-sonnet-4-5", APIKey: "secret", Ready: true,
	}})
	_, err := model.Generate(context.Background(), []*schema.AgenticMessage{testUser("hi")})
	if !errors.Is(err, modelhost.ErrProfileNotFound) {
		t.Fatalf("Generate() error = %v, want ErrProfileNotFound", err)
	}
}

// A Profile whose family is not in the sealed capability map cannot be
// compiled at all, so data can never widen the executable set.
func TestModelHostRejectsUnsealedProfileFamily(t *testing.T) {
	profile := testProfile(t, testOpenAIVendor("https://network-must-not-run.invalid/v1"))
	profile.AdapterFamily = "gemini-generate-content"
	if _, err := modelhost.New([]providerprofile.Profile{profile}, AdapterFamilies()); err == nil {
		t.Fatal("a Profile naming an unsealed adapter family must not compile")
	}
}

// A Profile for a family this build does not seal cannot compile at all, so a
// retired family can never become executable through data.
func TestModelHostRejectsRetiredFamilyProfile(t *testing.T) {
	profile := testProfile(t, testDeepSeekVendor("https://api.deepseek.com"))
	profile.ID = "openai-responses"
	profile.AdapterFamily = "openai-responses"
	if _, err := modelhost.New([]providerprofile.Profile{profile}, AdapterFamilies()); err == nil {
		t.Fatal("a Profile naming a retired adapter family must not compile")
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

	vendor := testOpenAIVendor(server.URL)
	profile := testProfile(t, vendor)
	profile.EndpointClass = providerprofile.EndpointGateway
	profile.ModelIDs = []string{"anthropic/claude-sonnet-4"}
	host := routedHost(t, profile)
	model := NewResolvingChatModel(host, NewCatalog(vendor), staticSpecSource{live: LiveSpec{
		Provider: vendor.Name, Model: "anthropic/claude-sonnet-4", BaseURL: server.URL,
		APIKey: "secret", Ready: true,
	}})
	if _, err := model.Generate(context.Background(), []*schema.AgenticMessage{testUser("hi")}); err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if outbound.Model != "anthropic/claude-sonnet-4" {
		t.Fatalf("outbound model = %q, want exact raw gateway ID", outbound.Model)
	}
}
