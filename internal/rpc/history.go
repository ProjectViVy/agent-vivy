package rpc

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/runtime"
	"agent-vivy/internal/storage"
	"agent-vivy/internal/tools"
)

type historySearchParams struct {
	SessionID string `json:"session_id"`
	domain.HistorySearchRequest
}

type historyReadParams struct {
	SessionID string `json:"session_id"`
	domain.HistoryReadRequest
}

type historyTraceParams struct {
	SessionID string `json:"session_id"`
	domain.HistoryTraceRequest
}

type historySessionParams struct {
	Query  string `json:"query"`
	Cursor string `json:"cursor"`
	Limit  int    `json:"limit"`
}

type historySessionResult struct {
	ID             string `json:"id"`
	Title          string `json:"title"`
	WorkspaceLabel string `json:"workspace_label"`
	UpdatedAt      int64  `json:"updated_at"`
}

func rejectHistorySpoofFields(request Request) *Error {
	if len(request.Params) == 0 || string(request.Params) == "null" {
		return nil
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(request.Params, &raw); err != nil {
		return &Error{Code: InvalidParams, Message: "params must be a valid JSON object"}
	}
	for _, field := range []string{"actor", "caller", "identity", "permissions", "scope_hash", "accepted_scope", "destination_session_id"} {
		if _, ok := raw[field]; ok {
			return &Error{Code: InvalidParams, Message: field + " is server-derived"}
		}
	}
	return nil
}

func (h *controlHandler) historyAuthorityContext(ctx context.Context, sessionID string) (context.Context, *Error) {
	if h.deps.History == nil {
		return nil, &Error{Code: MethodNotFound, Message: "history operations are not configured"}
	}
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return nil, &Error{Code: InvalidParams, Message: "session_id is required"}
	}
	if h.deps.Sessions == nil {
		return nil, &Error{Code: MethodNotFound, Message: "session store is not configured"}
	}
	if _, err := h.deps.Sessions.GetSession(ctx, domain.SessionID(sessionID)); err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return nil, &Error{Code: CodeNotFound, Message: "session not found"}
		}
		return nil, internalError(err)
	}
	return runtime.WithHistoryOperator(tools.WithSessionID(ctx, domain.SessionID(sessionID))), nil
}

func (h *controlHandler) historySearch(ctx context.Context, request Request) (any, *Error) {
	if rpcErr := rejectHistorySpoofFields(request); rpcErr != nil {
		return nil, rpcErr
	}
	var params historySearchParams
	if err := decodeParams(request, &params); err != nil {
		return nil, err
	}
	ctx, rpcErr := h.historyAuthorityContext(ctx, params.SessionID)
	if rpcErr != nil {
		return nil, rpcErr
	}
	page, err := h.deps.History.Search(ctx, params.HistorySearchRequest)
	if err != nil {
		return nil, internalError(err)
	}
	return page, nil
}

func (h *controlHandler) historyRead(ctx context.Context, request Request) (any, *Error) {
	if rpcErr := rejectHistorySpoofFields(request); rpcErr != nil {
		return nil, rpcErr
	}
	var params historyReadParams
	if err := decodeParams(request, &params); err != nil {
		return nil, err
	}
	ctx, rpcErr := h.historyAuthorityContext(ctx, params.SessionID)
	if rpcErr != nil {
		return nil, rpcErr
	}
	page, err := h.deps.History.Read(ctx, params.HistoryReadRequest)
	if err != nil {
		return nil, internalError(err)
	}
	return page, nil
}

func (h *controlHandler) historyTrace(ctx context.Context, request Request) (any, *Error) {
	if rpcErr := rejectHistorySpoofFields(request); rpcErr != nil {
		return nil, rpcErr
	}
	var params historyTraceParams
	if err := decodeParams(request, &params); err != nil {
		return nil, err
	}
	ctx, rpcErr := h.historyAuthorityContext(ctx, params.SessionID)
	if rpcErr != nil {
		return nil, rpcErr
	}
	page, err := h.deps.History.Trace(ctx, params.HistoryTraceRequest)
	if err != nil {
		return nil, internalError(err)
	}
	return page, nil
}

func (h *controlHandler) historyCapabilities(ctx context.Context) (any, *Error) {
	if h.deps.History == nil {
		return nil, &Error{Code: MethodNotFound, Message: "history operations are not configured"}
	}
	provider, ok := h.deps.History.(tools.HistoryCapabilitiesOperations)
	if !ok {
		return nil, &Error{Code: MethodNotFound, Message: "history capabilities are not supported"}
	}
	capabilities, err := provider.Capabilities(ctx)
	if err != nil {
		return nil, internalError(err)
	}
	return capabilities, nil
}

func (h *controlHandler) historySessions(ctx context.Context, request Request) (any, *Error) {
	if h.deps.Sessions == nil {
		return nil, &Error{Code: MethodNotFound, Message: "session store is not configured"}
	}
	var params historySessionParams
	if err := decodeParams(request, &params); err != nil {
		return nil, err
	}
	if params.Limit == 0 {
		params.Limit = 20
	}
	if params.Limit < 1 || params.Limit > 50 {
		return nil, &Error{Code: InvalidParams, Message: "limit must be between 1 and 50"}
	}
	offset, err := decodeHistorySessionCursor(params.Cursor)
	if err != nil {
		return nil, &Error{Code: InvalidParams, Message: "invalid history session cursor"}
	}
	sessions, err := h.deps.Sessions.ListSessions(ctx)
	if err != nil {
		return nil, internalError(err)
	}
	query := strings.ToLower(strings.TrimSpace(params.Query))
	filtered := make([]domain.Session, 0, len(sessions))
	for _, session := range sessions {
		if query != "" && !strings.Contains(strings.ToLower(string(session.ID)), query) && !strings.Contains(strings.ToLower(session.Title), query) {
			continue
		}
		filtered = append(filtered, session)
	}
	sort.SliceStable(filtered, func(i, j int) bool {
		left := filtered[i].UpdatedAt
		if left == 0 {
			left = filtered[i].CreatedAt
		}
		right := filtered[j].UpdatedAt
		if right == 0 {
			right = filtered[j].CreatedAt
		}
		if left != right {
			return left > right
		}
		return filtered[i].ID > filtered[j].ID
	})
	if offset > len(filtered) {
		return nil, &Error{Code: InvalidParams, Message: "invalid history session cursor"}
	}
	end := offset + params.Limit
	if end > len(filtered) {
		end = len(filtered)
	}
	result := make([]historySessionResult, 0, end-offset)
	for _, session := range filtered[offset:end] {
		label := "private"
		if strings.TrimSpace(session.WorkspacePath) != "" {
			label = "selected"
		}
		updated := session.UpdatedAt
		if updated == 0 {
			updated = session.CreatedAt
		}
		result = append(result, historySessionResult{ID: string(session.ID), Title: session.Title, WorkspaceLabel: label, UpdatedAt: updated})
	}
	response := map[string]any{
		"sessions":  result,
		"truncated": end < len(filtered),
	}
	if end < len(filtered) {
		response["next_cursor"] = encodeHistorySessionCursor(end)
	}
	return response, nil
}

func encodeHistorySessionCursor(offset int) string {
	return base64.RawURLEncoding.EncodeToString([]byte(fmt.Sprintf("%d", offset)))
}

func decodeHistorySessionCursor(cursor string) (int, error) {
	if cursor == "" {
		return 0, nil
	}
	raw, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil || len(raw) > 32 {
		return 0, errors.New("invalid cursor")
	}
	var offset int
	if _, err := fmt.Sscanf(string(raw), "%d", &offset); err != nil || offset < 0 {
		return 0, errors.New("invalid cursor")
	}
	return offset, nil
}
