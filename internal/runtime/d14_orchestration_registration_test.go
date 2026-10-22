package runtime

import (
	"context"
	"errors"
	"testing"
	"time"

	"agent-vivy/internal/domain"
)

func d14OrchestrationSuspensionGap(t *testing.T) (nativeOrchestrationApprovalFixture, *d14SuspensionGap, <-chan error) {
	t.Helper()
	fixture := newNativeOrchestrationApprovalFixture(t)
	svc, runID := fixture.svc, fixture.runID
	ctx, cancel := context.WithCancel(fixture.workflowCtx)
	gap := &d14SuspensionGap{Context: ctx, reached: make(chan struct{}), released: make(chan struct{})}
	svc.deps.Sink = &d14SuspensionSink{inner: svc.deps.Sink.(*testSink), gap: gap}
	svc.mu.Lock()
	svc.active[runID], svc.runSessions[runID] = cancel, fixture.sessionID
	svc.mu.Unlock()
	t.Cleanup(func() {
		gap.release()
		svc.CancelAll()
		ctx, stop := context.WithTimeout(context.Background(), 10*time.Second)
		defer stop()
		if !svc.WaitIdle(ctx) {
			t.Error("native workflow did not drain")
		}
		cancel()
	})
	invoked := make(chan error, 1)
	svc.wg.Add(1)
	go func() {
		defer svc.wg.Done()
		_, err := svc.executeNativeOrchestrationWorkflow(gap, fixture.input)
		invoked <- err
	}()
	select {
	case <-gap.reached:
	case <-time.After(5 * time.Second):
		t.Fatal("native workflow did not reach the post-publication registration gap")
	}
	if !svc.projectionMu.TryLock() {
		t.Fatal("workflow fixture blocked publication rather than registration")
	}
	svc.projectionMu.Unlock()
	return fixture, gap, invoked
}

func TestD14OrchestrationResponseWaitsForSuspensionRegistration(t *testing.T) {
	for _, decision := range []string{domain.ApprovalApproved, domain.ApprovalDenied} {
		t.Run(decision, func(t *testing.T) {
			fixture, gap, invoked := d14OrchestrationSuspensionGap(t)
			svc, backend, runID := fixture.svc, fixture.backend, fixture.runID
			approval := waitForPendingApproval(t, backend, runID)
			if !isNativeOrchestrationResumeTarget(approval.ResumeTarget) || !svc.ApprovalRequiredDurable(fixture.ctx, runID, approval.ID) {
				t.Fatal("fixture did not publish its actual native workflow approval")
			}
			svc.mu.Lock()
			_, pending := svc.pending[runID]
			svc.mu.Unlock()
			if pending {
				t.Fatal("workflow suspension registered before the boundary")
			}
			if _, found, err := svc.engine.cfg.Checkpoints.Get(fixture.workflowCtx, checkpointIDFor(runID)); err != nil || !found {
				t.Fatalf("actual Eino workflow checkpoint unavailable: %v, %v", found, err)
			}
			operationB := workflowNodeOperationID("b")
			before := countWorkflowOperationEvents(t, backend, runID, operationB)
			if before != 3 {
				t.Fatalf("workflow sibling did not complete before suspension: transitions=%d", before)
			}
			responded := make(chan error, 1)
			go func() { responded <- svc.DecideApproval(context.Background(), approval.ID, decision) }()
			select {
			case err := <-responded:
				t.Fatalf("workflow response settled before native registration: %v", err)
			case <-time.After(100 * time.Millisecond):
			}
			gap.release()
			select {
			case err := <-invoked:
				if !errors.Is(err, ErrNativeOrchestrationApprovalPending) {
					t.Fatalf("native suspend owner = %v", err)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("native workflow suspend owner did not finish")
			}
			select {
			case err := <-responded:
				if err != nil {
					t.Fatal(err)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("workflow response did not settle")
			}
			waitForNativeOrchestrationRunStatus(t, backend, runID, domain.RunCompleted)
			if err := svc.DecideApproval(context.Background(), approval.ID, decision); !errors.Is(err, ErrApprovalAlreadyDecided) {
				t.Fatalf("duplicate workflow response = %v", err)
			}
			operation, err := backend.GetToolOperation(fixture.ctx, runID, workflowNodeOperationID("a"))
			if err != nil || operation.State != domain.ToolOperationCompleted {
				t.Fatalf("workflow node did not settle: %+v err=%v", operation, err)
			}
			wantOutcome := domain.ToolOutcomeSuccess
			if decision == domain.ApprovalDenied {
				wantOutcome = domain.ToolOutcomeRefused
			}
			if operation.OutcomeStatus != wantOutcome {
				t.Fatalf("workflow outcome = %s, want %s", operation.OutcomeStatus, wantOutcome)
			}
			if after := countWorkflowOperationEvents(t, backend, runID, operationB); after != before {
				t.Fatalf("completed sibling ran again: %d -> %d", before, after)
			}
			if countWorkflowOperationEvents(t, backend, runID, workflowNodeOperationID("a")) != 3 || countTerminal(replayEvents(t, backend, runID)) != 1 {
				t.Fatal("workflow response duplicated node execution or terminal")
			}
		})
	}
}

func TestD14OrchestrationCancelledApprovalHandoffIsNotPending(t *testing.T) {
	fixture, gap, invoked := d14OrchestrationSuspensionGap(t)
	svc, backend, runID := fixture.svc, fixture.backend, fixture.runID
	approval := waitForPendingApproval(t, backend, runID)
	cancelled := make(chan bool, 1)
	go func() { cancelled <- svc.Cancel(runID) }()
	select {
	case <-gap.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("Cancel did not reach the native workflow")
	}
	gap.release()
	select {
	case ok := <-cancelled:
		if !ok {
			t.Fatal("active workflow cancellation was rejected")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("workflow cancellation did not finish")
	}
	select {
	case err := <-invoked:
		if err == nil || errors.Is(err, ErrNativeOrchestrationApprovalPending) {
			t.Fatalf("cancelled approval reported successful handoff: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("cancelled workflow owner did not return")
	}
	waitForNativeOrchestrationRunStatus(t, backend, runID, domain.RunCancelled)
	stored, err := backend.GetApproval(fixture.ctx, approval.ID)
	if err != nil || stored.Decision != domain.ApprovalCancelled {
		t.Fatalf("cancelled workflow approval = %+v, err=%v", stored, err)
	}
	operation, err := backend.GetToolOperation(fixture.ctx, runID, workflowNodeOperationID("a"))
	if err != nil || operation.State != domain.ToolOperationAdmitted {
		t.Fatalf("cancelled workflow claimed its node: %+v err=%v", operation, err)
	}
	if countTerminal(replayEvents(t, backend, runID)) != 1 {
		t.Fatal("cancelled workflow did not close exactly once")
	}
}

func TestD14OrchestrationFailedApprovalHandoffIsNotPending(t *testing.T) {
	fixture := newNativeOrchestrationApprovalFixture(t)
	fixture.svc.deps.Approvals = nil
	_, err := fixture.svc.executeNativeOrchestrationWorkflow(fixture.workflowCtx, fixture.input)
	if err == nil || errors.Is(err, ErrNativeOrchestrationApprovalPending) {
		t.Fatalf("failed approval reported successful handoff: %v", err)
	}
	waitForNativeOrchestrationRunStatus(t, fixture.backend, fixture.runID, domain.RunFailed)
	fixture.svc.mu.Lock()
	_, pending := fixture.svc.pending[fixture.runID]
	fixture.svc.mu.Unlock()
	if pending || countTerminal(replayEvents(t, fixture.backend, fixture.runID)) != 1 {
		t.Fatal("failed approval left a pending workflow or duplicate terminal")
	}
}
