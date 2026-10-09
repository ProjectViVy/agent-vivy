// Package vivyworkflow owns the INOFY workflow editor UI Module.
//
// The Module contributes only a std/ui-extension@v1 Provider: its page hosts
// the vendored INOFY editor over the host's session-scoped `inofy.*` action
// surface, so authorization, journal authority, and run governance stay in
// the backend. It deliberately provides no backend Port.
package vivyworkflow

import (
	"context"

	"agent-vivy/sdk/module"
)

const (
	ModuleID        = "vivy/workflow-ui"
	ProviderID      = "vivy.workflow-ui.sidebar"
	UIExtensionPort = "std/ui-extension@v1"
	SourceRef       = "repo:plugins/vivy-workflow"
	SourceSHA256    = "e98c354a4d2522cd3b99290553303d89badd1731d1fc2217a1a51c1922e6c7d2"
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
