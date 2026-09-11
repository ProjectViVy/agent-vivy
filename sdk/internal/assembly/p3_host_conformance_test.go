package assembly

import (
	"context"
	"testing"

	"agent-vivy/sdk/module"
)

func TestP3ObserverAndStatusHostsAreSelectableCoreOwners(t *testing.T) {
	observerHost := testDescriptor("vivy/observer-host")
	observerHost.Source.Ref = "internal:vivy/observer-host"
	observerHost.Provides = []module.PortRef{{Port: "core/observer-host@v1", ID: "vivy.observer-host"}}
	statusHost := testDescriptor("vivy/status-host")
	statusHost.Source.Ref = "internal:vivy/status-host"
	statusHost.Provides = []module.PortRef{{Port: "core/status-host@v1", ID: "vivy.status-host"}}
	provider := testDescriptor("fixture/governance")
	provider.Provides = []module.PortRef{
		{Port: "std/observer/run@v1", ID: "fixture.run"},
		{Port: "std/observer/diagnostic@v1", ID: "fixture.diagnostic"},
		{Port: "std/status-source@v1", ID: "fixture.status"},
	}
	provider.Requires = []module.Requirement{
		{PortRef: module.PortRef{Port: "core/observer-host@v1"}, Provider: observerHost.Module.ID},
		{PortRef: module.PortRef{Port: "core/status-host@v1"}, Provider: statusHost.Module.ID},
	}

	compiler := fixtureCompiler(t, []module.Descriptor{observerHost, statusHost, provider})
	if _, err := compiler.Compile(context.Background(), Recipe{
		APIVersion: RecipeAPIVersionV1,
		Modules:    []string{observerHost.Module.ID, statusHost.Module.ID, provider.Module.ID},
	}); err != nil {
		t.Fatalf("valid observer/status Host graph rejected: %v", err)
	}
}
