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
)

func TestUserDefinedProviderRoutesThroughSealedAdapter(t *testing.T) {
	var path, authorization, modelID string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		authorization = r.Header.Get("Authorization")
		var request struct {
			Model string `json:"model"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Errorf("decode OpenAI-compatible request: %v", err)
		}
		modelID = request.Model
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, openAITestReply)
	}))
	defer server.Close()
	baseURL := server.URL + "/v1"

	model := NewResolvingChatModel(
		testModelHost(t, testOpenAIVendor(server.URL)),
		NewCatalog(),
		staticSpecSource{live: LiveSpec{
			Provider: "Gemini local gateway", Adapter: AdapterOpenAICompletions,
			Model: "gemini-3.8-flash-tiered", BaseURL: baseURL,
			APIKey: "registry-key", Ready: true, UserDefined: true,
		}},
	)
	response, err := model.Generate(context.Background(), []*schema.AgenticMessage{testUser("Hello")})
	if err != nil {
		t.Fatalf("Generate custom provider: %v", err)
	}
	if got := agenticMessageText(response); got != "ok" {
		t.Fatalf("response = %q, want %q", got, "ok")
	}
	if path != "/v1/chat/completions" || authorization != "Bearer registry-key" || modelID != "gemini-3.8-flash-tiered" {
		t.Fatalf("request path=%q authorization=%q model=%q", path, authorization, modelID)
	}
}

func TestUserDefinedProviderWithoutKeyFailsBeforeGateway(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, openAITestReply)
	}))
	defer server.Close()

	model := NewResolvingChatModel(
		testModelHost(t, testOpenAIVendor(server.URL)),
		NewCatalog(),
		staticSpecSource{live: LiveSpec{
			Provider: "Gemini local gateway", Adapter: AdapterOpenAICompletions,
			Model: "gemini-3.8-flash-tiered", BaseURL: server.URL,
			Ready: false, UserDefined: true,
		}},
	)
	_, err := model.Generate(context.Background(), []*schema.AgenticMessage{testUser("Hello")})
	var missing *KeyMissingError
	if !errors.As(err, &missing) || missing.Provider != "Gemini local gateway" || missing.EnvKey != "" {
		t.Fatalf("Generate without custom key error = %v, want missing custom credential", err)
	}
	if calls != 0 {
		t.Fatalf("gateway received %d requests without a configured key", calls)
	}
}

func TestD14ResolvingCacheKeepsCredentialRotationAndReadiness(t *testing.T) {
	var auth []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth = append(auth, r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"id":"reply","object":"chat.completion","model":"gpt-4o","choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}]}`)
	}))
	defer server.Close()
	vendor := testOpenAIVendor(server.URL)
	firstSpec := LiveSpec{Provider: vendor.Name, Model: "gpt-4o", BaseURL: server.URL, APIKey: "key-A", Ready: true}
	model := NewResolvingChatModel(testModelHost(t, vendor), NewCatalog(vendor), staticSpecSource{live: firstSpec}).(*resolvingChatModel)
	ctx := context.Background()
	first, err := model.inner(ctx)
	if err != nil {
		t.Fatal(err)
	}
	again, err := model.inner(ctx)
	if err != nil || first != again {
		t.Fatal("unchanged exact model selection did not reuse the cache")
	}
	if _, err := model.Generate(ctx, []*schema.AgenticMessage{testUser("hi")}); err != nil {
		t.Fatal(err)
	}
	rotated := firstSpec
	rotated.APIKey = "key-B"
	model.src = staticSpecSource{live: rotated}
	second, err := model.inner(ctx)
	if err != nil || first == second {
		t.Fatal("same-length credential rotation reused an old model")
	}
	if _, err := model.Generate(ctx, []*schema.AgenticMessage{testUser("hi")}); err != nil {
		t.Fatal(err)
	}
	if len(auth) != 2 || auth[0] != "Bearer key-A" || auth[1] != "Bearer key-B" {
		t.Fatal("outbound authorization did not follow current credential selection")
	}
	rotated.Ready = false
	model.src = staticSpecSource{live: rotated}
	_, err = model.inner(ctx)
	var missing *KeyMissingError
	if !errors.As(err, &missing) {
		t.Fatalf("cached model bypassed missing credential state: %v", err)
	}
	if len(auth) != 2 {
		t.Fatal("unready selection reached the provider")
	}
}
