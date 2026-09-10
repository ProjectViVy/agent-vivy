package tui

import (
	"testing"

	"agent-vivy/sdk/module"
	faceport "agent-vivy/sdk/port/face"
)

func TestNewDelegatesCanonicalTUI(t *testing.T) {
	var m module.Module = New()
	if err := m.Descriptor().Validate(); err != nil {
		t.Fatal(err)
	}
	if m.Descriptor().Module.ID != "vivy/tui" {
		t.Fatal(m.Descriptor().Module.ID)
	}
	var p faceport.FaceProvider = NewProvider()
	if p.Definition().Kind != FaceKind {
		t.Fatal(p.Definition())
	}
}
