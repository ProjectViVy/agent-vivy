package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCanonicalPackedTUIProjectRoot(t *testing.T) {
	realParent := t.TempDir()
	realProject := filepath.Join(realParent, "project")
	if err := os.Mkdir(realProject, 0o700); err != nil {
		t.Fatal(err)
	}
	linkBase := t.TempDir()
	linkedParent := filepath.Join(linkBase, "linked-parent")
	if err := os.Symlink(realParent, linkedParent); err != nil {
		t.Skipf("directory symlink creation unavailable: %v", err)
	}
	got, err := canonicalPackedTUIProjectRoot(filepath.Join(linkedParent, "project"))
	if err != nil {
		t.Fatal(err)
	}
	want, _ := filepath.EvalSymlinks(realProject)
	want, _ = filepath.Abs(want)
	if got != want {
		t.Fatalf("project root = %q, want %q", got, want)
	}
}
