package vivymemory

import (
	"os"
	"strings"
	"testing"

	"agent-vivy/sdk/module"
)

// TestDescriptorOwnsOneUISidebarExtension pins the whole contribution surface:
// a UI Module that grows a backend Port, a Grant, or a Requirement is a design
// change, not a refactor.
func TestDescriptorOwnsOneUISidebarExtension(t *testing.T) {
	descriptor := New().Descriptor()
	if descriptor.APIVersion != module.APIVersionV1 {
		t.Fatalf("apiVersion = %q, want %q", descriptor.APIVersion, module.APIVersionV1)
	}
	if descriptor.Module.ID != ModuleID {
		t.Fatalf("module id = %q, want %q", descriptor.Module.ID, ModuleID)
	}
	if len(descriptor.Provides) != 1 || descriptor.Provides[0].Port != UIExtensionPort || descriptor.Provides[0].ID != ProviderID {
		t.Fatalf("provides = %#v, want one %s/%s", descriptor.Provides, UIExtensionPort, ProviderID)
	}
	if len(descriptor.Requires) != 0 || len(descriptor.Optional) != 0 || len(descriptor.Conflicts) != 0 {
		t.Fatalf("UI Module declared backend port relations: %#v", descriptor)
	}
	if len(descriptor.RequestedGrants) != 0 {
		t.Fatalf("UI Module requested grants: %#v", descriptor.RequestedGrants)
	}
	if descriptor.Lifecycle.Scope != module.ScopeGeneration {
		t.Fatalf("lifecycle scope = %q, want %q", descriptor.Lifecycle.Scope, module.ScopeGeneration)
	}
	if descriptor.I18N == nil {
		t.Fatal("descriptor declares no i18n catalog")
	}
	if _, err := os.Stat(descriptor.I18N.Catalog); err != nil {
		t.Fatalf("declared catalog %s is missing: %v", descriptor.I18N.Catalog, err)
	}
}

// TestDeclarationMatchesDescriptor keeps vivy-module.yaml, the build-owned
// declaration the Source Catalog reads, from drifting away from the compiled
// Descriptor the runtime assembly reads.
func TestDeclarationMatchesDescriptor(t *testing.T) {
	raw, err := os.ReadFile("vivy-module.yaml")
	if err != nil {
		t.Fatal(err)
	}
	declaration := string(raw)
	descriptor := New().Descriptor()
	for _, want := range []string{
		"id: " + ModuleID,
		"version: " + descriptor.Module.Version,
		"port: " + UIExtensionPort + ", id: " + ProviderID,
		"ref: " + descriptor.Source.Ref,
		"sha256: " + descriptor.Source.SHA256,
		"catalog: " + descriptor.I18N.Catalog,
		"scope: " + string(descriptor.Lifecycle.Scope),
	} {
		if !strings.Contains(declaration, want) {
			t.Errorf("vivy-module.yaml does not declare %q", want)
		}
	}
}
