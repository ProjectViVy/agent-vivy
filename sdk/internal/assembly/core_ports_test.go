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
