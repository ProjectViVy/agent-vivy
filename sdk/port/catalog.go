// Package port owns the v1 public Port catalog and build-evidence support
// states. Closed core Port definitions live in internal/moduleport; public
// Modules may reference core consumers but cannot enumerate or implement them
// through this package.
package port

import "agent-vivy/sdk/module"

type Visibility string

const (
	VisibilityPublic Visibility = "public"
)

type Cardinality string

const (
	CardinalityMany      Cardinality = "0..n"
	CardinalityExclusive Cardinality = "0..1"
)

type Definition struct {
	Ref           module.PortRef
	Visibility    Visibility
	Cardinality   Cardinality
	Consumer      module.PortRef
	Ordered       bool
	AllowedGrants []module.Grant
}

type Catalog struct {
	definitions []Definition
}

// PublicCatalog returns the sole ordered list of selectable public v1 Ports.
func PublicCatalog() Catalog {
	return Catalog{definitions: []Definition{
		public("std/tool@v1", CardinalityMany, "core/tool-host@v1", false, toolGrants()...),
		public("std/tool-world@v1", CardinalityMany, "core/tool-host@v1", false, toolGrants()...),
		public("std/channel@v1", CardinalityMany, "core/channel-host@v1", false,
			module.GrantChannelPoll, module.GrantChannelWebhook, module.GrantChannelListen,
			module.GrantChannelA2A, module.GrantSecretRead, module.GrantNetClient),
		public("std/face@v1", CardinalityExclusive, "core/face-host@v1", false, module.GrantRPCClient),
		public("std/provider-profile@v1", CardinalityMany, "core/chat-model-host@v1", false),
		public("std/context-source@v1", CardinalityMany, "core/context-host@v1", false,
			module.GrantFSRead, module.GrantSecretRead, module.GrantRPCClient, module.GrantNetClient),
		public("std/skill-source@v1", CardinalityMany, "core/skill-host@v1", false,
			module.GrantFSRead, module.GrantRPCClient, module.GrantNetClient),
		public("std/middleware/pre-tool@v1", CardinalityMany, "core/tool-host@v1", true),
		public("std/observer/run@v1", CardinalityMany, "core/observer-host@v1", false,
			module.GrantFSWrite, module.GrantRPCClient, module.GrantNetClient),
		public("std/observer/diagnostic@v1", CardinalityMany, "core/observer-host@v1", false,
			module.GrantFSWrite, module.GrantRPCClient, module.GrantNetClient),
		public("std/status-source@v1", CardinalityMany, "core/status-host@v1", false, module.GrantRPCClient),
		public("std/ui-extension@v1", CardinalityMany, "core/presentation-host@v1", true),
		public("std/ui-root@v1", CardinalityExclusive, "core/presentation-host@v1", false),
		public("std/control-action@v1", CardinalityMany, "core/action-host@v1", false, module.GrantRPCClient),
	}}
}

func public(name string, cardinality Cardinality, consumer string, ordered bool, allowedGrants ...module.Grant) Definition {
	return Definition{
		Ref:           module.PortRef{Port: name},
		Visibility:    VisibilityPublic,
		Cardinality:   cardinality,
		Consumer:      module.PortRef{Port: consumer},
		Ordered:       ordered,
		AllowedGrants: append([]module.Grant(nil), allowedGrants...),
	}
}

func toolGrants() []module.Grant {
	return []module.Grant{
		module.GrantFSRead, module.GrantFSWrite, module.GrantSecretRead,
		module.GrantProcSpawn, module.GrantTTY, module.GrantArgv,
		module.GrantRPCClient, module.GrantNetClient,
	}
}

func (catalog Catalog) Definitions() []Definition {
	definitions := append([]Definition(nil), catalog.definitions...)
	for index := range definitions {
		definitions[index].AllowedGrants = append([]module.Grant(nil), definitions[index].AllowedGrants...)
	}
	return definitions
}

func (catalog Catalog) Lookup(ref module.PortRef) (Definition, bool) {
	for _, definition := range catalog.definitions {
		if definition.Ref.Port == ref.Port {
			definition.AllowedGrants = append([]module.Grant(nil), definition.AllowedGrants...)
			return definition, true
		}
	}
	return Definition{}, false
}
