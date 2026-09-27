package masks

import (
	"context"
	"testing"

	"agent-vivy/sdk/module"
)

func TestMaskModuleDescriptorIsClosedBackendOnly(t *testing.T) {
	descriptor := NewModule().Descriptor()
	if descriptor.Module.ID != ID {
		t.Fatalf("module id = %q, want %q", descriptor.Module.ID, ID)
	}
	if len(descriptor.Provides) != 8 || descriptor.Provides[0] != (module.PortRef{Port: Port, ID: "vivy.mask-service"}) {
		t.Fatalf("provides = %#v", descriptor.Provides)
	}
	for _, provided := range descriptor.Provides[1:] {
		if provided.Port != "std/control-action@v1" || provided.ID == "" {
			t.Fatalf("invalid mask action PortRef: %#v", provided)
		}
	}
	if len(descriptor.Requires) != 0 || len(descriptor.Optional) != 0 {
		t.Fatalf("mask module unexpectedly requires another module: requires=%#v optional=%#v", descriptor.Requires, descriptor.Optional)
	}
	for _, provided := range descriptor.Provides {
		if provided.Port == "std/ui-extension@v1" || provided.Port == "std/ui-root@v1" {
			t.Fatalf("mask backend unexpectedly provides UI Port %s", provided.Port)
		}
	}
	if err := descriptor.Validate(); err != nil {
		t.Fatalf("descriptor validation failed: %v", err)
	}
}

func TestMaskModuleConstructIsPureAndLifecycleSafe(t *testing.T) {
	instance, err := NewModule().Construct(context.Background(), testHost{})
	if err != nil {
		t.Fatal(err)
	}
	if err := instance.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := instance.Ready(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := instance.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := instance.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
}

type testHost struct{}

func (testHost) ModuleID() string { return ID }
