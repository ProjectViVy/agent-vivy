// Package moduleport owns Vivy's closed internal Port catalog.
//
// The package deliberately lives under internal/: public Modules may name a
// core Port as a requirement, but they cannot implement, enumerate, or extend
// these build-owned contracts through the public SDK.
package moduleport

import (
	"fmt"

	"agent-vivy/sdk/module"
)

type Cardinality string

const (
	CardinalityExactlyOne Cardinality = "1"
	CardinalityAtMostOne  Cardinality = "0..1"
)

type Requirement string

const (
	RequirementAlways      Requirement = "always"
	RequirementAgent       Requirement = "agent"
	RequirementDefault     Requirement = "default"
	RequirementConditional Requirement = "conditional"
)

type SourceClass string

const (
	SourceInternal SourceClass = "internal"
	SourcePublic   SourceClass = "public"
)

// Definition is build-owned metadata for one non-public core Port. RequiredBy
// contains public Ports whose selection makes an optional Host mandatory.
type Definition struct {
	Ref         module.PortRef
	Owner       string
	Cardinality Cardinality
	Requirement Requirement
	RequiredBy  []module.PortRef
}

type Registry struct {
	definitions []Definition
}

// Catalog returns the canonical closed internal Port definitions in normative
// order. Callers receive copies so build code cannot mutate the authority
// table at runtime.
func Catalog() Registry {
	return Registry{definitions: []Definition{
		closed("core/loop-driver@v1", "vivy/loop", CardinalityExactlyOne, RequirementAgent),
		closed("core/chat-model-host@v1", "vivy/model", CardinalityExactlyOne, RequirementAgent),
		closed("core/tool-host@v1", "vivy/tool-host", CardinalityExactlyOne, RequirementDefault),
		closed("core/storage-engine@v1", "vivy/storage", CardinalityExactlyOne, RequirementAlways),
		closed("core/checkpoint-store@v1", "vivy/checkpoint", CardinalityExactlyOne, RequirementAgent),
		closed("core/credential-resolver@v1", "vivy/credential", CardinalityExactlyOne, RequirementAlways),
		closed("core/sandbox-backend@v1", "vivy/sandbox", CardinalityExactlyOne, RequirementAgent),
		conditional("core/context-host@v1", "vivy/context-host", "std/context-source@v1"),
		conditional("core/skill-host@v1", "vivy/skill-host", "std/skill-source@v1"),
		closed("core/mcp-host@v1", "vivy/mcp-host", CardinalityAtMostOne, RequirementDefault),
		conditional("core/observer-host@v1", "vivy/observer-host", "std/observer/run@v1", "std/observer/diagnostic@v1"),
		conditional("core/status-host@v1", "vivy/status-host", "std/status-source@v1"),
		conditional("core/presentation-host@v1", "vivy/presentation-host", "std/ui-extension@v1", "std/ui-root@v1"),
		conditional("core/action-host@v1", "vivy/action-host", "std/control-action@v1"),
	}}
}

func (registry Registry) Definitions() []Definition {
	out := make([]Definition, len(registry.definitions))
	for index, definition := range registry.definitions {
		out[index] = definition
		out[index].RequiredBy = append([]module.PortRef(nil), definition.RequiredBy...)
	}
	return out
}

func (registry Registry) Lookup(ref module.PortRef) (Definition, bool) {
	for _, definition := range registry.definitions {
		if definition.Ref.Port == ref.Port {
			definition.RequiredBy = append([]module.PortRef(nil), definition.RequiredBy...)
			return definition, true
		}
	}
	return Definition{}, false
}

// ValidateProvider enforces that only the one build-owned T1 owner can provide
// a known core Port.
func (registry Registry) ValidateProvider(ref module.PortRef, moduleID string, source SourceClass) error {
	definition, ok := registry.Lookup(ref)
	if !ok {
		return fmt.Errorf("unknown closed internal Port %s", ref.Port)
	}
	if source != SourceInternal || moduleID != definition.Owner {
		return fmt.Errorf("core Port %s may only be provided by build-owned T1 module %s", ref.Port, definition.Owner)
	}
	return nil
}

// RequiredHosts returns conditional internal Hosts activated by selection of
// the supplied public Port.
func (registry Registry) RequiredHosts(publicPort module.PortRef) []module.PortRef {
	var out []module.PortRef
	for _, definition := range registry.definitions {
		if definition.Requirement != RequirementConditional {
			continue
		}
		for _, trigger := range definition.RequiredBy {
			if trigger.Port == publicPort.Port {
				out = append(out, definition.Ref)
				break
			}
		}
	}
	return out
}

func closed(port, owner string, cardinality Cardinality, requirement Requirement) Definition {
	return Definition{
		Ref:         module.PortRef{Port: port},
		Owner:       owner,
		Cardinality: cardinality,
		Requirement: requirement,
	}
}

func conditional(port, owner string, requiredBy ...string) Definition {
	definition := closed(port, owner, CardinalityAtMostOne, RequirementConditional)
	for _, publicPort := range requiredBy {
		definition.RequiredBy = append(definition.RequiredBy, module.PortRef{Port: publicPort})
	}
	return definition
}
