package runtime

import (
	"context"
	"sync"
	"testing"
	"time"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/testsupport"
)

func TestPreflightDoesNotPersistOrCallProvider(t *testing.T) {
	svc, backend, _ := newTestService(t, testsupport.NewEchoModel())
	result, err := svc.Preflight(context.Background(), "sess-preflight", "hello vivy", RunOptions{})
	if err != nil {
		t.Fatalf("preflight: %v", err)
	}
	if result.Status != PreflightReady || result.Mode != domain.RunModeNormal {
		t.Fatalf("preflight result = %+v, want ready normal", result)
	}
	if len(result.SelectedTools) != 0 || len(result.Warnings) != 0 || len(result.Blockers) != 0 {
		t.Fatalf("preflight unexpected policy output = %+v", result)
	}
	if msgs, err := backend.ListMessages(context.Background(), "sess-preflight"); err != nil || len(msgs) != 0 {
		t.Fatalf("preflight messages = %+v, err=%v", msgs, err)
	}
	if runs, err := backend.ListActiveRuns(context.Background()); err != nil || len(runs) != 0 {
		t.Fatalf("preflight runs = %+v, err=%v", runs, err)
	}
}

func TestPreflightReportsApprovalWarningAndPlanBlocker(t *testing.T) {
	svc, backend, _ := newApprovalService(t, 5*time.Minute)
	ctx := context.Background()

	warning, err := svc.Preflight(ctx, "sess-preflight", "note that I need milk", RunOptions{})
	if err != nil {
		t.Fatalf("normal preflight: %v", err)
	}
	if warning.Status != PreflightWarning || len(warning.Warnings) == 0 || len(warning.SelectedTools) != 1 || warning.SelectedTools[0] != "write_note" {
		t.Fatalf("normal preflight = %+v", warning)
	}
	blocked, err := svc.Preflight(ctx, "sess-preflight", "note that I need milk", RunOptions{Mode: domain.RunModePlan})
	if err != nil {
		t.Fatalf("plan preflight: %v", err)
	}
	if blocked.Status != PreflightBlocked || len(blocked.Blockers) == 0 {
		t.Fatalf("plan preflight = %+v", blocked)
	}
	if rows, err := backend.ListPendingApprovals(ctx); err != nil || len(rows) != 0 {
		t.Fatalf("preflight approvals = %+v, err=%v", rows, err)
	}
}

type recordingHook struct {
	mu     sync.Mutex
	events []domain.EventType
}

func (h *recordingHook) OnRunEvent(_ context.Context, ev domain.RunEvent) {
	h.mu.Lock()
	h.events = append(h.events, ev.Type)
	h.mu.Unlock()
}

func (h *recordingHook) snapshot() []domain.EventType {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]domain.EventType(nil), h.events...)
}

func TestLifecycleHookSeesPersistedRunEvents(t *testing.T) {
	svc, backend, _ := newTestService(t, testsupport.NewEchoModel())
	hook := &recordingHook{}
	svc.deps.Hooks = []RunHook{hook}
	runID, err := svc.Run(context.Background(), "sess-hook", "hello")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	waitForRunStatus(t, backend, runID, domain.RunCompleted)
	events := hook.snapshot()
	if len(events) == 0 || events[0] != domain.EventRunStarted || events[len(events)-1] != domain.EventRunCompleted {
		t.Fatalf("hook events = %v", events)
	}
}
