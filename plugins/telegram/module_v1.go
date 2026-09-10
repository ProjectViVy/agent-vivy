package telegram

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
func (channelProvider) Definition() channel.Definition {
	return channel.Definition{ID: "vivy.telegram", MaxMessageRunes: 4096}
}
func (channelProvider) Construct(_ context.Context, h channel.Host) (channel.Instance, error) {
	return &boundChannel{adapter: newAdapter(), host: h}, nil
}

type boundChannel struct {
	adapter *Plugin
	host    channel.Host
}

func (c *boundChannel) Start(x context.Context) error { return c.adapter.Start(x, c.host) }
func (c *boundChannel) Stop(x context.Context) error  { return c.adapter.Stop(x) }
func (c *boundChannel) Send(x context.Context, m channel.OutboundMessage) ([]string, error) {
	return c.adapter.Send(x, m)
}
func (vivyModule) Descriptor() module.Descriptor {
	return module.Descriptor{APIVersion: module.APIVersionV1, Module: module.Identity{ID: "vivy/telegram", Version: "0.1.0"}, Source: module.Source{Ref: "repo:plugins/telegram", SHA256: "c3b73462f1e9fa55abd1d544a31efe0014676d76cc677278505dafa54f34b7d0"}, Provides: []module.PortRef{{Port: "std/channel@v1", ID: "vivy.telegram"}}, Requires: []module.Requirement{{PortRef: module.PortRef{Port: "core/channel-host@v1"}, Provider: "vivy/channel-host"}}, RequestedGrants: []module.Grant{module.GrantChannelPoll, module.GrantSecretRead, module.GrantNetClient}, Lifecycle: module.Lifecycle{Scope: module.ScopeGeneration}}
}
