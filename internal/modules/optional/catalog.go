// Package optional declares the build-owned default and conditional Host
// inventory. It carries identifiers only; executable Host authority remains in
// the established L0 owners.
package optional

type Definition struct {
	ModuleID    string
	Port        string
	Constructor string
	DefaultOn   bool
	RequiredBy  []string
}

func Catalog() []Definition {
	return []Definition{
		{ModuleID: "vivy/tool-host", Port: "core/tool-host@v1", Constructor: "NewToolHost", DefaultOn: true},
		{ModuleID: "vivy/context-host", Port: "core/context-host@v1", Constructor: "NewContextHost", DefaultOn: true, RequiredBy: []string{"std/context-source@v1"}},
		{ModuleID: "vivy/skill-host", Port: "core/skill-host@v1", Constructor: "NewSkillHost", DefaultOn: true, RequiredBy: []string{"std/skill-source@v1"}},
		{ModuleID: "vivy/mcp-host", Port: "core/mcp-host@v1", Constructor: "NewMCPHost", DefaultOn: true},
		{ModuleID: "vivy/observer-host", Port: "core/observer-host@v1", Constructor: "NewObserverHost", DefaultOn: true, RequiredBy: []string{"std/observer/run@v1", "std/observer/diagnostic@v1"}},
		{ModuleID: "vivy/status-host", Port: "core/status-host@v1", Constructor: "NewStatusHost", DefaultOn: true, RequiredBy: []string{"std/status-source@v1"}},
		{ModuleID: "vivy/presentation-host", Port: "core/presentation-host@v1", Constructor: "NewPresentationHost", DefaultOn: true, RequiredBy: []string{"std/ui-extension@v1", "std/ui-root@v1"}},
		{ModuleID: "vivy/action-host", Port: "core/action-host@v1", Constructor: "NewActionHost", DefaultOn: true, RequiredBy: []string{"std/control-action@v1"}},
	}
}
