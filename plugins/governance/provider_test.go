package governance

import (
	"context"
	"testing"

	"agent-vivy/sdk/module"
	"agent-vivy/sdk/port/observer"
	"agent-vivy/sdk/port/pretool"
	statusport "agent-vivy/sdk/port/status"
)

func TestReferenceProviderConformance(t *testing.T) {
	owner := New()
	descriptor := owner.Descriptor()
	if descriptor.Module.ID != "vivy/governance-reference" || len(descriptor.Provides) != 4 {
		t.Fatalf("Descriptor() = %#v", descriptor)
	}
	instance, err := owner.Construct(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	for name, operation := range map[string]func(context.Context) error{
		"start": instance.Start, "ready": instance.Ready, "stop": instance.Stop, "close": instance.Close,
	} {
		if err := operation(context.Background()); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
	}

	provider := NewProvider()
	if provider.ID() != "vivy.governance.reference" {
		t.Fatalf("Provider ID = %q", provider.ID())
	}
	decision, err := provider.Evaluate(context.Background(), pretool.Request{ToolID: "fixture.tool"})
	if err != nil || decision.Kind != pretool.Pass {
		t.Fatalf("Evaluate() = %#v, %v", decision, err)
	}
	if err := provider.ObserveRun(context.Background(), observer.RunEvent{}); err != nil {
		t.Fatalf("ObserveRun() = %v", err)
	}
	provider.ObserveDiagnostic(context.Background(), observer.Diagnostic{})
	snapshot, err := provider.Status(context.Background(), statusport.Request{})
	if err != nil || !snapshot.Available || snapshot.Revision != "reference-v1" || len(snapshot.Items) != 1 || snapshot.Items[0].State != "ready" {
		t.Fatalf("Status() = %#v, %v", snapshot, err)
	}
}

var (
	_ module.Module               = vivyModule{}
	_ module.Instance             = moduleInstance{}
	_ pretool.Provider            = (*Provider)(nil)
	_ observer.RunProvider        = (*Provider)(nil)
	_ observer.DiagnosticProvider = (*Provider)(nil)
	_ statusport.Provider         = (*Provider)(nil)
)
