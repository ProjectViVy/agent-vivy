// Package masks owns the build-linked lifecycle descriptor, typed service
// factory, and compiler-bound control actions for the optional mask service.
// Construction remains side-effect free; Core Storage is injected by App.
package masks

import (
	"context"

	"agent-vivy/sdk/module"
	controlaction "agent-vivy/sdk/port/controlaction"
)

const (
	ID   = "vivy/masks"
	Port = "core/mask-service@v1"
)

// NewModule is the pure Assembly lifecycle owner. It is deliberately
// independent from the optional UI contribution and from Core Storage.
func NewModule() module.Module { return ownerModule{} }

type ownerModule struct{}

func (ownerModule) Descriptor() module.Descriptor {
	provides := []module.PortRef{{Port: Port, ID: "vivy.mask-service"}}
	for _, id := range []string{
		ActionCatalogList,
		ActionCatalogGet,
		ActionCatalogCreate,
		ActionCatalogUpdate,
		ActionCatalogDelete,
		ActionSelectionGet,
		ActionSelectionSet,
	} {
		provides = append(provides, module.PortRef{Port: controlaction.Port, ID: id})
	}
	return module.Descriptor{
		APIVersion: module.APIVersionV1,
		Module:     module.Identity{ID: ID, Version: "1.0.0"},
		Source:     module.Source{Ref: "file:internal", SHA256: zeroDigest},
		Provides:   provides,
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
