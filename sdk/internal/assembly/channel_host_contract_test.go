package assembly

import (
	"strings"
	"testing"

	"agent-vivy/internal/moduleport"
	"agent-vivy/sdk/module"
)

func requiredCoreProviders() map[string][]SourceRecord {
	providers := make(map[string][]SourceRecord)
	for _, definition := range moduleport.Catalog().Definitions() {
		if definition.Cardinality == moduleport.CardinalityExactlyOne {
			providers[definition.Ref.Port] = []SourceRecord{{}}
		}
	}
	return providers
}

func TestChannelHostContract(t *testing.T) {
	t.Run("valid empty host", func(t *testing.T) {
		providers := requiredCoreProviders()
		providers["core/channel-host@v1"] = []SourceRecord{{}}
		if got := validateClosedInternalSelection(nil, providers); len(got) != 0 {
			t.Fatalf("diagnostics = %v, want none", got)
		}
	})

	t.Run("provider without host", func(t *testing.T) {
		providers := requiredCoreProviders()
		selected := map[string]SourceRecord{
			"fixture/channel": {Descriptor: withProvides(testDescriptor("fixture/channel"), module.PortRef{Port: "std/channel@v1", ID: "fixture.channel"})},
		}
		diagnostics := validateClosedInternalSelection(selected, providers)
		if len(diagnostics) != 1 || !strings.Contains(diagnostics[0], "requires conditional Host core/channel-host@v1") {
			t.Fatalf("diagnostics = %v, want missing Channel Host", diagnostics)
		}
	})

	t.Run("duplicate host", func(t *testing.T) {
		providers := requiredCoreProviders()
		providers["core/channel-host@v1"] = []SourceRecord{{}, {}}
		diagnostics := validateClosedInternalSelection(nil, providers)
		if len(diagnostics) != 1 || !strings.Contains(diagnostics[0], "allows at most one Provider; selected 2") {
			t.Fatalf("diagnostics = %v, want Channel Host cardinality error", diagnostics)
		}
	})

	t.Run("public core provider", func(t *testing.T) {
		err := validateInternalProvider(module.PortRef{Port: "core/channel-host@v1", ID: "fixture.channel-host"}, "fixture/channel-host", TrustT2)
		if err == nil || !strings.Contains(err.Error(), "may only be provided by build-owned T1 module vivy/channel-host") {
			t.Fatalf("error = %v, want canonical T1 owner diagnostic", err)
		}
	})
}
