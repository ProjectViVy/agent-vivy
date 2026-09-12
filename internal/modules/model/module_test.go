package model

import (
	"encoding/json"
	"errors"
	"testing"

	"agent-vivy/internal/modelhost"
	"agent-vivy/sdk/port/providerprofile"
)

func TestModelModuleComposesExistingModelHost(t *testing.T) {
	profile := providerprofile.Profile{
		ID: "openai", AdapterFamily: "openai-compatible", ModelIDs: []string{"gpt-test"},
		EndpointClass: providerprofile.EndpointGateway, SecretRefs: []string{"OPENAI_API_KEY"},
		OptionsSchema: json.RawMessage(`{"type":"object"}`),
	}
	provider, err := Compose([]providerprofile.Profile{profile}, modelhost.Capabilities{
		"openai-compatible": modelhost.CapabilitySupported,
	})
	if err != nil {
		t.Fatal(err)
	}
	resolved, err := provider.Host().ResolveExecutable("openai")
	if err != nil {
		t.Fatal(err)
	}
	if resolved.ModelIDs[0] != "gpt-test" {
		t.Fatalf("resolved Profile = %#v", resolved)
	}
}

func TestModelModuleKeepsModelHostValidation(t *testing.T) {
	_, err := Compose([]providerprofile.Profile{{ID: "invalid"}}, nil)
	if err == nil || errors.Is(err, modelhost.ErrProfileNotFound) {
		t.Fatalf("Compose() error = %v, want Profile validation error", err)
	}
}

func TestModelModuleOwnsCanonicalCorePort(t *testing.T) {
	descriptor := NewModule().Descriptor()
	if descriptor.Module.ID != ID || len(descriptor.Provides) != 1 || descriptor.Provides[0].Port != Port {
		t.Fatalf("descriptor = %#v", descriptor)
	}
}
