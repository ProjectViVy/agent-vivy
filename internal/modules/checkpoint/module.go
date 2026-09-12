// Package checkpoint composes the versioned checkpoint store while keeping
// Eino's CheckPointStore adapter inside the runtime quarantine.
package checkpoint

import (
	"context"

	"agent-vivy/internal/runtime"
	"agent-vivy/internal/storage"
	"agent-vivy/sdk/module"
)

const (
	ID   = "vivy/checkpoint"
	Port = "core/checkpoint-store@v1"
)

type Provider struct {
	store *runtime.VersionedCheckpointStore
}

func Compose(blobs storage.BlobStore, engineVersion string) (*Provider, error) {
	store, err := runtime.NewVersionedCheckpointStore(blobs, engineVersion)
	if err != nil {
		return nil, err
	}
	return &Provider{store: store}, nil
}

func (provider *Provider) Store() *runtime.VersionedCheckpointStore {
	if provider == nil {
		return nil
	}
	return provider.store
}

func NewModule() module.Module { return ownerModule{} }

type ownerModule struct{}

func (ownerModule) Descriptor() module.Descriptor {
	return module.Descriptor{
		APIVersion: module.APIVersionV1,
		Module:     module.Identity{ID: ID, Version: "1.0.0"},
		Source:     module.Source{Ref: "file:internal", SHA256: zeroDigest},
		Provides:   []module.PortRef{{Port: Port, ID: "vivy.checkpoint-store"}},
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
