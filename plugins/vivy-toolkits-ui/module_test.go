package vivytoolkitsui

import "testing"

func TestToolkitUIIsASeparateRemovableExtension(t *testing.T) {
	descriptor := New().Descriptor()
	if descriptor.Module.ID != ModuleID || ModuleID != "vivy/toolkits-ui" {
		t.Fatalf("module ID = %q", descriptor.Module.ID)
	}
	if len(descriptor.Provides) != 1 || descriptor.Provides[0].Port != UIExtensionPort || descriptor.Provides[0].ID != ProviderID {
		t.Fatalf("UI extension surface = %#v", descriptor.Provides)
	}
}
