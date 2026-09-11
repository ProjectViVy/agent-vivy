package ui

import "testing"

func TestPinnedPackageExposesAuthoritativeIdentity(t *testing.T) {
	metadata, err := PinnedPackage()
	if err != nil {
		t.Fatal(err)
	}
	if metadata.Name != "@vivy/ui-sdk" {
		t.Fatalf("package name = %q, want @vivy/ui-sdk", metadata.Name)
	}
	if metadata.Version != "1.0.0" {
		t.Fatalf("package version = %q, want 1.0.0", metadata.Version)
	}
}
