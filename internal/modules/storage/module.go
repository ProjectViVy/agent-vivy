// Package storage composes the build-selected durable Storage Engine while
// leaving Journal schema and transaction authority in internal/storage.
package storage

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"agent-vivy/internal/config"
	storagecontract "agent-vivy/internal/storage"
	"agent-vivy/internal/storage/postgres"
	"agent-vivy/internal/storage/sqlite"
	"agent-vivy/sdk/module"
)

const (
	ID   = "vivy/storage"
	Port = "core/storage-engine@v1"
)

// Open selects one of the existing first-party engines. It does not wrap or
// reinterpret storage contracts, so atomic Journal and blob-generation rules
// remain owned by their existing implementations.
func Open(ctx context.Context, cfg config.Config) (storagecontract.Engine, error) {
	switch cfg.Storage.Backend {
	case "postgres":
		dsn := os.Getenv(cfg.Storage.Postgres.DSNEnv)
		if dsn == "" {
			return nil, fmt.Errorf("storage module: %s is empty; postgres DSN is read from the environment (D-010)", cfg.Storage.Postgres.DSNEnv)
		}
		backend, err := postgres.Open(ctx, dsn)
		if err != nil {
			return nil, fmt.Errorf("storage module: open postgres storage: %w", err)
		}
		return backend, nil
	default:
		if dir := filepath.Dir(cfg.Storage.SQLite.Path); dir != "" && dir != "." {
			if err := os.MkdirAll(dir, 0o700); err != nil {
				return nil, fmt.Errorf("storage module: create storage dir: %w", err)
			}
		}
		backend, err := sqlite.Open(ctx, cfg.Storage.SQLite.Path)
		if err != nil {
			return nil, fmt.Errorf("storage module: open storage: %w", err)
		}
		if err := backend.TakeOrganismLease(ctx); err != nil {
			_ = backend.Close()
			return nil, fmt.Errorf("storage module: occupy shared workspace: %w", err)
		}
		return backend, nil
	}
}

func NewModule() module.Module { return ownerModule{} }

type ownerModule struct{}

func (ownerModule) Descriptor() module.Descriptor {
	return module.Descriptor{
		APIVersion: module.APIVersionV1,
		Module:     module.Identity{ID: ID, Version: "1.0.0"},
		Source:     module.Source{Ref: "file:internal", SHA256: zeroDigest},
		Provides:   []module.PortRef{{Port: Port, ID: "vivy.storage-engine"}},
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
