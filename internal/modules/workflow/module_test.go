package workflow

import (
	"testing"

	"agent-vivy/sdk/module"
	toolport "agent-vivy/sdk/port/tool"
)

func TestModuleDescriptorAndToolInventoryAreSealed(t *testing.T) {
	descriptor := NewModule().Descriptor()
	if descriptor.Module.ID != ID || len(descriptor.Provides) != 7 {
		t.Fatalf("descriptor = %+v", descriptor)
	}
	if descriptor.Provides[0] != (module.PortRef{Port: Port, ID: "vivy.workflow-host"}) {
		t.Fatalf("workflow host provider = %+v", descriptor.Provides[0])
	}
	providers := ToolProviders()
	if len(providers) != 6 {
		t.Fatalf("tool provider count = %d, want 6", len(providers))
	}
	for _, provider := range providers {
		if provider == nil || provider.Definition().ID == "" || len(provider.Definition().Schema) == 0 {
			t.Fatalf("invalid workflow provider: %#v", provider)
		}
		if provider.Definition().Effect != toolport.EffectRead && provider.Definition().Effect != toolport.EffectWrite {
			t.Fatalf("invalid effect for %s", provider.Definition().ID)
		}
	}
}
