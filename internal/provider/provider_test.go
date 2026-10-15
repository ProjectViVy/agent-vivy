package provider

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	"agent-vivy/internal/modelhost"
	"agent-vivy/sdk/port/providerprofile"
)

func TestOpenAIRefKeyMissing(t *testing.T) {
	ref := testRef(t, testOpenAIVendor("https://api.openai.com/v1"))
	_, err := ref.Model(context.Background(), ModelSpec{})
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
	m, err := testRef(t, testOpenAIVendor("https://api.openai.com/v1")).
		Model(context.Background(), ModelSpec{APIKey: "sk-fake-offline-test"})
	if err != nil {
		t.Fatalf("construct model: %v", err)
	}
	if m == nil {
		t.Fatal("model must not be nil")
	}
}

func TestOpenAIRefUsesSpecNotEnv(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "sk-from-env-must-not-win")
	t.Setenv(APIBaseEnvVar, "https://env-gateway.example/v1")

	m, err := testRef(t, testOpenAIVendor("https://api.openai.com/v1")).Model(context.Background(), ModelSpec{
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
	ref, err := NewCatalog(testClaudeVendor("https://api.anthropic.com")).For("anthropic")
	if err != nil {
		t.Fatalf("anthropic must resolve via the anthropic-messages adapter: %v", err)
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

func TestResolvingModelConformanceFailureTimeoutCancellationAndStreamError(t *testing.T) {
	t.Run("adapter failure becomes unavailable", func(t *testing.T) {
		vendor := testOpenAIVendor("https://network-must-not-run.invalid/v1")
		host := routedHost(t, testProfile(t, vendor))
		// A selection that projects as ready but carries no credential is the
		// post-readiness credential-disappearance case: the capability gate
		// passes and the adapter refuses to construct, which must be recorded
		// as UNAVAILABLE rather than as an unconfigured provider.
		chatModel := NewResolvingChatModel(host, NewCatalog(vendor), staticSpec{live: LiveSpec{
			Provider: vendor.Name, Model: "gpt-4o", Ready: true,
		}})
		_, err := chatModel.Generate(context.Background(), []*schema.Message{schema.UserMessage("hi")})
		var missing *KeyMissingError
		if !errors.As(err, &missing) {
			t.Fatalf("adapter failure = %v, want KeyMissingError", err)
		}
		if got := host.Statuses(AdapterOpenAICompletions, true)[0].State; got != modelhost.ProfileUnavailable {
			t.Fatalf("status after adapter failure = %s, want UNAVAILABLE", got)
		}
	})

	t.Run("timeout", func(t *testing.T) {
		chatModel, closeServer := blockingResolvingModel(t)
		defer closeServer()
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
		defer cancel()
		_, err := chatModel.Generate(ctx, []*schema.Message{schema.UserMessage("hi")})
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("timeout error = %v, want DeadlineExceeded", err)
		}
	})

	t.Run("cancellation", func(t *testing.T) {
		chatModel, closeServer := blockingResolvingModel(t)
		defer closeServer()
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err := chatModel.Generate(ctx, []*schema.Message{schema.UserMessage("hi")})
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("cancellation error = %v, want Canceled", err)
		}
	})

	t.Run("streaming upstream error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
			http.Error(writer, "stream failed", http.StatusBadRequest)
		}))
		defer server.Close()
		vendor := testOpenAIVendor(server.URL)
		chatModel := NewResolvingChatModel(routedHost(t, testProfile(t, vendor)), NewCatalog(vendor), staticSpec{live: LiveSpec{
			Provider: vendor.Name, Model: "gpt-4o", BaseURL: server.URL, APIKey: "secret", Ready: true,
		}})
		if stream, err := chatModel.Stream(context.Background(), []*schema.Message{schema.UserMessage("hi")}); err == nil {
			if stream != nil {
				stream.Close()
			}
			t.Fatal("streaming upstream failure returned nil")
		}
	})
}

func blockingResolvingModel(t *testing.T) (model.ToolCallingChatModel, func()) {
	t.Helper()
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		<-release
	}))
	vendor := testOpenAIVendor(server.URL)
	resolved := NewResolvingChatModel(routedHost(t, testProfile(t, vendor)), NewCatalog(vendor), staticSpec{live: LiveSpec{
		Provider: vendor.Name, Model: "gpt-4o", BaseURL: server.URL, APIKey: "secret", Ready: true,
	}})
	return resolved, func() {
		close(release)
		server.CloseClientConnections()
		server.Close()
	}
}

func TestMain(m *testing.M) {
	// Provider tests must never touch a real key from the environment.
	os.Unsetenv("DEEPSEEK_API_KEY")
	os.Unsetenv("OPENAI_API_KEY")
	os.Unsetenv("ANTHROPIC_API_KEY")
	os.Exit(m.Run())
}

// A user-registered provider is an endpoint the embedded catalog does not
// describe. The selection carries that endpoint itself, so the model must be
// built for the registered address without the catalog holding an entry for the
// name — and a deferred adapter must still fail closed.
func TestResolvingChatModelRunsARegisteredEndpoint(t *testing.T) {
	t.Run("requests reach the registered address", func(t *testing.T) {
		var path string
		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			path = request.URL.Path
			writer.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(writer, `{"id":"chatcmpl-test","object":"chat.completion","created":1,"model":"gw-chat","choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`)
		}))
		defer server.Close()

		endpoint := Endpoint{Adapter: AdapterOpenAICompletions, BaseURL: server.URL, DefaultModel: "gw-chat", Models: []Model{{ID: "gw-chat"}}}
		chatModel := NewResolvingChatModel(
			routedHost(t, testRegisteredEndpointProfile(t, AdapterOpenAICompletions)),
			// The catalog knows no "WXYG": before this path existed, every call
			// failed with `provider "WXYG": no embedded vendor data`.
			NewCatalog(),
			staticSpecSource{live: LiveSpec{
				Provider: "WXYG", Adapter: AdapterOpenAICompletions, Model: "gw-chat",
				BaseURL: server.URL, APIKey: "secret", Ready: true, Endpoint: endpoint,
			}},
		)
		if _, err := chatModel.Generate(context.Background(), []*schema.Message{schema.UserMessage("hi")}); err != nil {
			t.Fatalf("registered endpoint generate: %v", err)
		}
		if path != "/chat/completions" {
			t.Fatalf("outbound path = %q, want /chat/completions", path)
		}
	})

	t.Run("a deferred adapter still fails closed", func(t *testing.T) {
		endpoint := Endpoint{Adapter: AdapterOpenAIResponses, BaseURL: "https://network-must-not-run.invalid/v1", DefaultModel: "r-1", Models: []Model{{ID: "r-1"}}}
		// The compiled Generation seals adapters, so the ModelHost refuses the
		// deferred family before any credential or address is considered. The
		// Ref constructor refuses it too, whether or not a host is compiled
		// (see TestCatalogRefForSelectionFailsClosed).
		if _, err := NewCatalog().RefForSelection("WXYG", endpoint); !errors.Is(err, ErrAdapterDeferred) {
			t.Fatalf("RefForSelection error = %v, want ErrAdapterDeferred", err)
		}
		chatModel := NewResolvingChatModel(
			routedHost(t, testRegisteredEndpointProfile(t, AdapterOpenAICompletions)),
			NewCatalog(),
			staticSpecSource{live: LiveSpec{
				Provider: "WXYG", Adapter: AdapterOpenAIResponses, Model: "r-1",
				BaseURL: endpoint.BaseURL, APIKey: "secret", Ready: true, Endpoint: endpoint,
			}},
		)
		_, err := chatModel.Generate(context.Background(), []*schema.Message{schema.UserMessage("hi")})
		if !errors.Is(err, modelhost.ErrProfileNotFound) {
			t.Fatalf("deferred registered endpoint error = %v, want ErrProfileNotFound", err)
		}
	})
}

// testRegisteredEndpointProfile is the adapter Profile a user-registered
// endpoint compiles to: the registry declares an adapter, never a vendor, so the
// Profile identity is that adapter.
func testRegisteredEndpointProfile(t *testing.T, adapter string) providerprofile.Profile {
	t.Helper()
	profiles, err := AdapterProfiles()
	if err != nil {
		t.Fatalf("AdapterProfiles: %v", err)
	}
	for _, profile := range profiles {
		if profile.AdapterFamily == adapter {
			return profile
		}
	}
	t.Fatalf("no sealed Profile for adapter %q", adapter)
	return providerprofile.Profile{}
}

// TestOpenAIRefModelInfoMetadata covers the metadata path: known ids resolve
// reference pricing and image support; unknown ids stay zero (callers must
// treat unpriced as unknown, never free).
func TestOpenAIRefModelInfoMetadata(t *testing.T) {
	ref := testRef(t, testOpenAIVendor("https://api.openai.com/v1"))
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
