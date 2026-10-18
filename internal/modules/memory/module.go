// Package memory owns the BML-backed memory module records: the
// control-action plane (vivy/memory-bml) and the sync plane
// (vivy/memory-bml-sync, one provider for context recall and run
// observation). The BML store is opened once at composition through Open and
// bound to generated App provider copies. Standalone legacy providers use
// the package-level Active registry; closed bindings report explicit
// unavailable/failed outcomes when no service is open.
package memory

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"

	"agent-vivy/internal/config"
	"agent-vivy/internal/modules/memory/catalog"
	"agent-vivy/sdk/module"
	"github.com/ProjectViVy/agent-vivy/bml"
)

const (
	ID         = catalog.ID
	SyncID     = catalog.SyncID
	ProviderID = catalog.ProviderID
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
var lifecycleMu sync.Mutex

// Open is the composition entry point, called once after
// storagemodule.Open: it warms the BML home under cfg.DataDirectory() and
// publishes the service for providers to resolve at call time. A same-root repeat returns the existing service. A foreign-root call fails
// without exposing or closing the active Home.
func Open(ctx context.Context, cfg config.Config) (*Service, error) { return openOwned(ctx, cfg, "") }

// OpenOwned binds the Home to the actual core storage lease, so two distinct
// core databases cannot accidentally share and close the same memory Home.
func OpenOwned(ctx context.Context, cfg config.Config, leaseOwner string) (*Service, error) {
	if leaseOwner == "" {
		return nil, fmt.Errorf("memory: core storage lease owner required")
	}
	return openOwned(ctx, cfg, leaseOwner)
}
func openOwned(ctx context.Context, cfg config.Config, leaseOwner string) (*Service, error) {
	root, err := memoryRoot(cfg.DataDirectory())
	if err != nil {
		return nil, err
	}
	lifecycleMu.Lock()
	defer lifecycleMu.Unlock()
	if service := active.Load(); service != nil {
		if service.home.ConfigDir() != root || service.leaseOwner != leaseOwner {
			return nil, fmt.Errorf("memory: active Home belongs to another data root or core lease")
		}
		return service, nil
	}
	home := bml.NewHome(root)
	if err := home.Warmup(ctx); err != nil {
		_ = home.Close()
		return nil, err
	}
	service := &Service{home: home, leaseOwner: leaseOwner}
	active.Store(service)
	return service, nil
}

// Resolve existing ancestors too, so a linked parent and its real path cannot
// acquire different identities merely because the lazy memory file is absent.
func memoryRoot(path string) (string, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	var suffix []string
	for current := absolute; ; current = filepath.Dir(current) {
		resolved, err := filepath.EvalSymlinks(current)
		if err == nil {
			for i := len(suffix) - 1; i >= 0; i-- {
				resolved = filepath.Join(resolved, suffix[i])
			}
			return resolved, nil
		}
		if !errors.Is(err, os.ErrNotExist) || filepath.Dir(current) == current {
			return "", err
		}
		suffix = append(suffix, filepath.Base(current))
	}
}

// Active returns the service published by Open, or nil when the module was
// never opened or was closed.
func Active() *Service { return active.Load() }

// Close detaches and closes the active service. It is safe to call more
// than once and when no service was ever opened.
func Close() error {
	lifecycleMu.Lock()
	defer lifecycleMu.Unlock()
	service := active.Swap(nil)
	if service == nil {
		return nil
	}
	return service.close()
}

// Close releases only this exact composition's Home. A stale or unselected
// App cannot detach a replacement service published in the same registry.
func (s *Service) Close() error {
	lifecycleMu.Lock()
	defer lifecycleMu.Unlock()
	if s == nil || !active.CompareAndSwap(s, nil) {
		return nil
	}
	return s.close()
}

const zeroDigest = "0000000000000000000000000000000000000000000000000000000000000000"
