// Package notebookcontract carries the dependency-free content DTOs for the
// scoped, revisioned notebook (N1). It imports neither Storage nor the
// runtime/Eino layers; Hosts bind it in N2 and report generation binds the
// GeneratedWriter seam in R1.
package notebookcontract

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
)

// Design bounds (spec section 7): body bytes count UTF-8 bytes, not runes;
// metadata pages never embed bodies and never exceed MaxPageRows.
const (
	MaxBodyBytes    = 256 * 1024
	MaxCommentBytes = 16 * 1024
	MaxPageRows     = 100
)

// ScopeID is the host-resolved notebook namespace carried inside every data
// key. Transport selectors never become durable owner keys.
type ScopeID string

// HomeScopeID is the fixed local scope key for the personal/home notebook.
// Workspace scopes carry a versioned prefix derived by the Host (N2).
const HomeScopeID ScopeID = "home"

// WorkspaceScope builds the versioned scope key for a canonical workspace
// identity. The Host owns when this is called; callers never hash paths.
func WorkspaceScope(workspaceID string) ScopeID { return ScopeID("ws.v1:" + workspaceID) }

// ActorKind is the trusted admission origin the Host stamps on every write.
type ActorKind string

const (
	ActorHuman    ActorKind = "human"
	ActorAgent    ActorKind = "agent"
	ActorWorkflow ActorKind = "workflow"
)

// Valid reports whether kind is a known origin.
func (k ActorKind) Valid() bool {
	switch k {
	case ActorHuman, ActorAgent, ActorWorkflow:
		return true
	}
	return false
}

// Actor is a trusted identity reference (e.g. "local:operator" or a Run ID),
// never a client-claimed flag.
type Actor struct {
	Kind ActorKind
	Ref  string
}

// MutationContext carries the internally trusted envelope every notebook
// mutation requires. Only trusted consumers construct it (N2 binds scope and
// actor); JSON requests contain no authoritative fields.
type MutationContext struct {
	ScopeID       ScopeID
	Actor         Actor
	OperationKey  string
	RequestDigest string
}

// Origin is stamped on an immutable revision by the writer path, never by the
// request body.
type Origin string

const (
	OriginLegacy    Origin = "legacy"
	OriginHuman     Origin = "human"
	OriginAgent     Origin = "agent"
	OriginGenerated Origin = "generated"
)

// OriginFor maps a trusted actor kind to the revision origin it produces.
// Report generation uses the reserved GeneratedWriter seam instead.
func (k ActorKind) OriginFor() (Origin, error) {
	switch k {
	case ActorHuman:
		return OriginHuman, nil
	case ActorAgent:
		return OriginAgent, nil
	default:
		return "", fmt.Errorf("notebook: actor kind %q cannot write ordinary revisions", k)
	}
}

// Code is the stable domain error code of the bounded result envelope.
type Code string

const (
	CodeInvalidRequest        Code = "invalid_request"
	CodeNotFound              Code = "not_found"
	CodeRevisionConflict      Code = "revision_conflict"
	CodeIdempotencyConflict   Code = "idempotency_conflict"
	CodeSectionNotEmpty       Code = "section_not_empty"
	CodeSectionInUse          Code = "section_in_use"
	CodeLimitExceeded         Code = "limit_exceeded"
	CodeCapabilityUnavailable Code = "capability_unavailable"
	CodeStorageUnavailable    Code = "storage_unavailable"
	CodeCancelled             Code = "cancelled"
	CodeRecoveryRequired      Code = "recovery_required"
	CodeOutcomeUnknown        Code = "outcome_unknown"
	CodeDestinationDeleted    Code = "destination_deleted"
)

// Error is the typed domain error. CurrentVersion/CurrentRevisionID carry the
// server's current metadata on revision_conflict so callers can rebase without
// another read.
type Error struct {
	Code              Code
	Message           string
	Retryable         bool
	CurrentVersion    int64
	CurrentRevisionID string
}

func (e *Error) Error() string {
	if e.Message == "" {
		return "notebook: " + string(e.Code)
	}
	return "notebook: " + e.Message
}

// Is matches by code so sentinel values work with errors.Is.
func (e *Error) Is(target error) bool {
	t, ok := target.(*Error)
	return ok && t.Code == e.Code
}

// CodeOf extracts the stable code; unknown errors map to
// CodeStorageUnavailable.
func CodeOf(err error) Code {
	var e *Error
	if errors.As(err, &e) {
		return e.Code
	}
	return CodeStorageUnavailable
}

// CurrentOf extracts current-version metadata when present.
func CurrentOf(err error) (version int64, revisionID string) {
	var e *Error
	if errors.As(err, &e) {
		return e.CurrentVersion, e.CurrentRevisionID
	}
	return 0, ""
}

// Sentinel values for errors.Is comparisons.
var (
	ErrNotFound            = &Error{Code: CodeNotFound, Message: "not found"}
	ErrRevisionConflict    = &Error{Code: CodeRevisionConflict, Message: "revision conflict"}
	ErrIdempotencyConflict = &Error{Code: CodeIdempotencyConflict, Message: "operation key reused with a different request"}
	ErrSectionNotEmpty     = &Error{Code: CodeSectionNotEmpty, Message: "section still has content"}
	ErrSectionInUse        = &Error{Code: CodeSectionInUse, Message: "section is required by the notebook"}
	ErrLimitExceeded       = &Error{Code: CodeLimitExceeded, Message: "content limit exceeded"}
	ErrInvalidRequest      = &Error{Code: CodeInvalidRequest, Message: "invalid request"}
)

// SystemRole marks built-in sections; localized titles are presentation only
// and never part of identity.
type SystemRole string

const (
	RoleNone    SystemRole = ""
	RoleNotes   SystemRole = "notes"
	RoleDaily   SystemRole = "daily"
	RoleWeekly  SystemRole = "weekly"
	RoleMonthly SystemRole = "monthly"
)

// SystemRoles is the seed set created idempotently per scope.
var SystemRoles = []SystemRole{RoleNotes, RoleDaily, RoleWeekly, RoleMonthly}

// RoleSectionID is the deterministic section ID for a system role inside one
// scope. Stable role IDs survive display-title changes.
func RoleSectionID(role SystemRole) string { return "section-" + string(role) }

// RoleDefaultTitle is the fallback English title for a role; localized labels
// stay presentation values.
func RoleDefaultTitle(role SystemRole) string {
	switch role {
	case RoleNotes:
		return "Notes"
	case RoleDaily:
		return "Daily"
	case RoleWeekly:
		return "Weekly"
	case RoleMonthly:
		return "Monthly"
	default:
		return ""
	}
}

// Section is one content container. Version guards every mutation.
type Section struct {
	ID         string     `json:"id"`
	Title      string     `json:"title"`
	SystemRole SystemRole `json:"system_role,omitempty"`
	Version    int64      `json:"version"`
	CreatedAt  int64      `json:"created_at"`
	UpdatedAt  int64      `json:"updated_at"`
	DeletedAt  int64      `json:"deleted_at,omitempty"`
}

// EntryKind is fixed at creation; entries never change kind.
type EntryKind string

const (
	EntryNote   EntryKind = "note"
	EntryReport EntryKind = "report"
)

// Entry is one notebook document head. Title mirrors the head revision so
// metadata pages never join bodies.
type Entry struct {
	ID             string    `json:"id"`
	SectionID      string    `json:"section_id"`
	Kind           EntryKind `json:"kind"`
	Title          string    `json:"title"`
	HeadRevisionID string    `json:"head_revision_id"`
	Version        int64     `json:"version"`
	ReportSeriesID string    `json:"report_series_id,omitempty"`
	ReportWindowID string    `json:"report_window_id,omitempty"`
	CreatedAt      int64     `json:"created_at"`
	UpdatedAt      int64     `json:"updated_at"`
	DeletedAt      int64     `json:"deleted_at,omitempty"`
}

// Revision is immutable content. ParentRevisionID links the edit chain;
// BaseRevisionID records the adopted/generated source a new revision refers
// to.
type Revision struct {
	ID               string `json:"id"`
	EntryID          string `json:"entry_id"`
	Sequence         int64  `json:"sequence"`
	ParentRevisionID string `json:"parent_revision_id,omitempty"`
	Title            string `json:"title"`
	Markdown         string `json:"markdown"`
	Origin           Origin `json:"origin"`
	BaseRevisionID   string `json:"base_revision_id,omitempty"`
	Actor            string `json:"actor"`
	CreatedAt        int64  `json:"created_at"`
}

// CommentStatus covers the document-level feedback lifecycle.
type CommentStatus string

const (
	CommentActive   CommentStatus = "active"
	CommentResolved CommentStatus = "resolved"
	CommentDeleted  CommentStatus = "deleted"
)

// Valid reports whether status is a known comment state.
func (s CommentStatus) Valid() bool {
	switch s {
	case CommentActive, CommentResolved, CommentDeleted:
		return true
	}
	return false
}

// Comment is document-level feedback, optionally anchored to one revision.
type Comment struct {
	ID               string        `json:"id"`
	EntryID          string        `json:"entry_id"`
	AnchorRevisionID string        `json:"anchor_revision_id,omitempty"`
	Body             string        `json:"body"`
	Version          int64         `json:"version"`
	Author           string        `json:"author"`
	Status           CommentStatus `json:"status"`
	CreatedAt        int64         `json:"created_at"`
	UpdatedAt        int64         `json:"updated_at"`
}

// MutationReceipt is the durable outcome of one committed mutation; identical
// retries return the stored receipt instead of minting new state.
type MutationReceipt struct {
	ResourceID string `json:"resource_id"`
	Version    int64  `json:"version"`
	RevisionID string `json:"revision_id,omitempty"`
	Replayed   bool   `json:"replayed"`
}

// --- List requests and pages -------------------------------------------------

// ListSectionsRequest pages section metadata.
type ListSectionsRequest struct {
	Cursor string `json:"cursor,omitempty"`
	Limit  int    `json:"limit,omitempty"`
}

// SectionPage is one stable page of section metadata.
type SectionPage struct {
	Sections   []Section `json:"sections"`
	NextCursor string    `json:"next_cursor,omitempty"`
}

// ListEntriesRequest pages entry metadata inside a scope; bodies are never
// embedded.
type ListEntriesRequest struct {
	SectionID      string `json:"section_id,omitempty"`
	IncludeDeleted bool   `json:"include_deleted,omitempty"`
	Cursor         string `json:"cursor,omitempty"`
	Limit          int    `json:"limit,omitempty"`
}

// EntryPage is one stable (updated_at,id) page of entry metadata.
type EntryPage struct {
	Entries    []Entry `json:"entries"`
	NextCursor string  `json:"next_cursor,omitempty"`
}

// GetEntryRequest selects one entry and one revision (head when empty).
type GetEntryRequest struct {
	EntryID    string `json:"entry_id"`
	RevisionID string `json:"revision_id,omitempty"`
}

// EntryView returns entry metadata with the selected revision body.
type EntryView struct {
	Entry    Entry    `json:"entry"`
	Revision Revision `json:"revision"`
}

// ListRevisionsRequest pages one entry's revision metadata, including
// generated candidates.
type ListRevisionsRequest struct {
	EntryID string `json:"entry_id"`
	Cursor  string `json:"cursor,omitempty"`
	Limit   int    `json:"limit,omitempty"`
}

// RevisionPage is one sequence-ordered page; Markdown bodies are fetched
// individually.
type RevisionPage struct {
	Revisions  []Revision `json:"revisions"`
	NextCursor string     `json:"next_cursor,omitempty"`
}

// ListCommentsRequest pages comments for one entry.
type ListCommentsRequest struct {
	EntryID string        `json:"entry_id"`
	Status  CommentStatus `json:"status,omitempty"`
	Cursor  string        `json:"cursor,omitempty"`
	Limit   int           `json:"limit,omitempty"`
}

// CommentPage is one creation-ordered page of comments.
type CommentPage struct {
	Comments   []Comment `json:"comments"`
	NextCursor string    `json:"next_cursor,omitempty"`
}

// --- Mutation requests -------------------------------------------------------

// CreateSectionRequest makes one custom section.
type CreateSectionRequest struct {
	Title string `json:"title"`
}

// UpdateSectionRequest renames a section under CAS.
type UpdateSectionRequest struct {
	ID              string `json:"id"`
	Title           string `json:"title"`
	ExpectedVersion int64  `json:"expected_version"`
}

// DeleteSectionRequest tombstones an empty, non-system section under CAS.
type DeleteSectionRequest struct {
	ID              string `json:"id"`
	ExpectedVersion int64  `json:"expected_version"`
}

// RestoreSectionRequest clears a section tombstone under CAS.
type RestoreSectionRequest struct {
	ID              string `json:"id"`
	ExpectedVersion int64  `json:"expected_version"`
}

// CreateEntryRequest makes one ordinary note; it cannot claim generated or
// report provenance.
type CreateEntryRequest struct {
	SectionID string `json:"section_id"`
	Title     string `json:"title"`
	Markdown  string `json:"markdown"`
}

// SaveEntryRequest commits a human/agent body edit under version + base CAS.
type SaveEntryRequest struct {
	EntryID         string `json:"entry_id"`
	ExpectedVersion int64  `json:"expected_version"`
	BaseRevisionID  string `json:"base_revision_id"`
	Title           string `json:"title"`
	Markdown        string `json:"markdown"`
}

// MoveEntryRequest changes an entry's section metadata under CAS.
type MoveEntryRequest struct {
	EntryID         string `json:"entry_id"`
	SectionID       string `json:"section_id"`
	ExpectedVersion int64  `json:"expected_version"`
}

// DeleteEntryRequest tombstones an entry under CAS; history is retained.
type DeleteEntryRequest struct {
	EntryID         string `json:"entry_id"`
	ExpectedVersion int64  `json:"expected_version"`
}

// RestoreEntryRequest clears an entry tombstone under CAS.
type RestoreEntryRequest struct {
	EntryID         string `json:"entry_id"`
	ExpectedVersion int64  `json:"expected_version"`
}

// AdoptRevisionRequest promotes an existing revision's content by creating a
// new revision that references it; history is never rewritten.
type AdoptRevisionRequest struct {
	EntryID         string `json:"entry_id"`
	RevisionID      string `json:"revision_id"`
	ExpectedVersion int64  `json:"expected_version"`
}

// CreateCommentRequest attaches document-level feedback to an entry.
type CreateCommentRequest struct {
	EntryID          string `json:"entry_id"`
	AnchorRevisionID string `json:"anchor_revision_id,omitempty"`
	Body             string `json:"body"`
}

// UpdateCommentRequest edits and/or transitions a comment under CAS. A nil
// Body keeps the stored text; an empty Status keeps the stored state.
type UpdateCommentRequest struct {
	CommentID       string        `json:"comment_id"`
	ExpectedVersion int64         `json:"expected_version"`
	Body            *string       `json:"body,omitempty"`
	Status          CommentStatus `json:"status,omitempty"`
}

// --- Generated publication seam (N1 defines; R1 implements) -------------------

// GeneratedOutcome marks whether a generated revision became the visible head
// or was retained as a candidate for explicit adoption.
type GeneratedOutcome string

const (
	GeneratedPromoted  GeneratedOutcome = "promoted"
	GeneratedCandidate GeneratedOutcome = "candidate"
)

// GeneratedCommit is the internal workflow-writer input: the snapshot head and
// version captured at report admission, plus the generated revision content.
// No caller-selected origin=generated path exists outside this seam.
type GeneratedCommit struct {
	EntryID        string `json:"entry_id,omitempty"`
	SectionID      string `json:"section_id,omitempty"`
	ReportSeriesID string `json:"report_series_id,omitempty"`
	ReportWindowID string `json:"report_window_id,omitempty"`
	SnapshotHead   string `json:"snapshot_head"`
	SnapshotVer    int64  `json:"snapshot_version"`
	Title          string `json:"title"`
	Markdown       string `json:"markdown"`
	ActorRef       string `json:"actor_ref"`
}

// GeneratedReceipt returns the committed generated revision reference and its
// publication outcome.
type GeneratedReceipt struct {
	EntryID    string           `json:"entry_id"`
	RevisionID string           `json:"revision_id"`
	Version    int64            `json:"version"`
	Outcome    GeneratedOutcome `json:"outcome"`
}

// RequestDigest is the canonical request digest carried inside
// MutationContext: sha-256 over "<kind>\n" + deterministic JSON of every
// semantic request field. Mutable transport metadata is excluded by taking a
// typed request value, never a wire map.
func RequestDigest(kind string, req any) (string, error) {
	raw, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("notebook: encode request digest: %w", err)
	}
	sum := sha256.Sum256(append([]byte(kind+"\n"), raw...))
	return hex.EncodeToString(sum[:]), nil
}
