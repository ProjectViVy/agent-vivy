package provider

import (
	"errors"
)

// The sealed adapter families. This is the only adapter list in the
// codebase: provider data may name one of these two and nothing else, and
// adding a third wire protocol is a code change, not a data edit
// (docs/plans/provider-registry/DESIGN.md §2.1).
//
// Both families are executable in this build: they are the agentic
// generation's two channels, bound in openai.go (agenticopenai.NewChatModel)
// and claude.go (agenticclaude.New). A family with no constructor has no
// entry here, so an endpoint naming one fails closed as ErrAdapterUnknown.
const (
	// AdapterOpenAICompletions speaks POST {base}/chat/completions via
	// eino-ext/components/model/agenticopenai (agentic generation).
	AdapterOpenAICompletions = "openai-completions"
	// AdapterAnthropicMessages speaks POST {base}/messages via
	// eino-ext/components/model/agenticclaude (agentic generation).
	AdapterAnthropicMessages = "anthropic-messages"
)

// ErrAdapterUnknown reports a family that is not sealed in this build.
var ErrAdapterUnknown = errors.New("provider adapter is not sealed in this build")

// adapterTable is the single adapter list in the codebase, in stable order.
// The order is the declaration order of DESIGN.md §2.1 and is the order every
// projection (compiled Profiles, RPC status) derives from.
var adapterTable = []string{
	AdapterOpenAICompletions,
	AdapterAnthropicMessages,
}

// AdapterFamilies returns the sealed adapter families in stable order.
func AdapterFamilies() []string {
	return append([]string(nil), adapterTable...)
}

// IsSealedAdapter reports whether family is one of the sealed adapters.
func IsSealedAdapter(family string) bool {
	for _, adapter := range adapterTable {
		if adapter == family {
			return true
		}
	}
	return false
}

// endpointCapabilities is the closed capability vocabulary (DESIGN.md §3.3):
// a named flag is legal only on an adapter that implements it. This is an
// enum, never free-form request injection.
var endpointCapabilities = map[string][]string{
	AdapterOpenAICompletions: {CapabilityDeepSeekThinking},
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
