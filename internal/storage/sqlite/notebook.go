package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	nb "agent-vivy/internal/notebookcontract"
	"agent-vivy/internal/storage"
)

// notebookStore implements the scoped revisioned-content contract over this
// backend's own transaction helpers (N1). All keys are composite on scope.
type notebookStore struct {
	db *sql.DB
}

// Notebook exposes the notebook content surface; callers keep the narrow
// contract, never the backend handle.
func (b *Backend) Notebook() storage.NotebookStore { return notebookStore{db: b.db} }

var _ storage.NotebookStore = notebookStore{}

type nbReceiptProbe struct {
	digest, kind, resourceID, revisionID string
	version                              int64
}

// mutate runs one notebook mutation inside the single-writer transaction:
// receipt lookup first, then state checks, revision write, head CAS, receipt
// insert — commit makes success visible, any failure rolls back all of it.
func (s notebookStore) mutate(ctx context.Context, mc nb.MutationContext, kind string, req any, fn func(tx *sql.Tx, now int64) (nb.MutationReceipt, error)) (nb.MutationReceipt, error) {
	digest, err := storage.NotebookValidateMutation(mc, kind, req)
	if err != nil {
		return nb.MutationReceipt{}, err
	}
	now := time.Now().UnixMilli()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nb.MutationReceipt{}, nbErrStorage(err)
	}
	defer func() { _ = tx.Rollback() }()

	if replay, replayErr := notebookReceiptLookup(ctx, tx, mc, digest); replayErr != nil {
		return nb.MutationReceipt{}, replayErr
	} else if replay != nil {
		return *replay, nil
	}
	if err := ensureNotebookSections(ctx, tx, mc.ScopeID, now); err != nil {
		return nb.MutationReceipt{}, err
	}
	receipt, err := fn(tx, now)
	if err != nil {
		_ = tx.Rollback()
		// A concurrent same-key commit may have landed while we waited on a
		// row lock: re-probe the receipt so identical retries converge on one
		// outcome instead of reporting a spurious conflict.
		if isUniqueConflict(err) || errors.Is(err, nb.ErrRevisionConflict) {
			if replay, rerr := notebookReceiptLookup(ctx, s.db, mc, digest); rerr == nil && replay != nil {
				return *replay, nil
			}
		}
		return nb.MutationReceipt{}, err
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO notebook_mutations (scope, operation_key, request_digest, resource_kind, resource_id, version, revision_id, created_at)
		 VALUES (?,?,?,?,?,?,?,?)`,
		string(mc.ScopeID), mc.OperationKey, digest, kind, receipt.ResourceID, receipt.Version, receipt.RevisionID, now); err != nil {
		if isUniqueConflict(err) {
			// Concurrent same-key commit won; roll back ours and return the
			// stored outcome (or refuse a divergent payload).
			_ = tx.Rollback()
			return notebookReceiptAfterConflict(ctx, s.db, mc, digest)
		}
		return nb.MutationReceipt{}, nbErrStorage(err)
	}
	if err := tx.Commit(); err != nil {
		return nb.MutationReceipt{}, nbErrStorage(err)
	}
	return receipt, nil
}

func notebookReceiptLookup(ctx context.Context, q interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}, mc nb.MutationContext, digest string) (*nb.MutationReceipt, error) {
	var p nbReceiptProbe
	err := q.QueryRowContext(ctx,
		`SELECT request_digest, resource_kind, resource_id, version, revision_id FROM notebook_mutations WHERE scope = ? AND operation_key = ?`,
		string(mc.ScopeID), mc.OperationKey).
		Scan(&p.digest, &p.kind, &p.resourceID, &p.version, &p.revisionID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, nbErrStorage(err)
	}
	if p.digest != digest {
		return nil, nb.ErrIdempotencyConflict
	}
	return &nb.MutationReceipt{ResourceID: p.resourceID, Version: p.version, RevisionID: p.revisionID, Replayed: true}, nil
}

func notebookReceiptAfterConflict(ctx context.Context, db *sql.DB, mc nb.MutationContext, digest string) (nb.MutationReceipt, error) {
	replay, err := notebookReceiptLookup(ctx, db, mc, digest)
	if err != nil {
		return nb.MutationReceipt{}, err
	}
	if replay == nil {
		return nb.MutationReceipt{}, nb.ErrIdempotencyConflict
	}
	return *replay, nil
}

// ensureNotebookSections seeds the four system-role sections per scope; the
// insert is idempotent so a deliberate tombstone is never resurrected.
func ensureNotebookSections(ctx context.Context, tx *sql.Tx, scope nb.ScopeID, now int64) error {
	for _, role := range nb.SystemRoles {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO notebook_sections (scope, id, title, system_role, version, created_at, updated_at, deleted_at)
			 VALUES (?,?,?,?,1,?,?,0) ON CONFLICT (scope,id) DO NOTHING`,
			string(scope), nb.RoleSectionID(role), nb.RoleDefaultTitle(role), string(role), now, now); err != nil {
			return nbErrStorage(err)
		}
	}
	return nil
}

// --- row loaders (scope-filtered; absent/deleted-as-required = not_found) ---

func loadSection(ctx context.Context, q interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}, scope nb.ScopeID, id string) (nb.Section, error) {
	var s nb.Section
	err := q.QueryRowContext(ctx,
		`SELECT id, title, system_role, version, created_at, updated_at, deleted_at
		 FROM notebook_sections WHERE scope = ? AND id = ?`, string(scope), id).
		Scan(&s.ID, &s.Title, &s.SystemRole, &s.Version, &s.CreatedAt, &s.UpdatedAt, &s.DeletedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nb.Section{}, nb.ErrNotFound
	}
	if err != nil {
		return nb.Section{}, nbErrStorage(err)
	}
	return s, nil
}

func loadEntry(ctx context.Context, q interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}, scope nb.ScopeID, id string) (nb.Entry, error) {
	var e nb.Entry
	err := q.QueryRowContext(ctx,
		`SELECT id, section_id, kind, title, head_revision_id, version, report_series_id, report_window_id, created_at, updated_at, deleted_at
		 FROM notebook_entries WHERE scope = ? AND id = ?`, string(scope), id).
		Scan(&e.ID, &e.SectionID, &e.Kind, &e.Title, &e.HeadRevisionID, &e.Version,
			&e.ReportSeriesID, &e.ReportWindowID, &e.CreatedAt, &e.UpdatedAt, &e.DeletedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nb.Entry{}, nb.ErrNotFound
	}
	if err != nil {
		return nb.Entry{}, nbErrStorage(err)
	}
	return e, nil
}

func loadRevision(ctx context.Context, q interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}, scope nb.ScopeID, entryID, revisionID string) (nb.Revision, error) {
	var r nb.Revision
	err := q.QueryRowContext(ctx,
		`SELECT revision_id, entry_id, sequence, parent_revision_id, title, markdown, origin, base_revision_id, actor, created_at
		 FROM notebook_revisions WHERE scope = ? AND revision_id = ? AND entry_id = ?`,
		string(scope), revisionID, entryID).
		Scan(&r.ID, &r.EntryID, &r.Sequence, &r.ParentRevisionID, &r.Title, &r.Markdown,
			&r.Origin, &r.BaseRevisionID, &r.Actor, &r.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nb.Revision{}, nb.ErrNotFound
	}
	if err != nil {
		return nb.Revision{}, nbErrStorage(err)
	}
	return r, nil
}

func nextRevisionSeq(ctx context.Context, tx *sql.Tx, scope nb.ScopeID, entryID string) (int64, error) {
	var max sql.NullInt64
	if err := tx.QueryRowContext(ctx,
		`SELECT MAX(sequence) FROM notebook_revisions WHERE scope = ? AND entry_id = ?`,
		string(scope), entryID).Scan(&max); err != nil {
		return 0, nbErrStorage(err)
	}
	if !max.Valid {
		return 1, nil
	}
	return max.Int64 + 1, nil
}

func insertRevision(ctx context.Context, tx *sql.Tx, scope nb.ScopeID, r nb.Revision) error {
	_, err := tx.ExecContext(ctx,
		`INSERT INTO notebook_revisions (scope, entry_id, revision_id, sequence, parent_revision_id, title, markdown, origin, base_revision_id, actor, created_at)
		 VALUES (?,?,?,?,?,?,?,?,?,?,?)`,
		string(scope), r.EntryID, r.ID, r.Sequence, r.ParentRevisionID, r.Title, r.Markdown,
		string(r.Origin), r.BaseRevisionID, r.Actor, r.CreatedAt)
	return err
}

// casEntry applies an entry row update guarded by expected version; a zero
// row count means a concurrent writer won and is reported as conflict.
func casEntry(ctx context.Context, tx *sql.Tx, scope nb.ScopeID, e nb.Entry, now int64, set string, args ...any) error {
	query := `UPDATE notebook_entries SET ` + set + `, version = version + 1, updated_at = ? WHERE scope = ? AND id = ? AND version = ?`
	args = append(args, now, string(scope), e.ID, e.Version)
	res, err := tx.ExecContext(ctx, query, args...)
	if err != nil {
		return nbErrStorage(err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return nbErrStorage(err)
	}
	if n != 1 {
		return &nb.Error{Code: nb.CodeRevisionConflict, Message: "entry changed concurrently", CurrentVersion: e.Version, CurrentRevisionID: e.HeadRevisionID}
	}
	return nil
}

func nbErrStorage(err error) error {
	return &nb.Error{Code: nb.CodeStorageUnavailable, Message: err.Error(), Retryable: true}
}

func nbRequireEntry(ctx context.Context, tx *sql.Tx, scope nb.ScopeID, id string) (nb.Entry, error) {
	e, err := loadEntry(ctx, tx, scope, id)
	if err != nil {
		return nb.Entry{}, err
	}
	if e.DeletedAt != 0 {
		return nb.Entry{}, nb.ErrNotFound
	}
	return e, nil
}

func nbConflictWith(e nb.Entry, msg string) *nb.Error {
	return &nb.Error{Code: nb.CodeRevisionConflict, Message: msg, CurrentVersion: e.Version, CurrentRevisionID: e.HeadRevisionID}
}

func newActorRevision(ctx context.Context, tx *sql.Tx, scope nb.ScopeID, mc nb.MutationContext, entry nb.Entry, title, markdown, base string, now int64) (nb.Revision, error) {
	origin, err := mc.Actor.Kind.OriginFor()
	if err != nil {
		return nb.Revision{}, &nb.Error{Code: nb.CodeInvalidRequest, Message: err.Error()}
	}
	seq, err := nextRevisionSeq(ctx, tx, scope, entry.ID)
	if err != nil {
		return nb.Revision{}, err
	}
	id, err := storage.NewNotebookID("rev-")
	if err != nil {
		return nb.Revision{}, err
	}
	rev := nb.Revision{
		ID: id, EntryID: entry.ID, Sequence: seq, ParentRevisionID: entry.HeadRevisionID,
		Title: title, Markdown: markdown, Origin: origin, BaseRevisionID: base,
		Actor: mc.Actor.Ref, CreatedAt: now,
	}
	if err := insertRevision(ctx, tx, scope, rev); err != nil {
		return nb.Revision{}, nbErrStorage(err)
	}
	return rev, nil
}

// --- sections ----------------------------------------------------------------

func (s notebookStore) ListSections(ctx context.Context, scope nb.ScopeID, req nb.ListSectionsRequest) (nb.SectionPage, error) {
	if scope == "" {
		return nb.SectionPage{}, nb.ErrInvalidRequest
	}
	now := time.Now().UnixMilli()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nb.SectionPage{}, nbErrStorage(err)
	}
	defer func() { _ = tx.Rollback() }()
	if err := ensureNotebookSections(ctx, tx, scope, now); err != nil {
		return nb.SectionPage{}, err
	}
	if err := tx.Commit(); err != nil {
		return nb.SectionPage{}, nbErrStorage(err)
	}

	tag := storage.NotebookFilterTag("sections", string(scope))
	c, err := storage.DecodeNotebookCursor(req.Cursor, tag)
	if err != nil {
		return nb.SectionPage{}, err
	}
	limit := storage.NotebookPageLimit(req.Limit)
	args := []any{string(scope)}
	query := `SELECT id, title, system_role, version, created_at, updated_at, deleted_at FROM notebook_sections WHERE scope = ?`
	if req.Cursor != "" {
		query += ` AND (created_at > ? OR (created_at = ? AND id > ?))`
		args = append(args, c.U, c.U, c.I)
	}
	query += ` ORDER BY created_at ASC, id ASC LIMIT ?`
	args = append(args, limit+1)
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nb.SectionPage{}, nbErrStorage(err)
	}
	defer rows.Close()
	page := nb.SectionPage{Sections: []nb.Section{}}
	for rows.Next() {
		var sec nb.Section
		if err := rows.Scan(&sec.ID, &sec.Title, &sec.SystemRole, &sec.Version, &sec.CreatedAt, &sec.UpdatedAt, &sec.DeletedAt); err != nil {
			return nb.SectionPage{}, nbErrStorage(err)
		}
		if len(page.Sections) == limit {
			last := page.Sections[len(page.Sections)-1]
			page.NextCursor = storage.EncodeNotebookCursor(last.CreatedAt, last.ID, tag)
			return page, nil
		}
		page.Sections = append(page.Sections, sec)
	}
	return page, rows.Err()
}

func (s notebookStore) CreateSection(ctx context.Context, mc nb.MutationContext, req nb.CreateSectionRequest) (nb.MutationReceipt, error) {
	return s.mutate(ctx, mc, "vivy.notebook.sections.create", req, func(tx *sql.Tx, now int64) (nb.MutationReceipt, error) {
		if err := storage.NotebookValidateTitle(req.Title); err != nil {
			return nb.MutationReceipt{}, err
		}
		id, err := storage.NewNotebookID("sec-")
		if err != nil {
			return nb.MutationReceipt{}, err
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO notebook_sections (scope, id, title, system_role, version, created_at, updated_at, deleted_at) VALUES (?,?,?, '',1,?,?,0)`,
			string(mc.ScopeID), id, req.Title, now, now); err != nil {
			return nb.MutationReceipt{}, nbErrStorage(err)
		}
		return nb.MutationReceipt{ResourceID: id, Version: 1}, nil
	})
}

func (s notebookStore) UpdateSection(ctx context.Context, mc nb.MutationContext, req nb.UpdateSectionRequest) (nb.MutationReceipt, error) {
	return s.mutate(ctx, mc, "vivy.notebook.sections.update", req, func(tx *sql.Tx, now int64) (nb.MutationReceipt, error) {
		if err := storage.NotebookValidateTitle(req.Title); err != nil {
			return nb.MutationReceipt{}, err
		}
		sec, err := loadSection(ctx, tx, mc.ScopeID, req.ID)
		if err != nil {
			return nb.MutationReceipt{}, err
		}
		if sec.DeletedAt != 0 {
			return nb.MutationReceipt{}, nb.ErrNotFound
		}
		if sec.Version != req.ExpectedVersion {
			return nb.MutationReceipt{}, &nb.Error{Code: nb.CodeRevisionConflict, Message: "section changed", CurrentVersion: sec.Version}
		}
		res, err := tx.ExecContext(ctx,
			`UPDATE notebook_sections SET title = ?, version = version + 1, updated_at = ? WHERE scope = ? AND id = ? AND version = ?`,
			req.Title, now, string(mc.ScopeID), req.ID, req.ExpectedVersion)
		if err != nil {
			return nb.MutationReceipt{}, nbErrStorage(err)
		}
		if n, _ := res.RowsAffected(); n != 1 {
			return nb.MutationReceipt{}, &nb.Error{Code: nb.CodeRevisionConflict, Message: "section changed concurrently", CurrentVersion: sec.Version}
		}
		return nb.MutationReceipt{ResourceID: req.ID, Version: sec.Version + 1}, nil
	})
}

func (s notebookStore) DeleteSection(ctx context.Context, mc nb.MutationContext, req nb.DeleteSectionRequest) (nb.MutationReceipt, error) {
	return s.mutate(ctx, mc, "vivy.notebook.sections.delete", req, func(tx *sql.Tx, now int64) (nb.MutationReceipt, error) {
		sec, err := loadSection(ctx, tx, mc.ScopeID, req.ID)
		if err != nil {
			return nb.MutationReceipt{}, err
		}
		if sec.DeletedAt != 0 {
			return nb.MutationReceipt{}, nb.ErrNotFound
		}
		if sec.SystemRole != nb.RoleNone {
			return nb.MutationReceipt{}, nb.ErrSectionInUse
		}
		if sec.Version != req.ExpectedVersion {
			return nb.MutationReceipt{}, &nb.Error{Code: nb.CodeRevisionConflict, Message: "section changed", CurrentVersion: sec.Version}
		}
		var live int
		if err := tx.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM notebook_entries WHERE scope = ? AND section_id = ? AND deleted_at = 0`,
			string(mc.ScopeID), req.ID).Scan(&live); err != nil {
			return nb.MutationReceipt{}, nbErrStorage(err)
		}
		if live > 0 {
			return nb.MutationReceipt{}, nb.ErrSectionNotEmpty
		}
		res, err := tx.ExecContext(ctx,
			`UPDATE notebook_sections SET deleted_at = ?, version = version + 1, updated_at = ? WHERE scope = ? AND id = ? AND version = ? AND deleted_at = 0`,
			now, now, string(mc.ScopeID), req.ID, req.ExpectedVersion)
		if err != nil {
			return nb.MutationReceipt{}, nbErrStorage(err)
		}
		if n, _ := res.RowsAffected(); n != 1 {
			return nb.MutationReceipt{}, &nb.Error{Code: nb.CodeRevisionConflict, Message: "section changed concurrently", CurrentVersion: sec.Version}
		}
		return nb.MutationReceipt{ResourceID: req.ID, Version: sec.Version + 1}, nil
	})
}

func (s notebookStore) RestoreSection(ctx context.Context, mc nb.MutationContext, req nb.RestoreSectionRequest) (nb.MutationReceipt, error) {
	return s.mutate(ctx, mc, "vivy.notebook.sections.restore", req, func(tx *sql.Tx, now int64) (nb.MutationReceipt, error) {
		sec, err := loadSection(ctx, tx, mc.ScopeID, req.ID)
		if err != nil {
			return nb.MutationReceipt{}, err
		}
		if sec.DeletedAt == 0 {
			return nb.MutationReceipt{}, &nb.Error{Code: nb.CodeInvalidRequest, Message: "section is not deleted"}
		}
		if sec.Version != req.ExpectedVersion {
			return nb.MutationReceipt{}, &nb.Error{Code: nb.CodeRevisionConflict, Message: "section changed", CurrentVersion: sec.Version}
		}
		res, err := tx.ExecContext(ctx,
			`UPDATE notebook_sections SET deleted_at = 0, version = version + 1, updated_at = ? WHERE scope = ? AND id = ? AND version = ?`,
			now, string(mc.ScopeID), req.ID, req.ExpectedVersion)
		if err != nil {
			return nb.MutationReceipt{}, nbErrStorage(err)
		}
		if n, _ := res.RowsAffected(); n != 1 {
			return nb.MutationReceipt{}, &nb.Error{Code: nb.CodeRevisionConflict, Message: "section changed concurrently", CurrentVersion: sec.Version}
		}
		return nb.MutationReceipt{ResourceID: req.ID, Version: sec.Version + 1}, nil
	})
}

// --- entries -----------------------------------------------------------------

func (s notebookStore) ListEntries(ctx context.Context, scope nb.ScopeID, req nb.ListEntriesRequest) (nb.EntryPage, error) {
	if scope == "" {
		return nb.EntryPage{}, nb.ErrInvalidRequest
	}
	tag := storage.NotebookFilterTag("entries", string(scope), req.SectionID, boolTag(req.IncludeDeleted))
	c, err := storage.DecodeNotebookCursor(req.Cursor, tag)
	if err != nil {
		return nb.EntryPage{}, err
	}
	limit := storage.NotebookPageLimit(req.Limit)
	args := []any{string(scope)}
	query := `SELECT id, section_id, kind, title, head_revision_id, version, report_series_id, report_window_id, created_at, updated_at, deleted_at
		FROM notebook_entries WHERE scope = ?`
	if req.SectionID != "" {
		query += ` AND section_id = ?`
		args = append(args, req.SectionID)
	}
	if !req.IncludeDeleted {
		query += ` AND deleted_at = 0`
	}
	if req.Cursor != "" {
		query += ` AND (updated_at < ? OR (updated_at = ? AND id < ?))`
		args = append(args, c.U, c.U, c.I)
	}
	query += ` ORDER BY updated_at DESC, id DESC LIMIT ?`
	args = append(args, limit+1)
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nb.EntryPage{}, nbErrStorage(err)
	}
	defer rows.Close()
	page := nb.EntryPage{Entries: []nb.Entry{}}
	for rows.Next() {
		var e nb.Entry
		if err := rows.Scan(&e.ID, &e.SectionID, &e.Kind, &e.Title, &e.HeadRevisionID, &e.Version,
			&e.ReportSeriesID, &e.ReportWindowID, &e.CreatedAt, &e.UpdatedAt, &e.DeletedAt); err != nil {
			return nb.EntryPage{}, nbErrStorage(err)
		}
		if len(page.Entries) == limit {
			last := page.Entries[len(page.Entries)-1]
			page.NextCursor = storage.EncodeNotebookCursor(last.UpdatedAt, last.ID, tag)
			return page, nil
		}
		page.Entries = append(page.Entries, e)
	}
	return page, rows.Err()
}

func (s notebookStore) GetEntry(ctx context.Context, scope nb.ScopeID, req nb.GetEntryRequest) (nb.EntryView, error) {
	if scope == "" || req.EntryID == "" {
		return nb.EntryView{}, nb.ErrInvalidRequest
	}
	e, err := loadEntry(ctx, s.db, scope, req.EntryID)
	if err != nil {
		return nb.EntryView{}, err
	}
	if e.DeletedAt != 0 {
		return nb.EntryView{}, nb.ErrNotFound
	}
	revID := req.RevisionID
	if revID == "" {
		revID = e.HeadRevisionID
	}
	rev, err := loadRevision(ctx, s.db, scope, e.ID, revID)
	if err != nil {
		return nb.EntryView{}, err
	}
	return nb.EntryView{Entry: e, Revision: rev}, nil
}

func (s notebookStore) CreateEntry(ctx context.Context, mc nb.MutationContext, req nb.CreateEntryRequest) (nb.MutationReceipt, error) {
	return s.mutate(ctx, mc, "vivy.notebook.entries.create", req, func(tx *sql.Tx, now int64) (nb.MutationReceipt, error) {
		if err := storage.NotebookValidateTitle(req.Title); err != nil {
			return nb.MutationReceipt{}, err
		}
		if err := storage.NotebookValidateBody(req.Markdown); err != nil {
			return nb.MutationReceipt{}, err
		}
		sec, err := loadSection(ctx, tx, mc.ScopeID, req.SectionID)
		if err != nil {
			return nb.MutationReceipt{}, err
		}
		if sec.DeletedAt != 0 {
			return nb.MutationReceipt{}, nb.ErrNotFound
		}
		entryID, err := storage.NewNotebookID("nbe-")
		if err != nil {
			return nb.MutationReceipt{}, err
		}
		revID, err := storage.NewNotebookID("rev-")
		if err != nil {
			return nb.MutationReceipt{}, err
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO notebook_entries (scope, id, section_id, kind, title, head_revision_id, version, report_series_id, report_window_id, created_at, updated_at, deleted_at)
			 VALUES (?,?,?,'note',?,?,1,'','',?,?,0)`,
			string(mc.ScopeID), entryID, req.SectionID, req.Title, revID, now, now); err != nil {
			return nb.MutationReceipt{}, nbErrStorage(err)
		}
		origin, err := mc.Actor.Kind.OriginFor()
		if err != nil {
			return nb.MutationReceipt{}, &nb.Error{Code: nb.CodeInvalidRequest, Message: err.Error()}
		}
		if err := insertRevision(ctx, tx, mc.ScopeID, nb.Revision{
			ID: revID, EntryID: entryID, Sequence: 1, Title: req.Title, Markdown: req.Markdown,
			Origin: origin, Actor: mc.Actor.Ref, CreatedAt: now,
		}); err != nil {
			return nb.MutationReceipt{}, nbErrStorage(err)
		}
		return nb.MutationReceipt{ResourceID: entryID, Version: 1, RevisionID: revID}, nil
	})
}

func (s notebookStore) SaveEntry(ctx context.Context, mc nb.MutationContext, req nb.SaveEntryRequest) (nb.MutationReceipt, error) {
	return s.mutate(ctx, mc, "vivy.notebook.entries.save", req, func(tx *sql.Tx, now int64) (nb.MutationReceipt, error) {
		if err := storage.NotebookValidateTitle(req.Title); err != nil {
			return nb.MutationReceipt{}, err
		}
		if err := storage.NotebookValidateBody(req.Markdown); err != nil {
			return nb.MutationReceipt{}, err
		}
		e, err := nbRequireEntry(ctx, tx, mc.ScopeID, req.EntryID)
		if err != nil {
			return nb.MutationReceipt{}, err
		}
		if e.Version != req.ExpectedVersion {
			return nb.MutationReceipt{}, nbConflictWith(e, "stale expected version")
		}
		if e.HeadRevisionID != req.BaseRevisionID {
			return nb.MutationReceipt{}, nbConflictWith(e, "stale base revision")
		}
		rev, err := newActorRevision(ctx, tx, mc.ScopeID, mc, e, req.Title, req.Markdown, req.BaseRevisionID, now)
		if err != nil {
			return nb.MutationReceipt{}, err
		}
		if err := casEntry(ctx, tx, mc.ScopeID, e, now, `head_revision_id = ?, title = ?`, rev.ID, req.Title); err != nil {
			return nb.MutationReceipt{}, err
		}
		return nb.MutationReceipt{ResourceID: e.ID, Version: e.Version + 1, RevisionID: rev.ID}, nil
	})
}

func (s notebookStore) MoveEntry(ctx context.Context, mc nb.MutationContext, req nb.MoveEntryRequest) (nb.MutationReceipt, error) {
	return s.mutate(ctx, mc, "vivy.notebook.entries.move", req, func(tx *sql.Tx, now int64) (nb.MutationReceipt, error) {
		e, err := nbRequireEntry(ctx, tx, mc.ScopeID, req.EntryID)
		if err != nil {
			return nb.MutationReceipt{}, err
		}
		if e.Version != req.ExpectedVersion {
			return nb.MutationReceipt{}, nbConflictWith(e, "stale expected version")
		}
		sec, err := loadSection(ctx, tx, mc.ScopeID, req.SectionID)
		if err != nil {
			return nb.MutationReceipt{}, err
		}
		if sec.DeletedAt != 0 {
			return nb.MutationReceipt{}, nb.ErrNotFound
		}
		if err := casEntry(ctx, tx, mc.ScopeID, e, now, `section_id = ?`, req.SectionID); err != nil {
			return nb.MutationReceipt{}, err
		}
		return nb.MutationReceipt{ResourceID: e.ID, Version: e.Version + 1}, nil
	})
}

func (s notebookStore) DeleteEntry(ctx context.Context, mc nb.MutationContext, req nb.DeleteEntryRequest) (nb.MutationReceipt, error) {
	return s.mutate(ctx, mc, "vivy.notebook.entries.delete", req, func(tx *sql.Tx, now int64) (nb.MutationReceipt, error) {
		e, err := nbRequireEntry(ctx, tx, mc.ScopeID, req.EntryID)
		if err != nil {
			return nb.MutationReceipt{}, err
		}
		if e.Version != req.ExpectedVersion {
			return nb.MutationReceipt{}, nbConflictWith(e, "stale expected version")
		}
		if err := casEntry(ctx, tx, mc.ScopeID, e, now, `deleted_at = ?`, now); err != nil {
			return nb.MutationReceipt{}, err
		}
		return nb.MutationReceipt{ResourceID: e.ID, Version: e.Version + 1}, nil
	})
}

func (s notebookStore) RestoreEntry(ctx context.Context, mc nb.MutationContext, req nb.RestoreEntryRequest) (nb.MutationReceipt, error) {
	return s.mutate(ctx, mc, "vivy.notebook.entries.restore", req, func(tx *sql.Tx, now int64) (nb.MutationReceipt, error) {
		e, err := loadEntry(ctx, tx, mc.ScopeID, req.EntryID)
		if err != nil {
			return nb.MutationReceipt{}, err
		}
		if e.DeletedAt == 0 {
			return nb.MutationReceipt{}, &nb.Error{Code: nb.CodeInvalidRequest, Message: "entry is not deleted"}
		}
		if e.Version != req.ExpectedVersion {
			return nb.MutationReceipt{}, nbConflictWith(e, "stale expected version")
		}
		// A deleted parent section is never resurrected silently: restoring the
		// entry leaves the section tombstone exactly as it is.
		if err := casEntry(ctx, tx, mc.ScopeID, e, now, `deleted_at = 0`); err != nil {
			return nb.MutationReceipt{}, err
		}
		return nb.MutationReceipt{ResourceID: e.ID, Version: e.Version + 1}, nil
	})
}

// --- revisions ---------------------------------------------------------------

func (s notebookStore) ListRevisions(ctx context.Context, scope nb.ScopeID, req nb.ListRevisionsRequest) (nb.RevisionPage, error) {
	if scope == "" || req.EntryID == "" {
		return nb.RevisionPage{}, nb.ErrInvalidRequest
	}
	tag := storage.NotebookFilterTag("revisions", string(scope), req.EntryID)
	c, err := storage.DecodeNotebookCursor(req.Cursor, tag)
	if err != nil {
		return nb.RevisionPage{}, err
	}
	if _, err := loadEntry(ctx, s.db, scope, req.EntryID); err != nil {
		return nb.RevisionPage{}, err
	}
	limit := storage.NotebookPageLimit(req.Limit)
	args := []any{string(scope), req.EntryID}
	query := `SELECT revision_id, entry_id, sequence, parent_revision_id, title, '', origin, base_revision_id, actor, created_at
		FROM notebook_revisions WHERE scope = ? AND entry_id = ?`
	if req.Cursor != "" {
		query += ` AND sequence < ?`
		args = append(args, c.U)
	}
	query += ` ORDER BY sequence DESC LIMIT ?`
	args = append(args, limit+1)
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nb.RevisionPage{}, nbErrStorage(err)
	}
	defer rows.Close()
	page := nb.RevisionPage{Revisions: []nb.Revision{}}
	for rows.Next() {
		var r nb.Revision
		if err := rows.Scan(&r.ID, &r.EntryID, &r.Sequence, &r.ParentRevisionID, &r.Title, &r.Markdown,
			&r.Origin, &r.BaseRevisionID, &r.Actor, &r.CreatedAt); err != nil {
			return nb.RevisionPage{}, nbErrStorage(err)
		}
		if len(page.Revisions) == limit {
			last := page.Revisions[len(page.Revisions)-1]
			page.NextCursor = storage.EncodeNotebookCursor(last.Sequence, last.ID, tag)
			return page, nil
		}
		page.Revisions = append(page.Revisions, r)
	}
	return page, rows.Err()
}

func (s notebookStore) AdoptRevision(ctx context.Context, mc nb.MutationContext, req nb.AdoptRevisionRequest) (nb.MutationReceipt, error) {
	return s.mutate(ctx, mc, "vivy.notebook.revisions.adopt", req, func(tx *sql.Tx, now int64) (nb.MutationReceipt, error) {
		e, err := nbRequireEntry(ctx, tx, mc.ScopeID, req.EntryID)
		if err != nil {
			return nb.MutationReceipt{}, err
		}
		if e.Version != req.ExpectedVersion {
			return nb.MutationReceipt{}, nbConflictWith(e, "stale expected version")
		}
		// The adopted revision must belong to this entry and scope; foreign or
		// unknown IDs share not_found.
		old, err := loadRevision(ctx, tx, mc.ScopeID, e.ID, req.RevisionID)
		if err != nil {
			return nb.MutationReceipt{}, err
		}
		rev, err := newActorRevision(ctx, tx, mc.ScopeID, mc, e, old.Title, old.Markdown, old.ID, now)
		if err != nil {
			return nb.MutationReceipt{}, err
		}
		if err := casEntry(ctx, tx, mc.ScopeID, e, now, `head_revision_id = ?, title = ?`, rev.ID, old.Title); err != nil {
			return nb.MutationReceipt{}, err
		}
		return nb.MutationReceipt{ResourceID: e.ID, Version: e.Version + 1, RevisionID: rev.ID}, nil
	})
}

// --- comments ----------------------------------------------------------------

func (s notebookStore) ListComments(ctx context.Context, scope nb.ScopeID, req nb.ListCommentsRequest) (nb.CommentPage, error) {
	if scope == "" || req.EntryID == "" {
		return nb.CommentPage{}, nb.ErrInvalidRequest
	}
	if req.Status != "" && !req.Status.Valid() {
		return nb.CommentPage{}, nb.ErrInvalidRequest
	}
	tag := storage.NotebookFilterTag("comments", string(scope), req.EntryID, string(req.Status))
	c, err := storage.DecodeNotebookCursor(req.Cursor, tag)
	if err != nil {
		return nb.CommentPage{}, err
	}
	limit := storage.NotebookPageLimit(req.Limit)
	args := []any{string(scope), req.EntryID}
	query := `SELECT id, entry_id, COALESCE(anchor_revision_id,''), body, version, author, status, created_at, updated_at
		FROM notebook_comments WHERE scope = ? AND entry_id = ?`
	if req.Status != "" {
		query += ` AND status = ?`
		args = append(args, string(req.Status))
	}
	if req.Cursor != "" {
		query += ` AND (created_at > ? OR (created_at = ? AND id > ?))`
		args = append(args, c.U, c.U, c.I)
	}
	query += ` ORDER BY created_at ASC, id ASC LIMIT ?`
	args = append(args, limit+1)
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nb.CommentPage{}, nbErrStorage(err)
	}
	defer rows.Close()
	page := nb.CommentPage{Comments: []nb.Comment{}}
	for rows.Next() {
		var cm nb.Comment
		if err := rows.Scan(&cm.ID, &cm.EntryID, &cm.AnchorRevisionID, &cm.Body, &cm.Version,
			&cm.Author, &cm.Status, &cm.CreatedAt, &cm.UpdatedAt); err != nil {
			return nb.CommentPage{}, nbErrStorage(err)
		}
		if len(page.Comments) == limit {
			last := page.Comments[len(page.Comments)-1]
			page.NextCursor = storage.EncodeNotebookCursor(last.CreatedAt, last.ID, tag)
			return page, nil
		}
		page.Comments = append(page.Comments, cm)
	}
	return page, rows.Err()
}

func (s notebookStore) CreateComment(ctx context.Context, mc nb.MutationContext, req nb.CreateCommentRequest) (nb.MutationReceipt, error) {
	return s.mutate(ctx, mc, "vivy.notebook.comments.create", req, func(tx *sql.Tx, now int64) (nb.MutationReceipt, error) {
		if err := storage.NotebookValidateCommentBody(req.Body); err != nil {
			return nb.MutationReceipt{}, err
		}
		e, err := nbRequireEntry(ctx, tx, mc.ScopeID, req.EntryID)
		if err != nil {
			return nb.MutationReceipt{}, err
		}
		var anchor any
		if req.AnchorRevisionID != "" {
			if _, err := loadRevision(ctx, tx, mc.ScopeID, e.ID, req.AnchorRevisionID); err != nil {
				return nb.MutationReceipt{}, err
			}
			anchor = req.AnchorRevisionID
		}
		id, err := storage.NewNotebookID("cm-")
		if err != nil {
			return nb.MutationReceipt{}, err
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO notebook_comments (scope, id, entry_id, anchor_revision_id, body, version, author, status, created_at, updated_at)
			 VALUES (?,?,?,?,?,1,?,?,?,?)`,
			string(mc.ScopeID), id, e.ID, anchor, req.Body, mc.Actor.Ref, string(nb.CommentActive), now, now); err != nil {
			return nb.MutationReceipt{}, nbErrStorage(err)
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO notebook_comment_versions (scope, comment_id, version, body, status, updated_at) VALUES (?,?,1,?,?,?)`,
			string(mc.ScopeID), id, req.Body, string(nb.CommentActive), now); err != nil {
			return nb.MutationReceipt{}, nbErrStorage(err)
		}
		return nb.MutationReceipt{ResourceID: id, Version: 1}, nil
	})
}

func (s notebookStore) UpdateComment(ctx context.Context, mc nb.MutationContext, req nb.UpdateCommentRequest) (nb.MutationReceipt, error) {
	return s.mutate(ctx, mc, "vivy.notebook.comments.update", req, func(tx *sql.Tx, now int64) (nb.MutationReceipt, error) {
		if req.Body == nil && req.Status == "" {
			return nb.MutationReceipt{}, &nb.Error{Code: nb.CodeInvalidRequest, Message: "no comment change requested"}
		}
		if req.Status != "" && !req.Status.Valid() {
			return nb.MutationReceipt{}, &nb.Error{Code: nb.CodeInvalidRequest, Message: "unknown comment status"}
		}
		var cm nb.Comment
		err := tx.QueryRowContext(ctx,
			`SELECT id, entry_id, COALESCE(anchor_revision_id,''), body, version, author, status, created_at, updated_at
			 FROM notebook_comments WHERE scope = ? AND id = ?`, string(mc.ScopeID), req.CommentID).
			Scan(&cm.ID, &cm.EntryID, &cm.AnchorRevisionID, &cm.Body, &cm.Version, &cm.Author, &cm.Status, &cm.CreatedAt, &cm.UpdatedAt)
		if errors.Is(err, sql.ErrNoRows) {
			return nb.MutationReceipt{}, nb.ErrNotFound
		}
		if err != nil {
			return nb.MutationReceipt{}, nbErrStorage(err)
		}
		if cm.Version != req.ExpectedVersion {
			return nb.MutationReceipt{}, &nb.Error{Code: nb.CodeRevisionConflict, Message: "comment changed", CurrentVersion: cm.Version}
		}
		body, status := cm.Body, cm.Status
		if req.Body != nil {
			if err := storage.NotebookValidateCommentBody(*req.Body); err != nil {
				return nb.MutationReceipt{}, err
			}
			body = *req.Body
		}
		if req.Status != "" {
			status = req.Status
		}
		newVersion := cm.Version + 1
		res, err := tx.ExecContext(ctx,
			`UPDATE notebook_comments SET body = ?, status = ?, version = ?, updated_at = ? WHERE scope = ? AND id = ? AND version = ?`,
			body, string(status), newVersion, now, string(mc.ScopeID), req.CommentID, req.ExpectedVersion)
		if err != nil {
			return nb.MutationReceipt{}, nbErrStorage(err)
		}
		if n, _ := res.RowsAffected(); n != 1 {
			return nb.MutationReceipt{}, &nb.Error{Code: nb.CodeRevisionConflict, Message: "comment changed concurrently", CurrentVersion: cm.Version}
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO notebook_comment_versions (scope, comment_id, version, body, status, updated_at) VALUES (?,?,?,?,?,?)`,
			string(mc.ScopeID), cm.ID, newVersion, body, string(status), now); err != nil {
			return nb.MutationReceipt{}, nbErrStorage(err)
		}
		return nb.MutationReceipt{ResourceID: cm.ID, Version: newVersion}, nil
	})
}

func boolTag(b bool) string {
	if b {
		return "1"
	}
	return "0"
}

// OpenExisting opens an already-migrated database without applying migrations
// or taking leases. It is the read path for offline tooling (N1 export); a
// deployment that predates the notebook schema reports an upgrade requirement.
func OpenExisting(ctx context.Context, path string) (*Backend, error) {
	db, err := sql.Open("sqlite", "file:"+path+"?mode=ro")
	if err != nil {
		return nil, fmt.Errorf("storage: open sqlite %s: %w", path, err)
	}
	db.SetMaxOpenConns(1)
	if err := requireNotebookHead(ctx, db); err != nil {
		_ = db.Close()
		return nil, err
	}
	return &Backend{db: db}, nil
}

// requireNotebookHead fails when the catalog is below the notebook migration.
func requireNotebookHead(ctx context.Context, db *sql.DB) error {
	var head int64
	err := db.QueryRowContext(ctx, `SELECT COALESCE(MAX(version),0) FROM schema_migrations`).Scan(&head)
	if err != nil {
		return fmt.Errorf("storage: read migration head: %w", err)
	}
	if head < 36 {
		return fmt.Errorf("storage predates the notebook schema (head %d < 36); run vivy once to upgrade storage", head)
	}
	return nil
}
