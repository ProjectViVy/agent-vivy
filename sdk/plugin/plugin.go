// Package plugin is the only import window a user plugin may use.
// It must not grow Journal, Policy, Eino, or pack APIs.
package plugin

import (
	"context"
	"encoding/json"
	"errors"
	"io"
)

type Seam string

const (
	SeamTool      Seam = "tool"
	SeamToolWorld Seam = "tool-world"
	SeamProvider  Seam = "provider"
	// SeamChannel marks a messaging-channel plugin. Its Consumer is the
	// kernel ChannelHost (VIVY-CHANNEL-PACK.md), never the model tool
	// table: channel plugins are never Adapt-ed into tools.
	SeamChannel Seam = "channel"
)

func (s Seam) Valid() bool {
	switch s {
	case SeamTool, SeamToolWorld, SeamProvider, SeamChannel:
		return true
	default:
		return false
	}
}

type Grant string

const (
	GrantFSRead  Grant = "fs.read"
	GrantFSWrite Grant = "fs.write"
	// GrantChannelPoll allows Start to run an outbound long-poll or
	// outbound websocket client (the only channel exception to the
	// "no long-running background service" rule). Listen stays with
	// the kernel ChannelHost.
	GrantChannelPoll Grant = "channel.poll"
	// GrantChannelWebhook declares a webhook path only; the socket
	// itself is owned and opened by the kernel ChannelHost.
	GrantChannelWebhook Grant = "channel.webhook"
	// GrantChannelListen is reserved for heavy channels (e.g. NeuroLink);
	// Listen remains a ChannelHost capability even under this grant.
	GrantChannelListen Grant = "channel.listen"
	// GrantChannelA2A is reserved for the deferred A2A transport.
	GrantChannelA2A Grant = "channel.a2a"
	// GrantSecretRead allows reading the values behind configured env_key
	// names through ChannelEnv.Secret. Values are fail-closed and never
	// logged.
	GrantSecretRead Grant = "secret.read"
)

// Valid reports whether the grant is part of the known vocabulary. Whether
// a grant is available to a given seam is not decided here — that
// restriction is enforced by the SDK verifier per batch.
func (g Grant) Valid() bool {
	switch g {
	case GrantFSRead, GrantFSWrite,
		GrantChannelPoll, GrantChannelWebhook, GrantChannelListen, GrantChannelA2A,
		GrantSecretRead:
		return true
	default:
		return false
	}
}

type Effect string

const (
	EffectRead  Effect = "read"
	EffectWrite Effect = "write"
)

func (e Effect) Valid() bool {
	switch e {
	case EffectRead, EffectWrite:
		return true
	default:
		return false
	}
}

// Plugin is one user capability source package.
type Plugin interface {
	Name() string
	Seam() Seam
	Grants() []Grant
	Tools() []Tool
}

// Tool is one model-visible function owned by a plugin.
type Tool interface {
	Name() string
	Effect() Effect
	Schema() json.RawMessage
	Run(ctx context.Context, env Env, args json.RawMessage) (string, error)
}

// Env is the only world a plugin may touch. Missing grants fail closed.
type Env interface {
	Workspace() string
	OpenRead(path string) (io.ReadCloser, error)
	OpenWrite(path string) (io.WriteCloser, error)
}

var (
	ErrDenied      = errors.New("plugin: grant denied")
	ErrInvalidArgs = errors.New("plugin: invalid args")
)
