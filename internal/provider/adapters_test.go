package provider

import (
	"reflect"
	"testing"
)

// The sealed set is a table, in a stable order, with exactly two rows: both
// families are executable in the agentic generation.
func TestAdaptersIsTheSealedTable(t *testing.T) {
	want := []string{AdapterOpenAICompletions, AdapterAnthropicMessages}
	got := AdapterFamilies()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("AdapterFamilies() = %#v, want %#v", got, want)
	}
	got[0] = "mutated"
	if AdapterFamilies()[0] != AdapterOpenAICompletions {
		t.Fatal("AdapterFamilies must return a copy")
	}
}

// AdapterFamilies() is the only family source: every other projection is
// derived from it, so data can never introduce a third family.
func TestAdapterProjectionsDeriveFromTheTable(t *testing.T) {
	families := AdapterFamilies()
	if len(families) != 2 {
		t.Fatalf("families = %v, want two sealed adapters", families)
	}
	for _, family := range families {
		if !IsSealedAdapter(family) {
			t.Fatalf("IsSealedAdapter(%q) = false", family)
		}
	}
	if IsSealedAdapter("gemini-generate-content") {
		t.Fatal("the sealed set must reject an unsealed family")
	}
	if IsSealedAdapter("openai-responses") {
		t.Fatal("openai-responses was retired and must not be sealed")
	}
}

// A family this build does not seal fails closed at every catalog entry point.
func TestUnknownAdapterFailsClosed(t *testing.T) {
	catalog := NewCatalog()
	if _, err := catalog.Adapter("gemini-generate-content"); err == nil {
		t.Fatal("unknown adapter family was accepted")
	}
	if _, err := catalog.Adapter("openai-responses"); err == nil {
		t.Fatal("retired adapter family was accepted")
	}
}
