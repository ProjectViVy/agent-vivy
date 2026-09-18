package defaults

import (
	"reflect"
	"testing"

	"agent-vivy/sdk/port/providerprofile"
)

func TestDefaultProviderProfilesMatchExistingRuntimeFamilies(t *testing.T) {
	providers := ProviderProfiles()
	if len(providers) != 3 {
		t.Fatalf("ProviderProfiles() = %d entries, want 3", len(providers))
	}
	definitions := make([]providerprofile.Profile, 0, len(providers))
	for _, provider := range providers {
		profile := provider.Definition()
		if err := profile.Validate(); err != nil {
			t.Fatalf("Profile %q invalid: %v", profile.ID, err)
		}
		definitions = append(definitions, profile)
	}
	if got := []string{definitions[0].ID, definitions[1].ID, definitions[2].ID}; !reflect.DeepEqual(got, []string{"deepseek", "openai", "anthropic"}) {
		t.Fatalf("Profile IDs = %v, want deepseek first", got)
	}
	if definitions[0].AdapterFamily != "openai-compatible" || definitions[1].AdapterFamily != "openai-compatible" || definitions[2].AdapterFamily != "anthropic" {
		t.Fatalf("adapter families = %q/%q/%q", definitions[0].AdapterFamily, definitions[1].AdapterFamily, definitions[2].AdapterFamily)
	}
	if !reflect.DeepEqual(definitions[0].SecretRefs, []string{"DEEPSEEK_API_KEY"}) || !reflect.DeepEqual(definitions[1].SecretRefs, []string{"OPENAI_API_KEY"}) || !reflect.DeepEqual(definitions[2].SecretRefs, []string{"ANTHROPIC_API_KEY"}) {
		t.Fatalf("Secret references = %v/%v/%v", definitions[0].SecretRefs, definitions[1].SecretRefs, definitions[2].SecretRefs)
	}
}
