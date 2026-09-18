package provider

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestCatalogForResolvesDefaultEndpoint(t *testing.T) {
	vendor := testDeepSeekVendor("https://api.deepseek.com")
	ref, err := NewCatalog(vendor).For("deepseek")
	if err != nil {
		t.Fatalf("For: %v", err)
	}
	if ref.Name() != "deepseek" {
		t.Fatalf("ref name = %q", ref.Name())
	}
	info, err := ref.ModelInfo(context.Background(), "")
	if err != nil {
		t.Fatalf("ModelInfo: %v", err)
	}
	if info.ID != "deepseek-flash" || info.Provider != "deepseek" {
		t.Fatalf("empty model id must resolve to the endpoint default, got %+v", info)
	}
}

// A deferred adapter is sealed but unimplemented: reaching it must fail
// closed with an explanation instead of silently using another adapter.
func TestCatalogRefusesDeferredAdapter(t *testing.T) {
	vendor := testVendor("openai", "OpenAI", "OPENAI_API_KEY", AdapterOpenAIResponses, "https://api.openai.com/v1", "gpt-5", []Model{{ID: "gpt-5"}})
	_, err := NewCatalog(vendor).For("openai")
	if err == nil {
		t.Fatal("the openai-responses adapter must not be executable")
	}
	if !strings.Contains(err.Error(), AdapterOpenAIResponses) {
		t.Fatalf("error %q must name the deferred adapter", err)
	}
}

// A sealed family resolves to the build-owned adapter, independent of any
// vendor: the address, the credential and the model id all come from the spec.
func TestCatalogAdapterResolvesSealedFamily(t *testing.T) {
	for _, family := range []string{AdapterOpenAICompletions, AdapterAnthropicMessages} {
		ref, err := NewCatalog().Adapter(family)
		if err != nil {
			t.Fatalf("Adapter(%q): %v", family, err)
		}
		if ref.Name() != family {
			t.Fatalf("adapter Ref name = %q, want the family %q", ref.Name(), family)
		}
	}
}

// A deferred family is sealed but unimplemented, and a family that is not in
// the table is unknown: the two failures are distinct and neither falls back to
// a substitute.
func TestCatalogAdapterRefusesDeferredAndUnknownFamilies(t *testing.T) {
	_, err := NewCatalog().Adapter(AdapterOpenAIResponses)
	if !errors.Is(err, ErrAdapterDeferred) {
		t.Fatalf("Adapter(openai-responses) error = %v, want ErrAdapterDeferred", err)
	}
	if errors.Is(err, ErrAdapterUnknown) {
		t.Fatal("a deferred family must not be reported as unknown")
	}
	if !strings.Contains(err.Error(), AdapterOpenAIResponses) {
		t.Fatalf("error %q must name the deferred adapter", err)
	}
	_, err = NewCatalog().Adapter("nope")
	if !errors.Is(err, ErrAdapterUnknown) {
		t.Fatalf("Adapter(nope) error = %v, want ErrAdapterUnknown", err)
	}
}

// Two vendors speaking the same family share one adapter but keep their own
// identity: their own environment key and their own declared address. This is
// the case the vendor-sealed design could not express — DeepSeek's
// anthropic-messages endpoint needed a vendor of its own.
func TestCatalogRefForEndpointCarriesVendorIdentity(t *testing.T) {
	deepseek := testDeepSeekVendor("https://api.deepseek.com")
	deepseek.Endpoints = append(deepseek.Endpoints, Endpoint{
		Adapter: AdapterAnthropicMessages, BaseURL: "https://api.deepseek.com/anthropic",
		DefaultModel: "deepseek-chat", Models: []Model{{ID: "deepseek-chat"}},
	})
	catalog := NewCatalog(deepseek, testClaudeVendor("https://api.anthropic.com"))

	for _, test := range []struct {
		vendor   string
		baseURL  string
		wantEnv  string
		wantBase string
	}{
		{"deepseek", "https://api.deepseek.com", "DEEPSEEK_API_KEY", "https://api.deepseek.com"},
		{"deepseek", "https://api.deepseek.com/anthropic", "DEEPSEEK_API_KEY", "https://api.deepseek.com/anthropic"},
		{"anthropic", "", "ANTHROPIC_API_KEY", "https://api.anthropic.com"},
	} {
		endpoint, vendor, err := catalog.EndpointForVendor(test.vendor, test.baseURL)
		if err != nil {
			t.Fatalf("EndpointForVendor(%q, %q): %v", test.vendor, test.baseURL, err)
		}
		if endpoint.BaseURL != test.wantBase {
			t.Fatalf("endpoint = %q, want %q", endpoint.BaseURL, test.wantBase)
		}
		if vendor.EnvKey != test.wantEnv {
			t.Fatalf("vendor %q env key = %q, want %q", test.vendor, vendor.EnvKey, test.wantEnv)
		}
		ref, err := catalog.RefForEndpoint(test.vendor, endpoint)
		if err != nil {
			t.Fatalf("RefForEndpoint(%q, %q): %v", test.vendor, endpoint.BaseURL, err)
		}
		if ref.Name() != test.vendor {
			t.Fatalf("ref name = %q, want the vendor %q", ref.Name(), test.vendor)
		}
		_, err = ref.Model(context.Background(), ModelSpec{})
		var missing *KeyMissingError
		if !errors.As(err, &missing) {
			t.Fatalf("Model() without a key error = %v, want KeyMissingError", err)
		}
		if missing.EnvKey != test.wantEnv || missing.Provider != test.vendor {
			t.Fatalf("KeyMissingError = %+v, want provider %q with env key %q", missing, test.vendor, test.wantEnv)
		}
		// The sealed adapter is the same for both vendors on one family.
		adapter, err := catalog.Adapter(endpoint.Adapter)
		if err != nil {
			t.Fatalf("Adapter(%q): %v", endpoint.Adapter, err)
		}
		if adapter.Name() != endpoint.Adapter {
			t.Fatalf("adapter name = %q, want %q", adapter.Name(), endpoint.Adapter)
		}
	}
}

func TestCatalogRefForEndpointRejectsUnknownVendor(t *testing.T) {
	endpoint := testEndpoint(t, testOpenAIVendor("https://api.openai.com/v1"))
	if _, err := NewCatalog().RefForEndpoint("does-not-exist", endpoint); err == nil {
		t.Fatal("a vendor with no embedded data must fail closed")
	}
}

// A user-supplied base URL that no endpoint declares keeps the vendor's
// protocol and capabilities in force; the address itself comes from the spec.
func TestCatalogEndpointForVendorFallsBackToDefault(t *testing.T) {
	vendor := testDeepSeekVendor("https://api.deepseek.com")
	declared, _, err := NewCatalog(vendor).EndpointForVendor("deepseek", "https://api.deepseek.com")
	if err != nil {
		t.Fatalf("declared endpoint: %v", err)
	}
	if declared.BaseURL != "https://api.deepseek.com" || !declared.HasCapability(CapabilityDeepSeekThinking) {
		t.Fatalf("declared endpoint = %+v", declared)
	}
	proxy, _, err := NewCatalog(vendor).EndpointForVendor("deepseek", "https://my-proxy.example/v1")
	if err != nil {
		t.Fatalf("undeclared endpoint: %v", err)
	}
	if proxy.BaseURL != "https://api.deepseek.com" || !proxy.HasCapability(CapabilityDeepSeekThinking) {
		t.Fatalf("undeclared base URL must fall back to the vendor default, got %+v", proxy)
	}
	if _, _, err := NewCatalog(vendor).EndpointForVendor("nope", ""); err == nil {
		t.Fatal("unknown vendor must fail")
	}
}

func TestCatalogVendorsAreOrderedAndCopied(t *testing.T) {
	catalog := NewCatalog(testOpenAIVendor("https://api.openai.com/v1"), testClaudeVendor("https://api.anthropic.com"), testDeepSeekVendor("https://api.deepseek.com"))
	vendors := catalog.Vendors()
	if len(vendors) != 3 || vendors[0].Name != "anthropic" || vendors[1].Name != "deepseek" || vendors[2].Name != "openai" {
		t.Fatalf("vendors = %v, want stable name order", vendors)
	}
	if _, ok := (*Catalog)(nil).Vendor("openai"); ok {
		t.Fatal("a nil catalog must not resolve vendors")
	}
}
