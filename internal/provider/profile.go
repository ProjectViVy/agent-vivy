package provider

import (
	"encoding/json"

	"agent-vivy/sdk/port/providerprofile"
)

const (
	AdapterFamilyOpenAICompatible = "openai-compatible"
	AdapterFamilyAnthropic        = "anthropic"
)

var providerOptionsSchema = json.RawMessage(`{"type":"object","additionalProperties":false}`)

// ProfileFromBundle projects an existing build-owned provider bundle into
// the declarative public Port contract. It copies metadata only; executable
// construction remains in the quarantined provider adapter files.
func ProfileFromBundle(bundle Bundle) providerprofile.Profile {
	family := ""
	switch bundle.Backend {
	case BackendEinoOpenAI:
		family = AdapterFamilyOpenAICompatible
	case BackendEinoClaude:
		family = AdapterFamilyAnthropic
	}
	return providerprofile.Profile{
		ID: bundle.Name, AdapterFamily: family,
		ModelIDs:      append([]string(nil), bundle.Models...),
		EndpointClass: providerprofile.EndpointNative,
		SecretRefs:    []string{bundle.EnvKey},
		OptionsSchema: append(json.RawMessage(nil), providerOptionsSchema...),
	}
}
