package provider

import (
	"testing"

	"agent-vivy/sdk/port/providerprofile"
)

// In-memory vendors for tests that exercise the Ref, Catalog and ModelHost
// paths. They mirror the shape of the embedded data without reading it, so a
// data edit cannot silently rewrite what these tests assert.

func testVendor(name, displayName, envKey, adapter, baseURL, defaultModel string, models []Model, capabilities ...string) Vendor {
	return Vendor{
		Name: name, DisplayName: displayName, EnvKey: envKey,
		Endpoints: []Endpoint{{
			Adapter: adapter, BaseURL: baseURL, DefaultModel: defaultModel,
			Models: models, Capabilities: capabilities,
		}},
		Provenance: Provenance{Source: "test", Entry: name, DerivedAt: "2026-09-18"},
	}
}

func testOpenAIVendor(baseURL string) Vendor {
	return testVendor("openai", "OpenAI", "OPENAI_API_KEY", AdapterOpenAICompletions, baseURL, "gpt-4o", []Model{
		{ID: "gpt-4o", ContextWindow: 128000, InputPerMTok: 2.5, OutputPerMTok: 10, SupportsImages: true},
		{ID: "gpt-4o-mini", ContextWindow: 128000, InputPerMTok: 0.15, OutputPerMTok: 0.6, SupportsImages: true},
		{ID: "gpt-3.5-turbo", ContextWindow: 16385, InputPerMTok: 0.5, OutputPerMTok: 1.5},
	})
}

func testClaudeVendor(baseURL string) Vendor {
	vendor := testVendor("anthropic", "Anthropic", "ANTHROPIC_API_KEY", AdapterAnthropicMessages, baseURL, "claude-sonnet-4-5", []Model{
		{ID: "claude-sonnet-4-5", ContextWindow: 200000, InputPerMTok: 3, OutputPerMTok: 15, SupportsImages: true, SupportsThinking: true},
	})
	vendor.Endpoints[0].SupportsPromptCaching = true
	return vendor
}

func testDeepSeekVendor(baseURL string) Vendor {
	return testVendor("deepseek", "DeepSeek", "DEEPSEEK_API_KEY", AdapterOpenAICompletions, baseURL, "deepseek-flash", []Model{
		{ID: "deepseek-flash", ContextWindow: 1000000, InputPerMTok: 0.30, OutputPerMTok: 1.20, SupportsImages: true, SupportsThinking: true},
		{ID: "deepseek-chat", ContextWindow: 128000, InputPerMTok: 0.28, OutputPerMTok: 0.42, SupportsImages: true},
	}, CapabilityDeepSeekThinking)
}

// testEndpoint returns the vendor's default endpoint or fails the test.
func testEndpoint(t *testing.T, vendor Vendor) Endpoint {
	t.Helper()
	endpoint, ok := vendor.DefaultEndpoint()
	if !ok {
		t.Fatalf("test vendor %q declares no default endpoint", vendor.Name)
	}
	return endpoint
}

// testRef resolves the vendor's default endpoint through the real catalog
// path, so tests exercise the same switch production uses.
func testRef(t *testing.T, vendor Vendor) Ref {
	t.Helper()
	ref, err := NewCatalog(vendor).RefForEndpoint(vendor, testEndpoint(t, vendor))
	if err != nil {
		t.Fatalf("RefForEndpoint(%q): %v", vendor.Name, err)
	}
	return ref
}

// testProfile projects the vendor's default endpoint into the declarative
// Profile shape the ModelHost compiles.
func testProfile(t *testing.T, vendor Vendor) providerprofile.Profile {
	t.Helper()
	return ProfileFromEndpoint(vendor, testEndpoint(t, vendor))
}
