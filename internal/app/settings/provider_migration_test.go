package settings

import (
	"os"
	"strings"
	"testing"

	"agent-vivy/internal/provider"
)

// The three values the previous vocabulary could store translate to the sealed
// adapter that vendor's default endpoint speaks. The vendor is reported
// alongside, because an address-less document must keep resolving to the
// endpoint it selected before the migration (MIGRATION.md §3 rule 1).
func TestNormalizeProviderSelectionTranslatesLegacyVocabulary(t *testing.T) {
	for _, test := range []struct {
		stored  string
		adapter string
		vendor  string
	}{
		{ProviderDeepSeek, provider.AdapterOpenAICompletions, ProviderDeepSeek},
		{ProviderOpenAI, provider.AdapterOpenAICompletions, ProviderOpenAI},
		{ProviderAnthropic, provider.AdapterAnthropicMessages, ProviderAnthropic},
		// A value that already names an adapter carries no vendor.
		{provider.AdapterOpenAICompletions, provider.AdapterOpenAICompletions, ""},
		{provider.AdapterAnthropicMessages, provider.AdapterAnthropicMessages, ""},
		// A value this vocabulary never held passes through unchanged so
		// validation still rejects it and a read still cannot crash on it.
		{"banana", "banana", ""},
		{"", "", ""},
	} {
		got := NormalizeProviderSelection(test.stored)
		if got.Adapter != test.adapter || got.LegacyVendor != test.vendor {
			t.Fatalf("NormalizeProviderSelection(%q) = %+v, want adapter %q vendor %q",
				test.stored, got, test.adapter, test.vendor)
		}
		if NormalizeAdapter(test.stored) != test.adapter {
			t.Fatalf("NormalizeAdapter(%q) = %q, want %q", test.stored, NormalizeAdapter(test.stored), test.adapter)
		}
	}
}

// The registry key keeps the `bundle` name but carries the same vocabulary, so
// the acceptance rule and the uniqueness key both normalize it.
func TestValidProviderValueAcceptsBothVocabularies(t *testing.T) {
	for _, value := range []string{
		"", ProviderDeepSeek, ProviderOpenAI, ProviderAnthropic,
		provider.AdapterOpenAICompletions, provider.AdapterOpenAIResponses, provider.AdapterAnthropicMessages,
	} {
		if !ValidProviderValue(value) {
			t.Fatalf("ValidProviderValue(%q) = false, want true", value)
		}
	}
	for _, value := range []string{"banana", "vendor:openai", "OpenAI"} {
		if ValidProviderValue(value) {
			t.Fatalf("ValidProviderValue(%q) = true, want false", value)
		}
	}
}

func TestProviderValueErrorNamesTheSealedAdapters(t *testing.T) {
	err := ProviderValueError("provider", "banana")
	if err == nil {
		t.Fatal("want an error")
	}
	message := err.Error()
	if !strings.Contains(message, `provider "banana"`) {
		t.Fatalf("error %q must name the field and value", message)
	}
	for _, adapter := range provider.AdapterFamilies() {
		if !strings.Contains(message, adapter) {
			t.Fatalf("error %q must list the accepted adapter %q", message, adapter)
		}
	}
}

// The pre-migration vocabulary keeps its order, because the resolver reports
// the vendor a legacy value named and the Settings card offers those vendors
// until the catalog arrives (PROV-P4 replaced the pre-baked list with the
// embedded catalog).
func TestLegacyProviderAliasesKeepDeclarationOrder(t *testing.T) {
	want := []legacyProviderAlias{
		{Vendor: ProviderDeepSeek, Adapter: provider.AdapterOpenAICompletions},
		{Vendor: ProviderOpenAI, Adapter: provider.AdapterOpenAICompletions},
		{Vendor: ProviderAnthropic, Adapter: provider.AdapterAnthropicMessages},
	}
	if len(legacyProviderAliases) != len(want) {
		t.Fatalf("legacyProviderAliases = %+v, want %+v", legacyProviderAliases, want)
	}
	for i := range want {
		if legacyProviderAliases[i] != want[i] {
			t.Fatalf("legacyProviderAliases = %+v, want %+v", legacyProviderAliases, want)
		}
	}
}

// A legacy document is valid: the old spelling is a legal stored value, and
// only an unknown value is rejected on write.
func TestValidateAcceptsLegacyProviderVocabulary(t *testing.T) {
	for _, value := range []string{ProviderDeepSeek, ProviderOpenAI, ProviderAnthropic} {
		s := Settings{
			Provider: value, DefaultModel: "m",
			Providers: []ProviderEntry{{ID: "c", DisplayName: "A", Bundle: value, BaseURL: "https://a.example.com/v1"}},
		}
		if err := s.Validate(); err != nil {
			t.Fatalf("legacy document rejected for %q: %v", value, err)
		}
	}
	if err := (Settings{Provider: "banana"}).Validate(); err == nil {
		t.Fatal("an unknown provider value must be rejected on write")
	}
	if err := (Settings{Providers: []ProviderEntry{
		{ID: "c", DisplayName: "A", Bundle: "banana", BaseURL: "https://a.example.com/v1"},
	}}).Validate(); err == nil {
		t.Fatal("an unknown bundle value must be rejected on write")
	}
}

// The endpoint identity is (adapter, base_url), so the registry uniqueness rule
// must not be bypassable by spelling the same adapter two ways.
func TestProviderRegistryUniquenessIgnoresVocabularySpelling(t *testing.T) {
	s := Settings{Providers: []ProviderEntry{
		{ID: "c1", DisplayName: "A", Bundle: ProviderOpenAI, BaseURL: "https://a.example.com/v1"},
		{ID: "c2", DisplayName: "B", Bundle: provider.AdapterOpenAICompletions, BaseURL: "https://a.example.com/v1"},
	}}
	if err := s.Validate(); err == nil {
		t.Fatal("the same endpoint identity spelled two ways must collide")
	}
	// Different adapters on one address stay legal: DeepSeek serves both.
	ok := Settings{Providers: []ProviderEntry{
		{ID: "c1", DisplayName: "A", Bundle: provider.AdapterOpenAICompletions, BaseURL: "https://a.example.com/v1"},
		{ID: "c2", DisplayName: "B", Bundle: ProviderAnthropic, BaseURL: "https://a.example.com/v1"},
	}}
	if err := ok.Validate(); err != nil {
		t.Fatalf("different adapters on one address must be allowed: %v", err)
	}
}

// FindProvider and ActiveKey match on the normalized adapter, so a legacy
// registry row still supplies the credential overlay for an adapter selection.
func TestActiveKeyMatchesAcrossVocabularySpelling(t *testing.T) {
	s := Settings{
		Provider: provider.AdapterOpenAICompletions, BaseURL: "https://a.example.com/v1",
		Providers: []ProviderEntry{{
			ID: "c", DisplayName: "A", Bundle: ProviderOpenAI,
			BaseURL: "https://a.example.com/v1", ApiKey: "sk-registry",
		}},
	}
	if got := ActiveKey(s, s.Provider, s.BaseURL); got != "sk-registry" {
		t.Fatalf("ActiveKey = %q, want the legacy registry row's key", got)
	}
	if entry, ok := s.FindProvider(s.Provider, s.BaseURL); !ok || entry.ID != "c" {
		t.Fatalf("FindProvider = %+v, %v", entry, ok)
	}
}

// IsOpenAICompatibleSelection is how the model-refresh RPC decides whether a
// provider speaks the OpenAI listing protocol, so both spellings must answer.
func TestIsOpenAICompatibleSelection(t *testing.T) {
	for _, value := range []string{
		ProviderDeepSeek, ProviderOpenAI, provider.AdapterOpenAICompletions,
	} {
		if !IsOpenAICompatibleSelection(value) {
			t.Fatalf("IsOpenAICompatibleSelection(%q) = false, want true", value)
		}
	}
	for _, value := range []string{ProviderAnthropic, provider.AdapterAnthropicMessages, "banana", ""} {
		if IsOpenAICompatibleSelection(value) {
			t.Fatalf("IsOpenAICompatibleSelection(%q) = true, want false", value)
		}
	}
}

// An unusable value is rejected on write and on load, exactly as before the
// migration; the model resolver is the reader that stays non-fatal
// (MIGRATION.md §3 rule 3, covered in the app package).
func TestUnknownProviderValueIsRejectedNotInvented(t *testing.T) {
	path := t.TempDir() + "/" + FileName
	if err := os.WriteFile(path, []byte("provider: banana\ndefault_model: m\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := Load(path)
	if err == nil {
		t.Fatal("an unknown provider value must still fail validation on read")
	}
	if !strings.Contains(err.Error(), "banana") {
		t.Fatalf("error %q must name the offending value", err)
	}
	if NormalizeAdapter("banana") != "banana" {
		t.Fatal("normalization must not invent an adapter for an unknown value")
	}
	if ValidProviderValue("banana") {
		t.Fatal("an unknown value must not become valid by normalization")
	}
}
