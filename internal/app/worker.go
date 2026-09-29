package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"agent-vivy/internal/domain"
	controlrpc "agent-vivy/internal/rpc"
	"agent-vivy/internal/runtime"
	"agent-vivy/internal/storage"
	"agent-vivy/internal/tools"
)

// workerManager adapts the public child RPC to the native Service child-run
// lifecycle. It deliberately owns no model, tools, policy, or approval loop.
type workerManager struct {
	service *runtime.Service
	runs    storage.RunStore
}

func newWorkerManager(service *runtime.Service, runs storage.RunStore) *workerManager {
	return &workerManager{service: service, runs: runs}
}

// RunWorkflow blocks the authoring tool until its durable workflow Run reaches
// a terminal state, then returns only the Definition's bounded outputs.
func (m *workerManager) RunWorkflow(ctx context.Context, parentRunID domain.RunID, operationKey string, definition json.RawMessage) (tools.WorkflowTaskResult, error) {
	if m == nil || m.service == nil {
		return tools.WorkflowTaskResult{}, errors.New("workflow controller is not wired")
	}
	started, err := m.service.StartINOFYWorkflow(ctx, parentRunID, operationKey, definition)
	if err != nil {
		return tools.WorkflowTaskResult{}, err
	}
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	for {
		details, err := m.service.GetWorkflow(ctx, started.Run.ID)
		if err != nil {
			return tools.WorkflowTaskResult{}, err
		}
		switch details.Run.Status {
		case domain.RunCompleted:
			return tools.WorkflowTaskResult{WorkflowRunID: string(details.Run.ID), Outputs: details.Outputs}, nil
		case domain.RunFailed, domain.RunCancelled:
			for _, node := range details.Nodes {
				if node.Message != "" {
					return tools.WorkflowTaskResult{}, fmt.Errorf("workflow stopped: %s", node.Message)
				}
			}
			if details.Run.Status == domain.RunCancelled {
				return tools.WorkflowTaskResult{}, errors.New("workflow was cancelled")
			}
			return tools.WorkflowTaskResult{}, errors.New("workflow failed before producing its declared outputs")
		}
		select {
		case <-ctx.Done():
			cancelCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
			defer cancel()
			_, _ = m.service.CancelWorkflow(cancelCtx, started.Run.ID)
			return tools.WorkflowTaskResult{}, ctx.Err()
		case <-ticker.C:
		}
	}
}

func (m *workerManager) StartChild(ctx context.Context, request controlrpc.ChildRequest) (controlrpc.ChildResult, error) {
	if m == nil || m.service == nil || m.runs == nil {
		return controlrpc.ChildResult{}, errors.New("child controller is not wired")
	}
	mode := domain.ChildMode(request.Mode)
	if mode == "" {
		mode = domain.ChildModeOneShot
	}
	switch mode {
	case domain.ChildModeOneShot:
		started, err := m.service.StartOneShotChild(ctx, runtime.OneShotChildRequest{
			ParentRunID:   domain.RunID(request.ParentRunID),
			Task:          request.Text,
			PolicyProfile: domain.PolicyProfile(request.PolicyProfile),
			ToolNames:     request.ToolNames,
		})
		if err != nil {
			return controlrpc.ChildResult{}, err
		}
		return m.childResult(ctx, started.Run, started.WorkspaceID)
	case domain.ChildModeContinuable:
		if request.OperationID == "" {
			return controlrpc.ChildResult{}, errors.New("continuable child requires operation_id")
		}
		admitted, err := m.service.AdmitChildSession(ctx, runtime.ChildSessionRequest{
			AuthorizerRunID: domain.RunID(request.ParentRunID), OperationKey: request.OperationID,
			Task: request.Text, ToolNames: request.ToolNames,
		})
		if err != nil {
			return controlrpc.ChildResult{}, err
		}
		if admitted.Run.Status == domain.RunAccepted || admitted.Run.Status == domain.RunActive {
			if err := m.service.StartChildActivation(ctx, admitted.Binding.ChildSessionID, admitted.Run.ID); err != nil {
				return controlrpc.ChildResult{}, err
			}
		}
		return m.childResult(ctx, admitted.Run, admitted.WorkspaceID)
	default:
		return controlrpc.ChildResult{}, errors.New("child mode must be one-shot or continuable")
	}
}

func (m *workerManager) FollowupChild(ctx context.Context, request controlrpc.ChildFollowupRequest) (controlrpc.ChildResult, error) {
	if m == nil || m.service == nil || m.runs == nil {
		return controlrpc.ChildResult{}, errors.New("child controller is not wired")
	}
	admitted, err := m.service.AdmitChildSessionActivation(ctx, runtime.ChildSessionContinuationRequest{
		ChildSessionID: domain.SessionID(request.ChildSessionID), AuthorizerRunID: domain.RunID(request.ParentRunID),
		OperationKey: request.OperationID, Task: request.Text, ToolNames: request.ToolNames,
	})
	if err != nil {
		return controlrpc.ChildResult{}, err
	}
	if admitted.Run.Status == domain.RunAccepted || admitted.Run.Status == domain.RunActive {
		if err := m.service.StartChildActivation(ctx, admitted.Binding.ChildSessionID, admitted.Run.ID); err != nil {
			return controlrpc.ChildResult{}, err
		}
	}
	return m.childResult(ctx, admitted.Run, admitted.WorkspaceID)
}

func (m *workerManager) InterruptChild(ctx context.Context, runID string) (controlrpc.ChildResult, error) {
	if m == nil || m.service == nil || m.runs == nil || runID == "" {
		return controlrpc.ChildResult{}, errors.New("child controller is not wired or run id is empty")
	}
	if err := m.service.InterruptChildActivation(ctx, domain.RunID(runID)); err != nil {
		return controlrpc.ChildResult{}, err
	}
	run, err := m.runs.GetRun(ctx, domain.RunID(runID))
	if err != nil {
		return controlrpc.ChildResult{}, err
	}
	return m.childResult(ctx, run, "")
}

func (m *workerManager) ChildHistory(ctx context.Context, childSessionID, parentRunID string) ([]controlrpc.ChildHistoryMessage, error) {
	if m == nil || m.service == nil {
		return nil, errors.New("child controller is not wired")
	}
	messages, err := m.service.ChildSessionHistory(ctx, domain.SessionID(childSessionID), domain.RunID(parentRunID))
	if err != nil {
		return nil, err
	}
	result := make([]controlrpc.ChildHistoryMessage, 0, len(messages))
	for _, message := range messages {
		result = append(result, controlrpc.ChildHistoryMessage{
			ID: message.ID, RunID: string(message.RunID), Role: string(message.Role),
			Content: message.Content, CreatedAt: message.CreatedAt,
		})
	}
	return result, nil
}

func (m *workerManager) SendChildMessage(ctx context.Context, request controlrpc.ChildMessageRequest) (controlrpc.ChildMessageResult, bool, error) {
	if m == nil || m.service == nil {
		return controlrpc.ChildMessageResult{}, false, errors.New("child controller is not wired")
	}
	message, inserted, err := m.service.SendChildMessage(ctx, runtime.ChildMessageSendRequest{
		ChildSessionID: domain.SessionID(request.ChildSessionID), AuthorizerRunID: domain.RunID(request.ParentRunID),
		IdempotencyKey: request.OperationID, Body: []byte(request.Text),
	})
	if err != nil {
		return controlrpc.ChildMessageResult{}, false, err
	}
	return childMessageResult(message), inserted, nil
}

func (m *workerManager) ListChildMessages(ctx context.Context, request controlrpc.ChildMessageListRequest) ([]controlrpc.ChildMessageResult, error) {
	if m == nil || m.service == nil {
		return nil, errors.New("child controller is not wired")
	}
	messages, err := m.service.ListChildMessages(ctx, domain.SessionID(request.ChildSessionID), domain.RunID(request.AuthorizerRunID), 100)
	if err != nil {
		return nil, err
	}
	result := make([]controlrpc.ChildMessageResult, 0, len(messages))
	for _, message := range messages {
		result = append(result, childMessageResult(message))
	}
	return result, nil
}

func childMessageResult(message domain.ChildMailboxMessage) controlrpc.ChildMessageResult {
	return controlrpc.ChildMessageResult{
		ID: message.ID, ChildSessionID: string(message.ChildSessionID),
		SenderSessionID: string(message.SenderSessionID), RecipientSessionID: string(message.RecipientSessionID),
		Sequence: message.Sequence, Text: string(message.Body), Status: string(message.Status), CreatedAt: message.CreatedAt,
	}
}

func (m *workerManager) GetChild(ctx context.Context, id string) (controlrpc.ChildResult, error) {
	if m == nil || m.service == nil || m.runs == nil || id == "" {
		return controlrpc.ChildResult{}, errors.New("child controller is not wired or run id is empty")
	}
	run, err := m.runs.GetRun(ctx, domain.RunID(id))
	if err != nil {
		return controlrpc.ChildResult{}, err
	}
	if run.Kind != domain.RunKindChild {
		return controlrpc.ChildResult{}, errors.New("run is not a child")
	}
	return m.childResult(ctx, run, "")
}

func (m *workerManager) ListChildren(ctx context.Context, parentID string, tree bool) ([]controlrpc.ChildResult, error) {
	if m == nil || m.runs == nil || parentID == "" {
		return nil, errors.New("child controller is not wired or parent run id is empty")
	}
	var runs []domain.Run
	var err error
	if tree {
		runs, err = m.runs.ListRunTree(ctx, domain.RunID(parentID))
	} else {
		runs, err = m.runs.ListChildRuns(ctx, domain.RunID(parentID))
	}
	if err != nil {
		return nil, err
	}
	result := make([]controlrpc.ChildResult, 0, len(runs))
	for _, run := range runs {
		if run.Kind != domain.RunKindChild {
			continue
		}
		item, err := m.childResult(ctx, run, "")
		if err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, nil
}

func (m *workerManager) WaitChild(ctx context.Context, id string) (controlrpc.ChildResult, error) {
	if id == "" {
		return controlrpc.ChildResult{}, errors.New("child run id is required")
	}
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	for {
		result, err := m.GetChild(ctx, id)
		if err != nil {
			return controlrpc.ChildResult{}, err
		}
		if isTerminalString(result.Status) {
			return result, nil
		}
		select {
		case <-ctx.Done():
			return controlrpc.ChildResult{}, ctx.Err()
		case <-ticker.C:
		}
	}
}

func (m *workerManager) CancelChild(ctx context.Context, id string) (controlrpc.ChildResult, error) {
	result, err := m.GetChild(ctx, id)
	if err != nil {
		return controlrpc.ChildResult{}, err
	}
	if isTerminalString(result.Status) {
		return result, nil
	}
	if !m.service.Cancel(domain.RunID(id)) {
		return m.GetChild(ctx, id)
	}
	return m.WaitChild(ctx, id)
}

// CancelChildRun is called by Service while a Session is being deleted. The
// Service owns both the active Eino context and terminal event, so this is a
// direct cancellation request rather than a separate process signal.
func (m *workerManager) CancelChildRun(id domain.RunID) bool {
	if m == nil || m.service == nil || id == "" {
		return false
	}
	return m.service.Cancel(id)
}

// Close is retained for App shutdown ordering. Service.CancelAll and
// Service.WaitIdle own cancellation and draining for native child Runs.
func (m *workerManager) Close(context.Context) error { return nil }

func (m *workerManager) childResult(ctx context.Context, run domain.Run, workspaceID string) (controlrpc.ChildResult, error) {
	result := controlrpc.ChildResult{
		ID: string(run.ID), ParentRunID: string(run.ParentID), RootRunID: string(run.RootID), ChildMode: string(run.EffectiveChildMode()),
		SessionID: string(run.SessionID), Status: string(run.Status), Depth: run.Depth,
		WorkspaceID: workspaceID, CreatedAt: run.CreatedAt,
	}
	if m.service != nil {
		summary, message, storedWorkspaceID, err := m.service.ChildRunDetails(ctx, run.ID)
		if err != nil {
			return controlrpc.ChildResult{}, fmt.Errorf("read child outcome: %w", err)
		}
		result.Result, result.Error = summary, message
		if result.WorkspaceID == "" {
			result.WorkspaceID = storedWorkspaceID
		}
	}
	return result, nil
}

func isTerminalString(status string) bool {
	return domain.RunStatus(status).Terminal()
}

var _ controlrpc.ChildController = (*workerManager)(nil)
