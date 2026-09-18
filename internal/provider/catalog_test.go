package provider

import (
	"context"
	"errors"
	"strings"
	"testing"

	"agent-vivy/sdk/port/providerprofile"
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

func TestCatalogForProfileResolvesExecutableProfile(t *testing.T) {
	vendor := testOpenAIVendor("https://api.openai.com/v1")
	profile := testProfile(t, vendor)
	ref, err := NewCatalog(vendor).ForProfile(profile)
	if err != nil {
		t.Fatalf("ForProfile: %v", err)
	}
	if ref.Name() != "openai" {
		t.Fatalf("ref name = %q", ref.Name())
	}
	if profile.ID != "openai" || profile.EndpointClass != providerprofile.EndpointNative {
		t.Fatalf("unexpected profile projection: %+v", profile)
	}
	if len(profile.SecretRefs) != 1 || profile.SecretRefs[0] != "OPENAI_API_KEY" {
		t.Fatalf("profile secret refs = %v", profile.SecretRefs)
	}
}

func TestCatalogForProfileRejectsVendorWithoutData(t *testing.T) {
	profile := testProfile(t, testOpenAIVendor("https://api.openai.com/v1"))
	profile.ID = "does-not-exist"
	if _, err := NewCatalog().ForProfile(profile); err == nil {
		t.Fatal("a Profile with no embedded vendor data must fail closed")
	}
}

func TestCatalogForProfileRejectsUnknownAdapterFamily(t *testing.T) {
	vendor := testOpenAIVendor("https://api.openai.com/v1")
	profile := testProfile(t, vendor)
	profile.AdapterFamily = "gemini"
	_, err := NewCatalog(vendor).ForProfile(profile)
	if !errors.Is(err, ErrAdapterFamilyMismatch) {
		t.Fatalf("err = %v, want ErrAdapterFamilyMismatch", err)
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
