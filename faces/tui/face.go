// Package tui is the packable first-party terminal organ. The implementation
// lives in sdk/tui so every Vivy Code entry runs the same state machine.
package tui

import (
	"context"

	"agent-vivy/sdk/module"
	faceport "agent-vivy/sdk/port/face"
	tuiface "agent-vivy/sdk/tui/face"
)

const FaceKind = tuiface.Kind

// New is the seam-face constructor required by the packer.
type vivyModule struct{}

func New() module.Module { return vivyModule{} }
func (vivyModule) Construct(context.Context, module.Host) (module.Instance, error) {
	return moduleInstance{}, nil
}

type moduleInstance struct{}

func (moduleInstance) Start(context.Context) error { return nil }
func (moduleInstance) Ready(context.Context) error { return nil }
func (moduleInstance) Stop(context.Context) error  { return nil }
func (moduleInstance) Close(context.Context) error { return nil }
func (vivyModule) Descriptor() module.Descriptor {
	return module.Descriptor{APIVersion: module.APIVersionV1, Module: module.Identity{ID: "vivy/tui", Version: "0.1.0"}, Source: module.Source{Ref: "repo:faces/tui", SHA256: "1e9ff549d8b1ca4e6bcd76050e236d3be3c2f16a06e920dd5814ddeca397d464"}, Provides: []module.PortRef{{Port: "std/face@v1", ID: "vivy.tui"}}, Requires: []module.Requirement{{PortRef: module.PortRef{Port: "core/face-host@v1"}, Provider: "vivy/face-host"}}, RequestedGrants: []module.Grant{module.GrantRPCClient}, Lifecycle: module.Lifecycle{Scope: module.ScopeGeneration}}
}

type provider struct{}

func NewProvider() faceport.FaceProvider { return provider{} }
func (provider) Definition() faceport.Definition {
	return faceport.Definition{ID: "vivy.tui", Kind: FaceKind}
}
func (provider) Construct(_ context.Context, h faceport.Host) (faceport.Instance, error) {
	return boundFace{host: h}, nil
}

type boundFace struct{ host faceport.Host }

func (f boundFace) Run(ctx context.Context, o faceport.Options) (faceport.Result, error) {
	return tuiface.New(o).Run(ctx, f.host)
}
