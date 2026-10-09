package sqlite

import (
	"context"
	"errors"
	"fmt"
	"time"

	"agent-vivy/internal/domain"
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
