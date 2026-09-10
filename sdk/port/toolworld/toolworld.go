// Package toolworld defines the focused std/tool-world@v1 Provider contract.
package toolworld

import (
	"context"
	"encoding/json"
	"errors"
	"io"

	"agent-vivy/sdk/module"
)

var (
	ErrDenied      = errors.New("toolworld capability denied")
	ErrInvalidArgs = errors.New("toolworld invalid arguments")
)

type Effect string

const (
	EffectRead  Effect = "read"
	EffectWrite Effect = "write"
)

type Definition struct{ ID, Description string }
type ToolDefinition struct {
	ID, Description string
	Effect          Effect
	Schema          json.RawMessage
}
type Result struct{ Text string }

type Provider interface {
	Definition() Definition
	Discover(context.Context, Host) ([]ToolDefinition, error)
	Invoke(context.Context, Host, string, json.RawMessage) (Result, error)
	Close(context.Context) error
}
type LanguageServerStatus struct{ Language, State string }
type LanguageServerStatusProvider interface {
	LanguageServerStatuses(context.Context, string) []LanguageServerStatus
}
type DiagnosticObserver interface {
	ObserveWrite(context.Context, Host, []string) []string
}

type SpawnSpec struct {
	Command string
	Args    []string
}
type Proc interface {
	Stdin() io.WriteCloser
	Stdout() io.ReadCloser
	Stderr() io.ReadCloser
	Wait() error
	Close() error
}
type Host interface {
	module.Host
	Workspace() string
	OpenRead(string) (io.ReadCloser, error)
	OpenWrite(string) (io.WriteCloser, error)
	Spawn(context.Context, SpawnSpec) (Proc, error)
}
