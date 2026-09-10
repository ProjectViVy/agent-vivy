// Package fake provides an in-memory channel adapter for the ChannelHost
// TCK and for adapter authors. It is a test double, never a product
// plugin: it lives outside plugins/, implements no optional capability
// interface, and never opens a network connection.
package fake

import (
	"context"
	"sync"

	plugin "agent-vivy/sdk/port/channel"
)

// Channel is the test double. Start runs the injectable Publish func
// (default: publish one hello inbound message); Send records outbound
// envelopes in memory.
type Channel struct {
	// Publish, when set, replaces the default Start behavior. It receives
	// the env the host handed out, so tests drive the real PublishInbound
	// surface.
	Publish func(ctx context.Context, env plugin.ChannelEnv) error

	// Sent accumulates the envelopes delivered through Send. Guarded by mu.
	Sent []plugin.OutboundMessage

	mu sync.Mutex
}

// New returns a ready-to-start fake channel.
func New() *Channel { return &Channel{} }

// Name returns the configured channel key.
func (c *Channel) Name() string { return "fake" }

// Grants declares the capabilities exercised by the fake.
func (c *Channel) Grants() []plugin.Grant {
	return []plugin.Grant{plugin.GrantChannelPoll, plugin.GrantSecretRead}
}

// Start implements plugin.Channel. The default publish sends one hello
// text from sender "alice" in chat "chat-1".
func (c *Channel) Start(ctx context.Context, env plugin.ChannelEnv) error {
	if c.Publish != nil {
		return c.Publish(ctx, env)
	}
	return env.PublishInbound(ctx, plugin.InboundMessage{
		Channel:   "fake",
		ChatID:    "chat-1",
		Sender:    "alice",
		MessageID: "m-1",
		Parts:     []plugin.Part{{Kind: plugin.PartText, Text: "hello vivy"}},
	})
}

// Stop implements plugin.Channel; the fake holds no platform resources.
func (c *Channel) Stop(_ context.Context) error { return nil }

// Send implements plugin.Channel: the envelope is recorded, ids are
// synthetic.
func (c *Channel) Send(_ context.Context, msg plugin.OutboundMessage) ([]string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Sent = append(c.Sent, msg)
	return []string{"fake-out-1"}, nil
}

// Snapshot returns a copy of the recorded outbound envelopes.
func (c *Channel) Snapshot() []plugin.OutboundMessage {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]plugin.OutboundMessage(nil), c.Sent...)
}
