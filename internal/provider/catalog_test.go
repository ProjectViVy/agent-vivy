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

// A retired adapter family is not sealed: reaching it through data must fail
// closed instead of silently using another adapter.
func TestCatalogRefusesRetiredAdapter(t *testing.T) {
	vendor := testVendor("openai", "OpenAI", "OPENAI_API_KEY", "openai-responses", "https://api.openai.com/v1", "gpt-5", []Model{{ID: "gpt-5"}})
	_, err := NewCatalog(vendor).For("openai")
	if err == nil {
		t.Fatal("the openai-responses adapter must not be executable")
	}
	if !strings.Contains(err.Error(), "openai-responses") {
		t.Fatalf("error %q must name the retired adapter", err)
	}
}

// The sealed table has no openai-responses member, so no vendor can declare it
// without failing the startup reconcile either.
func TestRetiredAdapterIsNotInTheSealedSet(t *testing.T) {
	if IsSealedAdapter("openai-responses") {
		t.Fatal("openai-responses must not be sealed")
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

// A family that is not in the sealed table fails closed as unknown, whether
// it is a retired family or one this build never spoke.
func TestCatalogAdapterRefusesUnknownFamilies(t *testing.T) {
	for _, family := range []string{"openai-responses", "nope"} {
		_, err := NewCatalog().Adapter(family)
		if !errors.Is(err, ErrAdapterUnknown) {
			t.Fatalf("Adapter(%q) error = %v, want ErrAdapterUnknown", family, err)
		}
		if !strings.Contains(err.Error(), family) {
			t.Fatalf("error %q must name %q", err, family)
		}
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
		adapter  string
		baseURL  string
		wantEnv  string
		wantBase string
	}{
		{"deepseek", "", "https://api.deepseek.com", "DEEPSEEK_API_KEY", "https://api.deepseek.com"},
		{"deepseek", AdapterAnthropicMessages, "", "DEEPSEEK_API_KEY", "https://api.deepseek.com/anthropic"},
		{"deepseek", "", "https://api.deepseek.com/anthropic", "DEEPSEEK_API_KEY", "https://api.deepseek.com/anthropic"},
		{"anthropic", "", "", "ANTHROPIC_API_KEY", "https://api.anthropic.com"},
	} {
		endpoint, vendor, err := catalog.EndpointForVendor(test.vendor, test.adapter, test.baseURL)
		if err != nil {
			t.Fatalf("EndpointForVendor(%q, %q, %q): %v", test.vendor, test.adapter, test.baseURL, err)
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

// Image capability is tri-state: a declared true/false is authoritative, and
// an absent declaration stays unknown so the attachment gate fails open.
func TestResolveModelInfoCarriesImageKnownness(t *testing.T) {
	vendor := testDeepSeekVendor("https://api.deepseek.com")
	vendor.Endpoints[0].Models = append(vendor.Endpoints[0].Models,
		Model{ID: "text-only", SupportsImages: boolPointer(false)},
		Model{ID: "images-undeclared"},
	)
	catalog := NewCatalog(vendor)
	for _, test := range []struct {
		model           string
		supports, known bool
	}{
		{"deepseek-flash", true, true},
		{"text-only", false, true},
		{"images-undeclared", false, false},
	} {
		info, err := catalog.ResolveModelInfo(context.Background(), "deepseek", test.model)
		if err != nil {
			t.Fatalf("%s: %v", test.model, err)
		}
		if info.SupportsImages != test.supports || info.ImagesKnown != test.known {
			t.Fatalf("%s image capability = %v/%v, want %v/%v", test.model, info.SupportsImages, info.ImagesKnown, test.supports, test.known)
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
	vendor.Endpoints = append(vendor.Endpoints, Endpoint{
		Adapter: AdapterAnthropicMessages, BaseURL: "https://api.deepseek.com/anthropic",
		DefaultModel: "deepseek-chat", Models: []Model{{ID: "deepseek-chat"}},
	})
	catalog := NewCatalog(vendor)
	declared, _, err := catalog.EndpointForVendor("deepseek", "", "https://api.deepseek.com")
	if err != nil {
		t.Fatalf("declared endpoint: %v", err)
	}
	if declared.BaseURL != "https://api.deepseek.com" || !declared.HasCapability(CapabilityDeepSeekThinking) {
		t.Fatalf("declared endpoint = %+v", declared)
	}
	proxy, _, err := catalog.EndpointForVendor("deepseek", "", "https://my-proxy.example/v1")
	if err != nil {
		t.Fatalf("undeclared endpoint: %v", err)
	}
	if proxy.BaseURL != "https://api.deepseek.com" || !proxy.HasCapability(CapabilityDeepSeekThinking) {
		t.Fatalf("undeclared base URL must fall back to the vendor default, got %+v", proxy)
	}
	// A proxy of the same vendor still speaks the protocol the selection
	// named, which is what makes a third-party gateway usable.
	variant, _, err := catalog.EndpointForVendor("deepseek", AdapterAnthropicMessages, "https://my-proxy.example/v1")
	if err != nil {
		t.Fatalf("undeclared variant endpoint: %v", err)
	}
	if variant.Adapter != AdapterAnthropicMessages || variant.BaseURL != "https://api.deepseek.com/anthropic" {
		t.Fatalf("proxy variant = %+v, want the declared anthropic endpoint", variant)
	}
	if _, _, err := catalog.EndpointForVendor("deepseek", "openai-responses", ""); err == nil {
		t.Fatal("a vendor with no endpoint for the requested adapter must fail")
	}
	if _, _, err := catalog.EndpointForVendor("nope", "", ""); err == nil {
		t.Fatal("unknown vendor must fail")
	}
}

// (adapter, base_url) is the endpoint identity, and data validation makes it
// globally unique, so an explicit address identifies its vendor without any
// per-vendor configuration.
func TestCatalogVendorForEndpointIdentifiesTheVendor(t *testing.T) {
	deepseek := testDeepSeekVendor("https://api.deepseek.com")
	catalog := NewCatalog(deepseek, testClaudeVendor("https://api.anthropic.com"))

	vendor, endpoint, ok := catalog.VendorForEndpoint(AdapterAnthropicMessages, "https://api.anthropic.com")
	if !ok || vendor.Name != "anthropic" || endpoint.Adapter != AdapterAnthropicMessages {
		t.Fatalf("VendorForEndpoint = %q, %+v, %v", vendor.Name, endpoint, ok)
	}
	if vendor, _, ok := catalog.VendorForEndpoint("", "https://api.deepseek.com"); !ok || vendor.Name != "deepseek" {
		t.Fatalf("an empty adapter must still identify the endpoint's vendor, got %q, %v", vendor.Name, ok)
	}
	if _, _, ok := catalog.VendorForEndpoint(AdapterAnthropicMessages, "https://api.deepseek.com"); ok {
		t.Fatal("an address that does not speak the requested adapter must not match")
	}
	if _, _, ok := catalog.VendorForEndpoint("", "https://gateway.example/v1"); ok {
		t.Fatal("an address no vendor declares is a user gateway, not an embedded endpoint")
	}
	if _, _, ok := (*Catalog)(nil).VendorForEndpoint("", "https://api.deepseek.com"); ok {
		t.Fatal("a nil catalog must not resolve endpoints")
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
