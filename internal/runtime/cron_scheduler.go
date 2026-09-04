package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
)

// Cron trigger sources (diva's CronTrigger).
const (
	CronTriggerScheduled = "scheduled"
	CronTriggerManual    = "manual"
)

// ErrCronRunning is returned when a job already owns an active run; the
// control plane maps it to a conflict error.
var ErrCronRunning = errors.New("runtime: cron job already has an active run")

// ErrCronUnsupportedKind is returned for stored jobs whose payload kind the
// scheduler does not implement (only agent_turn fires today).
var ErrCronUnsupportedKind = errors.New("runtime: cron payload kind is not supported")

// CronRunSnapshot describes one in-flight cron run. It mirrors the UI's
// CronRunSnapshot shape (the RPC layer re-tags it).
type CronRunSnapshot struct {
	RunID             domain.RunID
	JobID             string
	StartedAtMs       int64
	LastHeartbeatAtMs int64
	Trigger           string
}

// CronRunner is the control-plane surface for manual triggers, stops and
// live snapshots. The scheduler below is the implementation; RPC handlers
// only see this interface.
type CronRunner interface {
	// TriggerCron fires a job immediately outside its schedule. It fails
	// with storage.ErrNotFound for unknown jobs and ErrCronRunning when a
	// run is already in flight.
	TriggerCron(ctx context.Context, jobID string) (domain.CronJob, error)
	// StopCron cancels the job's active run; false when nothing is
	// running.
	StopCron(jobID string) bool
	// ActiveCronRun returns the in-flight snapshot for the job, if any.
	ActiveCronRun(jobID string) (CronRunSnapshot, bool)
}

// CronSchedulerOptions tunes the scheduler for tests; zero values are the
// production defaults.
type CronSchedulerOptions struct {
	// MaxSleep caps the armed timer so store edits that skipped the kick
	// (clock steps, operator writes) are re-read at least this often.
	MaxSleep time.Duration
	// TerminalPoll is the watcher's run-status polling cadence. The bus
	// cannot deliver terminal events to late subscribers (they close
	// instead), so terminal detection polls the durable run row.
	TerminalPoll time.Duration
	// Now overrides the schedule clock in tests.
	Now func() time.Time
}

// cronState is the scheduler's process state; it exists only when the
// service is wired with a CronStore. One run per job is tracked in memory
// only — after a restart no cron run is considered active (the organism
// lease guarantees a single scheduler process, so no cross-process guard
// is needed).
type cronState struct {
	mu      sync.Mutex
	running bool
	cancel  context.CancelFunc
	wake    chan struct{}
	wg      sync.WaitGroup
	active  map[string]*cronActiveRun

	now          func() time.Time
	maxSleep     time.Duration
	terminalPoll time.Duration
}

type cronActiveRun struct {
	snapshot CronRunSnapshot
}

func (s *Service) ensureCronState(opts CronSchedulerOptions) *cronState {
	s.cronInit.Lock()
	defer s.cronInit.Unlock()
	if s.cron == nil {
		s.cron = &cronState{
			wake:         make(chan struct{}, 1),
			active:       make(map[string]*cronActiveRun),
			now:          opts.Now,
			maxSleep:     opts.MaxSleep,
			terminalPoll: opts.TerminalPoll,
		}
		if s.cron.now == nil {
			s.cron.now = time.Now
		}
		if s.cron.maxSleep <= 0 {
			s.cron.maxSleep = 30 * time.Second
		}
		if s.cron.terminalPoll <= 0 {
			s.cron.terminalPoll = time.Second
		}
	}
	return s.cron
}

// StartCronScheduler launches the armed-timer loop. Nil CronStore keeps
// the whole cron family disabled. Startup recovery recomputes every
// enabled job's next fire (skipping missed occurrences, diva semantics)
// and disables past-due one-shot jobs.
func (s *Service) StartCronScheduler(parent context.Context, opts CronSchedulerOptions) {
	if s.deps.Crons == nil {
		return
	}
	st := s.ensureCronState(opts)

	st.mu.Lock()
	if st.running {
		st.mu.Unlock()
		return
	}
	ctx, cancel := context.WithCancel(parent)
	st.cancel = cancel
	st.running = true
	st.mu.Unlock()

	st.wg.Add(1)
	go func() {
		defer st.wg.Done()
		if err := s.recoverCron(ctx); err != nil {
			slog.Warn("cron startup recovery failed", "err", err)
		}
		st.loop(ctx, s)
	}()
}

// StopCronScheduler stops the timer loop and waits (bounded) for in-flight
// terminal watchers to write their state back. Callers shutting down must
// invoke this before the storage backend closes, but after CancelAll the
// runs reach terminal states while the journal is still open (E4).
func (s *Service) StopCronScheduler() {
	if s.cron == nil {
		return
	}
	st := s.cron
	st.mu.Lock()
	if !st.running {
		st.mu.Unlock()
		return
	}
	st.running = false
	cancel := st.cancel
	st.cancel = nil
	st.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	done := make(chan struct{})
	go func() {
		st.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		slog.Warn("cron scheduler drain timed out; terminal write-backs may be missing")
	}
}

// KickCronScheduler wakes the armed timer so a job mutation is reflected
// without waiting out the current sleep. Safe to call at any time.
func (s *Service) KickCronScheduler() {
	if s.cron == nil {
		return
	}
	select {
	case s.cron.wake <- struct{}{}:
	default:
	}
}

// loop sleeps until the nearest next_run_at (capped by MaxSleep), then
// fires every due job sequentially.
func (st *cronState) loop(ctx context.Context, s *Service) {
	for {
		nowMs := st.now().UnixMilli()
		wait := st.maxSleep
		if next := s.nextCronWakeMs(ctx); next > 0 {
			d := time.Duration(next-nowMs) * time.Millisecond
			if d < 0 {
				d = 0
			}
			if d < wait {
				wait = d
			}
		}
		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-st.wake:
			timer.Stop()
		case <-timer.C:
		}
		s.fireDueCronJobs(ctx)
	}
}

// nextCronWakeMs returns the smallest next_run_at_ms among enabled jobs
// (0 when none), straight from the store.
func (s *Service) nextCronWakeMs(ctx context.Context) int64 {
	jobs, err := s.deps.Crons.ListCronJobs(ctx)
	if err != nil {
		slog.Warn("cron list for wake time failed", "err", err)
		return 0
	}
	var min int64
	for _, job := range jobs {
		if !job.Enabled || job.State.NextRunAtMs <= 0 {
			continue
		}
		if min == 0 || job.State.NextRunAtMs < min {
			min = job.State.NextRunAtMs
		}
	}
	return min
}

// fireDueCronJobs starts every enabled, due, single-run job. Jobs already
// holding an active run are skipped; their watcher reschedules on
// completion.
func (s *Service) fireDueCronJobs(ctx context.Context) {
	jobs, err := s.deps.Crons.ListCronJobs(ctx)
	if err != nil {
		slog.Warn("cron list for due fire failed", "err", err)
		return
	}
	nowMs := s.cron.now().UnixMilli()
	for _, job := range jobs {
		if !job.Enabled || job.State.NextRunAtMs <= 0 || nowMs < job.State.NextRunAtMs {
			continue
		}
		if _, err := s.fireCron(ctx, job, CronTriggerScheduled); err != nil {
			if !errors.Is(err, ErrCronRunning) {
				slog.Warn("cron scheduled fire failed", "job", job.ID, "err", err)
			}
		}
	}
}

// recoverCron recomputes next fire times at startup: enabled recurring
// jobs step to the next occurrence after now (missed runs are skipped,
// never replayed), and past-due one-shot jobs are disabled.
func (s *Service) recoverCron(ctx context.Context) error {
	jobs, err := s.deps.Crons.ListCronJobs(ctx)
	if err != nil {
		return err
	}
	st := s.ensureCronState(CronSchedulerOptions{})
	nowMs := st.now().UnixMilli()
	for _, job := range jobs {
		changed := false
		if !job.Enabled {
			if job.State.NextRunAtMs != 0 {
				job.State.NextRunAtMs = 0
				changed = true
			}
		} else {
			next := NextCronAfter(job.Schedule, nowMs)
			if next == 0 && job.Schedule.Kind == domain.CronScheduleAt {
				job.Enabled = false
				job.State.NextRunAtMs = 0
				changed = true
			} else if job.Schedule.Kind != domain.CronScheduleAt && job.State.NextRunAtMs > 0 && job.State.NextRunAtMs <= nowMs {
				// Due while offline: preserve NextRunAtMs so loop's initial
				// fireDueCronJobs fires once on wake. settleCronRun will advance
				// to the next future period, skipping missed storms (UI-CRON-P2).
			} else if next != job.State.NextRunAtMs {
				job.State.NextRunAtMs = next
				changed = true
			}
		}
		if changed {
			job.UpdatedAt = nowMs
			if err := s.deps.Crons.UpdateCronJob(ctx, job); err != nil {
				return fmt.Errorf("storage: recover cron job %s: %w", job.ID, err)
			}
		}
	}
	return nil
}

// TriggerCron implements CronRunner.
func (s *Service) TriggerCron(ctx context.Context, jobID string) (domain.CronJob, error) {
	if s.deps.Crons == nil {
		return domain.CronJob{}, storage.ErrNotFound
	}
	job, err := s.deps.Crons.GetCronJob(ctx, jobID)
	if err != nil {
		return domain.CronJob{}, err
	}
	if job.Payload.Kind != domain.CronPayloadKindAgentTurn {
		return domain.CronJob{}, fmt.Errorf("%w: %q", ErrCronUnsupportedKind, job.Payload.Kind)
	}
	return s.fireCron(ctx, job, CronTriggerManual)
}

// StopCron implements CronRunner.
func (s *Service) StopCron(jobID string) bool {
	if s.cron == nil {
		return false
	}
	s.cron.mu.Lock()
	entry, ok := s.cron.active[jobID]
	s.cron.mu.Unlock()
	if !ok || entry.snapshot.RunID == "" {
		return false
	}
	return s.Cancel(entry.snapshot.RunID)
}

// ActiveCronRun implements CronRunner.
func (s *Service) ActiveCronRun(jobID string) (CronRunSnapshot, bool) {
	if s.cron == nil {
		return CronRunSnapshot{}, false
	}
	s.cron.mu.Lock()
	defer s.cron.mu.Unlock()
	entry, ok := s.cron.active[jobID]
	if !ok {
		return CronRunSnapshot{}, false
	}
	return entry.snapshot, true
}

// fireCron registers the job as running, drives one run in the job's
// dedicated session, and spawns the terminal watcher. The run itself is
// async: RunWithOptions returns once the start sequence is durable.
func (s *Service) fireCron(ctx context.Context, job domain.CronJob, trigger string) (domain.CronJob, error) {
	st := s.ensureCronState(CronSchedulerOptions{})
	st.mu.Lock()
	if _, busy := st.active[job.ID]; busy {
		st.mu.Unlock()
		return domain.CronJob{}, ErrCronRunning
	}
	startedAt := st.now().UnixMilli()
	entry := &cronActiveRun{snapshot: CronRunSnapshot{
		JobID: job.ID, StartedAtMs: startedAt, LastHeartbeatAtMs: startedAt, Trigger: trigger,
	}}
	st.active[job.ID] = entry
	st.mu.Unlock()

	if _, err := s.startCronRun(ctx, job, entry); err != nil {
		st.mu.Lock()
		delete(st.active, job.ID)
		st.mu.Unlock()
		s.recordCronFailure(ctx, job, entry, err)
		return domain.CronJob{}, err
	}

	st.wg.Add(1)
	go s.watchCronRun(st, entry, job)
	return job, nil
}

// startCronRun lazily creates the job's dedicated session and starts one
// agent turn with the payload message. The run context is detached inside
// RunWithOptions, so a returning RPC or a scheduler stop cannot kill an
// already-started run mid-flight.
func (s *Service) startCronRun(ctx context.Context, job domain.CronJob, entry *cronActiveRun) (domain.RunID, error) {
	if job.SessionID == "" {
		session := domain.Session{
			ID:        domain.SessionID(newPrefixedID("sess_")),
			Title:     "Cron: " + job.Name,
			CreatedAt: s.cron.now().UnixMilli(),
		}
		if mode, policy, ok := domain.PermissionPresetSmart.Bundle(); ok {
			session.SandboxMode = string(mode)
			session.ApprovalPolicy = string(policy)
		}
		if err := s.deps.Sessions.CreateSession(ctx, session); err != nil {
			return "", fmt.Errorf("runtime: create cron session: %w", err)
		}
		job.SessionID = session.ID
		job.UpdatedAt = s.cron.now().UnixMilli()
		if err := s.deps.Crons.UpdateCronJob(ctx, job); err != nil {
			return "", fmt.Errorf("runtime: bind cron session: %w", err)
		}
	}
	if job.Payload.Message == "" {
		return "", fmt.Errorf("runtime: cron job %s has an empty message", job.ID)
	}
	runID, err := s.RunWithOptions(context.WithoutCancel(ctx), job.SessionID, job.Payload.Message, RunOptions{})
	if err != nil {
		return "", err
	}
	s.cron.mu.Lock()
	entry.snapshot.RunID = runID
	s.cron.mu.Unlock()
	return runID, nil
}

// watchCronRun polls the durable run row until a terminal status, then
// writes the job state back and reschedules. It polls instead of
// subscribing to the bus because terminal publishes close subscriber
// channels without delivering the event.
func (s *Service) watchCronRun(st *cronState, entry *cronActiveRun, job domain.CronJob) {
	defer st.wg.Done()
	// Survive scheduler stop so the terminal write-back lands while
	// storage is still open during shutdown (E4).
	ctx := context.WithoutCancel(context.Background())

	failures := 0
	for {
		run, err := s.deps.Runs.GetRun(ctx, entry.snapshot.RunID)
		if err == nil && run.Status.Terminal() {
			s.settleCronRun(ctx, st, entry, job, run.Status)
			return
		}
		if err != nil {
			failures++
			if failures > 30 {
				slog.Warn("cron terminal watcher gave up", "job", entry.snapshot.JobID, "run", entry.snapshot.RunID)
				st.mu.Lock()
				delete(st.active, entry.snapshot.JobID)
				st.mu.Unlock()
				return
			}
		} else {
			failures = 0
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(st.terminalPoll):
		}
	}
}

// settleCronRun records the run's outcome on the job row and reschedules:
// recurring jobs step to the next occurrence, one-shot jobs disable (or
// delete themselves on success with delete_after_run).
func (s *Service) settleCronRun(ctx context.Context, st *cronState, entry *cronActiveRun, job domain.CronJob, status domain.RunStatus) {
	lastStatus := "ok"
	lastErr := ""
	switch status {
	case domain.RunFailed:
		lastStatus = "error"
		lastErr = s.cronTerminalError(ctx, entry.snapshot.RunID)
	case domain.RunCancelled:
		lastStatus = "error"
		lastErr = "run cancelled"
	}

	// Re-read the job: it may have been edited or deleted while the run
	// was in flight.
	fresh, err := s.deps.Crons.GetCronJob(ctx, job.ID)
	if err != nil {
		slog.Warn("cron job vanished before settle", "job", job.ID, "err", err)
	} else {
		nowMs := st.now().UnixMilli()
		fresh.State.LastRunAtMs = entry.snapshot.StartedAtMs
		fresh.State.LastStatus = lastStatus
		fresh.State.LastError = lastErr
		fresh.UpdatedAt = nowMs
		switch {
		case fresh.Schedule.Kind == domain.CronScheduleAt && fresh.DeleteAfterRun && lastStatus == "ok":
			err = s.deps.Crons.DeleteCronJob(ctx, fresh.ID)
		case fresh.Schedule.Kind == domain.CronScheduleAt:
			fresh.Enabled = false
			fresh.State.NextRunAtMs = 0
			err = s.deps.Crons.UpdateCronJob(ctx, fresh)
		default:
			fresh.State.NextRunAtMs = 0
			if fresh.Enabled {
				fresh.State.NextRunAtMs = NextCronAfter(fresh.Schedule, nowMs)
			}
			err = s.deps.Crons.UpdateCronJob(ctx, fresh)
		}
		if err != nil {
			slog.Warn("cron settle write failed", "job", job.ID, "err", err)
		}
		if fresh.Payload.Deliver {
			s.deliverCronOutbound(fresh, entry.snapshot.RunID, status, lastErr)
		}
	}

	st.mu.Lock()
	delete(st.active, job.ID)
	st.mu.Unlock()
	s.KickCronScheduler()
}

// recordCronFailure writes a start-time failure onto the job row so the
// UI shows why nothing ran.
func (s *Service) recordCronFailure(ctx context.Context, job domain.CronJob, entry *cronActiveRun, cause error) {
	if s.deps.Crons == nil {
		return
	}
	fresh, err := s.deps.Crons.GetCronJob(ctx, job.ID)
	if err != nil {
		return
	}
	nowMs := s.cron.now().UnixMilli()
	fresh.State.LastRunAtMs = entry.snapshot.StartedAtMs
	fresh.State.LastStatus = "error"
	fresh.State.LastError = cause.Error()
	fresh.UpdatedAt = nowMs
	if fresh.Schedule.Kind != domain.CronScheduleAt && fresh.Enabled {
		fresh.State.NextRunAtMs = NextCronAfter(fresh.Schedule, nowMs)
	} else if fresh.Schedule.Kind == domain.CronScheduleAt {
		fresh.Enabled = false
		fresh.State.NextRunAtMs = 0
	}
	if err := s.deps.Crons.UpdateCronJob(ctx, fresh); err != nil {
		slog.Warn("cron failure write failed", "job", job.ID, "err", err)
	}
	if fresh.Payload.Deliver {
		s.deliverCronOutbound(fresh, entry.snapshot.RunID, domain.RunFailed, cause.Error())
	}
}

// deliverCronOutbound delivers the finished run's summary or failure to the configured
// external channel (CH-0) on a detached, bounded goroutine.
func (s *Service) deliverCronOutbound(job domain.CronJob, runID domain.RunID, status domain.RunStatus, lastErr string) {
	if !job.Payload.Deliver {
		return
	}
	if job.Payload.Channel == "" || job.Payload.To == "" {
		slog.Warn("cron: payload deliver requested but channel or to is empty",
			"job", job.ID, "channel", job.Payload.Channel, "to", job.Payload.To)
		return
	}
	if s.deps.Channels == nil {
		slog.Warn("cron: payload deliver requested but channels deliverer is not wired",
			"job", job.ID, "channel", job.Payload.Channel)
		return
	}

	var summary string
	if status == domain.RunCompleted {
		ctx := context.WithoutCancel(context.Background())
		msgs, err := s.deps.Messages.ListMessages(ctx, job.SessionID)
		if err == nil {
			for i := len(msgs) - 1; i >= 0; i-- {
				m := msgs[i]
				if m.RunID == runID && m.Role == domain.RoleAssistant && m.ToolCallID == "" {
					summary = m.Content
					break
				}
			}
		}
		if summary == "" {
			summary = fmt.Sprintf("[Cron: %s] Run completed", job.Name)
		}
	} else if status == domain.RunFailed {
		if lastErr != "" {
			summary = fmt.Sprintf("[Cron: %s] Run failed: %s", job.Name, lastErr)
		} else {
			summary = fmt.Sprintf("[Cron: %s] Run failed", job.Name)
		}
	} else if status == domain.RunCancelled {
		summary = fmt.Sprintf("[Cron: %s] Run cancelled", job.Name)
	}

	channelName := job.Payload.Channel
	to := job.Payload.To
	jobName := job.Name
	channels := s.deps.Channels
	go func() {
		delivCtx, cancel := context.WithTimeout(context.WithoutCancel(context.Background()), 30*time.Second)
		defer cancel()
		if err := channels.Deliver(delivCtx, channelName, to, summary); err != nil {
			slog.Error("cron: deliver outbound failed",
				"job", jobName, "run", string(runID), "channel", channelName, "to", to, "err", err)
		} else {
			slog.Info("cron: deliver outbound completed",
				"job", jobName, "run", string(runID), "channel", channelName, "to", to)
		}
	}()
}

// cronTerminalError extracts the human-readable failure text from the
// run's terminal journal event (structured payload, never leaky).
func (s *Service) cronTerminalError(ctx context.Context, runID domain.RunID) string {
	it, err := s.deps.Journal.Replay(ctx, runID, 0)
	if err != nil {
		return ""
	}
	defer func() { _ = it.Close() }()
	text := ""
	for it.Next() {
		ev := it.Value().Event
		switch ev.Type {
		case domain.EventRunFailed:
			var p payloadRunFailed
			if err := json.Unmarshal(ev.Payload, &p); err == nil && p.Message != "" {
				text = p.Message
			}
		case domain.EventRunCancelled:
			var p payloadRunCancelled
			if err := json.Unmarshal(ev.Payload, &p); err == nil && p.Reason != "" {
				text = "run cancelled: " + p.Reason
			} else {
				text = "run cancelled"
			}
		}
	}
	return text
}
