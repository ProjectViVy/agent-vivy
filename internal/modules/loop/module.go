// Package loop composes the single build-selected LoopDriver without moving
// Eino types outside the runtime quarantine.
package loop

import (
	"context"
	"errors"

	"agent-vivy/internal/runtime"
	"agent-vivy/internal/tools"
	"agent-vivy/sdk/module"
)

const (
	ID   = "vivy/loop"
	Port = "core/loop-driver@v1"
)

// EngineFactory is the Eino-free construction seam consumed by the module.
// runtime.EngineFactory is its production implementation.
type EngineFactory interface {
	Build(context.Context, []tools.Tool, runtime.EngineConfig) (*runtime.Engine, error)
}

// Driver is the sole LoopDriver selected for a Generation.
type Driver struct{ factory EngineFactory }

func Compose(factory EngineFactory) (*Driver, error) {
	if factory == nil {
		return nil, errors.New("loop module: EngineFactory is required")
	}
	return &Driver{factory: factory}, nil
}

func (driver *Driver) Build(ctx context.Context, active []tools.Tool, cfg runtime.EngineConfig) (*runtime.Engine, error) {
	if driver == nil || driver.factory == nil {
		return nil, errors.New("loop module: LoopDriver is not composed")
	}
	return driver.factory.Build(ctx, active, cfg)
}

// NewModule constructs the Generation-scoped lifecycle owner used by generated
// Assembly. Runtime dependencies are composed later at the app boundary.
func NewModule() module.Module { return ownerModule{} }

type ownerModule struct{}

func (ownerModule) Descriptor() module.Descriptor {
	return module.Descriptor{
		APIVersion: module.APIVersionV1,
		Module:     module.Identity{ID: ID, Version: "1.0.0"},
		Source:     module.Source{Ref: "file:internal", SHA256: zeroDigest},
		Provides:   []module.PortRef{{Port: Port, ID: "vivy.loop-driver"}},
		Lifecycle:  module.Lifecycle{Scope: module.ScopeGeneration},
	}
}

func (ownerModule) Construct(context.Context, module.Host) (module.Instance, error) {
	return ownerInstance{}, nil
}

type ownerInstance struct{}

func (ownerInstance) Start(context.Context) error { return nil }
func (ownerInstance) Ready(context.Context) error { return nil }
func (ownerInstance) Stop(context.Context) error  { return nil }
func (ownerInstance) Close(context.Context) error { return nil }

const zeroDigest = "0000000000000000000000000000000000000000000000000000000000000000"
