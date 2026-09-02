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
	// SeamFace marks a face organ (VIVY-FACE-PACK.md §6): a mouth of the
	// species body hosted by the kernel FaceHost. A face is a
	// control-plane client — it is never a model tool.
	SeamFace Seam = "face"
)

func (s Seam) Valid() bool {
	switch s {
	case SeamTool, SeamToolWorld, SeamProvider, SeamChannel, SeamFace:
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
	// GrantProcSpawn allows starting child processes through Env.Spawn.
	// The child's working directory is pinned to the plugin workspace; the
	// spawned process outlives the tool call that started it and remains
	// the plugin's responsibility until Close. os/exec stays banned in
	// plugin sources — spawn is a kernel-hosted capability exactly like
	// Listen is a ChannelHost capability (VC-3, D4).
	GrantProcSpawn Grant = "proc.spawn"
	// Face-family grants (VIVY-FACE-PACK.md §6). A face organ is a
	// control-plane client, not a model tool: tty is the right to draw to
	// and read from the terminal, argv the right to read its command-line
	// arguments, rpc.client the right to call the kernel control plane
	// through FaceEnv.Call. No other grant is meaningful to a face.
	GrantTTY       Grant = "tty"
	GrantArgv      Grant = "argv"
	GrantRPCClient Grant = "rpc.client"
)

// Valid reports whether the grant is part of the known vocabulary. Whether
// a grant is available to a given seam is not decided here — that
// restriction is enforced by the SDK verifier per batch.
func (g Grant) Valid() bool {
	switch g {
	case GrantFSRead, GrantFSWrite,
		GrantChannelPoll, GrantChannelWebhook, GrantChannelListen, GrantChannelA2A,
		GrantSecretRead, GrantProcSpawn,
		GrantTTY, GrantArgv, GrantRPCClient:
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

// DiagnosticObserver is an optional tool-world capability (VC-3 backfill).
// After the kernel's file mutation tools (write_file/patch/multiedit)
// change a file, the kernel asks every tool-world plugin implementing this
// interface for diagnostics on the touched paths and attaches the lines to
// the mutation result, so lint/type errors reach the model without a
// separate call. Lines are pre-formatted text; return nil when there is
// nothing to report. Implementations must respect ctx cancellation, use
// only the granted Env, and keep the output bounded — the kernel decides
// what it forwards.
type DiagnosticObserver interface {
	Plugin
	ObserveWrite(ctx context.Context, env Env, paths []string) []string
}

// SpawnSpec names one child process. Command is either a bare executable
// name (resolved through PATH) or a workspace-relative path; absolute
// paths and workspace escapes fail closed. The child runs with its working
// directory pinned to the plugin workspace and inherits the species
// environment.
type SpawnSpec struct {
	Command string
	Args    []string
}

// Proc is one spawned child process. Stdout/stderr are blocking readers;
// Wait blocks until exit and Close kills it.
type Proc interface {
	Stdin() io.WriteCloser
	Stdout() io.ReadCloser
	Stderr() io.ReadCloser
	Wait() error
	Close() error
}

// Env is the only world a plugin may touch. Missing grants fail closed.
type Env interface {
	Workspace() string
	OpenRead(path string) (io.ReadCloser, error)
	OpenWrite(path string) (io.WriteCloser, error)
	Spawn(ctx context.Context, spec SpawnSpec) (Proc, error)
}

var (
	ErrDenied      = errors.New("plugin: grant denied")
	ErrInvalidArgs = errors.New("plugin: invalid args")
)
