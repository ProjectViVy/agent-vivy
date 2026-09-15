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
func (channelProvider) Definition() channel.Definition {
	return channel.Definition{ID: "vivy.dingtalk"}
}
func (channelProvider) Construct(_ context.Context, host channel.Host) (channel.Instance, error) {
	return &boundChannel{adapter: newAdapter(), host: host}, nil
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
func (c *boundChannel) Send(ctx context.Context, msg channel.OutboundMessage) ([]string, error) {
	return c.adapter.Send(ctx, msg)
}
func (vivyModule) Descriptor() module.Descriptor {
	return channelDescriptor("vivy/dingtalk", "vivy.dingtalk")
}
func channelDescriptor(id, provider string) module.Descriptor {
	return module.Descriptor{APIVersion: module.APIVersionV1, Module: module.Identity{ID: id, Version: "0.1.0"}, Source: module.Source{Ref: "repo:plugins/dingtalk", SHA256: "4886d9c9abb35b7629d544ff81f6f5ad7574ef3902adaf01ed49a32a57a62843"}, Provides: []module.PortRef{{Port: "std/channel@v1", ID: provider}}, Requires: []module.Requirement{{PortRef: module.PortRef{Port: "core/channel-host@v1"}, Provider: "vivy/channel-host"}}, RequestedGrants: []module.Grant{module.GrantChannelPoll, module.GrantSecretRead, module.GrantNetClient}, Lifecycle: module.Lifecycle{Scope: module.ScopeGeneration}}
}
