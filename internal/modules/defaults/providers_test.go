package defaults

import (
	"reflect"
	"testing"

	"agent-vivy/sdk/port/providerprofile"
)

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
	if got := []string{definitions[0].ID, definitions[1].ID}; !reflect.DeepEqual(got, []string{"openai", "anthropic"}) {
		t.Fatalf("Profile IDs = %v", got)
	}
	if definitions[0].AdapterFamily != "openai-compatible" || definitions[1].AdapterFamily != "anthropic" {
		t.Fatalf("adapter families = %q/%q", definitions[0].AdapterFamily, definitions[1].AdapterFamily)
	}
	if !reflect.DeepEqual(definitions[0].SecretRefs, []string{"OPENAI_API_KEY"}) || !reflect.DeepEqual(definitions[1].SecretRefs, []string{"ANTHROPIC_API_KEY"}) {
		t.Fatalf("Secret references = %v/%v", definitions[0].SecretRefs, definitions[1].SecretRefs)
	}
}
