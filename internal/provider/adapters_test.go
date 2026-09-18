package provider

import (
	"reflect"
	"testing"

	"agent-vivy/internal/modelhost"
)

// The sealed set is a table, in a stable order, with exactly three rows.
func TestAdaptersIsTheSealedTable(t *testing.T) {
	want := []Adapter{
		{Family: AdapterOpenAICompletions, State: modelhost.CapabilitySupported},
		{Family: AdapterOpenAIResponses, State: modelhost.CapabilityDeferredIndefinite},
		{Family: AdapterAnthropicMessages, State: modelhost.CapabilitySupported},
	}
	got := Adapters()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Adapters() = %#v, want %#v", got, want)
	}
	got[0].Family = "mutated"
	if Adapters()[0].Family != AdapterOpenAICompletions {
		t.Fatal("Adapters must return a copy")
	}
}

// Adapters() is the only family source: every other projection is derived from
// it, so data can never introduce a fourth family or a new capability state.
func TestAdapterProjectionsDeriveFromTheTable(t *testing.T) {
	adapters := Adapters()
	families := AdapterFamilies()
	capabilities := Capabilities()
	if len(families) != len(adapters) || len(capabilities) != len(adapters) {
		t.Fatalf("projections drift: %d adapters, %d families, %d capabilities", len(adapters), len(families), len(capabilities))
	}
	for i, adapter := range adapters {
		if families[i] != adapter.Family {
			t.Fatalf("AdapterFamilies() = %v, want it derived from %v", families, adapters)
		}
		if state, sealed := AdapterState(adapter.Family); !sealed || state != adapter.State {
			t.Fatalf("AdapterState(%q) = %q/%v, want %q", adapter.Family, state, sealed, adapter.State)
		}
		if capabilities[adapter.Family] != adapter.State {
			t.Fatalf("Capabilities()[%q] = %q, want %q", adapter.Family, capabilities[adapter.Family], adapter.State)
		}
		if !IsSealedAdapter(adapter.Family) {
			t.Fatalf("IsSealedAdapter(%q) = false", adapter.Family)
		}
	}
	if _, sealed := AdapterState("gemini-generate-content"); sealed || IsSealedAdapter("gemini-generate-content") {
		t.Fatal("the sealed set must reject an unsealed family")
	}
}

// A sealed family with no pinned implementation is reported as such, not
// silently omitted and not silently executable.
func TestDeferredAdapterStateIsExplicit(t *testing.T) {
	state, sealed := AdapterState(AdapterOpenAIResponses)
	if !sealed {
		t.Fatal("openai-responses must be declared, not omitted")
	}
	if state != modelhost.CapabilityDeferredIndefinite {
		t.Fatalf("openai-responses state = %q, want DEFERRED-INDEFINITE", state)
	}
}
