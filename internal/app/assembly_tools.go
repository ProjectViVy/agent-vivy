package app

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/modules/defaults"
	"agent-vivy/internal/toolhost"
	"agent-vivy/internal/tools"
	toolport "agent-vivy/sdk/port/tool"
	toolworldport "agent-vivy/sdk/port/toolworld"
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

type legacyToolHost string

func (host legacyToolHost) ModuleID() string { return string(host) }
func (host legacyToolHost) InvokeTool(context.Context, string, json.RawMessage) (string, error) {
	return "", fmt.Errorf("app: legacy Tool adapter cannot invoke nested tools")
}

type legacyToolProvider struct {
	tool tools.Tool
}

func (provider legacyToolProvider) Definition() toolport.Definition {
	spec := provider.tool.Spec()
	effect := toolport.EffectWrite
	if spec.Readonly {
		effect = toolport.EffectRead
	}
	return toolport.Definition{
		ID:          spec.Name,
		Description: spec.Description,
		Effect:      effect,
		Schema:      schemaFromToolSpec(spec),
	}
}

func (provider legacyToolProvider) Invoke(ctx context.Context, _ toolport.Host, args json.RawMessage) (toolport.Result, error) {
	text, err := provider.tool.InvokableRun(ctx, args)
	return toolport.Result{Text: text}, err
}

func schemaFromToolSpec(spec domain.ToolSpec) json.RawMessage {
	properties := make(map[string]map[string]any, len(spec.Params))
	names := make([]string, 0, len(spec.Params))
	for name := range spec.Params {
		names = append(names, name)
	}
	sort.Strings(names)
	required := make([]string, 0, len(spec.Params))
	for _, name := range names {
		param := spec.Params[name]
		property := map[string]any{}
		paramType := strings.TrimSpace(param.Type)
		if paramType == "" {
			paramType = "string"
		}
		property["type"] = paramType
		if param.Desc != "" {
			property["description"] = param.Desc
		}
		if len(param.Enum) > 0 {
			property["enum"] = append([]string(nil), param.Enum...)
		}
		properties[name] = property
		if param.Required {
			required = append(required, name)
		}
	}
	schema := map[string]any{"type": "object", "properties": properties}
	if len(required) > 0 {
		schema["required"] = required
	}
	raw, _ := json.Marshal(schema)
	return raw
}

type governedTool struct {
	host *toolhost.Host
	id   string
	spec domain.ToolSpec
}

func (tool governedTool) GovernedToolID() string { return tool.id }
func (tool governedTool) Spec() domain.ToolSpec  { return tool.spec }
func (tool governedTool) InvokableRun(ctx context.Context, args json.RawMessage) (string, error) {
	result, err := tool.host.Invoke(ctx, toolhost.Request{ID: tool.id, Args: args})
	return result.Text, err
}

type governedClassifiedTool struct {
	governedTool
	classifier tools.InvocationClassifier
}

func (tool governedClassifiedTool) ClassifyInvocation(args json.RawMessage) (tools.InvocationClass, []string, error) {
	return tool.classifier.ClassifyInvocation(args)
}

type governedProposalTool struct {
	governedTool
	proposal tools.ProposalProvider
}

func (tool governedProposalTool) PrepareProposal(ctx context.Context, args json.RawMessage) (domain.ToolProposal, error) {
	return tool.proposal.PrepareProposal(ctx, args)
}

type governedClassifiedProposalTool struct {
	governedTool
	classifier tools.InvocationClassifier
	proposal   tools.ProposalProvider
}

func (tool governedClassifiedProposalTool) ClassifyInvocation(args json.RawMessage) (tools.InvocationClass, []string, error) {
	return tool.classifier.ClassifyInvocation(args)
}

func (tool governedClassifiedProposalTool) PrepareProposal(ctx context.Context, args json.RawMessage) (domain.ToolProposal, error) {
	return tool.proposal.PrepareProposal(ctx, args)
}

func newGovernedRuntimeTool(host *toolhost.Host, id string, spec domain.ToolSpec, behavior tools.Tool) tools.Tool {
	base := governedTool{host: host, id: id, spec: spec}
	classifier, classified := behavior.(tools.InvocationClassifier)
	proposal, proposed := behavior.(tools.ProposalProvider)
	switch {
	case classified && proposed:
		return governedClassifiedProposalTool{governedTool: base, classifier: classifier, proposal: proposal}
	case classified:
		return governedClassifiedTool{governedTool: base, classifier: classifier}
	case proposed:
		return governedProposalTool{governedTool: base, proposal: proposal}
	default:
		return base
	}
}

func bindGeneratedTools(providers []toolport.ToolProvider, registry *tools.Registry) (*tools.Registry, error) {
	if registry == nil {
		return nil, fmt.Errorf("app: ToolHost requires a registry projection")
	}

	static := make([]toolhost.StaticBinding, 0, len(registry.Specs())+len(providers))
	worlds := make([]toolhost.WorldBinding, 0)
	specByID := make(map[string]domain.ToolSpec)
	behaviorByID := make(map[string]tools.Tool)
	legacyOrder := make([]string, 0, len(registry.Specs()))
	generatedOrder := make([]string, 0, len(providers))
	discoveryCtx := context.Background()

	for _, spec := range registry.Specs() {
		implementation, ok := registry.Lookup(spec.Name)
		if !ok || implementation == nil {
			continue
		}
		if stage, ok := implementation.(generatedWorldStage); ok {
			stage := stage
			if stage.ctx != nil {
				discoveryCtx = stage.ctx
			}
			worlds = append(worlds, toolhost.WorldBinding{
				OwnerID:  stage.provider.Definition().ID,
				Provider: stage.provider,
				HostFor: func(ctx context.Context) toolworldport.Host {
					return generatedWorldHost{
						moduleID: stage.provider.Definition().ID,
						grants:   stage.grants,
						lookup:   stage.lookup,
						recorder: stage.recorder,
						ctx:      ctx,
					}
				},
			})
			continue
		}
		if tools.IsAssemblyControlledTool(spec.Name) {
			continue
		}
		static = append(static, toolhost.StaticBinding{
			OwnerID:  "vivy/legacy/" + spec.Name,
			Provider: legacyToolProvider{tool: implementation},
			Host:     legacyToolHost("vivy/legacy/" + spec.Name),
			Trust:    toolhost.TrustPublic,
		})
		specByID[spec.Name] = spec
		behaviorByID[spec.Name] = implementation
		legacyOrder = append(legacyOrder, spec.Name)
	}

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
		if !ok && tools.IsAssemblyControlledTool(id) {
			continue
		}
		host := &generatedToolHost{providerID: id}
		definition := provider.Definition()
		spec := domain.ToolSpec{
			Name:        id,
			Description: definition.Description,
			Readonly:    definition.Effect != toolport.EffectWrite,
			Params:      schemaParams(definition.Schema),
		}
		if ok && implementation != nil {
			host.implementation = implementation
			spec = implementation.Spec()
			behaviorByID[id] = implementation
		}
		trust := toolhost.TrustPublic
		if defaults.IsProtectedToolProvider(provider) {
			trust = toolhost.TrustCore
		}
		static = append(static, toolhost.StaticBinding{OwnerID: id, Provider: provider, Host: host, Trust: trust})
		specByID[id] = spec
		generatedOrder = append(generatedOrder, id)
	}

	host, err := toolhost.New(toolhost.Config{
		Static:       static,
		Worlds:       worlds,
		ProtectedIDs: tools.AssemblyControlledToolNames(),
	})
	if err != nil {
		return nil, fmt.Errorf("app: build ToolHost: %w", err)
	}
	if _, err := host.Discover(discoveryCtx); err != nil {
		return nil, fmt.Errorf("app: discover ToolHost worlds: %w", err)
	}

	dynamicOrder := make([]string, 0)
	for _, definition := range host.ListVisible() {
		entry, ok := host.Lookup(definition.ID)
		if !ok || !entry.Dynamic {
			continue
		}
		specByID[definition.ID] = domain.ToolSpec{
			Name:        definition.ID,
			Description: definition.Description,
			Readonly:    definition.Effect != toolport.EffectWrite,
			Keywords:    []string{entry.WorldID, definition.ID},
			Params:      schemaParams(definition.Schema),
		}
		dynamicOrder = append(dynamicOrder, definition.ID)
	}

	order := make([]string, 0, len(legacyOrder)+len(dynamicOrder)+len(generatedOrder))
	order = append(order, legacyOrder...)
	order = append(order, dynamicOrder...)
	order = append(order, generatedOrder...)
	governed := make([]tools.Tool, 0, len(order))
	for _, id := range order {
		spec, ok := specByID[id]
		if !ok {
			return nil, fmt.Errorf("app: ToolHost catalog missing runtime spec for %s", id)
		}
		governed = append(governed, newGovernedRuntimeTool(host, id, spec, behaviorByID[id]))
	}
	return tools.NewRegistry(governed...), nil
}

// BindGeneratedTools exposes the internal ToolHost binding boundary to the
// SDK conformance suite. Product composition uses the same implementation.
func BindGeneratedTools(providers []toolport.ToolProvider, registry *tools.Registry) (*tools.Registry, error) {
	return bindGeneratedTools(providers, registry)
}
