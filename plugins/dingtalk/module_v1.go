package dingtalk

import (
	"context"

	"agent-vivy/sdk/module"
	"agent-vivy/sdk/port/channel"
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

// MaxMessageRunes: the official custom-robot webhook caps text content at
// 20000 bytes (open.dingtalk.com, custom bot message types). Runes are up
// to 4 UTF-8 bytes, so 5000 is the worst-case-safe rune ceiling.
func (channelProvider) Definition() channel.Definition {
	return channel.Definition{ID: "vivy.dingtalk", MaxMessageRunes: 5000}
}
func (channelProvider) Construct(_ context.Context, host channel.Host) (channel.Instance, error) {
	return &boundChannel{adapter: newAdapter(), host: host}, nil
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

func (c *boundChannel) Start(ctx context.Context) error { return c.adapter.Start(ctx, c.host) }
func (c *boundChannel) Stop(ctx context.Context) error  { return c.adapter.Stop(ctx) }
func (c *boundChannel) Send(ctx context.Context, msg channel.OutboundMessage) ([]string, error) {
	return c.adapter.Send(ctx, msg)
}

// CapabilityTarget discloses the live adapter (plugin.CapabilitySource):
// once the instance exists, host call paths resolve to the adapter's own
// method set through the wrapper instead of the bind-time typed nil.
func (c *boundChannel) CapabilityTarget() any { return c.adapter }

func (vivyModule) Descriptor() module.Descriptor {
	return channelDescriptor("vivy/dingtalk", "vivy.dingtalk")
}
func channelDescriptor(id, provider string) module.Descriptor {
	return module.Descriptor{APIVersion: module.APIVersionV1, Module: module.Identity{ID: id, Version: "0.1.0"}, Source: module.Source{Ref: "repo:plugins/dingtalk", SHA256: "2bb377560b27c5f24db1535dc2eaf86148edbe9a9a2977970d86d7a4ff05fff8"}, Provides: []module.PortRef{{Port: "std/channel@v1", ID: provider}}, Requires: []module.Requirement{{PortRef: module.PortRef{Port: "core/channel-host@v1"}, Provider: "vivy/channel-host"}}, RequestedGrants: []module.Grant{module.GrantChannelPoll, module.GrantSecretRead, module.GrantNetClient}, Lifecycle: module.Lifecycle{Scope: module.ScopeGeneration}}
}
