package app

import (
	"context"
	"testing"

	genassembly "agent-vivy/internal/generated/assembly"
	"agent-vivy/sdk/generation"
)

func TestMaskManagerForAssemblyIsAbsentWhenCapabilityIsOmitted(t *testing.T) {
	assembly := genassembly.RuntimeAssembly{
		Manifest: generation.Manifest{Modules: []string{"vivy/storage"}},
	}
	manager, err := maskManagerForAssembly(context.Background(), assembly, nil, "generation-1", true)
	if err != nil {
		t.Fatal(err)
	}
	if manager != nil {
		t.Fatal("mask manager was constructed for an omitted capability")
	}
}

func TestMaskManagerForAssemblyIsDormantWhenUnsealed(t *testing.T) {
	assembly := genassembly.RuntimeAssembly{
		Manifest: generation.Manifest{Modules: []string{"vivy/masks"}},
	}
	manager, err := maskManagerForAssembly(context.Background(), assembly, nil, "", false)
	if err != nil {
		t.Fatal(err)
	}
	if manager != nil {
		t.Fatal("unsealed composition constructed a mask manager")
	}
}

func TestMaskManagerForAssemblyFailsClosedWithoutTypedFactory(t *testing.T) {
	assembly := genassembly.RuntimeAssembly{
		Manifest: generation.Manifest{Modules: []string{"vivy/masks"}},
	}
	if _, err := maskManagerForAssembly(context.Background(), assembly, nil, "generation-1", true); err == nil {
		t.Fatal("selected mask capability without a factory was accepted")
	}
}
