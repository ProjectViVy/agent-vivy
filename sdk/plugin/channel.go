package plugin

import (
	"context"
	"encoding/json"
	"io"
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
// RunID/TaskID slots are Host-written per VIVY-CHANNEL-PACK.md §8; they are
// deferred until a delivery path needs them (the C3 host journals inbound
// events before a run exists and tracks delivery targets internally), and
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

// MediaStore is the capability handle returned by ChannelEnv.Media. It
// accepts outbound media bytes and returns the opaque media-ref that
// Part.MediaRef carries.
//
// Minimal v1 surface; the first real adapter (C4) pins the ABI.
type MediaStore interface {
	Put(ctx context.Context, name string, r io.Reader) (ref string, err error)
}

// Typing signals "typing…" on a chat.
//
// Minimal v1 surface; the first real adapter (C4) pins the ABI.
type Typing interface {
	Typing(ctx context.Context, chatID string) error
}

// MessageEditor edits a previously delivered message.
//
// Minimal v1 surface; the first real adapter (C4) pins the ABI.
type MessageEditor interface {
	EditMessage(ctx context.Context, messageID string, msg OutboundMessage) error
}

// MessageDeleter deletes a previously delivered message.
//
// Minimal v1 surface; the first real adapter (C4) pins the ABI.
type MessageDeleter interface {
	DeleteMessage(ctx context.Context, chatID, messageID string) error
}

// ReactionSender adds an emoji reaction to a delivered message.
//
// Minimal v1 surface; the first real adapter (C4) pins the ABI.
type ReactionSender interface {
	React(ctx context.Context, chatID, messageID, emoji string) error
}

// Placeholder posts a replaceable "working…" message and returns its
// platform message id; the host finalizes it through MessageEditor.
//
// Minimal v1 surface; the first real adapter (C4) pins the ABI.
type Placeholder interface {
	Placeholder(ctx context.Context, chatID string) (messageID string, err error)
}

// MediaSender delivers one media part to a chat and returns the platform
// message ids it produced.
//
// Minimal v1 surface; the first real adapter (C4) pins the ABI.
type MediaSender interface {
	SendMedia(ctx context.Context, chatID string, part Part) (ids []string, err error)
}

// WebhookHandler receives webhook deliveries on a socket the ChannelHost
// owns. The adapter only declares the path and the handler; it never
// opens a listen socket itself.
//
// Minimal v1 surface; the first real adapter (C4) pins the ABI.
type WebhookHandler interface {
	WebhookPath() string
	WebhookHandler() http.Handler
}

// ListenHandler serves a dedicated channel transport on a socket the
// ChannelHost owns (reserved for heavy channels, e.g. NeuroLink). As with
// WebhookHandler, the adapter only declares the handler; Listen stays a
// ChannelHost capability.
//
// Minimal v1 surface; the first real adapter (C4) pins the ABI.
type ListenHandler interface {
	ListenHandler() http.Handler
}

// StreamingCapable streams outbound deltas onto a previously delivered
// message (edit-based streaming).
//
// Minimal v1 surface; the first real adapter (C4) pins the ABI.
type StreamingCapable interface {
	Stream(ctx context.Context, chatID, messageID, delta string) error
}

// HealthChecker lets the host probe the adapter's platform connectivity.
//
// Minimal v1 surface; the first real adapter (C4) pins the ABI.
type HealthChecker interface {
	Health(ctx context.Context) error
}

// Reserved capability slot (VIVY-CHANNEL-PACK.md §8); the method set lands
// when ChannelHost learns to assert it. TaskLifecycle carries long-running
// A2A tasks (stage H).
type TaskLifecycle interface{}

// Reserved capability slot (VIVY-CHANNEL-PACK.md §8); the method set lands
// when ChannelHost learns to assert it. PipeServer serves the NeuroLink
// pipe transport (stage H).
type PipeServer interface{}
