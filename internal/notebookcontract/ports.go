package notebookcontract

import (
	"context"

	controlaction "agent-vivy/sdk/port/controlaction"
)

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

// --- N2 seam: construction and owner-bound facades -----------------------------

// Factory is emitted into a generated RuntimeAssembly only when the
// composition selected core/notebook-service@v1. App unwraps the typed value
// through the generated NotebookFactoryValue accessor and invokes it once.
// No raw DB, runtime Service, or untyped locator crosses this seam.
type Factory func(context.Context, FactoryInput) (Bundle, error)

// FactoryInput carries the N1 narrow Store plus the trusted scope resolver.
type FactoryInput struct {
	// Store is the scoped revisioned content surface owned by Storage.
	Store Store
	// Scopes resolves Home or the canonical workspace of a
	// server-authorized Session; it never accepts a raw path or an
	// arbitrary scope ID from a provider.
	Scopes ScopeResolver
	// GenerationID is the sealed Generation identity.
	GenerationID string
}

// ScopeResolver is the only trusted source of scope IDs. Home always
// resolves; a workspace scope resolves only from a session identity the host
// already authenticated — never from provider JSON.
type ScopeResolver interface {
	Home() ScopeID
	// ForSession returns the canonical workspace scope of a
	// server-authorized Session. Unknown sessions fail not_found.
	ForSession(ctx context.Context, sessionID string) (ScopeID, error)
}

// ScopedActions is the trusted, owner-bound facade the ActionHost grants the
// notebook module. Scope and actor live in the host binding — request DTOs
// carry no scope, actor, or origin claims, and JSON schemas reject them.
type ScopedActions interface {
	ListSections(context.Context, ListSectionsRequest) (SectionPage, error)
	CreateSection(context.Context, OperationKeyed[CreateSectionRequest]) (MutationReceipt, error)
	UpdateSection(context.Context, OperationKeyed[UpdateSectionRequest]) (MutationReceipt, error)
	DeleteSection(context.Context, OperationKeyed[DeleteSectionRequest]) (MutationReceipt, error)
	RestoreSection(context.Context, OperationKeyed[RestoreSectionRequest]) (MutationReceipt, error)

	ListEntries(context.Context, ListEntriesRequest) (EntryPage, error)
	GetEntry(context.Context, GetEntryRequest) (EntryView, error)
	CreateEntry(context.Context, OperationKeyed[CreateEntryRequest]) (MutationReceipt, error)
	SaveEntry(context.Context, OperationKeyed[SaveEntryRequest]) (MutationReceipt, error)
	MoveEntry(context.Context, OperationKeyed[MoveEntryRequest]) (MutationReceipt, error)
	DeleteEntry(context.Context, OperationKeyed[DeleteEntryRequest]) (MutationReceipt, error)
	RestoreEntry(context.Context, OperationKeyed[RestoreEntryRequest]) (MutationReceipt, error)

	ListRevisions(context.Context, ListRevisionsRequest) (RevisionPage, error)
	AdoptRevision(context.Context, OperationKeyed[AdoptRevisionRequest]) (MutationReceipt, error)

	ListComments(context.Context, ListCommentsRequest) (CommentPage, error)
	CreateComment(context.Context, OperationKeyed[CreateCommentRequest]) (MutationReceipt, error)
	UpdateComment(context.Context, OperationKeyed[UpdateCommentRequest]) (MutationReceipt, error)

	// Export returns bounded Markdown plus provenance/comment sidecar data
	// for one exact revision; it writes nothing.
	Export(context.Context, GetEntryRequest) (ExportBundle, error)
}

// OperationKeyed wraps one typed request with its caller-chosen idempotent
// operation key — the only mutation envelope a provider may supply.
type OperationKeyed[T any] struct {
	OperationKey string `json:"operation_key"`
	Request      T      `json:"request"`
}

// ExportBundle is the vivy.notebook.export result: one revision body plus
// provenance and comment sidecar data, bounded by the Host wire ceiling.
type ExportBundle struct {
	Entry    Entry     `json:"entry"`
	Revision Revision  `json:"revision"`
	Comments []Comment `json:"comments"`
}

// Bundle is the constructed owner surface returned by the factory: the
// service bound to Storage plus an idempotent close handle. It is not an
// action provider; ActionHost builds the scoped facade from it.
type Bundle interface {
	// Service returns the raw scoped store for trusted host-side use
	// (tools, export, provenance classification). It is never handed to a
	// provider.
	Service() Store
	// Actions binds the facade to one trusted scope and actor; the host
	// supplies both, never the caller.
	Actions(scope ScopeID, actor Actor) ScopedActions
	Close(context.Context) error
}

// ActionHost is the sealed internal extension of the public
// controlaction.Host granted only to the vivy/notebook-core owner. The public
// Host interface deliberately does not grow a notebook method.
type ActionHost interface {
	controlaction.Host
	// Notebook returns the facade bound to this invocation's authenticated
	// scope and actor. Unarmed or mismatched bindings fail closed.
	Notebook() (ScopedActions, error)
}
