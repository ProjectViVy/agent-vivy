package headless

import (
	"context"

	"agent-vivy/sdk/module"
	faceport "agent-vivy/sdk/port/face"
)

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
	return module.Descriptor{APIVersion: module.APIVersionV1, Module: module.Identity{ID: "vivy/headless", Version: "0.1.0"}, Source: module.Source{Ref: "repo:faces/headless", SHA256: "0c1a976e1753e628d4da679c3eae6c755d6dba49d437264295177a1267f0a7fc"}, Provides: []module.PortRef{{Port: "std/face@v1", ID: "vivy.headless"}}, Requires: []module.Requirement{{PortRef: module.PortRef{Port: "core/face-host@v1"}, Provider: "vivy/face-host"}}, RequestedGrants: []module.Grant{module.GrantRPCClient}, Lifecycle: module.Lifecycle{Scope: module.ScopeGeneration}}
}

type provider struct{}

func NewProvider() faceport.FaceProvider { return provider{} }
func (provider) Definition() faceport.Definition {
	return faceport.Definition{ID: "vivy.headless", Kind: FaceKind}
}
func (provider) Construct(_ context.Context, h faceport.Host) (faceport.Instance, error) {
	return boundFace{host: h}, nil
}

type boundFace struct{ host faceport.Host }

func (f boundFace) Run(ctx context.Context, o faceport.Options) (faceport.Result, error) {
	return newRunner(o).Run(ctx, f.host)
}
