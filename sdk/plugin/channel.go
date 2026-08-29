package plugin

import (
	"context"
	"encoding/json"
	"net/http"
)

// Channel is the ABI of a seam-channel plugin (VIVY-CHANNEL-PACK.md §9.3).
// A channel plugin's Consumer is the kernel ChannelHost, never the model
// tool table: channel plugins are never Adapt-ed into tools.
type Channel interface {
	Name() string
	// Seam must return SeamChannel.
	Seam() Seam
	Grants() []Grant
	// Start runs until the context is cancelled or the channel gives up.
	// With GrantChannelPoll it may run an outbound long-poll or outbound
	// websocket client. It must never open a listen socket; Listen belongs
	// to the kernel ChannelHost.
	Start(ctx context.Context, env ChannelEnv) error
	// Stop releases platform resources. It must return even when the
	// context is already cancelled.
	Stop(ctx context.Context) error
	// Send delivers one outbound envelope and returns the platform
	// message ids it produced.
	Send(ctx context.Context, msg OutboundMessage) (ids []string, err error)
}

// ChannelEnv is the only world a channel plugin may touch. It is handed to
// Channel.Start by the kernel ChannelHost; missing grants fail closed.
type ChannelEnv interface {
	// Secret resolves a configured env_key name to its value.
	// Fail-closed: missing grant or missing value is an error, and values
	// are never logged.
	Secret(envKey string) (string, error)
	// HTTP returns an outbound-only client. There is no Listen capability.
	HTTP() *http.Client
	// PublishInbound is the only path an InboundMessage may travel into
	// the kernel — even when the plugin runs in the same process as the
	// Host. Adapters never write to the Journal or the session directly.
	PublishInbound(ctx context.Context, msg InboundMessage) error
	// Media returns the media store. The first cut may be a no-op.
	Media() MediaStore
}

// InboundMessage is the normalized envelope a channel adapter publishes
// toward the kernel. It travels world→kernel only through
// ChannelEnv.PublishInbound (even same-process), so the package can later
// move to a `vivy channel` subprocess without touching adapters.
//
// RunID/TaskID slots are Host-written and land with the ChannelHost (C3);
// adapters never synthesize them.
type InboundMessage struct {
	// Channel is the platform name, e.g. "telegram".
	Channel   string
	ChatID    string
	Sender    string
	MessageID string
	ReplyTo   string
	TopicID   string
	Parts     []Part
}

// OutboundMessage is the normalized envelope the kernel hands back to a
// channel adapter for delivery.
type OutboundMessage struct {
	ChatID  string
	ReplyTo string
	TopicID string
	Parts   []Part
}

// PartKind enumerates the typed envelope slots (VIVY-CHANNEL-PACK.md §8).
type PartKind string

const (
	PartText       PartKind = "text"
	PartMediaRef   PartKind = "media-ref"
	PartStructured PartKind = "structured"
)

// Part is one typed slot of an envelope. Envelopes are typed; a
// map[string]string is not the contract.
type Part struct {
	Kind       PartKind
	Text       string
	MediaRef   string
	Structured json.RawMessage
}

// Reserved capability slot (VIVY-CHANNEL-PACK.md §8); the method set lands
// when ChannelHost learns to assert it. MediaStore is the capability handle
// returned by ChannelEnv.Media.
type MediaStore interface{}

// Reserved capability slot (VIVY-CHANNEL-PACK.md §8); the method set lands
// when ChannelHost learns to assert it. Typing signals "typing…" on a chat.
type Typing interface{}

// Reserved capability slot (VIVY-CHANNEL-PACK.md §8); the method set lands
// when ChannelHost learns to assert it. MessageEditor edits a previously
// delivered message.
type MessageEditor interface{}

// Reserved capability slot (VIVY-CHANNEL-PACK.md §8); the method set lands
// when ChannelHost learns to assert it. Placeholder posts a replaceable
// "working…" message that is finalized when the run ends.
type Placeholder interface{}

// Reserved capability slot (VIVY-CHANNEL-PACK.md §8); the method set lands
// when ChannelHost learns to assert it. MediaSender delivers media parts.
type MediaSender interface{}

// Reserved capability slot (VIVY-CHANNEL-PACK.md §8); the method set lands
// when ChannelHost learns to assert it. WebhookHandler receives webhook
// deliveries on a socket the ChannelHost owns.
type WebhookHandler interface{}

// Reserved capability slot (VIVY-CHANNEL-PACK.md §8); the method set lands
// when ChannelHost learns to assert it. StreamingCapable streams outbound
// deltas.
type StreamingCapable interface{}

// Reserved capability slot (VIVY-CHANNEL-PACK.md §8); the method set lands
// when ChannelHost learns to assert it. TaskLifecycle carries long-running
// A2A tasks.
type TaskLifecycle interface{}

// Reserved capability slot (VIVY-CHANNEL-PACK.md §8); the method set lands
// when ChannelHost learns to assert it. PipeServer serves the NeuroLink
// pipe transport.
type PipeServer interface{}
