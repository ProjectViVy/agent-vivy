package port

import (
	"reflect"
	"testing"

	"agent-vivy/sdk/module"
)

func TestCatalogContainsExactlyApprovedPublicPorts(t *testing.T) {
	want := []module.PortRef{
		{Port: "std/tool@v1"},
		{Port: "std/tool-world@v1"},
		{Port: "std/channel@v1"},
		{Port: "std/face@v1"},
		{Port: "std/provider-profile@v1"},
		{Port: "std/context-source@v1"},
		{Port: "std/skill-source@v1"},
		{Port: "std/middleware/pre-tool@v1"},
		{Port: "std/observer/run@v1"},
		{Port: "std/observer/diagnostic@v1"},
		{Port: "std/status-source@v1"},
		{Port: "std/ui-extension@v1"},
		{Port: "std/ui-root@v1"},
		{Port: "std/control-action@v1"},
	}

	definitions := PublicCatalog().Definitions()
	got := make([]module.PortRef, 0, len(definitions))
	for _, definition := range definitions {
		got = append(got, definition.Ref)
		if definition.Visibility != VisibilityPublic {
			t.Errorf("port %s visibility = %q, want public", definition.Ref.Port, definition.Visibility)
		}
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("public Port refs = %#v, want %#v", got, want)
	}
}

func TestCatalogRejectsUnknownPort(t *testing.T) {
	if _, ok := PublicCatalog().Lookup(module.PortRef{Port: "std/unknown@v1"}); ok {
		t.Fatal("Lookup accepted an unknown Port")
	}
}
