package defaults

import (
	"context"

	"agent-vivy/sdk/module"
)

func NewKernel() module.Module           { return ownerModule{id: "vivy/kernel"} }
func NewToolHost() module.Module         { return ownerModule{id: "vivy/tool-host"} }
func NewProtectedTools() module.Module   { return ownerModule{id: "vivy/protected-tools"} }
func NewMCPHost() module.Module          { return ownerModule{id: "vivy/mcp-host"} }
func NewContextHost() module.Module      { return ownerModule{id: "vivy/context-host"} }
func NewContextSource() module.Module    { return ownerModule{id: "vivy/context-source"} }
func NewSkillHost() module.Module        { return ownerModule{id: "vivy/skill-host"} }
func NewSkillSource() module.Module      { return ownerModule{id: "vivy/skill-source"} }
func NewChannelHost() module.Module      { return ownerModule{id: "vivy/channel-host"} }
func NewFaceHost() module.Module         { return ownerModule{id: "vivy/face-host"} }
func NewObserverHost() module.Module     { return ownerModule{id: "vivy/observer-host"} }
func NewStatusHost() module.Module       { return ownerModule{id: "vivy/status-host"} }
func NewProviderProfiles() module.Module { return ownerModule{id: "vivy/provider-profiles"} }

type ownerModule struct{ id string }

func (m ownerModule) Descriptor() module.Descriptor {
	return module.Descriptor{APIVersion: module.APIVersionV1, Module: module.Identity{ID: m.id, Version: "1.0.0"}, Source: module.Source{Ref: "repo:internal", SHA256: "0000000000000000000000000000000000000000000000000000000000000000"}, Lifecycle: module.Lifecycle{Scope: module.ScopeGeneration}}
}
func (ownerModule) Construct(context.Context, module.Host) (module.Instance, error) {
	return ownerInstance{}, nil
}

type ownerInstance struct{}

func (ownerInstance) Start(context.Context) error { return nil }
func (ownerInstance) Ready(context.Context) error { return nil }
func (ownerInstance) Stop(context.Context) error  { return nil }
func (ownerInstance) Close(context.Context) error { return nil }
