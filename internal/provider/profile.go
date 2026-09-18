package provider

import (
	"encoding/json"

	"agent-vivy/sdk/port/providerprofile"
)

// The adapter-family vocabulary the compiled Profiles and the ModelHost
// capability map currently use. PROV-P2 replaces these values with the sealed
// adapter ids and deletes this bridge.
const (
	AdapterFamilyOpenAICompatible = "openai-compatible"
	AdapterFamilyAnthropic        = "anthropic"
)

var providerOptionsSchema = json.RawMessage(`{"type":"object","additionalProperties":false}`)

// ProfileFromEndpoint projects one embedded vendor endpoint into the
// declarative public Port contract. It copies metadata only; executable
// construction remains in the quarantined provider adapter files.
//
// The Profile id is the vendor name in PROV-P1, matching how the ModelHost and
// the settings overlay address a provider today; PROV-P2 re-seals the Profile
// set on adapter identities instead.
func ProfileFromEndpoint(vendor Vendor, endpoint Endpoint) providerprofile.Profile {
	return providerprofile.Profile{
		ID:            vendor.Name,
		AdapterFamily: legacyAdapterFamily(endpoint.Adapter),
		ModelIDs:      endpoint.ModelIDs(),
		EndpointClass: providerprofile.EndpointNative,
		SecretRefs:    []string{vendor.EnvKey},
		OptionsSchema: append(json.RawMessage(nil), providerOptionsSchema...),
	}
}
