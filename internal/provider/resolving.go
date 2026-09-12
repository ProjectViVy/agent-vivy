package provider

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"sync"

	einoclaude "github.com/cloudwego/eino-ext/components/model/claude"
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
	profile, err := m.host.ResolveExecutable(live.Provider)
	if err != nil {
		return nil, err
	}
	if !live.Ready {
		return nil, &KeyMissingError{Provider: live.Provider}
	}
	secretHash := sha256.Sum256([]byte(live.APIKey))
	key := fmt.Sprintf("%s\x00%s\x00%s\x00%x", live.Provider, live.Model, live.BaseURL, secretHash)
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.cached != nil && m.cacheKey == key {
		return m.cached, nil
	}
	ref, err := m.catalog.ForProfile(profile)
	if err != nil {
		m.host.MarkUnavailable(live.Provider)
		return nil, err
	}
	cm, err := ref.Model(ctx, ModelSpec{ID: live.Model, APIKey: live.APIKey, BaseURL: live.BaseURL})
	if err != nil {
		m.host.MarkUnavailable(live.Provider)
		return nil, err
	}
	m.host.MarkAvailable(live.Provider)
	m.cached = cm
	m.cacheKey = key
	return cm, nil
}

type resolvingChatModelWithTools struct {
	parent *resolvingChatModel
	tools  []*schema.ToolInfo
}

// thinkingOptions translates the run's thinking preference (domain context)
// into a provider-native per-call option. Only "on" injects anything: for
// models without an explicit request the provider default applies. The
// Anthropic path is the one wired knob this generation; the option is
// gated on D9 model metadata so models that reject the thinking parameter
// never receive it (unknown models keep the conservative off).
func (m *resolvingChatModel) thinkingOptions(ctx context.Context) []model.Option {
	if domain.ThinkingModeFromContext(ctx) != domain.ThinkingModeOn {
		return nil
	}
	live := m.src.Live()
	bundle, ok := m.catalog.Bundle(live.Provider)
	if !ok || bundle.Backend != BackendEinoClaude {
		return nil
	}
	info, err := m.catalog.ResolveModelInfo(ctx, live.Provider, live.Model)
	if err != nil || !info.SupportsThinking {
		return nil
	}
	return []model.Option{einoclaude.WithThinking(&einoclaude.Thinking{Enable: true, BudgetTokens: claudeThinkingBudgetTokens})}
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
