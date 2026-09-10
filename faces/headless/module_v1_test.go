package headless

import (
	"agent-vivy/sdk/module"
	faceport "agent-vivy/sdk/port/face"
	"testing"
)

func TestV1ModuleAndProviderIdentity(t *testing.T) {
	var m module.Module = New()
	if err := m.Descriptor().Validate(); err != nil {
		t.Fatal(err)
	}
	if m.Descriptor().Module.ID != "vivy/headless" {
		t.Fatal(m.Descriptor().Module.ID)
	}
	var p faceport.FaceProvider = NewProvider()
	if p.Definition().ID != "vivy.headless" {
		t.Fatal(p.Definition())
	}
}
