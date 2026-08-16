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
)

func (s Seam) Valid() bool {
	switch s {
	case SeamTool, SeamToolWorld, SeamProvider:
		return true
	default:
		return false
	}
}

type Grant string

const (
	GrantFSRead  Grant = "fs.read"
	GrantFSWrite Grant = "fs.write"
)

func (g Grant) Valid() bool {
	switch g {
	case GrantFSRead, GrantFSWrite:
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
