package assembly

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agent-vivy/sdk/module"
)

func TestSourceCatalogAssignsTrustOutsideDescriptor(t *testing.T) {
	descriptor := testDescriptor("fixture/search")
	catalog, err := NewSourceCatalog([]SourceRecord{{Descriptor: descriptor, Trust: TrustT2}})
	if err != nil {
		t.Fatalf("NewSourceCatalog() error = %v", err)
	}

	resolved, err := catalog.Resolve("fixture/search")
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if resolved.Trust != TrustT2 {
		t.Fatalf("resolved trust = %q, want %q", resolved.Trust, TrustT2)
	}
	if resolved.Descriptor.Module.ID != descriptor.Module.ID {
		t.Fatalf("resolved module = %q, want %q", resolved.Descriptor.Module.ID, descriptor.Module.ID)
	}
}

func TestSourceCatalogRejectsRefAndInvalidRoots(t *testing.T) {
	root := t.TempDir()
	descriptor := testDescriptor("fixture/source")
	descriptor.Source = module.Source{Ref: "repo:fixture/source"}
	record := SourceRecord{Descriptor: descriptor, Trust: TrustT1, Root: root, Ref: descriptor.Source.Ref}
	if _, err := NewSourceCatalog([]SourceRecord{record}); err != nil {
		t.Fatal(err)
	}
	record.Ref = "repo:other/source"
	if _, err := NewSourceCatalog([]SourceRecord{record}); err == nil || !strings.Contains(err.Error(), "source ref mismatch") {
		t.Fatalf("reference drift error = %v", err)
	}
	record.Ref = descriptor.Source.Ref
	record.Root = filepath.Join(root, "absent")
	if _, err := NewSourceCatalog([]SourceRecord{record}); err == nil {
		t.Fatal("missing source root accepted")
	}
	record.Root = filepath.Join(root, "module.go")
	if err := os.WriteFile(record.Root, []byte("package fixture\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := NewSourceCatalog([]SourceRecord{record}); err == nil {
		t.Fatal("file source root accepted")
	}
}

func TestSourceCatalogAcceptsChangedBytesWithoutHashClaim(t *testing.T) {
	root := t.TempDir()
	filename := filepath.Join(root, "module.go")
	descriptor := testDescriptor("fixture/source")
	record := SourceRecord{Descriptor: descriptor, Trust: TrustT1, Root: root, Ref: descriptor.Source.Ref}
	for _, body := range []string{"package fixture\n", "package fixture\n// updated\n"} {
		if err := os.WriteFile(filename, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		catalog, err := NewSourceCatalog([]SourceRecord{record})
		if err != nil {
			t.Fatal(err)
		}
		resolved, err := catalog.Resolve(descriptor.Module.ID)
		if err != nil {
			t.Fatal(err)
		}
		if resolved.Descriptor.Source.Ref != record.Ref || resolved.Root != root || resolved.Trust != TrustT1 {
			t.Fatalf("source ownership changed: %#v", resolved)
		}
	}
}

func TestSourceCatalogRejectsAmbiguousAndMissingSources(t *testing.T) {
	descriptor := testDescriptor("fixture/search")
	if _, err := NewSourceCatalog([]SourceRecord{
		{Descriptor: descriptor, Trust: TrustT1},
		{Descriptor: descriptor, Trust: TrustT2},
	}); err == nil {
		t.Fatal("NewSourceCatalog() accepted an ambiguous module source")
	}

	catalog, err := NewSourceCatalog(nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := catalog.Resolve("fixture/missing"); err == nil {
		t.Fatal("Resolve() accepted a missing module source")
	}
}

func testDescriptor(id string) module.Descriptor {
	return module.Descriptor{
		APIVersion: module.APIVersionV1,
		Module:     module.Identity{ID: id, Version: "1.0.0"},
		Source: module.Source{
			Ref: "git:" + id + "@0123456",
		},
		Provides:        []module.PortRef{},
		Requires:        []module.Requirement{},
		RequestedGrants: []module.Grant{},
		Lifecycle:       module.Lifecycle{Scope: module.ScopeGeneration},
	}
}
