package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"agent-vivy/internal/domain"
	mask "agent-vivy/internal/maskcontract"
	"agent-vivy/internal/storage"
)

var _ storage.MaskStore = (*Backend)(nil)

// ListCustomMasks returns only the mutable catalog rows. Built-ins belong to
// the compiled mask provider and are intentionally absent from Core Storage.
func (b *Backend) ListCustomMasks(ctx context.Context, in mask.ListRequest) (mask.Page, error) {
	in, err := mask.NormalizeList(in)
	if err != nil {
		return mask.Page{}, err
	}
	rows, err := b.db.QueryContext(ctx, `
		SELECT id, name, description, digest, revision
		FROM mask_definitions
		WHERE id > ?
		ORDER BY id
		LIMIT ?`, in.AfterID, in.Limit+1)
	if err != nil {
		return mask.Page{}, maskStorageError("list custom masks", err)
	}
	defer func() { _ = rows.Close() }()

	items := make([]mask.Metadata, 0, in.Limit)
	for rows.Next() {
		var item mask.Metadata
		if err := rows.Scan(&item.ID, &item.Name, &item.Description, &item.Digest, &item.Revision); err != nil {
			return mask.Page{}, maskStorageError("scan custom mask", err)
		}
		item.BuiltIn = false
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

	if existing, found, err := sqliteFindMaskByOperation(ctx, tx, in.OperationID); err != nil {
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
	_, err = tx.ExecContext(ctx, `
		INSERT INTO mask_definitions
			(id, name, description, body, revision, digest, created_at, updated_at, create_operation_id, create_request_digest)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(create_operation_id) DO NOTHING`,
		id, in.Name, in.Description, []byte(in.Body), 1, digest, now, now, in.OperationID, requestDigest)
	if err != nil {
		return mask.Definition{}, maskStorageError("insert custom mask", err)
	}

	// A concurrent retry can win the operation unique key between the initial
	// lookup and insert. Re-read the winner in the same transaction and apply
	// the immutable-original-request rule before returning it.
	created, found, err := sqliteFindMaskByOperation(ctx, tx, in.OperationID)
	if err != nil {
		return mask.Definition{}, err
	}
	if !found {
		return mask.Definition{}, maskStorageError("read inserted custom mask", errors.New("operation row missing after insert"))
	}
	if created.requestDigest != requestDigest {
		return mask.Definition{}, mask.NewError(mask.CodeRevisionConflict, nil)
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

	// Acquire SQLite's serialized writer lock before the revision read. This
	// keeps the check and write from observing a stale row across handles.
	if _, err := tx.ExecContext(ctx, `UPDATE mask_definitions SET updated_at = updated_at WHERE id = ?`, in.ID); err != nil {
		return mask.Definition{}, maskStorageError("lock custom mask for update", err)
	}
	current, err := b.getCustomMask(ctx, tx, in.ID)
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

	if _, err := tx.ExecContext(ctx, `UPDATE mask_definitions SET updated_at = updated_at WHERE id = ?`, in.ID); err != nil {
		return mask.DeleteResult{}, maskStorageError("lock custom mask for delete", err)
	}
	current, err := b.getCustomMask(ctx, tx, in.ID)
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
	if err := tx.Commit(); err != nil {
		return mask.DeleteResult{}, maskStorageError("commit delete custom mask", err)
	}
	return mask.DeleteResult{ID: in.ID}, nil
}

func (b *Backend) ReadMaskCapture(ctx context.Context, sessionID domain.SessionID) (mask.Capture, error) {
	if err := mask.ValidateSessionID(sessionID); err != nil {
		return mask.Capture{}, mask.NewError(mask.CodeInvalidMask, err)
	}
	tx, err := b.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return mask.Capture{}, maskStorageError("begin read mask capture", err)
	}
	defer func() { _ = tx.Rollback() }()
	var exists int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM sessions WHERE id = ?`, sessionID).Scan(&exists); err != nil {
		return mask.Capture{}, maskStorageError("check mask capture session", err)
	}
	if exists == 0 {
		return mask.Capture{}, mask.NewError(mask.CodeNotFound, storage.ErrNotFound)
	}
	selection, err := sqliteReadSelection(ctx, tx, sessionID)
	if err != nil {
		return mask.Capture{}, err
	}
	capture := mask.Capture{Selection: selection}
	if selection.MaskID != "" && !mask.IsBuiltinID(selection.MaskID) {
		definition, err := b.getCustomMask(ctx, tx, selection.MaskID)
		if err != nil {
			return mask.Capture{}, err
		}
		capture.Mask = &mask.Snapshot{
			ID: definition.ID, Name: definition.Name, Body: definition.Body,
			Digest: definition.Digest, DefinitionRevision: definition.Revision,
			SelectionRevision: selection.Revision, GenerationID: definition.GenerationID,
		}
	}
	if err := tx.Commit(); err != nil {
		return mask.Capture{}, maskStorageError("commit read mask capture", err)
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
	// A no-op parent update acquires SQLite's serialized writer lock before
	// reading the virtual revision and selected definition.
	if _, err := tx.ExecContext(ctx, `UPDATE sessions SET id = id WHERE id = ?`, in.SessionID); err != nil {
		return mask.Selection{}, maskStorageError("lock mask selection session", err)
	}
	var exists int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM sessions WHERE id = ?`, in.SessionID).Scan(&exists); err != nil {
		return mask.Selection{}, maskStorageError("check mask selection session", err)
	}
	if exists == 0 {
		return mask.Selection{}, mask.NewError(mask.CodeNotFound, storage.ErrNotFound)
	}
	current, err := sqliteReadSelection(ctx, tx, in.SessionID)
	if err != nil {
		return mask.Selection{}, err
	}
	if current.Revision != in.ExpectedRevision {
		return mask.Selection{}, mask.NewRevisionError(mask.CodeRevisionConflict, current.Revision, nil)
	}
	if in.MaskID != "" && !mask.IsBuiltinID(in.MaskID) {
		if _, err := sqliteGetCustomMaskForUpdate(ctx, tx, in.MaskID); err != nil {
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
	if err := tx.Commit(); err != nil {
		return mask.Selection{}, maskStorageError("commit mask selection", err)
	}
	return next, nil
}

type sqliteOperationResult struct {
	definition    mask.Definition
	requestDigest string
}

func sqliteFindMaskByOperation(ctx context.Context, q interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}, operationID string) (sqliteOperationResult, bool, error) {
	var result sqliteOperationResult
	var body []byte
	err := q.QueryRowContext(ctx, `
		SELECT id, name, description, body, revision, digest, create_request_digest
		FROM mask_definitions WHERE create_operation_id = ?`, operationID).
		Scan(&result.definition.ID, &result.definition.Name, &result.definition.Description, &body,
			&result.definition.Revision, &result.definition.Digest, &result.requestDigest)
	if errors.Is(err, sql.ErrNoRows) {
		return sqliteOperationResult{}, false, nil
	}
	if err != nil {
		return sqliteOperationResult{}, false, maskStorageError("find mask operation", err)
	}
	result.definition.Body = string(body)
	result.definition.BuiltIn = false
	return result, true, nil
}

func sqliteReadSelection(ctx context.Context, q interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}, sessionID domain.SessionID) (mask.Selection, error) {
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

func (b *Backend) getCustomMask(ctx context.Context, q interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}, id string) (mask.Definition, error) {
	return sqliteGetCustomMask(ctx, q, id)
}

func sqliteGetCustomMask(ctx context.Context, q interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}, id string) (mask.Definition, error) {
	var definition mask.Definition
	var body []byte
	err := q.QueryRowContext(ctx, `
		SELECT id, name, description, body, revision, digest
		FROM mask_definitions WHERE id = ?`, id).
		Scan(&definition.ID, &definition.Name, &definition.Description, &body, &definition.Revision, &definition.Digest)
	if errors.Is(err, sql.ErrNoRows) {
		return mask.Definition{}, mask.NewError(mask.CodeNotFound, storage.ErrNotFound)
	}
	if err != nil {
		return mask.Definition{}, maskStorageError("get custom mask", err)
	}
	definition.Body = string(body)
	definition.BuiltIn = false
	return definition, nil
}

func sqliteGetCustomMaskForUpdate(ctx context.Context, tx *sql.Tx, id string) (mask.Definition, error) {
	return sqliteGetCustomMask(ctx, tx, id)
}

func maskStorageError(operation string, err error) error {
	return mask.NewError(mask.CodeUnavailable, fmt.Errorf("storage: %s: %w", operation, err))
}
