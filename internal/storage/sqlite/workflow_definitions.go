package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"agent-vivy/internal/storage"
)

var _ storage.WorkflowDefinitionStore = (*Backend)(nil)

func (b *Backend) GetWorkflowDraft(ctx context.Context, sessionID, workflowID string) (storage.WorkflowDraft, error) {
	return sqliteReadWorkflowDraft(ctx, b.db, sessionID, workflowID)
}

func sqliteReadWorkflowDraft(ctx context.Context, q sqliteWorkflowQueryer, sessionID, workflowID string) (storage.WorkflowDraft, error) {
	var d storage.WorkflowDraft
	var archived int
	err := q.QueryRowContext(ctx, `SELECT workflow_id,etag,artifact,definition_digest,artifact_digest,archived,author_session_id,created_at,updated_at
		FROM workflow_definition_drafts WHERE workflow_id = ?`, workflowID).
		Scan(&d.WorkflowID, &d.ETag, &d.ArtifactJSON, &d.DefinitionDigest, &d.ArtifactDigest,
			&archived, &d.AuthorSessionID, &d.CreatedAt, &d.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return storage.WorkflowDraft{}, storage.ErrWorkflowDefinitionNotFound
	}
	if err != nil {
		return storage.WorkflowDraft{}, fmt.Errorf("sqlite: read workflow draft: %w", err)
	}
	d.Archived = archived != 0
	// Author scope is enforced by the backend, not the caller: a draft owned
	// by another session reads as not-found.
	if d.AuthorSessionID != sessionID {
		return storage.WorkflowDraft{}, storage.ErrWorkflowDefinitionNotFound
	}
	return d, nil
}

func (b *Backend) UpdateWorkflowDraftCAS(ctx context.Context, sessionID string, in storage.WorkflowDraftUpdate) (storage.WorkflowDraft, error) {
	if err := storage.ValidateWorkflowDraftUpdate(in); err != nil {
		return storage.WorkflowDraft{}, err
	}
	if sessionID == "" {
		return storage.WorkflowDraft{}, storage.ErrWorkflowDefinitionNotFound
	}
	tx, err := b.db.BeginTx(ctx, nil)
	if err != nil {
		return storage.WorkflowDraft{}, fmt.Errorf("sqlite: begin draft CAS: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	// Serialize draft writes per workflow: sqlite's single writer plus an
	// explicit session lease is unnecessary; the CAS predicates carry the
	// conflict semantics and BEGIN IMMEDIATE is the writer lock.
	existing, readErr := sqliteReadDraftAnyAuthor(ctx, tx, in.WorkflowID)
	if readErr != nil && !errors.Is(readErr, storage.ErrWorkflowDefinitionNotFound) {
		return storage.WorkflowDraft{}, readErr
	}
	found := readErr == nil
	switch {
	case in.ExpectedETag == storage.WorkflowDefinitionETagAbsent && found:
		return storage.WorkflowDraft{}, storage.ErrWorkflowDefinitionConflict
	case in.ExpectedETag != storage.WorkflowDefinitionETagAbsent && !found:
		return storage.WorkflowDraft{}, storage.ErrWorkflowDefinitionNotFound
	case found && existing.AuthorSessionID != sessionID:
		return storage.WorkflowDraft{}, storage.ErrWorkflowDefinitionAuthor
	case found && in.ExpectedETag != "" && existing.ETag != in.ExpectedETag:
		return storage.WorkflowDraft{}, storage.ErrWorkflowDefinitionConflict
	}
	seq := int64(1)
	if found {
		seq = existing.UpdatedAt + 1
		if existing.CreatedAt >= in.Now {
			seq = existing.CreatedAt + 1
		}
	}
	etag := storage.WorkflowDefinitionETag(in.WorkflowID, seq)
	if found {
		res, err := tx.ExecContext(ctx, `UPDATE workflow_definition_drafts
			SET etag=?, artifact=?, definition_digest=?, artifact_digest=?, updated_at=?
			WHERE workflow_id=? AND etag=? AND author_session_id=?`,
			etag, in.ArtifactJSON, in.DefinitionDigest, in.ArtifactDigest, in.Now,
			in.WorkflowID, existing.ETag, sessionID)
		if err != nil {
			return storage.WorkflowDraft{}, fmt.Errorf("sqlite: update workflow draft: %w", err)
		}
		if n, _ := res.RowsAffected(); n != 1 {
			return storage.WorkflowDraft{}, storage.ErrWorkflowDefinitionConflict
		}
	} else {
		if _, err := tx.ExecContext(ctx, `INSERT INTO workflow_definition_drafts
			(workflow_id,etag,artifact,definition_digest,artifact_digest,archived,author_session_id,created_at,updated_at)
			VALUES (?,?,?,?,?,0,?,?,?)`,
			in.WorkflowID, etag, in.ArtifactJSON, in.DefinitionDigest, in.ArtifactDigest,
			sessionID, in.Now, in.Now); err != nil {
			return storage.WorkflowDraft{}, fmt.Errorf("sqlite: insert workflow draft: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return storage.WorkflowDraft{}, fmt.Errorf("sqlite: commit draft CAS: %w", err)
	}
	created := in.Now
	if found {
		created = existing.CreatedAt
	}
	return storage.WorkflowDraft{
		WorkflowID: in.WorkflowID, ETag: etag, ArtifactJSON: append([]byte(nil), in.ArtifactJSON...),
		DefinitionDigest: in.DefinitionDigest, ArtifactDigest: in.ArtifactDigest,
		AuthorSessionID: sessionID, CreatedAt: created, UpdatedAt: in.Now,
	}, nil
}

func sqliteReadDraftAnyAuthor(ctx context.Context, q sqliteWorkflowQueryer, workflowID string) (storage.WorkflowDraft, error) {
	var d storage.WorkflowDraft
	var archived int
	err := q.QueryRowContext(ctx, `SELECT workflow_id,etag,artifact,definition_digest,artifact_digest,archived,author_session_id,created_at,updated_at
		FROM workflow_definition_drafts WHERE workflow_id = ?`, workflowID).
		Scan(&d.WorkflowID, &d.ETag, &d.ArtifactJSON, &d.DefinitionDigest, &d.ArtifactDigest,
			&archived, &d.AuthorSessionID, &d.CreatedAt, &d.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return storage.WorkflowDraft{}, storage.ErrWorkflowDefinitionNotFound
	}
	if err != nil {
		return storage.WorkflowDraft{}, fmt.Errorf("sqlite: read workflow draft: %w", err)
	}
	d.Archived = archived != 0
	return d, nil
}

func (b *Backend) PublishWorkflowRevisionCAS(ctx context.Context, sessionID, workflowID, expectedETag string, in storage.WorkflowPublishedRevision) (storage.WorkflowPublishedRevision, error) {
	if in.WorkflowID != workflowID {
		return storage.WorkflowPublishedRevision{}, storage.ErrWorkflowDefinitionConflict
	}
	in.AuthorSessionID = sessionID
	if err := storage.ValidateWorkflowPublishedRevision(in); err != nil {
		return storage.WorkflowPublishedRevision{}, err
	}
	tx, err := b.db.BeginTx(ctx, nil)
	if err != nil {
		return storage.WorkflowPublishedRevision{}, fmt.Errorf("sqlite: begin publish CAS: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	draft, err := sqliteReadDraftAnyAuthor(ctx, tx, workflowID)
	if errors.Is(err, storage.ErrWorkflowDefinitionNotFound) {
		return storage.WorkflowPublishedRevision{}, err
	}
	if err != nil {
		return storage.WorkflowPublishedRevision{}, err
	}
	if draft.AuthorSessionID != sessionID {
		return storage.WorkflowPublishedRevision{}, storage.ErrWorkflowDefinitionAuthor
	}
	if draft.Archived {
		return storage.WorkflowPublishedRevision{}, storage.ErrWorkflowDefinitionConflict
	}
	if draft.ETag != expectedETag {
		return storage.WorkflowPublishedRevision{}, storage.ErrWorkflowDefinitionConflict
	}
	// Same-artifact republication dedups to the stored revision.
	var existing storage.WorkflowPublishedRevision
	var implJSON []byte
	err = tx.QueryRowContext(ctx, `SELECT workflow_id,revision,artifact,definition_digest,artifact_digest,used_catalog_digest,used_implementations,author_session_id,published_at
		FROM workflow_definition_revisions
		WHERE workflow_id=? AND artifact_digest=? AND used_catalog_digest=?`,
		workflowID, in.ArtifactDigest, in.UsedCatalogDigest).
		Scan(&existing.WorkflowID, &existing.Revision, &existing.ArtifactJSON, &existing.DefinitionDigest,
			&existing.ArtifactDigest, &existing.UsedCatalogDigest, &implJSON, &existing.AuthorSessionID, &existing.PublishedAt)
	if err == nil {
		existing.UsedImplementationsJSON = implJSON
		if err := tx.Commit(); err != nil {
			return storage.WorkflowPublishedRevision{}, fmt.Errorf("sqlite: commit publish dedup: %w", err)
		}
		return existing, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return storage.WorkflowPublishedRevision{}, fmt.Errorf("sqlite: read publish dedup: %w", err)
	}
	var next uint64
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(revision),0)+1 FROM workflow_definition_revisions WHERE workflow_id=?`,
		workflowID).Scan(&next); err != nil {
		return storage.WorkflowPublishedRevision{}, fmt.Errorf("sqlite: allocate revision: %w", err)
	}
	in.Revision = next
	if _, err := tx.ExecContext(ctx, `INSERT INTO workflow_definition_revisions
		(workflow_id,revision,artifact,definition_digest,artifact_digest,used_catalog_digest,used_implementations,author_session_id,published_at)
		VALUES (?,?,?,?,?,?,?,?,?)`,
		workflowID, next, in.ArtifactJSON, in.DefinitionDigest, in.ArtifactDigest,
		in.UsedCatalogDigest, in.UsedImplementationsJSON, sessionID, in.PublishedAt); err != nil {
		return storage.WorkflowPublishedRevision{}, fmt.Errorf("sqlite: insert published revision: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return storage.WorkflowPublishedRevision{}, fmt.Errorf("sqlite: commit publish CAS: %w", err)
	}
	return in, nil
}

func (b *Backend) GetWorkflowPublishedRevision(ctx context.Context, workflowID string, revision uint64) (storage.WorkflowPublishedRevision, error) {
	var r storage.WorkflowPublishedRevision
	err := b.db.QueryRowContext(ctx, `SELECT workflow_id,revision,artifact,definition_digest,artifact_digest,used_catalog_digest,used_implementations,author_session_id,published_at
		FROM workflow_definition_revisions WHERE workflow_id=? AND revision=?`, workflowID, revision).
		Scan(&r.WorkflowID, &r.Revision, &r.ArtifactJSON, &r.DefinitionDigest, &r.ArtifactDigest,
			&r.UsedCatalogDigest, &r.UsedImplementationsJSON, &r.AuthorSessionID, &r.PublishedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return storage.WorkflowPublishedRevision{}, storage.ErrWorkflowDefinitionNotFound
	}
	if err != nil {
		return storage.WorkflowPublishedRevision{}, fmt.Errorf("sqlite: read published revision: %w", err)
	}
	return r, nil
}

func (b *Backend) ListWorkflowDefinitions(ctx context.Context, cursor string, limit int) (storage.WorkflowDefinitionPage, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	var curWf string
	var curRev int64
	if cursor != "" {
		parts := strings.SplitN(cursor, ":", 2)
		if len(parts) != 2 {
			return storage.WorkflowDefinitionPage{}, fmt.Errorf("sqlite: invalid workflow definition cursor")
		}
		v, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil {
			return storage.WorkflowDefinitionPage{}, fmt.Errorf("sqlite: invalid workflow definition cursor")
		}
		curWf, curRev = parts[0], v
	}
	rows, err := b.db.QueryContext(ctx, `SELECT workflow_id,revision,artifact,definition_digest,artifact_digest,used_catalog_digest,used_implementations,author_session_id,published_at
		FROM workflow_definition_revisions
		WHERE (workflow_id > ?) OR (workflow_id = ? AND revision > ?)
		ORDER BY workflow_id, revision LIMIT ?`, curWf, curWf, curRev, limit+1)
	if err != nil {
		return storage.WorkflowDefinitionPage{}, fmt.Errorf("sqlite: list workflow definitions: %w", err)
	}
	defer func() { _ = rows.Close() }()
	page := storage.WorkflowDefinitionPage{}
	for rows.Next() {
		var r storage.WorkflowPublishedRevision
		if err := rows.Scan(&r.WorkflowID, &r.Revision, &r.ArtifactJSON, &r.DefinitionDigest, &r.ArtifactDigest,
			&r.UsedCatalogDigest, &r.UsedImplementationsJSON, &r.AuthorSessionID, &r.PublishedAt); err != nil {
			return page, fmt.Errorf("sqlite: scan workflow definition: %w", err)
		}
		page.Revisions = append(page.Revisions, r)
	}
	if err := rows.Err(); err != nil {
		return page, fmt.Errorf("sqlite: list workflow definitions: %w", err)
	}
	if len(page.Revisions) > limit {
		page.Revisions = page.Revisions[:limit]
		last := page.Revisions[len(page.Revisions)-1]
		page.NextCursor = last.WorkflowID + ":" + strconv.FormatUint(last.Revision, 10)
	}
	return page, nil
}

func (b *Backend) ListWorkflowDefinitionRuns(ctx context.Context, sessionID, cursor string, limit int) (storage.WorkflowRunPage, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	var after int64
	if cursor != "" {
		v, err := strconv.ParseInt(cursor, 10, 64)
		if err != nil {
			return storage.WorkflowRunPage{}, fmt.Errorf("sqlite: invalid workflow run cursor")
		}
		after = v
	}
	rows, err := b.db.QueryContext(ctx, `SELECT r.workflow_run_id, r.parent_session_id, r.definition_id, r.definition_revision, r.created_at, ru.status
		FROM workflow_revisions r JOIN runs ru ON ru.id = r.workflow_run_id
		WHERE r.definition_id IS NOT NULL AND r.parent_session_id = ? AND (? = 0 OR r.created_at < ?)
		ORDER BY r.created_at DESC, r.workflow_run_id LIMIT ?`, sessionID, after, after, limit+1)
	if err != nil {
		return storage.WorkflowRunPage{}, fmt.Errorf("sqlite: list workflow definition runs: %w", err)
	}
	defer func() { _ = rows.Close() }()
	page := storage.WorkflowRunPage{}
	for rows.Next() {
		var row storage.WorkflowRunSummary
		var rev sql.NullInt64
		if err := rows.Scan(&row.RunID, &row.SessionID, &row.DefinitionID, &rev, &row.CreatedAt, &row.Status); err != nil {
			return page, fmt.Errorf("sqlite: scan workflow run: %w", err)
		}
		row.Revision = uint64(rev.Int64)
		page.Runs = append(page.Runs, row)
	}
	if err := rows.Err(); err != nil {
		return page, fmt.Errorf("sqlite: list workflow runs: %w", err)
	}
	if len(page.Runs) > limit {
		page.Runs = page.Runs[:limit]
		page.NextCursor = strconv.FormatInt(page.Runs[len(page.Runs)-1].CreatedAt, 10)
	}
	return page, nil
}
