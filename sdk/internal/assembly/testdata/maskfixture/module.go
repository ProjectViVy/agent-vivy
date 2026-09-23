// Package maskfixture is a test-only, typed mask-service binding. It gives
// generated Assembly tests a real importable Factory and lifecycle Module
// without adding a production masks.Open implementation.
package maskfixture

import (
	"context"

	"agent-vivy/internal/maskcontract"
	"agent-vivy/internal/moduleport"
	"agent-vivy/sdk/module"
)

// Factory is the typed selector emitted into a generated RuntimeAssembly.
// The fixture does not exercise service behavior; its purpose is to make the
// generated function reference compile against the exact production seam.
func Factory(context.Context, moduleport.MaskDependencies) (maskcontract.Service, error) {
	return nil, nil
}

var _ moduleport.MaskFactory = Factory

// NewModule supplies the lifecycle owner required by generated Assembly code.
func NewModule() module.Module { return moduleDefinition{} }

type moduleDefinition struct{}

func (moduleDefinition) Descriptor() module.Descriptor {
	return module.Descriptor{
		APIVersion: module.APIVersionV1,
		Module:     module.Identity{ID: "vivy/masks", Version: "test"},
		Provides:   []module.PortRef{{Port: "core/mask-service@v1", ID: "vivy.mask-service"}},
		Lifecycle:  module.Lifecycle{Scope: module.ScopeGeneration},
	}
}

func (moduleDefinition) Construct(context.Context, module.Host) (module.Instance, error) {
	return lifecycleInstance{}, nil
}

type lifecycleInstance struct{}

func (lifecycleInstance) Start(context.Context) error { return nil }
func (lifecycleInstance) Ready(context.Context) error { return nil }
func (lifecycleInstance) Stop(context.Context) error  { return nil }
func (lifecycleInstance) Close(context.Context) error { return nil }
