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
// without constructing one (plugin.CapabilitySource). This bind-time probe
// is only ever type-asserted; once Start has built the instance, the
// boundChannel disclosure below exposes the live adapter for host call
// paths (typing, health probes).
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

// CapabilityTarget discloses the live adapter (plugin.CapabilitySource):
// once the instance exists, host call paths resolve to the adapter's own
// method set through the wrapper instead of the bind-time typed nil.
func (c *boundChannel) CapabilityTarget() any { return c.adapter }

func (vivyModule) Descriptor() module.Descriptor {
	return module.Descriptor{APIVersion: module.APIVersionV1, Module: module.Identity{ID: "vivy/telegram", Version: "0.1.0"}, Source: module.Source{Ref: "repo:plugins/telegram", SHA256: "32b8f1c210f23ef21ff1b83e75f720a52fc7efd67a0119539a4b8b513213f971"}, Provides: []module.PortRef{{Port: "std/channel@v1", ID: "vivy.telegram"}}, Requires: []module.Requirement{{PortRef: module.PortRef{Port: "core/channel-host@v1"}, Provider: "vivy/channel-host"}}, RequestedGrants: []module.Grant{module.GrantChannelPoll, module.GrantSecretRead, module.GrantNetClient}, Lifecycle: module.Lifecycle{Scope: module.ScopeGeneration}}
}
