package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
)

var _ storage.CronStore = (*Backend)(nil)

func (b *Backend) CreateCronJob(ctx context.Context, job domain.CronJob) error {
	schedule, payload, err := cronJSON(job)
	if err != nil {
		return err
	}
	if _, err := b.db.ExecContext(ctx, `
		INSERT INTO cron_jobs (id, name, enabled, schedule_json, payload_json, session_id,
			next_run_at_ms, last_run_at_ms, last_status, last_error, delete_after_run, created_at_ms, updated_at_ms)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)`,
		job.ID, job.Name, job.Enabled, schedule, payload, string(job.SessionID),
		job.State.NextRunAtMs, job.State.LastRunAtMs, job.State.LastStatus, job.State.LastError,
		job.DeleteAfterRun, job.CreatedAt, job.UpdatedAt); err != nil {
		return fmt.Errorf("storage: create cron job %s: %w", job.ID, err)
	}
	return nil
}

func (b *Backend) GetCronJob(ctx context.Context, id string) (domain.CronJob, error) {
	row := b.db.QueryRowContext(ctx, `
		SELECT id, name, enabled, schedule_json, payload_json, session_id,
			next_run_at_ms, last_run_at_ms, last_status, last_error, delete_after_run, created_at_ms, updated_at_ms
		FROM cron_jobs WHERE id = $1`, id)
	return scanCronJob(row)
}

func (b *Backend) ListCronJobs(ctx context.Context) ([]domain.CronJob, error) {
	rows, err := b.db.QueryContext(ctx, `
		SELECT id, name, enabled, schedule_json, payload_json, session_id,
			next_run_at_ms, last_run_at_ms, last_status, last_error, delete_after_run, created_at_ms, updated_at_ms
		FROM cron_jobs ORDER BY created_at_ms, id`)
	if err != nil {
		return nil, fmt.Errorf("storage: list cron jobs: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []domain.CronJob
	for rows.Next() {
		job, err := scanCronJob(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, job)
	}
	return out, rows.Err()
}

func (b *Backend) UpdateCronJob(ctx context.Context, job domain.CronJob) error {
	schedule, payload, err := cronJSON(job)
	if err != nil {
		return err
	}
	res, err := b.db.ExecContext(ctx, `
		UPDATE cron_jobs SET name = $1, enabled = $2, schedule_json = $3, payload_json = $4, session_id = $5,
			next_run_at_ms = $6, last_run_at_ms = $7, last_status = $8, last_error = $9, delete_after_run = $10, updated_at_ms = $11
		WHERE id = $12`,
		job.Name, job.Enabled, schedule, payload, string(job.SessionID),
		job.State.NextRunAtMs, job.State.LastRunAtMs, job.State.LastStatus, job.State.LastError,
		job.DeleteAfterRun, job.UpdatedAt, job.ID)
	if err != nil {
		return fmt.Errorf("storage: update cron job %s: %w", job.ID, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("storage: update cron job %s rows: %w", job.ID, err)
	}
	if n == 0 {
		return storage.ErrNotFound
	}
	return nil
}

func (b *Backend) DeleteCronJob(ctx context.Context, id string) error {
	res, err := b.db.ExecContext(ctx, `DELETE FROM cron_jobs WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("storage: delete cron job %s: %w", id, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("storage: delete cron job %s rows: %w", id, err)
	}
	if n == 0 {
		return storage.ErrNotFound
	}
	return nil
}

type cronRowScanner interface{ Scan(dest ...any) error }

func scanCronJob(row cronRowScanner) (domain.CronJob, error) {
	var job domain.CronJob
	var sid, lastStatus, lastError string
	var schedule, payload []byte
	err := row.Scan(&job.ID, &job.Name, &job.Enabled, &schedule, &payload, &sid,
		&job.State.NextRunAtMs, &job.State.LastRunAtMs, &lastStatus, &lastError,
		&job.DeleteAfterRun, &job.CreatedAt, &job.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.CronJob{}, storage.ErrNotFound
	}
	if err != nil {
		return domain.CronJob{}, fmt.Errorf("storage: scan cron job: %w", err)
	}
	job.SessionID = domain.SessionID(sid)
	job.State.LastStatus = lastStatus
	job.State.LastError = lastError
	if err := json.Unmarshal(schedule, &job.Schedule); err != nil {
		return domain.CronJob{}, fmt.Errorf("storage: decode cron schedule: %w", err)
	}
	if err := json.Unmarshal(payload, &job.Payload); err != nil {
		return domain.CronJob{}, fmt.Errorf("storage: decode cron payload: %w", err)
	}
	return job, nil
}

func cronJSON(job domain.CronJob) ([]byte, []byte, error) {
	schedule, err := json.Marshal(job.Schedule)
	if err != nil {
		return nil, nil, fmt.Errorf("storage: encode cron schedule: %w", err)
	}
	payload, err := json.Marshal(job.Payload)
	if err != nil {
		return nil, nil, fmt.Errorf("storage: encode cron payload: %w", err)
	}
	return schedule, payload, nil
}
