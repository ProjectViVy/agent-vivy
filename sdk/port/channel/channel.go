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

// CapabilitySource lets a wrapper channel point capability discovery at the
// object whose concrete method set defines the optional surface — the
// adapter itself — instead of at the wrapper. Discovery only asserts
// interfaces against the returned value, so a typed nil pointer is a valid
// side-effect-free bind-time target. Host call paths (typing, health
// probes) call through the same seam after Start: a wrapper exposes the
// live instance's target once constructed, and the host never calls on the
// bind-time probe. A wrapper must never implement the optional capability
// interfaces itself: that would advertise capabilities the adapter does
// not have (VIVY-CHANNEL-PACK.md §7 capability discovery, §8 matrix).
type CapabilitySource interface{ CapabilityTarget() any }

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
	// PartMedia carries bounded media bytes by value (channel tier 2,
	// VIVY-CHANNEL-PACK.md §1 Decision Record 2026-09-15). The Host
	// re-validates every part against the shared attachment limits and
	// rejects oversize or non-image content; an adapter that downloads
	// platform media must apply the same bound before publishing.
	PartMedia PartKind = "media"
)

// Media is one bounded by-value media payload on a Part (inbound photos
// today). Name is display-only and host-sanitized; MimeType is the
// adapter's claim, verified by the Host's content sniff; Data carries the
// bytes.
type Media struct {
	Name     string
	MimeType string
	Data     []byte
}

type Part struct {
	Kind           PartKind
	Text, MediaRef string
	Structured     json.RawMessage
	Media          Media
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

// ErrorClass is the minimal error-classification vocabulary of the §8
// Reliability row (CH-R-1). It answers "why is this ear degraded" with one
// word so the Host inspect surface and the Settings UI can show it without
// parsing adapter-specific error strings.
type ErrorClass string

const (
	// ClassRateLimit: the platform is throttling this bot (429s, quota
	// windows). Recovery is expected once the throttle lifts; the adapter
	// keeps retrying with backoff.
	ClassRateLimit ErrorClass = "rate-limit"
	// ClassTemporary: a transient transport or connectivity failure
	// (dropped socket, refused dial, handshake timeout). The supervised
	// redial loop is expected to recover on its own.
	ClassTemporary ErrorClass = "temporary"
	// ClassDead: the ear cannot recover without operator action — revoked
	// credentials, a delisted or banned bot, a gateway that permanently
	// refuses the session. Retrying only burns quota.
	ClassDead ErrorClass = "dead"
)

// HealthError classifies a Health report (CH-R-1). HealthChecker
// implementations return it to explain an unhealthy ear; the Host extracts
// the class with errors.As and surfaces it alongside the plain message. A
// Health error that is not a *HealthError is reported as ClassTemporary —
// the default assumption for a supervised, redialing ear.
type HealthError struct {
	Class ErrorClass
	Err   error
}

func (e *HealthError) Error() string {
	if e.Err == nil {
		return string(e.Class)
	}
	return e.Err.Error()
}

func (e *HealthError) Unwrap() error { return e.Err }

// HealthChecker reports the adapter's live transport state. The contract is
// deliberately narrow: Health reads internal state only — it never performs
// network I/O and must return promptly — so the Host may call it inline
// while building the inspect surface. A nil error means healthy.
type HealthChecker interface{ Health(context.Context) error }

type TaskLifecycle interface{}
type PipeServer interface{}
