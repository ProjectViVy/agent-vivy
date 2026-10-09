package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"agent-vivy/internal/storage"
)

var _ storage.ReportGenerationStore = (*Backend)(nil)

const reportGenerationCols = "scope, run_id, period, series_id, window_id, entry_id, revision_id, config_revision, timezone, window_start_ms, window_end_ms, as_of_ms, input_digest, facts_digest, provider, model_id, outcome_mode, outcome_reason, facts_json, feedback_json, payload_json, created_at"

func scanReportGeneration(row interface{ Scan(dest ...any) error }) (storage.ReportGeneration, error) {
	var g storage.ReportGeneration
	err := row.Scan(&g.Scope, &g.RunID, &g.Period, &g.SeriesID, &g.WindowID, &g.EntryID, &g.RevisionID,
		&g.ConfigRevision, &g.Timezone, &g.WindowStartMs, &g.WindowEndMs, &g.AsOfMs,
		&g.InputDigest, &g.FactsDigest, &g.Provider, &g.ModelID, &g.OutcomeMode, &g.OutcomeReason,
		&g.FactsJSON, &g.FeedbackJSON, &g.PayloadJSON, &g.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return storage.ReportGeneration{}, storage.ErrNotFound
	}
	if err != nil {
		return storage.ReportGeneration{}, fmt.Errorf("storage: scan report generation: %w", err)
	}
	return g, nil
}

func (b *Backend) InsertReportGeneration(ctx context.Context, g storage.ReportGeneration) error {
	_, err := b.db.ExecContext(ctx,
		"INSERT INTO report_generations ("+reportGenerationCols+") VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)",
		g.Scope, g.RunID, g.Period, g.SeriesID, g.WindowID, g.EntryID, g.RevisionID,
		g.ConfigRevision, g.Timezone, g.WindowStartMs, g.WindowEndMs, g.AsOfMs,
		g.InputDigest, g.FactsDigest, g.Provider, g.ModelID, g.OutcomeMode, g.OutcomeReason,
		string(g.FactsJSON), string(g.FeedbackJSON), string(g.PayloadJSON), g.CreatedAt)
	if err != nil {
		return fmt.Errorf("storage: insert report generation %s: %w", g.RunID, err)
	}
	return nil
}

func (b *Backend) GetReportGenerationByRun(ctx context.Context, scope, runID string) (storage.ReportGeneration, error) {
	row := b.db.QueryRowContext(ctx,
		"SELECT "+reportGenerationCols+" FROM report_generations WHERE scope = ? AND run_id = ?", scope, runID)
	return scanReportGeneration(row)
}

func (b *Backend) ListReportGenerations(ctx context.Context, scope, seriesID, fromWindow, toWindow string) ([]storage.ReportGeneration, error) {
	rows, err := b.db.QueryContext(ctx,
		"SELECT "+reportGenerationCols+" FROM report_generations WHERE scope = ? AND series_id = ? AND window_id >= ? AND window_id <= ? ORDER BY window_id, run_id",
		scope, seriesID, fromWindow, toWindow)
	if err != nil {
		return nil, fmt.Errorf("storage: list report generations: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []storage.ReportGeneration
	for rows.Next() {
		g, err := scanReportGeneration(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, rows.Err()
}
