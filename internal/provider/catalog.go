package provider

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"agent-vivy/internal/domain"
	"agent-vivy/sdk/port/providerprofile"
)

// ErrAdapterFamilyMismatch reports that a compiled Profile's declared adapter
// family disagrees with the adapter of the endpoint the selection resolved
// to. PROV-P2 removes the mismatch by making the Profile's family the
// endpoint's adapter, at which point this error is no longer representable.
var ErrAdapterFamilyMismatch = errors.New("provider Profile adapter family does not match the embedded endpoint's adapter")

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

// EndpointForVendor resolves the endpoint a selection addresses. An empty
// baseURL, or one that is not declared by the vendor (a user-supplied gateway
// URL), resolves to the vendor's default endpoint; otherwise the declared
// endpoint at that address is returned. This keeps one vendor's protocol and
// capabilities in force when the user points at a proxy of that vendor.
func (c *Catalog) EndpointForVendor(vendor, baseURL string) (Endpoint, Vendor, error) {
	entry, ok := c.Vendor(vendor)
	if !ok {
		return Endpoint{}, Vendor{}, fmt.Errorf("provider %q: no embedded vendor data", vendor)
	}
	if baseURL != "" {
		for _, endpoint := range entry.Endpoints {
			if endpoint.BaseURL == baseURL {
				return endpoint, entry, nil
			}
		}
	}
	endpoint, ok := entry.DefaultEndpoint()
	if !ok {
		return Endpoint{}, Vendor{}, fmt.Errorf("provider %q: no endpoint declared", vendor)
	}
	return endpoint, entry, nil
}

// RefForEndpoint builds the Ref for one vendor endpoint. An adapter that is
// sealed but deferred, or a family this build does not know, fails closed
// instead of falling back to a substitute.
func (c *Catalog) RefForEndpoint(vendor Vendor, endpoint Endpoint) (Ref, error) {
	switch endpoint.Adapter {
	case AdapterOpenAICompletions:
		return newOpenAIRef(vendor, endpoint), nil
	case AdapterAnthropicMessages:
		return newClaudeRef(vendor, endpoint), nil
	case AdapterOpenAIResponses:
		return nil, fmt.Errorf("provider %q: adapter %q is DEFERRED-INDEFINITE and has no implementation in this build", vendor.Name, endpoint.Adapter)
	default:
		return nil, fmt.Errorf("provider %q: unsupported adapter %q", vendor.Name, endpoint.Adapter)
	}
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
	return c.RefForEndpoint(vendor, endpoint)
}

// ForProfile resolves one ModelHost-approved declarative Profile to its
// build-owned executable adapter. The embedded data supplies the endpoint; the
// Profile selects the adapter family. A mismatch fails closed.
func (c *Catalog) ForProfile(profile providerprofile.Profile) (Ref, error) {
	vendor, ok := c.Vendor(profile.ID)
	if !ok {
		return nil, fmt.Errorf("provider %q: no embedded vendor data", profile.ID)
	}
	endpoint, ok := vendor.DefaultEndpoint()
	if !ok {
		return nil, fmt.Errorf("provider %q: no endpoint declared", profile.ID)
	}
	wantFamily := legacyAdapterFamily(endpoint.Adapter)
	if wantFamily == "" {
		return nil, fmt.Errorf("provider %q: adapter %q is not executable in this build", profile.ID, endpoint.Adapter)
	}
	if profile.AdapterFamily != wantFamily {
		return nil, fmt.Errorf("%w: Profile %q declares %q, endpoint requires %q", ErrAdapterFamilyMismatch, profile.ID, profile.AdapterFamily, wantFamily)
	}
	return c.RefForEndpoint(vendor, endpoint)
}

// ResolveModelInfo looks up capacity metadata for a specific provider and
// model combination from the embedded data. Zero values mean unknown and
// callers must use conservative defaults.
func (c *Catalog) ResolveModelInfo(ctx context.Context, providerName, modelID string) (domain.ModelInfo, error) {
	ref, err := c.For(providerName)
	if err != nil {
		return domain.ModelInfo{}, err
	}
	return ref.ModelInfo(ctx, modelID)
}
