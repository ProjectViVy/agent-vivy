// Package vivytoolkitsui owns the removable, backend-driven Toolkits page.
// It contributes a UI extension only; all installation and authorization
// authority remains behind the compiled vivy/toolkits control actions.
package vivytoolkitsui

import (
	"context"

	"agent-vivy/sdk/module"
)

const (
	ModuleID        = "vivy/toolkits-ui"
	ProviderID      = "vivy.toolkits-ui.sidebar"
	UIExtensionPort = "std/ui-extension@v1"
	SourceRef       = "repo:plugins/vivy-toolkits-ui"
	SourceSHA256    = "00a277212d96789aee4fd8ba3907e9c4189a7516e648a4ef6221bae180c2e29f"
)

type owner struct{}
type instance struct{}
type provider struct{}

func New() module.Module    { return owner{} }
func NewProvider() provider { return provider{} }
func (owner) Descriptor() module.Descriptor {
	return module.Descriptor{APIVersion: module.APIVersionV1, Module: module.Identity{ID: ModuleID, Version: "0.1.0"}, Source: module.Source{Ref: SourceRef, SHA256: SourceSHA256}, Provides: []module.PortRef{{Port: UIExtensionPort, ID: ProviderID}}, I18N: &module.I18N{Catalog: "i18n/catalog.json", DefaultLocale: "en", Locales: []string{"en", "zh"}}, Lifecycle: module.Lifecycle{Scope: module.ScopeGeneration}}
}
func (owner) Construct(context.Context, module.Host) (module.Instance, error) { return instance{}, nil }
func (instance) Start(context.Context) error                                  { return nil }
func (instance) Ready(context.Context) error                                  { return nil }
func (instance) Stop(context.Context) error                                   { return nil }
func (instance) Close(context.Context) error                                  { return nil }
