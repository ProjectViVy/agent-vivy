package runtime

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	inofy "github.com/ProjectViVy/inofy"

	"agent-vivy/internal/domain"
	rc "agent-vivy/internal/reportcontract"
	"agent-vivy/internal/storage"
)

// reportControlSessionID derives the deterministic hidden control Session
// identity for one notebook scope. The control Session owns the report
// admission lock and namespaces all report operation keys for that scope.
func reportControlSessionID(scope string) domain.SessionID {
	sum := sha256.Sum256([]byte("vivy.report.control\x00" + scope))
	return domain.SessionID("reportctl_" + hex.EncodeToString(sum[:16]))
}

// reportRequestDigest is the canonical digest of the caller's stable
// semantic request. Dynamic source/feedback snapshots never join it, so a
// retried request rejoins the admitted Run instead of forking a new one.
func reportRequestDigest(scope string, req rc.ReportRequest) (string, error) {
	canonical, err := json.Marshal(struct {
		Scope  string            `json:"scope"`
		Period rc.Period         `json:"period"`
		Window rc.WindowSelector `json:"window"`
		Target *rc.TargetRef     `json:"target,omitempty"`
	}{Scope: scope, Period: req.Period, Window: req.Window, Target: req.Target})
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(canonical)
	return hex.EncodeToString(sum[:]), nil
}

// reportTargetKey scopes the same-active-target exclusivity check inside the
// admission namespace.
func reportTargetKey(req rc.ReportRequest) string {
	var target string
	if req.Target != nil {
		target = req.Target.SectionID + "/" + req.Target.EntryID
	}
	return string(req.Period) + "/" + string(req.Window) + "/" + target
}

// ensureReportControlSession returns the hidden control Session for the
// scope, creating it on first use. Creation races resolve to the same
// deterministic id; a row that already exists must carry the report-control
// purpose or the admission refuses to share a chat Session's lock.
func (s *Service) ensureReportControlSession(ctx context.Context, scope string) (domain.Session, error) {
	id := reportControlSessionID(scope)
	if existing, err := s.deps.Sessions.GetSession(ctx, id); err == nil {
		if existing.Purpose != domain.SessionPurposeReportControl {
			return domain.Session{}, errors.New("runtime: report control session id collides with an interactive session")
		}
		return existing, nil
	} else if !errors.Is(err, storage.ErrNotFound) {
		return domain.Session{}, err
	}
	now := time.Now().UnixMilli()
	session := domain.Session{
		ID:        id,
		Title:     "report control",
		CreatedAt: now,
		UpdatedAt: now,
		Purpose:   domain.SessionPurposeReportControl,
	}
	if err := s.deps.Sessions.CreateSession(ctx, session); err != nil {
		// A concurrent creator won; re-read and require the trusted purpose.
		existing, getErr := s.deps.Sessions.GetSession(ctx, id)
		if getErr != nil {
			return domain.Session{}, err
		}
		if existing.Purpose != domain.SessionPurposeReportControl {
			return domain.Session{}, errors.New("runtime: report control session id collides with an interactive session")
		}
		return existing, nil
	}
	return session, nil
}

// StartReport is the internal adapter the reports module calls to admit one
// trusted root workflow Run. It shares the workflow admission transaction,
// Journal, policy snapshot, execution ownership and cancellation machinery;
// the only difference from the child lane is the namespace owner: the
// hidden per-scope control Session instead of a parent Run.
func (s *Service) StartReport(ctx context.Context, ac rc.AdmissionContext, req rc.ReportRequest) (rc.ReportAdmission, error) {
	if s == nil || s.engine == nil || s.deps.Journal == nil || s.deps.Runs == nil ||
		s.deps.Sessions == nil || s.deps.Sink == nil || s.deps.Workspaces == nil ||
		s.deps.WorkflowRevisions == nil {
		return rc.ReportAdmission{}, errors.New("runtime: report admission is not wired")
	}
	if _, err := s.inofyEngine(); err != nil {
		return rc.ReportAdmission{}, err
	}
	operationKey := strings.TrimSpace(req.OperationKey)
	if operationKey == "" || len(operationKey) > 128 ||
		!req.Period.Valid() || !req.Window.Valid() || ac.Scope == "" ||
		ac.Actor.Kind == "" || ac.Origin == "" {
		return rc.ReportAdmission{}, &rc.Error{Code: rc.CodeInvalidRequest,
			Message: "report request requires a valid period, window, scope, actor, origin, and operation key"}
	}
	control, err := s.ensureReportControlSession(ctx, string(ac.Scope))
	if err != nil {
		return rc.ReportAdmission{}, err
	}
	requestDigest, err := reportRequestDigest(string(ac.Scope), req)
	if err != nil {
		return rc.ReportAdmission{}, err
	}
	targetKey := reportTargetKey(req)

	// Serialize same-process retries so an accepted Run cannot be activated
	// twice while its process-local authority state is built.
	s.workflowStartMu.Lock()
	defer s.workflowStartMu.Unlock()

	s.projectionMu.Lock()
	if s.sessionDeleted(control.ID) {
		s.projectionMu.Unlock()
		return rc.ReportAdmission{}, storage.ErrNotFound
	}
	admitted, err := reportStrategyAdmission(ctx)
	if err != nil {
		s.projectionMu.Unlock()
		return rc.ReportAdmission{}, err
	}
	runInput, err := json.Marshal(struct {
		Scope         string            `json:"scope"`
		Period        rc.Period         `json:"period"`
		Window        rc.WindowSelector `json:"window"`
		Target        *rc.TargetRef     `json:"target,omitempty"`
		OperationKey  string            `json:"operation_key"`
		RequestDigest string            `json:"request_digest"`
	}{Scope: string(ac.Scope), Period: req.Period, Window: req.Window, Target: req.Target,
		OperationKey: operationKey, RequestDigest: requestDigest})
	if err != nil {
		s.projectionMu.Unlock()
		return rc.ReportAdmission{}, err
	}
	canonicalInput, inputDigest, err := normalizeINOFYInput(runInput)
	if err != nil {
		s.projectionMu.Unlock()
		return rc.ReportAdmission{}, err
	}
	snapshot, err := s.engine.cfg.Policy.Snapshot(s.deps.PolicyDefaultProfile)
	if err != nil {
		s.projectionMu.Unlock()
		return rc.ReportAdmission{}, err
	}
	sandboxMode, approvalPolicy := control.EffectiveSandbox()
	authorityJSON, authorityDigest, err := trustedWorkflowAuthorityRecord(snapshot, sandboxMode, approvalPolicy, TrustedStrategyReport)
	if err != nil {
		s.projectionMu.Unlock()
		return rc.ReportAdmission{}, err
	}
	limitsJSON, err := json.Marshal(inofyWorkflowLimits())
	if err != nil {
		s.projectionMu.Unlock()
		return rc.ReportAdmission{}, err
	}
	descriptorDigest := sha256Hex(admitted.CanonicalJSON)
	hostBinding := inofyHostBinding(authorityDigest, admitted.Meta.ProgramDigest)
	namespace := string(control.ID)

	if existing, lookupErr := s.deps.WorkflowRevisions.GetWorkflowRevisionByOperationInNamespace(ctx, namespace, operationKey); lookupErr == nil {
		if existing.SchemaVersion != 2 || existing.DescriptorDigest != descriptorDigest ||
			existing.AuthorityDigest != authorityDigest || existing.ParentSessionID != control.ID ||
			existing.ProgramDigest != admitted.Meta.ProgramDigest || existing.HostBindingID != hostBinding ||
			existing.InputDigest != inputDigest || existing.RequestDigest != requestDigest ||
			existing.TargetKey != targetKey || existing.RootPurpose != TrustedStrategyReport ||
			!bytes.Equal(existing.DescriptorJSON, admitted.CanonicalJSON) {
			s.projectionMu.Unlock()
			return rc.ReportAdmission{}, &rc.Error{Code: rc.CodeIdempotencyConflict,
				Message: "operation key was admitted with a different report request"}
		}
		run, getErr := s.deps.Runs.GetRun(ctx, existing.RunID)
		if getErr != nil {
			s.projectionMu.Unlock()
			return rc.ReportAdmission{}, getErr
		}
		if run.Status.Terminal() {
			s.projectionMu.Unlock()
			return rc.ReportAdmission{RunID: string(run.ID), Rejoined: true}, nil
		}
		if run.Status == domain.RunActive {
			s.mu.Lock()
			_, local := s.active[run.ID]
			s.mu.Unlock()
			s.projectionMu.Unlock()
			if local {
				return rc.ReportAdmission{RunID: string(run.ID), Rejoined: true}, nil
			}
			return rc.ReportAdmission{RunID: string(run.ID), Rejoined: true}, ErrWorkflowRecoveryRequired
		}
		// Run accepted but never executed (post-admission crash). Rebind the
		// committed projection and re-run the compiled program at the next
		// writer epoch.
		epoch, fenceErr := s.inofyResumeEpoch(ctx, existing.RunID)
		if fenceErr != nil {
			s.projectionMu.Unlock()
			return rc.ReportAdmission{}, fenceErr
		}
		activationCtx, activationCancel := context.WithTimeout(context.WithoutCancel(ctx), terminalPersistTimeout)
		result, startErr := s.activateReportLocked(activationCtx, control, snapshot, existing, run)
		s.projectionMu.Unlock()
		if startErr != nil {
			activationCancel()
			return rc.ReportAdmission{}, startErr
		}
		nodes, nodesErr := s.workflowNodes(activationCtx, existing.RunID, TrustedStrategyReport, canonicalInput)
		if nodesErr != nil {
			activationCancel()
			return rc.ReportAdmission{}, nodesErr
		}
		launchErr := s.launchINOFYWorkflow(activationCtx, result, admitted.Program, canonicalInput, inofy.ExecutionRef{
			RunID: string(existing.RunID), Epoch: epoch,
			ProgramDigest: existing.ProgramDigest, HostBindingID: existing.HostBindingID,
		}, nodes)
		activationCancel()
		if launchErr != nil {
			return rc.ReportAdmission{}, launchErr
		}
		return rc.ReportAdmission{RunID: string(result.Run.ID), Rejoined: true}, nil
	} else if !errors.Is(lookupErr, storage.ErrNotFound) {
		s.projectionMu.Unlock()
		return rc.ReportAdmission{}, lookupErr
	}

	now := time.Now().UnixMilli()
	workflowRunID := domain.RunID(newPrefixedID("workflow_"))
	run := domain.Run{
		ID: workflowRunID, SessionID: control.ID, Status: domain.RunAccepted, CreatedAt: now,
		Kind: domain.RunKindWorkflow, RootID: workflowRunID, Depth: 0, Purpose: domain.RunPurposeReport,
	}
	workspace, err := s.deps.Workspaces.Ensure(withSessionID(ctx, control.ID), run.ID)
	if err != nil {
		s.projectionMu.Unlock()
		return rc.ReportAdmission{}, fmt.Errorf("runtime: allocate report workspace: %w", err)
	}
	mapper := newEventMapper(run.ID, s.engine.cfg.MaxEventPayloadBytes)
	mapper.setRunScope(s.deps.TenantID, workspace.ID, string(control.ID))
	providerName, modelID := s.CurrentModel()
	mapper.setUsageRoutes(providerName, modelID, s.engine.cfg.SummaryModelID)
	started := mapper.build(domain.EventRunStarted, payloadRunStarted{
		Provider: providerName, Model: modelID, Mode: string(domain.RunModeNormal), Face: string(domain.FaceWeb),
		PolicyProfile: string(snapshot.Profile), PolicyHash: snapshot.Hash,
		SandboxMode: string(sandboxMode), ApprovalPolicy: string(approvalPolicy),
	})
	started.CreatedAt = now
	revision := domain.WorkflowRevision{
		RunID: run.ID, ParentSessionID: control.ID, RootRunID: run.ID,
		OperationKey: operationKey, RootPurpose: TrustedStrategyReport,
		AdmissionNamespace: namespace, RequestDigest: requestDigest, TargetKey: targetKey,
		DescriptorDigest: descriptorDigest, AuthorityDigest: authorityDigest,
		DescriptorJSON: append([]byte(nil), admitted.CanonicalJSON...), AuthorityJSON: authorityJSON,
		SchemaVersion: 2, CreatedAt: now,
		ProgramDigest: admitted.Meta.ProgramDigest, CatalogDigest: admitted.Meta.CatalogDigest,
		CompilerVersion: admitted.Meta.CompilerVersion, EinoBuild: admitted.Meta.EinoBuild,
		InputDigest: inputDigest, InputJSON: append([]byte(nil), canonicalInput...),
		EffectiveLimits: limitsJSON, HostBindingID: hostBinding,
	}
	committed, err := s.deps.WorkflowRevisions.CommitWorkflowAdmission(ctx, storage.WorkflowAdmission{
		Revision: revision, Run: run, Started: started,
	})
	if err != nil {
		if releaser, ok := s.deps.Workspaces.(WorkspaceReleaser); ok {
			_ = releaser.Release(context.WithoutCancel(ctx), workspace)
		}
		s.projectionMu.Unlock()
		return rc.ReportAdmission{}, err
	}
	if committed.Busy {
		s.projectionMu.Unlock()
		return rc.ReportAdmission{RunID: string(committed.Run.ID), Busy: true}, nil
	}
	if committed.Revision.DescriptorDigest != descriptorDigest || committed.Revision.AuthorityDigest != authorityDigest ||
		committed.Revision.SchemaVersion != 2 || committed.Revision.ProgramDigest != admitted.Meta.ProgramDigest ||
		committed.Revision.InputDigest != inputDigest || committed.Revision.RequestDigest != requestDigest ||
		committed.Revision.TargetKey != targetKey {
		s.projectionMu.Unlock()
		return rc.ReportAdmission{}, storage.ErrWorkflowRevisionConflict
	}
	if committed.Run.Status.Terminal() {
		s.projectionMu.Unlock()
		return rc.ReportAdmission{RunID: string(committed.Run.ID), Rejoined: true}, nil
	}
	if committed.Run.Status == domain.RunActive && !committed.Created {
		s.mu.Lock()
		_, local := s.active[committed.Run.ID]
		s.mu.Unlock()
		s.projectionMu.Unlock()
		if local {
			return rc.ReportAdmission{RunID: string(committed.Run.ID), Rejoined: true}, nil
		}
		return rc.ReportAdmission{RunID: string(committed.Run.ID), Rejoined: true}, ErrWorkflowRecoveryRequired
	}
	activationCtx, activationCancel := context.WithTimeout(context.WithoutCancel(ctx), terminalPersistTimeout)
	result, err := s.activateReportLocked(activationCtx, control, snapshot, committed.Revision, committed.Run)
	s.projectionMu.Unlock()
	if err != nil {
		activationCancel()
		return rc.ReportAdmission{}, err
	}
	if committed.Created {
		s.publish(activationCtx, committed.Started)
	}
	nodes, nodesErr := s.workflowNodes(activationCtx, committed.Run.ID, TrustedStrategyReport, canonicalInput)
	if nodesErr != nil {
		activationCancel()
		return rc.ReportAdmission{}, nodesErr
	}
	launchErr := s.launchINOFYWorkflow(activationCtx, result, admitted.Program, canonicalInput, inofy.ExecutionRef{
		RunID: string(committed.Run.ID), Epoch: 1,
		ProgramDigest: committed.Revision.ProgramDigest, HostBindingID: committed.Revision.HostBindingID,
	}, nodes)
	activationCancel()
	if launchErr != nil {
		return rc.ReportAdmission{}, launchErr
	}
	return rc.ReportAdmission{RunID: string(result.Run.ID), Created: committed.Created}, nil
}

// activateReportLocked installs process-local authority for a report root:
// its own session binding, a fresh root budget ledger, workspace, and run
// status, under the same deletion fence as the child path.
func (s *Service) activateReportLocked(ctx context.Context, control domain.Session, snapshot domain.PolicySnapshot,
	revision domain.WorkflowRevision, run domain.Run) (WorkflowStartResult, error) {
	if s.sessionDeleted(control.ID) {
		return WorkflowStartResult{}, storage.ErrNotFound
	}
	workspace, err := s.deps.Workspaces.Ensure(withSessionID(ctx, control.ID), run.ID)
	if err != nil {
		return WorkflowStartResult{}, err
	}
	ledger, err := NewBudgetLedger(s.deps.Budget)
	if err != nil {
		return WorkflowStartResult{}, err
	}
	if run.Status == domain.RunAccepted {
		if err := s.deps.Runs.SetRunStatus(ctx, run.ID, domain.RunActive); err != nil {
			return WorkflowStartResult{}, err
		}
		run.Status = domain.RunActive
	}
	s.mu.Lock()
	s.runSessions[run.ID] = control.ID
	s.ledgers[run.ID] = ledger
	s.snapshots[run.ID] = snapshot
	s.runTools[run.ID] = childToolSet(nil)
	s.mu.Unlock()
	_ = workspace
	return WorkflowStartResult{Run: run, Revision: revision, Created: true}, nil
}
