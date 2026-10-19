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
	err := ReconcileAdapters(vendors, []string{AdapterOpenAICompletions, AdapterAnthropicMessages})
	if err == nil {
		t.Fatal("expected a supported sealed adapter with no endpoint to fail the gate")
	}
	if !strings.Contains(err.Error(), AdapterAnthropicMessages) {
		t.Fatalf("error %q does not name the missing adapter", err)
	}
}

// Every sealed family is executable, so a sealed family with no endpoint is a
// startup failure — there is no deferral allowance any more.
func TestReconcileAdaptersRejectsSealedAdapterWithoutAnyEndpoint(t *testing.T) {
	vendors := []Vendor{{Name: "example", Endpoints: []Endpoint{
		{Adapter: AdapterOpenAICompletions, BaseURL: "https://a.example.com"},
	}}}
	err := ReconcileAdapters(vendors, []string{AdapterOpenAICompletions, AdapterAnthropicMessages})
	if err == nil || !strings.Contains(err.Error(), AdapterAnthropicMessages) {
		t.Fatalf("sealed family without data must fail the gate: %v", err)
	}
}

func TestReconcileAdaptersJoinsBothDirections(t *testing.T) {
	vendors := []Vendor{{Name: "example", Endpoints: []Endpoint{
		{Adapter: "gemini-generate-content", BaseURL: "https://a.example.com"},
	}}}
	err := ReconcileAdapters(vendors, []string{AdapterOpenAICompletions, AdapterAnthropicMessages})
	if err == nil {
		t.Fatal("expected a bidirectional mismatch to fail")
	}
	for _, want := range []string{"not sealed", AdapterOpenAICompletions, AdapterAnthropicMessages} {
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
	if len(families) != 2 {
		t.Fatalf("sealed adapters = %v, want exactly two", families)
	}
	want := []string{AdapterOpenAICompletions, AdapterAnthropicMessages}
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
	for _, adapter := range []string{"openai-responses", AdapterAnthropicMessages} {
		if AdapterSupportsCapability(adapter, CapabilityDeepSeekThinking) {
			t.Fatalf("deepseek-thinking must not be legal on %q", adapter)
		}
	}
	if AdapterSupportsCapability(AdapterOpenAICompletions, "turbo-mode") {
		t.Fatal("the capability vocabulary is closed")
	}
}
