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

func TestSourceCatalogRejectsTreeAndReferenceDrift(t *testing.T) {
	root := t.TempDir()
	filename := filepath.Join(root, "module.go")
	if err := os.WriteFile(filename, []byte("package fixture\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	digest, err := HashSourceTree(root, "")
	if err != nil {
		t.Fatal(err)
	}
	descriptor := testDescriptor("fixture/source")
	descriptor.Source = module.Source{Ref: "repo:fixture/source", SHA256: digest}
	record := SourceRecord{Descriptor: descriptor, Trust: TrustT1, Root: root, Ref: descriptor.Source.Ref}
	if _, err := NewSourceCatalog([]SourceRecord{record}); err != nil {
		t.Fatal(err)
	}
	record.Ref = "repo:other/source"
	if _, err := NewSourceCatalog([]SourceRecord{record}); err == nil || !strings.Contains(err.Error(), "source ref mismatch") {
		t.Fatalf("reference drift error = %v", err)
	}
	record.Ref = descriptor.Source.Ref
	if err := os.WriteFile(filename, []byte("package fixture\n// drift\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := NewSourceCatalog([]SourceRecord{record}); err == nil || !strings.Contains(err.Error(), "source hash mismatch") {
		t.Fatalf("tree drift error = %v", err)
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

func TestHashSourceTreeRejectsSymbolicLinks(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "secret.go")
	if err := os.WriteFile(outside, []byte("package secret\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "linked.go")); err != nil {
		t.Fatal(err)
	}
	if _, err := HashSourceTree(root, ""); err == nil || !strings.Contains(err.Error(), "symbolic link") {
		t.Fatalf("HashSourceTree() error = %v, want symbolic link rejection", err)
	}
}

func TestHashSourceTreeNormalizesTextLineEndings(t *testing.T) {
	lfRoot := t.TempDir()
	crlfRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(lfRoot, "module.go"), []byte("package fixture\n\nvar value = 1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(crlfRoot, "module.go"), []byte("package fixture\r\n\r\nvar value = 1\r\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	lfDigest, err := HashSourceTree(lfRoot, "")
	if err != nil {
		t.Fatal(err)
	}
	crlfDigest, err := HashSourceTree(crlfRoot, "")
	if err != nil {
		t.Fatal(err)
	}
	if crlfDigest != lfDigest {
		t.Fatalf("CRLF digest = %s, want canonical LF digest %s", crlfDigest, lfDigest)
	}
}

func TestHashSourceTreePreservesBinaryLineEndings(t *testing.T) {
	lfRoot := t.TempDir()
	crlfRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(lfRoot, "asset.bin"), []byte{0, 0xff, '\n'}, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(crlfRoot, "asset.bin"), []byte{0, 0xff, '\r', '\n'}, 0o600); err != nil {
		t.Fatal(err)
	}

	lfDigest, err := HashSourceTree(lfRoot, "")
	if err != nil {
		t.Fatal(err)
	}
	crlfDigest, err := HashSourceTree(crlfRoot, "")
	if err != nil {
		t.Fatal(err)
	}
	if crlfDigest == lfDigest {
		t.Fatalf("binary CRLF digest = %s, want a distinct byte-exact digest", crlfDigest)
	}
}

func testDescriptor(id string) module.Descriptor {
	return module.Descriptor{
		APIVersion: module.APIVersionV1,
		Module:     module.Identity{ID: id, Version: "1.0.0"},
		Source: module.Source{
			Ref:    "git:" + id + "@0123456",
			SHA256: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		},
		Provides:        []module.PortRef{},
		Requires:        []module.Requirement{},
		RequestedGrants: []module.Grant{},
		Lifecycle:       module.Lifecycle{Scope: module.ScopeGeneration},
	}
}
