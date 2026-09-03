package tools

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"runtime"
	"strings"
	"testing"
	"time"
)

func requireBash(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		return
	}
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash is not available on this host")
	}
}

func testJobSpec(script string, delay time.Duration) JobSpec {
	if runtime.GOOS != "windows" {
		return JobSpec{Display: "bash", Path: "bash", Args: []string{"-c", script}}
	}
	return JobSpec{Display: "embedded shell", Run: func(ctx context.Context, stdout, _ io.Writer) error {
		if strings.Contains(script, "vivy_job_marker") {
			_, _ = fmt.Fprintln(stdout, "vivy_job_marker")
		}
		if delay <= 0 {
			return nil
		}
		timer := time.NewTimer(delay)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
			return nil
		}
	}}
}

func waitForJobOutput(t *testing.T, r *JobRegistry, id string) JobReadResult {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		res, ok := r.Read(id)
		if !ok {
			t.Fatalf("job %s vanished from the registry", id)
		}
		if strings.Contains(res.Stdout, "vivy_job_marker") || res.Status != JobRunning {
			return res
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("job %s never produced output", id)
	return JobReadResult{}
}

func TestJobStreamTailRetention(t *testing.T) {
	s := newJobStream(16)
	large := strings.Repeat("a", 40) + "TAIL"
	if _, err := s.Write([]byte(large)); err != nil {
		t.Fatalf("write: %v", err)
	}
	data, gap, next := s.readNew(0)
	if data != strings.Repeat("a", 12)+"TAIL" {
		t.Fatalf("tail = %q, want last 16 bytes", data)
	}
	if gap != 28 {
		t.Fatalf("gap = %d, want 28 evicted bytes", gap)
	}
	if next != 44 {
		t.Fatalf("cursor = %d, want 44", next)
	}
	// A stale cursor before the eviction window skips to the gap.
	data2, gap2, _ := s.readNew(4)
	if data2 != data || gap2 != 24 {
		t.Fatalf("stale cursor read = %q/%d, want tail with gap 24", data2, gap2)
	}
}

func TestJobRegistryRunUntilCompletes(t *testing.T) {
	requireBash(t)
	r := NewJobRegistry()
	ctx := context.Background()
	id, res, err := r.RunUntil(ctx, testJobSpec("echo vivy_job_marker", 0), 5*time.Second)
	if err != nil {
		t.Fatalf("run until: %v", err)
	}
	if id != "" {
		t.Fatalf("completed run was registered as job %q, want no id", id)
	}
	if !strings.Contains(res.Stdout, "vivy_job_marker") || res.ExitCode != 0 || res.TimedOut {
		t.Fatalf("result = %+v, want marker with exit 0", res)
	}
}

func TestJobRegistryRunUntilAdoptsOnTimeout(t *testing.T) {
	requireBash(t)
	r := NewJobRegistry()
	ctx := context.Background()
	id, res, err := r.RunUntil(ctx, testJobSpec("echo vivy_job_marker; sleep 30", 30*time.Second), 300*time.Millisecond)
	if err != nil {
		t.Fatalf("run until: %v", err)
	}
	if id == "" || res.JobID == "" || !res.TimedOut {
		t.Fatalf("timed-out run = id %q result %+v, want adopted job", id, res)
	}
	got := waitForJobOutput(t, r, id)
	if got.Status != JobRunning || !strings.Contains(got.Stdout, "vivy_job_marker") {
		t.Fatalf("adopted job = %+v, want running with marker", got)
	}
	// Second read returns only fresh output.
	again, ok := r.Read(id)
	if !ok || again.Stdout != "" {
		t.Fatalf("second read = %+v ok=%v, want empty increment", again, ok)
	}
	if _, err := r.Kill(id); err != nil {
		t.Fatalf("kill: %v", err)
	}
	dead := waitForJobStatus(t, r, id, JobKilled)
	if dead.ExitCode != -1 {
		t.Fatalf("killed exit code = %d, want -1", dead.ExitCode)
	}
}

func TestJobRegistryLaunchReadKill(t *testing.T) {
	requireBash(t)
	r := NewJobRegistry()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	id, err := r.Launch(ctx, testJobSpec("echo vivy_job_marker; sleep 30", 30*time.Second))
	if err != nil {
		t.Fatalf("launch: %v", err)
	}
	got := waitForJobOutput(t, r, id)
	if got.Status != JobRunning || !strings.Contains(got.Stdout, "vivy_job_marker") {
		t.Fatalf("job read = %+v, want running with marker", got)
	}
	res, err := r.Kill(id)
	if err != nil || res.Status != JobKilled {
		t.Fatalf("kill = %+v/%v, want killed", res, err)
	}
	// Killing a terminal job reports its state without error.
	again, err := r.Kill(id)
	if err != nil || again.Status != JobKilled {
		t.Fatalf("second kill = %+v/%v, want killed without error", again, err)
	}
}

func TestJobRegistryRunContextCancelsJobs(t *testing.T) {
	requireBash(t)
	r := NewJobRegistry()
	ctx, cancel := context.WithCancel(context.Background())
	id, err := r.Launch(ctx, testJobSpec("sleep 30", 30*time.Second))
	if err != nil {
		t.Fatalf("launch: %v", err)
	}
	cancel()
	waitForJobStatus(t, r, id, JobKilled)
}

func TestJobRegistryForegroundRunContextError(t *testing.T) {
	requireBash(t)
	r := NewJobRegistry()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, _, err := r.RunUntil(ctx, testJobSpec("sleep 30", 30*time.Second), 5*time.Second)
	if err == nil {
		t.Fatal("cancelled context returned no error for a foreground run")
	}
}

func TestJobRegistryKillUnknownJob(t *testing.T) {
	r := NewJobRegistry()
	if _, ok := r.Read("job_nope"); ok {
		t.Fatal("unknown job read reported ok")
	}
	if _, err := r.Kill("job_nope"); err == nil || !strings.Contains(err.Error(), "unknown job") {
		t.Fatalf("unknown kill error = %v, want unknown job", err)
	}
}

func TestJobRegistryLaunchLimit(t *testing.T) {
	requireBash(t)
	r := NewJobRegistry()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ids := make([]string, 0, maxRunningJobs)
	for i := 0; i < maxRunningJobs; i++ {
		id, err := r.Launch(ctx, testJobSpec("sleep 2", 2*time.Second))
		if err != nil {
			t.Fatalf("launch %d: %v", i, err)
		}
		ids = append(ids, id)
	}
	if _, err := r.Launch(ctx, testJobSpec("sleep 2", 2*time.Second)); err == nil || !strings.Contains(err.Error(), "limit") {
		t.Fatalf("over-limit launch error = %v, want limit rejection", err)
	}
	for _, id := range ids {
		_, _ = r.Kill(id)
	}
}

func waitForJobStatus(t *testing.T, r *JobRegistry, id string, want JobStatus) JobReadResult {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		res, ok := r.Read(id)
		if ok && res.Status == want {
			return res
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("job %s never reached status %s", id, want)
	return JobReadResult{}
}
