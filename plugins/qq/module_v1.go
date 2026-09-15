package qq

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

// MaxMessageRunes: the platform publishes no text length limit — over-length
// sends are rejected with 40054007 (message too long) — so 2000 follows
// community practice (aligned with Discord) as the safe ceiling.
func (channelProvider) Definition() channel.Definition {
	return channel.Definition{ID: "vivy.qq", MaxMessageRunes: 2000}
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
	return module.Descriptor{APIVersion: module.APIVersionV1, Module: module.Identity{ID: "vivy/qq", Version: "0.1.0"}, Source: module.Source{Ref: "repo:plugins/qq", SHA256: "12d3e1dc0858840db555bb503ae9cfefe35953de6be285e0faaf3a9901b62170"}, Provides: []module.PortRef{{Port: "std/channel@v1", ID: "vivy.qq"}}, Requires: []module.Requirement{{PortRef: module.PortRef{Port: "core/channel-host@v1"}, Provider: "vivy/channel-host"}}, RequestedGrants: []module.Grant{module.GrantChannelPoll, module.GrantSecretRead, module.GrantNetClient}, Lifecycle: module.Lifecycle{Scope: module.ScopeGeneration}}
}
