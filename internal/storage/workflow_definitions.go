package storage

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
)

var (
	// ErrWorkflowDefinitionNotFound reports an absent draft or published
	// revision, including a draft outside the caller's author scope.
	ErrWorkflowDefinitionNotFound = errors.New("storage: workflow definition not found")
	// ErrWorkflowDefinitionConflict reports a CAS/ETag mismatch, a
	// create-against-existing draft, or an archived publish.
	ErrWorkflowDefinitionConflict = errors.New("storage: workflow definition revision conflict")
	// ErrWorkflowDefinitionAuthor reports a durable draft owned by a
	// different author session than the caller's bound scope.
	ErrWorkflowDefinitionAuthor = errors.New("storage: workflow definition belongs to another author")
)

// WorkflowDraft is the mutable editing row for one reusable workflow
// definition. ArtifactJSON is the verbatim inofy.Artifact encoding; storage
// stays engine-free and treats it as an opaque validated document.
type WorkflowDraft struct {
	WorkflowID      string
	ETag            string
	ArtifactJSON    []byte
	Archived        bool
	AuthorSessionID string
	CreatedAt       int64
	UpdatedAt       int64
}

// WorkflowPublishedRevision is one immutable publication row. The exact
// used catalog identity is bound at publication time.
type WorkflowPublishedRevision struct {
	WorkflowID              string
	Revision                uint64
	ArtifactJSON            []byte
	UsedImplementationsJSON []byte
	AuthorSessionID         string
	PublishedAt             int64
}

// WorkflowDefinitionSummary is one published workflow listing row: the
// workflow id and its latest published revision.
type WorkflowDefinitionSummary struct {
	WorkflowID string
	Revision   uint64
}

// WorkflowDefinitionPage is a bounded listing result.
type WorkflowDefinitionPage struct {
	Revisions  []WorkflowPublishedRevision
	NextCursor string
}

// WorkflowRunSummary is one product run row joined from the admission
// revisions table.
type WorkflowRunSummary struct {
	RunID        string
	SessionID    string
	Status       string
	DefinitionID string
	Revision     uint64
	CreatedAt    int64
}

// WorkflowRunPage is a bounded product-run listing.
type WorkflowRunPage struct {
	Runs       []WorkflowRunSummary
	NextCursor string
}

// WorkflowDefinitionStore is the Core Storage extension for reusable
// INOFY workflow definitions (S11-F). Every method is one atomic
// transaction; sessionID binds draft rows to their author. Published
// revisions are organism-visible and immutable once committed.
type WorkflowDefinitionStore interface {
	// GetWorkflowDraft returns the caller's draft or
	// ErrWorkflowDefinitionNotFound.
	GetWorkflowDraft(ctx context.Context, sessionID, workflowID string) (WorkflowDraft, error)
	// UpdateWorkflowDraftCAS atomically stores the draft when expectedETag
	// matches the stored ETag (expectedETag == WorkflowDefinitionETagAbsent
	// creates only); an empty expected ETag means "any current draft".
	// Rows owned by another author report ErrWorkflowDefinitionAuthor on
	// update and ErrWorkflowDefinitionNotFound on read paths.
	UpdateWorkflowDraftCAS(ctx context.Context, sessionID string, in WorkflowDraftUpdate) (WorkflowDraft, error)
	// PublishWorkflowRevisionCAS validates the draft ETag inside the same
	// transaction, then deduplicates by (workflow, actual artifact+used implementations):
	// republishing the identical artifact returns the stored revision
	// without allocating. Otherwise the next monotone revision is inserted.
	PublishWorkflowRevisionCAS(ctx context.Context, sessionID, workflowID, expectedETag string, in WorkflowPublishedRevision) (WorkflowPublishedRevision, error)
	// GetWorkflowPublishedRevision reads an immutable published revision.
	GetWorkflowPublishedRevision(ctx context.Context, workflowID string, revision uint64) (WorkflowPublishedRevision, error)
	// ListWorkflowDefinitions returns a bounded page of published revisions
	// ordered by (workflow_id, revision) after cursor "workflow:revision".
	ListWorkflowDefinitions(ctx context.Context, cursor string, limit int) (WorkflowDefinitionPage, error)
	// ListWorkflowDefinitionRuns returns one page of product runs — workflow
	// admissions bound to a published/draft definition — visible to the
	// caller's session, newest first.
	ListWorkflowDefinitionRuns(ctx context.Context, sessionID, cursor string, limit int) (WorkflowRunPage, error)
}

// WorkflowDefinitionETagAbsent is the create-only expected ETag sentinel;
// it mirrors INOFY definitions.ETagAbsent without an engine import.
const WorkflowDefinitionETagAbsent = "\x00absent"

// WorkflowDraftUpdate carries the validated draft write. The backend
// assigns the new ETag.
type WorkflowDraftUpdate struct {
	WorkflowID   string
	ExpectedETag string
	ArtifactJSON []byte
	Now          int64
}

// ValidateWorkflowDraftUpdate enforces the storage invariants of one draft
// write: bounded ids, valid JSON artifact, actual normalized artifact.
func ValidateWorkflowDraftUpdate(in WorkflowDraftUpdate) error {
	if in.WorkflowID == "" || len(in.WorkflowID) > 256 || len(in.ExpectedETag) > 256 {
		return errors.New("storage: workflow definition identity is incomplete")
	}
	if len(in.ArtifactJSON) == 0 || len(in.ArtifactJSON) > 1<<20 || !json.Valid(in.ArtifactJSON) {
		return errors.New("storage: workflow definition artifact is invalid or oversized")
	}

	if in.Now <= 0 {
		return errors.New("storage: workflow definition write requires a timestamp")
	}
	return nil
}

// ValidateWorkflowPublishedRevision enforces the identity of one
// publication insert. Dedup is by (workflow, actual artifact+used implementations), so
// the used implementation map are mandatory.
func ValidateWorkflowPublishedRevision(in WorkflowPublishedRevision) error {
	if in.WorkflowID == "" || len(in.WorkflowID) > 256 || in.AuthorSessionID == "" {
		return errors.New("storage: workflow publication identity is incomplete")
	}
	if len(in.ArtifactJSON) == 0 || len(in.ArtifactJSON) > 1<<20 || !json.Valid(in.ArtifactJSON) {
		return errors.New("storage: workflow publication artifact is invalid or oversized")
	}

	if len(in.UsedImplementationsJSON) == 0 || !json.Valid(in.UsedImplementationsJSON) {
		return errors.New("storage: workflow publication used-implementations are invalid")
	}
	if in.PublishedAt <= 0 {
		return errors.New("storage: workflow publication requires a timestamp")
	}
	return nil
}

// WorkflowDefinitionETag derives a deterministic next draft ETag. Host
// adapters keep one monotone counter per row in updated_at space; callers
// never hand-pick ETags.
func WorkflowDefinitionETag(workflowID string, seq int64) string {
	return itoaBase10(seq)
}

func itoaBase10(n int64) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}

// NormalizeWorkflowDocument preserves exact JSON numbers and canonical object
// ordering for artifact/publication comparisons in the storage owner.
func NormalizeWorkflowDocument(raw []byte) ([]byte, error) {
	var value any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}
	return json.Marshal(value)
}
