package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
	"agent-vivy/internal/tools"
)

// rebuildPendingChild restores only a child suspended before an effectful tool
// call. Other child Runs fail closed during startup recovery because their
// last operation may have crossed an external effect boundary.
func (s *Service) rebuildPendingChild(ctx context.Context, run domain.Run, approval domain.Approval, workspaceID string) error {
	if run.Kind != domain.RunKindChild || approval.Kind != domain.ApprovalKindChild || approval.RunID != run.ID {
		return errors.New("runtime: recovered child approval identity is invalid")
	}
	toolName, selectedTools, mode, face, profile, recordedSnapshot, sandboxMode, approvalPolicy := s.approvalDetails(ctx, run.ID)
	if toolName == "" || recordedSnapshot.Hash == "" {
		return errors.New("runtime: recovered child approval is missing its immutable request scope")
	}
	canonical, err := domain.CanonicalToolNames(selectedTools)
	if err != nil {
		return fmt.Errorf("runtime: recovered child tool selection: %w", err)
	}
	if !containsTool(canonical, toolName) {
		return errors.New("runtime: recovered approval tool is outside the child selection")
	}
	policy, err := s.engine.cfg.Policy.Snapshot(profile)
	if err != nil || policy.Hash != recordedSnapshot.Hash {
		return errors.New("runtime: recovered child policy no longer matches the admitted policy")
	}
	if approval.SandboxMode != "" && domain.SandboxMode(approval.SandboxMode) != sandboxMode {
		return errors.New("runtime: recovered child sandbox differs from the approval record")
	}
	if approval.ApprovalPolicy != "" && domain.ApprovalPolicy(approval.ApprovalPolicy) != approvalPolicy {
		return errors.New("runtime: recovered child approval policy differs from the approval record")
	}

	var childEngine *Engine
	var childState *childActivationDrive
	execution := runExecutionOptions{}
	if run.EffectiveChildMode() == domain.ChildModeContinuable {
		if s.deps.ChildSessions == nil || s.deps.ChildMailbox == nil || s.deps.Messages == nil || s.deps.Sessions == nil {
			return errors.New("runtime: continuable child recovery stores are unavailable")
		}
		binding, err := s.deps.ChildSessions.GetChildSessionBinding(ctx, run.SessionID)
		if err != nil {
			return fmt.Errorf("runtime: load child binding for recovery: %w", err)
		}
		if binding.State != domain.ChildSessionOpen || binding.ActivationRunID != run.ID || binding.AuthorizerRunID != run.ParentID {
			return storage.ErrChildAdmissionConflict
		}
		digest, err := binding.AuthorityCeiling.Digest()
		if err != nil || digest != binding.AuthorityCeilingDigest {
			return errors.New("runtime: recovered child authority ceiling failed its digest check")
		}
		if profile != binding.AuthorityCeiling.PolicyProfile || policy.Hash != binding.AuthorityCeiling.PolicyHash ||
			sandboxMode != binding.AuthorityCeiling.SandboxMode || approvalPolicy != binding.AuthorityCeiling.ApprovalPolicy ||
			!sameStrings(canonical, binding.ActivationToolNames) {
			return storage.ErrChildAdmissionConflict
		}
		childSession, err := s.deps.Sessions.GetSession(ctx, binding.ChildSessionID)
		if err != nil {
			return fmt.Errorf("runtime: load child Session for recovery: %w", err)
		}
		currentSandbox, currentApproval := childSession.EffectiveSandbox()
		if currentSandbox != binding.AuthorityCeiling.SandboxMode || currentApproval != binding.AuthorityCeiling.ApprovalPolicy {
			return errors.New("runtime: recovered child Session settings exceed its immutable authority ceiling")
		}
		childEngine, err = s.engine.ChildView(ctx, canonical)
		if err != nil {
			return err
		}
		task, err := activationTask(ctx, s.deps.Messages, run.SessionID, run.ID)
		if err != nil {
			return err
		}
		pending, err := s.recoverChildInProgressMail(ctx, binding, run.ID)
		if err != nil {
			return err
		}
		childState = &childActivationDrive{binding: binding, task: task, pending: pending, included: pending}
		execution.child = childState
	} else if run.EffectiveChildMode() == domain.ChildModeOneShot {
		for _, name := range canonical {
			if name == tools.AgentName || name == tools.WorkflowName || name == tools.ReplyParentName || strings.HasPrefix(name, "mcp_") {
				return fmt.Errorf("runtime: recovered one-shot child has forbidden tool %q", name)
			}
		}
		childEngine, err = s.engine.OneShotView(ctx, canonical)
		if err != nil {
			return err
		}
		task, err := childRunTask(ctx, s.deps.Journal, run.ID)
		if err != nil {
			return err
		}
		childState = &childActivationDrive{task: task}
		execution.oneShotChild = true
		execution.suppressSessionMessageProjection = true
		execution.child = childState
	} else {
		return errors.New("runtime: recovered child mode is unsupported")
	}
	execution.engine = childEngine

	ledger := s.recoverBudgetLedger(ctx, run.ID)
	if ledger == nil {
		return errors.New("runtime: recovered child budget ledger is unavailable")
	}
	mapper := newEventMapper(run.ID, childEngine.cfg.MaxEventPayloadBytes)
	mapper.setRunScope(s.deps.TenantID, workspaceID, string(run.SessionID))
	mapper.setContextViewID(s.contextViewForRun(ctx, run.ID))
	providerName, modelID := s.usageRoutesForRun(ctx, run.ID)
	mapper.setUsageRoutes(providerName, modelID, childEngine.cfg.SummaryModelID)
	mapper.registerOpenCall(openToolCall{id: approval.ToolCallID, name: toolName})

	s.mu.Lock()
	s.pending[run.ID] = pendingRun{
		sessionID: run.SessionID, workspaceID: workspaceID, mapper: mapper,
		selectedTools: canonical, mode: mode, profile: profile, snapshot: policy,
		sandboxMode: sandboxMode, approvalPolicy: approvalPolicy, face: face,
		mounted: s.recoveredMounts(ctx, run.ID), ledger: ledger,
		engine: childEngine, execution: execution,
	}
	s.runSessions[run.ID] = run.SessionID
	s.ledgers[run.ID] = ledger
	s.snapshots[run.ID] = policy
	s.runTools[run.ID] = childToolSet(canonical)
	s.mu.Unlock()
	return nil
}

func (s *Service) recoverChildInProgressMail(ctx context.Context, binding domain.ChildSessionBinding, runID domain.RunID) ([]domain.ChildMailboxMessage, error) {
	pending, err := s.deps.ChildMailbox.ListPendingChildMessages(ctx, binding.ChildSessionID, binding.ChildSessionID, binding.ConsumedMessageSequence, maxChildMailboxBatch)
	if err != nil {
		return nil, fmt.Errorf("runtime: list child mail for recovery: %w", err)
	}
	result := make([]domain.ChildMailboxMessage, 0, len(pending))
	for _, item := range pending {
		receipt, err := s.deps.ChildMailbox.GetChildMessageReceipt(ctx, binding.ChildSessionID, item.ID, runID)
		if errors.Is(err, storage.ErrNotFound) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("runtime: load child mail receipt for recovery: %w", err)
		}
		if receipt.State != domain.ChildMessageReceiptInProgress {
			continue
		}
		item.Body = append([]byte(nil), item.Body...)
		result = append(result, item)
	}
	return result, nil
}

func childRunTask(ctx context.Context, journal storage.Journal, runID domain.RunID) (string, error) {
	iterator, err := journal.Replay(ctx, runID, 0)
	if err != nil {
		return "", err
	}
	defer func() { _ = iterator.Close() }()
	for iterator.Next() {
		event := iterator.Value().Event
		if event.Type != domain.EventChildRequested {
			continue
		}
		var payload struct {
			Task string `json:"text"`
		}
		if err := json.Unmarshal(event.Payload, &payload); err != nil {
			return "", err
		}
		if strings.TrimSpace(payload.Task) == "" || len(payload.Task) > maxChildTaskBytes {
			return "", errors.New("runtime: recovered one-shot child task is invalid")
		}
		return payload.Task, nil
	}
	if err := iterator.Err(); err != nil {
		return "", err
	}
	return "", storage.ErrNotFound
}

func containsTool(names []string, target string) bool {
	for _, name := range names {
		if name == target {
			return true
		}
	}
	return false
}
