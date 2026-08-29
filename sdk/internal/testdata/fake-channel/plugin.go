// Package fakechannel is a standalone-module seam-channel fixture used by
// the SDK verify and pack tests. It lives in its own go.mod so pack must
// overlay the root go.mod to import it (VIVY-CHANNEL-PACK.md §9.1).
package fakechannel

import (
	"context"

	"agent-vivy/sdk/plugin"
)

// Compile-time proof that the fixture implements the channel ABI.
var _ plugin.Channel = Plugin{}

// Plugin is the smallest well-formed seam-channel plugin.
type Plugin struct{}

// New is the pack entry point.
func New() plugin.Plugin { return Plugin{} }

// Name matches the manifest name.
func (Plugin) Name() string { return "fake-channel" }

// Seam marks this plugin as a channel; its Consumer is the kernel
// ChannelHost, never the tool table.
func (Plugin) Seam() plugin.Seam { return plugin.SeamChannel }

// Grants are the channel-family grants this batch allows.
func (Plugin) Grants() []plugin.Grant {
	return []plugin.Grant{plugin.GrantChannelPoll, plugin.GrantSecretRead}
}

// Tools is empty: a channel is not a model tool.
func (Plugin) Tools() []plugin.Tool { return nil }

// Start proves the ChannelEnv flows by publishing one inbound envelope
// through the only world→kernel path.
func (Plugin) Start(ctx context.Context, env plugin.ChannelEnv) error {
	return env.PublishInbound(ctx, plugin.InboundMessage{
		Channel: "fake-channel",
		ChatID:  "fixture",
		Sender:  "fixture",
		Parts:   []plugin.Part{{Kind: plugin.PartText, Text: "hello"}},
	})
}

// Stop is a no-op for the fixture.
func (Plugin) Stop(ctx context.Context) error { return nil }

// Send is a no-op for the fixture and reports no platform ids.
func (Plugin) Send(ctx context.Context, msg plugin.OutboundMessage) ([]string, error) {
	return nil, nil
}
