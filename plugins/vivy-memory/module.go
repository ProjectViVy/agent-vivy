// Package vivymemory is the 记忆 (Memory) sidebar Module.
//
// It is a UI-only Module: it contributes one std/ui-extension@v1 Provider and
// no Go-side Port, so the Memory page and its sidebar entry exist in a
// Generation only while the Recipe selects this Module.
package vivymemory

import (
	"context"

	"agent-vivy/sdk/module"
)

const (
	// ModuleID is this Module's identity; it must match vivy-module.yaml.
	ModuleID = "vivy/memory"
	// ProviderID is the UI extension the web Face installs.
	ProviderID = "vivy.memory.sidebar"
	// UIExtensionPort is the only Port this Module provides.
	UIExtensionPort = "std/ui-extension@v1"
)

type owner struct{}
type instance struct{}

// New returns the Module owner.
func New() module.Module { return owner{} }

// NewProvider returns the Module's Go-side provider handle. A UI-only Module
// contributes no Go Port, so the generated Assembly never binds this value; it
// exists because every Module source exports the typed New/NewProvider pair.
func NewProvider() provider { return provider{} }

type provider struct{}

func (owner) Descriptor() module.Descriptor {
	return module.Descriptor{
		APIVersion: module.APIVersionV1,
		Module:     module.Identity{ID: ModuleID, Version: "0.1.0"},
		Source:     module.Source{Ref: "repo:plugins/vivy-memory", SHA256: "59b7e120eec30864a4c5501423678611d4504f4a24efaff2d65ef087fc6e9f41"},
		Provides:   []module.PortRef{{Port: UIExtensionPort, ID: ProviderID}},
		I18N:       &module.I18N{Catalog: "i18n/catalog.json", DefaultLocale: "en", Locales: []string{"en", "zh"}},
		Lifecycle:  module.Lifecycle{Scope: module.ScopeGeneration},
	}
}

func (owner) Construct(context.Context, module.Host) (module.Instance, error) { return instance{}, nil }
func (instance) Start(context.Context) error                                  { return nil }
func (instance) Ready(context.Context) error                                  { return nil }
func (instance) Stop(context.Context) error                                   { return nil }
func (instance) Close(context.Context) error                                  { return nil }
