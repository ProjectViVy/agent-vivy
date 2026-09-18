package provider

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	"agent-vivy/internal/modelhost"
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
		profile := testProfile(t, vendor)
		profile.AdapterFamily = AdapterFamilyAnthropic
		host := routedHost(t, profile)
		chatModel := NewResolvingChatModel(host, NewCatalog(vendor), staticSpec{live: LiveSpec{
			Provider: vendor.Name, Model: "gpt-4o", APIKey: "secret", Ready: true,
		}})
		if _, err := chatModel.Generate(context.Background(), []*schema.Message{schema.UserMessage("hi")}); !errors.Is(err, ErrAdapterFamilyMismatch) {
			t.Fatalf("adapter failure = %v, want ErrAdapterFamilyMismatch", err)
		}
		if got := host.Statuses("openai", true)[0].State; got != modelhost.ProfileUnavailable {
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
