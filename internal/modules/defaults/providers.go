package defaults

import (
	"context"
	"encoding/json"

	"agent-vivy/internal/tools"
	"agent-vivy/sdk/port/providerprofile"
	"agent-vivy/sdk/port/tool"
	"agent-vivy/sdk/port/toolworld"
)

var defaultProviderOptionsSchema = json.RawMessage(`{"type":"object","additionalProperties":false,"properties":{"temperature":{"type":"number"},"top_p":{"type":"number"},"max_tokens":{"type":"integer"},"thinking":{"type":"boolean"}}}`)

func ProviderProfiles() []providerprofile.Provider {
	return []providerprofile.Provider{
		providerProfile{profile: providerprofile.Profile{
			ID: "openai", AdapterFamily: "openai-compatible",
			ModelIDs:      []string{"gpt-4o", "gpt-4o-mini", "gpt-4-turbo", "gpt-4", "o1-preview", "o1-mini", "gpt-5.1", "gpt-5", "gpt-5-mini", "gpt-5-nano", "gpt-5-pro", "gpt-5-chat"},
			EndpointClass: providerprofile.EndpointNative,
			SecretRefs:    []string{"OPENAI_API_KEY"}, OptionsSchema: defaultProviderOptionsSchema,
		}},
		providerProfile{profile: providerprofile.Profile{
			ID: "anthropic", AdapterFamily: "anthropic",
			ModelIDs:      []string{"claude-opus-4-6", "claude-opus-4-5", "claude-sonnet-4-5", "claude-haiku-4-5"},
			EndpointClass: providerprofile.EndpointNative,
			SecretRefs:    []string{"ANTHROPIC_API_KEY"}, OptionsSchema: defaultProviderOptionsSchema,
		}},
	}
}

type providerProfile struct{ profile providerprofile.Profile }

func (provider providerProfile) Definition() providerprofile.Profile { return provider.profile.Clone() }

func ProtectedToolProviders() []tool.ToolProvider {
	ids := tools.AssemblyControlledToolNames()
	out := make([]tool.ToolProvider, 0, len(ids))
	for _, id := range ids {
		out = append(out, protectedToolProvider{id: id})
	}
	return out
}

type protectedToolProvider struct{ id string }

func (p protectedToolProvider) Definition() tool.Definition { return tool.Definition{ID: p.id} }
func (p protectedToolProvider) Invoke(ctx context.Context, host tool.Host, args json.RawMessage) (tool.Result, error) {
	text, err := host.InvokeTool(ctx, p.id, args)
	return tool.Result{Text: text}, err
}

func NewMCPProvider() toolworld.Provider { return mcpProvider{} }

type mcpProvider struct{}

func (mcpProvider) Definition() toolworld.Definition {
	return toolworld.Definition{ID: "mcp", Description: "Model Context Protocol tools"}
}
func (mcpProvider) Discover(ctx context.Context, host toolworld.Host) ([]toolworld.ToolDefinition, error) {
	return nil, nil
}
func (mcpProvider) Invoke(ctx context.Context, host toolworld.Host, id string, args json.RawMessage) (toolworld.Result, error) {
	return toolworld.Result{}, toolworld.ErrInvalidArgs
}
func (mcpProvider) Close(context.Context) error { return nil }
