package scxreference

import "agent-vivy/sdk/module"

func (vivyModule) Descriptor() module.Descriptor {
	return module.Descriptor{
		APIVersion: module.APIVersionV1,
		Module:     module.Identity{ID: "scx/reference-fixtures", Version: "0.1.0"},
		Source:     module.Source{Ref: "repo:plugins/scx-reference", SHA256: "3abef450f9dd9ccd7735e0a2d2db13421db53d4b0297bd9f50c1af3ce127d665"},
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
