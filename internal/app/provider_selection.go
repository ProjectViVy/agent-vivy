package app

import (
	"strings"

	"agent-vivy/internal/app/settings"
	"agent-vivy/internal/config"
	"agent-vivy/internal/provider"
)

// The selection model after PROV-P3 (DESIGN.md §5):
//
//	vendor   the credential owner and the owner of the default address.
//	adapter  the sealed protocol, from the stored selection or the endpoint.
//	address  the endpoint identity's second half; empty means the endpoint the
//	         vendor declares for the adapter.
//	model    the stored model, or the endpoint's default model.
//
// The vendor is never named by a new-style selection: it comes from an explicit
// base_url that an embedded endpoint declares, from the vendor a pre-migration
// document named, or from config.providers.active. That is what lets a
// third-party vendor work with no per-vendor configuration block.

// providerSelection is one effective model selection resolved against the
// embedded catalog.
type providerSelection struct {
	Vendor   string
	Adapter  string
	BaseURL  string
	Model    string
	endpoint provider.Endpoint
}

// resolveStoredSelection maps a stored `provider`/`base_url`/`default_model`
// triple onto the embedded data. The bool reports whether the selection names
// anything at all; an unusable value yields the zero selection so an old
// document stays non-fatal on read (MIGRATION.md §3 rule 3).
func resolveStoredSelection(catalog *provider.Catalog, cfg config.Config, stored settings.Settings) (providerSelection, bool) {
	if stored.IsZero() {
		return providerSelection{}, false
	}
	selection := settings.NormalizeProviderSelection(stored.Provider)
	vendorName := ""
	if catalog != nil {
		// An explicit address identifies the vendor that declares that
		// endpoint identity, which is the third-party case: no config block,
		// no vendor name, just an address the data knows.
		if vendor, _, ok := catalog.VendorForEndpoint(selection.Adapter, strings.TrimSpace(stored.BaseURL)); ok {
			vendorName = vendor.Name
		}
	}
	if vendorName == "" {
		vendorName = selection.LegacyVendor
	}
	if vendorName == "" {
		vendorName = cfg.Providers.Active
	}
	return resolveVendorSelection(catalog, vendorName, selection.Adapter, stored.BaseURL, stored.DefaultModel)
}

// resolveVendorSelection resolves one vendor's endpoint variant. It is shared
// by the stored selection and the frozen ENV session so both name the same
// endpoint for the same triple.
func resolveVendorSelection(catalog *provider.Catalog, vendorName, adapter, baseURL, modelID string) (providerSelection, bool) {
	if catalog == nil {
		return providerSelection{Vendor: vendorName, Adapter: adapter, BaseURL: baseURL, Model: modelID}, vendorName != ""
	}
	endpoint, vendor, err := catalog.EndpointForVendor(vendorName, adapter, baseURL)
	if err != nil {
		// An unusable selection is not fatal on read: the caller reports it as
		// not ready and the user can fix it in Settings.
		return providerSelection{Vendor: vendorName, Adapter: adapter, BaseURL: baseURL, Model: modelID}, false
	}
	if modelID == "" {
		modelID = endpoint.DefaultModel
	}
	return providerSelection{
		Vendor:   vendor.Name,
		Adapter:  endpoint.Adapter,
		BaseURL:  baseURL,
		Model:    modelID,
		endpoint: endpoint,
	}, true
}

// configDefaultSelection is the selection `config.providers.active` alone
// describes: the vendor's default endpoint and its default model. It is the
// pair the control plane offers as "the configuration default" and the model
// the runtime reports before any settings document exists.
func configDefaultSelection(catalog *provider.Catalog, cfg config.Config) providerSelection {
	return resolveVendorSelectionBestEffort(catalog, cfg.Providers.Active)
}

// resolveVendorSelectionBestEffort resolves a vendor's default endpoint without
// an adapter or an address, which is always expressible: every embedded vendor
// declares at least one endpoint.
func resolveVendorSelectionBestEffort(catalog *provider.Catalog, vendorName string) providerSelection {
	if catalog == nil {
		return providerSelection{Vendor: vendorName}
	}
	vendor, ok := catalog.Vendor(vendorName)
	if !ok {
		return providerSelection{Vendor: vendorName}
	}
	endpoint, ok := vendor.DefaultEndpoint()
	if !ok {
		return providerSelection{Vendor: vendorName}
	}
	return providerSelection{
		Vendor:   vendor.Name,
		Adapter:  endpoint.Adapter,
		Model:    endpoint.DefaultModel,
		endpoint: endpoint,
	}
}

// resolveSettingsAPIKey resolves the credential overlay for the live
// selection: the registry entry matching the endpoint identity, or the legacy
// `api_key` field. Empty means "no overlay" and the vendor's environment
// variable stands.
func resolveSettingsAPIKey(s settings.Settings, selection providerSelection) string {
	return settings.ActiveKey(s, selection.Adapter, s.BaseURL)
}
