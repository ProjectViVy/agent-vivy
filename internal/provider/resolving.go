package provider

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"sync"

	einoclaude "github.com/cloudwego/eino-ext/components/model/claude"
	einoopenai "github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/modelhost"
)

// ErrModelNotConfigured is returned when no provider has been selected. It
// is intentionally typed so the runtime can classify the failure without
// exposing resolver internals in the user-visible run.failed payload.
var ErrModelNotConfigured = errors.New("model provider is not configured")

// LiveSpec is the current provider selection without Eino types. App
// implements this so the resolving ChatModel can stay inside this package
// (D-007).
type LiveSpec struct {
	Provider string
	Model    string
	BaseURL  string
	APIKey   string
	Ready    bool
}

// SpecSource supplies the live ModelSpec. Implementations must be
// concurrency-safe.
type SpecSource interface {
	Live() LiveSpec
}

// NewResolvingChatModel returns a ChatModel that constructs the underlying
// provider model on each Generate/Stream from src. catalog must not be nil.
func NewResolvingChatModel(host *modelhost.Host, catalog *Catalog, src SpecSource) model.ToolCallingChatModel {
	return &resolvingChatModel{host: host, catalog: catalog, src: src}
}

type resolvingChatModel struct {
	host     *modelhost.Host
	catalog  *Catalog
	src      SpecSource
	mu       sync.Mutex
	cacheKey string
	cached   model.ToolCallingChatModel
}

func (m *resolvingChatModel) Generate(ctx context.Context, in []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	inner, err := m.inner(ctx)
	if err != nil {
		return nil, err
	}
	return inner.Generate(ctx, in, append(opts, m.thinkingOptions(ctx)...)...)
}

func (m *resolvingChatModel) Stream(ctx context.Context, in []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	inner, err := m.inner(ctx)
	if err != nil {
		return nil, err
	}
	return inner.Stream(ctx, in, append(opts, m.thinkingOptions(ctx)...)...)
}

func (m *resolvingChatModel) WithTools(tools []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	return &resolvingChatModelWithTools{parent: m, tools: tools}, nil
}

func (m *resolvingChatModel) inner(ctx context.Context) (model.ToolCallingChatModel, error) {
	if m.host == nil {
		return nil, fmt.Errorf("provider: %w", modelhost.ErrHostRequired)
	}
	live := m.src.Live()
	if live.Provider == "" {
		return nil, fmt.Errorf("%w: configure a provider in Settings → Model", ErrModelNotConfigured)
	}
	endpoint, vendor, err := m.catalog.EndpointForVendor(live.Provider, live.BaseURL)
	if err != nil {
		return nil, err
	}
	// The compiled Generation seals adapters, not vendors, so both the
	// capability gate and the availability mark are keyed by the adapter the
	// selection's endpoint speaks. A deferred family fails here.
	adapter := endpoint.Adapter
	if _, err := m.host.ResolveExecutable(adapter); err != nil {
		return nil, err
	}
	if !live.Ready {
		return nil, &KeyMissingError{Provider: live.Provider, EnvKey: vendor.EnvKey}
	}
	secretHash := sha256.Sum256([]byte(live.APIKey))
	key := fmt.Sprintf("%s\x00%s\x00%s\x00%s\x00%x", live.Provider, adapter, live.Model, live.BaseURL, secretHash)
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.cached != nil && m.cacheKey == key {
		return m.cached, nil
	}
	ref, err := m.catalog.RefForEndpoint(vendor.Name, endpoint)
	if err != nil {
		m.host.MarkUnavailable(adapter)
		return nil, err
	}
	cm, err := ref.Model(ctx, ModelSpec{ID: live.Model, APIKey: live.APIKey, BaseURL: live.BaseURL})
	if err != nil {
		m.host.MarkUnavailable(adapter)
		return nil, err
	}
	m.host.MarkAvailable(adapter)
	m.cached = cm
	m.cacheKey = key
	return cm, nil
}

type resolvingChatModelWithTools struct {
	parent *resolvingChatModel
	tools  []*schema.ToolInfo
}

// thinkingShape is the provider-neutral outcome of the thinking rule: what the
// outbound request must carry, decided without any vendor identity. Keeping the
// decision as data (rather than as options) is what makes it unit-testable, and
// keeping it out of the request path means no call site can special-case a
// vendor.
type thinkingShape struct {
	// claudeThinking sends the Anthropic extended-thinking budget.
	claudeThinking bool
	// deepSeekFlag is "enabled" or "disabled" for the DeepSeek thinking
	// object, or "" to omit it.
	deepSeekFlag string
	// reasoningEffort is an OpenAI reasoning_effort level, or "" to omit it.
	reasoningEffort string
}

// decideThinking is the whole rule:
//
//   - anthropic-messages with a thinking-capable model: "on" sends the
//     extended-thinking budget, every other preference sends nothing and leaves
//     the provider default in force.
//   - openai-completions on an endpoint declaring the deepseek-thinking
//     capability: "auto" and "on" send the documented canonical request
//     (thinking enabled plus reasoning_effort high); "off" disables thinking.
//   - openai-completions anywhere else: a thinking-capable model is a reasoning
//     model, so "auto" and "on" send reasoning_effort high; "off" sends nothing.
//
// A model whose metadata does not declare thinking support is left alone: the
// conservative default is off, and an unknown model must never be sent a field
// the upstream may reject.
func decideThinking(adapter string, deepSeekThinking, supportsThinking bool, mode domain.ThinkingMode) thinkingShape {
	if !supportsThinking {
		return thinkingShape{}
	}
	switch adapter {
	case AdapterAnthropicMessages:
		if mode != domain.ThinkingModeOn {
			return thinkingShape{}
		}
		return thinkingShape{claudeThinking: true}
	case AdapterOpenAICompletions:
		if !deepSeekThinking {
			if mode == domain.ThinkingModeOff {
				return thinkingShape{}
			}
			return thinkingShape{reasoningEffort: string(einoopenai.ReasoningEffortLevelHigh)}
		}
		if mode == domain.ThinkingModeOff {
			return thinkingShape{deepSeekFlag: "disabled"}
		}
		return thinkingShape{deepSeekFlag: "enabled", reasoningEffort: string(einoopenai.ReasoningEffortLevelHigh)}
	}
	return thinkingShape{}
}

// options renders the shape as Eino per-call options.
func (s thinkingShape) options() []model.Option {
	var options []model.Option
	if s.claudeThinking {
		options = append(options, einoclaude.WithThinking(&einoclaude.Thinking{Enable: true, BudgetTokens: claudeThinkingBudgetTokens}))
	}
	if s.deepSeekFlag != "" {
		options = append(options, einoopenai.WithExtraFields(map[string]any{"thinking": map[string]any{"type": s.deepSeekFlag}}))
	}
	if s.reasoningEffort != "" {
		options = append(options, einoopenai.WithReasoningEffort(einoopenai.ReasoningEffortLevel(s.reasoningEffort)))
	}
	return options
}

// thinkingOptions translates the run's thinking preference (domain context)
// into provider-native per-call options through the adapter, the endpoint's
// declared capabilities and the model's declared metadata.
func (m *resolvingChatModel) thinkingOptions(ctx context.Context) []model.Option {
	live := m.src.Live()
	endpoint, _, err := m.catalog.EndpointForVendor(live.Provider, live.BaseURL)
	if err != nil {
		return nil
	}
	info, err := m.catalog.ResolveModelInfo(ctx, live.Provider, live.Model)
	if err != nil {
		return nil
	}
	return decideThinking(
		endpoint.Adapter,
		endpoint.HasCapability(CapabilityDeepSeekThinking),
		info.SupportsThinking,
		domain.ThinkingModeFromContext(ctx),
	).options()
}

func (m *resolvingChatModelWithTools) Generate(ctx context.Context, in []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	inner, err := m.bound(ctx)
	if err != nil {
		return nil, err
	}
	return inner.Generate(ctx, in, append(opts, m.parent.thinkingOptions(ctx)...)...)
}

func (m *resolvingChatModelWithTools) Stream(ctx context.Context, in []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	inner, err := m.bound(ctx)
	if err != nil {
		return nil, err
	}
	return inner.Stream(ctx, in, append(opts, m.parent.thinkingOptions(ctx)...)...)
}

func (m *resolvingChatModelWithTools) WithTools(tools []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	return &resolvingChatModelWithTools{parent: m.parent, tools: tools}, nil
}

func (m *resolvingChatModelWithTools) bound(ctx context.Context) (model.ToolCallingChatModel, error) {
	inner, err := m.parent.inner(ctx)
	if err != nil {
		return nil, err
	}
	return inner.WithTools(m.tools)
}
