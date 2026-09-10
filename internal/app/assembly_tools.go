package app

import (
	"context"
	"encoding/json"
	"fmt"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/tools"
	toolport "agent-vivy/sdk/port/tool"
)

type generatedToolHost struct {
	providerID     string
	implementation tools.Tool
}

func (host *generatedToolHost) ModuleID() string { return host.providerID }

func (host *generatedToolHost) InvokeTool(ctx context.Context, id string, args json.RawMessage) (string, error) {
	if id != host.providerID {
		return "", fmt.Errorf("generated Tool %s cannot invoke protected implementation %s", host.providerID, id)
	}
	if host.implementation == nil {
		return "", fmt.Errorf("generated Tool %s has no host implementation", id)
	}
	return host.implementation.InvokableRun(ctx, args)
}

type generatedTool struct {
	provider toolport.ToolProvider
	host     *generatedToolHost
	spec     domain.ToolSpec
}

func (tool generatedTool) Spec() domain.ToolSpec { return tool.spec }

func (tool generatedTool) InvokableRun(ctx context.Context, args json.RawMessage) (string, error) {
	result, err := tool.provider.Invoke(ctx, tool.host, args)
	return result.Text, err
}

func bindGeneratedTools(providers []toolport.ToolProvider, registry *tools.Registry) (*tools.Registry, error) {
	generated := make([]tools.Tool, 0, len(providers))
	seen := make(map[string]bool, len(providers))
	for _, provider := range providers {
		if provider == nil || provider.Definition().ID == "" {
			return nil, fmt.Errorf("app: generated Tool provider has no identity")
		}
		id := provider.Definition().ID
		if seen[id] {
			return nil, fmt.Errorf("app: duplicate generated Tool provider %s", id)
		}
		seen[id] = true
		implementation, ok := registry.Lookup(id)
		if !ok {
			// Some kernel tools are configuration-conditional (for example,
			// filesystem mutation without a workspace). The Assembly still
			// owns their compiled identity; the Host activates only those for
			// which this process composition has an implementation.
			if tools.IsAssemblyControlledTool(id) {
				continue
			}
			definition := provider.Definition()
			generated = append(generated, generatedTool{provider: provider, host: &generatedToolHost{providerID: id}, spec: domain.ToolSpec{Name: id, Description: definition.Description, Readonly: definition.Effect != toolport.EffectWrite, Params: schemaParams(definition.Schema)}})
			continue
		}
		generated = append(generated, generatedTool{provider: provider, host: &generatedToolHost{providerID: id, implementation: implementation}, spec: implementation.Spec()})
	}
	return registry.Without(tools.AssemblyControlledToolNames()...).WithAdditional(generated...), nil
}

// BindGeneratedTools exposes the internal ToolHost binding boundary to the
// SDK conformance suite. Product composition uses the same implementation.
func BindGeneratedTools(providers []toolport.ToolProvider, registry *tools.Registry) (*tools.Registry, error) {
	return bindGeneratedTools(providers, registry)
}
