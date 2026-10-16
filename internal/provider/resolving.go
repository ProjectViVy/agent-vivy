package provider

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"sync"
	"time"

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
//
// Provider is the vendor whose credential and default address apply; Adapter
// is the sealed protocol the stored selection names, which is what the
// capability gate, the availability mark and the thinking rule key on. A
// selection with no stored adapter (an empty Adapter) is resolved from the
// endpoint's data instead.
//
// Endpoint is the endpoint a user-registered provider declared, in which case
// Provider is that registry's alias and only labels events and errors: the
// embedded catalog holds no entry for it, so Endpoint is the whole wire
// identity and the run path must not look the name up.
type LiveSpec struct {
	Provider string
	Adapter  string
	Model    string
	BaseURL  string
	APIKey   string
	Ready    bool
	Endpoint Endpoint
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
	live := m.src.Live()
	inner, err := m.innerForSpec(ctx, live)
	if err != nil {
		return nil, err
	}
	options, err := m.requestOptionsForSpec(ctx, opts, live)
	if err != nil {
		return nil, err
	}
	return inner.Generate(ctx, in, options...)
}

func (m *resolvingChatModel) Stream(ctx context.Context, in []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	live := m.src.Live()
	inner, err := m.innerForSpec(ctx, live)
	if err != nil {
		return nil, err
	}
	options, err := m.requestOptionsForSpec(ctx, opts, live)
	if err != nil {
		return nil, err
	}
	return inner.Stream(ctx, in, options...)
}

func (m *resolvingChatModel) WithTools(tools []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	return &resolvingChatModelWithTools{parent: m, tools: tools}, nil
}

func (m *resolvingChatModel) innerForSpec(ctx context.Context, live LiveSpec) (model.ToolCallingChatModel, error) {
	if m.host == nil {
		return nil, fmt.Errorf("provider: %w", modelhost.ErrHostRequired)
	}
	if live.Provider == "" {
		return nil, fmt.Errorf("%w: configure a provider in Settings → Model", ErrModelNotConfigured)
	}
	// A user-registered provider carries its own endpoint: the registry entry
	// declares the protocol, the address and the model list, and names no
	// embedded vendor at all. An embedded selection resolves its endpoint
	// variant — and the vendor that owns its credential error — from the data.
	endpoint, vendor, err := m.catalog.EndpointForVendor(live.Provider, live.Adapter, live.BaseURL)
	custom := live.Endpoint.Adapter != ""
	if custom {
		endpoint, vendor, err = live.Endpoint, Vendor{Name: live.Provider}, nil
	}
	if err != nil {
		return nil, err
	}
	// The compiled Generation seals adapters, not vendors, so both the
	// capability gate and the availability mark are keyed by the adapter the
	// selection speaks. A deferred family fails here.
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
	ref, refErr := m.catalog.RefForEndpoint(vendor.Name, endpoint)
	if custom {
		ref, refErr = m.catalog.RefForSelection(vendor.Name, endpoint)
	}
	if refErr != nil {
		m.host.MarkUnavailable(adapter)
		return nil, refErr
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
	// effective is the level the request resolves to, for diagnostics and
	// level reporting; "off"/"" means no explicit thinking option.
	effective domain.ThinkingMode
	// claudeBudget carries the Anthropic extended-thinking budget in tokens,
	// or 0 to send no thinking object.
	claudeBudget int
	// deepSeekFlag is "enabled" or "disabled" for the DeepSeek thinking
	// object, or "" to omit it.
	deepSeekFlag string
	// reasoningEffort is an OpenAI reasoning_effort level, or "" to omit it.
	reasoningEffort string
	// sampling carries the level's declared sampling overrides.
	sampling domain.ThinkingSampling
}

// claudeThinkingBudget maps an effort level to an Anthropic budget_tokens
// value. The budget stays modest so thinking never swallows the response
// window on its own.
func claudeThinkingBudget(level domain.ThinkingMode) int {
	switch level {
	case domain.ThinkingLevelMinimal:
		return 1024
	case domain.ThinkingLevelLow:
		return 2048
	case domain.ThinkingLevelMedium:
		return claudeThinkingBudgetTokens
	case domain.ThinkingLevelHigh:
		return 8192
	case domain.ThinkingLevelXHigh:
		return 16384
	case domain.ThinkingLevelMax:
		return 32768
	}
	return claudeThinkingBudgetTokens
}

// ResolveThinkingLevel normalizes a requested mode into the effective level:
//
//   - "auto": the model's declared default_thinking when it declares one,
//     else the provider default (reports "").
//   - "on": the model's declared default_thinking, else the highest declared
//     level, else the bare "on" marker.
//   - "off": stays "off".
//   - an explicit level: clamped to the model's declared levels.
//
// The result is what callers should treat as the effective level; the
// adapter mapping decides what it means on the wire.
func ResolveThinkingLevel(info domain.ModelInfo, mode domain.ThinkingMode) domain.ThinkingMode {
	switch {
	case mode == domain.ThinkingModeOff:
		return domain.ThinkingModeOff
	case mode.IsLevel():
		return domain.ClampThinkingLevel(mode, info.ThinkingLevels)
	case mode == domain.ThinkingModeOn || mode == domain.ThinkingModeAuto || mode == "":
		if info.DefaultThinking != "" {
			return domain.ClampThinkingLevel(domain.ThinkingMode(info.DefaultThinking), info.ThinkingLevels)
		}
		if mode == domain.ThinkingModeOn && len(info.ThinkingLevels) > 0 {
			return domain.ClampThinkingLevel(domain.ThinkingLevelMax, info.ThinkingLevels)
		}
		return mode // alias preserved
	}
	return mode
}

// decideThinking is the whole rule. It takes the resolved effective level
// (see ResolveThinkingLevel) plus the adapter and the endpoint's
// capabilities:
//
//   - anthropic-messages: any effective level sends extended thinking with
//     the level's budget; the legacy "on" marker sends the legacy budget;
//     everything else sends nothing (provider default).
//   - openai-completions on a deepseek-thinking endpoint: "off" sends
//     thinking disabled; "auto"/"" send nothing... wait, legacy behavior
//     must hold: auto/on send enabled + effort high, off sends disabled.
//     Levels send enabled + the level as reasoning_effort.
//   - openai-completions elsewhere: a reasoning model, so auto/on send
//     reasoning_effort high (legacy), a level sends it verbatim, "off"
//     sends nothing.
//
// A model whose metadata does not declare thinking support is left alone:
// the conservative default is off, and an unknown model must never be sent
// a field the upstream may reject.
func decideThinking(adapter string, deepSeekThinking bool, info domain.ModelInfo, mode domain.ThinkingMode) thinkingShape {
	if !info.SupportsThinking {
		return thinkingShape{}
	}
	resolved := ResolveThinkingLevel(info, mode)
	shape := thinkingShape{effective: resolved}
	switch adapter {
	case AdapterAnthropicMessages:
		switch {
		case resolved.IsLevel():
			shape.claudeBudget = claudeThinkingBudget(resolved)
		case resolved == domain.ThinkingModeOn:
			shape.claudeBudget = claudeThinkingBudgetTokens
			shape.effective = domain.ThinkingLevelMedium
		}
	case AdapterOpenAICompletions:
		effort := string(einoopenai.ReasoningEffortLevelHigh)
		if resolved.IsLevel() {
			effort = string(resolved)
		}
		switch {
		case resolved == domain.ThinkingModeOff && !deepSeekThinking:
			shape.effective = domain.ThinkingModeOff
		case resolved == domain.ThinkingModeOff:
			shape.deepSeekFlag = "disabled"
		case !deepSeekThinking:
			shape.reasoningEffort = effort
		default:
			// DeepSeek's thinking surface is binary; a requested level
			// still maps to the canonical effort the endpoint documents.
			shape.deepSeekFlag = "enabled"
			shape.reasoningEffort = string(einoopenai.ReasoningEffortLevelHigh)
		}
	}
	if shape.effective.IsLevel() {
		if sampling, ok := info.ThinkingSampling[string(shape.effective)]; ok {
			shape.sampling = sampling
		}
	}
	return shape
}

// EffectiveThinkingLevel resolves a requested mode against the model's
// declared policy — the level the run path will actually send. Exposed for
// the control plane's thinking report.
func EffectiveThinkingLevel(info domain.ModelInfo, mode domain.ThinkingMode) domain.ThinkingMode {
	return ResolveThinkingLevel(info, mode)
}

// claudeNonStreamingTokenCeiling is the Anthropic SDK's non-streaming
// ceiling: a Generate whose max_tokens crosses 128000/6 tokens is refused
// client-side with "streaming is required". Requests above it carry an
// explicit request timeout, which is the SDK's documented bypass.
const claudeNonStreamingTokenCeiling = 128000 / 6

// claudeLongRequestTimeout bounds a non-streaming thinking request whose
// max_tokens crosses the SDK ceiling. The estimate behind the ceiling puts
// 40960 tokens at ~19 minutes; half an hour leaves headroom.
const claudeLongRequestTimeout = 30 * time.Minute

// options renders the shape as Eino per-call options.
func (s thinkingShape) options() []model.Option {
	var options []model.Option
	if s.claudeBudget > 0 {
		options = append(options, einoclaude.WithThinking(&einoclaude.Thinking{Enable: true, BudgetTokens: s.claudeBudget}))
		// The response budget must exceed the thinking budget by headroom;
		// the provider rejects thinking budgets >= max_tokens.
		options = append(options, model.WithMaxTokens(s.claudeBudget+claudeDefaultMaxTokens))
		if s.claudeBudget+claudeDefaultMaxTokens > claudeNonStreamingTokenCeiling {
			options = append(options, einoclaude.WithRequestTimeout(claudeLongRequestTimeout))
		}
	}
	if s.deepSeekFlag != "" {
		options = append(options, einoopenai.WithExtraFields(map[string]any{"thinking": map[string]any{"type": s.deepSeekFlag}}))
	}
	if s.reasoningEffort != "" {
		options = append(options, einoopenai.WithReasoningEffort(einoopenai.ReasoningEffortLevel(s.reasoningEffort)))
	}
	if s.sampling.Temperature != nil {
		options = append(options, model.WithTemperature(float32(*s.sampling.Temperature)))
	}
	if s.sampling.TopP != nil {
		options = append(options, model.WithTopP(float32(*s.sampling.TopP)))
	}
	return options
}

// thinkingOptions translates the run's thinking preference (domain context)
// into provider-native per-call options through the adapter, the endpoint's
// declared capabilities and the model's declared metadata.
func (m *resolvingChatModel) thinkingOptionsForSpec(ctx context.Context, live LiveSpec) []model.Option {
	// A user-registered endpoint declares no capability and no model metadata,
	// and the embedded data does not describe it, so there is nothing to key
	// the rule on. Sending a reasoning field to an upstream that never
	// advertised one is the failure this leaves out.
	if live.Endpoint.Adapter != "" {
		return nil
	}
	endpoint, _, err := m.catalog.EndpointForVendor(live.Provider, live.Adapter, live.BaseURL)
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
		info,
		domain.ThinkingModeFromContext(ctx),
	).options()
}

func (m *resolvingChatModelWithTools) Generate(ctx context.Context, in []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	live := m.parent.src.Live()
	inner, err := m.boundForSpec(ctx, live)
	if err != nil {
		return nil, err
	}
	options, err := m.parent.requestOptionsForSpec(ctx, opts, live)
	if err != nil {
		return nil, err
	}
	return inner.Generate(ctx, in, options...)
}

func (m *resolvingChatModelWithTools) Stream(ctx context.Context, in []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	live := m.parent.src.Live()
	inner, err := m.boundForSpec(ctx, live)
	if err != nil {
		return nil, err
	}
	options, err := m.parent.requestOptionsForSpec(ctx, opts, live)
	if err != nil {
		return nil, err
	}
	return inner.Stream(ctx, in, options...)
}

func (m *resolvingChatModelWithTools) WithTools(tools []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	return &resolvingChatModelWithTools{parent: m.parent, tools: tools}, nil
}

func (m *resolvingChatModelWithTools) boundForSpec(ctx context.Context, live LiveSpec) (model.ToolCallingChatModel, error) {
	inner, err := m.parent.innerForSpec(ctx, live)
	if err != nil {
		return nil, err
	}
	return inner.WithTools(m.tools)
}

// requestOptions preserves the selected thinking shape and applies a trusted
// runtime output cap last. An incompatible thinking budget is unavailable,
// never silently disabled or shrunk to make a capacity check pass.
func (m *resolvingChatModel) requestOptionsForSpec(ctx context.Context, opts []model.Option, live LiveSpec) ([]model.Option, error) {
	if pin, ok := domain.ModelRequestScopeFromContext(ctx); ok && (pin.Provider == "" || pin.Model == "" || pin.Provider != live.Provider || pin.Model != live.Model) {
		return nil, fmt.Errorf("provider: admitted model route changed before invocation")
	}
	options := append(append([]model.Option(nil), opts...), m.thinkingOptionsForSpec(ctx, live)...)
	limit := domain.ModelOutputLimitFromContext(ctx)
	if limit <= 0 {
		return options, nil
	}
	endpoint := live.Endpoint
	if endpoint.Adapter == "" {
		var err error
		endpoint, _, err = m.catalog.EndpointForVendor(live.Provider, live.Adapter, live.BaseURL)
		if err != nil {
			return nil, err
		}
	}
	if endpoint.Adapter != AdapterOpenAICompletions && endpoint.Adapter != AdapterAnthropicMessages {
		return nil, ErrAdapterDeferred
	}
	if endpoint.Adapter == AdapterAnthropicMessages && live.Endpoint.Adapter == "" {
		info, err := m.catalog.ResolveModelInfo(ctx, live.Provider, live.Model)
		if err != nil {
			return nil, err
		}
		shape := decideThinking(endpoint.Adapter, false, info, domain.ThinkingModeFromContext(ctx))
		if shape.claudeBudget >= limit {
			return nil, fmt.Errorf("provider: requested output limit %d is incompatible with selected thinking budget %d", limit, shape.claudeBudget)
		}
	}
	return append(options, model.WithMaxTokens(limit)), nil
}
