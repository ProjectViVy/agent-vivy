// Package sessiontree is the Session Tree sidebar Module.
//
// It is a UI-only Module: it contributes one std/ui-extension@v1 Provider and
// no Go-side Port. The page renders the kernel's session/tree read model and
// drives the session/clone, session/import, session/export, and exports/read
// verbs through the face RPC plane, so the Module adds no second graph
// builder or file path of its own.
package sessiontree

import (
	"context"

	"agent-vivy/sdk/module"
)

const (
	// ModuleID is this Module's identity; it must match vivy-module.yaml.
	ModuleID = "vivy/session-tree"
	// ProviderID is the UI extension the web Face installs.
	ProviderID = "vivy.session-tree.page"
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
		Source:     module.Source{Ref: "repo:plugins/coding/session-tree", SHA256: "583d525d9188a108fac9d8b2fff8f3f0440bfa2300cdab2aae5ba6ded96c7fd0"},
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
