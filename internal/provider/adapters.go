package provider

// The sealed adapter families. This is the only adapter list in the
// codebase: provider data may name one of these three and nothing else, and
// adding a fourth wire protocol is a code change, not a data edit
// (docs/plans/provider-registry/DESIGN.md §2.1). PROV-P2 binds each family
// to its pinned Eino component and capability state; the names and the
// capability vocabulary below are already load-bearing here, because the
// vendor data is validated against them at startup.
const (
	// AdapterOpenAICompletions speaks POST {base}/chat/completions via
	// eino-ext/components/model/openai.
	AdapterOpenAICompletions = "openai-completions"
	// AdapterOpenAIResponses speaks POST {base}/responses. It is declared
	// and DEFERRED-INDEFINITE: no implementation placeholder exists, so an
	// endpoint naming it is visible but never executable
	// (docs/plans/provider-registry/EINO-CAPABILITY.md).
	AdapterOpenAIResponses = "openai-responses"
	// AdapterAnthropicMessages speaks POST {base}/messages via
	// eino-ext/components/model/claude.
	AdapterAnthropicMessages = "anthropic-messages"
)

// adapterFamilies is the stable, ordered sealed set.
var adapterFamilies = []string{
	AdapterOpenAICompletions,
	AdapterOpenAIResponses,
	AdapterAnthropicMessages,
}

// AdapterFamilies returns the sealed adapter families in stable order.
func AdapterFamilies() []string {
	return append([]string(nil), adapterFamilies...)
}

// IsSealedAdapter reports whether family is one of the sealed adapters.
func IsSealedAdapter(family string) bool {
	for _, candidate := range adapterFamilies {
		if candidate == family {
			return true
		}
	}
	return false
}

// endpointCapabilities is the closed capability vocabulary (DESIGN.md §3.3):
// a named flag is legal only on an adapter that implements it. This is an
// enum, never free-form request injection.
var endpointCapabilities = map[string][]string{
	AdapterOpenAICompletions: {"deepseek-thinking"},
	AdapterOpenAIResponses:   {},
	AdapterAnthropicMessages: {},
}

// AdapterSupportsCapability reports whether capability is legal on adapter.
func AdapterSupportsCapability(adapter, capability string) bool {
	for _, legal := range endpointCapabilities[adapter] {
		if legal == capability {
			return true
		}
	}
	return false
}

// CapabilityDeepSeekThinking is the endpoint capability that adds the
// DeepSeek "thinking" request object on top of reasoning_effort.
const CapabilityDeepSeekThinking = "deepseek-thinking"

// legacyAdapterFamily maps a sealed adapter to the pre-PROV-P2 Profile
// adapter-family vocabulary. It exists only for the one phase in which the
// sealed unit is moving from vendor to adapter: PROV-P2 replaces the
// Profile vocabulary with the adapter ids and deletes this function.
func legacyAdapterFamily(adapter string) string {
	switch adapter {
	case AdapterOpenAICompletions:
		return AdapterFamilyOpenAICompatible
	case AdapterAnthropicMessages:
		return AdapterFamilyAnthropic
	default:
		return ""
	}
}
