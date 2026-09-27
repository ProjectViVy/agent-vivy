// Package memory owns the BML-backed memory module records: the
// control-action plane (vivy/memory-bml) and the sync plane
// (vivy/memory-bml-sync, one provider for context recall and run
// observation). The BML store is opened once at composition through Open and
// shared with the generated providers through the package-level Active
// registry; providers resolve it at call time and report explicit
// unavailable/failed outcomes when no service is open.
package memory

import (
	"context"
	"sync/atomic"

	"agent-vivy/internal/config"
	"agent-vivy/sdk/module"
	"github.com/ProjectViVy/agent-vivy/bml"
)

const (
	// ID is the control-action module record.
	ID = "vivy/memory-bml"
	// SyncID is the sync-plane module record (context-source + run-observer).
	SyncID = "vivy/memory-bml-sync"
	// ProviderID is the single provider identity satisfying both the
	// ContextSources and RunObservers manifest lists.
	ProviderID = "vivy.memory.bml"
)

// NewModule is the shared Assembly lifecycle owner for both memory records.
// The sealed Provides/Requires live in the Source Catalog
// (internal/modules/defaults); this descriptor is not consulted by generated
// code, which calls Construct via the bound constructor.
func NewModule() module.Module { return ownerModule{} }

type ownerModule struct{}

func (ownerModule) Descriptor() module.Descriptor {
	return module.Descriptor{
		APIVersion: module.APIVersionV1,
		Module:     module.Identity{ID: ID, Version: "1.0.0"},
		Source:     module.Source{Ref: "file:internal", SHA256: zeroDigest},
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

var active atomic.Pointer[Service]

// Open is the composition entry point, called once after
// storagemodule.Open: it warms the BML home under cfg.DataDirectory() and
// publishes the service for providers to resolve at call time. A second
// call — or one racing the first — returns the existing service instead of
// orphaning the Home it already opened.
func Open(ctx context.Context, cfg config.Config) (*Service, error) {
	if service := active.Load(); service != nil {
		return service, nil
	}
	home := bml.NewHome(cfg.DataDirectory())
	if err := home.Warmup(ctx); err != nil {
		_ = home.Close()
		return nil, err
	}
	service := &Service{home: home}
	if !active.CompareAndSwap(nil, service) {
		_ = home.Close()
		return active.Load(), nil
	}
	return service, nil
}

// Active returns the service published by Open, or nil when the module was
// never opened or was closed.
func Active() *Service { return active.Load() }

// Close detaches and closes the active service. It is safe to call more
// than once and when no service was ever opened.
func Close() error {
	service := active.Swap(nil)
	if service == nil {
		return nil
	}
	return service.close()
}

const zeroDigest = "0000000000000000000000000000000000000000000000000000000000000000"
