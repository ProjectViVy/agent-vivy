// Package catalog owns static module metadata without loading its optional implementation.
package catalog

const (
	// ID is the control-action module record.
	ID = "vivy/memory-bml"
	// SyncID is the sync-plane module record (context-source + run-observer).
	SyncID = "vivy/memory-bml-sync"
	// ProviderID is the single provider identity satisfying both the
	// ContextSources and RunObservers manifest lists.
	ProviderID = "vivy.memory.bml"
)

const (
	ActionList       = "vivy.memory.list"
	ActionSearch     = "vivy.memory.search"
	ActionGet        = "vivy.memory.get"
	ActionAdd        = "vivy.memory.add"
	ActionUpdate     = "vivy.memory.update"
	ActionRemove     = "vivy.memory.remove"
	ActionRulesRead  = "vivy.memory.rules.read"
	ActionRulesWrite = "vivy.memory.rules.write"
	ActionStatus     = "vivy.memory.status"
)

const (
	ToolAdd    = "memory_add"
	ToolGet    = "memory_get"
	ToolList   = "memory_list"
	ToolSearch = "memory_search"
	ToolUpdate = "memory_update"
	ToolRemove = "memory_remove"
)
