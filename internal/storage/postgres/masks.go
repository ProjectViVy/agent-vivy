package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"agent-vivy/internal/domain"
	mask "agent-vivy/internal/maskcontract"
	"agent-vivy/internal/storage"
)

var _ storage.MaskStore = (*Backend)(nil)

func (b *Backend) ListCustomMasks(ctx context.Context, in mask.ListRequest) (mask.Page, error) {
	in, err := mask.NormalizeList(in)
	if err != nil {
		return mask.Page{}, err
	}
	rows, err := b.db.QueryContext(ctx, `
		SELECT m.id, m.name, m.description, m.digest, m.revision,
		       COALESCE(bm.install_id, ''), COALESCE(bm.bundle_id, ''), COALESCE(bm.local_id, ''),
		       COALESCE(bm.release_digest, ''), COALESCE(bm.release_sequence, 0), bm.recommended_skills_json
		FROM mask_definitions AS m
		LEFT JOIN mask_bundle_metadata AS bm ON bm.mask_id = m.id
		WHERE m.id > ?
		ORDER BY m.id
		LIMIT ?`, in.AfterID, in.Limit+1)
	if err != nil {
		return mask.Page{}, maskStorageError("list custom masks", err)
	}
	defer func() { _ = rows.Close() }()

	items := make([]mask.Metadata, 0, in.Limit)
	for rows.Next() {
		var item mask.Metadata
		var installID, bundleID, localID, releaseDigest string
		var releaseSequence int64
		var recommendationsJSON []byte
		if err := rows.Scan(&item.ID, &item.Name, &item.Description, &item.Digest, &item.Revision,
			&installID, &bundleID, &localID, &releaseDigest, &releaseSequence, &recommendationsJSON); err != nil {
			return mask.Page{}, maskStorageError("scan custom mask", err)
		}
		item.BuiltIn = false
		item.Origin = mask.OriginCustom
		item.Available = true
		item.Bundle, item.RecommendedSkills, err = storage.DecodeMaskBundleMetadata(installID, bundleID, localID, releaseDigest, releaseSequence, recommendationsJSON)
		if err != nil {
			return mask.Page{}, maskStorageError("decode custom Mask provenance", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return mask.Page{}, maskStorageError("iterate custom masks", err)
	}
	page := mask.Page{Items: items}
	if len(items) > in.Limit {
		page.NextAfterID = items[in.Limit-1].ID
		page.Items = items[:in.Limit]
	}
	return page, nil
}

func (b *Backend) GetCustomMask(ctx context.Context, id string) (mask.Definition, error) {
	if err := mask.ValidateCustomID(id); err != nil {
		return mask.Definition{}, mask.NewError(mask.CodeInvalidMask, err)
	}
	return b.getCustomMask(ctx, b.db, id)
}

func (b *Backend) CreateCustomMask(ctx context.Context, in mask.CreateRequest) (mask.Definition, error) {
	in, err := mask.NormalizeCreate(in)
	if err != nil {
		return mask.Definition{}, err
	}
	requestDigest := mask.CreateRequestDigest(in)
	tx, err := b.db.BeginTx(ctx, nil)
	if err != nil {
		return mask.Definition{}, maskStorageError("begin create custom mask", err)
	}
	defer func() { _ = tx.Rollback() }()

	if existing, found, err := postgresFindMaskByOperation(ctx, tx, in.OperationID); err != nil {
		return mask.Definition{}, err
	} else if found {
		if existing.requestDigest != requestDigest {
			return mask.Definition{}, mask.NewError(mask.CodeRevisionConflict, nil)
		}
		return existing.definition, nil
	}

	id := "custom/" + uuid.NewString()
	now := time.Now().UnixMilli()
	digest := mask.DefinitionDigest(id, in.Name, in.Body)
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO mask_definitions
			(id, name, description, body, revision, digest, created_at, updated_at, create_operation_id, create_request_digest)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (create_operation_id) DO NOTHING`,
		id, in.Name, in.Description, []byte(in.Body), 1, digest, now, now, in.OperationID, requestDigest); err != nil {
		return mask.Definition{}, maskStorageError("insert custom mask", err)
	}

	created, found, err := postgresFindMaskByOperation(ctx, tx, in.OperationID)
	if err != nil {
		return mask.Definition{}, err
	}
	if !found {
		return mask.Definition{}, maskStorageError("read inserted custom mask", errors.New("operation row missing after insert"))
	}
	if created.requestDigest != requestDigest {
		return mask.Definition{}, mask.NewError(mask.CodeRevisionConflict, nil)
	}
	if in.Bundle != nil {
		recommendations, marshalErr := json.Marshal(in.RecommendedSkills)
		if marshalErr != nil {
			return mask.Definition{}, maskStorageError("encode custom Mask recommendations", marshalErr)
		}
		provenance := in.Bundle
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO mask_bundle_metadata
				(mask_id, install_id, bundle_id, local_id, release_digest, release_sequence, recommended_skills_json)
			VALUES (?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT (mask_id) DO NOTHING`, created.definition.ID, provenance.Release.InstallID, provenance.Release.BundleID,
			provenance.LocalID, provenance.Release.ReleaseDigest, provenance.Release.ReleaseSequence, recommendations); err != nil {
			return mask.Definition{}, maskStorageError("persist custom Mask provenance", err)
		}
		created, found, err = postgresFindMaskByOperation(ctx, tx, in.OperationID)
		if err != nil || !found {
			if err == nil {
				err = errors.New("custom Mask operation row missing after provenance insert")
			}
			return mask.Definition{}, maskStorageError("read copied Mask provenance", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return mask.Definition{}, maskStorageError("commit create custom mask", err)
	}
	return created.definition, nil
}

func (b *Backend) UpdateCustomMask(ctx context.Context, in mask.UpdateRequest) (mask.Definition, error) {
	in, err := mask.NormalizeUpdate(in)
	if err != nil {
		return mask.Definition{}, err
	}
	tx, err := b.db.BeginTx(ctx, nil)
	if err != nil {
		return mask.Definition{}, maskStorageError("begin update custom mask", err)
	}
	defer func() { _ = tx.Rollback() }()

	current, err := b.getCustomMaskForUpdate(ctx, tx, in.ID)
	if err != nil {
		return mask.Definition{}, err
	}
	if current.Revision != in.ExpectedRevision {
		return mask.Definition{}, mask.NewRevisionError(mask.CodeRevisionConflict, current.Revision, nil)
	}
	nextRevision := current.Revision + 1
	digest := mask.DefinitionDigest(in.ID, in.Name, in.Body)
	now := time.Now().UnixMilli()
	result, err := tx.ExecContext(ctx, `
		UPDATE mask_definitions
		SET name = ?, description = ?, body = ?, revision = ?, digest = ?, updated_at = ?
		WHERE id = ? AND revision = ?`,
		in.Name, in.Description, []byte(in.Body), nextRevision, digest, now, in.ID, in.ExpectedRevision)
	if err != nil {
		return mask.Definition{}, maskStorageError("update custom mask", err)
	}
	if affected, err := result.RowsAffected(); err != nil {
		return mask.Definition{}, maskStorageError("update custom mask rows", err)
	} else if affected == 0 {
		return mask.Definition{}, mask.NewRevisionError(mask.CodeRevisionConflict, current.Revision, nil)
	}
	updated, err := b.getCustomMask(ctx, tx, in.ID)
	if err != nil {
		return mask.Definition{}, err
	}
	if err := tx.Commit(); err != nil {
		return mask.Definition{}, maskStorageError("commit update custom mask", err)
	}
	return updated, nil
}

func (b *Backend) DeleteCustomMask(ctx context.Context, in mask.DeleteRequest) (mask.DeleteResult, error) {
	in, err := mask.NormalizeDelete(in)
	if err != nil {
		return mask.DeleteResult{}, err
	}
	tx, err := b.db.BeginTx(ctx, nil)
	if err != nil {
		return mask.DeleteResult{}, maskStorageError("begin delete custom mask", err)
	}
	defer func() { _ = tx.Rollback() }()

	deleted, err := b.deleteCustomMaskTx(ctx, tx, in)
	if err != nil {
		return mask.DeleteResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return mask.DeleteResult{}, maskStorageError("commit delete custom mask", err)
	}
	return deleted, nil
}

// deleteCustomMaskTx exposes the production lock/delete sequence at the
// transaction boundary used by dedicated-connection ordering tests.
func (b *Backend) deleteCustomMaskTx(ctx context.Context, tx *Tx, in mask.DeleteRequest) (mask.DeleteResult, error) {
	current, err := b.getCustomMaskForUpdate(ctx, tx, in.ID)
	if err != nil {
		return mask.DeleteResult{}, err
	}
	if current.Revision != in.ExpectedRevision {
		return mask.DeleteResult{}, mask.NewRevisionError(mask.CodeRevisionConflict, current.Revision, nil)
	}
	var references int
	if err := tx.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM session_mask_selections WHERE mask_id = ?`, in.ID).Scan(&references); err != nil {
		return mask.DeleteResult{}, maskStorageError("count mask references", err)
	}
	if references > 0 {
		return mask.DeleteResult{}, mask.NewReferenceError(mask.CodeMaskInUse, references, nil)
	}
	result, err := tx.ExecContext(ctx,
		`DELETE FROM mask_definitions WHERE id = ? AND revision = ?`, in.ID, in.ExpectedRevision)
	if err != nil {
		return mask.DeleteResult{}, maskStorageError("delete custom mask", err)
	}
	if affected, err := result.RowsAffected(); err != nil {
		return mask.DeleteResult{}, maskStorageError("delete custom mask rows", err)
	} else if affected == 0 {
		return mask.DeleteResult{}, mask.NewRevisionError(mask.CodeRevisionConflict, current.Revision, nil)
	}
	return mask.DeleteResult{ID: in.ID}, nil
}

func (b *Backend) ReadMaskCapture(ctx context.Context, sessionID domain.SessionID) (mask.Capture, error) {
	if err := mask.ValidateSessionID(sessionID); err != nil {
		return mask.Capture{}, mask.NewError(mask.CodeInvalidMask, err)
	}
	selection := mask.Selection{SessionID: sessionID}
	var definitionID, name, digest sql.NullString
	var body []byte
	var definitionRevision sql.NullInt64
	var originInstallID, originBundleID, originLocalID, originDigest string
	var originReleaseSequence int64
	var recommendationsJSON []byte
	err := b.db.QueryRowContext(ctx, `
		SELECT COALESCE(sel.mask_id, ''), COALESCE(sel.revision, 0),
		       def.id, def.name, def.body, def.revision, def.digest,
		       COALESCE(bm.install_id, ''), COALESCE(bm.bundle_id, ''), COALESCE(bm.local_id, ''),
		       COALESCE(bm.release_digest, ''), COALESCE(bm.release_sequence, 0), bm.recommended_skills_json
		FROM sessions AS s
		LEFT JOIN session_mask_selections AS sel ON sel.session_id = s.id
		LEFT JOIN mask_definitions AS def ON def.id = sel.mask_id
		LEFT JOIN mask_bundle_metadata AS bm ON bm.mask_id = def.id
		WHERE s.id = ?`, sessionID).
		Scan(&selection.MaskID, &selection.Revision, &definitionID, &name, &body, &definitionRevision, &digest,
			&originInstallID, &originBundleID, &originLocalID, &originDigest, &originReleaseSequence, &recommendationsJSON)
	if errors.Is(err, sql.ErrNoRows) {
		return mask.Capture{}, mask.NewError(mask.CodeNotFound, storage.ErrNotFound)
	}
	if err != nil {
		return mask.Capture{}, maskStorageError("read mask capture", err)
	}
	capture := mask.Capture{Selection: selection}
	if selection.MaskID != "" && !mask.IsBuiltinID(selection.MaskID) && !mask.IsBundleID(selection.MaskID) {
		if !definitionID.Valid || !name.Valid || !definitionRevision.Valid || !digest.Valid {
			return mask.Capture{}, mask.NewError(mask.CodeNotFound, storage.ErrNotFound)
		}
		capture.Mask = &mask.Snapshot{
			ID: definitionID.String, Name: name.String, Body: string(body),
			Digest: digest.String, DefinitionRevision: definitionRevision.Int64,
			SelectionRevision: selection.Revision,
		}
		capture.Mask.Origin = mask.OriginCustom
		capture.Mask.Bundle, capture.Mask.RecommendedSkills, err = storage.DecodeMaskBundleMetadata(originInstallID, originBundleID, originLocalID, originDigest, originReleaseSequence, recommendationsJSON)
		if err != nil {
			return mask.Capture{}, maskStorageError("decode captured Mask provenance", err)
		}
	}
	return capture, nil
}

func (b *Backend) SetMaskSelection(ctx context.Context, in mask.SetSelectionRequest) (mask.Selection, error) {
	in, err := mask.NormalizeSelection(in)
	if err != nil {
		return mask.Selection{}, err
	}
	tx, err := b.db.BeginTx(ctx, nil)
	if err != nil {
		return mask.Selection{}, maskStorageError("begin set mask selection", err)
	}
	defer func() { _ = tx.Rollback() }()
	next, err := b.setMaskSelectionTx(ctx, tx, in)
	if err != nil {
		return mask.Selection{}, err
	}
	if err := tx.Commit(); err != nil {
		return mask.Selection{}, maskStorageError("commit mask selection", err)
	}
	return next, nil
}

// setMaskSelectionTx contains the production lock/CAS sequence so
// dedicated-connection tests can hold its commit boundary open without
// replacing the database with a mock.
func (b *Backend) setMaskSelectionTx(ctx context.Context, tx *Tx, in mask.SetSelectionRequest) (mask.Selection, error) {
	var sessionID string
	if err := tx.QueryRowContext(ctx,
		`SELECT id FROM sessions WHERE id = ? FOR UPDATE`, in.SessionID).Scan(&sessionID); errors.Is(err, sql.ErrNoRows) {
		return mask.Selection{}, mask.NewError(mask.CodeNotFound, storage.ErrNotFound)
	} else if err != nil {
		return mask.Selection{}, maskStorageError("lock mask selection session", err)
	}
	current, err := postgresReadSelection(ctx, tx, in.SessionID)
	if err != nil {
		return mask.Selection{}, err
	}
	if current.Revision != in.ExpectedRevision {
		return mask.Selection{}, mask.NewRevisionError(mask.CodeRevisionConflict, current.Revision, nil)
	}
	if mask.IsBundleID(in.MaskID) {
		var currentEpoch int64
		if err := tx.QueryRowContext(ctx, `SELECT epoch FROM bundle_catalog_state WHERE id = 1 FOR UPDATE`).Scan(&currentEpoch); err != nil {
			return mask.Selection{}, maskStorageError("read toolkit catalog epoch for mask selection", err)
		}
		if currentEpoch != in.ExpectedCatalogEpoch {
			return mask.Selection{}, mask.NewRevisionError(mask.CodeRevisionConflict, currentEpoch, nil)
		}
	} else if in.MaskID != "" && !mask.IsBuiltinID(in.MaskID) {
		if _, err := b.getCustomMaskForUpdate(ctx, tx, in.MaskID); err != nil {
			return mask.Selection{}, err
		}
	}
	next := mask.Selection{SessionID: in.SessionID, MaskID: in.MaskID, Revision: current.Revision + 1}
	if current.Revision == 0 {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO session_mask_selections (session_id, mask_id, revision) VALUES (?, ?, ?)`,
			in.SessionID, in.MaskID, next.Revision); err != nil {
			return mask.Selection{}, maskStorageError("insert mask selection", err)
		}
	} else {
		result, err := tx.ExecContext(ctx,
			`UPDATE session_mask_selections SET mask_id = ?, revision = ? WHERE session_id = ? AND revision = ?`,
			in.MaskID, next.Revision, in.SessionID, in.ExpectedRevision)
		if err != nil {
			return mask.Selection{}, maskStorageError("update mask selection", err)
		}
		if affected, err := result.RowsAffected(); err != nil {
			return mask.Selection{}, maskStorageError("update mask selection rows", err)
		} else if affected == 0 {
			return mask.Selection{}, mask.NewRevisionError(mask.CodeRevisionConflict, current.Revision, nil)
		}
	}
	return next, nil
}

type postgresOperationResult struct {
	definition    mask.Definition
	requestDigest string
}

type postgresQueryer interface {
	QueryRowContext(context.Context, string, ...any) *Row
}

func postgresFindMaskByOperation(ctx context.Context, q postgresQueryer, operationID string) (postgresOperationResult, bool, error) {
	var result postgresOperationResult
	var installID, bundleID, localID, releaseDigest string
	var releaseSequence int64
	var recommendationsJSON []byte
	err := q.QueryRowContext(ctx, `
		SELECT m.id, m.name, m.description, m.body, m.revision, m.digest, m.create_request_digest,
		       COALESCE(bm.install_id, ''), COALESCE(bm.bundle_id, ''), COALESCE(bm.local_id, ''),
		       COALESCE(bm.release_digest, ''), COALESCE(bm.release_sequence, 0), bm.recommended_skills_json
		FROM mask_definitions AS m
		LEFT JOIN mask_bundle_metadata AS bm ON bm.mask_id = m.id
		WHERE m.create_operation_id = ?`, operationID).
		Scan(&result.definition.ID, &result.definition.Name, &result.definition.Description, &result.definition.Body,
			&result.definition.Revision, &result.definition.Digest, &result.requestDigest,
			&installID, &bundleID, &localID, &releaseDigest, &releaseSequence, &recommendationsJSON)
	if errors.Is(err, sql.ErrNoRows) {
		return postgresOperationResult{}, false, nil
	}
	if err != nil {
		return postgresOperationResult{}, false, maskStorageError("find mask operation", err)
	}
	result.definition.BuiltIn = false
	result.definition.Origin = mask.OriginCustom
	result.definition.Bundle, result.definition.RecommendedSkills, err = storage.DecodeMaskBundleMetadata(installID, bundleID, localID, releaseDigest, releaseSequence, recommendationsJSON)
	if err != nil {
		return postgresOperationResult{}, false, maskStorageError("decode custom Mask provenance", err)
	}
	return result, true, nil
}

func postgresReadSelection(ctx context.Context, q postgresQueryer, sessionID domain.SessionID) (mask.Selection, error) {
	selection := mask.Selection{SessionID: sessionID}
	err := q.QueryRowContext(ctx,
		`SELECT mask_id, revision FROM session_mask_selections WHERE session_id = ?`, sessionID).
		Scan(&selection.MaskID, &selection.Revision)
	if errors.Is(err, sql.ErrNoRows) {
		return selection, nil
	}
	if err != nil {
		return mask.Selection{}, maskStorageError("read mask selection", err)
	}
	return selection, nil
}

func (b *Backend) getCustomMask(ctx context.Context, q postgresQueryer, id string) (mask.Definition, error) {
	return b.getCustomMaskQuery(ctx, q, id, false)
}

func (b *Backend) getCustomMaskForUpdate(ctx context.Context, q postgresQueryer, id string) (mask.Definition, error) {
	return b.getCustomMaskQuery(ctx, q, id, true)
}

func (b *Backend) getCustomMaskQuery(ctx context.Context, q postgresQueryer, id string, forUpdate bool) (mask.Definition, error) {
	var definition mask.Definition
	var installID, bundleID, localID, releaseDigest string
	var releaseSequence int64
	var recommendationsJSON []byte
	query := `
		SELECT m.id, m.name, m.description, m.body, m.revision, m.digest,
		       COALESCE(bm.install_id, ''), COALESCE(bm.bundle_id, ''), COALESCE(bm.local_id, ''),
		       COALESCE(bm.release_digest, ''), COALESCE(bm.release_sequence, 0), bm.recommended_skills_json
		FROM mask_definitions AS m
		LEFT JOIN mask_bundle_metadata AS bm ON bm.mask_id = m.id
		WHERE m.id = ?`
	if forUpdate {
		query += ` FOR UPDATE OF m`
	}
	err := q.QueryRowContext(ctx, query, id).
		Scan(&definition.ID, &definition.Name, &definition.Description, &definition.Body, &definition.Revision, &definition.Digest,
			&installID, &bundleID, &localID, &releaseDigest, &releaseSequence, &recommendationsJSON)
	if errors.Is(err, sql.ErrNoRows) {
		return mask.Definition{}, mask.NewError(mask.CodeNotFound, storage.ErrNotFound)
	}
	if err != nil {
		return mask.Definition{}, maskStorageError("get custom mask", err)
	}
	definition.BuiltIn = false
	definition.Origin = mask.OriginCustom
	definition.Bundle, definition.RecommendedSkills, err = storage.DecodeMaskBundleMetadata(installID, bundleID, localID, releaseDigest, releaseSequence, recommendationsJSON)
	if err != nil {
		return mask.Definition{}, maskStorageError("decode custom Mask provenance", err)
	}
	return definition, nil
}

func maskStorageError(operation string, err error) error {
	return mask.NewError(mask.CodeUnavailable, fmt.Errorf("storage: %s: %w", operation, err))
}
