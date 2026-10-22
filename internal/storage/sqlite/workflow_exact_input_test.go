package sqlite

import (
	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
	"agent-vivy/internal/storage/conformance"
	"context"
	"errors"
	"testing"
)

func TestWorkflowCommitRejectsChangedActualEvent(t *testing.T) {
	slot := workflowStepSlot(t)
	wf, store := conformance.WorkflowStepFixture(t, slot, "actual-input")
	c := conformance.NewStepCommit(wf, "same", 1, "", storage.WorkflowStepAdmitted, "actual-input")
	c.Events = []storage.WorkflowStepEvent{conformance.StepAdmitEvent("actual-input")}
	if _, err := store.CommitWorkflowStep(context.Background(), c); err != nil {
		t.Fatal(err)
	}
	c.Events = append(c.Events, conformance.StepEvent(domain.EventWorkflowStarted, `{"node_count":2}`))
	if _, err := store.CommitWorkflowStep(context.Background(), c); !errors.Is(err, storage.ErrWorkflowStepIdempotency) {
		t.Fatalf("changed actual body must conflict, got %v", err)
	}
}
