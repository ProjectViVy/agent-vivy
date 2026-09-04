package app

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	"agent-vivy/internal/domain"
	controlrpc "agent-vivy/internal/rpc"
	"agent-vivy/internal/runtime"
	"agent-vivy/internal/storage"
	"agent-vivy/internal/tools"
	"agent-vivy/internal/worker"
)

const (
	maxChildDepth        = 4
	maxChildrenPerParent = 4
	childMaxTurns        = 8
)

// workerManager owns the durable parent-child run tree and the live child
// supervisors. The child process has no storage, model, or tool authority;
// every privileged operation returns here through the worker RPC peer.
type workerManager struct {
	service          *runtime.Service
	runs             storage.RunStore
	approvals        storage.ApprovalStore
	policy           *runtime.PolicyEngine
	hooks            *runtime.ToolHookChain
	tools            map[string]tools.Tool
	toolOrder        []string
	maxResultSize    int
	approvalLifetime time.Duration
	model            runtime.WorkerChatModel
	workerLog        worker.WorkerLog

	mu             sync.Mutex
	children       map[domain.RunID]*childHandle
	parentCounts   map[domain.RunID]int
	approvalWaiter map[string]chan worker.ApprovalWaitResult
	resolved       map[string]worker.ApprovalWaitResult
}

type childHandle struct {
	run        domain.Run
	workspace  string
	profile    domain.PolicyProfile
	snapshot   domain.PolicySnapshot
	ledger     *runtime.BudgetLedger
	supervisor *worker.Supervisor
	cancel     context.CancelFunc
	done       chan struct{}
	once       sync.Once

	mu     sync.Mutex
	result controlrpc.ChildResult
}

func newWorkerManager(service *runtime.Service, runs storage.RunStore, approvals storage.ApprovalStore, policy *runtime.PolicyEngine, hooks *runtime.ToolHookChain, registered []tools.Tool, maxResultSize int, approvalLifetime time.Duration, model runtime.WorkerChatModel, workerLog worker.WorkerLog) *workerManager {
	byName := make(map[string]tools.Tool, len(registered))
	for _, tool := range registered {
		byName[tool.Spec().Name] = tool
	}
	order := make([]string, 0, len(byName))
	for name := range byName {
		order = append(order, name)
	}
	sort.Strings(order)
	if approvalLifetime <= 0 {
		approvalLifetime = 5 * time.Minute
	}
	return &workerManager{
		service: service, runs: runs, approvals: approvals, policy: policy, hooks: hooks,
		tools: byName, toolOrder: order, maxResultSize: maxResultSize, approvalLifetime: approvalLifetime,
		model: model, workerLog: workerLog, children: make(map[domain.RunID]*childHandle),
		parentCounts: make(map[domain.RunID]int), approvalWaiter: make(map[string]chan worker.ApprovalWaitResult),
		resolved: make(map[string]worker.ApprovalWaitResult),
	}
}

func (m *workerManager) StartChild(ctx context.Context, request controlrpc.ChildRequest) (controlrpc.ChildResult, error) {
	if m == nil || m.service == nil || m.runs == nil {
		return controlrpc.ChildResult{}, errors.New("worker manager is not wired")
	}
	if request.ParentRunID == "" || request.Text == "" {
		return controlrpc.ChildResult{}, errors.New("parent_run_id and text are required")
	}
	parentID := domain.RunID(request.ParentRunID)
	parent, err := m.runs.GetRun(ctx, parentID)
	if err != nil {
		return controlrpc.ChildResult{}, err
	}
	if parent.Status != domain.RunActive && parent.Status != domain.RunAccepted {
		return controlrpc.ChildResult{}, errors.New("worker parent is not active")
	}
	if parent.Depth >= maxChildDepth {
		return controlrpc.ChildResult{}, fmt.Errorf("worker child depth exceeds %d", maxChildDepth)
	}
	for _, name := range request.ToolNames {
		if _, ok := m.tools[name]; !ok {
			return controlrpc.ChildResult{}, fmt.Errorf("child tool %q is not configured", name)
		}
	}
	_, parentLedger, _, err := m.service.WorkerParentAuthority(ctx, parentID)
	if err != nil {
		return controlrpc.ChildResult{}, err
	}
	if err := parentLedger.ReserveEvent(); err != nil {
		return controlrpc.ChildResult{}, err
	}
	m.mu.Lock()
	if m.parentCounts[parentID] >= maxChildrenPerParent {
		m.mu.Unlock()
		return controlrpc.ChildResult{}, fmt.Errorf("worker parent concurrency exceeds %d", maxChildrenPerParent)
	}
	m.parentCounts[parentID]++
	m.mu.Unlock()

	childID := newChildRunID()
	rootID := parent.RootID
	if rootID == "" {
		rootID = parent.ID
	}
	child := domain.Run{ID: childID, SessionID: parent.SessionID, Status: domain.RunAccepted, CreatedAt: time.Now().UnixMilli(), Kind: domain.RunKindChild, ParentID: parent.ID, RootID: rootID, Depth: parent.Depth + 1}
	if err := m.service.CreateWorkerRun(ctx, child); err != nil {
		m.releaseParent(parentID)
		return controlrpc.ChildResult{}, fmt.Errorf("create child run: %w", err)
	}
	if _, err := m.service.RecordExternalRunEvent(ctx, childID, domain.EventChildRequested, map[string]any{
		"parent_run_id": string(parent.ID), "depth": child.Depth, "text": request.Text,
	}); err != nil {
		m.failCreatedChild(ctx, child, "child request could not be journaled")
		m.releaseParent(parentID)
		return controlrpc.ChildResult{}, err
	}
	snapshot, ledger, workspaceID, err := m.service.WorkerChildAuthority(ctx, parent.ID, child.ID)
	if err != nil {
		m.failCreatedChild(ctx, child, "child authority unavailable")
		m.releaseParent(parentID)
		return controlrpc.ChildResult{}, err
	}
	if request.PolicyProfile != "" && domain.PolicyProfile(request.PolicyProfile) != snapshot.Profile {
		m.failCreatedChild(ctx, child, "child policy cannot widen parent authority")
		m.releaseParent(parentID)
		return controlrpc.ChildResult{}, errors.New("child policy cannot widen parent authority")
	}
	if err := m.service.RegisterWorkerAuthority(child.ID, snapshot, ledger); err != nil {
		m.failCreatedChild(ctx, child, "child authority registration failed")
		m.releaseParent(parentID)
		return controlrpc.ChildResult{}, err
	}
	if err := m.runs.SetRunStatus(ctx, child.ID, domain.RunActive); err != nil {
		m.service.UnregisterWorkerAuthority(child.ID)
		m.failCreatedChild(ctx, child, "child activation failed")
		m.releaseParent(parentID)
		return controlrpc.ChildResult{}, err
	}
	if err := m.recordChildEvent(ctx, child.ID, ledger, domain.EventChildStarted, map[string]any{
		"parent_run_id": string(parent.ID), "workspace_id": workspaceID,
	}); err != nil {
		m.service.UnregisterWorkerAuthority(child.ID)
		m.failCreatedChild(ctx, child, "child start could not be journaled")
		m.releaseParent(parentID)
		return controlrpc.ChildResult{}, err
	}

	childCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
	handle := &childHandle{run: child, workspace: workspaceID, profile: snapshot.Profile, snapshot: snapshot, ledger: ledger, cancel: cancel, done: make(chan struct{})}
	handle.result = childResult(child, workspaceID, "active", "", "")
	if err := m.service.RegisterWorkerProcess(ctx, child.ID, func() {
		m.mu.Lock()
		m.children[child.ID] = handle
		m.mu.Unlock()
	}); err != nil {
		cancel()
		m.service.UnregisterWorkerAuthority(child.ID)
		m.failCreatedChild(ctx, child, "child process registration was fenced by session deletion")
		m.releaseParent(parentID)
		return controlrpc.ChildResult{}, err
	}
	if childCtx.Err() != nil {
		m.finishChild(handle, "cancelled", "", "child cancelled before process start", "session_deleted")
		return controlrpc.ChildResult{}, storage.ErrNotFound
	}

	modelBroker, err := runtime.NewWorkerModelBroker(m.model, ledger)
	if err != nil {
		m.finishChild(handle, "failed", "", "child model broker unavailable", "model_broker_unavailable")
		return controlrpc.ChildResult{}, err
	}
	authority := worker.Authority{ParentRunID: parent.ID, Snapshot: snapshot, WorkspaceID: workspaceID, Budget: ledger, Log: m.workerLog}
	toolBroker := &childToolBroker{manager: m, childID: child.ID, parentID: parent.ID, profile: snapshot.Profile, policyHash: snapshot.Hash, ledger: ledger}
	supervisor, err := worker.StartWithBrokers(childCtx, authority, toolBroker, &legacyModelBroker{inner: modelBroker, manager: m, childID: child.ID, ledger: ledger}, m, nil)
	if err != nil {
		m.finishChild(handle, "failed", "", "child worker could not start", "worker_start_failed")
		return controlrpc.ChildResult{}, err
	}
	handle.mu.Lock()
	handle.supervisor = supervisor
	handle.mu.Unlock()
	go m.driveChild(childCtx, handle, supervisor, request.Text, request.System, request.ToolNames)
	return handle.resultSnapshot(), nil
}

func (m *workerManager) driveChild(ctx context.Context, handle *childHandle, supervisor *worker.Supervisor, text, system string, requestedTools []string) {
	result, err := supervisor.Run(ctx, worker.Spec{
		RunID: handle.run.ID, ParentRunID: handle.run.ParentID, PolicyProfile: handle.profile,
		PolicyHash: handle.snapshot.Hash, WorkspaceID: handle.workspace, Text: text, System: system,
		MaxTurns: childMaxTurns, Tools: m.modelTools(requestedTools),
	})
	_ = supervisor.Close()
	if err != nil {
		if errors.Is(ctx.Err(), context.Canceled) {
			m.finishChild(handle, "cancelled", "", "child cancelled by operator", "user_requested")
		} else {
			m.finishChild(handle, "failed", "", "child worker failed", "worker_failed")
		}
		return
	}
	m.finishChild(handle, "completed", result, "", "")
}

func (m *workerManager) finishChild(handle *childHandle, status, result, message, cause string) {
	handle.once.Do(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		var typ domain.EventType
		var payload any
		switch status {
		case "completed":
			typ, payload = domain.EventChildCompleted, map[string]any{"summary": result}
		case "cancelled":
			typ, payload = domain.EventChildCancelled, map[string]any{"reason": cause}
		default:
			typ, payload = domain.EventChildFailed, map[string]any{"cause_category": cause, "message": message}
		}
		if _, err := m.service.RecordExternalRunEvent(ctx, handle.run.ID, typ, payload); err != nil {
			// The storage truth is authoritative; the handle still closes so
			// callers cannot wait forever on a failed persistence attempt.
			message = "child terminal event could not be persisted"
		}
		handle.mu.Lock()
		handle.result.Status = status
		handle.result.Result = result
		handle.result.Error = message
		handle.mu.Unlock()
		m.service.UnregisterWorkerAuthority(handle.run.ID)
		m.releaseParent(handle.run.ParentID)
		close(handle.done)
	})
}

func (m *workerManager) failCreatedChild(ctx context.Context, child domain.Run, message string) {
	_, _ = m.service.RecordExternalRunEvent(ctx, child.ID, domain.EventChildFailed, map[string]any{
		"cause_category": "child_setup_failed", "message": message,
	})
}

func (m *workerManager) GetChild(ctx context.Context, id string) (controlrpc.ChildResult, error) {
	run, err := m.runs.GetRun(ctx, domain.RunID(id))
	if err != nil {
		return controlrpc.ChildResult{}, err
	}
	if run.Kind != domain.RunKindChild {
		return controlrpc.ChildResult{}, errors.New("run is not a child")
	}
	m.mu.Lock()
	handle := m.children[run.ID]
	m.mu.Unlock()
	if handle != nil {
		return handle.resultSnapshot(), nil
	}
	return childResult(run, "", string(run.Status), "", ""), nil
}

func (m *workerManager) ListChildren(ctx context.Context, parentID string, tree bool) ([]controlrpc.ChildResult, error) {
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
	out := make([]controlrpc.ChildResult, 0, len(runs))
	for _, run := range runs {
		item, err := m.GetChild(ctx, string(run.ID))
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, nil
}

func (m *workerManager) WaitChild(ctx context.Context, id string) (controlrpc.ChildResult, error) {
	result, err := m.GetChild(ctx, id)
	if err != nil {
		return controlrpc.ChildResult{}, err
	}
	if isTerminalString(result.Status) {
		return result, nil
	}
	m.mu.Lock()
	handle := m.children[domain.RunID(id)]
	m.mu.Unlock()
	if handle == nil {
		return result, errors.New("child worker is not active in this process")
	}
	select {
	case <-handle.done:
		return handle.resultSnapshot(), nil
	case <-ctx.Done():
		return controlrpc.ChildResult{}, ctx.Err()
	}
}

func (m *workerManager) CancelChild(ctx context.Context, id string) (controlrpc.ChildResult, error) {
	run, err := m.runs.GetRun(ctx, domain.RunID(id))
	if err != nil {
		return controlrpc.ChildResult{}, err
	}
	m.mu.Lock()
	handle := m.children[run.ID]
	m.mu.Unlock()
	if handle != nil {
		handle.cancel()
		return handle.resultSnapshot(), nil
	}
	if run.Status.Terminal() {
		return childResult(run, "", string(run.Status), "", ""), nil
	}
	if _, err := m.service.RecordExternalRunEvent(ctx, run.ID, domain.EventChildCancelled, map[string]any{"reason": "user_requested"}); err != nil {
		return controlrpc.ChildResult{}, err
	}
	return m.GetChild(ctx, id)
}

// CancelChildRun implements runtime.ChildRunCanceller for session deletion.
// It only cancels the live process; the service tombstone owns durable event
// suppression and storage removal.
func (m *workerManager) CancelChildRun(id domain.RunID) bool {
	m.mu.Lock()
	handle := m.children[id]
	m.mu.Unlock()
	if handle == nil {
		return false
	}
	handle.cancel()
	return true
}

func (m *workerManager) CancelSessionChildren(sessionID domain.SessionID) {
	m.mu.Lock()
	handles := make([]*childHandle, 0)
	for _, handle := range m.children {
		if handle.run.SessionID == sessionID {
			handles = append(handles, handle)
		}
	}
	m.mu.Unlock()
	for _, handle := range handles {
		handle.cancel()
	}
}

func (m *workerManager) Wait(ctx context.Context, request worker.ApprovalWaitRequest) (worker.ApprovalWaitResult, error) {
	if request.ApprovalID == "" || request.RunID == "" {
		return worker.ApprovalWaitResult{}, errors.New("approval wait request is incomplete")
	}
	m.mu.Lock()
	if result, ok := m.resolved[request.ApprovalID]; ok {
		delete(m.resolved, request.ApprovalID)
		m.mu.Unlock()
		return result, nil
	}
	waiter := m.approvalWaiter[request.ApprovalID]
	if waiter == nil {
		waiter = make(chan worker.ApprovalWaitResult, 1)
		m.approvalWaiter[request.ApprovalID] = waiter
	}
	m.mu.Unlock()
	select {
	case result := <-waiter:
		m.mu.Lock()
		delete(m.approvalWaiter, request.ApprovalID)
		m.mu.Unlock()
		return result, nil
	case <-ctx.Done():
		return worker.ApprovalWaitResult{}, ctx.Err()
	}
}

func (m *workerManager) ResolveChildApproval(ctx context.Context, approval domain.Approval, decision string) error {
	if approval.Kind != domain.ApprovalKindChild {
		return errors.New("approval is not child-owned")
	}
	result := worker.ApprovalWaitResult{Decision: decision}
	m.mu.Lock()
	waiter := m.approvalWaiter[approval.ID]
	if waiter == nil {
		m.resolved[approval.ID] = result
	} else {
		waiter <- result
	}
	m.mu.Unlock()
	_, ledger, _, err := m.service.WorkerParentAuthority(ctx, approval.RunID)
	if err != nil {
		return err
	}
	err = m.recordChildEvent(ctx, approval.RunID, ledger, domain.EventChildResumed, map[string]any{"approval_id": approval.ID, "decision": decision})
	return err
}

func (m *workerManager) recordChildEvent(ctx context.Context, runID domain.RunID, ledger *runtime.BudgetLedger, typ domain.EventType, payload any) error {
	return m.recordChildEventVersion(ctx, runID, ledger, typ, 1, payload)
}

func (m *workerManager) recordChildEventVersion(ctx context.Context, runID domain.RunID, ledger *runtime.BudgetLedger, typ domain.EventType, version int, payload any) error {
	// Streaming chunks are transport framing, not semantic run-tree events.
	// Match the native mapper budget rule so a long answer cannot consume the
	// entire event allowance before its model.completed boundary arrives.
	if ledger != nil && typ != domain.EventModelDelta && typ != domain.EventModelReasoningDelta {
		if err := ledger.ReserveEvent(); err != nil {
			return err
		}
	}
	_, err := m.service.RecordExternalRunEventVersion(ctx, runID, typ, version, payload)
	return err
}

func (m *workerManager) Close(ctx context.Context) error {
	m.mu.Lock()
	handles := make([]*childHandle, 0, len(m.children))
	for _, handle := range m.children {
		if !isTerminalString(handle.resultSnapshot().Status) {
			handles = append(handles, handle)
		}
	}
	m.mu.Unlock()
	for _, handle := range handles {
		handle.cancel()
	}
	for _, handle := range handles {
		select {
		case <-handle.done:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}

func (m *workerManager) releaseParent(parentID domain.RunID) {
	m.mu.Lock()
	if m.parentCounts[parentID] > 1 {
		m.parentCounts[parentID]--
	} else {
		delete(m.parentCounts, parentID)
	}
	m.mu.Unlock()
}

func (m *workerManager) modelTools(names []string) []worker.ModelTool {
	allowed := map[string]struct{}{}
	for _, name := range names {
		allowed[name] = struct{}{}
	}
	result := make([]worker.ModelTool, 0, len(m.tools))
	for _, name := range m.toolOrder {
		tool := m.tools[name]
		if len(allowed) > 0 {
			if _, ok := allowed[name]; !ok {
				continue
			}
		}
		spec := tool.Spec()
		params := make(map[string]worker.ModelParam, len(spec.Params))
		for key, param := range spec.Params {
			params[key] = worker.ModelParam{Description: param.Desc, Required: param.Required}
		}
		result = append(result, worker.ModelTool{Name: spec.Name, Description: spec.Description, Readonly: spec.Readonly, Params: params})
	}
	return result
}

type childToolBroker struct {
	manager    *workerManager
	childID    domain.RunID
	parentID   domain.RunID
	profile    domain.PolicyProfile
	policyHash string
	ledger     *runtime.BudgetLedger
}

func (b *childToolBroker) Execute(ctx context.Context, call worker.ToolCall) (worker.ToolResult, error) {
	callID := call.CallID
	if callID == "" {
		callID = call.ToolName
	}
	if err := b.manager.recordChildEvent(ctx, b.childID, b.ledger, domain.EventToolRequested, map[string]any{
		"tool_call_id": callID, "tool_name": call.ToolName, "args": call.Args,
	}); err != nil {
		return worker.ToolResult{Status: "error", Error: err.Error()}, nil
	}
	tool, ok := b.manager.tools[call.ToolName]
	if !ok {
		return worker.ToolResult{Status: "error", Error: "tool is not configured"}, nil
	}
	if b.ledger != nil {
		if err := b.ledger.ReserveToolCall(); err != nil {
			return worker.ToolResult{Status: "error", Error: err.Error()}, nil
		}
	}
	argsBytes, err := json.Marshal(call.Args)
	if err != nil {
		return worker.ToolResult{Status: "error", Error: "tool arguments are not JSON"}, nil
	}
	spec := tool.Spec()
	if err := tools.ValidateArgs(spec, argsBytes); err != nil {
		return worker.ToolResult{Status: "error", Error: err.Error()}, nil
	}
	if err := tools.ValidateArgsSafety(spec, argsBytes); err != nil {
		return worker.ToolResult{Status: "error", Error: err.Error()}, nil
	}
	evaluation, err := b.manager.policy.Evaluate(b.profile, spec, argsBytes)
	if err != nil {
		return worker.ToolResult{Status: "error", Error: err.Error()}, nil
	}
	if call.ApprovalID != "" {
		approval, err := b.manager.approvals.GetApproval(ctx, call.ApprovalID)
		if err != nil || approval.RunID != b.childID || approval.ToolCallID == "" || approval.ToolCallID != call.CallID {
			return worker.ToolResult{Status: "error", Error: "invalid child approval"}, nil
		}
		if approval.Decision == domain.ApprovalDenied {
			return worker.ToolResult{Status: "denied", Result: "tool call denied by the user"}, nil
		}
		if approval.Decision != domain.ApprovalApproved {
			return worker.ToolResult{Status: "approval_required", ApprovalID: call.ApprovalID}, nil
		}
	} else if evaluation.Decision == domain.PolicyPrompt {
		approvalID := newApprovalID()
		approval := domain.Approval{ID: approvalID, RunID: b.childID, ToolCallID: callID, Decision: domain.ApprovalPending, ExpiresAt: time.Now().Add(b.manager.approvalLifetime).UnixMilli(), Kind: domain.ApprovalKindChild}
		if err := b.manager.approvals.CreateApproval(ctx, approval); err != nil {
			return worker.ToolResult{Status: "error", Error: "could not create child approval"}, nil
		}
		b.manager.mu.Lock()
		b.manager.approvalWaiter[approvalID] = make(chan worker.ApprovalWaitResult, 1)
		b.manager.mu.Unlock()
		if err := b.manager.recordChildEvent(ctx, b.childID, b.ledger, domain.EventToolApprovalRequired, map[string]any{
			"approval_id": approvalID, "tool_call_id": callID, "tool_name": call.ToolName,
			"args": call.Args, "expires_at": approval.ExpiresAt,
		}); err != nil {
			return worker.ToolResult{Status: "error", Error: err.Error()}, nil
		}
		if err := b.manager.recordChildEvent(ctx, b.childID, b.ledger, domain.EventChildSuspended, map[string]any{"approval_id": approvalID, "tool_call_id": callID}); err != nil {
			return worker.ToolResult{Status: "error", Error: err.Error()}, nil
		}
		return worker.ToolResult{Status: "approval_required", ApprovalID: approvalID}, nil
	} else if evaluation.Decision == domain.PolicyDeny {
		return worker.ToolResult{Status: "denied", Result: "tool call denied by policy"}, nil
	}
	execute := runtime.ExecuteBrokerTool
	if call.ApprovalID != "" {
		execute = runtime.ExecuteApprovedBrokerTool
	}
	if err := b.manager.recordChildEvent(ctx, b.childID, b.ledger, domain.EventToolStarted, map[string]any{"tool_call_id": callID, "tool_name": call.ToolName}); err != nil {
		return worker.ToolResult{Status: "error", Error: err.Error()}, nil
	}
	result, err := execute(ctx, tool, b.manager.policy, b.manager.hooks, b.childID, b.profile, call.Args, b.manager.maxResultSize)
	if err != nil {
		_ = b.manager.recordChildEvent(ctx, b.childID, b.ledger, domain.EventToolFinished, map[string]any{"tool_call_id": callID, "tool_name": call.ToolName, "result": "", "error": err.Error()})
		return worker.ToolResult{Status: "error", Error: err.Error()}, nil
	}
	_ = b.manager.recordChildEvent(ctx, b.childID, b.ledger, domain.EventToolFinished, map[string]any{"tool_call_id": callID, "tool_name": call.ToolName, "result": result})
	return worker.ToolResult{Status: "completed", Result: result}, nil
}

type legacyModelBroker struct {
	inner   *runtime.WorkerModelBroker
	manager *workerManager
	childID domain.RunID
	ledger  *runtime.BudgetLedger
}

func (b *legacyModelBroker) Complete(ctx context.Context, request worker.ModelRequest) (worker.ModelResponse, error) {
	converted := runtime.WorkerModelRequest{RunID: request.RunID, ParentRunID: request.ParentRunID}
	converted.Messages = make([]runtime.WorkerModelMessage, 0, len(request.Messages))
	for _, message := range request.Messages {
		item := runtime.WorkerModelMessage{Role: message.Role, Content: message.Content, ToolCallID: message.ToolCallID}
		for _, call := range message.ToolCalls {
			item.ToolCalls = append(item.ToolCalls, runtime.WorkerModelToolCall{ID: call.ID, Name: call.Name, Arguments: call.Arguments})
		}
		converted.Messages = append(converted.Messages, item)
	}
	converted.Tools = make([]runtime.WorkerModelTool, 0, len(request.Tools))
	for _, tool := range request.Tools {
		params := make(map[string]runtime.WorkerModelParam, len(tool.Params))
		for name, param := range tool.Params {
			params[name] = runtime.WorkerModelParam{Description: param.Description, Required: param.Required}
		}
		converted.Tools = append(converted.Tools, runtime.WorkerModelTool{Name: tool.Name, Description: tool.Description, Readonly: tool.Readonly, Params: params})
	}
	result, err := b.inner.Complete(ctx, converted)
	if err != nil {
		return worker.ModelResponse{}, err
	}
	if b.manager != nil {
		content := result.Message.Content
		budget := b.manager.service.MaxEventPayloadBytes()
		for _, delta := range runtime.SplitModelTextForPayload(content, budget) {
			if eventErr := b.manager.recordChildEvent(ctx, b.childID, b.ledger, domain.EventModelDelta, map[string]any{"delta": delta}); eventErr != nil {
				return worker.ModelResponse{}, eventErr
			}
		}
		sum := sha256.Sum256([]byte(content))
		if eventErr := b.manager.recordChildEventVersion(ctx, b.childID, b.ledger, domain.EventModelCompleted, 2, map[string]any{
			"content_sha256": fmt.Sprintf("%x", sum[:]), "byte_len": len([]byte(content)),
		}); eventErr != nil {
			return worker.ModelResponse{}, eventErr
		}
		if result.Usage != nil {
			providerName, modelID := b.manager.service.CurrentModel()
			if eventErr := b.manager.recordChildEvent(ctx, b.childID, b.ledger, domain.EventModelUsage, map[string]any{
				"prompt_tokens":     result.Usage.PromptTokens,
				"completion_tokens": result.Usage.CompletionTokens,
				"total_tokens":      result.Usage.TotalTokens,
				"reasoning_tokens":  result.Usage.ReasoningTokens,
				"cached_tokens":     result.Usage.CachedTokens,
				"provider":          providerName,
				"model":             modelID,
				"source":            "child",
			}); eventErr != nil {
				return worker.ModelResponse{}, eventErr
			}
		}
	}
	out := worker.ModelResponse{Status: result.Status, StopReason: result.StopReason}
	out.Message.Role, out.Message.Content, out.Message.ToolCallID = result.Message.Role, result.Message.Content, result.Message.ToolCallID
	for _, call := range result.Message.ToolCalls {
		out.Message.ToolCalls = append(out.Message.ToolCalls, worker.ModelToolCall{ID: call.ID, Name: call.Name, Arguments: call.Arguments})
	}
	if result.Usage != nil {
		out.Usage = &worker.ModelUsage{PromptTokens: result.Usage.PromptTokens, CompletionTokens: result.Usage.CompletionTokens, TotalTokens: result.Usage.TotalTokens, ReasoningTokens: result.Usage.ReasoningTokens, CachedTokens: result.Usage.CachedTokens}
	}
	return out, nil
}

func (h *childHandle) resultSnapshot() controlrpc.ChildResult {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.result
}

func childResult(run domain.Run, workspace, status, result, message string) controlrpc.ChildResult {
	return controlrpc.ChildResult{ID: string(run.ID), ParentRunID: string(run.ParentID), RootRunID: string(run.RootID), SessionID: string(run.SessionID), Status: status, Depth: run.Depth, WorkspaceID: workspace, Result: result, Error: message, CreatedAt: run.CreatedAt}
}

func isTerminalString(status string) bool {
	return status == string(domain.RunCompleted) || status == string(domain.RunFailed) || status == string(domain.RunCancelled)
}

func newChildRunID() domain.RunID {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprintf("app: crypto/rand unavailable: %v", err))
	}
	return domain.RunID("child_" + hex.EncodeToString(b))
}

func newApprovalID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprintf("app: crypto/rand unavailable: %v", err))
	}
	return "apr_child_" + hex.EncodeToString(b)
}

var _ controlrpc.ChildController = (*workerManager)(nil)
var _ runtime.ChildApprovalRouter = (*workerManager)(nil)
