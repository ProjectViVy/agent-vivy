// Package notebook is the build-linked T1 Module owning the scoped,
// revisioned notebook content capability (N2). Construction is side-effect
// free: the sealed factory seam opens the single owner, which binds the N1
// narrow Store and trusted scope resolver — never a raw DB, runtime Service,
// or untyped locator.
package notebook

import (
	"context"

	"agent-vivy/sdk/module"
)

const (
	// ID is this Module's identity in the build-owned Source Catalog.
	ID = "vivy/notebook-core"
	// Port is the closed internal Port it provides.
	Port = "core/notebook-service@v1"
	// ProviderID is the sole provider identity under the Port.
	ProviderID = "vivy.notebook-service"
)

// ActionIDs is the sealed std/control-action@v1 inventory.
var ActionIDs = []string{
	ActionSectionsList,
	ActionSectionsCreate,
	ActionSectionsUpdate,
	ActionSectionsDelete,
	ActionSectionsRestore,
	ActionEntriesList,
	ActionEntriesGet,
	ActionEntriesCreate,
	ActionEntriesSave,
	ActionEntriesMove,
	ActionEntriesDelete,
	ActionEntriesRestore,
	ActionRevisionsList,
	ActionRevisionsAdopt,
	ActionCommentsList,
	ActionCommentsCreate,
	ActionCommentsUpdate,
	ActionExport,
}

// ToolIDs is the retained agent-facing tool inventory rebind through
// ToolHost when the module's tool selection is on (N2 Task 3).
var ToolIDs = []string{
	ToolListNotes,
	ToolReadNote,
	ToolWriteNote,
}

type owner struct{}
type instance struct{}

// NewModule returns the Module owner used by the generated Assembly.
func NewModule() module.Module { return owner{} }

// Descriptor declares the closed Port plus the action/tool inventories. The
// Requires edge is added by the Source Catalog for the tools record; this
// descriptor carries the factory + action surface.
func (owner) Descriptor() module.Descriptor {
	provides := []module.PortRef{{Port: Port, ID: ProviderID}}
	for _, action := range ActionIDs {
		provides = append(provides, module.PortRef{Port: "std/control-action@v1", ID: action})
	}
	return module.Descriptor{
		APIVersion: module.APIVersionV1,
		Module:     module.Identity{ID: ID, Version: "1.0.0"},
		Source:     module.Source{Ref: "file:internal"},
		Provides:   provides,
		Lifecycle:  module.Lifecycle{Scope: module.ScopeGeneration},
	}
}

// ToolsDescriptor describes the separate vivy/notebook-tools record: the
// note tools ride ToolHost and require core/tool-host@v1.
func ToolsDescriptor() module.Descriptor {
	provides := make([]module.PortRef, 0, len(ToolIDs))
	for _, tool := range ToolIDs {
		provides = append(provides, module.PortRef{Port: "std/tool@v1", ID: tool})
	}
	return module.Descriptor{
		APIVersion: module.APIVersionV1,
		Module:     module.Identity{ID: ToolsID, Version: "1.0.0"},
		Source:     module.Source{Ref: "file:internal"},
		Provides:   provides,
		Requires:   []module.Requirement{{PortRef: module.PortRef{Port: "core/tool-host@v1"}, Provider: "vivy/tool-host"}},
		Lifecycle:  module.Lifecycle{Scope: module.ScopeGeneration},
	}
}

// ToolsID is the companion record ID for the tool contribution.
const ToolsID = "vivy/notebook-tools"

func (owner) Construct(context.Context, module.Host) (module.Instance, error) { return instance{}, nil }
func (instance) Start(context.Context) error                                  { return nil }
func (instance) Ready(context.Context) error                                  { return nil }
func (instance) Stop(context.Context) error                                   { return nil }
func (instance) Close(context.Context) error                                  { return nil }
