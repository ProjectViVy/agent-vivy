package discord

import (
	"agent-vivy/sdk/module"
	"agent-vivy/sdk/port/channel"
	"context"
)

type vivyModule struct{}

func New() module.Module { return vivyModule{} }
func (vivyModule) Construct(context.Context, module.Host) (module.Instance, error) {
	return moduleInstance{}, nil
}

type moduleInstance struct{}

func (moduleInstance) Start(context.Context) error { return nil }
func (moduleInstance) Ready(context.Context) error { return nil }
func (moduleInstance) Stop(context.Context) error  { return nil }
func (moduleInstance) Close(context.Context) error { return nil }

type channelProvider struct{}

func NewProvider() channel.ChannelProvider             { return channelProvider{} }
func (channelProvider) Definition() channel.Definition { return channel.Definition{ID: "vivy.discord"} }
func (channelProvider) Construct(_ context.Context, h channel.Host) (channel.Instance, error) {
	return &boundChannel{adapter: newAdapter(), host: h}, nil
}

type boundChannel struct {
	adapter *Plugin
	host    channel.Host
}

func (c *boundChannel) Start(ctx context.Context) error { return c.adapter.Start(ctx, c.host) }
func (c *boundChannel) Stop(ctx context.Context) error  { return c.adapter.Stop(ctx) }
func (c *boundChannel) Send(ctx context.Context, m channel.OutboundMessage) ([]string, error) {
	return c.adapter.Send(ctx, m)
}
func (vivyModule) Descriptor() module.Descriptor {
	return module.Descriptor{APIVersion: module.APIVersionV1, Module: module.Identity{ID: "vivy/discord", Version: "0.1.0"}, Source: module.Source{Ref: "repo:plugins/discord", SHA256: "82b537cc9f47cdd2d6fa4cc30a7f08740c633a78250a1b158e1fa861a794ff7c"}, Provides: []module.PortRef{{Port: "std/channel@v1", ID: "vivy.discord"}}, Requires: []module.Requirement{{PortRef: module.PortRef{Port: "core/channel-host@v1"}, Provider: "vivy/channel-host"}}, RequestedGrants: []module.Grant{module.GrantChannelPoll, module.GrantSecretRead, module.GrantNetClient}, Lifecycle: module.Lifecycle{Scope: module.ScopeGeneration}}
}
