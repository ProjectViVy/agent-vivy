package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/ProjectViVy/inofy/definitions"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
	"agent-vivy/internal/testsupport"
)

// workflowLaunchFixture publishes one runnable revision so launch admission can
// be exercised independently of the draft lifecycle.
func workflowLaunchFixture(t *testing.T, svc *Service, author domain.SessionID) definitions.Revision {
	t.Helper()
	ctx := context.Background()
	draft, err := svc.INOFYSaveDraft(ctx, author, "wf-launch", definitions.ETagAbsent, inofyProductArtifact(inofyProductDefinition))
	if err != nil {
		t.Fatalf("save draft: %v", err)
	}
	rev, err := svc.INOFYPublishDraft(ctx, author, "wf-launch", draft.ETag)
	if err != nil {
		t.Fatalf("publish draft: %v", err)
	}
	return rev
}

// TestWorkflowProductLaunchParentAdmission pins the backend half of the W2
// contract the editor mirrors: only an active parent Run in the author's
// session is admitted, a terminal parent is a conflict, and a parent that does
// not exist in the session is not found. Rejected launches admit no child.
func TestWorkflowProductLaunchParentAdmission(t *testing.T) {
	ctx := context.Background()
	svc, backend := inofyExecService(t, testsupport.NewEchoModel())
	author := domain.SessionID("sess-prod-launch")
	parentRunID := prepareProductSession(t, svc, backend, author)
	rev := workflowLaunchFixture(t, svc, author)

	start := func(parent domain.RunID, operation string) (WorkflowStartResult, error) {
		return svc.INOFYStartRun(ctx, author, INOFYStartRunParams{
			ParentRunID: parent, OperationKey: operation, WorkflowID: "wf-launch", Revision: rev.Revision,
		})
	}

	// Active parent: admitted, and the child is owned by it.
	active, err := start(parentRunID, "launch-active")
	if err != nil || !active.Created {
		t.Fatalf("active parent rejected: %v %+v", err, active)
	}
	waitForRunStatus(t, backend, active.Run.ID, domain.RunCompleted)
	children, err := backend.ListChildRuns(ctx, parentRunID)
	if err != nil || len(children) != 1 || children[0].ID != active.Run.ID {
		t.Fatalf("children = %+v err=%v", children, err)
	}

	// Unknown parent: not found, nothing admitted.
	if _, err := start(domain.RunID("run-missing"), "launch-missing"); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("missing parent err = %v", err)
	}
	if children, err := backend.ListChildRuns(ctx, domain.RunID("run-missing")); err != nil || len(children) != 0 {
		t.Fatalf("missing parent admitted children: %+v err=%v", children, err)
	}

	// Terminal parent: conflict, nothing admitted for it.
	if err := backend.SetRunStatus(ctx, parentRunID, domain.RunCompleted); err != nil {
		t.Fatalf("complete parent: %v", err)
	}
	if _, err := start(parentRunID, "launch-terminal"); !errors.Is(err, storage.ErrChildAdmissionConflict) {
		t.Fatalf("terminal parent err = %v", err)
	}
	children, err = backend.ListChildRuns(ctx, parentRunID)
	if err != nil || len(children) != 1 {
		t.Fatalf("terminal parent admitted another child: %+v err=%v", children, err)
	}
}

// TestWorkflowProductLaunchRunRequestShape keeps the admitted request's
// identity fields bound to the fixture so a future signature change cannot
// silently drop the operation key or the pinned revision.
func TestWorkflowProductLaunchRunRequestShape(t *testing.T) {
	ctx := context.Background()
	svc, backend := inofyExecService(t, testsupport.NewEchoModel())
	author := domain.SessionID("sess-prod-launch-shape")
	parentRunID := prepareProductSession(t, svc, backend, author)
	rev := workflowLaunchFixture(t, svc, author)

	started, err := svc.INOFYStartRun(ctx, author, INOFYStartRunParams{
		ParentRunID: parentRunID, OperationKey: "launch-shape", WorkflowID: "wf-launch", Revision: rev.Revision,
		Input: json.RawMessage(`{}`),
	})
	if err != nil || !started.Created {
		t.Fatalf("start: %v %+v", err, started)
	}
	waitForRunStatus(t, backend, started.Run.ID, domain.RunCompleted)
	view, err := svc.INOFYGetRun(ctx, author, started.Run.ID)
	if err != nil {
		t.Fatalf("get run: %v", err)
	}
	if view.Revision.ParentRunID != parentRunID || view.Revision.ParentSessionID != author || view.Revision.OperationKey != "launch-shape" {
		t.Fatalf("run binding = parent %q session %q op %q", view.Revision.ParentRunID, view.Revision.ParentSessionID, view.Revision.OperationKey)
	}
	if view.Details.Run.ParentID != parentRunID || view.Details.Run.SessionID != author {
		t.Fatalf("run lineage = parent %q session %q", view.Details.Run.ParentID, view.Details.Run.SessionID)
	}
}
