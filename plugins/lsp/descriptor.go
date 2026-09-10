package lsp

import "agent-vivy/sdk/module"

func (vivyModule) Descriptor() module.Descriptor {
	return module.Descriptor{APIVersion: module.APIVersionV1, Module: module.Identity{ID: "vivy/lsp", Version: "0.1.0"}, Source: module.Source{Ref: "repo:plugins/lsp", SHA256: "96dac7da535dbbc1557b3ec83c5b4c46c1449b38e9d9e957a4cd22fa583991d3"}, Provides: []module.PortRef{{Port: "std/tool-world@v1", ID: "vivy.lsp"}}, Requires: []module.Requirement{{PortRef: module.PortRef{Port: "core/tool-host@v1"}, Provider: "vivy/tool-host"}}, RequestedGrants: []module.Grant{module.GrantFSRead, module.GrantFSWrite, module.GrantProcSpawn}, Lifecycle: module.Lifecycle{Scope: module.ScopeGeneration}}
}
