// A2A server Module: exposes the optional governed task surface through
// the official A2A JSON-RPC protocol. The module is removable — absent
// from a Recipe, no listener, route or SDK dependency ships.
package a2aserver

import (
	"context"
	"net/http"

	"agent-vivy/sdk/module"
	"agent-vivy/sdk/port/channel"
)

type vivyModule struct{}

// New is the v1 Module entrypoint.
func New() module.Module { return vivyModule{} }

func (vivyModule) Construct(context.Context, module.Host) (module.Instance, error) {
	return moduleInstance{}, nil
}

type moduleInstance struct{}

func (moduleInstance) Start(context.Context) error { return nil }
func (moduleInstance) Ready(context.Context) error { return nil }
func (moduleInstance) Stop(context.Context) error  { return nil }
func (moduleInstance) Close(context.Context) error { return nil }

// Descriptor is the module manifest truth: projectvivy/a2a-server
// provides the a2a channel provider and requires the channel host.
func (vivyModule) Descriptor() module.Descriptor {
	return module.Descriptor{
		APIVersion: module.APIVersionV1,
		Module:     module.Identity{ID: "projectvivy/a2a-server", Version: "0.1.0"},
		Source:     module.Source{Ref: "repo:plugins/a2a-server"},
		Provides:   []module.PortRef{{Port: "std/channel@v1", ID: "vivy.a2a"}},
		Requires: []module.Requirement{
			{PortRef: module.PortRef{Port: "core/channel-host@v1"}, Provider: "vivy/channel-host"},
		},
		RequestedGrants: []module.Grant{module.GrantChannelA2A, module.GrantSecretRead},
		Lifecycle:       module.Lifecycle{Scope: module.ScopeGeneration},
	}
}

type channelProvider struct{}

// NewProvider is the channel-provider entrypoint the Assembly binds.
func NewProvider() channel.ChannelProvider { return channelProvider{} }

// Definition IDs the provider as "a2a" — the channel envelope name the
// operator writes is channels.<name> where <name> matches the binding,
// and the provider is selected by this definition ID.
func (channelProvider) Definition() channel.Definition {
	return channel.Definition{ID: "vivy.a2a"}
}

func (channelProvider) Construct(_ context.Context, h channel.Host) (channel.Instance, error) {
	settings, err := decodeSettings(h.Settings())
	if err != nil {
		return nil, err
	}
	return &boundChannel{host: h, settings: settings}, nil
}

// CapabilityTarget points capability discovery at the instance method set
// (typed-nil safe): the adapter type, not a live instance.
func (channelProvider) CapabilityTarget() any { return (*boundChannel)(nil) }

// boundChannel is the constructed instance: Start prepares the handler,
// Stop releases it; outbound Send is typed-unsupported (server channel).
type boundChannel struct {
	host     channel.Host
	settings a2aSettings
	handler  http.Handler
}

func (c *boundChannel) Start(context.Context) error {
	// The task surface is optional: assert the bound env against the
	// TaskHost/TaskServiceInfoHost contracts. Without them the channel
	// still starts — ListenHandler reports absent and no route mounts.
	tasks, tok := c.host.(channel.TaskHost)
	info, iok := c.host.(channel.TaskServiceInfoHost)
	if tok && iok {
		c.handler = newHTTPHandler(newRequestHandler(tasks, info))
	}
	return nil
}

func (c *boundChannel) Stop(context.Context) error {
	c.handler = nil
	return nil
}

// Send is typed-unsupported: the A2A server surface is inbound-only in
// this generation; outbound delivery uses native channels.
func (c *boundChannel) Send(context.Context, channel.OutboundMessage) ([]string, error) {
	return nil, &channel.TaskError{Code: channel.TaskErrUnsupported, Message: "a2a server channel does not deliver outbound messages"}
}

// CapabilityTarget discloses the live instance (plugin.CapabilitySource).
func (c *boundChannel) CapabilityTarget() any { return c }

// ListenHandler returns the live handler after Start, nil before —
// never a typed-nil discovery target.
func (c *boundChannel) ListenHandler() http.Handler { return c.handler }
