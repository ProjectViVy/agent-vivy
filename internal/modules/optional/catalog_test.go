package optional

import (
	"reflect"
	"testing"
)

func TestCatalogPinsDefaultAndConditionalHosts(t *testing.T) {
	want := map[string]Definition{
		"vivy/tool-host":         {ModuleID: "vivy/tool-host", Port: "core/tool-host@v1", Constructor: "NewToolHost", DefaultOn: true},
		"vivy/context-host":      {ModuleID: "vivy/context-host", Port: "core/context-host@v1", Constructor: "NewContextHost", DefaultOn: true, RequiredBy: []string{"std/context-source@v1"}},
		"vivy/skill-host":        {ModuleID: "vivy/skill-host", Port: "core/skill-host@v1", Constructor: "NewSkillHost", DefaultOn: true, RequiredBy: []string{"std/skill-source@v1"}},
		"vivy/mcp-host":          {ModuleID: "vivy/mcp-host", Port: "core/mcp-host@v1", Constructor: "NewMCPHost", DefaultOn: true},
		"vivy/observer-host":     {ModuleID: "vivy/observer-host", Port: "core/observer-host@v1", Constructor: "NewObserverHost", DefaultOn: true, RequiredBy: []string{"std/observer/run@v1", "std/observer/diagnostic@v1"}},
		"vivy/status-host":       {ModuleID: "vivy/status-host", Port: "core/status-host@v1", Constructor: "NewStatusHost", DefaultOn: true, RequiredBy: []string{"std/status-source@v1"}},
		"vivy/presentation-host": {ModuleID: "vivy/presentation-host", Port: "core/presentation-host@v1", Constructor: "NewPresentationHost", DefaultOn: true, RequiredBy: []string{"std/ui-extension@v1", "std/ui-root@v1"}},
		"vivy/action-host":       {ModuleID: "vivy/action-host", Port: "core/action-host@v1", Constructor: "NewActionHost", DefaultOn: true, RequiredBy: []string{"std/control-action@v1"}},
	}
	got := map[string]Definition{}
	for _, definition := range Catalog() {
		got[definition.ModuleID] = definition
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Catalog() = %#v, want %#v", got, want)
	}
}
