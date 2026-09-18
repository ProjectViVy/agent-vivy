package settings

import (
	"fmt"
	"strings"

	"agent-vivy/internal/provider"
)

// A stored provider selection used to name a bundle, which was a vendor. It now
// names the sealed protocol adapter (DESIGN.md §5.1, decision D9). Reading an
// old document must never fail because of that change (MIGRATION.md §3), so the
// three values the previous vocabulary could hold are translated wherever a
// stored value is validated or used, and the next save persists the adapter.
//
// LegacyVendor is what the old value meant: that vendor's default endpoint. A
// document that names no address keeps resolving to the vendor and endpoint it
// selected before the migration, and the caller only has to fill the address in
// when it has no explicit base_url.

// ProviderSelection is one stored provider value normalized to the adapter
// vocabulary. It is a read-time projection: the stored document is never
// rewritten by loading it.
type ProviderSelection struct {
	// Adapter is the sealed protocol adapter the value names, or the empty
	// string when the document names no provider at all.
	Adapter string
	// LegacyVendor is the vendor a pre-migration value named, or the empty
	// string for a value that already names an adapter.
	LegacyVendor string
}

// legacyProviderAlias is one row of the pre-migration vocabulary: the vendor
// name older documents stored and the adapter that vendor's default endpoint
// speaks.
type legacyProviderAlias struct {
	Vendor  string
	Adapter string
}

// legacyProviderAliases holds the three vendor names older documents stored, in
// the order the previous pre-baked catalog listed them. The order is part of
// the Settings/TUI payload until PROV-P4 replaces that catalog with the whole
// embedded one.
var legacyProviderAliases = []legacyProviderAlias{
	{Vendor: "deepseek", Adapter: provider.AdapterOpenAICompletions},
	{Vendor: "openai", Adapter: provider.AdapterOpenAICompletions},
	{Vendor: "anthropic", Adapter: provider.AdapterAnthropicMessages},
}

// legacyAdapterFor returns the adapter a vendor name older documents stored
// names, and whether the value was one of them.
func legacyAdapterFor(stored string) (string, bool) {
	for _, alias := range legacyProviderAliases {
		if alias.Vendor == stored {
			return alias.Adapter, true
		}
	}
	return "", false
}

// LegacyVendorNames lists the vendors the pre-migration settings vocabulary
// could name. They are the address-less selections an old document can still
// hold, so they are the vendors the pre-baked Settings/TUI catalog must keep
// offering until the UI reads the embedded catalog itself (PROV-P4).
func LegacyVendorNames() []string {
	names := make([]string, 0, len(legacyProviderAliases))
	for _, alias := range legacyProviderAliases {
		names = append(names, alias.Vendor)
	}
	return names
}

// NormalizeProviderSelection translates one stored provider value. A value that
// already names a sealed adapter passes through unchanged; a value this
// vocabulary never held also passes through, so validation still rejects it on
// write and an old document still loads (rule 3).
func NormalizeProviderSelection(stored string) ProviderSelection {
	if adapter, legacy := legacyAdapterFor(stored); legacy {
		return ProviderSelection{Adapter: adapter, LegacyVendor: stored}
	}
	return ProviderSelection{Adapter: stored}
}

// NormalizeAdapter is NormalizeProviderSelection for callers that only need the
// adapter, such as the registry-entry uniqueness key.
func NormalizeAdapter(stored string) string {
	return NormalizeProviderSelection(stored).Adapter
}

// ValidProviderValue reports whether a stored or normalized value names a
// sealed protocol adapter. It is the write-time rule for both
// `Settings.Provider` and a registry entry's `bundle`: a vendor name older
// documents hold is accepted, an unknown name is rejected.
func ValidProviderValue(stored string) bool {
	if stored == "" {
		return true
	}
	if _, legacy := legacyAdapterFor(stored); legacy {
		return true
	}
	return provider.IsSealedAdapter(stored)
}

// ProviderValueError is the one error message for an unusable provider value,
// so every rejection names the accepted vocabulary.
func ProviderValueError(field, value string) error {
	return fmt.Errorf("settings: %s %q unsupported; want one of %s", field, value, strings.Join(provider.AdapterFamilies(), ", "))
}
