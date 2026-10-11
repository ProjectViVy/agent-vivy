package toolkits

import (
	"context"

	"agent-vivy/sdk/module"
	controlaction "agent-vivy/sdk/port/controlaction"
)

const zeroDigest = "0000000000000000000000000000000000000000000000000000000000000000"

var actionIDs = []string{
	ActionList, ActionGet, ActionInspect, ActionPreview, ActionPreviewGit, ActionInstall,
	ActionPrepareUpdate, ActionPrepareUpdateGit, ActionEnable, ActionDisable, ActionUninstall,
	ActionRollback, ActionOperationGet, ActionOperationPublish, ActionOperationCancel, ActionComponentGet,
	ActionHookArm, ActionHookDisarm, ActionMCPAuthorize, ActionMCPRevoke, ActionBindingsSet,
	ActionSourceRebind, ActionSourceRebindGit,
}

func ControlActionIDs() []string { return append([]string(nil), actionIDs...) }

func NewModule() module.Module { return ownerModule{} }

type ownerModule struct{}
type ownerInstance struct{}

func (ownerModule) Descriptor() module.Descriptor {
	provides := make([]module.PortRef, 0, len(actionIDs))
	for _, id := range actionIDs {
		provides = append(provides, module.PortRef{Port: controlaction.Port, ID: id})
	}
	return module.Descriptor{APIVersion: module.APIVersionV1, Module: module.Identity{ID: OwnerID, Version: "1.0.0"}, Source: module.Source{Ref: "file:internal", SHA256: zeroDigest}, Provides: provides, Requires: []module.Requirement{
		{PortRef: module.PortRef{Port: "core/action-host@v1"}, Provider: "vivy/action-host"},
		{PortRef: module.PortRef{Port: "core/storage-engine@v1"}, Provider: "vivy/storage"},
	}, Lifecycle: module.Lifecycle{Scope: module.ScopeGeneration}}
}
func (ownerModule) Construct(context.Context, module.Host) (module.Instance, error) {
	return ownerInstance{}, nil
}
func (ownerInstance) Start(context.Context) error { return nil }
func (ownerInstance) Ready(context.Context) error { return nil }
func (ownerInstance) Stop(context.Context) error  { return nil }
func (ownerInstance) Close(context.Context) error { return nil }
