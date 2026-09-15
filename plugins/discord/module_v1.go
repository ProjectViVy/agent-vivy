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

func NewProvider() channel.ChannelProvider { return channelProvider{} }

// MaxMessageRunes: the official create-message endpoint caps content at
// 2000 characters (discord.com/developers/docs, create message) and
// rejects longer bodies with 400 50035.
func (channelProvider) Definition() channel.Definition {
	return channel.Definition{ID: "vivy.discord", MaxMessageRunes: 2000}
}
func (channelProvider) Construct(_ context.Context, h channel.Host) (channel.Instance, error) {
	return &boundChannel{adapter: newAdapter(), host: h}, nil
}

// CapabilityTarget points capability discovery at the adapter's method set
// without constructing one (plugin.CapabilitySource). The value is only
// ever type-asserted, never called, so the typed nil is enough.
func (channelProvider) CapabilityTarget() any { return (*Plugin)(nil) }

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
	return module.Descriptor{APIVersion: module.APIVersionV1, Module: module.Identity{ID: "vivy/discord", Version: "0.1.0"}, Source: module.Source{Ref: "repo:plugins/discord", SHA256: "4715b2c2c8ed61cba34d2dfc95a9da0730850c0b52b92372d69482f4aaa9a40e"}, Provides: []module.PortRef{{Port: "std/channel@v1", ID: "vivy.discord"}}, Requires: []module.Requirement{{PortRef: module.PortRef{Port: "core/channel-host@v1"}, Provider: "vivy/channel-host"}}, RequestedGrants: []module.Grant{module.GrantChannelPoll, module.GrantSecretRead, module.GrantNetClient}, Lifecycle: module.Lifecycle{Scope: module.ScopeGeneration}}
}
