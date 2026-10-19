package runtime

import (
	"context"

	inofy "github.com/ProjectViVy/inofy"

	"agent-vivy/internal/domain"
)

// reportNodeExecutor is the restricted effect boundary for the sealed
// report/v1 program. Before any effect it re-verifies the call's Run
// identity against the persisted revision — live workflow run, schema-2,
// matching program digest and host binding, trusted report purpose, known
// node type. R0 binds no effects: a verified call fails closed with
// capability_unavailable rather than fabricating report output.
type reportNodeExecutor struct {
	svc *Service
}

func newReportNodeExecutor(svc *Service) *reportNodeExecutor {
	return &reportNodeExecutor{svc: svc}
}

var reportSealedNodeTypes = map[string]struct{}{
	reportNodeCollect:        {},
	reportNodeNarrate:        {},
	reportNodeValidateRender: {},
	reportNodePersist:        {},
}

func (e *reportNodeExecutor) Execute(ctx context.Context, call inofy.NodeCall) (inofy.NodeReply, error) {
	if _, sealed := reportSealedNodeTypes[call.TypeID]; !sealed || call.ImplementationID != reportImplementationID {
		return inofy.NodeReply{}, &inofy.Error{Code: inofy.ErrAuthorityDenied, Path: call.Path,
			Message: "untrusted node implementation " + call.TypeID + "/" + call.ImplementationID}
	}
	if e == nil || e.svc == nil || e.svc.deps.Runs == nil || e.svc.deps.WorkflowRevisions == nil {
		return inofy.NodeReply{}, &inofy.Error{Code: inofy.ErrStorageFailed, Path: call.Path,
			Message: "report node executor is not wired"}
	}
	if call.Ref.RunID == "" || call.OperationKey == "" {
		return inofy.NodeReply{}, &inofy.Error{Code: inofy.ErrInvalidDefinition, Path: call.Path,
			Message: "node call identity is incomplete"}
	}
	runID := domain.RunID(call.Ref.RunID)
	run, err := e.svc.deps.Runs.GetRun(ctx, runID)
	if err != nil {
		return inofy.NodeReply{}, mapINOFYStoreError(err)
	}
	if run.Kind != domain.RunKindWorkflow || run.Purpose != domain.RunPurposeReport || run.Status.Terminal() {
		return inofy.NodeReply{}, &inofy.Error{Code: inofy.ErrAuthorityDenied, Path: call.Path,
			Message: "call is not bound to a live report workflow run"}
	}
	revision, err := e.svc.deps.WorkflowRevisions.GetWorkflowRevision(ctx, runID)
	if err != nil {
		return inofy.NodeReply{}, mapINOFYStoreError(err)
	}
	if revision.SchemaVersion != 2 || revision.ProgramDigest != call.Ref.ProgramDigest ||
		revision.HostBindingID != call.Ref.HostBindingID || revision.RootPurpose != TrustedStrategyReport {
		return inofy.NodeReply{}, &inofy.Error{Code: inofy.ErrAuthorityDenied, Path: call.Path,
			Message: "report run binding does not match its admitted revision"}
	}
	// Verified report call → dispatch to the bounded sealed effect. Only the
	// four report node types reach here; every other type already failed
	// closed above.
	switch call.TypeID {
	case reportNodeCollect:
		return e.effectCollect(ctx, call)
	case reportNodeNarrate:
		return e.effectNarrate(ctx, call)
	case reportNodeValidateRender:
		return e.effectValidateRender(ctx, call)
	case reportNodePersist:
		return e.effectPersist(ctx, call)
	}
	return inofy.NodeReply{}, &inofy.Error{Code: inofy.ErrAuthorityDenied, Path: call.Path,
		Message: "report effect capability is not configured"}
}
