// Package model composes the single build-selected ModelHost over declarative
// Provider Profiles. Executable provider adapters remain quarantined in
// internal/provider.
package model

import (
	"context"

	"agent-vivy/internal/modelhost"
	"agent-vivy/sdk/module"
	"agent-vivy/sdk/port/providerprofile"
)

const (
	ID   = "vivy/model"
	Port = "core/chat-model-host@v1"
)

type Provider struct{ host *modelhost.Host }

func Compose(profiles []providerprofile.Profile, capabilities modelhost.Capabilities) (*Provider, error) {
	host, err := modelhost.New(profiles, capabilities)
	if err != nil {
		return nil, err
	}
	return &Provider{host: host}, nil
}

func (provider *Provider) Host() *modelhost.Host {
	if provider == nil {
		return nil
	}
	return provider.host
}

func NewModule() module.Module { return ownerModule{} }

type ownerModule struct{}

func (ownerModule) Descriptor() module.Descriptor {
	return module.Descriptor{
		APIVersion: module.APIVersionV1,
		Module:     module.Identity{ID: ID, Version: "1.0.0"},
		Source:     module.Source{Ref: "file:internal", SHA256: zeroDigest},
		Provides:   []module.PortRef{{Port: Port, ID: "vivy.chat-model-host"}},
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
