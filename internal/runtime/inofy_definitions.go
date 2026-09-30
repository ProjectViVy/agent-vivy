package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/ProjectViVy/inofy"
	"github.com/ProjectViVy/inofy/definitions"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
)

// inofyDefinitionRepository adapts Core Storage's WorkflowDefinitionStore to
// INOFY's definitions.Repository contract. Draft rows are bound to one
// author session; published revisions are organism-visible and immutable.
// Storage authority stays with the backend: every method is one storage
// transaction, mirroring the library's CAS/allocation semantics.
type inofyDefinitionRepository struct {
	store   storage.WorkflowDefinitionStore
	session domain.SessionID
}

var _ definitions.Repository = (*inofyDefinitionRepository)(nil)

// inofyDefinitionError maps storage authority errors onto the library's
// typed conflict/not-found/authority envelope (the memory repository uses
// ErrRevisionConflict for both not-found and stale-ETag cases).
func inofyDefinitionError(workflowID string, err error) error {
	switch {
	case errors.Is(err, storage.ErrWorkflowDefinitionNotFound):
		return &inofy.Error{Code: inofy.ErrRevisionConflict, Path: workflowID, Message: "workflow not found"}
	case errors.Is(err, storage.ErrWorkflowDefinitionConflict):
		return &inofy.Error{Code: inofy.ErrRevisionConflict, Path: workflowID, Message: "revision conflict"}
	case errors.Is(err, storage.ErrWorkflowDefinitionAuthor):
		return &inofy.Error{Code: inofy.ErrAuthorityDenied, Path: workflowID, Message: "draft belongs to another author"}
	default:
		return &inofy.Error{Code: inofy.ErrStorageFailed, Path: workflowID, Message: err.Error()}
	}
}

func decodeArtifactRecord(rec storage.WorkflowDraft) (inofy.Artifact, error) {
	var a inofy.Artifact
	if err := json.Unmarshal(rec.ArtifactJSON, &a); err != nil {
		return inofy.Artifact{}, err
	}
	return a, nil
}

func (r *inofyDefinitionRepository) GetDraft(ctx context.Context, workflowID string) (definitions.Draft, error) {
	rec, err := r.store.GetWorkflowDraft(ctx, string(r.session), workflowID)
	if err != nil {
		return definitions.Draft{}, inofyDefinitionError(workflowID, err)
	}
	a, err := decodeArtifactRecord(rec)
	if err != nil {
		return definitions.Draft{}, &inofy.Error{Code: inofy.ErrStorageFailed, Path: workflowID, Message: "stored draft artifact is unreadable"}
	}
	return definitions.Draft{
		WorkflowID: rec.WorkflowID, ETag: rec.ETag, Artifact: a,
		DefinitionDigest: rec.DefinitionDigest, ArtifactDigest: rec.ArtifactDigest,
		Archived: rec.Archived,
	}, nil
}

func (r *inofyDefinitionRepository) UpdateDraftCAS(ctx context.Context, workflowID, expectedETag string, a inofy.Artifact) (definitions.Draft, error) {
	artifactJSON, err := json.Marshal(a)
	if err != nil {
		return definitions.Draft{}, &inofy.Error{Code: inofy.ErrInvalidDefinition, Path: workflowID, Message: err.Error()}
	}
	defDigest, err := inofy.DefinitionDigest(a.Definition)
	if err != nil {
		return definitions.Draft{}, err
	}
	artDigest, err := inofy.ArtifactDigest(a)
	if err != nil {
		return definitions.Draft{}, err
	}
	expected := expectedETag
	switch expectedETag {
	case definitions.ETagAbsent:
		expected = storage.WorkflowDefinitionETagAbsent
	case "":
		// The host contract treats an empty expected ETag as
		// create-or-overwrite-unconditional; storage's "" means
		// "any current draft", so create intent needs the sentinel.
		if _, err := r.store.GetWorkflowDraft(ctx, string(r.session), workflowID); errors.Is(err, storage.ErrWorkflowDefinitionNotFound) {
			expected = storage.WorkflowDefinitionETagAbsent
		}
	}
	rec, err := r.store.UpdateWorkflowDraftCAS(ctx, string(r.session), storage.WorkflowDraftUpdate{
		WorkflowID: workflowID, ExpectedETag: expected, ArtifactJSON: artifactJSON,
		DefinitionDigest: defDigest, ArtifactDigest: artDigest, Now: time.Now().UnixMilli(),
	})
	if err != nil {
		return definitions.Draft{}, inofyDefinitionError(workflowID, err)
	}
	return definitions.Draft{
		WorkflowID: rec.WorkflowID, ETag: rec.ETag, Artifact: a,
		DefinitionDigest: rec.DefinitionDigest, ArtifactDigest: rec.ArtifactDigest,
	}, nil
}

func (r *inofyDefinitionRepository) PublishCAS(ctx context.Context, workflowID, expectedETag string, rev definitions.Revision) (definitions.Revision, error) {
	artifactJSON, err := json.Marshal(rev.Artifact)
	if err != nil {
		return definitions.Revision{}, &inofy.Error{Code: inofy.ErrInvalidDefinition, Path: workflowID, Message: err.Error()}
	}
	implJSON, err := json.Marshal(rev.UsedImplementations)
	if err != nil {
		return definitions.Revision{}, err
	}
	rec, err := r.store.PublishWorkflowRevisionCAS(ctx, string(r.session), workflowID, expectedETag, storage.WorkflowPublishedRevision{
		WorkflowID: workflowID, ArtifactJSON: artifactJSON,
		DefinitionDigest: rev.DefinitionDigest, ArtifactDigest: rev.ArtifactDigest,
		UsedCatalogDigest: rev.UsedCatalogDigest, UsedImplementationsJSON: implJSON,
		PublishedAt: time.Now().UnixMilli(),
	})
	if err != nil {
		return definitions.Revision{}, inofyDefinitionError(workflowID, err)
	}
	var a inofy.Artifact
	if err := json.Unmarshal(rec.ArtifactJSON, &a); err != nil {
		return definitions.Revision{}, &inofy.Error{Code: inofy.ErrStorageFailed, Path: workflowID, Message: "stored revision artifact is unreadable"}
	}
	return definitions.Revision{
		WorkflowID: rec.WorkflowID, Revision: rec.Revision, Artifact: a,
		DefinitionDigest: rec.DefinitionDigest, ArtifactDigest: rec.ArtifactDigest,
		UsedCatalogDigest: rec.UsedCatalogDigest, UsedImplementations: rev.UsedImplementations,
	}, nil
}

func (r *inofyDefinitionRepository) GetRevision(ctx context.Context, workflowID string, revision uint64) (definitions.Revision, error) {
	rec, err := r.store.GetWorkflowPublishedRevision(ctx, workflowID, revision)
	if err != nil {
		return definitions.Revision{}, inofyDefinitionError(workflowID, err)
	}
	var a inofy.Artifact
	if err := json.Unmarshal(rec.ArtifactJSON, &a); err != nil {
		return definitions.Revision{}, &inofy.Error{Code: inofy.ErrStorageFailed, Path: workflowID, Message: "stored revision artifact is unreadable"}
	}
	var used map[string]string
	_ = json.Unmarshal(rec.UsedImplementationsJSON, &used)
	return definitions.Revision{
		WorkflowID: rec.WorkflowID, Revision: rec.Revision, Artifact: a,
		DefinitionDigest: rec.DefinitionDigest, ArtifactDigest: rec.ArtifactDigest,
		UsedCatalogDigest: rec.UsedCatalogDigest, UsedImplementations: used,
	}, nil
}

func (r *inofyDefinitionRepository) List(ctx context.Context, cursor string, limit int) (definitions.Page, error) {
	page, err := r.store.ListWorkflowDefinitions(ctx, cursor, limit)
	if err != nil {
		return definitions.Page{}, &inofy.Error{Code: inofy.ErrStorageFailed, Message: err.Error()}
	}
	out := definitions.Page{NextCursor: page.NextCursor}
	for _, rec := range page.Revisions {
		var a inofy.Artifact
		if err := json.Unmarshal(rec.ArtifactJSON, &a); err != nil {
			return definitions.Page{}, &inofy.Error{Code: inofy.ErrStorageFailed, Path: rec.WorkflowID, Message: "stored revision artifact is unreadable"}
		}
		var used map[string]string
		_ = json.Unmarshal(rec.UsedImplementationsJSON, &used)
		out.Revisions = append(out.Revisions, definitions.Revision{
			WorkflowID: rec.WorkflowID, Revision: rec.Revision, Artifact: a,
			DefinitionDigest: rec.DefinitionDigest, ArtifactDigest: rec.ArtifactDigest,
			UsedCatalogDigest: rec.UsedCatalogDigest, UsedImplementations: used,
		})
	}
	return out, nil
}
