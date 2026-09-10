// Package channel defines the focused std/channel@v1 Provider contract.
package channel

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"

	"agent-vivy/sdk/module"
)

var ErrDenied = errors.New("channel capability denied")

type Grant = module.Grant

const (
	GrantChannelPoll = module.GrantChannelPoll
	GrantSecretRead  = module.GrantSecretRead
)

type Definition struct {
	ID              string
	MaxMessageRunes int
}
type ChannelProvider interface {
	Definition() Definition
	Construct(context.Context, Host) (Instance, error)
}
type Instance interface {
	Start(context.Context) error
	Stop(context.Context) error
	Send(context.Context, OutboundMessage) ([]string, error)
}

// Channel is the focused host-owned adapter shape. Assemblies expose
// ChannelProvider values; channel hosts bind those providers into Channels.
type Channel interface {
	Name() string
	Grants() []module.Grant
	Start(context.Context, Host) error
	Stop(context.Context) error
	Send(context.Context, OutboundMessage) ([]string, error)
}
type Host interface {
	module.Host
	Secret(string) (string, error)
	HTTP() *http.Client
	DialTLS(context.Context, string, string) (net.Conn, error)
	Settings() json.RawMessage
	PublishInbound(context.Context, InboundMessage) error
	Media() MediaStore
	Logger() *slog.Logger
}

// ChannelEnv and ChannelLogger retain the focused Host vocabulary used by
// adapters while the construction call now binds the Host once.
type ChannelEnv = Host
type ChannelLogger interface{ Logger() *slog.Logger }
type InboundMessage struct {
	Channel, ChatID, Sender, MessageID, ReplyTo, TopicID string
	Parts                                                []Part
}
type OutboundMessage struct {
	ChatID, ReplyTo, TopicID string
	Parts                    []Part
}
type PartKind string

const (
	PartText       PartKind = "text"
	PartMediaRef   PartKind = "media-ref"
	PartStructured PartKind = "structured"
)

type Part struct {
	Kind           PartKind
	Text, MediaRef string
	Structured     json.RawMessage
}
type MediaStore interface {
	Put(context.Context, string, io.Reader) (string, error)
}
type RunesLimiter interface{ MaxMessageRunes() int }
type Typing interface {
	Typing(context.Context, string) error
}
type MessageEditor interface {
	EditMessage(context.Context, string, OutboundMessage) error
}
type MessageDeleter interface {
	DeleteMessage(context.Context, string, string) error
}
type ReactionSender interface {
	React(context.Context, string, string, string) error
}
type Placeholder interface {
	Placeholder(context.Context, string) (string, error)
}
type MediaSender interface {
	SendMedia(context.Context, string, Part) ([]string, error)
}
type WebhookHandler interface {
	WebhookPath() string
	WebhookHandler() http.Handler
}
type ListenHandler interface{ ListenHandler() http.Handler }
type StreamingCapable interface {
	Stream(context.Context, string, string, string) error
}
type HealthChecker interface{ Health(context.Context) error }
type TaskLifecycle interface{}
type PipeServer interface{}
