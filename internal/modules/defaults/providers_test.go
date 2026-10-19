package defaults

import (
	"reflect"
	"testing"

	"agent-vivy/internal/modelhost"
	"agent-vivy/internal/provider"
	"agent-vivy/sdk/port/providerprofile"
)

// The sealed unit is the adapter, not the vendor: the compiled Generation
// carries one Profile per adapter family the embedded data reaches. The test
// name is cited as Port evidence by sdk/internal/assembly/evidence.go, so it is
// kept.
func TestDefaultProviderProfilesMatchExistingRuntimeFamilies(t *testing.T) {
	providers := ProviderProfiles()
	if len(providers) != 2 {
		t.Fatalf("ProviderProfiles() = %d entries, want 2", len(providers))
	}
	definitions := make([]providerprofile.Profile, 0, len(providers))
	for _, provider := range providers {
		profile := provider.Definition()
		if err := profile.Validate(); err != nil {
			t.Fatalf("Profile %q invalid: %v", profile.ID, err)
		}
		definitions = append(definitions, profile)
	}
	want := []string{provider.AdapterOpenAICompletions, provider.AdapterAnthropicMessages}
	got := []string{definitions[0].ID, definitions[1].ID}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Profile IDs = %v, want %v", got, want)
	}
	families := provider.AdapterFamilies()
	host, err := modelhost.New(definitions, families)
	if err != nil {
		t.Fatalf("modelhost.New: %v", err)
	}
	for _, profile := range definitions {
		if profile.AdapterFamily != profile.ID {
			t.Fatalf("Profile %q family = %q, want the adapter id", profile.ID, profile.AdapterFamily)
		}
		if _, err := host.ResolveExecutable(profile.ID); err != nil {
			t.Fatalf("Profile %q names adapter family %q, which is not executable: %v", profile.ID, profile.AdapterFamily, err)
		}
		if len(profile.ModelIDs) == 0 || len(profile.SecretRefs) == 0 {
			t.Fatalf("Profile %q carries %d model ids and %d Secret references", profile.ID, len(profile.ModelIDs), len(profile.SecretRefs))
		}
	}
	if !reflect.DeepEqual(families, want) {
		t.Fatalf("sealed adapter families = %v, want %v", families, want)
	}
}

// The union projection is data-derived: every model id and Secret reference a
// Profile declares must actually exist in the embedded vendor data, and no
// embedded endpoint may be left out.
func TestDefaultProviderProfilesCoverTheEmbeddedData(t *testing.T) {
	vendors, err := provider.LoadEmbedded()
	if err != nil {
		t.Fatalf("LoadEmbedded: %v", err)
	}
	definitions := make(map[string]providerprofile.Profile, 3)
	for _, entry := range ProviderProfiles() {
		profile := entry.Definition()
		definitions[profile.ID] = profile
	}
	for _, vendor := range vendors {
		for _, endpoint := range vendor.Endpoints {
			profile, ok := definitions[endpoint.Adapter]
			if !ok {
				t.Fatalf("vendor %q endpoint %q has no compiled Profile", vendor.Name, endpoint.BaseURL)
			}
			models := make(map[string]struct{}, len(profile.ModelIDs))
			for _, id := range profile.ModelIDs {
				models[id] = struct{}{}
			}
			for _, id := range endpoint.ModelIDs() {
				if _, ok := models[id]; !ok {
					t.Fatalf("Profile %q is missing model %q of vendor %q", profile.ID, id, vendor.Name)
				}
			}
			secrets := make(map[string]struct{}, len(profile.SecretRefs))
			for _, reference := range profile.SecretRefs {
				secrets[reference] = struct{}{}
			}
			if _, ok := secrets[vendor.EnvKey]; !ok {
				t.Fatalf("Profile %q is missing Secret reference %q of vendor %q", profile.ID, vendor.EnvKey, vendor.Name)
			}
		}
	}
}
