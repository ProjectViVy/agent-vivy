package provider

import (
	"strings"
	"testing"
)

func TestReconcileAdaptersAcceptsMatchingSets(t *testing.T) {
	vendors := []Vendor{{Name: "example", Endpoints: []Endpoint{
		{Adapter: AdapterOpenAICompletions, BaseURL: "https://a.example.com"},
		{Adapter: AdapterAnthropicMessages, BaseURL: "https://b.example.com"},
	}}}
	if err := ReconcileAdapters(vendors, []string{AdapterOpenAICompletions, AdapterAnthropicMessages}); err != nil {
		t.Fatalf("ReconcileAdapters: %v", err)
	}
}

func TestReconcileAdaptersRejectsUnsealedAdapterInData(t *testing.T) {
	vendors := []Vendor{{Name: "example", Endpoints: []Endpoint{
		{Adapter: "gemini-generate-content", BaseURL: "https://a.example.com"},
	}}}
	err := ReconcileAdapters(vendors, []string{AdapterOpenAICompletions})
	if err == nil {
		t.Fatal("expected an unsealed adapter in the data to fail the gate")
	}
	if !strings.Contains(err.Error(), "not sealed") {
		t.Fatalf("error %q does not explain the mismatch", err)
	}
}

func TestReconcileAdaptersRejectsSealedAdapterWithoutEndpoint(t *testing.T) {
	vendors := []Vendor{{Name: "example", Endpoints: []Endpoint{
		{Adapter: AdapterOpenAICompletions, BaseURL: "https://a.example.com"},
	}}}
	err := ReconcileAdapters(vendors, []string{AdapterOpenAICompletions, AdapterOpenAIResponses})
	if err == nil {
		t.Fatal("expected a sealed adapter with no endpoint to fail the gate")
	}
	if !strings.Contains(err.Error(), AdapterOpenAIResponses) {
		t.Fatalf("error %q does not name the missing adapter", err)
	}
}

func TestReconcileAdaptersJoinsBothDirections(t *testing.T) {
	vendors := []Vendor{{Name: "example", Endpoints: []Endpoint{
		{Adapter: "gemini-generate-content", BaseURL: "https://a.example.com"},
	}}}
	err := ReconcileAdapters(vendors, []string{AdapterOpenAICompletions, AdapterOpenAIResponses})
	if err == nil {
		t.Fatal("expected a bidirectional mismatch to fail")
	}
	for _, want := range []string{"not sealed", AdapterOpenAICompletions, AdapterOpenAIResponses} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("joined error %q is missing %q", err, want)
		}
	}
}

// The startup gate must pass against the data this build actually ships.
func TestReconcileAdaptersAcceptsEmbeddedData(t *testing.T) {
	vendors, err := LoadEmbedded()
	if err != nil {
		t.Fatalf("LoadEmbedded: %v", err)
	}
	if err := ReconcileAdapters(vendors, AdapterFamilies()); err != nil {
		t.Fatalf("embedded data must reconcile with the sealed adapter set: %v", err)
	}
}

func TestAdapterFamiliesIsTheSealedSet(t *testing.T) {
	families := AdapterFamilies()
	if len(families) != 3 {
		t.Fatalf("sealed adapters = %v, want exactly three", families)
	}
	want := []string{AdapterOpenAICompletions, AdapterOpenAIResponses, AdapterAnthropicMessages}
	for i, family := range want {
		if families[i] != family {
			t.Fatalf("sealed adapters = %v, want %v", families, want)
		}
	}
	if !IsSealedAdapter(AdapterAnthropicMessages) || IsSealedAdapter("nope") {
		t.Fatal("IsSealedAdapter disagrees with the sealed set")
	}
	families[0] = "mutated"
	if AdapterFamilies()[0] != AdapterOpenAICompletions {
		t.Fatal("AdapterFamilies must return a copy")
	}
}

func TestAdapterCapabilityVocabulary(t *testing.T) {
	if !AdapterSupportsCapability(AdapterOpenAICompletions, CapabilityDeepSeekThinking) {
		t.Fatal("deepseek-thinking must be legal on openai-completions")
	}
	for _, adapter := range []string{AdapterOpenAIResponses, AdapterAnthropicMessages} {
		if AdapterSupportsCapability(adapter, CapabilityDeepSeekThinking) {
			t.Fatalf("deepseek-thinking must not be legal on %q", adapter)
		}
	}
	if AdapterSupportsCapability(AdapterOpenAICompletions, "turbo-mode") {
		t.Fatal("the capability vocabulary is closed")
	}
}
