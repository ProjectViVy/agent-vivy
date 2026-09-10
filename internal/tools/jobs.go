package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
	"time"

	"agent-vivy/internal/domain"
)

const (
	JobOutputName = "job_output"
	JobKillName   = "job_kill"
	// maxJobOutputBytes bounds the tail of output retained per stream: a
	// chatty job cannot grow the registry without bound, and readers past a
	// gap see how many bytes were evicted.
	maxJobOutputBytes = 64 << 10
	// maxRunningJobs caps concurrently remembered jobs per backend. A
	// foreground bash run only enters the map when its timeout adopts it.
	maxRunningJobs = 16
	// maxRetainedJobs caps remembered terminal jobs before the oldest are
	// evicted on the next registration.
	maxRetainedJobs = 64
)

type JobStatus string

const (
	JobRunning   JobStatus = "running"
	JobCompleted JobStatus = "completed"
	JobFailed    JobStatus = "failed"
	JobKilled    JobStatus = "killed"
)

// JobSpec describes one process launch owned by the registry.
type JobSpec struct {
	Display string // model-readable command line reported back
	Path    string // resolved executable path
	Args    []string
	Dir     string
	Env     []string
	// Run provides an in-process command implementation. When set, Path and
	// Args are ignored while lifecycle/output/cancellation still use the same
	// bounded job registry.
	Run func(context.Context, io.Writer, io.Writer) error
}

// JobReadResult is what job_output returns: new output since the previous
// read plus the job's lifecycle snapshot.
type JobReadResult struct {
	JobID          string    `json:"job_id"`
	Status         JobStatus `json:"status"`
	Command        string    `json:"command,omitempty"`
	ExitCode       int       `json:"exit_code,omitempty"`
	DurationMS     int64     `json:"duration_ms"`
	Stdout         string    `json:"stdout,omitempty"`
	Stderr         string    `json:"stderr,omitempty"`
	StdoutGapBytes int64     `json:"stdout_gap_bytes,omitempty"`
	StderrGapBytes int64     `json:"stderr_gap_bytes,omitempty"`
}

// JobKillResult reports the job state after a kill attempt. Killing an
// already-terminal job succeeds with its current status.
type JobKillResult struct {
	JobID  string    `json:"job_id"`
	Status JobStatus `json:"status"`
}

// JobOperations is the job-management seam the job_output/job_kill tools
// are built over; the process backend implements it.
type JobOperations interface {
	// JobRead returns the job's new output since the last read. Unknown ids
	// report false.
	JobRead(jobID string) (JobReadResult, bool)
	// JobKill terminates a running job. Unknown ids error.
	JobKill(jobID string) (JobKillResult, error)
}

// jobStream is a bounded tail buffer with byte accounting so readers can
// resume by offset and notice evicted gaps.
type jobStream struct {
	limit   int
	mu      sync.Mutex
	buf     []byte
	written int64
	dropped int64
}

func newJobStream(limit int) *jobStream { return &jobStream{limit: limit} }

func (s *jobStream) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.written += int64(len(p))
	s.buf = append(s.buf, p...)
	if over := len(s.buf) - s.limit; over > 0 {
		s.buf = s.buf[over:]
		s.dropped += int64(over)
	}
	return len(p), nil
}

// readNew returns the retained bytes after the caller's cursor, how many
// bytes were skipped because they were evicted, and the new cursor.
func (s *jobStream) readNew(cursor int64) (data string, gap int64, next int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	from := cursor
	if from < s.dropped {
		from = s.dropped
	}
	if from > s.written {
		from = s.written
	}
	gap = from - cursor
	if from >= s.written {
		return "", gap, s.written
	}
	start := int(from - s.dropped)
	return string(s.buf[start:]), gap, s.written
}

type job struct {
	id      string
	display string
	started time.Time

	mu           sync.Mutex
	stdoutCursor int64
	stderrCursor int64
	stdout       *jobStream
	stderr       *jobStream

	status   JobStatus
	exitCode int

	cmd    *exec.Cmd
	cancel context.CancelFunc
	done   chan struct{}
}

func (j *job) read() JobReadResult {
	j.mu.Lock()
	defer j.mu.Unlock()
	stdout, stdoutGap, stdoutNext := j.stdout.readNew(j.stdoutCursor)
	j.stdoutCursor = stdoutNext
	stderr, stderrGap, stderrNext := j.stderr.readNew(j.stderrCursor)
	j.stderrCursor = stderrNext
	return JobReadResult{
		JobID: j.id, Status: j.status, Command: j.display, ExitCode: j.exitCode,
		DurationMS: time.Since(j.started).Milliseconds(), Stdout: stdout, Stderr: stderr,
		StdoutGapBytes: stdoutGap, StderrGapBytes: stderrGap,
	}
}

// readAll is the foreground path: one full snapshot with cursor zero.
// Callers must hold j.mu.
func (j *job) readAll() (stdout, stderr string) {
	stdout, _, _ = j.stdout.readNew(0)
	stderr, _, _ = j.stderr.readNew(0)
	return stdout, stderr
}

// finish records the terminal state; a status already set by Kill wins.
func (j *job) finish(waitErr error, ctxErr error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.status != JobRunning {
		return
	}
	switch {
	case ctxErr != nil:
		j.status, j.exitCode = JobKilled, -1
	case waitErr != nil:
		j.status = JobFailed
		if exitErr, ok := waitErr.(*exec.ExitError); ok {
			j.exitCode = exitErr.ExitCode()
		} else {
			j.exitCode = -1
		}
	default:
		j.status, j.exitCode = JobCompleted, 0
	}
}

// JobRegistry owns background-capable processes. Jobs are bound to the
// caller's context — the run context — so a finishing or cancelled run
// reaps its own jobs; there are no cross-run orphans.
type JobRegistry struct {
	mu   sync.Mutex
	next int
	jobs map[string]*job
}

func NewJobRegistry() *JobRegistry { return &JobRegistry{jobs: make(map[string]*job)} }

// Launch starts spec as a background job bound to ctx and returns its id.
func (r *JobRegistry) Launch(ctx context.Context, spec JobSpec) (string, error) {
	j, err := r.spawn(ctx, spec)
	if err != nil {
		return "", err
	}
	return r.register(j)
}

// RunUntil runs spec synchronously for up to timeout. A process that exits
// within the budget is never registered; one that exceeds it is adopted as
// a background job and its id is returned with the partial output.
func (r *JobRegistry) RunUntil(ctx context.Context, spec JobSpec, timeout time.Duration) (string, CommandResult, error) {
	j, err := r.spawn(ctx, spec)
	if err != nil {
		return "", CommandResult{}, err
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-j.done:
		return "", j.collectFinal(), nil
	case <-ctx.Done():
		return "", j.collectFinal(), ctx.Err()
	case <-timer.C:
		id, err := r.register(j)
		if err != nil {
			r.discard(j)
			return "", CommandResult{}, err
		}
		j.mu.Lock()
		stdout, stderr := j.readAll()
		elapsed := time.Since(j.started).Milliseconds()
		j.mu.Unlock()
		return id, CommandResult{
			ExitCode: -1, Stdout: stdout, Stderr: stderr,
			TimedOut: true, DurationMS: elapsed,
			JobID: id, Background: true, JobStatus: string(JobRunning),
		}, nil
	}
}

// RunForeground runs spec within timeout and never registers or adopts the
// process as a background job. It is used by caller-owned foreground surfaces
// whose lifecycle must end with the request (for example direct !shell).
func (r *JobRegistry) RunForeground(ctx context.Context, spec JobSpec, timeout time.Duration) (CommandResult, error) {
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	j, err := r.spawn(runCtx, spec)
	if err != nil {
		return CommandResult{}, err
	}
	select {
	case <-j.done:
		return j.collectFinal(), nil
	case <-runCtx.Done():
		// spawn binds both native and in-process commands to runCtx. Wait for
		// finalization so no process or output goroutine survives the request.
		<-j.done
		result := j.collectFinal()
		result.TimedOut = errors.Is(runCtx.Err(), context.DeadlineExceeded)
		return result, runCtx.Err()
	}
}

// Read returns the job's incremental output snapshot.
func (r *JobRegistry) Read(id string) (JobReadResult, bool) {
	r.mu.Lock()
	j, ok := r.jobs[id]
	r.mu.Unlock()
	if !ok {
		return JobReadResult{}, false
	}
	return j.read(), true
}

// Kill terminates a running job; an already-terminal job reports its state.
func (r *JobRegistry) Kill(id string) (JobKillResult, error) {
	r.mu.Lock()
	j, ok := r.jobs[id]
	r.mu.Unlock()
	if !ok {
		return JobKillResult{}, fmt.Errorf("job_kill: unknown job %s", id)
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.status != JobRunning {
		return JobKillResult{JobID: j.id, Status: j.status}, nil
	}
	if j.cmd != nil && j.cmd.Process != nil {
		_ = j.cmd.Process.Kill()
	} else if j.cancel != nil {
		j.cancel()
	}
	j.status, j.exitCode = JobKilled, -1
	return JobKillResult{JobID: j.id, Status: JobKilled}, nil
}

// spawn starts the process with readers and a finalizer; the job is not
// yet registered or counted against the limit.
func (r *JobRegistry) spawn(ctx context.Context, spec JobSpec) (*job, error) {
	if spec.Run != nil {
		jobCtx, cancel := context.WithCancel(ctx)
		j := &job{
			display: spec.Display, started: time.Now(), status: JobRunning, exitCode: -1,
			stdout: newJobStream(maxJobOutputBytes), stderr: newJobStream(maxJobOutputBytes),
			cancel: cancel, done: make(chan struct{}),
		}
		go func() {
			runErr := spec.Run(jobCtx, j.stdout, j.stderr)
			j.finish(runErr, jobCtx.Err())
			close(j.done)
		}()
		return j, nil
	}
	cmd := exec.CommandContext(ctx, spec.Path, spec.Args...)
	cmd.Dir = spec.Dir
	cmd.Env = spec.Env
	// A grandchild holding the pipes must not block finalization forever
	// after the process itself is gone.
	cmd.WaitDelay = 2 * time.Second
	j := &job{
		display: spec.Display, started: time.Now(), status: JobRunning, exitCode: -1,
		stdout: newJobStream(maxJobOutputBytes), stderr: newJobStream(maxJobOutputBytes),
		cmd: cmd, done: make(chan struct{}),
	}
	cmd.Stdout = j.stdout
	cmd.Stderr = j.stderr
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("command: start: %w", err)
	}
	go func() {
		waitErr := cmd.Wait()
		j.finish(waitErr, ctx.Err())
		close(j.done)
	}()
	return j, nil
}

// register assigns the id and remembers the job, evicting the oldest
// terminal jobs; a full map of running jobs fails closed.
func (r *JobRegistry) register(j *job) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for len(r.jobs) >= maxRunningJobs && r.evictOldestTerminalLocked() {
	}
	for len(r.jobs) >= maxRetainedJobs && r.evictOldestTerminalLocked() {
	}
	if len(r.jobs) >= maxRunningJobs {
		return "", errors.New("command: background job limit reached")
	}
	r.next++
	j.id = fmt.Sprintf("job_%06d", r.next)
	r.jobs[j.id] = j
	return j.id, nil
}

func (r *JobRegistry) evictOldestTerminalLocked() bool {
	for id, j := range r.jobs {
		select {
		case <-j.done:
			delete(r.jobs, id)
			return true
		default:
		}
	}
	return false
}

func (r *JobRegistry) discard(j *job) {
	if j.cmd != nil && j.cmd.Process != nil {
		_ = j.cmd.Process.Kill()
	} else if j.cancel != nil {
		j.cancel()
	}
	<-j.done
}

func (j *job) collectFinal() CommandResult {
	j.mu.Lock()
	defer j.mu.Unlock()
	stdout, stderr := j.readAll()
	return CommandResult{
		ExitCode: j.exitCode, Stdout: stdout, Stderr: stderr,
		DurationMS: time.Since(j.started).Milliseconds(),
	}
}

type jobsTool struct {
	name string
	ops  JobOperations
}

func NewJobOutput(ops JobOperations) Tool { return &jobsTool{name: JobOutputName, ops: ops} }
func NewJobKill(ops JobOperations) Tool   { return &jobsTool{name: JobKillName, ops: ops} }

func (t *jobsTool) Spec() domain.ToolSpec {
	if t.name == JobOutputName {
		return domain.ToolSpec{
			Name: JobOutputName,
			Description: "Reads new output from a background job since the last read and reports its lifecycle status " +
				"(running/completed/failed/killed). Poll it after starting a command with bash run_in_background.",
			Readonly: true,
			Keywords: []string{"job", "background", "output", "process"},
			Params:   map[string]domain.ToolParam{"job_id": {Desc: "Job id returned by bash.", Required: true}},
		}
	}
	return domain.ToolSpec{
		Name:        JobKillName,
		Description: "Terminates a running background job by id. Killing an already-finished job reports its state without error.",
		Readonly:    false,
		Keywords:    []string{"job", "background", "kill", "process"},
		Params:      map[string]domain.ToolParam{"job_id": {Desc: "Job id returned by bash.", Required: true}},
	}
}

func (t *jobsTool) InvokableRun(ctx context.Context, args json.RawMessage) (string, error) {
	var input struct {
		JobID string `json:"job_id"`
	}
	if err := json.Unmarshal(args, &input); err != nil {
		return "", fmt.Errorf("%s: invalid arguments: %w", t.name, err)
	}
	id := strings.TrimSpace(input.JobID)
	if id == "" {
		return "", &ArgError{Field: "job_id", Reason: "is required"}
	}
	if t.ops == nil {
		return "", fmt.Errorf("tools: %s backend not wired", t.name)
	}
	if t.name == JobOutputName {
		res, ok := t.ops.JobRead(id)
		if !ok {
			return "", fmt.Errorf("%s: unknown job %s", t.name, id)
		}
		return marshalToolResult(res)
	}
	res, err := t.ops.JobKill(id)
	if err != nil {
		return "", err
	}
	return marshalToolResult(res)
}
