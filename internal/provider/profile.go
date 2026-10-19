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
// The ids are the adapter families, which is also the vocabulary the executable
// adapter set is keyed by: the agentic generation compiles one Profile per
// sealed family that the embedded data reaches, because a Profile with no model
// ids would claim reachability the data does not have.
func AdapterProfiles() ([]providerprofile.Profile, error) {
	vendors, err := LoadEmbedded()
	if err != nil {
		return nil, err
	}
	profiles := make([]providerprofile.Profile, 0, len(adapterTable))
	for _, adapter := range AdapterFamilies() {
		models := make([]string, 0, 32)
		seenModel := make(map[string]struct{}, 32)
		secrets := make([]string, 0, 8)
		seenSecret := make(map[string]struct{}, 8)
		for _, vendor := range vendors {
			speaks := false
			for _, endpoint := range vendor.Endpoints {
				if endpoint.Adapter != adapter {
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
		if len(models) == 0 {
			// No embedded vendor speaks this family, so there is no endpoint to
			// compile. A Profile with no model ids would be rejected by the
			// ModelHost and would claim reachability the data does not have.
			continue
		}
		profiles = append(profiles, providerprofile.Profile{
			ID:            adapter,
			AdapterFamily: adapter,
			ModelIDs:      models,
			EndpointClass: providerprofile.EndpointNative,
			SecretRefs:    secrets,
			OptionsSchema: append(json.RawMessage(nil), providerOptionsSchema...),
		})
	}
	return profiles, nil
}
