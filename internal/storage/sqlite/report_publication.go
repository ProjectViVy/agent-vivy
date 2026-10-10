package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"agent-vivy/internal/storage"
)

var _ storage.ReportPublicationStore = (*Backend)(nil)

// CommitReportPublication performs the atomic report publication:
//   - replay: same operation_key → return the committed receipt unchanged
//   - deleted target → destination_deleted recorded, document never recreated
//   - head unchanged since admission → promoted (entry head+version CAS)
//   - head moved (human or another generation) → candidate (revision only)
//   - no target entry → first publication creates entry+head together
//
// The mutation receipt, revision and generation row land in one tx.
func (b *Backend) CommitReportPublication(ctx context.Context, in storage.ReportPublicationInput) (storage.ReportPublicationReceipt, error) {
	tx, err := b.db.BeginTx(ctx, nil)
	if err != nil {
		return storage.ReportPublicationReceipt{}, fmt.Errorf("storage: report publication tx: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	// 1. Output receipt replay: an identical admitted persist returns its
	//    committed receipt instead of redoing work.
	var receipt storage.ReportPublicationReceipt
	var reqDigest string
	err = tx.QueryRowContext(ctx,
		`SELECT request_digest, resource_id, version, revision_id FROM notebook_mutations
		 WHERE scope = ? AND operation_key = ?`, in.Scope, in.OperationKey).
		Scan(&reqDigest, &receipt.EntryID, &receipt.Version, &receipt.RevisionID)
	if err == nil {
		var mode string
		_ = tx.QueryRowContext(ctx,
			`SELECT outcome_mode FROM report_generations WHERE scope = ? AND run_id = ?`,
			in.Generation.Scope, in.Generation.RunID).Scan(&mode)
		receipt.Outcome = mode
		if receipt.Outcome == "" {
			receipt.Outcome = storage.OutcomePromoted
		}
		committed = true
		return receipt, tx.Commit()
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return storage.ReportPublicationReceipt{}, fmt.Errorf("storage: report receipt lookup: %w", err)
	}
	// Deleted-path replay: the generation row exists but no mutation
	// receipt was written — return its recorded outcome unchanged.
	var genOutcome, genEntry string
	err = tx.QueryRowContext(ctx,
		`SELECT outcome_mode, entry_id FROM report_generations WHERE scope = ? AND run_id = ?`,
		in.Generation.Scope, in.Generation.RunID).Scan(&genOutcome, &genEntry)
	if err == nil {
		committed = true
		return storage.ReportPublicationReceipt{EntryID: genEntry, Outcome: genOutcome}, tx.Commit()
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return storage.ReportPublicationReceipt{}, fmt.Errorf("storage: report generation replay: %w", err)
	}

	now := time.Now().UnixMilli()

	// 2. Resolve target entry.
	var (
		entryID, head      string
		version, deletedAt int64
		found              bool
	)
	lookup := func(query string, args ...any) error {
		return tx.QueryRowContext(ctx, query, args...).Scan(&entryID, &head, &version, &deletedAt)
	}
	if in.EntryID != "" {
		err = lookup(`SELECT id, head_revision_id, version, deleted_at FROM notebook_entries WHERE scope = ? AND id = ?`, in.Scope, in.EntryID)
	} else {
		err = lookup(`SELECT id, head_revision_id, version, deleted_at FROM notebook_entries WHERE scope = ? AND report_series_id = ? AND report_window_id = ?`, in.Scope, in.SeriesID, in.WindowID)
	}
	switch {
	case err == nil:
		found = true
	case errors.Is(err, sql.ErrNoRows):
		found = false
	default:
		return storage.ReportPublicationReceipt{}, fmt.Errorf("storage: report target lookup: %w", err)
	}

	gen := in.Generation
	outcome := storage.OutcomePromoted
	var revisionID, parentHead, resolvedEntryID string
	var entryVersion int64 = 1

	if found {
		resolvedEntryID = entryID
		parentHead = head
		if deletedAt != 0 {
			outcome = storage.OutcomeDestinationDeleted
		}
	} else {
		resolvedEntryID = in.EntryID
		if resolvedEntryID == "" {
			resolvedEntryID, err = storage.NewNotebookID("ent-")
			if err != nil {
				return storage.ReportPublicationReceipt{}, err
			}
		}
	}
	if outcome != storage.OutcomeDestinationDeleted {
		revisionID, err = storage.NewNotebookID("rev-")
		if err != nil {
			return storage.ReportPublicationReceipt{}, err
		}
	}

	gen.EntryID = resolvedEntryID
	gen.RevisionID = revisionID

	if outcome == storage.OutcomeDestinationDeleted {
		gen.OutcomeMode = outcome
		if err := insertReportGenerationRow(ctx, tx, gen, now); err != nil {
			return storage.ReportPublicationReceipt{}, err
		}
		if err := tx.Commit(); err != nil {
			return storage.ReportPublicationReceipt{}, fmt.Errorf("storage: report publication commit: %w", err)
		}
		committed = true
		return storage.ReportPublicationReceipt{EntryID: resolvedEntryID, Version: version, Outcome: outcome}, nil
	}

	if !found {
		// First publication creates the entry row before its head revision
		// so the revision's FK resolves inside the same transaction.
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO notebook_entries (scope, id, section_id, kind, title, head_revision_id, version, report_series_id, report_window_id, created_at, updated_at, deleted_at)
			 VALUES (?,?,?,'report',?,?,1,?,?,?,?,0)`,
			in.Scope, resolvedEntryID, in.SectionID, in.Title, revisionID, in.SeriesID, in.WindowID, now, now); err != nil {
			return storage.ReportPublicationReceipt{}, fmt.Errorf("storage: report entry create: %w", err)
		}
	}
	// 4. Immutable revision row.
	var seq int64
	if err := tx.QueryRowContext(ctx,
		`SELECT COALESCE(MAX(sequence),0)+1 FROM notebook_revisions WHERE scope = ? AND entry_id = ?`,
		in.Scope, resolvedEntryID).Scan(&seq); err != nil {
		return storage.ReportPublicationReceipt{}, fmt.Errorf("storage: report revision sequence: %w", err)
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO notebook_revisions (scope, entry_id, revision_id, sequence, parent_revision_id, title, markdown, origin, base_revision_id, actor, created_at)
		 VALUES (?,?,?,?,?,?,?,?,?,?,?)`,
		in.Scope, resolvedEntryID, revisionID, seq, parentHead, in.Title, in.Markdown, "generated", "", in.Actor, now); err != nil {
		return storage.ReportPublicationReceipt{}, fmt.Errorf("storage: report revision insert: %w", err)
	}

	if found {
		if head != in.AdmittedHead || version != in.AdmittedVersion {
			outcome = storage.OutcomeCandidate
		} else {
			res, err := tx.ExecContext(ctx,
				`UPDATE notebook_entries SET title = ?, head_revision_id = ?, version = version + 1, updated_at = ?
				 WHERE scope = ? AND id = ? AND version = ? AND head_revision_id = ? AND deleted_at = 0`,
				in.Title, revisionID, now, in.Scope, resolvedEntryID, version, in.AdmittedHead)
			if err != nil {
				return storage.ReportPublicationReceipt{}, fmt.Errorf("storage: report head update: %w", err)
			}
			n, err := res.RowsAffected()
			if err != nil || n == 0 {
				outcome = storage.OutcomeCandidate
			}
		}
		if outcome == storage.OutcomeCandidate {
			entryVersion = version
		} else {
			entryVersion = version + 1
		}
	} else {
		entryVersion = 1
	}

	// 5. Output receipt + immutable generation provenance, committed
	//    atomically with the revision and head update.
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO notebook_mutations (scope, operation_key, request_digest, resource_kind, resource_id, version, revision_id, created_at)
		 VALUES (?,?,?,?,?,?,?,?)`,
		in.Scope, in.OperationKey, in.RequestDigest, "report", resolvedEntryID, entryVersion, revisionID, now); err != nil {
		return storage.ReportPublicationReceipt{}, fmt.Errorf("storage: report receipt insert: %w", err)
	}
	gen.OutcomeMode = outcome
	if err := insertReportGenerationRow(ctx, tx, gen, now); err != nil {
		return storage.ReportPublicationReceipt{}, err
	}
	if err := tx.Commit(); err != nil {
		return storage.ReportPublicationReceipt{}, fmt.Errorf("storage: report publication commit: %w", err)
	}
	committed = true
	return storage.ReportPublicationReceipt{
		EntryID: resolvedEntryID, RevisionID: revisionID, Version: entryVersion, Outcome: outcome,
	}, nil
}

func insertReportGenerationRow(ctx context.Context, tx *sql.Tx, gen storage.ReportGeneration, now int64) error {
	_, err := tx.ExecContext(ctx,
		`INSERT INTO report_generations (scope, run_id, period, series_id, window_id, entry_id, revision_id, config_revision, timezone, window_start_ms, window_end_ms, as_of_ms, input_digest, facts_digest, provider, model_id, outcome_mode, outcome_reason, facts_json, feedback_json, payload_json, created_at)
		 VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		gen.Scope, gen.RunID, gen.Period, gen.SeriesID, gen.WindowID, gen.EntryID, gen.RevisionID,
		gen.ConfigRevision, gen.Timezone, gen.WindowStartMs, gen.WindowEndMs, gen.AsOfMs,
		gen.InputDigest, gen.FactsDigest, gen.Provider, gen.ModelID, gen.OutcomeMode, gen.OutcomeReason,
		string(gen.FactsJSON), string(gen.FeedbackJSON), string(gen.PayloadJSON), now)
	if err != nil {
		return fmt.Errorf("storage: report generation insert: %w", err)
	}
	return nil
}
