package moduleport

import (
	"reflect"
	"testing"

	"agent-vivy/sdk/module"
)

func TestCoreCatalogDefinesCanonicalClosedPorts(t *testing.T) {
	want := []Definition{
		testClosed("core/loop-driver@v1", "vivy/loop", CardinalityExactlyOne, RequirementAgent),
		testClosed("core/chat-model-host@v1", "vivy/model", CardinalityExactlyOne, RequirementAgent),
		testClosed("core/tool-host@v1", "vivy/tool-host", CardinalityExactlyOne, RequirementDefault),
		testClosed("core/storage-engine@v1", "vivy/storage", CardinalityExactlyOne, RequirementAlways),
		testClosed("core/checkpoint-store@v1", "vivy/checkpoint", CardinalityExactlyOne, RequirementAgent),
		testClosed("core/credential-resolver@v1", "vivy/credential", CardinalityExactlyOne, RequirementAlways),
		testClosed("core/sandbox-backend@v1", "vivy/sandbox", CardinalityExactlyOne, RequirementAgent),
		testConditional("core/context-host@v1", "vivy/context-host", "std/context-source@v1"),
		testConditional("core/skill-host@v1", "vivy/skill-host", "std/skill-source@v1"),
		testClosed("core/mcp-host@v1", "vivy/mcp-host", CardinalityAtMostOne, RequirementDefault),
		testConditional("core/observer-host@v1", "vivy/observer-host", "std/observer/run@v1", "std/observer/diagnostic@v1"),
		testConditional("core/status-host@v1", "vivy/status-host", "std/status-source@v1"),
		testConditional("core/presentation-host@v1", "vivy/presentation-host", "std/ui-extension@v1", "std/ui-root@v1"),
		testConditional("core/action-host@v1", "vivy/action-host", "std/control-action@v1"),
	}

	if got := Catalog().Definitions(); !reflect.DeepEqual(got, want) {
		t.Fatalf("closed Port definitions = %#v, want %#v", got, want)
	}
}

func TestPublicSourceCannotProvideCorePort(t *testing.T) {
	catalog := Catalog()
	for _, definition := range catalog.Definitions() {
		if err := catalog.ValidateProvider(definition.Ref, definition.Owner, SourcePublic); err == nil {
			t.Errorf("public source provided %s", definition.Ref.Port)
		}
		if err := catalog.ValidateProvider(definition.Ref, definition.Owner, SourceInternal); err != nil {
			t.Errorf("build-owned provider for %s rejected: %v", definition.Ref.Port, err)
		}
		if err := catalog.ValidateProvider(definition.Ref, "vivy/not-the-owner", SourceInternal); err == nil {
			t.Errorf("wrong internal owner provided %s", definition.Ref.Port)
		}
	}
}

func TestConditionalCoreHostsFollowOnlyTheirCatalogedPublicPorts(t *testing.T) {
	catalog := Catalog()
	want := map[string][]string{
		"std/context-source@v1":      {"core/context-host@v1"},
		"std/skill-source@v1":        {"core/skill-host@v1"},
		"std/observer/run@v1":        {"core/observer-host@v1"},
		"std/observer/diagnostic@v1": {"core/observer-host@v1"},
		"std/status-source@v1":       {"core/status-host@v1"},
		"std/ui-extension@v1":        {"core/presentation-host@v1"},
		"std/ui-root@v1":             {"core/presentation-host@v1"},
		"std/control-action@v1":      {"core/action-host@v1"},
	}

	for publicPort, corePorts := range want {
		got := catalog.RequiredHosts(module.PortRef{Port: publicPort})
		if !reflect.DeepEqual(got, refs(corePorts...)) {
			t.Errorf("RequiredHosts(%s) = %#v, want %#v", publicPort, got, refs(corePorts...))
		}
	}
	if got := catalog.RequiredHosts(module.PortRef{Port: "std/tool@v1"}); len(got) != 0 {
		t.Fatalf("non-conditional ToolHost returned as conditional: %#v", got)
	}
}

func testClosed(port, owner string, cardinality Cardinality, requirement Requirement) Definition {
	return Definition{
		Ref:         module.PortRef{Port: port},
		Owner:       owner,
		Cardinality: cardinality,
		Requirement: requirement,
	}
}

func testConditional(port, owner string, requiredBy ...string) Definition {
	definition := testClosed(port, owner, CardinalityAtMostOne, RequirementConditional)
	definition.RequiredBy = refs(requiredBy...)
	return definition
}

func refs(ports ...string) []module.PortRef {
	out := make([]module.PortRef, 0, len(ports))
	for _, port := range ports {
		out = append(out, module.PortRef{Port: port})
	}
	return out
}
