package runtime

import (
	"context"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
	"agent-vivy/internal/storage/sqlite"
	"agent-vivy/internal/testsupport"
	"agent-vivy/internal/tools"
)

// newCronTestService mirrors newTestService but wires the session and cron
// stores the scheduler needs.
func newCronTestService(t *testing.T) (*Service, *sqlite.Backend) {
	t.Helper()
	ctx := context.Background()

	backend, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "cron.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = backend.Close() })

	ts, err := tools.Builtin(backend).Resolve([]string{tools.EchoInfoName})
	if err != nil {
		t.Fatalf("resolve tools: %v", err)
	}
	eng, err := NewEngine(ctx, WrapModel(testsupport.NewEchoModel()), ts, EngineConfig{StreamBuffer: 8, MaxEventPayloadBytes: 64 << 10})
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}
	svc := NewService(eng, "test", "test-model", ServiceDeps{
		Journal: backend, Runs: backend, Messages: backend, Notes: backend,
		Sessions: backend, Crons: backend, Sink: newTestSink(),
	})
	return svc, backend
}

// waitCronState polls the job row until want(status, nextRunAtMs) holds.
func waitCronState(t *testing.T, store storage.CronStore, id string, want func(domain.CronJob) bool) domain.CronJob {
	t.Helper()
	// The full product gate runs runtime alongside the app/RPC packages and
	// can heavily contend on Windows CI. Keep polling bounded, but allow the
	// asynchronous agent turn to settle under that representative load.
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		job, err := store.GetCronJob(context.Background(), id)
		if err == nil && want(job) {
			return job
		}
		time.Sleep(10 * time.Millisecond)
	}
	job, _ := store.GetCronJob(context.Background(), id)
	t.Fatalf("cron job %s never reached wanted state; last = %+v", id, job)
	return domain.CronJob{}
}

func createTestJob(t *testing.T, store storage.CronStore, mutate func(*domain.CronJob)) domain.CronJob {
	t.Helper()
	job := domain.CronJob{
		ID:      "cron_test01",
		Name:    "test job",
		Enabled: true,
		Schedule: domain.CronSchedule{
			Kind: domain.CronScheduleEvery, EveryMs: 120,
		},
		Payload:   domain.CronPayload{Kind: domain.CronPayloadKindAgentTurn, Message: "cron ping"},
		CreatedAt: time.Now().UnixMilli(),
		UpdatedAt: time.Now().UnixMilli(),
	}
	if mutate != nil {
		mutate(&job)
	}
	if err := store.CreateCronJob(context.Background(), job); err != nil {
		t.Fatalf("create job: %v", err)
	}
	return job
}

func TestCronSchedulerFiresDueJobAndWritesBack(t *testing.T) {
	svc, backend := newCronTestService(t)
	ctx := context.Background()

	now := time.Now().UnixMilli()
	job := createTestJob(t, backend, func(j *domain.CronJob) {
		// This test observes the first write-back, not repeated firing. Keep the
		// next recurrence visible while a loaded Windows runner polls SQLite.
		j.Schedule.EveryMs = 30_000
		j.State.NextRunAtMs = now + 80 // first fire ~80ms out
	})

	svc.StartCronScheduler(ctx, CronSchedulerOptions{MaxSleep: 20 * time.Millisecond, TerminalPoll: 10 * time.Millisecond})
	defer svc.StopCronScheduler()

	settled := waitCronState(t, backend, job.ID, func(j domain.CronJob) bool {
		return j.State.LastStatus == "ok" && j.State.NextRunAtMs > time.Now().UnixMilli()
	})
	if settled.State.LastRunAtMs == 0 {
		t.Fatalf("last_run_at_ms not recorded: %+v", settled)
	}
	// The run landed in a lazily created dedicated session.
	if settled.SessionID == "" {
		t.Fatalf("session id not bound: %+v", settled)
	}
	if _, err := backend.GetSession(ctx, settled.SessionID); err != nil {
		t.Fatalf("cron session missing: %v", err)
	}
	if settled.UpdatedAt < settled.CreatedAt {
		t.Fatalf("updated_at not advanced: %+v", settled)
	}
}

func TestCronTriggerManualConflictAndDisabledJob(t *testing.T) {
	svc, backend := newCronTestService(t)
	ctx := context.Background()
	job := createTestJob(t, backend, func(j *domain.CronJob) {
		j.Enabled = false
		j.State.NextRunAtMs = 0
	})

	started := time.Now().UnixMilli()
	if _, err := svc.TriggerCron(ctx, job.ID); err != nil {
		t.Fatalf("trigger: %v", err)
	}
	if _, running := svc.ActiveCronRun(job.ID); !running {
		t.Fatal("triggered job is not marked running")
	}
	if _, err := svc.TriggerCron(ctx, job.ID); err == nil {
		t.Fatal("second trigger succeeded, want ErrCronRunning")
	}

	settled := waitCronState(t, backend, job.ID, func(j domain.CronJob) bool {
		return j.State.LastStatus == "ok"
	})
	if settled.Enabled {
		t.Fatalf("disabled job became enabled: %+v", settled)
	}
	if settled.State.LastRunAtMs < started {
		t.Fatalf("last_run_at_ms before trigger: %+v", settled)
	}
	if _, running := svc.ActiveCronRun(job.ID); running {
		t.Fatal("active run not cleared after settle")
	}
}

func TestCronStopCancelsActiveRun(t *testing.T) {
	svc, backend := newCronTestService(t)
	ctx := context.Background()
	job := createTestJob(t, backend, nil)

	if _, err := svc.TriggerCron(ctx, job.ID); err != nil {
		t.Fatalf("trigger: %v", err)
	}
	if !svc.StopCron(job.ID) {
		t.Fatal("stop reported nothing running")
	}
	waitCronState(t, backend, job.ID, func(j domain.CronJob) bool {
		return j.State.LastStatus == "error"
	})
	// A second stop conflicts: nothing runs anymore.
	if svc.StopCron(job.ID) {
		t.Fatal("second stop succeeded, want false")
	}
}

func TestCronAtJobDisablesAfterRun(t *testing.T) {
	svc, backend := newCronTestService(t)
	ctx := context.Background()
	now := time.Now().UnixMilli()
	job := createTestJob(t, backend, func(j *domain.CronJob) {
		// Leave enough startup margin that a loaded Windows scheduler cannot
		// classify this fresh job as an offline, already-missed one-shot.
		j.Schedule = domain.CronSchedule{Kind: domain.CronScheduleAt, AtMs: now + 5_000}
		j.State.NextRunAtMs = now + 5_000
	})

	svc.StartCronScheduler(ctx, CronSchedulerOptions{MaxSleep: 20 * time.Millisecond, TerminalPoll: 10 * time.Millisecond})
	defer svc.StopCronScheduler()

	settled := waitCronState(t, backend, job.ID, func(j domain.CronJob) bool {
		return j.State.LastStatus == "ok" && !j.Enabled && j.State.NextRunAtMs == 0
	})
	if settled.State.LastRunAtMs == 0 {
		t.Fatalf("one-shot last_run_at_ms missing: %+v", settled)
	}
}

func TestCronAtJobDeletesAfterSuccessfulRun(t *testing.T) {
	svc, backend := newCronTestService(t)
	ctx := context.Background()
	now := time.Now().UnixMilli()
	job := createTestJob(t, backend, func(j *domain.CronJob) {
		// See TestCronAtJobDisablesAfterRun: the recovery contract intentionally
		// disables truly past one-shots, so this wiring test needs startup margin.
		j.Schedule = domain.CronSchedule{Kind: domain.CronScheduleAt, AtMs: now + 5_000}
		j.State.NextRunAtMs = now + 5_000
		j.DeleteAfterRun = true
	})

	svc.StartCronScheduler(ctx, CronSchedulerOptions{MaxSleep: 20 * time.Millisecond, TerminalPoll: 10 * time.Millisecond})
	defer svc.StopCronScheduler()

	// The full `just ci` run saturates cores and slows the whole
	// fire→run→settle pipeline; a fixed 5s budget tripped intermittently
	// (TFLAKE-CRON). Poll generously and dump the settled row on failure.
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := backend.GetCronJob(ctx, job.ID); err == storage.ErrNotFound {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	settled, err := backend.GetCronJob(ctx, job.ID)
	if err != nil {
		t.Fatalf("delete_after_run job not deleted; final read: %v", err)
	}
	t.Fatalf("delete_after_run job was not deleted; settled state: %+v", settled)
}

func TestCronRecoveryDisablesPastOneShot(t *testing.T) {
	svc, backend := newCronTestService(t)
	ctx := context.Background()
	now := time.Now().UnixMilli()
	job := createTestJob(t, backend, func(j *domain.CronJob) {
		j.Schedule = domain.CronSchedule{Kind: domain.CronScheduleAt, AtMs: now - 1000}
		j.State.NextRunAtMs = now - 1000
	})

	svc.StartCronScheduler(ctx, CronSchedulerOptions{MaxSleep: time.Second})
	settled := waitCronState(t, backend, job.ID, func(j domain.CronJob) bool {
		return !j.Enabled && j.State.NextRunAtMs == 0
	})
	_ = settled
	svc.StopCronScheduler()
}

func TestCronManualTriggerOfMissingJob(t *testing.T) {
	svc, _ := newCronTestService(t)
	if _, err := svc.TriggerCron(context.Background(), "cron_nope"); err != storage.ErrNotFound {
		t.Fatalf("trigger missing = %v, want storage.ErrNotFound", err)
	}
}

// TFLAKE-CRON root fix: the delete-after-run contract is settleCronRun's
// branch, so it is asserted synchronously here — no scheduler loop, no
// wall-clock budget for the async fire→run→watch pipeline. The end-to-end
// test above stays as the wiring canary.
func TestCronSettleDeletesSuccessfulAtJob(t *testing.T) {
	svc, backend := newCronTestService(t)
	ctx := context.Background()
	st := svc.ensureCronState(CronSchedulerOptions{})
	now := time.Now().UnixMilli()
	job := createTestJob(t, backend, func(j *domain.CronJob) {
		j.Schedule = domain.CronSchedule{Kind: domain.CronScheduleAt, AtMs: now - 1000}
		j.State.NextRunAtMs = now - 1000
		j.DeleteAfterRun = true
	})
	entry := &cronActiveRun{snapshot: CronRunSnapshot{JobID: job.ID, RunID: "run_settle01", StartedAtMs: now}}

	svc.settleCronRun(ctx, st, entry, job, domain.RunCompleted)

	if _, err := backend.GetCronJob(ctx, job.ID); err != storage.ErrNotFound {
		t.Fatalf("successful delete_after_run one-shot = %v, want storage.ErrNotFound", err)
	}
	if _, running := svc.ActiveCronRun(job.ID); running {
		t.Fatal("active run not cleared after settle")
	}
}

// The failure twin: a failed one-shot with delete_after_run must NOT be
// deleted — the operator needs the error on the row to know why.
func TestCronSettleKeepsFailedAtJobDisabled(t *testing.T) {
	svc, backend := newCronTestService(t)
	ctx := context.Background()
	st := svc.ensureCronState(CronSchedulerOptions{})
	now := time.Now().UnixMilli()
	job := createTestJob(t, backend, func(j *domain.CronJob) {
		j.Schedule = domain.CronSchedule{Kind: domain.CronScheduleAt, AtMs: now - 1000}
		j.State.NextRunAtMs = now - 1000
		j.DeleteAfterRun = true
	})
	entry := &cronActiveRun{snapshot: CronRunSnapshot{JobID: job.ID, RunID: "run_settle02", StartedAtMs: now}}

	svc.settleCronRun(ctx, st, entry, job, domain.RunFailed)

	settled, err := backend.GetCronJob(ctx, job.ID)
	if err != nil {
		t.Fatalf("failed delete_after_run one-shot must survive: %v", err)
	}
	if settled.Enabled || settled.State.NextRunAtMs != 0 {
		t.Fatalf("failed one-shot must be disabled without a next fire: %+v", settled)
	}
	if settled.State.LastStatus != "error" {
		t.Fatalf("failed one-shot must record the error status: %+v", settled.State)
	}
}

type fakeDeliverer struct {
	mu         sync.Mutex
	deliveries []struct{ channel, to, content string }
}

func (f *fakeDeliverer) Deliver(_ context.Context, channel, to, content string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.deliveries = append(f.deliveries, struct{ channel, to, content string }{channel, to, content})
	return nil
}

func (f *fakeDeliverer) snapshot() []struct{ channel, to, content string } {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]struct{ channel, to, content string }(nil), f.deliveries...)
}

func TestCronSettleDeliversOutboundWhenEnabled(t *testing.T) {
	svc, backend := newCronTestService(t)
	ctx := context.Background()
	deliv := &fakeDeliverer{}
	svc.deps.Channels = deliv

	st := svc.ensureCronState(CronSchedulerOptions{})
	now := time.Now().UnixMilli()
	job := createTestJob(t, backend, func(j *domain.CronJob) {
		j.Payload.Deliver = true
		j.Payload.Channel = "telegram"
		j.Payload.To = "chat_test_1"
	})
	session := domain.Session{ID: "sess_cron_deliv", Title: "Cron deliv test", CreatedAt: now}
	if err := backend.CreateSession(ctx, session); err != nil {
		t.Fatalf("create session: %v", err)
	}
	job.SessionID = session.ID
	if err := backend.UpdateCronJob(ctx, job); err != nil {
		t.Fatalf("update job: %v", err)
	}

	msg := domain.Message{
		ID:        "msg_deliv_1",
		SessionID: session.ID,
		RunID:     "run_deliv_1",
		Role:      domain.RoleAssistant,
		Content:   "Cron job summary result",
		CreatedAt: now,
	}
	if err := backend.AppendMessage(ctx, msg); err != nil {
		t.Fatalf("append message: %v", err)
	}

	entry := &cronActiveRun{snapshot: CronRunSnapshot{JobID: job.ID, RunID: "run_deliv_1", StartedAtMs: now}}
	svc.settleCronRun(ctx, st, entry, job, domain.RunCompleted)

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if len(deliv.snapshot()) > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	deliveries := deliv.snapshot()
	if len(deliveries) != 1 {
		t.Fatalf("expected 1 delivery, got %d", len(deliveries))
	}
	if deliveries[0].channel != "telegram" || deliveries[0].to != "chat_test_1" {
		t.Errorf("unexpected delivery target: %+v", deliveries[0])
	}
	if deliveries[0].content != "Cron job summary result" {
		t.Errorf("expected assistant text in delivery, got %q", deliveries[0].content)
	}
}

func TestCronRecoveryPastDueRecurringJobFiresOnceOnWakeAndSkipsStorm(t *testing.T) {
	svc, backend := newCronTestService(t)
	ctx := context.Background()
	now := time.Now().UnixMilli()

	job := createTestJob(t, backend, func(j *domain.CronJob) {
		// The recovery behavior needs one fire and a future write-back. A long
		// recurrence keeps that state observable under loaded Windows CI.
		j.Schedule = domain.CronSchedule{Kind: domain.CronScheduleEvery, EveryMs: 30_000}
		j.State.NextRunAtMs = now - 5000
	})

	if err := svc.recoverCron(ctx); err != nil {
		t.Fatalf("recoverCron: %v", err)
	}
	recovered, err := backend.GetCronJob(ctx, job.ID)
	if err != nil {
		t.Fatalf("get recovered job: %v", err)
	}
	if recovered.State.NextRunAtMs != now-5000 {
		t.Fatalf("recoverCron should keep past-due NextRunAtMs, got %d", recovered.State.NextRunAtMs)
	}

	svc.StartCronScheduler(ctx, CronSchedulerOptions{MaxSleep: 20 * time.Millisecond, TerminalPoll: 10 * time.Millisecond})
	defer svc.StopCronScheduler()

	settled := waitCronState(t, backend, job.ID, func(j domain.CronJob) bool {
		return j.State.LastStatus == "ok" && j.State.NextRunAtMs > time.Now().UnixMilli()
	})
	if settled.State.LastRunAtMs == 0 {
		t.Fatalf("last_run_at_ms not recorded: %+v", settled)
	}
	if settled.State.NextRunAtMs <= now {
		t.Fatalf("settled next_run_at_ms must jump to future, got %d <= %d", settled.State.NextRunAtMs, now)
	}
}
