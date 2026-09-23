// Package vivymasksui owns the removable backend-driven mask UI Module.
//
// The UI calls the separately selected vivy/masks action owner through the
// authenticated Module Action Host. This Module deliberately provides no
// backend Port and remains selectable independently of the optional service.
package vivymasksui

import (
	"context"

	"agent-vivy/sdk/module"
)

const (
	ModuleID        = "vivy/masks-ui"
	ProviderID      = "vivy.masks-ui.sidebar"
	UIExtensionPort = "std/ui-extension@v1"
	SourceRef       = "repo:plugins/vivy-masks-ui"
	SourceSHA256    = "0275a4a482ee5e8c93dca22d0c09ff1143a49fc8b677c3c8212c448d80365711"
)

type owner struct{}
type instance struct{}

type provider struct{}

func New() module.Module    { return owner{} }
func NewProvider() provider { return provider{} }

func (owner) Descriptor() module.Descriptor {
	return module.Descriptor{
		APIVersion: module.APIVersionV1,
		Module:     module.Identity{ID: ModuleID, Version: "0.1.0"},
		Source:     module.Source{Ref: SourceRef, SHA256: SourceSHA256},
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
