package modelhost

import (
	"encoding/json"
	"errors"
	"reflect"
	"slices"
	"strings"
	"testing"

	"agent-vivy/sdk/port/providerprofile"
)

func testProfile(id, family string) providerprofile.Profile {
	return providerprofile.Profile{
		ID: id, AdapterFamily: family, ModelIDs: []string{"model-1"},
		EndpointClass: providerprofile.EndpointGateway,
		SecretRefs:    []string{"TEST_API_KEY"},
		OptionsSchema: json.RawMessage(`{"type":"object"}`),
	}
}

func TestNewRejectsDuplicateProfileIDs(t *testing.T) {
	_, err := New([]providerprofile.Profile{
		testProfile("duplicate", "openai-compatible"),
		testProfile("duplicate", "openai-compatible"),
	}, Capabilities{"openai-compatible": CapabilitySupported})
	if err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("New() error = %v, want duplicate profile", err)
	}
}

func TestNewRejectsUnsupportedAdapterFamily(t *testing.T) {
	_, err := New([]providerprofile.Profile{testProfile("unknown", "not-pinned")}, Capabilities{
		"openai-compatible": CapabilitySupported,
	})
	if err == nil || !strings.Contains(err.Error(), "unsupported adapter family") {
		t.Fatalf("New() error = %v, want unsupported adapter family", err)
	}
}

func TestResolveExecutableRejectsDeferredAdapter(t *testing.T) {
	host, err := New([]providerprofile.Profile{testProfile("future", "native-future")}, Capabilities{
		"native-future": CapabilityDeferredIndefinite,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	_, err = host.ResolveExecutable("future")
	if !errors.Is(err, ErrAdapterUnavailable) {
		t.Fatalf("ResolveExecutable() error = %v, want ErrAdapterUnavailable", err)
	}
}

func TestProfilesReturnsDefensiveDeclarativeCopy(t *testing.T) {
	host, err := New([]providerprofile.Profile{testProfile("openai", "openai-compatible")}, Capabilities{
		"openai-compatible": CapabilitySupported,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	profiles := host.Profiles()
	profiles[0].ModelIDs[0] = "mutated"
	profiles[0].SecretRefs[0] = "MUTATED"
	profiles[0].OptionsSchema[0] = '['
	resolved, err := host.ResolveExecutable("openai")
	if err != nil {
		t.Fatalf("ResolveExecutable() error = %v", err)
	}
	if resolved.ModelIDs[0] != "model-1" || resolved.SecretRefs[0] != "TEST_API_KEY" || string(resolved.OptionsSchema) != `{"type":"object"}` {
		t.Fatalf("host profile was mutated through returned copy: %#v", resolved)
	}
}

func TestProfileStatusesProjectCompiledRuntimeTruthWithoutSecrets(t *testing.T) {
	host, err := New([]providerprofile.Profile{
		testProfile("active", "openai-compatible"),
		testProfile("compiled", "openai-compatible"),
		testProfile("broken", "openai-compatible"),
		testProfile("future", "native-future"),
	}, Capabilities{
		"openai-compatible": CapabilitySupported,
		"native-future":     CapabilityDeferredIndefinite,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	host.MarkUnavailable("broken")

	statuses := host.Statuses("active", true)
	got := make(map[string]ProfileState, len(statuses))
	for _, status := range statuses {
		got[status.ID] = status.State
	}
	want := map[string]ProfileState{
		"active":   ProfileReady,
		"compiled": ProfileCompiled,
		"broken":   ProfileUnavailable,
		"future":   ProfileDeferredIndefinite,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Statuses() states = %#v, want %#v", got, want)
	}

	unconfigured := host.Statuses("active", false)
	if unconfigured[0].ID != "active" || unconfigured[0].State != ProfileUnconfigured {
		t.Fatalf("active unconfigured status = %#v", unconfigured[0])
	}
	if fields := reflect.VisibleFields(reflect.TypeOf(ProfileStatus{})); slices.ContainsFunc(fields, func(field reflect.StructField) bool {
		return field.Name == "SecretRefs" || field.Name == "OptionsSchema"
	}) {
		t.Fatalf("ProfileStatus exposes secret or configuration schema fields: %#v", fields)
	}
}
