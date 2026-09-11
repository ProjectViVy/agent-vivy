package modelhost_test

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"agent-vivy/internal/modelhost"
	"agent-vivy/sdk/port/providerprofile"
)

func conformanceProfile(id, family string) providerprofile.Profile {
	return providerprofile.Profile{
		ID: id, AdapterFamily: family, ModelIDs: []string{"raw/model-id"},
		EndpointClass: providerprofile.EndpointGateway,
		SecretRefs:    []string{"CONFORMANCE_API_KEY"},
		OptionsSchema: json.RawMessage(`{"type":"object"}`),
	}
}

func TestModelHostConformanceRejectsInvalidAndMissingProfiles(t *testing.T) {
	supported := modelhost.Capabilities{"openai-compatible": modelhost.CapabilitySupported}
	if _, err := modelhost.New([]providerprofile.Profile{
		conformanceProfile("duplicate", "openai-compatible"),
		conformanceProfile("duplicate", "openai-compatible"),
	}, supported); err == nil {
		t.Fatal("duplicate Profile IDs were accepted")
	}
	if _, err := modelhost.New([]providerprofile.Profile{
		conformanceProfile("unsupported", "custom-executable"),
	}, supported); err == nil {
		t.Fatal("unsupported executable adapter family was accepted")
	}
	host, err := modelhost.New([]providerprofile.Profile{
		conformanceProfile("openai", "openai-compatible"),
	}, supported)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := host.ResolveExecutable("missing"); !errors.Is(err, modelhost.ErrProfileNotFound) {
		t.Fatalf("missing Profile error = %v, want ErrProfileNotFound", err)
	}
}

func TestModelHostConformanceStatusInspectionIsPassiveAndSecretFree(t *testing.T) {
	const canary = "SECRET_CANARY_MUST_NOT_CROSS_STATUS"
	profile := conformanceProfile("openai", "openai-compatible")
	profile.SecretRefs = []string{canary}
	host, err := modelhost.New([]providerprofile.Profile{profile}, modelhost.Capabilities{
		"openai-compatible": modelhost.CapabilitySupported,
	})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(host.Statuses("openai", false))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), canary) || strings.Contains(string(raw), "OptionsSchema") {
		t.Fatalf("status inspection leaked declarative configuration: %s", raw)
	}
	if got := host.Statuses("openai", false)[0].State; got != modelhost.ProfileUnconfigured {
		t.Fatalf("unconfigured Profile status = %s", got)
	}
}
