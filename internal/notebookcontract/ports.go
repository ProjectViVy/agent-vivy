package notebookcontract

import "context"

// Store is the scoped, revisioned notebook content surface. Every key —
// query, uniqueness, revision, comment — carries the trusted scope; unknown
// and out-of-scope resource IDs share not_found behavior.
//
// Mutations take a MutationContext built only by trusted consumers: the
// operation key must be non-empty and unique per scope, and RequestDigest is
// verified against the canonical digest of the request kind and fields.
type Store interface {
	// ListSections returns section metadata; the first access per scope
	// idempotently seeds the Notes/Daily/Weekly/Monthly role sections.
	ListSections(context.Context, ScopeID, ListSectionsRequest) (SectionPage, error)
	// CreateSection creates one custom section.
	CreateSection(context.Context, MutationContext, CreateSectionRequest) (MutationReceipt, error)
	// UpdateSection renames a section under expected-version CAS.
	UpdateSection(context.Context, MutationContext, UpdateSectionRequest) (MutationReceipt, error)
	// DeleteSection tombstones an empty non-system section under CAS.
	// Populated sections yield section_not_empty; required system-role
	// sections yield section_in_use.
	DeleteSection(context.Context, MutationContext, DeleteSectionRequest) (MutationReceipt, error)
	// RestoreSection clears a tombstone under CAS.
	RestoreSection(context.Context, MutationContext, RestoreSectionRequest) (MutationReceipt, error)

	// ListEntries returns entry metadata only; bodies are fetched
	// individually. The cursor is bound to scope and filter.
	ListEntries(context.Context, ScopeID, ListEntriesRequest) (EntryPage, error)
	// GetEntry returns entry metadata plus one selected revision body.
	GetEntry(context.Context, ScopeID, GetEntryRequest) (EntryView, error)
	// CreateEntry makes one ordinary note in an existing section.
	CreateEntry(context.Context, MutationContext, CreateEntryRequest) (MutationReceipt, error)
	// SaveEntry commits a body edit under expected-version and
	// base-revision CAS; a stale base returns revision_conflict with
	// current metadata and never merges.
	SaveEntry(context.Context, MutationContext, SaveEntryRequest) (MutationReceipt, error)
	// MoveEntry changes section metadata under CAS.
	MoveEntry(context.Context, MutationContext, MoveEntryRequest) (MutationReceipt, error)
	// DeleteEntry tombstones an entry; revisions and comments are retained.
	DeleteEntry(context.Context, MutationContext, DeleteEntryRequest) (MutationReceipt, error)
	// RestoreEntry clears a tombstone under CAS.
	RestoreEntry(context.Context, MutationContext, RestoreEntryRequest) (MutationReceipt, error)

	// ListRevisions returns revision metadata newest-first, including
	// generated candidates, without bodies.
	ListRevisions(context.Context, ScopeID, ListRevisionsRequest) (RevisionPage, error)
	// AdoptRevision creates a new revision whose content references the
	// selected revision — an explicit restore, never an erase.
	AdoptRevision(context.Context, MutationContext, AdoptRevisionRequest) (MutationReceipt, error)

	// ListComments returns comments for one entry, optionally filtered by
	// status.
	ListComments(context.Context, ScopeID, ListCommentsRequest) (CommentPage, error)
	// CreateComment attaches feedback optionally anchored to a revision.
	CreateComment(context.Context, MutationContext, CreateCommentRequest) (MutationReceipt, error)
	// UpdateComment edits body and/or status under CAS; every version is
	// retained so generation can refer to older versions.
	UpdateComment(context.Context, MutationContext, UpdateCommentRequest) (MutationReceipt, error)
}

// GeneratedWriter is the reserved internal seam bound to an admitted report
// Run in R1. It is the only path that stamps origin=generated. N1 defines the
// surface only — report provenance lands with its implementation.
type GeneratedWriter interface {
	// CommitGenerated publishes one generated revision: promoted to the
	// visible head when the snapshot head still matches and is unedited
	// generated content, otherwise retained as an adoptable candidate.
	CommitGenerated(context.Context, MutationContext, GeneratedCommit) (GeneratedReceipt, error)
}
