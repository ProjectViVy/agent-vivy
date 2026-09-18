package provider

import (
	"strings"
	"testing"
)

const validVendorYAML = `
- name: example
  display_name: Example
  env_key: EXAMPLE_API_KEY
  endpoints:
    - adapter: openai-completions
      base_url: https://api.example.com/v1
      default_model: example-chat
      models:
        - id: example-chat
          context_window: 128000
          input_per_mtok: 1
          output_per_mtok: 2
          supports_images: true
          supports_thinking: true
        - id: example-reasoner
          supports_thinking: true
      capabilities:
        - deepseek-thinking
  provenance:
    source: test
    entry: example
    derived_at: "2026-09-18"
`

func TestParseVendorsAcceptsValidDocument(t *testing.T) {
	vendors, err := ParseVendors([]byte(validVendorYAML))
	if err != nil {
		t.Fatalf("ParseVendors: %v", err)
	}
	if len(vendors) != 1 {
		t.Fatalf("vendors = %d, want 1", len(vendors))
	}
	vendor := vendors[0]
	if vendor.Name != "example" || vendor.EnvKey != "EXAMPLE_API_KEY" || vendor.DisplayName != "Example" {
		t.Fatalf("unexpected vendor identity: %+v", vendor)
	}
	endpoint, ok := vendor.DefaultEndpoint()
	if !ok {
		t.Fatal("DefaultEndpoint: no endpoint")
	}
	if endpoint.Adapter != AdapterOpenAICompletions || endpoint.DefaultModel != "example-chat" {
		t.Fatalf("unexpected default endpoint: %+v", endpoint)
	}
	model, ok := endpoint.Model("example-chat")
	if !ok {
		t.Fatal("endpoint does not carry example-chat")
	}
	if model.ContextWindow != 128000 || !model.SupportsThinking || !model.SupportsImages {
		t.Fatalf("unexpected model metadata: %+v", model)
	}
	if got := endpoint.ModelIDs(); len(got) != 2 || got[0] != "example-chat" || got[1] != "example-reasoner" {
		t.Fatalf("ModelIDs = %v", got)
	}
	if _, ok := endpoint.Model("nope"); ok {
		t.Fatal("unknown model must not resolve")
	}
}

func TestParseVendorsRejectsInvalidDocuments(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(string) string
		wantErr string
	}{
		{
			name: "unknown key",
			mutate: func(in string) string {
				return strings.Replace(in, "  display_name:", "  keywords:\n    - example\n  display_name:", 1)
			},
			wantErr: "keywords",
		},
		{
			name:    "missing env_key",
			mutate:  func(in string) string { return strings.Replace(in, "  env_key: EXAMPLE_API_KEY\n", "", 1) },
			wantErr: "env_key",
		},
		{
			name:    "missing display_name",
			mutate:  func(in string) string { return strings.Replace(in, "  display_name: Example\n", "", 1) },
			wantErr: "display_name",
		},
		{
			name: "missing provenance",
			mutate: func(in string) string {
				return strings.Replace(in, "  provenance:\n    source: test\n    entry: example\n    derived_at: \"2026-09-18\"\n", "", 1)
			},
			wantErr: "provenance",
		},
		{
			name:    "uppercase vendor name",
			mutate:  func(in string) string { return strings.Replace(in, "name: example", "name: Example", 1) },
			wantErr: "must match",
		},
		{
			name: "lowercase env_key",
			mutate: func(in string) string {
				return strings.Replace(in, "env_key: EXAMPLE_API_KEY", "env_key: example_api_key", 1)
			},
			wantErr: "env_key",
		},
		{
			name: "unsealed adapter",
			mutate: func(in string) string {
				return strings.Replace(in, "adapter: openai-completions", "adapter: gemini-generate-content", 1)
			},
			wantErr: "not sealed",
		},
		{
			name: "relative base url",
			mutate: func(in string) string {
				return strings.Replace(in, "base_url: https://api.example.com/v1", "base_url: api.example.com/v1", 1)
			},
			wantErr: "absolute http or https URL",
		},
		{
			name: "default_model outside models",
			mutate: func(in string) string {
				return strings.Replace(in, "default_model: example-chat", "default_model: example-missing", 1)
			},
			wantErr: "must appear in models",
		},
		{
			name: "vendor-prefixed model id",
			mutate: func(in string) string {
				return strings.Replace(in, "        - id: example-reasoner", "        - id: example/example-reasoner", 1)
			},
			wantErr: "forbidden",
		},
		{
			name: "adapter-prefixed model id",
			mutate: func(in string) string {
				return strings.Replace(in, "        - id: example-reasoner", "        - id: openai-completions/example-reasoner", 1)
			},
			wantErr: "forbidden",
		},
		{
			name: "duplicate model id",
			mutate: func(in string) string {
				return strings.Replace(in, "        - id: example-reasoner", "        - id: example-chat", 1)
			},
			wantErr: "duplicate model id",
		},
		{
			name: "unknown capability",
			mutate: func(in string) string {
				return strings.Replace(in, "        - deepseek-thinking", "        - turbo-mode", 1)
			},
			wantErr: "not implemented",
		},
		{
			name:    "negative metadata",
			mutate:  func(in string) string { return strings.Replace(in, "context_window: 128000", "context_window: -1", 1) },
			wantErr: "must not be negative",
		},
		{
			name: "duplicate vendor name",
			mutate: func(in string) string {
				return in + strings.Replace(validVendorYAML, "name: example", "name: example", 1)
			},
			wantErr: "duplicate vendor name",
		},
		{
			name: "duplicate endpoint identity",
			mutate: func(in string) string {
				return in + strings.Replace(validVendorYAML, "name: example", "name: other", 1)
			},
			wantErr: "already declared",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := ParseVendors([]byte(test.mutate(validVendorYAML)))
			if err == nil {
				t.Fatal("expected an error, got nil")
			}
			if !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("error %q does not mention %q", err, test.wantErr)
			}
		})
	}
}

func TestParseVendorsRejectsVendorWithoutEndpoints(t *testing.T) {
	document := `
- name: example
  display_name: Example
  env_key: EXAMPLE_API_KEY
  endpoints: []
  provenance:
    source: test
    entry: example
    derived_at: "2026-09-18"
`
	_, err := ParseVendors([]byte(document))
	if err == nil {
		t.Fatal("expected a vendor without endpoints to be rejected")
	}
	if !strings.Contains(err.Error(), "at least one endpoint") {
		t.Fatalf("error %q does not explain the missing endpoints", err)
	}
}

func TestParseVendorsRejectsEndpointWithoutModels(t *testing.T) {
	document := `
- name: example
  display_name: Example
  env_key: EXAMPLE_API_KEY
  endpoints:
    - adapter: openai-completions
      base_url: https://api.example.com/v1
      default_model: example-chat
      models: []
  provenance:
    source: test
    entry: example
    derived_at: "2026-09-18"
`
	_, err := ParseVendors([]byte(document))
	if err == nil {
		t.Fatal("expected an endpoint without models to be rejected")
	}
	if !strings.Contains(err.Error(), "at least one model") {
		t.Fatalf("error %q does not explain the missing models", err)
	}
}

func TestParseVendorsAcceptsDigitLeadingIdentifiers(t *testing.T) {
	// Upstream catalogs carry vendor ids and credential names that start with
	// a digit; they must survive validation unchanged (evidence: "302ai").
	document := strings.Replace(validVendorYAML, "name: example", "name: 302ai", 1)
	document = strings.Replace(document, "env_key: EXAMPLE_API_KEY", "env_key: 302AI_API_KEY", 1)
	vendors, err := ParseVendors([]byte(document))
	if err != nil {
		t.Fatalf("ParseVendors: %v", err)
	}
	if vendors[0].Name != "302ai" || vendors[0].EnvKey != "302AI_API_KEY" {
		t.Fatalf("identifiers were rewritten: %+v", vendors[0])
	}
}

func TestParseVendorsRejectsCapabilityOnUnsupportedAdapter(t *testing.T) {
	document := strings.Replace(validVendorYAML, "adapter: openai-completions", "adapter: anthropic-messages", 1)
	document = strings.Replace(document, "base_url: https://api.example.com/v1", "base_url: https://api.anthropic.com", 1)
	_, err := ParseVendors([]byte(document))
	if err == nil {
		t.Fatal("expected deepseek-thinking on anthropic-messages to be rejected")
	}
	if !strings.Contains(err.Error(), "deepseek-thinking") {
		t.Fatalf("error %q does not name the offending capability", err)
	}
}

func TestParseVendorsJoinsEveryFailure(t *testing.T) {
	document := strings.Replace(validVendorYAML, "name: example", "name: Example", 1)
	document = strings.Replace(document, "env_key: EXAMPLE_API_KEY", "env_key: nope", 1)
	document = strings.Replace(document, "  display_name: Example\n", "", 1)
	_, err := ParseVendors([]byte(document))
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	message := err.Error()
	for _, want := range []string{"must match", "env_key", "display_name"} {
		if !strings.Contains(message, want) {
			t.Fatalf("joined error %q is missing %q", message, want)
		}
	}
}

func TestParseVendorsRejectsEmptyDocument(t *testing.T) {
	if _, err := ParseVendors([]byte("[]")); err == nil {
		t.Fatal("expected an empty document to be rejected")
	}
}

// The embedded document is the product's provider truth. These assertions are
// the data-contract anchors the phase exit criteria name.
func TestLoadEmbeddedProvidesTheSealedProviderCatalog(t *testing.T) {
	files := EmbeddedDataFiles()
	if len(files) != 1 || files[0] != "data/vendors.yaml" {
		t.Fatalf("embedded data files = %v, want [data/vendors.yaml]", files)
	}
	vendors, err := LoadEmbedded()
	if err != nil {
		t.Fatalf("LoadEmbedded: %v", err)
	}
	if len(vendors) != 45 {
		t.Fatalf("vendors = %d, want 45", len(vendors))
	}
	endpoints, models := 0, 0
	seen := map[string]bool{}
	for _, vendor := range vendors {
		if seen[vendor.Name] {
			t.Fatalf("duplicate vendor %q", vendor.Name)
		}
		seen[vendor.Name] = true
		endpoints += len(vendor.Endpoints)
		for _, endpoint := range vendor.Endpoints {
			models += len(endpoint.Models)
		}
	}
	if endpoints != 47 || models != 168 {
		t.Fatalf("endpoints = %d, models = %d; want 47 and 168", endpoints, models)
	}
	for _, dropped := range []string{"custom", "cherryin"} {
		if seen[dropped] {
			t.Fatalf("vendor %q must not be carried into the Vivy catalog", dropped)
		}
	}
}

func TestLoadEmbeddedDeepSeekDefaultChain(t *testing.T) {
	vendors, err := LoadEmbedded()
	if err != nil {
		t.Fatalf("LoadEmbedded: %v", err)
	}
	var deepseek Vendor
	for _, vendor := range vendors {
		if vendor.Name == "deepseek" {
			deepseek = vendor
		}
	}
	if deepseek.Name == "" {
		t.Fatal("deepseek is missing from the embedded data")
	}
	if deepseek.EnvKey != "DEEPSEEK_API_KEY" {
		t.Fatalf("deepseek env_key = %q", deepseek.EnvKey)
	}
	endpoint, ok := deepseek.DefaultEndpoint()
	if !ok {
		t.Fatal("deepseek has no default endpoint")
	}
	if endpoint.Adapter != AdapterOpenAICompletions {
		t.Fatalf("deepseek default adapter = %q, want %q", endpoint.Adapter, AdapterOpenAICompletions)
	}
	if endpoint.BaseURL != "https://api.deepseek.com" || endpoint.DefaultModel != "deepseek-flash" {
		t.Fatalf("deepseek default endpoint = %s / %s", endpoint.BaseURL, endpoint.DefaultModel)
	}
	// D12: the DeepSeek reasoning request shape is an endpoint capability on
	// one of the sealed three, never a fourth adapter.
	if len(endpoint.Capabilities) != 1 || endpoint.Capabilities[0] != CapabilityDeepSeekThinking {
		t.Fatalf("deepseek capabilities = %v, want [%s]", endpoint.Capabilities, CapabilityDeepSeekThinking)
	}
	for _, id := range []string{"deepseek-flash", "deepseek-v4-pro", "deepseek-v4-flash", "deepseek-reasoner"} {
		model, ok := endpoint.Model(id)
		if !ok {
			t.Fatalf("deepseek default endpoint is missing %q", id)
		}
		if !model.SupportsThinking {
			t.Fatalf("%s must declare supports_thinking", id)
		}
	}
	anthropicEndpoint, ok := deepseek.Endpoint(AdapterAnthropicMessages, "https://api.deepseek.com/anthropic")
	if !ok {
		t.Fatal("deepseek must declare its Anthropic-compatible endpoint")
	}
	if anthropicEndpoint.DefaultModel != "deepseek-chat" || !anthropicEndpoint.SupportsPromptCaching {
		t.Fatalf("unexpected deepseek anthropic endpoint: %+v", anthropicEndpoint)
	}
}

func TestLoadEmbeddedDeclaresTheDeferredResponsesEndpoint(t *testing.T) {
	vendors, err := LoadEmbedded()
	if err != nil {
		t.Fatalf("LoadEmbedded: %v", err)
	}
	declared := map[string]bool{}
	for _, vendor := range vendors {
		for _, endpoint := range vendor.Endpoints {
			declared[endpoint.Adapter] = true
		}
	}
	for _, adapter := range AdapterFamilies() {
		if !declared[adapter] {
			t.Fatalf("adapter %q is sealed but has no endpoint in the data", adapter)
		}
	}
	var openai Vendor
	for _, vendor := range vendors {
		if vendor.Name == "openai" {
			openai = vendor
		}
	}
	if _, ok := openai.Endpoint(AdapterOpenAIResponses, "https://api.openai.com/v1"); !ok {
		t.Fatal("openai must declare the deferred openai-responses endpoint so the capability is visible")
	}
}

// The reference metadata the product asserts must survive a data edit
// unnoticed: every number below is a product claim, not a fixture detail.
func TestLoadEmbeddedMetadataAnchors(t *testing.T) {
	vendors, err := LoadEmbedded()
	if err != nil {
		t.Fatalf("LoadEmbedded: %v", err)
	}
	index := map[string]Vendor{}
	for _, vendor := range vendors {
		index[vendor.Name] = vendor
	}

	openai, ok := index["openai"].DefaultEndpoint()
	if !ok {
		t.Fatal("openai has no default endpoint")
	}
	if openai.Adapter != AdapterOpenAICompletions || openai.BaseURL != "https://api.openai.com/v1" || openai.DefaultModel != "gpt-4o" {
		t.Fatalf("openai default endpoint = %+v", openai)
	}
	gpt4o, ok := openai.Model("gpt-4o")
	if !ok {
		t.Fatal("openai must declare gpt-4o")
	}
	if gpt4o.ContextWindow != 128000 || gpt4o.InputPerMTok != 2.5 || gpt4o.OutputPerMTok != 10 || !gpt4o.SupportsImages {
		t.Fatalf("gpt-4o metadata = %+v", gpt4o)
	}
	if gpt4o.SupportsThinking {
		t.Fatal("gpt-4o must not claim thinking support")
	}
	turbo, ok := openai.Model("gpt-3.5-turbo")
	if !ok {
		t.Fatal("openai must declare gpt-3.5-turbo")
	}
	if turbo.SupportsImages {
		t.Fatal("gpt-3.5-turbo must not claim image support")
	}

	anthropic, ok := index["anthropic"].DefaultEndpoint()
	if !ok {
		t.Fatal("anthropic has no default endpoint")
	}
	if anthropic.Adapter != AdapterAnthropicMessages || anthropic.BaseURL != "https://api.anthropic.com" || anthropic.DefaultModel != "claude-sonnet-4-5" {
		t.Fatalf("anthropic default endpoint = %+v", anthropic)
	}
	if !anthropic.SupportsPromptCaching {
		t.Fatal("anthropic must enable automatic prompt caching")
	}
	sonnet, ok := anthropic.Model("claude-sonnet-4-5")
	if !ok {
		t.Fatal("anthropic must declare claude-sonnet-4-5")
	}
	if sonnet.ContextWindow != 200000 || sonnet.InputPerMTok != 3 || sonnet.OutputPerMTok != 15 || !sonnet.SupportsImages || !sonnet.SupportsThinking {
		t.Fatalf("claude-sonnet-4-5 metadata = %+v", sonnet)
	}
	// A model listed without metadata stays unknown: zero means unknown, never
	// free and never zero-cost (DESIGN §6).
	if legacy, ok := anthropic.Model("claude-3-opus-20240229"); !ok || legacy != (Model{ID: "claude-3-opus-20240229"}) {
		t.Fatalf("unannotated model must carry only its id, got %+v", legacy)
	}
}

func TestParseVendorsRejectsDeletedVocabulary(t *testing.T) {
	// The pre-PROV-P1 bundle vocabulary must not creep back into the data.
	for _, field := range []string{"backend", "api_type", "keywords", "gateway_prefix", "is_gateway", "model_overrides", "strip_model_prefix"} {
		document := strings.Replace(validVendorYAML, "  display_name: Example\n", "  display_name: Example\n  "+field+": []\n", 1)
		if _, err := ParseVendors([]byte(document)); err == nil {
			t.Fatalf("field %q must be rejected as an unknown key", field)
		}
	}
}
