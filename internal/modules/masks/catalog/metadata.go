// Package catalog owns static module metadata without loading its optional implementation.
package catalog

const (
	ID   = "vivy/masks"
	Port = "core/mask-service@v1"
)

const (
	ActionCatalogList   = "vivy.masks.catalog.list"
	ActionCatalogGet    = "vivy.masks.catalog.get"
	ActionCatalogCreate = "vivy.masks.catalog.create"
	ActionCatalogUpdate = "vivy.masks.catalog.update"
	ActionCatalogDelete = "vivy.masks.catalog.delete"
	ActionSelectionGet  = "vivy.masks.selection.get"
	ActionSelectionSet  = "vivy.masks.selection.set"
)
