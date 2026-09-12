package assembly

import (
	"context"
	"strings"
	"testing"

	"agent-vivy/sdk/module"
	"agent-vivy/sdk/port"
)

func TestPublicSourceCannotProvideCorePort(t *testing.T) {
	descriptor := withProvides(testDescriptor("fixture/action-host"), module.PortRef{Port: "core/action-host@v1", ID: "fixture.action-host"})
	catalog, err := NewSourceCatalog([]SourceRecord{{Descriptor: descriptor, Trust: TrustT2}})
	if err != nil {
		t.Fatal(err)
	}
	compiler := Compiler{Ports: port.PublicCatalog(), Sources: catalog, PortEvidence: SupportedPortEvidence()}
	_, err = compiler.Compile(context.Background(), Recipe{APIVersion: RecipeAPIVersionV1, Modules: []string{descriptor.Module.ID}})
	if err == nil || !strings.Contains(err.Error(), "may only be provided by build-owned T1 module vivy/action-host") {
		t.Fatalf("public core Provider error = %v", err)
	}
}

func TestCanonicalCoreOwnerCanProvideActionHost(t *testing.T) {
	descriptor := withProvides(testDescriptor("vivy/action-host"), module.PortRef{Port: "core/action-host@v1", ID: "vivy.action-host"})
	descriptor.Source.Ref = "internal:vivy/action-host"
	catalog, err := NewSourceCatalog([]SourceRecord{{Descriptor: descriptor, Trust: TrustT1}})
	if err != nil {
		t.Fatal(err)
	}
	compiler := Compiler{Ports: port.PublicCatalog(), Sources: catalog, PortEvidence: SupportedPortEvidence()}
	if _, err := compiler.Compile(context.Background(), Recipe{APIVersion: RecipeAPIVersionV1, Modules: []string{descriptor.Module.ID}}); err != nil {
		t.Fatalf("canonical action Host rejected: %v", err)
	}
}

func TestSelectedPublicProviderRequiresConditionalCoreHost(t *testing.T) {
	publicPorts := []string{
		"std/context-source@v1", "std/skill-source@v1", "std/observer/run@v1",
		"std/observer/diagnostic@v1", "std/status-source@v1", "std/ui-extension@v1",
		"std/ui-root@v1", "std/control-action@v1",
	}
	for _, publicPort := range publicPorts {
		t.Run(publicPort, func(t *testing.T) {
			selected := map[string]SourceRecord{
				"fixture/provider": {Descriptor: withProvides(testDescriptor("fixture/provider"), module.PortRef{Port: publicPort, ID: "fixture.provider"})},
			}
			providers := map[string][]SourceRecord{}
			for _, portName := range []string{
				"core/loop-driver@v1", "core/chat-model-host@v1", "core/tool-host@v1",
				"core/storage-engine@v1", "core/checkpoint-store@v1",
				"core/credential-resolver@v1", "core/sandbox-backend@v1",
			} {
				providers[portName] = []SourceRecord{{}}
			}
			diagnostics := validateClosedInternalSelection(selected, providers)
			if len(diagnostics) != 1 || !strings.Contains(diagnostics[0], "requires conditional Host core/") {
				t.Fatalf("diagnostics = %v", diagnostics)
			}
		})
	}
}
