// Package minimalchat owns the minimal browser UI. Execution remains in the host.
package minimalchat

import (
	"context"

	"agent-vivy/sdk/module"
)

func New() module.Module    { return owner{} }
func NewProvider() provider { return provider{} }

type owner struct{}
type instance struct{}
type provider struct{}

func (owner) Descriptor() module.Descriptor {
	return module.Descriptor{
		APIVersion: module.APIVersionV1,
		Module:     module.Identity{ID: "vivy/minimal-chat", Version: "0.1.0"},
		Source:     module.Source{Ref: "repo:faces/minimal-chat"},
		Provides:   []module.PortRef{{Port: "std/ui-root@v1", ID: "vivy.minimal-chat.root"}},
		Requires:   []module.Requirement{{PortRef: module.PortRef{Port: "core/presentation-host@v1"}, Provider: "vivy/presentation-host"}},
		I18N:       &module.I18N{Catalog: "i18n/catalog.json", DefaultLocale: "en", Locales: []string{"en", "zh"}},
		Lifecycle:  module.Lifecycle{Scope: module.ScopeGeneration},
	}
}

func (owner) Construct(context.Context, module.Host) (module.Instance, error) { return instance{}, nil }
func (instance) Start(context.Context) error                                  { return nil }
func (instance) Ready(context.Context) error                                  { return nil }
func (instance) Stop(context.Context) error                                   { return nil }
func (instance) Close(context.Context) error                                  { return nil }
