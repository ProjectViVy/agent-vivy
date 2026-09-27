package runtime

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
	"agent-vivy/internal/tools"
)

const (
	maxOneShotChildResultBytes = 32 << 10
)

type OneShotChildRequest struct {
	ParentRunID domain.RunID
	// RunID is reserved for host-owned DAG nodes that need stable identity
	// across Eino checkpoint retries. Ordinary callers leave it empty.
	RunID         domain.RunID
	Task          string
	PolicyProfile domain.PolicyProfile
	ToolNames     []string
}

type OneShotChildResult struct {
	Run         domain.Run
	WorkspaceID string
}

// StartOneShotChild admits and starts a non-addressable child task. It uses a
// private workspace, clean task context, parent-bounded tools and the shared
// Service/Eino execution path. Empty ToolNames means read-only tools only.
func (s *Service) StartOneShotChild(ctx context.Context, request OneShotChildRequest) (OneShotChildResult, error) {
	if s == nil {
		return OneShotChildResult{}, errors.New("runtime: service is not wired")
	}
	if request.RunID != "" {
		if !validWorkflowChildRunID(request.RunID) {
			return OneShotChildResult{}, errors.New("runtime: stable child Run id is invalid")
		}
		s.workflowNodeMu.Lock()
		defer s.workflowNodeMu.Unlock()
	}
	if s == nil || s.engine == nil || s.deps.Runs == nil || s.deps.Journal == nil ||
		s.deps.Sink == nil || s.deps.Workspaces == nil || s.deps.Sessions == nil {
		return OneShotChildResult{}, errors.New("runtime: native one-shot child execution is not wired")
	}
	if request.ParentRunID == "" || strings.TrimSpace(request.Task) == "" || len(request.Task) > maxChildTaskBytes {
		return OneShotChildResult{}, errors.New("runtime: child parent run and bounded task are required")
	}

	s.projectionMu.Lock()
	parent, snapshot, parentLedger, parentTools, err := s.currentChildAuthorizer(ctx, request.ParentRunID)
	if err != nil {
		s.projectionMu.Unlock()
		return OneShotChildResult{}, err
	}
	if s.sessionDeleted(parent.SessionID) {
		s.projectionMu.Unlock()
		return OneShotChildResult{}, storage.ErrNotFound
	}
	if parent.Depth >= maxChildSessionDepth {
		s.projectionMu.Unlock()
		return OneShotChildResult{}, fmt.Errorf("runtime: child depth exceeds %d", maxChildSessionDepth)
	}
	if request.PolicyProfile != "" && request.PolicyProfile != snapshot.Profile {
		s.projectionMu.Unlock()
		return OneShotChildResult{}, errors.New("runtime: child policy cannot widen parent authority")
	}
	parentSession, err := s.deps.Sessions.GetSession(ctx, parent.SessionID)
	if err != nil {
		s.projectionMu.Unlock()
		return OneShotChildResult{}, err
	}
	sandboxMode, approvalPolicy := parentSession.EffectiveSandbox()
	selectedTools := s.oneShotChildTools(parentTools)
	if len(request.ToolNames) > 0 {
		selectedTools, err = selectChildTools(request.ToolNames, parentTools)
		if err != nil {
			s.projectionMu.Unlock()
			return OneShotChildResult{}, err
		}
	}
	for _, name := range selectedTools {
		if name == tools.AgentName || name == tools.WorkflowName || name == tools.ReplyParentName || name == tools.AskUserName || strings.HasPrefix(name, "mcp_") {
			s.projectionMu.Unlock()
			return OneShotChildResult{}, fmt.Errorf("runtime: tool %q is not available to one-shot children", name)
		}
	}
	childEngine, err := s.engine.OneShotView(ctx, selectedTools)
	if err != nil {
		s.projectionMu.Unlock()
		return OneShotChildResult{}, err
	}
	children, err := s.deps.Runs.ListChildRuns(ctx, parent.ID)
	if err != nil {
		s.projectionMu.Unlock()
		return OneShotChildResult{}, err
	}
	childID := request.RunID
	var existing *domain.Run
	requestedEventFound, startedEventFound := false, false
	if childID != "" {
		prior, getErr := s.deps.Runs.GetRun(ctx, childID)
		if getErr == nil {
			if prior.Kind != domain.RunKindChild || prior.EffectiveChildMode() != domain.ChildModeOneShot ||
				prior.ParentID != parent.ID || prior.SessionID != parent.SessionID || prior.RootID != parent.RootID && prior.RootID != parent.ID ||
				prior.Depth != parent.Depth+1 {
				s.projectionMu.Unlock()
				return OneShotChildResult{}, storage.ErrChildAdmissionConflict
			}
			requestedEventFound, startedEventFound, err = s.verifyOneShotChildRequest(ctx, prior.ID, parent.ID, request.Task)
			if err != nil {
				s.projectionMu.Unlock()
				return OneShotChildResult{}, err
			}
			if prior.Status.Terminal() {
				workspace, workspaceErr := s.deps.Workspaces.Ensure(withSessionID(ctx, parent.SessionID), prior.ID)
				s.projectionMu.Unlock()
				if workspaceErr != nil {
					return OneShotChildResult{}, workspaceErr
				}
				return OneShotChildResult{Run: prior, WorkspaceID: workspace.ID}, nil
			}
			if startedEventFound {
				s.mu.Lock()
				_, active := s.active[prior.ID]
				_, pending := s.pending[prior.ID]
				_, shellPending := s.shellPending[prior.ID]
				s.mu.Unlock()
				if !active && !pending && !shellPending {
					s.projectionMu.Unlock()
					return OneShotChildResult{}, ErrWorkflowNodeUnknownOutcome
				}
				workspace, workspaceErr := s.deps.Workspaces.Ensure(withSessionID(ctx, parent.SessionID), prior.ID)
				s.projectionMu.Unlock()
				if workspaceErr != nil {
					return OneShotChildResult{}, workspaceErr
				}
				return OneShotChildResult{Run: prior, WorkspaceID: workspace.ID}, nil
			}
			existing = &prior
		} else if !errors.Is(getErr, storage.ErrNotFound) {
			s.projectionMu.Unlock()
			return OneShotChildResult{}, getErr
		}
	}
	activeChildren := 0
	for _, child := range children {
		if child.Status == domain.RunAccepted || child.Status == domain.RunQueued || child.Status == domain.RunActive {
			activeChildren++
		}
	}
	if existing == nil && activeChildren >= storage.MaxActiveChildrenPerRun {
		s.projectionMu.Unlock()
		return OneShotChildResult{}, storage.ErrChildConcurrencyLimit
	}
	if existing == nil || !requestedEventFound {
		if err := parentLedger.ReserveEvent(); err != nil {
			s.projectionMu.Unlock()
			return OneShotChildResult{}, err
		}
	}
	if childID == "" {
		childID = domain.RunID(newPrefixedID("child_"))
	}
	rootID := parent.RootID
	if rootID == "" {
		rootID = parent.ID
	}
	child := domain.Run{
		ID: childID, SessionID: parent.SessionID, Status: domain.RunAccepted, CreatedAt: time.Now().UnixMilli(),
		Kind: domain.RunKindChild, ChildMode: domain.ChildModeOneShot,
		ParentID: parent.ID, RootID: rootID, Depth: parent.Depth + 1,
	}
	if existing != nil {
		child = *existing
	} else if s.deps.Admission == nil {
		if err := s.deps.Runs.CreateRun(ctx, child); err != nil {
			s.projectionMu.Unlock()
			return OneShotChildResult{}, fmt.Errorf("runtime: create one-shot child run: %w", err)
		}
	}
	s.projectionMu.Unlock()

	mapper := newEventMapper(child.ID, childEngine.cfg.MaxEventPayloadBytes)
	var childLedger *BudgetLedger
	workspaceID := ""
	if s.deps.Admission != nil {
		workspace, workspaceErr := s.deps.Workspaces.Ensure(withSessionID(ctx, parent.SessionID), child.ID)
		if workspaceErr != nil {
			return OneShotChildResult{}, workspaceErr
		}
		workspaceID = workspace.ID
		childLedger, err = parentLedger.Child(s.deps.Budget)
		if err != nil {
			return OneShotChildResult{}, err
		}
		if existing == nil {
			admission, admissionErr := s.buildChildRunAdmission(parentSession, child, request.Task, snapshot, workspaceID)
			if admissionErr != nil {
				return OneShotChildResult{}, admissionErr
			}
			admission.Message = domain.Message{}
			admission.OmitMessage = true
			started, admissionErr := s.deps.Admission.CommitRunAdmission(ctx, admission)
			if admissionErr != nil {
				return OneShotChildResult{}, fmt.Errorf("runtime: commit one-shot child admission: %w", admissionErr)
			}
			s.publish(ctx, started)
		} else if _, hasPrompt, promptErr := s.promptSnapshotForRun(ctx, child.ID); promptErr != nil || !hasPrompt {
			if promptErr == nil {
				promptErr = errors.New("runtime: existing one-shot child has no admitted prompt snapshot")
			}
			s.failAdmittedChildRun(ctx, child, promptErr)
			return OneShotChildResult{}, promptErr
		}
	}
	if !requestedEventFound {
		if _, err := s.RecordExternalRunEvent(ctx, child.ID, domain.EventChildRequested, map[string]any{
			"parent_run_id": string(parent.ID), "depth": child.Depth, "text": request.Task,
		}); err != nil {
			s.failAdmittedChildRun(ctx, child, err)
			return OneShotChildResult{}, err
		}
	}
	childSnapshot := snapshot
	if childLedger == nil {
		var resolvedWorkspaceID string
		childSnapshot, childLedger, resolvedWorkspaceID, err = s.WorkerChildAuthority(ctx, parent.ID, child.ID)
		if err != nil {
			s.failAdmittedChildRun(ctx, child, err)
			return OneShotChildResult{}, err
		}
		workspaceID = resolvedWorkspaceID
	}
	if childSnapshot.Hash != snapshot.Hash || childSnapshot.Profile != snapshot.Profile {
		err = errors.New("runtime: child authority changed during admission")
		s.failAdmittedChildRun(ctx, child, err)
		return OneShotChildResult{}, err
	}
	if err := childLedger.ReserveEvent(); err != nil {
		s.failAdmittedChildRun(ctx, child, err)
		return OneShotChildResult{}, err
	}
	if err := s.RegisterWorkerAuthority(child.ID, snapshot, childLedger); err != nil {
		s.failAdmittedChildRun(ctx, child, err)
		return OneShotChildResult{}, err
	}
	if s.deps.Admission == nil {
		if err := s.deps.Runs.SetRunStatus(ctx, child.ID, domain.RunActive); err != nil {
			s.UnregisterWorkerAuthority(child.ID)
			s.failAdmittedChildRun(ctx, child, err)
			return OneShotChildResult{}, err
		}
	}
	if !startedEventFound {
		if _, err := s.RecordExternalRunEvent(ctx, child.ID, domain.EventChildStarted, map[string]any{
			"parent_run_id": string(parent.ID), "workspace_id": workspaceID,
		}); err != nil {
			s.UnregisterWorkerAuthority(child.ID)
			s.failAdmittedChildRun(ctx, child, err)
			return OneShotChildResult{}, err
		}
	}
	mapper.setRunScope(s.deps.TenantID, workspaceID, string(parent.SessionID))
	providerName, modelID := s.CurrentModel()
	mapper.setUsageRoutes(providerName, modelID, childEngine.cfg.SummaryModelID)
	state := &childActivationDrive{task: request.Task}
	runCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
	err = s.RegisterWorkerProcess(ctx, child.ID, func() {
		s.mu.Lock()
		s.active[child.ID] = cancel
		s.runSessions[child.ID] = parent.SessionID
		s.ledgers[child.ID] = childLedger
		s.snapshots[child.ID] = snapshot
		s.runTools[child.ID] = childToolSet(selectedTools)
		s.mu.Unlock()
	})
	if err != nil {
		cancel()
		s.UnregisterWorkerAuthority(child.ID)
		s.failAdmittedChildRun(ctx, child, err)
		return OneShotChildResult{}, err
	}
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.driveWithExecution(runCtx, mapper, parent.SessionID, request.Task, domain.RunModeNormal,
			snapshot.Profile, "", snapshot, sandboxMode, approvalPolicy, domain.FaceWeb, workspaceID, nil,
			runExecutionOptions{engine: childEngine, child: state, oneShotChild: true, suppressSessionMessageProjection: true})
	}()
	started, err := s.deps.Runs.GetRun(context.WithoutCancel(ctx), child.ID)
	if err != nil {
		return OneShotChildResult{}, err
	}
	return OneShotChildResult{Run: started, WorkspaceID: workspaceID}, nil
}

func validWorkflowChildRunID(runID domain.RunID) bool {
	value := strings.TrimPrefix(string(runID), "workflow_child_")
	if value == string(runID) || len(value) != 64 {
		return false
	}
	decoded, err := hex.DecodeString(value)
	return err == nil && len(decoded) == 32
}

func (s *Service) verifyOneShotChildRequest(ctx context.Context, childRunID, parentRunID domain.RunID, task string) (requested, started bool, err error) {
	iterator, err := s.deps.Journal.Replay(ctx, childRunID, 0)
	if err != nil {
		return false, false, err
	}
	defer func() { _ = iterator.Close() }()
	for iterator.Next() {
		event := iterator.Value().Event
		switch event.Type {
		case domain.EventChildRequested:
			var payload struct {
				ParentRunID string `json:"parent_run_id"`
				Depth       int    `json:"depth"`
				Text        string `json:"text"`
			}
			if err := json.Unmarshal(event.Payload, &payload); err != nil {
				return false, false, err
			}
			if payload.ParentRunID != string(parentRunID) || payload.Text != task {
				return false, false, storage.ErrChildAdmissionConflict
			}
			requested = true
		case domain.EventChildStarted:
			started = true
		}
	}
	if err := iterator.Err(); err != nil {
		return false, false, err
	}
	return requested, started, nil
}

func (s *Service) failAdmittedChildRun(ctx context.Context, run domain.Run, cause error) {
	mapper := newEventMapper(run.ID, s.engine.cfg.MaxEventPayloadBytes)
	mapper.setRunScope(s.deps.TenantID, "", string(run.SessionID))
	providerName, modelID := s.CurrentModel()
	mapper.setUsageRoutes(providerName, modelID, s.engine.cfg.SummaryModelID)
	childCtx := withChildTerminal(ctx)
	s.emitTerminal(childCtx, mapper, s.terminalEvent(childCtx, mapper, cause))
}

// OneShotChildOutcome reads the terminal result from the child run Journal.
// Empty values indicate that the child has not reached a terminal.
func (s *Service) OneShotChildOutcome(ctx context.Context, runID domain.RunID) (summary, message string, err error) {
	summary, message, _, err = s.ChildRunDetails(ctx, runID)
	return summary, message, err
}

// ChildRunDetails reads the projected result and workspace from durable child
// Journal events. It remains useful after the child execution process state is
// gone; empty result/message indicate a non-terminal child.
func (s *Service) ChildRunDetails(ctx context.Context, runID domain.RunID) (summary, message, workspaceID string, err error) {
	if s == nil || s.deps.Journal == nil || runID == "" {
		return "", "", "", errors.New("runtime: child outcome is not wired")
	}
	iterator, err := s.deps.Journal.Replay(ctx, runID, 0)
	if err != nil {
		return "", "", "", err
	}
	defer func() { _ = iterator.Close() }()
	for iterator.Next() {
		event := iterator.Value().Event
		switch event.Type {
		case domain.EventChildStarted:
			var payload map[string]string
			if err := json.Unmarshal(event.Payload, &payload); err != nil {
				return "", "", "", err
			}
			workspaceID = payload["workspace_id"]
		case domain.EventChildCompleted:
			var payload map[string]string
			if err := json.Unmarshal(event.Payload, &payload); err != nil {
				return "", "", "", err
			}
			summary = payload["summary"]
		case domain.EventChildFailed:
			var payload map[string]string
			if err := json.Unmarshal(event.Payload, &payload); err != nil {
				return "", "", "", err
			}
			message = payload["message"]
		case domain.EventChildCancelled:
			message = "child task was cancelled"
		}
	}
	if err := iterator.Err(); err != nil {
		return "", "", "", err
	}
	return summary, message, workspaceID, nil
}

func boundedChildSummary(summary string) string {
	if len(summary) <= maxOneShotChildResultBytes {
		return summary
	}
	suffix := "\n[child result truncated]"
	end := maxOneShotChildResultBytes - len(suffix)
	if end < 0 {
		end = 0
	}
	return strings.ToValidUTF8(summary[:end], "") + suffix
}
