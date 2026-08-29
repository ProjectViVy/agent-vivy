package sqlite

import (
	"context"
	"path/filepath"
	"testing"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
)

func TestCronStoreCRUD(t *testing.T) {
	ctx := context.Background()
	backend, err := Open(ctx, filepath.Join(t.TempDir(), "crons.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer func() { _ = backend.Close() }()

	job := domain.CronJob{
		ID: "cron_abc123", Name: "nightly digest", Enabled: true,
		Schedule:  domain.CronSchedule{Kind: domain.CronScheduleCron, Expr: "0 9 * * *", TZ: "Asia/Shanghai"},
		Payload:   domain.CronPayload{Kind: domain.CronPayloadKindAgentTurn, Message: "write the digest"},
		State:     domain.CronJobState{NextRunAtMs: 1000},
		CreatedAt: 10, UpdatedAt: 10,
	}
	if err := backend.CreateCronJob(ctx, job); err != nil {
		t.Fatalf("create: %v", err)
	}

	got, err := backend.GetCronJob(ctx, job.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Name != job.Name || got.Schedule.Expr != job.Schedule.Expr || got.Schedule.TZ != job.Schedule.TZ ||
		got.Payload.Message != job.Payload.Message || !got.Enabled || got.State.NextRunAtMs != 1000 {
		t.Fatalf("round trip mismatch: %+v", got)
	}

	got.State.LastRunAtMs = 2000
	got.State.LastStatus = "ok"
	got.UpdatedAt = 3000
	if err := backend.UpdateCronJob(ctx, got); err != nil {
		t.Fatalf("update: %v", err)
	}
	reread, err := backend.GetCronJob(ctx, job.ID)
	if err != nil {
		t.Fatalf("get after update: %v", err)
	}
	if reread.State.LastStatus != "ok" || reread.State.LastRunAtMs != 2000 {
		t.Fatalf("update lost: %+v", reread)
	}

	second := job
	second.ID = "cron_def456"
	second.CreatedAt = 5
	second.UpdatedAt = 5
	if err := backend.CreateCronJob(ctx, second); err != nil {
		t.Fatalf("create second: %v", err)
	}
	jobs, err := backend.ListCronJobs(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(jobs) != 2 || jobs[0].ID != second.ID {
		t.Fatalf("list order/content = %+v", jobs)
	}

	if err := backend.DeleteCronJob(ctx, job.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := backend.GetCronJob(ctx, job.ID); err != storage.ErrNotFound {
		t.Fatalf("get deleted = %v, want ErrNotFound", err)
	}
	if err := backend.DeleteCronJob(ctx, job.ID); err != storage.ErrNotFound {
		t.Fatalf("double delete = %v, want ErrNotFound", err)
	}
}

func TestCronStoreUpdateMissingIsNotFound(t *testing.T) {
	ctx := context.Background()
	backend, err := Open(ctx, filepath.Join(t.TempDir(), "crons2.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer func() { _ = backend.Close() }()

	ghost := domain.CronJob{ID: "cron_ghost", Name: "ghost", CreatedAt: 1, UpdatedAt: 1,
		Payload: domain.CronPayload{Kind: "agent_turn"}}
	if err := backend.UpdateCronJob(ctx, ghost); err != storage.ErrNotFound {
		t.Fatalf("update missing = %v, want ErrNotFound", err)
	}
}
