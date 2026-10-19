package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"agent-vivy/internal/domain"
	nb "agent-vivy/internal/notebookcontract"
	rc "agent-vivy/internal/reportcontract"
	"agent-vivy/internal/storage"
)

var _ storage.ReportSettingsStore = (*Backend)(nil)

func reportSettingsJob(settings rc.ReportSettings, enabled bool, now int64) domain.CronJob {
	return domain.CronJob{
		ID:       storage.ReportSettingsJobID(settings.Scope, settings.Period),
		Name:     "vivy.reports." + string(settings.Period),
		Enabled:  enabled,
		Schedule: domain.CronSchedule{Kind: domain.CronScheduleCron, Expr: settings.ScheduleExpr, TZ: settings.Timezone},
		Payload: domain.CronPayload{Kind: domain.CronPayloadKindReport, Report: &domain.CronReportPayload{
			Scope: settings.Scope, Period: string(settings.Period), Timezone: settings.Timezone,
			SectionID: settings.SectionID, Provider: settings.Provider, ModelID: settings.ModelID}},
		CreatedAt: now, UpdatedAt: now,
		Revision: settings.Revision,
	}
}

func jobToReportSettings(job domain.CronJob) (rc.ReportSettings, error) {
	if job.Payload.Kind != domain.CronPayloadKindReport || job.Payload.Report == nil {
		return rc.ReportSettings{}, fmt.Errorf("storage: cron job %s is not a report settings row", job.ID)
	}
	p := job.Payload.Report
	return rc.ReportSettings{
		Scope: p.Scope, Period: rc.Period(p.Period), Timezone: p.Timezone, SectionID: p.SectionID,
		Provider: p.Provider, ModelID: p.ModelID, Enabled: job.Enabled,
		ScheduleExpr: job.Schedule.Expr, Revision: job.Revision,
	}, nil
}

func (b *Backend) GetReportSettings(ctx context.Context, scope string, period rc.Period) (rc.ReportSettings, error) {
	job, err := b.GetCronJob(ctx, storage.ReportSettingsJobID(scope, period))
	if err != nil {
		return rc.ReportSettings{}, err
	}
	return jobToReportSettings(job)
}

func (b *Backend) EnsureReportSettings(ctx context.Context, scope string, period rc.Period, defaults rc.ReportSettings) (rc.ReportSettings, error) {
	id := storage.ReportSettingsJobID(scope, period)
	if job, err := b.GetCronJob(ctx, id); err == nil {
		return jobToReportSettings(job)
	} else if !errors.Is(err, storage.ErrNotFound) {
		return rc.ReportSettings{}, err
	}
	defaults.Scope = scope
	defaults.Period = period
	if defaults.Revision == 0 {
		defaults.Revision = 1
	}
	job := reportSettingsJob(defaults, defaults.Enabled, time.Now().UnixMilli())
	if err := b.CreateCronJob(ctx, job); err != nil {
		// A concurrent ensure won: reread the committed row.
		if existing, getErr := b.GetReportSettings(ctx, scope, period); getErr == nil {
			return existing, nil
		}
		return rc.ReportSettings{}, err
	}
	return defaults, nil
}

func (b *Backend) WriteReportSettingsCAS(ctx context.Context, expected int64, settings rc.ReportSettings) (rc.ReportSettings, error) {
	job := reportSettingsJob(settings, settings.Enabled, time.Now().UnixMilli())
	job.Revision = expected
	updated, err := b.UpdateCronJobCAS(ctx, job, expected)
	if err != nil {
		return rc.ReportSettings{}, err
	}
	return jobToReportSettings(updated)
}

// reportSettingsWriteRequest is the canonical digest input of a settings
// commit: scope binds the write to its trusted namespace; every other
// field is the caller's desired row.
type reportSettingsWriteRequest struct {
	Scope string `json:"scope"`
	rc.ReportSettingsWrite
}

func (b *Backend) CommitReportSettings(ctx context.Context, mc nb.MutationContext, expected int64, settings rc.ReportSettings, nextRunAtMs int64) (rc.ReportSettings, nb.MutationReceipt, error) {
	req := reportSettingsWriteRequest{
		Scope: settings.Scope,
		ReportSettingsWrite: rc.ReportSettingsWrite{
			Period: settings.Period, ExpectedRevision: expected,
			Timezone: settings.Timezone, SectionID: settings.SectionID,
			Provider: settings.Provider, ModelID: settings.ModelID,
			Enabled: settings.Enabled, ScheduleExpr: settings.ScheduleExpr,
		},
	}
	job := reportSettingsJob(settings, settings.Enabled, time.Now().UnixMilli())
	job.State.NextRunAtMs = nextRunAtMs
	committed := settings
	committed.Revision = expected + 1
	receipt, err := notebookStore{db: b.db}.mutate(ctx, mc, storage.ReportSettingsMutationKind, req,
		func(tx *sql.Tx, now int64) (nb.MutationReceipt, error) {
			job.UpdatedAt = now
			schedule, payload, err := cronJSON(job)
			if err != nil {
				return nb.MutationReceipt{}, err
			}
			res, err := tx.ExecContext(ctx, `
				UPDATE cron_jobs SET name = ?, enabled = ?, schedule_json = ?, payload_json = ?, session_id = ?,
					next_run_at_ms = ?, last_run_at_ms = ?, last_status = ?, last_error = ?, delete_after_run = ?,
					updated_at_ms = ?, revision = revision + 1
				WHERE id = ? AND revision = ?`,
				job.Name, boolInt(job.Enabled), schedule, payload, string(job.SessionID),
				job.State.NextRunAtMs, job.State.LastRunAtMs, job.State.LastStatus, job.State.LastError,
				boolInt(job.DeleteAfterRun), job.UpdatedAt, job.ID, expected)
			if err != nil {
				return nb.MutationReceipt{}, fmt.Errorf("storage: commit report settings %s: %w", job.ID, err)
			}
			if n, _ := res.RowsAffected(); n == 0 {
				var exists string
				scanErr := tx.QueryRowContext(ctx, `SELECT id FROM cron_jobs WHERE id = ?`, job.ID).Scan(&exists)
				if errors.Is(scanErr, sql.ErrNoRows) {
					return nb.MutationReceipt{}, storage.ErrNotFound
				}
				if scanErr != nil {
					return nb.MutationReceipt{}, scanErr
				}
				return nb.MutationReceipt{}, storage.ErrRevisionConflict
			}
			return nb.MutationReceipt{ResourceID: job.ID, Version: committed.Revision}, nil
		})
	if err != nil {
		return rc.ReportSettings{}, nb.MutationReceipt{}, err
	}
	if receipt.Replayed {
		row, getErr := b.GetReportSettings(ctx, settings.Scope, settings.Period)
		if getErr != nil {
			return rc.ReportSettings{}, nb.MutationReceipt{}, getErr
		}
		return row, receipt, nil
	}
	return committed, receipt, nil
}
