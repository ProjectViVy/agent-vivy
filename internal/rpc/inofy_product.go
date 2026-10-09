package rpc

// inofy.* host action surface (S11-F). The workflow editor bridge calls these
// literal method names (studio/src/vivy-transport.ts); every session-scoped
// operation resolves the caller's session from the bound peer identity or a
// session_id param validated through the SessionStore. Live event
// subscription ("inofy.events" channel) reuses the run/subscribe machinery —
// the UI bridge maps event.seq/type/payload onto the editor's RunEvent shape.

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/ProjectViVy/inofy"
	"github.com/ProjectViVy/inofy/definitions"

	"agent-vivy/internal/actionhost"
	"agent-vivy/internal/domain"
	"agent-vivy/internal/runtime"
	"agent-vivy/internal/storage"
)

// inofySessionID resolves the product caller's session. The peer carries a
// transport-attested Face identity; the session component arrives only after
// a server-validated bind (session/get, or session_id echoed on any request
// and checked against the SessionStore here).
func (h *controlHandler) inofySessionID(ctx context.Context, peer *Peer, request Request) (domain.SessionID, *Error) {
	authCtx := peer.authenticatedContext(ctx)
	if identity, ok := actionhost.IdentityFromContext(authCtx); ok && strings.TrimSpace(identity.SessionID) != "" {
		return domain.SessionID(strings.TrimSpace(identity.SessionID)), nil
	}
	h.bindPeerSessionRequest(ctx, peer, request)
	authCtx = peer.authenticatedContext(ctx)
	if identity, ok := actionhost.IdentityFromContext(authCtx); ok && strings.TrimSpace(identity.SessionID) != "" {
		return domain.SessionID(strings.TrimSpace(identity.SessionID)), nil
	}
	return "", &Error{Code: InvalidParams, Message: "inofy methods require a session_id bound to a live session"}
}

// inofyErrorData carries the engine error code so the editor's TransportError
// surfaces the stable code instead of a bare rpc_error.
func inofyErrorData(code string) json.RawMessage {
	data, _ := json.Marshal(map[string]string{"code": code})
	return data
}

func inofyRPCError(err error) *Error {
	var ierr *inofy.Error
	switch {
	case errors.Is(err, storage.ErrNotFound):
		return &Error{Code: CodeNotFound, Message: "workflow, revision or run not found", Data: inofyErrorData("not_found")}
	case errors.Is(err, storage.ErrWorkflowDefinitionConflict), errors.Is(err, storage.ErrWorkflowRevisionConflict):
		return &Error{Code: CodeConflict, Message: "workflow definition etag or publication conflicts", Data: inofyErrorData("revision_conflict")}
	case errors.Is(err, storage.ErrWorkflowDefinitionAuthor):
		return &Error{Code: CodeConflict, Message: "workflow definition belongs to another author", Data: inofyErrorData(string(inofy.ErrAuthorityDenied))}
	case errors.Is(err, storage.ErrWorkflowCursorInvalid):
		return &Error{Code: InvalidParams, Message: err.Error(), Data: inofyErrorData("invalid_input")}
	case errors.Is(err, runtime.ErrWorkflowProductUnavailable):
		return &Error{Code: CodeConflict, Message: "workflow product is not available on this deployment", Data: inofyErrorData("unavailable")}
	case errors.Is(err, runtime.ErrINOFYInvalidDefinition):
		return &Error{Code: InvalidParams, Message: err.Error(), Data: inofyErrorData(string(inofy.ErrInvalidDefinition))}
	case errors.As(err, &ierr):
		code := CodeConflict
		switch ierr.Code {
		case inofy.ErrInvalidDefinition, inofy.ErrUnknownNodeType, inofy.ErrSchemaMismatch, inofy.ErrBindingMissing:
			code = InvalidParams
		}
		return &Error{Code: code, Message: ierr.Error(), Data: inofyErrorData(string(ierr.Code))}
	}
	return workflowRPCError(err)
}

func (h *controlHandler) inofyService(ctx context.Context) (*runtime.Service, *Error) {
	if h.deps.Service == nil {
		return nil, &Error{Code: CodeConflict, Message: "service unavailable", Data: inofyErrorData("unavailable")}
	}
	return h.deps.Service, nil
}

func (h *controlHandler) inofyCapabilities(ctx context.Context) (any, *Error) {
	svc, rpcErr := h.inofyService(ctx)
	if rpcErr != nil {
		return nil, rpcErr
	}
	caps, err := svc.INOFYCapabilities(ctx)
	if err != nil {
		return nil, inofyRPCError(err)
	}
	return caps, nil
}

func (h *controlHandler) inofyNodeTypes(ctx context.Context) (any, *Error) {
	svc, rpcErr := h.inofyService(ctx)
	if rpcErr != nil {
		return nil, rpcErr
	}
	descriptors, err := svc.INOFYNodeTypes(ctx)
	if err != nil {
		return nil, inofyRPCError(err)
	}
	return descriptors, nil
}

type inofySessionParams struct {
	SessionID string `json:"session_id"`
}

func (h *controlHandler) inofyListWorkflows(ctx context.Context, peer *Peer, request Request) (any, *Error) {
	svc, rpcErr := h.inofyService(ctx)
	if rpcErr != nil {
		return nil, rpcErr
	}
	sessionID, rpcErr := h.inofySessionID(ctx, peer, request)
	if rpcErr != nil {
		return nil, rpcErr
	}
	var params struct {
		inofySessionParams
		Cursor string `json:"cursor"`
		Limit  int    `json:"limit"`
	}
	if err := decodeParams(request, &params); err != nil {
		return nil, err
	}
	page, err := svc.INOFYListWorkflows(ctx, sessionID, strings.TrimSpace(params.Cursor), params.Limit)
	if err != nil {
		return nil, inofyRPCError(err)
	}
	items := make([]map[string]any, 0, len(page.Revisions))
	for _, rev := range page.Revisions {
		items = append(items, map[string]any{"workflow_id": rev.WorkflowID, "revision": rev.Revision})
	}
	return map[string]any{"items": items, "next_cursor": page.NextCursor}, nil
}

func (h *controlHandler) inofyLoadDraft(ctx context.Context, peer *Peer, request Request) (any, *Error) {
	svc, rpcErr := h.inofyService(ctx)
	if rpcErr != nil {
		return nil, rpcErr
	}
	sessionID, rpcErr := h.inofySessionID(ctx, peer, request)
	if rpcErr != nil {
		return nil, rpcErr
	}
	var params struct {
		inofySessionParams
		Workflow string `json:"workflow"`
	}
	if err := decodeParams(request, &params); err != nil {
		return nil, err
	}
	draft, err := svc.INOFYLoadDraft(ctx, sessionID, strings.TrimSpace(params.Workflow))
	if err != nil {
		return nil, inofyRPCError(err)
	}
	return map[string]any{"workflow": draft.WorkflowID, "etag": draft.ETag, "artifact": draft.Artifact}, nil
}

func (h *controlHandler) inofySaveDraft(ctx context.Context, peer *Peer, request Request) (any, *Error) {
	svc, rpcErr := h.inofyService(ctx)
	if rpcErr != nil {
		return nil, rpcErr
	}
	sessionID, rpcErr := h.inofySessionID(ctx, peer, request)
	if rpcErr != nil {
		return nil, rpcErr
	}
	var params struct {
		inofySessionParams
		Workflow string          `json:"workflow"`
		Artifact json.RawMessage `json:"artifact"`
		Create   bool            `json:"create"`
		ETag     *string         `json:"etag"`
	}
	if err := decodeParams(request, &params); err != nil {
		return nil, err
	}
	expectedETag := ""
	if params.Create {
		if params.ETag != nil && *params.ETag != "" {
			return nil, &Error{Code: InvalidParams, Message: "create cannot include an edit ETag"}
		}
		expectedETag = definitions.ETagAbsent
	} else {
		if params.ETag == nil || *params.ETag == "" {
			return nil, &Error{Code: InvalidParams, Message: "a nonempty ETag is required when editing a draft"}
		}
		expectedETag = *params.ETag
	}
	if len(params.Artifact) == 0 {
		return nil, &Error{Code: InvalidParams, Message: "artifact is required"}
	}
	draft, err := svc.INOFYSaveDraft(ctx, sessionID, strings.TrimSpace(params.Workflow), expectedETag, params.Artifact)
	if err != nil {
		return nil, inofyRPCError(err)
	}
	return map[string]any{"workflow": draft.WorkflowID, "etag": draft.ETag, "artifact": draft.Artifact}, nil
}

func (h *controlHandler) inofyValidateDraft(ctx context.Context, peer *Peer, request Request) (any, *Error) {
	svc, rpcErr := h.inofyService(ctx)
	if rpcErr != nil {
		return nil, rpcErr
	}
	sessionID, rpcErr := h.inofySessionID(ctx, peer, request)
	if rpcErr != nil {
		return nil, rpcErr
	}
	var params struct {
		inofySessionParams
		Workflow string `json:"workflow"`
		ETag     string `json:"etag"`
	}
	if err := decodeParams(request, &params); err != nil {
		return nil, err
	}
	diagnostics, err := svc.INOFYValidateDraft(ctx, sessionID, strings.TrimSpace(params.Workflow), params.ETag)
	if err != nil {
		return nil, inofyRPCError(err)
	}
	if diagnostics == nil {
		diagnostics = []inofy.Diagnostic{}
	}
	return map[string]any{"valid": len(diagnostics) == 0, "diagnostics": diagnostics}, nil
}

func (h *controlHandler) inofyPublish(ctx context.Context, peer *Peer, request Request) (any, *Error) {
	svc, rpcErr := h.inofyService(ctx)
	if rpcErr != nil {
		return nil, rpcErr
	}
	sessionID, rpcErr := h.inofySessionID(ctx, peer, request)
	if rpcErr != nil {
		return nil, rpcErr
	}
	var params struct {
		inofySessionParams
		Workflow string `json:"workflow"`
		ETag     string `json:"etag"`
	}
	if err := decodeParams(request, &params); err != nil {
		return nil, err
	}
	revision, err := svc.INOFYPublishDraft(ctx, sessionID, strings.TrimSpace(params.Workflow), params.ETag)
	if err != nil {
		return nil, inofyRPCError(err)
	}
	return map[string]any{"workflow": revision.WorkflowID, "revision": revision.Revision, "definition_digest": revision.DefinitionDigest}, nil
}

func (h *controlHandler) inofyGetRevision(ctx context.Context, peer *Peer, request Request) (any, *Error) {
	svc, rpcErr := h.inofyService(ctx)
	if rpcErr != nil {
		return nil, rpcErr
	}
	sessionID, rpcErr := h.inofySessionID(ctx, peer, request)
	if rpcErr != nil {
		return nil, rpcErr
	}
	var params struct {
		inofySessionParams
		Workflow string `json:"workflow"`
		Revision uint64 `json:"revision"`
	}
	if err := decodeParams(request, &params); err != nil {
		return nil, err
	}
	if params.Revision == 0 {
		return nil, &Error{Code: InvalidParams, Message: "revision is required"}
	}
	revision, err := svc.INOFYGetRevision(ctx, sessionID, strings.TrimSpace(params.Workflow), params.Revision)
	if err != nil {
		return nil, inofyRPCError(err)
	}
	return map[string]any{
		"workflow": revision.WorkflowID, "revision": revision.Revision,
		"definition_digest": revision.DefinitionDigest, "artifact": revision.Artifact,
		"used_catalog_digest": revision.UsedCatalogDigest,
	}, nil
}

func (h *controlHandler) inofyStartRun(ctx context.Context, peer *Peer, request Request) (any, *Error) {
	svc, rpcErr := h.inofyService(ctx)
	if rpcErr != nil {
		return nil, rpcErr
	}
	sessionID, rpcErr := h.inofySessionID(ctx, peer, request)
	if rpcErr != nil {
		return nil, rpcErr
	}
	var params struct {
		inofySessionParams
		Workflow     string          `json:"workflow"`
		Revision     uint64          `json:"revision"`
		DraftETag    string          `json:"draft_etag"`
		Input        json.RawMessage `json:"input"`
		ParentRunID  string          `json:"parent_run_id"`
		OperationKey string          `json:"operation_id"`
	}
	if err := decodeParams(request, &params); err != nil {
		return nil, err
	}
	if strings.TrimSpace(params.SessionID) == "" || strings.TrimSpace(params.SessionID) != string(sessionID) {
		return nil, &Error{Code: InvalidParams, Message: "captured session_id must match the bound session"}
	}
	params.ParentRunID, params.OperationKey = strings.TrimSpace(params.ParentRunID), strings.TrimSpace(params.OperationKey)
	if params.ParentRunID == "" || params.OperationKey == "" || len(params.OperationKey) > 128 {
		return nil, &Error{Code: InvalidParams, Message: "parent_run_id and operation_id up to 128 bytes are required"}
	}
	started, err := svc.INOFYStartRun(ctx, sessionID, runtime.INOFYStartRunParams{
		ParentRunID:  domain.RunID(params.ParentRunID),
		OperationKey: params.OperationKey,
		WorkflowID:   strings.TrimSpace(params.Workflow),
		Revision:     params.Revision,
		DraftETag:    params.DraftETag,
		Input:        params.Input,
	})
	if err != nil {
		return nil, inofyRPCError(err)
	}
	return map[string]any{"run_id": string(started.Run.ID), "status": string(started.Run.Status), "created": started.Created}, nil
}

func (h *controlHandler) inofyListRuns(ctx context.Context, peer *Peer, request Request) (any, *Error) {
	svc, rpcErr := h.inofyService(ctx)
	if rpcErr != nil {
		return nil, rpcErr
	}
	sessionID, rpcErr := h.inofySessionID(ctx, peer, request)
	if rpcErr != nil {
		return nil, rpcErr
	}
	var params struct {
		inofySessionParams
		Cursor string `json:"cursor"`
		Limit  int    `json:"limit"`
	}
	if err := decodeParams(request, &params); err != nil {
		return nil, err
	}
	page, err := svc.INOFYListRuns(ctx, sessionID, strings.TrimSpace(params.Cursor), params.Limit)
	if err != nil {
		return nil, inofyRPCError(err)
	}
	items := make([]map[string]any, 0, len(page.Runs))
	for _, run := range page.Runs {
		items = append(items, map[string]any{
			"run_id": run.RunID, "status": run.Status,
			"workflow_id": run.DefinitionID, "revision": run.Revision,
			"created_at": run.CreatedAt,
		})
	}
	return map[string]any{"items": items, "next_cursor": page.NextCursor}, nil
}

func (h *controlHandler) inofyGetRun(ctx context.Context, peer *Peer, request Request) (any, *Error) {
	svc, rpcErr := h.inofyService(ctx)
	if rpcErr != nil {
		return nil, rpcErr
	}
	sessionID, rpcErr := h.inofySessionID(ctx, peer, request)
	if rpcErr != nil {
		return nil, rpcErr
	}
	params, rpcErr := parseRunParams(request)
	if rpcErr != nil {
		return nil, rpcErr
	}
	view, err := svc.INOFYGetRun(ctx, sessionID, domain.RunID(params.RunID))
	if err != nil {
		return nil, inofyRPCError(err)
	}
	return map[string]any{
		"run_id": string(view.Details.Run.ID), "status": string(view.Details.Run.Status),
		"workflow_id": view.Revision.DefinitionID, "revision": view.Revision.DefinitionRevision,
		"created_at": view.Details.Run.CreatedAt, "nodes": view.Details.Nodes,
		"outputs": view.Details.Outputs, "engine_status": view.Details.EngineStatus,
	}, nil
}

func (h *controlHandler) inofyNodeOutput(ctx context.Context, peer *Peer, request Request) (any, *Error) {
	svc, rpcErr := h.inofyService(ctx)
	if rpcErr != nil {
		return nil, rpcErr
	}
	sessionID, rpcErr := h.inofySessionID(ctx, peer, request)
	if rpcErr != nil {
		return nil, rpcErr
	}
	var params struct {
		inofySessionParams
		RunID string `json:"run_id"`
		Node  string `json:"node"`
	}
	if err := decodeParams(request, &params); err != nil {
		return nil, err
	}
	if strings.TrimSpace(params.RunID) == "" || strings.TrimSpace(params.Node) == "" {
		return nil, &Error{Code: InvalidParams, Message: "run_id and node are required"}
	}
	output, err := svc.INOFYNodeOutput(ctx, sessionID, domain.RunID(strings.TrimSpace(params.RunID)), params.Node)
	if err != nil {
		return nil, inofyRPCError(err)
	}
	return map[string]any{"output": output}, nil
}

func (h *controlHandler) inofyCancelRun(ctx context.Context, peer *Peer, request Request) (any, *Error) {
	svc, rpcErr := h.inofyService(ctx)
	if rpcErr != nil {
		return nil, rpcErr
	}
	sessionID, rpcErr := h.inofySessionID(ctx, peer, request)
	if rpcErr != nil {
		return nil, rpcErr
	}
	params, rpcErr := parseRunParams(request)
	if rpcErr != nil {
		return nil, rpcErr
	}
	run, err := svc.INOFYCancelRun(ctx, sessionID, domain.RunID(params.RunID))
	if err != nil {
		return nil, inofyRPCError(err)
	}
	return map[string]any{"ok": true, "run_id": string(run.ID), "status": string(run.Status)}, nil
}

func (h *controlHandler) inofyResumeRun(ctx context.Context, peer *Peer, request Request) (any, *Error) {
	svc, rpcErr := h.inofyService(ctx)
	if rpcErr != nil {
		return nil, rpcErr
	}
	sessionID, rpcErr := h.inofySessionID(ctx, peer, request)
	if rpcErr != nil {
		return nil, rpcErr
	}
	params, rpcErr := parseRunParams(request)
	if rpcErr != nil {
		return nil, rpcErr
	}
	if err := svc.INOFYResumeRun(ctx, sessionID, domain.RunID(params.RunID)); err != nil {
		return nil, inofyRPCError(err)
	}
	return map[string]any{"ok": true}, nil
}

// inofyRunEvents pages the durable Journal for the session's own run in the
// journal's own vocabulary ({seq,type,at,data}); the editor bridge maps
// journal types onto engine event kinds on the client.
func (h *controlHandler) inofyRunEvents(ctx context.Context, peer *Peer, request Request) (any, *Error) {
	svc, rpcErr := h.inofyService(ctx)
	if rpcErr != nil {
		return nil, rpcErr
	}
	sessionID, rpcErr := h.inofySessionID(ctx, peer, request)
	if rpcErr != nil {
		return nil, rpcErr
	}
	var params struct {
		inofySessionParams
		RunID string `json:"run_id"`
		After uint64 `json:"after"`
		Limit int    `json:"limit"`
	}
	if err := decodeParams(request, &params); err != nil {
		return nil, err
	}
	if strings.TrimSpace(params.RunID) == "" {
		return nil, &Error{Code: InvalidParams, Message: "run_id is required"}
	}
	events, nextCursor, more, err := svc.INOFYRunEvents(ctx, sessionID, domain.RunID(strings.TrimSpace(params.RunID)), params.After, params.Limit)
	if err != nil {
		return nil, inofyRPCError(err)
	}
	items := make([]map[string]any, 0, len(events))
	for _, event := range events {
		items = append(items, map[string]any{
			"seq": uint64(event.Seq), "type": string(event.Type),
			"at": event.CreatedAt, "data": json.RawMessage(event.Payload),
		})
	}
	var cursor any
	if more {
		cursor = nextCursor
	}
	return map[string]any{"events": items, "next_cursor": cursor}, nil
}

// inofyListConnections exposes the existing provider registry read-only as
// editor connections (kind/base_url/model/has_secret). Credentials never
// cross this surface and lifecycle stays in settings/providers.
func (h *controlHandler) inofyListConnections(ctx context.Context) (any, *Error) {
	if h.deps.SettingsPath == "" {
		return []map[string]any{}, nil
	}
	s, rpcErr := h.loadSettingsOrError()
	if rpcErr != nil {
		return nil, rpcErr
	}
	connections := make([]map[string]any, 0, len(s.Providers))
	for _, e := range s.Providers {
		connections = append(connections, map[string]any{
			"id": e.ID, "kind": e.Bundle, "base_url": e.BaseURL,
			"model": e.DefaultModel, "has_secret": e.ApiKey != "", "source": "file",
		})
	}
	return connections, nil
}

// inofyConnectionUnsupported honestly reports that the host manages provider
// credentials itself; the editor must not duplicate a credential store.
func (h *controlHandler) inofyConnectionUnsupported() (any, *Error) {
	return nil, &Error{
		Code:    CodeConflict,
		Message: "connections are managed by the host provider settings; the inofy surface is read-only",
		Data:    inofyErrorData(string(inofy.ErrUnsupportedFeature)),
	}
}
