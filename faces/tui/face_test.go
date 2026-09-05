package tui

import (
	"testing"

	"agent-vivy/sdk/plugin"
)

func TestNewDelegatesCanonicalTUI(t *testing.T) {
	face := New(plugin.FaceOptions{})
	if face == nil || face.Kind() != FaceKind {
		t.Fatalf("face = %#v", face)
	}
}
