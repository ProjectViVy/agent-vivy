// Package face defines the focused std/face@v1 Provider contract.
package face

import (
	"context"
	"encoding/json"
	"io"

	"agent-vivy/sdk/module"
)

type Definition struct{ ID, Kind string }
type FaceProvider interface {
	Definition() Definition
	Construct(context.Context, Host) (Instance, error)
}
type Instance interface {
	Run(context.Context, Options) (Result, error)
}

// Runner is the transport-facing implementation shape used inside a Provider.
// It is not a Module entry point; generated assemblies construct Providers.
type Runner interface {
	Kind() string
	Run(context.Context, Host) (Result, error)
}
type Face = Runner
type FaceOptions = Options
type FaceResult = Result
type FaceEnv = Host
type FaceConstructor func(Options) Runner
type Options struct {
	Prompt                          string
	ContinueNewest, DebugToolOutput bool
	Out, Err                        io.Writer
}
type Result struct{ Status string }
type Host interface {
	module.Host
	Call(context.Context, string, any) (json.RawMessage, error)
	OnEvent(func(string, json.RawMessage))
}
