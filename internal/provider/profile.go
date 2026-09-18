package provider

import (
	"encoding/json"
	"sort"

	"agent-vivy/sdk/port/providerprofile"
)

var providerOptionsSchema = json.RawMessage(`{"type":"object","additionalProperties":false}`)

// AdapterProfiles projects the sealed adapters onto the declarative
// std/provider-profile@v1 contract the compiled Generation seals. There is one
// Profile per sealed adapter, and its model ids and Secret references are the
// union over every embedded endpoint that speaks that adapter: the ModelHost
// status surface then describes the protocol the build can actually reach,
// instead of one vendor's slice of it.
//
// The ids are the adapter families, which is also what the ModelHost capability
// map is keyed by (Capabilities), so a deferred family is visible as a
// DEFERRED-INDEFINITE Profile rather than missing.
func AdapterProfiles() ([]providerprofile.Profile, error) {
	vendors, err := LoadEmbedded()
	if err != nil {
		return nil, err
	}
	profiles := make([]providerprofile.Profile, 0, len(adapterTable))
	for _, adapter := range Adapters() {
		models := make([]string, 0, 32)
		seenModel := make(map[string]struct{}, 32)
		secrets := make([]string, 0, 8)
		seenSecret := make(map[string]struct{}, 8)
		for _, vendor := range vendors {
			speaks := false
			for _, endpoint := range vendor.Endpoints {
				if endpoint.Adapter != adapter.Family {
					continue
				}
				speaks = true
				for _, id := range endpoint.ModelIDs() {
					if _, duplicate := seenModel[id]; duplicate {
						continue
					}
					seenModel[id] = struct{}{}
					models = append(models, id)
				}
			}
			if !speaks {
				continue
			}
			if _, duplicate := seenSecret[vendor.EnvKey]; !duplicate {
				seenSecret[vendor.EnvKey] = struct{}{}
				secrets = append(secrets, vendor.EnvKey)
			}
		}
		sort.Strings(models)
		sort.Strings(secrets)
		profiles = append(profiles, providerprofile.Profile{
			ID:            adapter.Family,
			AdapterFamily: adapter.Family,
			ModelIDs:      models,
			EndpointClass: providerprofile.EndpointNative,
			SecretRefs:    secrets,
			OptionsSchema: append(json.RawMessage(nil), providerOptionsSchema...),
		})
	}
	return profiles, nil
}
