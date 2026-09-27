package runtime

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/orchestration"
	"agent-vivy/internal/storage"
)

var (
	ErrWorkflowRecoveryRequired   = errors.New("runtime: active workflow requires recovery; refusing to replay it")
	ErrWorkflowNodeUnknownOutcome = errors.New("runtime: workflow node outcome is unknown; refusing to replay it")
)

type WorkflowRequest struct {
	ParentRunID  domain.RunID
	OperationKey string
	Descriptor   orchestration.Descriptor
}

type WorkflowStartResult struct {
	Run      domain.Run
	Revision domain.WorkflowRevision
	Created  bool
}

type WorkflowNodeProjection struct {
	Key           string `json:"key"`
	Status        string `json:"status"`
	ChildRunID    string `json:"child_run_id,omitempty"`
	ResultDigest  string `json:"result_digest,omitempty"`
	ErrorCategory string `json:"error_category,omitempty"`
	Message       string `json:"message,omitempty"`
}

type WorkflowDetails struct {
	Run            domain.Run
	RevisionDigest string
	Descriptor     orchestration.Descriptor
	Nodes          []WorkflowNodeProjection
	Outputs        map[string]string
}

// ProposeWorkflow validates a descriptor against the active author's current
// read-only child-tool ceiling. It does not create a Run or persist the graph.
func (s *Service) ProposeWorkflow(ctx context.Context, parentRunID domain.RunID, descriptor orchestration.Descriptor) (orchestration.ValidatedDescriptor, error) {
	if s == nil || s.engine == nil || s.deps.Runs == nil {
		return orchestration.ValidatedDescriptor{}, errors.New("runtime: workflow validation is not wired")
	}
	parent, _, _, parentTools, err := s.currentChildAuthorizer(ctx, parentRunID)
	if err != nil {
		return orchestration.ValidatedDescriptor{}, err
	}
	if err := s.validateWorkflowDepth(parent); err != nil {
		return orchestration.ValidatedDescriptor{}, err
	}
	return orchestration.Validate(descriptor, s.workflowChildTools(parentTools))
}

func (s *Service) validateWorkflowDepth(parent domain.Run) error {
	// A workflow occupies one child level and each graph node occupies one
	// additional child level. Keep both inside the existing bounded tree.
	if parent.Depth+2 > maxChildSessionDepth {
		return fmt.Errorf("runtime: workflow and node children exceed maximum child depth %d", maxChildSessionDepth)
	}
	if parent.Kind == domain.RunKindWorkflow {
		return errors.New("runtime: nested workflows are not supported")
	}
	return nil
}

// StartWorkflow atomically admits an immutable descriptor revision and its
// workflow Run, then executes the graph through Eino and governed one-shot
// child Runs. A repeated operation key returns the same Run; an active Run
// from another process is fenced for explicit recovery instead of replay.
func (s *Service) StartWorkflow(ctx context.Context, request WorkflowRequest) (WorkflowStartResult, error) {
	if s == nil || s.engine == nil || s.engine.cfg.Checkpoints == nil || s.deps.Journal == nil ||
		s.deps.Runs == nil || s.deps.Sessions == nil || s.deps.Sink == nil || s.deps.Workspaces == nil ||
		s.deps.WorkflowRevisions == nil {
		return WorkflowStartResult{}, errors.New("runtime: workflow execution is not wired")
	}
	if request.ParentRunID == "" || strings.TrimSpace(request.OperationKey) == "" || len(request.OperationKey) > 128 {
		return WorkflowStartResult{}, errors.New("runtime: workflow parent Run and operation key are required")
	}
	request.OperationKey = strings.TrimSpace(request.OperationKey)

	// Serialize same-process retries so an accepted Run cannot be activated
	// twice while its process-local cancellation and authority state is built.
	s.workflowStartMu.Lock()
	defer s.workflowStartMu.Unlock()
	knownRevision, revisionLookupErr := s.deps.WorkflowRevisions.GetWorkflowRevisionByOperation(ctx, request.ParentRunID, request.OperationKey)
	if revisionLookupErr == nil {
		_, retryErr := validateStoredWorkflowDescriptor(knownRevision, request.Descriptor)
		if retryErr != nil {
			return WorkflowStartResult{}, retryErr
		}
		run, getErr := s.deps.Runs.GetRun(ctx, knownRevision.RunID)
		if getErr != nil {
			return WorkflowStartResult{}, getErr
		}
		if run.Status.Terminal() {
			return WorkflowStartResult{Run: run, Revision: knownRevision, Created: false}, nil
		}
		if run.Status == domain.RunActive {
			s.mu.Lock()
			_, local := s.active[run.ID]
			s.mu.Unlock()
			if local {
				return WorkflowStartResult{Run: run, Revision: knownRevision, Created: false}, nil
			}
			return WorkflowStartResult{Run: run, Revision: knownRevision, Created: false}, ErrWorkflowRecoveryRequired
		}
	} else if !errors.Is(revisionLookupErr, storage.ErrNotFound) {
		return WorkflowStartResult{}, revisionLookupErr
	}

	s.projectionMu.Lock()
	parent, snapshot, parentLedger, parentTools, err := s.currentChildAuthorizer(ctx, request.ParentRunID)
	if err != nil {
		s.projectionMu.Unlock()
		return WorkflowStartResult{}, err
	}
	if err := s.validateWorkflowDepth(parent); err != nil {
		s.projectionMu.Unlock()
		return WorkflowStartResult{}, err
	}
	if s.sessionDeleted(parent.SessionID) {
		s.projectionMu.Unlock()
		return WorkflowStartResult{}, storage.ErrNotFound
	}
	allowedTools := s.workflowChildTools(parentTools)
	validated, err := orchestration.Validate(request.Descriptor, allowedTools)
	if err != nil {
		s.projectionMu.Unlock()
		return WorkflowStartResult{}, err
	}
	session, err := s.deps.Sessions.GetSession(ctx, parent.SessionID)
	if err != nil {
		s.projectionMu.Unlock()
		return WorkflowStartResult{}, err
	}
	sandboxMode, approvalPolicy := session.EffectiveSandbox()
	authorityJSON, authorityDigest, err := workflowAuthorityRecord(snapshot, sandboxMode, approvalPolicy, allowedTools)
	if err != nil {
		s.projectionMu.Unlock()
		return WorkflowStartResult{}, err
	}

	if existing, lookupErr := s.deps.WorkflowRevisions.GetWorkflowRevisionByOperation(ctx, parent.ID, request.OperationKey); lookupErr == nil {
		if existing.DescriptorDigest != validated.Digest || existing.AuthorityDigest != authorityDigest ||
			existing.ParentSessionID != parent.SessionID || string(existing.DescriptorJSON) != string(validated.CanonicalJSON) {
			s.projectionMu.Unlock()
			return WorkflowStartResult{}, storage.ErrWorkflowRevisionConflict
		}
		run, getErr := s.deps.Runs.GetRun(ctx, existing.RunID)
		if getErr != nil {
			s.projectionMu.Unlock()
			return WorkflowStartResult{}, getErr
		}
		if run.Status.Terminal() {
			s.projectionMu.Unlock()
			return WorkflowStartResult{Run: run, Revision: existing, Created: false}, nil
		}
		if run.Status == domain.RunActive {
			s.mu.Lock()
			_, local := s.active[run.ID]
			s.mu.Unlock()
			s.projectionMu.Unlock()
			if local {
				return WorkflowStartResult{Run: run, Revision: existing, Created: false}, nil
			}
			return WorkflowStartResult{Run: run, Revision: existing, Created: false}, ErrWorkflowRecoveryRequired
		}
		activationCtx, activationCancel := context.WithTimeout(context.WithoutCancel(ctx), terminalPersistTimeout)
		result, startErr := s.activateWorkflowLocked(activationCtx, parent, snapshot, parentLedger, allowedTools, existing, run, false)
		s.projectionMu.Unlock()
		if startErr != nil {
			activationCancel()
			return WorkflowStartResult{}, startErr
		}
		launchErr := s.launchWorkflow(activationCtx, result, validated, sandboxMode, approvalPolicy, false)
		activationCancel()
		return result, launchErr
	} else if !errors.Is(lookupErr, storage.ErrNotFound) {
		s.projectionMu.Unlock()
		return WorkflowStartResult{}, lookupErr
	}

	if err := parentLedger.ReserveEvent(); err != nil {
		s.projectionMu.Unlock()
		return WorkflowStartResult{}, err
	}
	now := time.Now().UnixMilli()
	workflowRunID := domain.RunID(newPrefixedID("workflow_"))
	rootID := parent.RootID
	if rootID == "" {
		rootID = parent.ID
	}
	run := domain.Run{
		ID: workflowRunID, SessionID: parent.SessionID, Status: domain.RunAccepted, CreatedAt: now,
		Kind: domain.RunKindWorkflow, ParentID: parent.ID, RootID: rootID, Depth: parent.Depth + 1,
	}
	workspace, err := s.deps.Workspaces.Ensure(withSessionID(ctx, parent.SessionID), run.ID)
	if err != nil {
		s.projectionMu.Unlock()
		return WorkflowStartResult{}, fmt.Errorf("runtime: allocate workflow workspace: %w", err)
	}
	mapper := newEventMapper(run.ID, s.engine.cfg.MaxEventPayloadBytes)
	mapper.setRunScope(s.deps.TenantID, workspace.ID, string(parent.SessionID))
	providerName, modelID := s.CurrentModel()
	mapper.setUsageRoutes(providerName, modelID, s.engine.cfg.SummaryModelID)
	started := mapper.build(domain.EventRunStarted, payloadRunStarted{
		Provider: providerName, Model: modelID, Mode: string(domain.RunModeNormal), Face: string(domain.FaceWeb),
		PolicyProfile: string(snapshot.Profile), PolicyHash: snapshot.Hash,
		SandboxMode: string(sandboxMode), ApprovalPolicy: string(approvalPolicy),
	})
	started.CreatedAt = now
	revision := domain.WorkflowRevision{
		RunID: run.ID, ParentRunID: parent.ID, ParentSessionID: parent.SessionID, RootRunID: rootID,
		OperationKey: request.OperationKey, DescriptorDigest: validated.Digest, AuthorityDigest: authorityDigest,
		DescriptorJSON: append([]byte(nil), validated.CanonicalJSON...), AuthorityJSON: authorityJSON,
		SchemaVersion: validated.SchemaVersion, CreatedAt: now,
	}
	admitted, err := s.deps.WorkflowRevisions.CommitWorkflowAdmission(ctx, storage.WorkflowAdmission{
		Revision: revision, Run: run, Started: started,
	})
	if err != nil {
		if releaser, ok := s.deps.Workspaces.(WorkspaceReleaser); ok {
			_ = releaser.Release(context.WithoutCancel(ctx), workspace)
		}
		s.projectionMu.Unlock()
		return WorkflowStartResult{}, err
	}
	if admitted.Revision.DescriptorDigest != validated.Digest || admitted.Revision.AuthorityDigest != authorityDigest {
		s.projectionMu.Unlock()
		return WorkflowStartResult{}, storage.ErrWorkflowRevisionConflict
	}
	if admitted.Run.Status.Terminal() {
		s.projectionMu.Unlock()
		return WorkflowStartResult{Run: admitted.Run, Revision: admitted.Revision, Created: false}, nil
	}
	if admitted.Run.Status == domain.RunActive && !admitted.Created {
		s.mu.Lock()
		_, local := s.active[admitted.Run.ID]
		s.mu.Unlock()
		s.projectionMu.Unlock()
		if local {
			return WorkflowStartResult{Run: admitted.Run, Revision: admitted.Revision, Created: false}, nil
		}
		return WorkflowStartResult{Run: admitted.Run, Revision: admitted.Revision, Created: false}, ErrWorkflowRecoveryRequired
	}
	activationCtx, activationCancel := context.WithTimeout(context.WithoutCancel(ctx), terminalPersistTimeout)
	result, err := s.activateWorkflowLocked(activationCtx, parent, snapshot, parentLedger, allowedTools, admitted.Revision, admitted.Run, admitted.Created)
	s.projectionMu.Unlock()
	if err != nil {
		activationCancel()
		return WorkflowStartResult{}, err
	}
	if admitted.Created {
		s.publish(activationCtx, admitted.Started)
	}
	launchErr := s.launchWorkflow(activationCtx, result, validated, sandboxMode, approvalPolicy, admitted.Created)
	activationCancel()
	return result, launchErr
}

// activateWorkflowLocked installs process-local lineage, budget, workspace,
// and cancellation state while the session deletion fence is held.
func (s *Service) activateWorkflowLocked(ctx context.Context, parent domain.Run, snapshot domain.PolicySnapshot, parentLedger *BudgetLedger,
	allowedTools []string, revision domain.WorkflowRevision, run domain.Run, created bool) (WorkflowStartResult, error) {
	if s.sessionDeleted(parent.SessionID) {
		return WorkflowStartResult{}, storage.ErrNotFound
	}
	workspace, err := s.deps.Workspaces.Ensure(withSessionID(ctx, parent.SessionID), run.ID)
	if err != nil {
		return WorkflowStartResult{}, err
	}
	ledger, err := parentLedger.Child(s.deps.Budget)
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
	s.runSessions[run.ID] = parent.SessionID
	s.ledgers[run.ID] = ledger
	s.snapshots[run.ID] = snapshot
	s.runTools[run.ID] = childToolSet(allowedTools)
	s.mu.Unlock()
	_ = workspace // Ensure is also the host's workflow workspace authority.
	return WorkflowStartResult{Run: run, Revision: revision, Created: created}, nil
}

func (s *Service) launchWorkflow(ctx context.Context, result WorkflowStartResult, validated orchestration.ValidatedDescriptor,
	sandboxMode domain.SandboxMode, approvalPolicy domain.ApprovalPolicy, _ bool) error {
	if result.Run.ID == "" {
		return errors.New("runtime: admitted workflow Run is empty")
	}
	mapper := newEventMapper(result.Run.ID, s.engine.cfg.MaxEventPayloadBytes)
	workspace, err := s.deps.Workspaces.Ensure(withSessionID(context.WithoutCancel(ctx), result.Run.SessionID), result.Run.ID)
	if err != nil {
		s.emitTerminal(context.Background(), mapper, s.terminalEvent(context.Background(), mapper, err))
		return err
	}
	mapper.setRunScope(s.deps.TenantID, workspace.ID, string(result.Run.SessionID))
	providerName, modelID := s.CurrentModel()
	mapper.setUsageRoutes(providerName, modelID, s.engine.cfg.SummaryModelID)
	runCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
	s.mu.Lock()
	if currentCancel, active := s.active[result.Run.ID]; active {
		_ = currentCancel
		s.mu.Unlock()
		cancel()
		return nil
	}
	s.active[result.Run.ID] = cancel
	s.runSessions[result.Run.ID] = result.Run.SessionID
	s.mu.Unlock()
	started, err := s.workflowStartEventMatches(ctx, result.Run.ID, result.Revision.DescriptorDigest, len(validated.Nodes))
	if err != nil {
		s.emitTerminal(context.Background(), mapper, s.terminalEvent(context.Background(), mapper, err))
		return err
	}
	if !started {
		if _, err := s.persistWorkflowEvent(ctx, result.Run.ID, domain.EventWorkflowStarted, map[string]any{
			"revision_digest": result.Revision.DescriptorDigest, "node_count": len(validated.Nodes),
		}); err != nil {
			s.emitTerminal(context.Background(), mapper, s.terminalEvent(context.Background(), mapper, err))
			return err
		}
	}
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		outputs, runErr := s.executeWorkflowGraph(runCtx, result.Run.ID, validated, s.runWorkflowNode)
		if runErr != nil {
			s.emitTerminal(runCtx, mapper, s.terminalEvent(runCtx, mapper, runErr))
			return
		}
		if runCtx.Err() != nil {
			s.emitTerminal(runCtx, mapper, s.terminalEvent(runCtx, mapper, runCtx.Err()))
			return
		}
		encodedOutputs, encodeErr := json.Marshal(outputs)
		if encodeErr != nil || len(encodedOutputs) > orchestration.MaxOutputBytes {
			if encodeErr == nil {
				encodeErr = fmt.Errorf("runtime: workflow outputs exceed %d bytes", orchestration.MaxOutputBytes)
			}
			s.emitTerminal(runCtx, mapper, s.terminalEvent(runCtx, mapper, encodeErr))
			return
		}
		s.emitTerminal(runCtx, mapper, mapper.build(domain.EventRunCompleted, map[string]string{"summary": string(encodedOutputs)}))
	}()
	return nil
}

func (s *Service) runWorkflowNode(ctx context.Context, request WorkflowNodeRequest) (string, error) {
	childRunID := workflowNodeRunID(request.WorkflowRunID, request.Node.Key)
	state, err := s.readWorkflowNodeState(ctx, request.WorkflowRunID, request.Node.Key)
	if err != nil {
		return "", err
	}
	if state.failed {
		return "", errors.New("runtime: workflow node already has a durable failure")
	}
	if state.childRunID != "" && state.childRunID != string(childRunID) {
		return "", storage.ErrWorkflowRevisionConflict
	}
	if !state.started {
		if _, err := s.persistWorkflowEvent(ctx, request.WorkflowRunID, domain.EventWorkflowNodeStarted, map[string]string{
			"node_key": request.Node.Key, "child_run_id": string(childRunID),
		}); err != nil {
			return "", s.failWorkflowNode(ctx, request, childRunID, err)
		}
	}
	if state.completed {
		if state.childRunID == "" {
			return "", ErrWorkflowNodeUnknownOutcome
		}
		return s.completedWorkflowNodeOutput(ctx, request, domain.RunID(state.childRunID))
	}

	task, err := workflowNodeTask(request.Node.Task, request.Dependencies)
	if err != nil {
		return "", s.failWorkflowNode(ctx, request, childRunID, err)
	}
	child, err := s.StartOneShotChild(ctx, OneShotChildRequest{
		ParentRunID: request.WorkflowRunID, RunID: childRunID, Task: task, ToolNames: request.Node.ToolNames,
	})
	if err != nil {
		return "", s.failWorkflowNode(ctx, request, childRunID, err)
	}
	stopCancel := context.AfterFunc(ctx, func() { s.Cancel(childRunID) })
	defer stopCancel()
	for {
		current, getErr := s.deps.Runs.GetRun(ctx, childRunID)
		if getErr != nil {
			return "", s.failWorkflowNode(ctx, request, childRunID, getErr)
		}
		if current.Status.Terminal() {
			if current.Status != domain.RunCompleted {
				_, message, _, outcomeErr := s.ChildRunDetails(context.WithoutCancel(ctx), childRunID)
				if outcomeErr != nil {
					return "", s.failWorkflowNode(ctx, request, childRunID, outcomeErr)
				}
				if message == "" {
					message = "The child task did not complete successfully."
				}
				return "", s.failWorkflowNode(ctx, request, childRunID, errors.New(message))
			}
			return s.completeWorkflowNode(ctx, request, childRunID)
		}
		select {
		case <-ctx.Done():
			return "", s.failWorkflowNode(ctx, request, childRunID, ctx.Err())
		case <-time.After(40 * time.Millisecond):
		}
		_ = child // The durable RunStore row is authoritative after admission.
	}
}

func (s *Service) completedWorkflowNodeOutput(ctx context.Context, request WorkflowNodeRequest, childRunID domain.RunID) (string, error) {
	run, err := s.deps.Runs.GetRun(ctx, childRunID)
	if err != nil {
		return "", err
	}
	if run.Status != domain.RunCompleted {
		return "", ErrWorkflowNodeUnknownOutcome
	}
	summary, message, _, err := s.ChildRunDetails(ctx, childRunID)
	if err != nil {
		return "", err
	}
	if message != "" {
		return "", errors.New("runtime: completed child contains a failure event")
	}
	return summary, nil
}

func (s *Service) completeWorkflowNode(ctx context.Context, request WorkflowNodeRequest, childRunID domain.RunID) (string, error) {
	summary, message, _, err := s.ChildRunDetails(context.WithoutCancel(ctx), childRunID)
	if err != nil {
		return "", s.failWorkflowNode(ctx, request, childRunID, err)
	}
	if message != "" {
		return "", s.failWorkflowNode(ctx, request, childRunID, errors.New(message))
	}
	sum := sha256.Sum256([]byte(summary))
	if _, err := s.persistWorkflowEvent(ctx, request.WorkflowRunID, domain.EventWorkflowNodeCompleted, map[string]string{
		"node_key": request.Node.Key, "child_run_id": string(childRunID), "result_digest": hex.EncodeToString(sum[:]),
	}); err != nil {
		return "", err
	}
	return summary, nil
}

func (s *Service) failWorkflowNode(ctx context.Context, request WorkflowNodeRequest, childRunID domain.RunID, cause error) error {
	if cause == nil {
		cause = errors.New("workflow node failed")
	}
	state, readErr := s.readWorkflowNodeState(context.WithoutCancel(ctx), request.WorkflowRunID, request.Node.Key)
	if readErr == nil && !state.failed && !state.completed {
		message := "The workflow node could not be completed. Please retry."
		if errors.Is(cause, context.Canceled) || errors.Is(cause, context.DeadlineExceeded) {
			message = "The workflow node was cancelled."
		}
		_, _ = s.persistWorkflowEvent(ctx, request.WorkflowRunID, domain.EventWorkflowNodeFailed, map[string]string{
			"node_key": request.Node.Key, "child_run_id": string(childRunID),
			"cause_category": causeCategoryOf(cause), "message": message,
		})
	}
	return cause
}

func (s *Service) readWorkflowNodeState(ctx context.Context, workflowRunID domain.RunID, nodeKey string) (workflowNodeState, error) {
	state := workflowNodeState{}
	iterator, err := s.deps.Journal.Replay(ctx, workflowRunID, 0)
	if err != nil {
		return state, err
	}
	defer func() { _ = iterator.Close() }()
	for iterator.Next() {
		event := iterator.Value().Event
		if event.Type != domain.EventWorkflowNodeStarted && event.Type != domain.EventWorkflowNodeCompleted && event.Type != domain.EventWorkflowNodeFailed {
			continue
		}
		var payload struct {
			NodeKey       string `json:"node_key"`
			ChildRunID    string `json:"child_run_id"`
			ResultDigest  string `json:"result_digest"`
			CauseCategory string `json:"cause_category"`
			Message       string `json:"message"`
		}
		if err := json.Unmarshal(event.Payload, &payload); err != nil {
			return state, err
		}
		if payload.NodeKey != nodeKey {
			continue
		}
		expectedChildID := string(workflowNodeRunID(workflowRunID, nodeKey))
		if payload.ChildRunID != "" && payload.ChildRunID != expectedChildID {
			return state, storage.ErrWorkflowRevisionConflict
		}
		if state.childRunID != "" && payload.ChildRunID != "" && state.childRunID != payload.ChildRunID {
			return state, storage.ErrWorkflowRevisionConflict
		}
		if payload.ChildRunID != "" {
			state.childRunID = payload.ChildRunID
		}
		switch event.Type {
		case domain.EventWorkflowNodeStarted:
			state.started = true
		case domain.EventWorkflowNodeCompleted:
			if state.failed || len(payload.ResultDigest) != 64 {
				return state, storage.ErrWorkflowRevisionConflict
			}
			if _, err := hex.DecodeString(payload.ResultDigest); err != nil {
				return state, storage.ErrWorkflowRevisionConflict
			}
			state.completed = true
			state.resultDigest = payload.ResultDigest
		case domain.EventWorkflowNodeFailed:
			if state.completed {
				return state, storage.ErrWorkflowRevisionConflict
			}
			state.failed = true
			state.errorCategory, state.message = payload.CauseCategory, payload.Message
		}
	}
	if err := iterator.Err(); err != nil {
		return state, err
	}
	return state, nil
}

type workflowNodeState struct {
	started       bool
	completed     bool
	failed        bool
	childRunID    string
	resultDigest  string
	errorCategory string
	message       string
}

func (s *Service) persistWorkflowEvent(ctx context.Context, runID domain.RunID, typ domain.EventType, payload any) (domain.RunEvent, error) {
	s.mu.Lock()
	ledger := s.ledgers[runID]
	s.mu.Unlock()
	if ledger == nil {
		return domain.RunEvent{}, errors.New("runtime: workflow budget authority is unavailable")
	}
	if err := ledger.ReserveEvent(); err != nil {
		return domain.RunEvent{}, err
	}
	persistCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), terminalPersistTimeout)
	defer cancel()
	return s.RecordExternalRunEvent(persistCtx, runID, typ, payload)
}

func (s *Service) workflowStartEventMatches(ctx context.Context, runID domain.RunID, revisionDigest string, nodeCount int) (bool, error) {
	iterator, err := s.deps.Journal.Replay(ctx, runID, 0)
	if err != nil {
		return false, err
	}
	defer func() { _ = iterator.Close() }()
	seen := false
	for iterator.Next() {
		event := iterator.Value().Event
		if event.Type != domain.EventWorkflowStarted {
			continue
		}
		if seen {
			return false, storage.ErrWorkflowRevisionConflict
		}
		var payload struct {
			RevisionDigest string `json:"revision_digest"`
			NodeCount      int    `json:"node_count"`
		}
		if err := json.Unmarshal(event.Payload, &payload); err != nil || payload.RevisionDigest != revisionDigest || payload.NodeCount != nodeCount {
			return false, storage.ErrWorkflowRevisionConflict
		}
		seen = true
	}
	if err := iterator.Err(); err != nil {
		return false, err
	}
	return seen, nil
}

type persistedWorkflowAuthority struct {
	PolicyProfile  domain.PolicyProfile  `json:"policy_profile"`
	PolicyHash     string                `json:"policy_hash"`
	SandboxMode    domain.SandboxMode    `json:"sandbox_mode"`
	ApprovalPolicy domain.ApprovalPolicy `json:"approval_policy"`
	ToolNames      []string              `json:"tool_names"`
}

func (s *Service) recoverWorkflowRun(ctx context.Context, run domain.Run, _ string) error {
	if s == nil || run.Kind != domain.RunKindWorkflow || run.Status.Terminal() || s.deps.WorkflowRevisions == nil ||
		s.engine == nil || s.engine.cfg.Checkpoints == nil || s.deps.Sessions == nil {
		return errors.New("runtime: workflow recovery dependencies are incomplete")
	}
	s.mu.Lock()
	_, active := s.active[run.ID]
	_, pending := s.pending[run.ID]
	s.mu.Unlock()
	if active || pending {
		return nil
	}
	revision, err := s.deps.WorkflowRevisions.GetWorkflowRevision(ctx, run.ID)
	if err != nil {
		return fmt.Errorf("runtime: load workflow revision for recovery: %w", err)
	}
	if revision.RunID != run.ID || revision.ParentRunID != run.ParentID || revision.ParentSessionID != run.SessionID ||
		revision.RootRunID != run.RootID || revision.CreatedAt != run.CreatedAt {
		return storage.ErrWorkflowRevisionConflict
	}
	parent, err := s.deps.Runs.GetRun(ctx, revision.ParentRunID)
	if err != nil {
		return fmt.Errorf("runtime: load workflow authorizer lineage: %w", err)
	}
	rootID := parent.RootID
	if rootID == "" {
		rootID = parent.ID
	}
	if parent.SessionID != run.SessionID || rootID != run.RootID || parent.Depth+1 != run.Depth {
		return storage.ErrWorkflowRevisionConflict
	}
	authority, err := decodeWorkflowAuthority(revision)
	if err != nil {
		return err
	}
	canonicalTools := authority.ToolNames
	snapshot, err := s.engine.cfg.Policy.Snapshot(authority.PolicyProfile)
	if err != nil || snapshot.Hash != authority.PolicyHash {
		return errors.New("runtime: workflow policy snapshot is no longer compatible")
	}
	session, err := s.deps.Sessions.GetSession(ctx, run.SessionID)
	if err != nil {
		return fmt.Errorf("runtime: load workflow Session during recovery: %w", err)
	}
	sandboxMode, approvalPolicy := session.EffectiveSandbox()
	if sandboxMode != authority.SandboxMode || approvalPolicy != authority.ApprovalPolicy {
		return errors.New("runtime: workflow Session authority changed before recovery")
	}
	allowedTools := s.workflowChildTools(canonicalTools)
	if !sameStrings(allowedTools, canonicalTools) {
		return errors.New("runtime: workflow tools no longer match the read-only authority ceiling")
	}
	validated, err := orchestration.DecodeValidated(revision.DescriptorJSON, revision.DescriptorDigest, allowedTools)
	if err != nil {
		return fmt.Errorf("runtime: stored workflow descriptor failed validation: %w", err)
	}
	ledgers, err := s.recoverBudgetLedgers(ctx, run.ID)
	if err != nil {
		return fmt.Errorf("runtime: restore workflow run-tree budget: %w", err)
	}
	current, err := s.deps.Runs.GetRun(ctx, run.ID)
	if err != nil {
		return err
	}
	if current.Status.Terminal() {
		return nil
	}
	s.mu.Lock()
	for id, ledger := range ledgers {
		s.ledgers[id] = ledger
		if p, ok := s.pending[id]; ok {
			p.ledger = ledger
			s.pending[id] = p
		}
	}
	s.snapshots[run.ID] = snapshot
	s.runTools[run.ID] = childToolSet(allowedTools)
	s.runSessions[run.ID] = run.SessionID
	s.mu.Unlock()
	return s.launchWorkflow(ctx, WorkflowStartResult{Run: current, Revision: revision}, validated, sandboxMode, approvalPolicy, false)
}

func workflowAuthorityRecord(snapshot domain.PolicySnapshot, sandbox domain.SandboxMode, approval domain.ApprovalPolicy, tools []string) ([]byte, string, error) {
	canonical, err := domain.CanonicalToolNames(tools)
	if err != nil {
		return nil, "", err
	}
	encoded, err := json.Marshal(persistedWorkflowAuthority{
		PolicyProfile: snapshot.Profile, PolicyHash: snapshot.Hash, SandboxMode: sandbox,
		ApprovalPolicy: approval, ToolNames: canonical,
	})
	if err != nil {
		return nil, "", err
	}
	sum := sha256.Sum256(encoded)
	return encoded, hex.EncodeToString(sum[:]), nil
}

func decodeWorkflowAuthority(revision domain.WorkflowRevision) (persistedWorkflowAuthority, error) {
	var authority persistedWorkflowAuthority
	decoder := json.NewDecoder(bytes.NewReader(revision.AuthorityJSON))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&authority); err != nil {
		return authority, errors.New("runtime: stored workflow authority is invalid")
	}
	if err := decoder.Decode(new(any)); !errors.Is(err, io.EOF) {
		return authority, errors.New("runtime: stored workflow authority has trailing data")
	}
	canonicalTools, err := domain.CanonicalToolNames(authority.ToolNames)
	if err != nil || authority.PolicyHash == "" || !authority.PolicyProfile.Valid() || !sameStrings(canonicalTools, authority.ToolNames) {
		return authority, errors.New("runtime: stored workflow authority is incomplete")
	}
	canonicalAuthority, authorityDigest, err := workflowAuthorityRecord(
		domain.PolicySnapshot{Profile: authority.PolicyProfile, Hash: authority.PolicyHash},
		authority.SandboxMode, authority.ApprovalPolicy, canonicalTools,
	)
	if err != nil || authorityDigest != revision.AuthorityDigest || !bytes.Equal(canonicalAuthority, revision.AuthorityJSON) {
		return authority, errors.New("runtime: stored workflow authority digest does not match")
	}
	return authority, nil
}

func validateStoredWorkflowDescriptor(revision domain.WorkflowRevision, descriptor orchestration.Descriptor) (orchestration.ValidatedDescriptor, error) {
	authority, err := decodeWorkflowAuthority(revision)
	if err != nil {
		return orchestration.ValidatedDescriptor{}, storage.ErrWorkflowRevisionConflict
	}
	validated, err := orchestration.Validate(descriptor, authority.ToolNames)
	if err != nil || validated.Digest != revision.DescriptorDigest || !bytes.Equal(validated.CanonicalJSON, revision.DescriptorJSON) {
		return orchestration.ValidatedDescriptor{}, storage.ErrWorkflowRevisionConflict
	}
	return validated, nil
}

// GetWorkflow returns the durable descriptor identity and bounded node
// lifecycle projection for one workflow Run.
func (s *Service) GetWorkflow(ctx context.Context, runID domain.RunID) (WorkflowDetails, error) {
	if s == nil || s.deps.WorkflowRevisions == nil || s.deps.Runs == nil || s.deps.Journal == nil {
		return WorkflowDetails{}, errors.New("runtime: workflow inspection is not wired")
	}
	revision, err := s.deps.WorkflowRevisions.GetWorkflowRevision(ctx, runID)
	if err != nil {
		return WorkflowDetails{}, err
	}
	run, err := s.deps.Runs.GetRun(ctx, runID)
	if err != nil {
		return WorkflowDetails{}, err
	}
	var descriptor orchestration.Descriptor
	if err := json.Unmarshal(revision.DescriptorJSON, &descriptor); err != nil {
		return WorkflowDetails{}, fmt.Errorf("runtime: decode stored workflow revision: %w", err)
	}
	validated, err := validateStoredWorkflowDescriptor(revision, descriptor)
	if err != nil {
		return WorkflowDetails{}, fmt.Errorf("runtime: verify stored workflow revision: %w", err)
	}
	result := WorkflowDetails{Run: run, RevisionDigest: revision.DescriptorDigest, Descriptor: validated.Descriptor}
	states := make(map[string]workflowNodeState, len(validated.Nodes))
	for _, node := range validated.Nodes {
		state, stateErr := s.readWorkflowNodeState(ctx, runID, node.Key)
		if stateErr != nil {
			return WorkflowDetails{}, stateErr
		}
		status := "waiting"
		if state.started {
			status = "running"
		}
		if state.completed {
			status = "completed"
		}
		if state.failed {
			status = "failed"
			if state.errorCategory == causeCancelled {
				status = "cancelled"
			}
		}
		if !state.started && run.Status == domain.RunCancelled {
			status = "cancelled"
		}
		if !state.started && run.Status == domain.RunFailed {
			status = "blocked"
		}
		result.Nodes = append(result.Nodes, WorkflowNodeProjection{
			Key: node.Key, Status: status, ChildRunID: state.childRunID, ResultDigest: state.resultDigest,
			ErrorCategory: state.errorCategory, Message: state.message,
		})
		states[node.Key] = state
	}
	if run.Status == domain.RunCompleted {
		result.Outputs = make(map[string]string, len(validated.Outputs))
		for _, key := range validated.Outputs {
			state := states[key]
			if !state.completed || state.childRunID == "" || state.resultDigest == "" {
				return WorkflowDetails{}, ErrWorkflowNodeUnknownOutcome
			}
			childRunID := domain.RunID(state.childRunID)
			childRun, childErr := s.deps.Runs.GetRun(ctx, childRunID)
			if childErr != nil {
				return WorkflowDetails{}, childErr
			}
			if childRun.Status != domain.RunCompleted {
				return WorkflowDetails{}, ErrWorkflowNodeUnknownOutcome
			}
			value, message, _, childErr := s.ChildRunDetails(ctx, childRunID)
			if childErr != nil {
				return WorkflowDetails{}, childErr
			}
			if message != "" {
				return WorkflowDetails{}, errors.New("runtime: completed workflow output child contains a failure event")
			}
			digest := sha256.Sum256([]byte(value))
			if hex.EncodeToString(digest[:]) != state.resultDigest {
				return WorkflowDetails{}, storage.ErrWorkflowRevisionConflict
			}
			result.Outputs[key] = value
		}
		encoded, encodeErr := json.Marshal(result.Outputs)
		if encodeErr != nil {
			return WorkflowDetails{}, encodeErr
		}
		if len(encoded) > orchestration.MaxOutputBytes {
			return WorkflowDetails{}, fmt.Errorf("runtime: workflow outputs exceed %d bytes", orchestration.MaxOutputBytes)
		}
	}
	return result, nil
}

func (s *Service) ListWorkflows(ctx context.Context, parentRunID domain.RunID) ([]WorkflowDetails, error) {
	if s == nil || s.deps.WorkflowRevisions == nil {
		return nil, errors.New("runtime: workflow listing is not wired")
	}
	revisions, err := s.deps.WorkflowRevisions.ListWorkflowRevisions(ctx, parentRunID)
	if err != nil {
		return nil, err
	}
	if len(revisions) > 100 {
		revisions = revisions[len(revisions)-100:]
	}
	result := make([]WorkflowDetails, 0, len(revisions))
	for _, revision := range revisions {
		details, err := s.GetWorkflow(ctx, revision.RunID)
		if err != nil {
			return nil, err
		}
		result = append(result, details)
	}
	return result, nil
}

func (s *Service) CancelWorkflow(ctx context.Context, runID domain.RunID) (domain.Run, error) {
	if s == nil || s.deps.WorkflowRevisions == nil || s.deps.Runs == nil {
		return domain.Run{}, errors.New("runtime: workflow cancellation is not wired")
	}
	if _, err := s.deps.WorkflowRevisions.GetWorkflowRevision(ctx, runID); err != nil {
		return domain.Run{}, err
	}
	if !s.Cancel(runID) {
		run, err := s.deps.Runs.GetRun(ctx, runID)
		if err != nil {
			return domain.Run{}, err
		}
		if !run.Status.Terminal() {
			return domain.Run{}, ErrWorkflowRecoveryRequired
		}
		return run, nil
	}
	return s.deps.Runs.GetRun(ctx, runID)
}
