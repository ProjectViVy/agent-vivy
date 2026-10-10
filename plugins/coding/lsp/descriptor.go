package lsp

import "agent-vivy/sdk/module"

func (vivyModule) Descriptor() module.Descriptor {
	return module.Descriptor{APIVersion: module.APIVersionV1, Module: module.Identity{ID: "vivy/lsp", Version: "0.1.0"}, Source: module.Source{Ref: "repo:plugins/coding/lsp", SHA256: "6220b77f88856be028015925972c6fe7adf751895ff4c8c32894bd43f818067b"}, Provides: []module.PortRef{{Port: "std/tool-world@v1", ID: "vivy.lsp"}}, Requires: []module.Requirement{{PortRef: module.PortRef{Port: "core/tool-host@v1"}, Provider: "vivy/tool-host"}}, RequestedGrants: []module.Grant{module.GrantFSRead, module.GrantFSWrite, module.GrantProcSpawn}, Lifecycle: module.Lifecycle{Scope: module.ScopeGeneration}}
}
