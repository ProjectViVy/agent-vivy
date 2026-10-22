package provider

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/cloudwego/eino-ext/components/model/agenticclaude"
	"github.com/cloudwego/eino-ext/components/model/agenticopenai"
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
// implements this so the resolving AgenticModel can stay inside this package
// (D-007). UserDefined marks a settings registry endpoint whose protocol is
// sealed but whose address, model and credential are user-owned.
//
// Provider is the vendor whose credential and default address apply; Adapter
// is the sealed protocol the stored selection names, which is what the
// availability mark and the thinking rule key on. A selection with no stored
// adapter (an empty Adapter) is resolved from the endpoint's data instead.
type LiveSpec struct {
	Provider    string
	Adapter     string
	Model       string
	BaseURL     string
	APIKey      string
	Ready       bool
	UserDefined bool
}

// SpecSource supplies the live ModelSpec. Implementations must be
// concurrency-safe.
type SpecSource interface {
	Live() LiveSpec
}

// NewResolvingChatModel returns an agentic chat model that constructs the
// underlying provider model on each Generate/Stream from src. catalog must not
// be nil.
func NewResolvingChatModel(host *modelhost.Host, catalog *Catalog, src SpecSource) model.AgenticModel {
	return &resolvingChatModel{host: host, catalog: catalog, src: src}
}

type resolvingChatModel struct {
	host     *modelhost.Host
	catalog  *Catalog
	src      SpecSource
	mu       sync.Mutex
	cacheKey resolvingModelKey
	cached   model.AgenticModel
}

// resolvingModelKey is private construction input, never a diagnostic or
// persisted identifier. The cached provider client already owns its credential;
// exact equality here ensures rotation cannot reuse that old client.
type resolvingModelKey struct {
	provider, adapter, model, baseURL, credential string
	userDefined                                   bool
}

func (m *resolvingChatModel) Generate(ctx context.Context, in []*schema.AgenticMessage, opts ...model.Option) (*schema.AgenticMessage, error) {
	inner, err := m.inner(ctx)
	if err != nil {
		return nil, err
	}
	return inner.Generate(ctx, in, append(opts, m.thinkingOptions(ctx)...)...)
}

func (m *resolvingChatModel) Stream(ctx context.Context, in []*schema.AgenticMessage, opts ...model.Option) (*schema.StreamReader[*schema.AgenticMessage], error) {
	inner, err := m.inner(ctx)
	if err != nil {
		return nil, err
	}
	return inner.Stream(ctx, in, append(opts, m.thinkingOptions(ctx)...)...)
}

func (m *resolvingChatModel) inner(ctx context.Context) (model.AgenticModel, error) {
	if m.host == nil {
		return nil, fmt.Errorf("provider: %w", modelhost.ErrHostRequired)
	}
	live := m.src.Live()
	if live.Provider == "" {
		return nil, fmt.Errorf("%w: configure a provider in Settings → Model", ErrModelNotConfigured)
	}
	var endpoint Endpoint
	var vendor Vendor
	var ref Ref
	var err error
	if live.UserDefined {
		// User-defined entries own their model id, address and key in Settings;
		// only the wire protocol comes from the sealed adapter set.
		endpoint.Adapter = live.Adapter
		ref, err = m.catalog.Adapter(live.Adapter)
	} else {
		endpoint, vendor, err = m.catalog.EndpointForVendor(live.Provider, live.Adapter, live.BaseURL)
	}
	if err != nil {
		return nil, err
	}
	// The compiled Generation seals adapters, not vendors, so both the
	// capability gate and the availability mark are keyed by the adapter the
	// selection speaks. A family this build does not seal fails here.
	adapter := endpoint.Adapter
	if _, err := m.host.ResolveExecutable(adapter); err != nil {
		return nil, err
	}
	if !live.Ready {
		return nil, &KeyMissingError{Provider: live.Provider, EnvKey: vendor.EnvKey}
	}
	key := resolvingModelKey{
		provider: live.Provider, adapter: adapter, model: live.Model,
		baseURL: live.BaseURL, credential: live.APIKey, userDefined: live.UserDefined,
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.cached != nil && m.cacheKey == key {
		return m.cached, nil
	}
	if ref == nil {
		ref, err = m.catalog.RefForEndpoint(vendor.Name, endpoint)
		if err != nil {
			m.host.MarkUnavailable(adapter)
			return nil, err
		}
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

// reasoningEffortHigh is the legacy default effort for a reasoning model that
// asks for thinking without naming a level. The agentic openai component has no
// effort enum of its own; the value lands in the request body verbatim, so it
// must be the domain level string.
const reasoningEffortHigh = "high"

// thinkingShape is the provider-neutral outcome of the thinking rule: what the
// outbound request must carry, decided without any vendor identity. Keeping the
// decision as data (rather than as options) is what makes it unit-testable, and
// keeping it out of the request path means no call site can special-case a
// vendor.
type thinkingShape struct {
	// adapter is the sealed family the shape is rendered for.
	adapter string
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
//     thinking disabled; levels send enabled + the canonical effort.
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
	shape := thinkingShape{adapter: adapter, effective: resolved}
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
		effort := reasoningEffortHigh
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
			shape.reasoningEffort = reasoningEffortHigh
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

// options renders the shape as Eino per-call options for the adapter the shape
// was decided for.
func (s thinkingShape) options() []model.Option {
	switch s.adapter {
	case AdapterAnthropicMessages:
		return s.claudeOptions()
	case AdapterOpenAICompletions:
		return s.openAIOptions()
	}
	return nil
}

// claudeOptions renders the Anthropic shape: the thinking object travels as a
// request-body extra field, the protocol-required response budget as
// max_tokens, and sampling through the component's common options.
func (s thinkingShape) claudeOptions() []model.Option {
	var options []model.Option
	if s.claudeBudget > 0 {
		options = append(options,
			agenticclaude.WithExtraFields(map[string]any{
				"thinking": map[string]any{"type": "enabled", "budget_tokens": s.claudeBudget},
			}),
		)
		// The response budget must exceed the thinking budget by headroom;
		// the provider rejects thinking budgets >= max_tokens.
		options = append(options, model.WithMaxTokens(s.claudeBudget+claudeDefaultMaxTokens))
		if s.claudeBudget+claudeDefaultMaxTokens > claudeNonStreamingTokenCeiling {
			options = append(options, agenticclaude.WithRequestTimeout(claudeLongRequestTimeout))
		}
	}
	if s.sampling.Temperature != nil {
		options = append(options, model.WithTemperature(float32(*s.sampling.Temperature)))
	}
	if s.sampling.TopP != nil {
		options = append(options, model.WithTopP(float32(*s.sampling.TopP)))
	}
	return options
}

// openAIOptions renders the openai-completions shape. The agentic component
// reads no common options, so the DeepSeek thinking flag, reasoning_effort and
// the sampling overrides all travel as request-body extra fields in one
// option.
func (s thinkingShape) openAIOptions() []model.Option {
	extra := map[string]any{}
	if s.deepSeekFlag != "" {
		extra["thinking"] = map[string]any{"type": s.deepSeekFlag}
	}
	if s.reasoningEffort != "" {
		extra["reasoning_effort"] = s.reasoningEffort
	}
	if s.sampling.Temperature != nil {
		extra["temperature"] = *s.sampling.Temperature
	}
	if s.sampling.TopP != nil {
		extra["top_p"] = *s.sampling.TopP
	}
	if len(extra) == 0 {
		return nil
	}
	return []model.Option{agenticopenai.WithExtraFields(extra)}
}

// thinkingOptions translates the run's thinking preference (domain context)
// into provider-native per-call options through the adapter, the endpoint's
// declared capabilities and the model's declared metadata.
func (m *resolvingChatModel) thinkingOptions(ctx context.Context) []model.Option {
	live := m.src.Live()
	if live.UserDefined {
		// Settings' custom registry carries no model capability metadata, so
		// keep the adapter's request conservative rather than guessing.
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
