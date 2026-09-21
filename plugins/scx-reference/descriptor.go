package scxreference

import "agent-vivy/sdk/module"

func (vivyModule) Descriptor() module.Descriptor {
	return module.Descriptor{
		APIVersion: module.APIVersionV1,
		Module:     module.Identity{ID: "scx/reference-fixtures", Version: "0.1.0"},
		Source:     module.Source{Ref: "repo:plugins/scx-reference", SHA256: "5c39ce4d73b0e1cb47fb9a3bf6e317a32191830cea87ed554e83c3c9db1fdfe4"},
		Provides: []module.PortRef{
			{Port: "std/context-source@v1", ID: providerID},
			{Port: "std/observer/run@v1", ID: providerID},
		},
		Requires: []module.Requirement{
			{PortRef: module.PortRef{Port: "core/context-host@v1"}, Provider: "vivy/context-host"},
			{PortRef: module.PortRef{Port: "core/observer-host@v1"}, Provider: "vivy/observer-host"},
		},
		Lifecycle: module.Lifecycle{Scope: module.ScopeGeneration},
	}
}
