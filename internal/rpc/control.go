package rpc

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/events"
	"agent-vivy/internal/runtime"
	"agent-vivy/internal/storage"
)

const (
	CodeNotFound = -32004
	CodeConflict = -32009
)

type ControlDeps struct {
	Sessions  storage.SessionStore
	Messages  storage.MessageStore
	Runs      storage.RunStore
	Journal   storage.Journal
	Approvals storage.ApprovalStore
	Questions storage.QuestionStore
	Bus       *events.Bus
	Service   *runtime.Service
	Children  ChildController
}

// ChildRequest starts one durable, asynchronous child run under a parent.
// The parent controller derives policy hash, workspace, and budget from the
// durable parent; callers cannot supply a wider authority.
type ChildRequest struct {
	ParentRunID   string   `json:"parent_run_id"`
	Text          string   `json:"text"`
	PolicyProfile string   `json:"policy_profile,omitempty"`
	ToolNames     []string `json:"tool_names,omitempty"`
}

type ChildResult struct {
	ID          string `json:"id"`
	ParentRunID string `json:"parent_run_id"`
	RootRunID   string `json:"root_run_id"`
	SessionID   string `json:"session_id"`
	Status      string `json:"status"`
	Depth       int    `json:"depth"`
	WorkspaceID string `json:"workspace_id,omitempty"`
	Result      string `json:"result,omitempty"`
	Error       string `json:"error,omitempty"`
	CreatedAt   int64  `json:"created_at"`
}

type ChildController interface {
	StartChild(context.Context, ChildRequest) (ChildResult, error)
	GetChild(context.Context, string) (ChildResult, error)
	ListChildren(context.Context, string, bool) ([]ChildResult, error)
	WaitChild(context.Context, string) (ChildResult, error)
	CancelChild(context.Context, string) (ChildResult, error)
}

func NewControlHandler(deps ControlDeps) (Handler, error) {
	if deps.Sessions == nil || deps.Messages == nil || deps.Runs == nil || deps.Journal == nil ||
		deps.Approvals == nil || deps.Questions == nil || deps.Bus == nil || deps.Service == nil {
		return nil, errors.New("rpc: control dependencies are incomplete")
	}
	return &controlHandler{deps: deps, subscriptions: make(map[string]context.CancelFunc)}, nil
}

type controlHandler struct {
	deps ControlDeps

	mu            sync.Mutex
	subscriptions map[string]context.CancelFunc
}

type sessionParams struct {
	SessionID string `json:"session_id"`
}

type turnParams struct {
	SessionID     string `json:"session_id"`
	Text          string `json:"text"`
	Mode          string `json:"mode,omitempty"`
	PolicyProfile string `json:"policy_profile,omitempty"`
}

type runParams struct {
	RunID string `json:"run_id"`
}

type subscribeParams struct {
	RunID    string `json:"run_id"`
	AfterSeq int64  `json:"after_seq,omitempty"`
}

type approvalParams struct {
	ApprovalID string `json:"approval_id"`
	Decision   string `json:"decision"`
}

type questionParams struct {
	QuestionID string `json:"question_id"`
	Answer     string `json:"answer"`
}

type unsubscribeParams struct {
	SubscriptionID string `json:"subscription_id"`
}

type sessionResult struct {
	ID        domain.SessionID `json:"id"`
	Title     string           `json:"title"`
	CreatedAt int64            `json:"created_at"`
}

type messageResult struct {
	ID        string       `json:"id"`
	RunID     domain.RunID `json:"run_id,omitempty"`
	Role      domain.Role  `json:"role"`
	Content   string       `json:"content"`
	CreatedAt int64        `json:"created_at"`
}

type runResult struct {
	ID        domain.RunID     `json:"id"`
	SessionID domain.SessionID `json:"session_id"`
	Status    domain.RunStatus `json:"status"`
	CreatedAt int64            `json:"created_at"`
}

type eventResult struct {
	RunID          domain.RunID     `json:"run_id"`
	Seq            domain.EventSeq  `json:"seq"`
	Type           domain.EventType `json:"type"`
	CreatedAt      int64            `json:"created_at"`
	PayloadVersion int              `json:"payload_version"`
	Payload        json.RawMessage  `json:"payload"`
}

type preflightResult struct {
	Status        runtime.PreflightStatus `json:"status"`
	Mode          domain.RunMode          `json:"mode"`
	PolicyProfile domain.PolicyProfile    `json:"policy_profile"`
	PolicyHash    string                  `json:"policy_hash,omitempty"`
	SelectedTools []string                `json:"selected_tools"`
	ToolDecisions []policyDecisionResult  `json:"tool_decisions"`
	ContextBytes  int                     `json:"context_bytes"`
	HookReady     bool                    `json:"hook_ready"`
	Warnings      []string                `json:"warnings"`
	Blockers      []string                `json:"blockers"`
	NextActions   []string                `json:"next_actions"`
}

type policyDecisionResult struct {
	ToolName string                `json:"tool_name"`
	Decision domain.PolicyDecision `json:"decision"`
	Reason   string                `json:"reason"`
}

type approvalResult struct {
	ID         string       `json:"id"`
	RunID      domain.RunID `json:"run_id"`
	ToolCallID string       `json:"tool_call_id"`
	Decision   string       `json:"decision"`
	ExpiresAt  int64        `json:"expires_at"`
}

type questionResult struct {
	ID         string                `json:"id"`
	RunID      domain.RunID          `json:"run_id"`
	ToolCallID string                `json:"tool_call_id"`
	Prompt     string                `json:"prompt"`
	Status     domain.QuestionStatus `json:"status"`
	ExpiresAt  int64                 `json:"expires_at"`
}

type backgroundResult struct {
	ID          domain.RunID     `json:"id"`
	SessionID   domain.SessionID `json:"session_id"`
	Status      domain.RunStatus `json:"status"`
	CreatedAt   int64            `json:"created_at"`
	WorkspaceID string           `json:"workspace_id,omitempty"`
}

func (h *controlHandler) Handle(ctx context.Context, peer *Peer, request Request) (any, *Error) {
	switch request.Method {
	case "initialize", "capabilities":
		return map[string]any{
			"protocol_version": ProtocolVersion,
			"capabilities": []string{
				"session", "turn", "run", "preflight", "approval", "question", "run.subscribe",
				"child.start", "child.get", "child.list", "child.wait", "child.cancel",
			},
		}, nil
	case "session/create":
		return h.createSession(ctx, request)
	case "session/list":
		return h.listSessions(ctx)
	case "session/get":
		return h.getSession(ctx, request)
	case "session/rename":
		return h.renameSession(ctx, request)
	case "session/delete":
		return h.deleteSession(ctx, request)
	case "session/messages":
		return h.listMessages(ctx, request)
	case "preflight/run":
		return h.preflight(ctx, request)
	case "turn/start":
		return h.startTurn(ctx, request)
	case "turn/interrupt", "run/cancel":
		return h.cancelRun(request)
	case "run/get":
		return h.getRun(ctx, request)
	case "run/subscribe":
		return h.subscribe(ctx, peer, request)
	case "run/unsubscribe":
		return h.unsubscribe(request)
	case "run/log":
		return h.runLog(ctx, request)
	case "approval/list":
		return h.listApprovals(ctx)
	case "approval/respond":
		return h.respondApproval(ctx, request)
	case "question/list":
		return h.listQuestions(ctx)
	case "question/respond":
		return h.respondQuestion(ctx, request)
	case "background/recover":
		if err := h.deps.Service.Recover(ctx); err != nil {
			return nil, internalError(err)
		}
		return map[string]any{"recovered": true}, nil
	case "background/list":
		return h.listBackground(ctx)
	case "background/attach":
		return h.attachBackground(ctx, request)
	case "child/start":
		return h.startChild(ctx, request)
	case "child/get":
		return h.getChild(ctx, request)
	case "child/list":
		return h.listChildren(ctx, request)
	case "child/wait":
		return h.waitChild(ctx, request)
	case "child/cancel":
		return h.cancelChild(ctx, request)
	default:
		return nil, &Error{Code: MethodNotFound, Message: "method not found: " + request.Method}
	}
}

func (h *controlHandler) childController() (ChildController, *Error) {
	if h.deps.Children == nil {
		return nil, &Error{Code: MethodNotFound, Message: "child controller is not configured"}
	}
	return h.deps.Children, nil
}

func (h *controlHandler) startChild(ctx context.Context, request Request) (any, *Error) {
	controller, rpcErr := h.childController()
	if rpcErr != nil {
		return nil, rpcErr
	}
	var params ChildRequest
	if err := decodeParams(request, &params); err != nil {
		return nil, err
	}
	if params.ParentRunID == "" || params.Text == "" {
		return nil, &Error{Code: InvalidParams, Message: "parent_run_id and text are required"}
	}
	result, err := controller.StartChild(ctx, params)
	if err != nil {
		return nil, internalError(err)
	}
	return result, nil
}

func (h *controlHandler) getChild(ctx context.Context, request Request) (any, *Error) {
	controller, rpcErr := h.childController()
	if rpcErr != nil {
		return nil, rpcErr
	}
	params, rpcErr := parseRunParams(request)
	if rpcErr != nil {
		return nil, rpcErr
	}
	result, err := controller.GetChild(ctx, params.RunID)
	if errors.Is(err, storage.ErrNotFound) {
		return nil, &Error{Code: CodeNotFound, Message: "child run not found"}
	}
	if err != nil {
		return nil, internalError(err)
	}
	return result, nil
}

func (h *controlHandler) listChildren(ctx context.Context, request Request) (any, *Error) {
	controller, rpcErr := h.childController()
	if rpcErr != nil {
		return nil, rpcErr
	}
	var params struct {
		ParentRunID string `json:"parent_run_id"`
		Tree        bool   `json:"tree,omitempty"`
	}
	if err := decodeParams(request, &params); err != nil {
		return nil, err
	}
	if params.ParentRunID == "" {
		return nil, &Error{Code: InvalidParams, Message: "parent_run_id is required"}
	}
	result, err := controller.ListChildren(ctx, params.ParentRunID, params.Tree)
	if err != nil {
		return nil, internalError(err)
	}
	return map[string]any{"children": result}, nil
}

func (h *controlHandler) waitChild(ctx context.Context, request Request) (any, *Error) {
	controller, rpcErr := h.childController()
	if rpcErr != nil {
		return nil, rpcErr
	}
	params, rpcErr := parseRunParams(request)
	if rpcErr != nil {
		return nil, rpcErr
	}
	result, err := controller.WaitChild(ctx, params.RunID)
	if errors.Is(err, storage.ErrNotFound) {
		return nil, &Error{Code: CodeNotFound, Message: "child run not found"}
	}
	if err != nil {
		return nil, internalError(err)
	}
	return result, nil
}

func (h *controlHandler) cancelChild(ctx context.Context, request Request) (any, *Error) {
	controller, rpcErr := h.childController()
	if rpcErr != nil {
		return nil, rpcErr
	}
	params, rpcErr := parseRunParams(request)
	if rpcErr != nil {
		return nil, rpcErr
	}
	result, err := controller.CancelChild(ctx, params.RunID)
	if errors.Is(err, storage.ErrNotFound) {
		return nil, &Error{Code: CodeNotFound, Message: "child run not found"}
	}
	if err != nil {
		return nil, internalError(err)
	}
	return result, nil
}

func (h *controlHandler) createSession(ctx context.Context, request Request) (any, *Error) {
	var params struct {
		Title string `json:"title"`
	}
	if err := decodeParams(request, &params); err != nil {
		return nil, err
	}
	if params.Title == "" {
		params.Title = "New session"
	}
	session := domain.Session{ID: domain.SessionID(newControlID("sess_")), Title: params.Title, CreatedAt: nowMillis()}
	if err := h.deps.Sessions.CreateSession(ctx, session); err != nil {
		return nil, internalError(err)
	}
	return sessionResult{ID: session.ID, Title: session.Title, CreatedAt: session.CreatedAt}, nil
}

func (h *controlHandler) listSessions(ctx context.Context) (any, *Error) {
	sessions, err := h.deps.Sessions.ListSessions(ctx)
	if err != nil {
		return nil, internalError(err)
	}
	out := make([]sessionResult, 0, len(sessions))
	for _, session := range sessions {
		out = append(out, sessionResult{ID: session.ID, Title: session.Title, CreatedAt: session.CreatedAt})
	}
	return map[string]any{"sessions": out}, nil
}

func (h *controlHandler) getSession(ctx context.Context, request Request) (any, *Error) {
	params, rpcErr := parseSessionParams(request)
	if rpcErr != nil {
		return nil, rpcErr
	}
	session, err := h.deps.Sessions.GetSession(ctx, domain.SessionID(params.SessionID))
	if errors.Is(err, storage.ErrNotFound) {
		return nil, &Error{Code: CodeNotFound, Message: "session not found"}
	}
	if err != nil {
		return nil, internalError(err)
	}
	messages, err := h.deps.Messages.ListMessages(ctx, session.ID)
	if err != nil {
		return nil, internalError(err)
	}
	out := make([]messageResult, 0, len(messages))
	for _, message := range messages {
		out = append(out, messageResult{ID: message.ID, RunID: message.RunID, Role: message.Role, Content: message.Content, CreatedAt: message.CreatedAt})
	}
	return map[string]any{
		"session":  sessionResult{ID: session.ID, Title: session.Title, CreatedAt: session.CreatedAt},
		"messages": out,
	}, nil
}

func (h *controlHandler) deleteSession(ctx context.Context, request Request) (any, *Error) {
	params, rpcErr := parseSessionParams(request)
	if rpcErr != nil {
		return nil, rpcErr
	}
	if err := h.deps.Sessions.DeleteSession(ctx, domain.SessionID(params.SessionID)); err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return nil, &Error{Code: CodeNotFound, Message: "session not found"}
		}
		return nil, internalError(err)
	}
	return map[string]any{"deleted": true}, nil
}

func (h *controlHandler) renameSession(ctx context.Context, request Request) (any, *Error) {
	var params struct {
		SessionID string `json:"session_id"`
		Title     string `json:"title"`
	}
	if err := decodeParams(request, &params); err != nil {
		return nil, err
	}
	if params.SessionID == "" || params.Title == "" {
		return nil, &Error{Code: InvalidParams, Message: "session_id and title are required"}
	}
	if err := h.deps.Sessions.RenameSession(ctx, domain.SessionID(params.SessionID), params.Title); err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return nil, &Error{Code: CodeNotFound, Message: "session not found"}
		}
		return nil, internalError(err)
	}
	session, err := h.deps.Sessions.GetSession(ctx, domain.SessionID(params.SessionID))
	if err != nil {
		return nil, internalError(err)
	}
	return sessionResult{ID: session.ID, Title: session.Title, CreatedAt: session.CreatedAt}, nil
}

func (h *controlHandler) listMessages(ctx context.Context, request Request) (any, *Error) {
	params, rpcErr := parseSessionParams(request)
	if rpcErr != nil {
		return nil, rpcErr
	}
	messages, err := h.deps.Messages.ListMessages(ctx, domain.SessionID(params.SessionID))
	if err != nil {
		return nil, internalError(err)
	}
	out := make([]messageResult, 0, len(messages))
	for _, message := range messages {
		out = append(out, messageResult{ID: message.ID, RunID: message.RunID, Role: message.Role, Content: message.Content, CreatedAt: message.CreatedAt})
	}
	return map[string]any{"messages": out}, nil
}

func (h *controlHandler) listBackground(ctx context.Context) (any, *Error) {
	runs, err := h.deps.Runs.ListActiveRuns(ctx)
	if err != nil {
		return nil, internalError(err)
	}
	out := make([]backgroundResult, 0, len(runs))
	for _, run := range runs {
		workspaceID := ""
		if workspace, workspaceErr := h.deps.Service.Workspace(ctx, run.ID); workspaceErr == nil {
			workspaceID = workspace.ID
		}
		out = append(out, backgroundResult{ID: run.ID, SessionID: run.SessionID, Status: run.Status, CreatedAt: run.CreatedAt, WorkspaceID: workspaceID})
	}
	return map[string]any{"runs": out}, nil
}

func (h *controlHandler) attachBackground(ctx context.Context, request Request) (any, *Error) {
	params, rpcErr := parseRunParams(request)
	if rpcErr != nil {
		return nil, rpcErr
	}
	run, err := h.deps.Runs.GetRun(ctx, domain.RunID(params.RunID))
	if errors.Is(err, storage.ErrNotFound) {
		return nil, &Error{Code: CodeNotFound, Message: "run not found"}
	}
	if err != nil {
		return nil, internalError(err)
	}
	workspace, err := h.deps.Service.Workspace(ctx, run.ID)
	if err != nil {
		return nil, internalError(err)
	}
	return backgroundResult{ID: run.ID, SessionID: run.SessionID, Status: run.Status, CreatedAt: run.CreatedAt, WorkspaceID: workspace.ID}, nil
}

func (h *controlHandler) preflight(ctx context.Context, request Request) (any, *Error) {
	params, rpcErr := parseTurnParams(request)
	if rpcErr != nil {
		return nil, rpcErr
	}
	result, err := h.deps.Service.Preflight(ctx, domain.SessionID(params.SessionID), params.Text, runtime.RunOptions{
		Mode: domain.RunMode(params.Mode), Profile: domain.PolicyProfile(params.PolicyProfile),
	})
	if err != nil {
		return nil, runtimeError(err)
	}
	return toPreflightResult(result), nil
}

func (h *controlHandler) startTurn(ctx context.Context, request Request) (any, *Error) {
	params, rpcErr := parseTurnParams(request)
	if rpcErr != nil {
		return nil, rpcErr
	}
	if params.Text == "" {
		return nil, &Error{Code: InvalidParams, Message: "text must not be empty"}
	}
	runID, err := h.deps.Service.RunWithOptions(ctx, domain.SessionID(params.SessionID), params.Text, runtime.RunOptions{
		Mode: domain.RunMode(params.Mode), Profile: domain.PolicyProfile(params.PolicyProfile),
	})
	if err != nil {
		return nil, runtimeError(err)
	}
	return map[string]any{"run_id": runID, "status": domain.RunAccepted}, nil
}

func (h *controlHandler) cancelRun(request Request) (any, *Error) {
	params, rpcErr := parseRunParams(request)
	if rpcErr != nil {
		return nil, rpcErr
	}
	if !h.deps.Service.Cancel(domain.RunID(params.RunID)) {
		return nil, &Error{Code: CodeNotFound, Message: "run is not active in this process"}
	}
	return map[string]any{"run_id": params.RunID, "status": "cancelling"}, nil
}

func (h *controlHandler) getRun(ctx context.Context, request Request) (any, *Error) {
	params, rpcErr := parseRunParams(request)
	if rpcErr != nil {
		return nil, rpcErr
	}
	run, err := h.deps.Runs.GetRun(ctx, domain.RunID(params.RunID))
	if errors.Is(err, storage.ErrNotFound) {
		return nil, &Error{Code: CodeNotFound, Message: "run not found"}
	}
	if err != nil {
		return nil, internalError(err)
	}
	return toRunResult(run), nil
}

func (h *controlHandler) runLog(ctx context.Context, request Request) (any, *Error) {
	params, rpcErr := parseSubscribeParams(request)
	if rpcErr != nil {
		return nil, rpcErr
	}
	entries, err := h.replayEvents(ctx, domain.RunID(params.RunID), domain.EventSeq(params.AfterSeq))
	if err != nil {
		return nil, internalError(err)
	}
	return map[string]any{"events": entries}, nil
}

func (h *controlHandler) listApprovals(ctx context.Context) (any, *Error) {
	approvals, err := h.deps.Approvals.ListPendingApprovals(ctx)
	if err != nil {
		return nil, internalError(err)
	}
	out := make([]approvalResult, 0, len(approvals))
	for _, approval := range approvals {
		out = append(out, approvalResult{ID: approval.ID, RunID: approval.RunID, ToolCallID: approval.ToolCallID, Decision: approval.Decision, ExpiresAt: approval.ExpiresAt})
	}
	return map[string]any{"approvals": out}, nil
}

func (h *controlHandler) respondApproval(ctx context.Context, request Request) (any, *Error) {
	var params approvalParams
	if err := decodeParams(request, &params); err != nil {
		return nil, err
	}
	if params.ApprovalID == "" || params.Decision == "" {
		return nil, &Error{Code: InvalidParams, Message: "approval_id and decision are required"}
	}
	if err := h.deps.Service.DecideApproval(ctx, params.ApprovalID, params.Decision); err != nil {
		return nil, runtimeError(err)
	}
	return map[string]any{"approval_id": params.ApprovalID, "decision": params.Decision}, nil
}

func (h *controlHandler) listQuestions(ctx context.Context) (any, *Error) {
	questions, err := h.deps.Questions.ListPendingQuestions(ctx)
	if err != nil {
		return nil, internalError(err)
	}
	out := make([]questionResult, 0, len(questions))
	for _, question := range questions {
		out = append(out, questionResult{ID: question.ID, RunID: question.RunID, ToolCallID: question.ToolCallID, Prompt: question.Prompt, Status: question.Status, ExpiresAt: question.ExpiresAt})
	}
	return map[string]any{"questions": out}, nil
}

func (h *controlHandler) respondQuestion(ctx context.Context, request Request) (any, *Error) {
	var params questionParams
	if err := decodeParams(request, &params); err != nil {
		return nil, err
	}
	if params.QuestionID == "" || params.Answer == "" {
		return nil, &Error{Code: InvalidParams, Message: "question_id and answer are required"}
	}
	if err := h.deps.Service.AnswerQuestion(ctx, params.QuestionID, params.Answer); err != nil {
		return nil, runtimeError(err)
	}
	return map[string]any{"question_id": params.QuestionID, "answer": params.Answer}, nil
}

func (h *controlHandler) subscribe(ctx context.Context, peer *Peer, request Request) (any, *Error) {
	params, rpcErr := parseSubscribeParams(request)
	if rpcErr != nil {
		return nil, rpcErr
	}
	if peer == nil {
		return nil, &Error{Code: InternalError, Message: "subscription requires a connected peer"}
	}
	streamCtx, cancel := context.WithCancel(ctx)
	subscriptionID := newControlID("sub_")
	h.mu.Lock()
	h.subscriptions[subscriptionID] = cancel
	h.mu.Unlock()
	stream := func() {
		defer func() {
			h.mu.Lock()
			delete(h.subscriptions, subscriptionID)
			h.mu.Unlock()
			cancel()
		}()
		h.streamRun(streamCtx, peer, subscriptionID, domain.RunID(params.RunID), domain.EventSeq(params.AfterSeq))
	}
	peer.AfterResponse(request.ID, stream)
	return map[string]any{"subscription_id": subscriptionID, "run_id": params.RunID, "after_seq": params.AfterSeq}, nil
}

func (h *controlHandler) unsubscribe(request Request) (any, *Error) {
	var params unsubscribeParams
	if err := decodeParams(request, &params); err != nil {
		return nil, err
	}
	h.mu.Lock()
	cancel := h.subscriptions[params.SubscriptionID]
	delete(h.subscriptions, params.SubscriptionID)
	h.mu.Unlock()
	if cancel == nil {
		return nil, &Error{Code: CodeNotFound, Message: "subscription not found"}
	}
	cancel()
	return map[string]any{"unsubscribed": true}, nil
}

func (h *controlHandler) streamRun(ctx context.Context, peer *Peer, subscriptionID string, runID domain.RunID, after domain.EventSeq) {
	ch, cancel := h.deps.Bus.Subscribe(runID)
	defer cancel()
	last := after
	send := func(event domain.RunEvent) bool {
		if event.Seq <= last {
			return true
		}
		if err := peer.Notify("run/event", map[string]any{
			"subscription_id": subscriptionID,
			"event":           toEventResult(event),
		}); err != nil {
			return false
		}
		last = event.Seq
		return true
	}
	entries, err := h.replayEvents(ctx, runID, last)
	if err != nil {
		_ = peer.Notify("run/stream_error", map[string]any{"subscription_id": subscriptionID, "message": "event replay failed"})
		return
	}
	for _, entry := range entries {
		if !send(domain.RunEvent{RunID: entry.RunID, Seq: entry.Seq, Type: entry.Type, CreatedAt: entry.CreatedAt, PayloadVersion: entry.PayloadVersion, Payload: entry.Payload}) {
			return
		}
	}
	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-ch:
			if ok {
				if !send(event) {
					return
				}
				continue
			}
			// Bus closes before sending terminal events. Replay the tail
			// so the JSON-RPC client still receives the terminal record.
			tail, replayErr := h.replayEvents(ctx, runID, last)
			if replayErr != nil {
				return
			}
			for _, entry := range tail {
				if !send(domain.RunEvent{RunID: entry.RunID, Seq: entry.Seq, Type: entry.Type, CreatedAt: entry.CreatedAt, PayloadVersion: entry.PayloadVersion, Payload: entry.Payload}) {
					return
				}
			}
			return
		}
	}
}

func (h *controlHandler) replayEvents(ctx context.Context, runID domain.RunID, after domain.EventSeq) ([]eventResult, error) {
	it, err := h.deps.Journal.Replay(ctx, runID, after)
	if err != nil {
		return nil, err
	}
	defer func() { _ = it.Close() }()
	var out []eventResult
	for it.Next() {
		out = append(out, toEventResult(it.Value().Event))
	}
	if err := it.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func decodeParams(request Request, target any) *Error {
	if len(request.Params) == 0 || string(request.Params) == "null" {
		return nil
	}
	if err := json.Unmarshal(request.Params, target); err != nil {
		return &Error{Code: InvalidParams, Message: "params must be a valid JSON object"}
	}
	return nil
}

func parseSessionParams(request Request) (sessionParams, *Error) {
	var params sessionParams
	if err := decodeParams(request, &params); err != nil {
		return params, err
	}
	if params.SessionID == "" {
		return params, &Error{Code: InvalidParams, Message: "session_id is required"}
	}
	return params, nil
}

func parseTurnParams(request Request) (turnParams, *Error) {
	var params turnParams
	if err := decodeParams(request, &params); err != nil {
		return params, err
	}
	if params.SessionID == "" {
		return params, &Error{Code: InvalidParams, Message: "session_id is required"}
	}
	return params, nil
}

func parseRunParams(request Request) (runParams, *Error) {
	var params runParams
	if err := decodeParams(request, &params); err != nil {
		return params, err
	}
	if params.RunID == "" {
		return params, &Error{Code: InvalidParams, Message: "run_id is required"}
	}
	return params, nil
}

func parseSubscribeParams(request Request) (subscribeParams, *Error) {
	var params subscribeParams
	if err := decodeParams(request, &params); err != nil {
		return params, err
	}
	if params.RunID == "" || params.AfterSeq < 0 {
		return params, &Error{Code: InvalidParams, Message: "run_id is required and after_seq must not be negative"}
	}
	return params, nil
}

func toRunResult(run domain.Run) runResult {
	return runResult{ID: run.ID, SessionID: run.SessionID, Status: run.Status, CreatedAt: run.CreatedAt}
}

func toPreflightResult(result runtime.PreflightResult) preflightResult {
	decisions := make([]policyDecisionResult, 0, len(result.ToolDecisions))
	for _, decision := range result.ToolDecisions {
		decisions = append(decisions, policyDecisionResult{ToolName: decision.ToolName, Decision: decision.Decision, Reason: decision.Reason})
	}
	return preflightResult{
		Status: result.Status, Mode: result.Mode, PolicyProfile: result.PolicyProfile, PolicyHash: result.PolicyHash,
		SelectedTools: result.SelectedTools, ToolDecisions: decisions, ContextBytes: result.ContextBytes,
		HookReady: result.HookReady, Warnings: result.Warnings, Blockers: result.Blockers, NextActions: result.NextActions,
	}
}

func toEventResult(event domain.RunEvent) eventResult {
	return eventResult{RunID: event.RunID, Seq: event.Seq, Type: event.Type, CreatedAt: event.CreatedAt, PayloadVersion: event.PayloadVersion, Payload: json.RawMessage(append([]byte(nil), event.Payload...))}
}

func runtimeError(err error) *Error {
	switch {
	case errors.Is(err, runtime.ErrInvalidRunMode), errors.Is(err, runtime.ErrInvalidPolicyProfile), errors.Is(err, runtime.ErrQuestionInvalidAnswer), errors.Is(err, runtime.ErrApprovalInvalidDecision):
		return &Error{Code: InvalidParams, Message: err.Error()}
	case errors.Is(err, runtime.ErrApprovalAlreadyDecided), errors.Is(err, runtime.ErrApprovalExpired), errors.Is(err, runtime.ErrQuestionAlreadyAnswered), errors.Is(err, runtime.ErrQuestionExpired), errors.Is(err, runtime.ErrRecoveryBusy):
		return &Error{Code: CodeConflict, Message: err.Error()}
	case errors.Is(err, runtime.ErrApprovalNotFound), errors.Is(err, runtime.ErrQuestionNotFound):
		return &Error{Code: CodeNotFound, Message: err.Error()}
	default:
		return internalError(err)
	}
}

func internalError(err error) *Error {
	if err == nil {
		return &Error{Code: InternalError, Message: "internal error"}
	}
	return &Error{Code: InternalError, Message: "internal error"}
}

func newControlID(prefix string) string {
	bytes := make([]byte, 8)
	if _, err := rand.Read(bytes); err != nil {
		return prefix + "fallback"
	}
	return prefix + hex.EncodeToString(bytes)
}

func nowMillis() int64 {
	return timeNow().UnixMilli()
}

var timeNow = func() time.Time { return time.Now() }
