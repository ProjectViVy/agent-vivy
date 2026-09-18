package provider

import (
	"errors"

	"agent-vivy/internal/modelhost"
)

// The sealed adapter families. This is the only adapter list in the
// codebase: provider data may name one of these three and nothing else, and
// adding a fourth wire protocol is a code change, not a data edit
// (docs/plans/provider-registry/DESIGN.md §2.1).
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

var (
	// ErrAdapterUnknown reports a family that is not sealed in this build.
	ErrAdapterUnknown = errors.New("provider adapter is not sealed in this build")
	// ErrAdapterDeferred reports a sealed family this build cannot execute.
	ErrAdapterDeferred = errors.New("provider adapter is DEFERRED-INDEFINITE")
)

// Adapter binds one sealed protocol family to the capability state this build
// reports for it.
//
// The Eino binding is deliberately not a field. Catalog.Adapter dispatches on
// Family to the pinned components, and those imports live in exactly two files:
// openai.go (eino-ext/components/model/openai v0.1.13) and claude.go
// (eino-ext/components/model/claude v0.1.25). A deferred family has no
// constructor at all, which is why it cannot be reached by data.
type Adapter struct {
	Family string
	State  modelhost.CapabilityState
}

// adapterTable is the single adapter list in the codebase, in stable order.
// The order is the declaration order of DESIGN.md §2.1 and is the order every
// projection (ModelHost capabilities, compiled Profiles, RPC status) derives
// from.
var adapterTable = []Adapter{
	{Family: AdapterOpenAICompletions, State: modelhost.CapabilitySupported},
	{Family: AdapterOpenAIResponses, State: modelhost.CapabilityDeferredIndefinite},
	{Family: AdapterAnthropicMessages, State: modelhost.CapabilitySupported},
}

// Adapters returns the sealed adapters in stable order. It is the single
// adapter list in the codebase.
func Adapters() []Adapter {
	return append([]Adapter(nil), adapterTable...)
}

// Capabilities projects the sealed adapter table onto the ModelHost capability
// map, so the compiled capability set and the adapter table cannot drift.
func Capabilities() modelhost.Capabilities {
	capabilities := make(modelhost.Capabilities, len(adapterTable))
	for _, adapter := range adapterTable {
		capabilities[adapter.Family] = adapter.State
	}
	return capabilities
}

// AdapterFamilies returns the sealed adapter families in stable order.
func AdapterFamilies() []string {
	families := make([]string, 0, len(adapterTable))
	for _, adapter := range adapterTable {
		families = append(families, adapter.Family)
	}
	return families
}

// AdapterState reports the sealed state of family.
func AdapterState(family string) (modelhost.CapabilityState, bool) {
	for _, adapter := range adapterTable {
		if adapter.Family == family {
			return adapter.State, true
		}
	}
	return "", false
}

// IsSealedAdapter reports whether family is one of the sealed adapters.
func IsSealedAdapter(family string) bool {
	_, sealed := AdapterState(family)
	return sealed
}

// endpointCapabilities is the closed capability vocabulary (DESIGN.md §3.3):
// a named flag is legal only on an adapter that implements it. This is an
// enum, never free-form request injection.
var endpointCapabilities = map[string][]string{
	AdapterOpenAICompletions: {CapabilityDeepSeekThinking},
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
