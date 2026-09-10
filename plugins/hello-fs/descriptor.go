package hellofs

import "agent-vivy/sdk/module"

func (vivyModule) Descriptor() module.Descriptor {
	return module.Descriptor{APIVersion: module.APIVersionV1, Module: module.Identity{ID: "vivy/hello-fs", Version: "0.1.0"}, Source: module.Source{Ref: "repo:plugins/hello-fs", SHA256: "40439b91831bda45ff7fe7fab1ccfbb5a89c8a613d37bedb6878e35e8fdab41e"}, Provides: []module.PortRef{{Port: "std/tool-world@v1", ID: "vivy.hello-fs"}}, Requires: []module.Requirement{{PortRef: module.PortRef{Port: "core/tool-host@v1"}, Provider: "vivy/tool-host"}}, RequestedGrants: []module.Grant{module.GrantFSRead}, Lifecycle: module.Lifecycle{Scope: module.ScopeGeneration}}
}
