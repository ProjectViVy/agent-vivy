package provider

import (
	"context"
	"fmt"
	"sort"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/modelhost"
)

// Catalog resolves a provider selection to live Refs over the embedded vendor
// data. It holds no bundle document and no file path: the data is part of the
// binary (D3), so a Catalog cannot be pointed at a different source at
// runtime.
type Catalog struct {
	vendors map[string]Vendor
}

// NewCatalog indexes the given vendors by name.
func NewCatalog(vendors ...Vendor) *Catalog {
	catalog := &Catalog{vendors: make(map[string]Vendor, len(vendors))}
	for _, vendor := range vendors {
		catalog.vendors[vendor.Name] = vendor
	}
	return catalog
}

// Vendor returns the embedded vendor entry for name, if any.
func (c *Catalog) Vendor(name string) (Vendor, bool) {
	if c == nil {
		return Vendor{}, false
	}
	vendor, ok := c.vendors[name]
	return vendor, ok
}

// Vendors returns every embedded vendor in stable name order.
func (c *Catalog) Vendors() []Vendor {
	if c == nil {
		return nil
	}
	names := make([]string, 0, len(c.vendors))
	for name := range c.vendors {
		names = append(names, name)
	}
	sort.Strings(names)
	vendors := make([]Vendor, 0, len(names))
	for _, name := range names {
		vendors = append(vendors, c.vendors[name])
	}
	return vendors
}

// VendorForEndpoint returns the vendor that declares one endpoint identity
// (adapter, base_url) (DESIGN.md §4.1). Data validation makes that pair
// globally unique, so a stored selection that names an address identifies at
// most one vendor — which is how a selection naming a third-party endpoint
// resolves that vendor's credential without any per-vendor configuration.
//
// An address no vendor declares is a user gateway or proxy, and the caller
// keeps whatever vendor it already had.
func (c *Catalog) VendorForEndpoint(adapter, baseURL string) (Vendor, Endpoint, bool) {
	if c == nil || baseURL == "" {
		return Vendor{}, Endpoint{}, false
	}
	for _, entry := range c.vendors {
		for _, endpoint := range entry.Endpoints {
			if endpoint.BaseURL != baseURL {
				continue
			}
			if adapter != "" && endpoint.Adapter != adapter {
				continue
			}
			return entry, endpoint, true
		}
	}
	return Vendor{}, Endpoint{}, false
}

// EndpointForVendor resolves the endpoint a stored selection addresses.
// (adapter, baseURL) is the endpoint identity; the vendor is the credential
// owner, which the caller resolves first (startup configuration, a legacy
// document's vendor, or VendorForEndpoint for an explicit address).
//
//   - a declared address wins, together with the adapter the data declares
//     for it;
//   - an empty adapter means "the vendor's default protocol";
//   - an address the vendor does not declare (a user gateway or proxy) keeps
//     the vendor's endpoint for the requested adapter, so a proxy of DeepSeek
//     still speaks DeepSeek's protocol and capabilities.
func (c *Catalog) EndpointForVendor(vendor, adapter, baseURL string) (Endpoint, Vendor, error) {
	entry, ok := c.Vendor(vendor)
	if !ok {
		return Endpoint{}, Vendor{}, fmt.Errorf("provider %q: no embedded vendor data", vendor)
	}
	if baseURL != "" {
		for _, endpoint := range entry.Endpoints {
			if endpoint.BaseURL == baseURL && (adapter == "" || endpoint.Adapter == adapter) {
				return endpoint, entry, nil
			}
		}
	}
	if adapter == "" {
		endpoint, ok := entry.DefaultEndpoint()
		if !ok {
			return Endpoint{}, Vendor{}, fmt.Errorf("provider %q: no endpoint declared", vendor)
		}
		return endpoint, entry, nil
	}
	endpoint, ok := entry.EndpointForAdapter(adapter)
	if !ok {
		return Endpoint{}, Vendor{}, fmt.Errorf("provider %q: no %s endpoint declared", vendor, adapter)
	}
	return endpoint, entry, nil
}

// Adapter resolves one sealed protocol family to the build-owned adapter that
// implements it, independent of any vendor: every address, credential and model
// id then has to come from the ModelSpec. It is the sealed-set lookup, and it
// is what makes a deferred family fail closed instead of falling back to a
// substitute.
//
// The runtime constructs models through RefForEndpoint rather than through this
// Ref, because a vendor endpoint is what supplies the default address and the
// default model when the stored selection leaves them empty, and what names the
// environment key in KeyMissingError.
func (c *Catalog) Adapter(family string) (Ref, error) {
	if err := checkAdapter(family); err != nil {
		return nil, err
	}
	return newAdapterRef(family, Vendor{}, Endpoint{})
}

// RefForEndpoint resolves one vendor endpoint (adapter + base URL) to its Ref.
// An adapter that is sealed but deferred, or a family this build does not know,
// fails closed instead of falling back to a substitute; so does a vendor the
// embedded data does not describe, because only the vendor carries the
// environment key and the identity an error message needs.
func (c *Catalog) RefForEndpoint(vendor string, endpoint Endpoint) (Ref, error) {
	entry, ok := c.Vendor(vendor)
	if !ok {
		return nil, fmt.Errorf("provider %q: no embedded vendor data", vendor)
	}
	if err := checkAdapter(endpoint.Adapter); err != nil {
		return nil, fmt.Errorf("provider %q: %w", vendor, err)
	}
	ref, err := newAdapterRef(endpoint.Adapter, entry, endpoint)
	if err != nil {
		return nil, fmt.Errorf("provider %q: %w", vendor, err)
	}
	return ref, nil
}

// For resolves a vendor name to a Ref over that vendor's default endpoint.
func (c *Catalog) For(name string) (Ref, error) {
	vendor, ok := c.Vendor(name)
	if !ok {
		return nil, fmt.Errorf("provider %q: no embedded vendor data", name)
	}
	endpoint, ok := vendor.DefaultEndpoint()
	if !ok {
		return nil, fmt.Errorf("provider %q: no endpoint declared", name)
	}
	return c.RefForEndpoint(vendor.Name, endpoint)
}

// checkAdapter is the one place that decides whether a family is executable:
// unknown families and deferred families both fail closed.
func checkAdapter(family string) error {
	state, sealed := AdapterState(family)
	if !sealed {
		return fmt.Errorf("%w: %q", ErrAdapterUnknown, family)
	}
	if state != modelhost.CapabilitySupported {
		return fmt.Errorf("%w: %q is %s", ErrAdapterDeferred, family, state)
	}
	return nil
}

// newAdapterRef is the Eino binding point of the sealed table: the only two
// constructors that import a pinned component.
func newAdapterRef(family string, vendor Vendor, endpoint Endpoint) (Ref, error) {
	switch family {
	case AdapterOpenAICompletions:
		return newOpenAIRef(vendor, endpoint), nil
	case AdapterAnthropicMessages:
		return newClaudeRef(vendor, endpoint), nil
	default:
		return nil, fmt.Errorf("%w: %q", ErrAdapterUnknown, family)
	}
}

// ResolveModelInfo looks up capacity metadata for a specific vendor and model
// combination from the embedded data. A vendor may speak several protocols and
// offer different models on each, so the model id selects the endpoint that
// declares it; an id no endpoint declares falls back to the vendor's default
// endpoint, where zero values mean unknown and callers must use conservative
// defaults.
func (c *Catalog) ResolveModelInfo(ctx context.Context, providerName, modelID string) (domain.ModelInfo, error) {
	vendor, ok := c.Vendor(providerName)
	if !ok {
		return domain.ModelInfo{}, fmt.Errorf("provider %q: no embedded vendor data", providerName)
	}
	if modelID != "" {
		for _, endpoint := range vendor.Endpoints {
			if _, declared := endpoint.Model(modelID); declared {
				return modelInfoFor(vendor, endpoint, modelID), nil
			}
		}
	}
	endpoint, ok := vendor.DefaultEndpoint()
	if !ok {
		return domain.ModelInfo{}, fmt.Errorf("provider %q: no endpoint declared", providerName)
	}
	return modelInfoFor(vendor, endpoint, modelID), nil
}

// modelInfoFor projects one endpoint's declared metadata for modelID. It is a
// pure data projection, so it reports the same values for a protocol this
// build cannot execute and never constructs a model.
func modelInfoFor(vendor Vendor, endpoint Endpoint, modelID string) domain.ModelInfo {
	if modelID == "" {
		modelID = endpoint.DefaultModel
	}
	meta, _ := endpoint.Model(modelID)
	return domain.ModelInfo{
		ID:               modelID,
		Provider:         vendor.Name,
		ContextWindow:    meta.ContextWindow, // zero means unknown; callers use defaults
		MaxOutputTokens:  0,                  // varies by model; let the API decide
		InputPerMTokens:  meta.InputPerMTok,
		OutputPerMTokens: meta.OutputPerMTok,
		SupportsImages:   meta.SupportsImages,
		SupportsThinking: meta.SupportsThinking,
	}
}
