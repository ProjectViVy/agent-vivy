// Package sandbox composes the existing governed workspace, filesystem, and
// command backends. Final Policy and approval decisions remain in runtime.
package sandbox

import (
	"context"
	"fmt"
	"time"

	"agent-vivy/internal/config"
	"agent-vivy/internal/domain"
	"agent-vivy/internal/runtime"
	"agent-vivy/sdk/module"
)

const (
	ID   = "vivy/sandbox"
	Port = "core/sandbox-backend@v1"
)

type Provider struct {
	workspaces *runtime.WorkspaceManager
	sandbox    *runtime.SandboxManager
	filesystem *runtime.EinoFilesystemBackend
	commands   *runtime.CommandBackend
}

func Compose(cfg config.Config) (*Provider, error) {
	provider := &Provider{}
	if cfg.Runtime.WorkspaceRoot == "" {
		return provider, nil
	}
	var (
		manager *runtime.WorkspaceManager
		err     error
	)
	if cfg.Runtime.World == "local" {
		manager, err = runtime.NewLocalWorkspaceManager(cfg.Runtime.WorkspaceRoot)
	} else {
		manager, err = runtime.NewWorkspaceManager(cfg.Runtime.WorkspaceRoot)
	}
	if err != nil {
		return nil, fmt.Errorf("sandbox module: build workspace isolation: %w", err)
	}
	mode := domain.SandboxMode(cfg.Runtime.Sandbox.DefaultMode)
	if !mode.Valid() {
		mode = domain.SandboxModeWorkspaceWrite
	}
	root := cfg.Runtime.Sandbox.WorkspaceRoot
	if root == "" {
		root = cfg.Runtime.WorkspaceRoot
	}
	policy := &domain.NetworkPolicy{
		AllowedDomains: cfg.Runtime.Sandbox.Network.AllowedDomains,
		DenyPrivateIPs: cfg.Runtime.Sandbox.Network.DenyPrivateIPs,
	}
	governor, err := runtime.NewSandboxManager(mode, root, cfg.Runtime.ExecuteAllowedCommands, policy)
	if err != nil {
		return nil, fmt.Errorf("sandbox module: build sandbox manager: %w", err)
	}
	provider.workspaces = manager
	provider.sandbox = governor
	provider.filesystem = runtime.NewEinoFilesystemBackend(manager, governor)
	provider.commands = runtime.NewCommandBackend(manager, governor, cfg.Runtime.ExecuteAllowedCommands, time.Duration(cfg.Runtime.ExecuteMaxTimeoutSeconds)*time.Second)
	return provider, nil
}

func (provider *Provider) Enabled() bool { return provider != nil && provider.workspaces != nil }
func (provider *Provider) Workspaces() *runtime.WorkspaceManager {
	if provider == nil {
		return nil
	}
	return provider.workspaces
}
func (provider *Provider) Sandbox() *runtime.SandboxManager {
	if provider == nil {
		return nil
	}
	return provider.sandbox
}
func (provider *Provider) Filesystem() *runtime.EinoFilesystemBackend {
	if provider == nil {
		return nil
	}
	return provider.filesystem
}
func (provider *Provider) Commands() *runtime.CommandBackend {
	if provider == nil {
		return nil
	}
	return provider.commands
}

func NewModule() module.Module { return ownerModule{} }

type ownerModule struct{}

func (ownerModule) Descriptor() module.Descriptor {
	return module.Descriptor{
		APIVersion: module.APIVersionV1,
		Module:     module.Identity{ID: ID, Version: "1.0.0"},
		Source:     module.Source{Ref: "file:internal", SHA256: zeroDigest},
		Provides:   []module.PortRef{{Port: Port, ID: "vivy.sandbox-backend"}},
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
