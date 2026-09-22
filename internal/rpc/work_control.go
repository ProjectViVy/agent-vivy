package rpc

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"

	"agent-vivy/internal/actionhost"
	"agent-vivy/internal/domain"
	"agent-vivy/internal/runtime"
	"agent-vivy/internal/storage"
)

const (
	maxWorkRequestIDBytes = 128
	maxWorkIdentifierBytes = 256
	maxGoalObjectiveBytes = 8 << 10
	maxPlanMarkdownBytes = 256 << 10
	maxPlanFeedbackBytes = 8 << 10
	maxWorkReasonBytes = 4 << 10
	maxGoalRounds = 1000
)

type workParams struct {
	SessionID string `json:"session_id"`
	ExpectedVersion int64 `json:"expected_version"`
	RequestID string `json:"request_id"`
	GoalID string `json:"goal_id,omitempty"`
	GoalRevision int64 `json:"goal_revision,omitempty"`
	Objective string `json:"objective,omitempty"`
	MaxRounds int `json:"max_rounds,omitempty"`
	Reason string `json:"reason,omitempty"`
	PlanSubmissionID string `json:"submission_id,omitempty"`
	PlanMarkdown string `json:"markdown,omitempty"`
	PlanAction string `json:"action,omitempty"`
	PlanFeedback string `json:"feedback,omitempty"`
	PlanOriginRunID string `json:"origin_run_id,omitempty"`
	PlanOriginToolCallID string `json:"origin_tool_call_id,omitempty"`
}

type workGoalResult struct {
	ID string `json:"id"`
	Revision int64 `json:"revision"`
	Objective string `json:"objective"`
	Phase string `json:"phase"`
	MaxRounds int `json:"max_rounds"`
	RoundsStarted int `json:"rounds_started"`
}

type workPlanResult struct {
	Active bool `json:"active"`
	SubmissionID string `json:"submission_id,omitempty"`
	Markdown string `json:"markdown,omitempty"`
	ReviewStatus string `json:"review_status"`
	Feedback string `json:"feedback,omitempty"`
	OriginRunID string `json:"origin_run_id,omitempty"`
	OriginToolCallID string `json:"origin_tool_call_id,omitempty"`
}

type workStateResult struct {
	SessionID string `json:"session_id"`
	Version int64 `json:"version"`
	Goal *workGoalResult `json:"goal,omitempty"`
	Plan workPlanResult `json:"plan"`
	Activation string `json:"activation"`
}

type workEventResult struct {
	Seq int64 `json:"seq"`
	Kind string `json:"kind"`
	RequestID string `json:"request_id"`
	CreatedAt int64 `json:"created_at"`
}

type workCommitResult struct {
	Work workStateResult `json:"work"`
	Event workEventResult `json:"event"`
	Replayed bool `json:"replayed"`
}

func workStateView(state domain.WorkState) workStateResult {
	status := state.Plan.ReviewStatus
	if status == "" {
		status = domain.PlanReviewNone
	}
	result := workStateResult{
		SessionID: string(state.SessionID), Version: int64(state.Version),
		Plan: workPlanResult{
			Active: state.Plan.Active, SubmissionID: state.Plan.SubmissionID,
			Markdown: state.Plan.Markdown, ReviewStatus: string(status),
			Feedback: state.Plan.Feedback, OriginRunID: string(state.Plan.OriginRunID),
			OriginToolCallID: state.Plan.OriginToolCallID,
		},
		Activation: "disarmed",
	}
	if state.Goal != nil {
		result.Goal = &workGoalResult{
			ID: state.Goal.Ref.ID, Revision: state.Goal.Ref.Revision,
			Objective: state.Goal.Objective, Phase: string(state.Goal.Phase),
			MaxRounds: state.Goal.MaxRounds, RoundsStarted: state.Goal.RoundsStarted,
		}
	}
	return result
}

func workEventView(event domain.WorkEvent) workEventResult {
	return workEventResult{Seq: int64(event.Seq), Kind: string(event.Kind), RequestID: event.RequestID, CreatedAt: event.CreatedAt}
}

func (h *controlHandler) getWork(ctx context.Context, request Request) (any, *Error) {
	if h.deps.Work == nil || h.deps.Service == nil {
		return nil, &Error{Code: MethodNotFound, Message: "work control is not configured"}
	}
	var params workParams
	if err := decodeParams(request, &params); err != nil {
		return nil, err
	}
	sessionID, rpcErr := h.authorizeWorkSession(ctx, params.SessionID)
	if rpcErr != nil {
		return nil, rpcErr
	}
	state, err := h.deps.Service.ReadWork(ctx, sessionID)
	if err != nil {
		return nil, workError(err)
	}
	return workStateView(state), nil
}

func (h *controlHandler) handleWorkMutation(ctx context.Context, peer *Peer, request Request, kind domain.WorkEventKind) (any, *Error) {
	if h.deps.Work == nil || h.deps.Service == nil {
		return nil, &Error{Code: MethodNotFound, Message: "work control is not configured"}
	}
	var params workParams
	if err := decodeParams(request, &params); err != nil {
		return nil, err
	}
	sessionID, rpcErr := h.authorizeWorkSession(ctx, params.SessionID)
	if rpcErr != nil {
		return nil, rpcErr
	}
	params.SessionID = string(sessionID)
	mutation, rpcErr := buildWorkMutation(request.Method, kind, params)
	if rpcErr != nil {
		return nil, rpcErr
	}
	result, err := h.deps.Service.CommitWork(ctx, mutation)
	if err != nil {
		return nil, workError(err)
	}
	if peer != nil {
		h.bindPeerSessionRequest(ctx, peer, request)
	}
	return workCommitResult{Work: workStateView(result.State), Event: workEventView(result.Event), Replayed: result.Replayed}, nil
}

func (h *controlHandler) authorizeWorkSession(ctx context.Context, raw string) (domain.SessionID, *Error) {
	sessionID := strings.TrimSpace(raw)
	if sessionID == "" {
		return "", &Error{Code: InvalidParams, Message: "session_id is required"}
	}
	if len(sessionID) > maxWorkIdentifierBytes || hasWorkControl(sessionID) {
		return "", &Error{Code: InvalidParams, Message: "session_id is invalid"}
	}
	if identity, ok := actionhost.IdentityFromContext(ctx); ok && strings.TrimSpace(identity.SessionID) != "" && strings.TrimSpace(identity.SessionID) != sessionID {
		return "", &Error{Code: CodeConflict, Message: "session is not bound to this connection"}
	}
	if h.deps.Sessions == nil {
		return "", &Error{Code: MethodNotFound, Message: "session store is not configured"}
	}
	if _, err := h.deps.Sessions.GetSession(ctx, domain.SessionID(sessionID)); err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return "", &Error{Code: CodeNotFound, Message: "session not found"}
		}
		return "", internalError(err)
	}
	return domain.SessionID(sessionID), nil
}

func buildWorkMutation(method string, kind domain.WorkEventKind, params workParams) (domain.WorkMutation, *Error) {
	params.SessionID, params.RequestID = strings.TrimSpace(params.SessionID), strings.TrimSpace(params.RequestID)
	params.GoalID, params.PlanSubmissionID = strings.TrimSpace(params.GoalID), strings.TrimSpace(params.PlanSubmissionID)
	params.PlanAction = strings.TrimSpace(params.PlanAction)
	params.PlanOriginRunID, params.PlanOriginToolCallID = strings.TrimSpace(params.PlanOriginRunID), strings.TrimSpace(params.PlanOriginToolCallID)
	params.Reason, params.PlanFeedback = strings.TrimSpace(params.Reason), strings.TrimSpace(params.PlanFeedback)
	if params.ExpectedVersion < 0 {
		return domain.WorkMutation{}, &Error{Code: InvalidParams, Message: "expected_version must be non-negative"}
	}
	if params.RequestID == "" || len(params.RequestID) > maxWorkRequestIDBytes || hasWorkControl(params.RequestID) {
		return domain.WorkMutation{}, &Error{Code: InvalidParams, Message: "request_id is required and invalid"}
	}
	if len(params.GoalID) > maxWorkIdentifierBytes || len(params.PlanSubmissionID) > maxWorkIdentifierBytes ||
		len(params.PlanOriginRunID) > maxWorkIdentifierBytes || len(params.PlanOriginToolCallID) > maxWorkIdentifierBytes ||
		hasWorkControl(params.GoalID) || hasWorkControl(params.PlanSubmissionID) || hasWorkControl(params.PlanOriginRunID) || hasWorkControl(params.PlanOriginToolCallID) {
		return domain.WorkMutation{}, &Error{Code: InvalidParams, Message: "work identifier is invalid"}
	}
	if len(params.Reason) > maxWorkReasonBytes || len(params.PlanFeedback) > maxPlanFeedbackBytes ||
		len(params.Objective) > maxGoalObjectiveBytes || len(params.PlanMarkdown) > maxPlanMarkdownBytes {
		return domain.WorkMutation{}, &Error{Code: InvalidParams, Message: "work text exceeds its limit"}
	}
	if !kind.Valid() || kind == domain.WorkEventGoalRoundAdmitted {
		return domain.WorkMutation{}, &Error{Code: InvalidParams, Message: "unsupported work operation"}
	}
	if kind == domain.WorkEventPlanDecided && params.PlanAction != string(domain.PlanDecisionRevise) {
		return domain.WorkMutation{}, &Error{Code: CodeConflict, Message: "plan execution handoff is not available yet"}
	}
	mutation := domain.WorkMutation{
		SessionID: domain.SessionID(params.SessionID), ExpectedVersion: domain.WorkVersion(params.ExpectedVersion),
		RequestID: params.RequestID, RequestHash: hashWorkRequest(method, params), Kind: kind, Reason: params.Reason,
	}
	switch kind {
	case domain.WorkEventGoalCreated:
		if params.GoalID == "" {
			params.GoalID = deterministicWorkID("goal", params.RequestID)
			mutation.RequestHash = hashWorkRequest(method, params)
		}
		if strings.TrimSpace(params.Objective) == "" || params.MaxRounds <= 0 || params.MaxRounds > maxGoalRounds {
			return domain.WorkMutation{}, &Error{Code: InvalidParams, Message: "objective and max_rounds are required"}
		}
		mutation.Goal, mutation.Objective, mutation.MaxRounds = domain.GoalRef{ID: params.GoalID, Revision: 1}, strings.TrimSpace(params.Objective), params.MaxRounds
	case domain.WorkEventGoalEdited:
		if params.GoalID == "" || params.GoalRevision <= 0 || params.GoalRevision >= 1<<62 ||
			strings.TrimSpace(params.Objective) == "" || params.MaxRounds <= 0 || params.MaxRounds > maxGoalRounds {
			return domain.WorkMutation{}, &Error{Code: InvalidParams, Message: "goal_id, goal_revision, objective and max_rounds are required"}
		}
		mutation.Goal, mutation.Objective, mutation.MaxRounds = domain.GoalRef{ID: params.GoalID, Revision: params.GoalRevision + 1}, strings.TrimSpace(params.Objective), params.MaxRounds
	case domain.WorkEventGoalPaused, domain.WorkEventGoalResumed, domain.WorkEventGoalCompleted, domain.WorkEventGoalBlocked, domain.WorkEventGoalCleared:
		if params.GoalID == "" || params.GoalRevision <= 0 {
			return domain.WorkMutation{}, &Error{Code: InvalidParams, Message: "goal_id and goal_revision are required"}
		}
		mutation.Goal = domain.GoalRef{ID: params.GoalID, Revision: params.GoalRevision}
	case domain.WorkEventPlanEntered, domain.WorkEventPlanLeft:
	case domain.WorkEventPlanSubmitted:
		if params.PlanSubmissionID == "" {
			params.PlanSubmissionID = deterministicWorkID("submission", params.RequestID)
			mutation.RequestHash = hashWorkRequest(method, params)
		}
		if strings.TrimSpace(params.PlanMarkdown) == "" {
			return domain.WorkMutation{}, &Error{Code: InvalidParams, Message: "markdown is required"}
		}
		mutation.PlanSubmissionID, mutation.PlanMarkdown = params.PlanSubmissionID, params.PlanMarkdown
		mutation.PlanOriginRunID, mutation.PlanOriginToolCallID = domain.RunID(params.PlanOriginRunID), params.PlanOriginToolCallID
	case domain.WorkEventPlanDecided:
		if params.PlanSubmissionID == "" || params.PlanAction == "" {
			return domain.WorkMutation{}, &Error{Code: InvalidParams, Message: "submission_id and action are required"}
		}
		mutation.PlanSubmissionID, mutation.PlanAction, mutation.PlanFeedback = params.PlanSubmissionID, domain.PlanDecisionAction(params.PlanAction), params.PlanFeedback
	default:
		return domain.WorkMutation{}, &Error{Code: InvalidParams, Message: "unsupported work operation"}
	}
	return mutation, nil
}

func hashWorkRequest(method string, params workParams) string {
	raw, _ := json.Marshal([]any{method, params})
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func deterministicWorkID(prefix, requestID string) string {
	sum := sha256.Sum256([]byte(prefix + "\x00" + requestID))
	return prefix + "-" + hex.EncodeToString(sum[:8])
}

func hasWorkControl(value string) bool {
	return strings.IndexFunc(value, func(r rune) bool { return r < 0x20 || r == 0x7f }) >= 0
}

func workError(err error) *Error {
	switch {
	case errors.Is(err, runtime.ErrWorkUnavailable):
		return &Error{Code: MethodNotFound, Message: "work control is not configured"}
	case errors.Is(err, runtime.ErrWorkSessionRequired):
		return &Error{Code: InvalidParams, Message: "session_id is required"}
	case errors.Is(err, storage.ErrNotFound):
		return &Error{Code: CodeNotFound, Message: "session not found"}
	case errors.Is(err, storage.ErrWorkVersionConflict):
		return &Error{Code: CodeConflict, Message: "stale work version"}
	case errors.Is(err, storage.ErrWorkRequestConflict):
		return &Error{Code: CodeConflict, Message: "request id was already used with different work"}
	case errors.Is(err, storage.ErrWorkRunConflict):
		return &Error{Code: CodeConflict, Message: "session has an active primary run"}
	case errors.Is(err, storage.ErrWorkInvalidMutation):
		return &Error{Code: InvalidParams, Message: "invalid work mutation"}
	case errors.Is(err, domain.ErrStaleGoalReference):
		return &Error{Code: CodeConflict, Message: "stale or invalid work reference"}
	case errors.Is(err, domain.ErrWorkRoundLimit):
		return &Error{Code: CodeConflict, Message: "goal round limit reached"}
	default:
		return internalError(err)
	}
}
