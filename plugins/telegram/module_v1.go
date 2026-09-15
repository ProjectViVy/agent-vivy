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

// MaxMessageRunes: the official Bot API caps sendMessage text at 4096
// characters. This Definition is the single source of the ceiling; the
// adapter no longer repeats it.
func (channelProvider) Definition() channel.Definition {
	return channel.Definition{ID: "vivy.telegram", MaxMessageRunes: 4096}
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

func (c *boundChannel) Start(x context.Context) error { return c.adapter.Start(x, c.host) }
func (c *boundChannel) Stop(x context.Context) error  { return c.adapter.Stop(x) }
func (c *boundChannel) Send(x context.Context, m channel.OutboundMessage) ([]string, error) {
	return c.adapter.Send(x, m)
}
func (vivyModule) Descriptor() module.Descriptor {
	return module.Descriptor{APIVersion: module.APIVersionV1, Module: module.Identity{ID: "vivy/telegram", Version: "0.1.0"}, Source: module.Source{Ref: "repo:plugins/telegram", SHA256: "043498e194151b08792177a149902343a198c26d0510598cfdf947f264535eb2"}, Provides: []module.PortRef{{Port: "std/channel@v1", ID: "vivy.telegram"}}, Requires: []module.Requirement{{PortRef: module.PortRef{Port: "core/channel-host@v1"}, Provider: "vivy/channel-host"}}, RequestedGrants: []module.Grant{module.GrantChannelPoll, module.GrantSecretRead, module.GrantNetClient}, Lifecycle: module.Lifecycle{Scope: module.ScopeGeneration}}
}
