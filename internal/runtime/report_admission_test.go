package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	inofy "github.com/ProjectViVy/inofy"

	"agent-vivy/internal/domain"
	nb "agent-vivy/internal/notebookcontract"
	rc "agent-vivy/internal/reportcontract"
	"agent-vivy/internal/storage"
	"agent-vivy/internal/testsupport"
)

func reportAdmissionContext() rc.AdmissionContext {
	return rc.AdmissionContext{
		Scope:  nb.HomeScopeID,
		Actor:  nb.Actor{Kind: nb.ActorHuman},
		Origin: nb.OriginHuman,
	}
}

func reportRequest(opKey string) rc.ReportRequest {
	return rc.ReportRequest{Period: rc.PeriodDaily, Window: rc.WindowCompleted, OperationKey: opKey}
}

// TestReportServiceAdmission covers the trusted root lane end to end:
// control Session creation, committed Run pins, replay, idempotency
// conflict, and the busy-target decline.
func TestReportServiceAdmission(t *testing.T) {
	ctx := context.Background()
	svc, backend := inofyExecService(t, testsupport.NewEchoModel())

	admission, err := svc.StartReport(ctx, reportAdmissionContext(), reportRequest("op-a"))
	if err != nil {
		t.Fatal(err)
	}
	if !admission.Created || admission.Rejoined || admission.Busy || admission.RunID == "" {
		t.Fatalf("first admission must create: %+v", admission)
	}
	run, err := backend.GetRun(ctx, domain.RunID(admission.RunID))
	if err != nil {
		t.Fatal(err)
	}
	if run.Kind != domain.RunKindWorkflow || run.Purpose != domain.RunPurposeReport ||
		run.ParentID != "" || run.RootID != run.ID || run.Depth != 0 {
		t.Fatalf("report run pins wrong: %+v", run)
	}
	control, err := backend.GetSession(ctx, run.SessionID)
	if err != nil {
		t.Fatal(err)
	}
	if control.Purpose != domain.SessionPurposeReportControl || !control.Hidden() {
		t.Fatalf("control session must be hidden report-control: %+v", control)
	}
	if sessions, err := backend.ListSessions(ctx); err != nil || len(sessions) != 0 {
		t.Fatalf("control session must not list in chat lane: %v %d", err, len(sessions))
	}
	revision, err := backend.GetWorkflowRevision(ctx, run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if revision.RootPurpose != TrustedStrategyReport || revision.ParentRunID != "" ||
		revision.AdmissionNamespace != string(control.ID) || revision.RequestDigest == "" ||
		revision.TargetKey == "" || revision.SchemaVersion != 2 {
		t.Fatalf("root revision pins wrong: %+v", revision)
	}

	// Same key + same request rejoins the committed run.
	replayed, err := svc.StartReport(ctx, reportAdmissionContext(), reportRequest("op-a"))
	if err != nil {
		t.Fatal(err)
	}
	if !replayed.Rejoined || replayed.Created || replayed.RunID != admission.RunID {
		t.Fatalf("same key must rejoin: %+v", replayed)
	}
	// Same key + different request is an idempotency conflict.
	conflictReq := reportRequest("op-a")
	conflictReq.Window = rc.WindowCurrent
	_, err = svc.StartReport(ctx, reportAdmissionContext(), conflictReq)
	var rcErr *rc.Error
	if !errors.As(err, &rcErr) || rcErr.Code != rc.CodeIdempotencyConflict {
		t.Fatalf("same key different request must conflict, got %v", err)
	}

	// Distinct key against the still-admitted same target reports busy once
	// the existing run is non-terminal. Wait for op-a's run to fail closed
	// (R0 binds no effects) so the parked row below wins the target.
	for i := 0; i < 200; i++ {
		fresh, getErr := backend.GetRun(ctx, run.ID)
		if getErr != nil {
			t.Fatal(getErr)
		}
		if fresh.Status.Terminal() {
			break
		}
		time.Sleep(5 * time.Millisecond)
		if i == 199 {
			t.Fatal("op-a run never reached a terminal state")
		}
	}
	parked := parkedReportAdmission(t, svc, backend, "op-parked", "daily/completed/")
	parkedResult, err := backend.CommitWorkflowAdmission(ctx, parked)
	if err != nil {
		t.Fatal(err)
	}
	if parkedResult.Busy || !parkedResult.Created {
		t.Fatalf("parked admission must own the free target: %+v", parkedResult)
	}
	busy, err := svc.StartReport(ctx, reportAdmissionContext(), reportRequest("op-busy"))
	if err != nil {
		t.Fatal(err)
	}
	if !busy.Busy || busy.Created {
		t.Fatalf("distinct key on active target must be busy: %+v", busy)
	}
	// A different target key still admits.
	other := reportRequest("op-other")
	other.Target = &rc.TargetRef{SectionID: "sec", EntryID: "ent"}
	created, err := svc.StartReport(ctx, reportAdmissionContext(), other)
	if err != nil {
		t.Fatal(err)
	}
	if !created.Created || created.Busy {
		t.Fatalf("different target must admit: %+v", created)
	}
	// Invalid requests fail before any control state mutates.
	if _, err := svc.StartReport(ctx, reportAdmissionContext(), rc.ReportRequest{}); err == nil {
		t.Fatal("empty request must be rejected")
	}
}

// parkedReportAdmission builds a valid schema-2 trusted-root admission that
// stays in the accepted state (never executed) for busy-target checks.
func parkedReportAdmission(t *testing.T, svc *Service, backend storage.Engine, opKey, targetKey string) storage.WorkflowAdmission {
	t.Helper()
	ctx := context.Background()
	admitted, err := reportStrategyAdmission(ctx)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := svc.engine.cfg.Policy.Snapshot(svc.deps.PolicyDefaultProfile)
	if err != nil {
		t.Fatal(err)
	}
	control, err := backend.GetSession(ctx, reportControlSessionID(string(nb.HomeScopeID)))
	if err != nil {
		t.Fatal(err)
	}
	sandboxMode, approvalPolicy := control.EffectiveSandbox()
	authorityJSON, authorityDigest, err := trustedWorkflowAuthorityRecord(snapshot, sandboxMode, approvalPolicy, TrustedStrategyReport)
	if err != nil {
		t.Fatal(err)
	}
	limitsJSON, err := json.Marshal(inofyWorkflowLimits())
	if err != nil {
		t.Fatal(err)
	}
	descriptorDigest := sha256Hex(admitted.CanonicalJSON)
	hostBinding := inofyHostBinding(authorityDigest, admitted.Meta.ProgramDigest)
	requestDigest := "00" + descriptorDigest[2:]
	workflowRunID := domain.RunID("wfr-" + opKey)
	now := int64(1)
	return storage.WorkflowAdmission{
		Revision: domain.WorkflowRevision{
			RunID: workflowRunID, ParentSessionID: control.ID, RootRunID: workflowRunID,
			OperationKey: opKey, RootPurpose: TrustedStrategyReport,
			AdmissionNamespace: string(control.ID), RequestDigest: requestDigest, TargetKey: targetKey,
			DescriptorDigest: descriptorDigest, AuthorityDigest: authorityDigest,
			DescriptorJSON: admitted.CanonicalJSON, AuthorityJSON: authorityJSON,
			SchemaVersion: 2, CreatedAt: now,
			ProgramDigest: admitted.Meta.ProgramDigest, CatalogDigest: admitted.Meta.CatalogDigest,
			CompilerVersion: admitted.Meta.CompilerVersion, EinoBuild: admitted.Meta.EinoBuild,
			InputDigest: descriptorDigest, InputJSON: []byte(`{}`), EffectiveLimits: limitsJSON,
			HostBindingID: hostBinding,
		},
		Run: domain.Run{ID: workflowRunID, SessionID: control.ID, Status: domain.RunAccepted,
			Kind: domain.RunKindWorkflow, RootID: workflowRunID, CreatedAt: now, Purpose: domain.RunPurposeReport},
		Started: domain.RunEvent{RunID: workflowRunID, Type: domain.EventRunStarted, CreatedAt: now,
			PayloadVersion: 1, Payload: []byte(`{"mode":"report"}`)},
	}
}

// TestReportExecutorRejectsEscalation proves the restricted executor never
// runs effects outside the sealed four-node program and fails closed on
// mismatched run identity or unbound capabilities.
func TestReportExecutorRejectsEscalation(t *testing.T) {
	ctx := context.Background()
	svc, backend := inofyExecService(t, testsupport.NewEchoModel())
	admission, err := svc.StartReport(ctx, reportAdmissionContext(), reportRequest("op-exec"))
	if err != nil {
		t.Fatal(err)
	}
	revision, err := backend.GetWorkflowRevision(ctx, domain.RunID(admission.RunID))
	if err != nil {
		t.Fatal(err)
	}
	exec := newReportNodeExecutor(svc)
	ref := inofy.ExecutionRef{RunID: admission.RunID, Epoch: 1,
		ProgramDigest: revision.ProgramDigest, HostBindingID: revision.HostBindingID}

	// Authored/foreign node types and implementations are denied outright.
	for _, call := range []inofy.NodeCall{
		{Ref: ref, TypeID: workflowChildType, ImplementationID: workflowChildType, OperationKey: "n1"},
		{Ref: ref, TypeID: reportNodeCollect, ImplementationID: "other/impl", OperationKey: "n2"},
		{Ref: ref, TypeID: "ev.evil@1", ImplementationID: reportImplementationID, OperationKey: "n3"},
	} {
		if _, err := exec.Execute(ctx, call); err == nil {
			t.Fatalf("untrusted node %+v must be denied", call)
		}
	}
	// A sealed call bound to a non-report run is denied. Reuse a parked
	// non-report workflow run.
	mustCreateSession(t, backend, "sess-plain")
	if err := backend.CreateRun(ctx, domain.Run{ID: "plain-1", SessionID: "sess-plain",
		Status: domain.RunActive, Kind: domain.RunKindPrimary, CreatedAt: 1}); err != nil {
		t.Fatal(err)
	}
	plainRef := inofy.ExecutionRef{RunID: "plain-1", Epoch: 1,
		ProgramDigest: revision.ProgramDigest, HostBindingID: revision.HostBindingID}
	if _, err := exec.Execute(ctx, inofy.NodeCall{Ref: plainRef, TypeID: reportNodeCollect,
		ImplementationID: reportImplementationID, OperationKey: "n4"}); err == nil {
		t.Fatal("sealed call on a non-report run must be denied")
	}
	// A verified sealed call dispatches to the bounded effect; an empty
	// bound input fails honestly with an INOFY-typed error, never a
	// fabricated success.
	if _, err := exec.Execute(ctx, inofy.NodeCall{Ref: ref, TypeID: reportNodeCollect,
		ImplementationID: reportImplementationID, OperationKey: "collect-1"}); err == nil {
		t.Fatal("report effect with empty input must fail honestly")
	} else {
		var inofyErr *inofy.Error
		if !errors.As(err, &inofyErr) {
			t.Fatalf("expected inofy-typed error, got %v", err)
		}
	}
}
